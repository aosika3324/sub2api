package service

import (
	"bufio"
	"bytes"
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/tidwall/gjson"
)

// studioImageGeneratorAdapter adapts the shared *OpenAIGatewayService.GenerateImages
// orchestrator (which streams the upstream response body to a gin writer and
// returns only metadata) into the studioImageGenerator port the JWT image-studio
// service depends on (which yields the decoded image bytes).
//
// The crux: GenerateImages calls ForwardImages with the gin.Context's
// ResponseWriter; on the non-streaming b64_json path used by the studio
// (handleOpenAIImagesNonStreamingResponse → c.Data(...)), the upstream JSON
// response body — {"data":[{"b64_json":"..."}], ...} — lands in that writer. We
// build a gin.Context whose ResponseWriter CAPTURES everything into a buffer, run
// the orchestrator, then parse the captured body for b64_json entries.
type studioImageGeneratorAdapter struct {
	gw *OpenAIGatewayService
}

// NewStudioImageGeneratorAdapter builds the capturing adapter over the shared
// gateway orchestrator.
func NewStudioImageGeneratorAdapter(gw *OpenAIGatewayService) *studioImageGeneratorAdapter {
	return &studioImageGeneratorAdapter{gw: gw}
}

// Compile-time assertion: the adapter satisfies the (unexported) studio port.
var _ studioImageGenerator = (*studioImageGeneratorAdapter)(nil)

// GenerateStudioImages runs the shared orchestration with a body-capturing gin
// context, then decodes the captured response into image bytes.
func (a *studioImageGeneratorAdapter) GenerateStudioImages(ctx context.Context, in ImageGenInput) (*ImageGenResult, []StudioGeneratedImage, error) {
	if a == nil || a.gw == nil {
		return nil, nil, errors.New("studio image generator: nil gateway service")
	}

	capture := newCaptureResponseWriter()
	c := newStudioCaptureContext(ctx, capture)
	in.OpsContext = c

	// The studio path does not set a failover switch cap; mirror the gateway
	// handler's resolution (default 3, overridable via config) so studio
	// generations failover across accounts exactly like the API-key path.
	if in.MaxAccountSwitches <= 0 {
		in.MaxAccountSwitches = a.maxAccountSwitches()
	}

	result, err := a.gw.GenerateImages(ctx, in)
	if err != nil {
		// On error the orchestrator has returned an imageGenTerminalError; surface
		// it (and any partial result) to the caller, no images decoded.
		return result, nil, err
	}

	images, decodeErr := a.decodeStudioImagesFromCapturedBody(ctx, capture.Body(), capture.ContentType(), result)
	if decodeErr != nil {
		return result, nil, decodeErr
	}
	return result, images, nil
}

// maxAccountSwitches mirrors handler.NewOpenAIGatewayHandler's resolution.
func (a *studioImageGeneratorAdapter) maxAccountSwitches() int {
	maxSwitches := 3
	if a.gw != nil && a.gw.cfg != nil && a.gw.cfg.Gateway.MaxAccountSwitches > 0 {
		maxSwitches = a.gw.cfg.Gateway.MaxAccountSwitches
	}
	return maxSwitches
}

// decodeStudioImagesFromCapturedBody parses the captured non-streaming images
// response body (usually {"data":[{"b64_json":"..."}], ...}) and returns the
// decoded image bytes. This pure helper preserves the historical behavior: HTTP
// URL-only entries need an explicit downloader and otherwise remain an error.
//
// Content type: defaults to image/png; if a per-image content type is present in
// the response (content_type / mime_type / output_format) it is used instead.
func decodeStudioImagesFromCapturedBody(body []byte, fallbackContentType string) ([]StudioGeneratedImage, error) {
	return decodeStudioImagesFromCapturedBodyWithDownloader(context.Background(), body, fallbackContentType, nil)
}

type studioImageURLDownloader func(ctx context.Context, rawURL string) ([]byte, string, error)

func decodeStudioImagesFromCapturedBodyWithDownloader(
	ctx context.Context,
	body []byte,
	fallbackContentType string,
	downloadURL studioImageURLDownloader,
) ([]StudioGeneratedImage, error) {
	if len(bytes.TrimSpace(body)) == 0 {
		return nil, ErrImageStudioNoImages
	}
	if !gjson.ValidBytes(body) {
		return nil, fmt.Errorf("image studio: upstream response is not valid JSON")
	}

	data := gjson.GetBytes(body, "data")
	if !data.Exists() || !data.IsArray() {
		return nil, ErrImageStudioNoImages
	}

	defaultCT := normalizeStudioImageContentType(fallbackContentType)
	if defaultCT == "" {
		defaultCT = "image/png"
	}

	var images []StudioGeneratedImage
	var firstErr error
	data.ForEach(func(_, item gjson.Result) bool {
		ct := studioImageEntryContentType(item)
		if ct == "" {
			ct = defaultCT
		}

		b64 := strings.TrimSpace(item.Get("b64_json").String())
		if b64 != "" {
			decoded, err := decodeStudioBase64(b64)
			if err != nil {
				if firstErr == nil {
					firstErr = fmt.Errorf("image studio: decode b64_json: %w", err)
				}
				return true
			}
			images = append(images, StudioGeneratedImage{Data: decoded, ContentType: ct})
			return true
		}

		rawURL := studioImageEntryURL(item)
		if rawURL == "" {
			if firstErr == nil {
				firstErr = fmt.Errorf("image studio: response entry missing b64_json")
			}
			return true
		}

		decoded, downloadedCT, err := decodeStudioURLImage(ctx, rawURL, downloadURL)
		if err != nil {
			if firstErr == nil {
				firstErr = fmt.Errorf("image studio: decode url image: %w", err)
			}
			return true
		}
		if downloadedCT != "" {
			ct = downloadedCT
		} else if detected := normalizeStudioImageContentType(http.DetectContentType(decoded)); detected != "" {
			ct = detected
		}
		images = append(images, StudioGeneratedImage{Data: decoded, ContentType: ct})
		return true
	})

	if len(images) == 0 {
		if firstErr != nil {
			return nil, firstErr
		}
		return nil, ErrImageStudioNoImages
	}
	return images, nil
}

func (a *studioImageGeneratorAdapter) decodeStudioImagesFromCapturedBody(
	ctx context.Context,
	body []byte,
	fallbackContentType string,
	result *ImageGenResult,
) ([]StudioGeneratedImage, error) {
	return decodeStudioImagesFromCapturedBodyWithDownloader(
		ctx,
		body,
		fallbackContentType,
		func(ctx context.Context, rawURL string) ([]byte, string, error) {
			return a.downloadStudioImageURL(ctx, rawURL, result)
		},
	)
}

func decodeStudioURLImage(
	ctx context.Context,
	rawURL string,
	downloadURL studioImageURLDownloader,
) ([]byte, string, error) {
	rawURL = strings.TrimSpace(rawURL)
	if rawURL == "" {
		return nil, "", fmt.Errorf("empty image url")
	}
	if strings.HasPrefix(strings.ToLower(rawURL), "data:image/") {
		data, err := decodeStudioBase64(rawURL)
		return data, studioDataURLContentType(rawURL), err
	}
	if downloadURL == nil {
		return nil, "", fmt.Errorf("url-only image response requires server download")
	}
	return downloadURL(ctx, rawURL)
}

func (a *studioImageGeneratorAdapter) downloadStudioImageURL(
	ctx context.Context,
	rawURL string,
	result *ImageGenResult,
) ([]byte, string, error) {
	rawURL = strings.TrimSpace(rawURL)
	if rawURL == "" {
		return nil, "", fmt.Errorf("empty image url")
	}
	if !strings.HasPrefix(strings.ToLower(rawURL), "http://") &&
		!strings.HasPrefix(strings.ToLower(rawURL), "https://") {
		return nil, "", fmt.Errorf("unsupported image url scheme")
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, http.NoBody)
	if err != nil {
		return nil, "", err
	}
	req = req.WithContext(WithHTTPUpstreamProfile(req.Context(), HTTPUpstreamProfileOpenAI))
	req.Header.Set("Accept", "image/*,*/*;q=0.8")
	req.Header.Set("User-Agent", openAIImageBackendUserAgent)
	if result != nil && result.Account != nil {
		if customUA := strings.TrimSpace(result.Account.GetOpenAIUserAgent()); customUA != "" {
			req.Header.Set("User-Agent", customUA)
		}
		if a != nil && a.gw != nil && strings.HasPrefix(rawURL, openAIChatGPTStartURL) {
			if token, _, tokenErr := a.gw.GetAccessToken(ctx, result.Account); tokenErr == nil && strings.TrimSpace(token) != "" {
				req.Header.Set("Authorization", "Bearer "+strings.TrimSpace(token))
			}
		}
	}

	proxyURL := ""
	accountID := int64(0)
	accountConcurrency := 0
	if result != nil && result.Account != nil {
		accountID = result.Account.ID
		accountConcurrency = result.Account.Concurrency
		if result.Account.ProxyID != nil && result.Account.Proxy != nil {
			proxyURL = result.Account.Proxy.URL()
		}
	}

	var resp *http.Response
	if a != nil && a.gw != nil && a.gw.httpUpstream != nil {
		resp, err = a.gw.httpUpstream.Do(req, proxyURL, accountID, accountConcurrency)
	} else {
		resp, err = http.DefaultClient.Do(req)
	}
	if err != nil {
		return nil, "", err
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		limit := openAIUpstreamErrorBodyReadLimit
		if a != nil && a.gw != nil {
			limit = openAIUpstreamErrorBodyReadLimitForConfig(a.gw.cfg)
		}
		body, _ := io.ReadAll(io.LimitReader(resp.Body, limit))
		msg := sanitizeUpstreamErrorMessage(extractUpstreamErrorMessage(body))
		if msg == "" {
			msg = fmt.Sprintf("download image url failed: status %d", resp.StatusCode)
		}
		return nil, "", fmt.Errorf("%s", msg)
	}

	data, err := io.ReadAll(io.LimitReader(resp.Body, openAIImageMaxDownloadBytes))
	if err != nil {
		return nil, "", err
	}
	if len(data) == 0 {
		return nil, "", ErrImageStudioNoImages
	}
	ct := normalizeStudioImageContentType(resp.Header.Get("Content-Type"))
	if ct == "" {
		ct = normalizeStudioImageContentType(http.DetectContentType(data))
	}
	if ct == "" {
		return nil, "", fmt.Errorf("downloaded URL did not return an image")
	}
	return data, ct, nil
}

func studioImageEntryURL(item gjson.Result) string {
	for _, field := range []string{"download_url", "url", "image_url"} {
		if raw := strings.TrimSpace(item.Get(field).String()); raw != "" {
			return raw
		}
	}
	return ""
}

func studioDataURLContentType(raw string) string {
	raw = strings.TrimSpace(raw)
	if !strings.HasPrefix(strings.ToLower(raw), "data:") {
		return ""
	}
	comma := strings.Index(raw, ",")
	if comma < 0 {
		return ""
	}
	meta := strings.TrimPrefix(raw[:comma], "data:")
	if semi := strings.Index(meta, ";"); semi >= 0 {
		meta = meta[:semi]
	}
	return normalizeStudioImageContentType(meta)
}

// studioImageEntryContentType resolves a per-image content type from the common
// fields an upstream may carry; returns "" when none is present/usable.
func studioImageEntryContentType(item gjson.Result) string {
	for _, field := range []string{"content_type", "mime_type", "output_format"} {
		raw := strings.TrimSpace(item.Get(field).String())
		if raw == "" {
			continue
		}
		if ct := normalizeStudioImageContentType(raw); ct != "" {
			return ct
		}
	}
	return ""
}

// normalizeStudioImageContentType maps a raw content-type/format token to a sane
// image/* MIME type. Bare format tokens (e.g. "png", "jpeg", "webp") are mapped
// to image/<token>; full image/* types pass through; anything else is rejected.
func normalizeStudioImageContentType(raw string) string {
	raw = strings.ToLower(strings.TrimSpace(raw))
	if raw == "" {
		return ""
	}
	// Strip any "; charset=..." parameters.
	if idx := strings.Index(raw, ";"); idx >= 0 {
		raw = strings.TrimSpace(raw[:idx])
	}
	switch raw {
	case "png":
		return "image/png"
	case "jpg", "jpeg":
		return "image/jpeg"
	case "webp":
		return "image/webp"
	case "gif":
		return "image/gif"
	}
	if strings.HasPrefix(raw, "image/") {
		return raw
	}
	return ""
}

// decodeStudioBase64 decodes a base64 image payload, tolerating data: URLs,
// URL-safe alphabets, and missing padding (mirroring normalizeOpenAIImageBase64).
func decodeStudioBase64(raw string) ([]byte, error) {
	raw = strings.TrimSpace(raw)
	if strings.HasPrefix(strings.ToLower(raw), "data:") {
		if idx := strings.Index(raw, ","); idx >= 0 && idx+1 < len(raw) {
			raw = strings.TrimSpace(raw[idx+1:])
		}
	}
	if raw == "" {
		return nil, fmt.Errorf("empty base64 payload")
	}
	// Pad to a multiple of 4.
	if pad := (4 - len(raw)%4) % 4; pad > 0 {
		raw = raw + strings.Repeat("=", pad)
	}
	if decoded, err := base64.StdEncoding.DecodeString(raw); err == nil {
		return decoded, nil
	}
	// Fall back to the URL-safe alphabet some upstreams use.
	return base64.URLEncoding.DecodeString(raw)
}

// ---------------------------------------------------------------------------
// Capturing gin ResponseWriter + Context.
// ---------------------------------------------------------------------------

// captureResponseWriter is a minimal gin.ResponseWriter that buffers everything
// written by ForwardImages (status, headers, body) instead of sending it to a
// real client. It is cleaner than httptest for production code and lets us read
// the upstream JSON body back out.
type captureResponseWriter struct {
	header http.Header
	buf    bytes.Buffer
	status int
	size   int
}

func newCaptureResponseWriter() *captureResponseWriter {
	return &captureResponseWriter{
		header: make(http.Header),
		status: http.StatusOK,
		size:   -1,
	}
}

// Body returns the captured response body bytes.
func (w *captureResponseWriter) Body() []byte { return w.buf.Bytes() }

// ContentType returns the Content-Type the writer captured (set via headers by
// handleOpenAIImagesNonStreamingResponse before c.Data).
func (w *captureResponseWriter) ContentType() string {
	return w.header.Get("Content-Type")
}

// --- http.ResponseWriter ---

func (w *captureResponseWriter) Header() http.Header { return w.header }

func (w *captureResponseWriter) Write(data []byte) (int, error) {
	if w.size < 0 {
		w.size = 0
		w.WriteHeaderNow()
	}
	n, err := w.buf.Write(data)
	w.size += n
	return n, err
}

func (w *captureResponseWriter) WriteHeader(code int) {
	if code > 0 {
		w.status = code
	}
}

// --- gin.ResponseWriter ---

func (w *captureResponseWriter) WriteHeaderNow() {
	if w.size < 0 {
		w.size = 0
	}
}

func (w *captureResponseWriter) Status() int { return w.status }

func (w *captureResponseWriter) Size() int { return w.size }

func (w *captureResponseWriter) Written() bool { return w.size != -1 }

func (w *captureResponseWriter) WriteString(s string) (int, error) {
	return w.Write([]byte(s))
}

func (w *captureResponseWriter) Pusher() http.Pusher { return nil }

// Flush satisfies http.Flusher / gin.ResponseWriter (no-op: buffered).
func (w *captureResponseWriter) Flush() {}

// Hijack satisfies http.Hijacker; capturing writer is not hijackable.
func (w *captureResponseWriter) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	return nil, nil, fmt.Errorf("capture response writer does not support hijack")
}

// CloseNotify satisfies http.CloseNotifier (never fires: buffered, no client).
func (w *captureResponseWriter) CloseNotify() <-chan bool {
	return make(<-chan bool)
}

var _ gin.ResponseWriter = (*captureResponseWriter)(nil)

// newStudioCaptureContext builds a gin.Context wired to the capturing writer and
// carrying a request that threads the outer ctx, so imageGenRequestContext(c,...)
// and the passthrough-header reads inside ForwardImages stay safe.
func newStudioCaptureContext(ctx context.Context, w gin.ResponseWriter) *gin.Context {
	c := &gin.Context{}
	c.Writer = w
	// A non-nil Request carrying ctx: ForwardImages reads c.Request.Header for
	// passthrough headers and imageGenRequestContext reads c.Request.Context().
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, openAIImagesGenerationsURL, http.NoBody)
	if err != nil {
		// Fall back to a header-only request so downstream header reads are safe.
		req = &http.Request{Header: make(http.Header)}
		req = req.WithContext(ctx)
	}
	c.Request = req
	return c
}
