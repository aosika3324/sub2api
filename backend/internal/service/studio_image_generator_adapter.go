package service

import (
	"bufio"
	"bytes"
	"context"
	"encoding/base64"
	"errors"
	"fmt"
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

	images, decodeErr := decodeStudioImagesFromCapturedBody(capture.Body(), capture.ContentType())
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
// response body (OpenAI b64_json shape: {"data":[{"b64_json":"...","content_type"?:"..."}], ...})
// and returns the decoded image bytes. This is the isolated, unit-tested core.
//
// Content type: defaults to image/png; if a per-image content type is present in
// the response (content_type / mime_type / output_format) it is used instead.
func decodeStudioImagesFromCapturedBody(body []byte, fallbackContentType string) ([]StudioGeneratedImage, error) {
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
		b64 := strings.TrimSpace(item.Get("b64_json").String())
		if b64 == "" {
			// No inline base64 for this entry (e.g. a url-only response). The studio
			// forces response_format=b64_json, so a missing field is a hard error we
			// record but keep scanning the remaining entries.
			if firstErr == nil {
				firstErr = fmt.Errorf("image studio: response entry missing b64_json")
			}
			return true
		}
		decoded, err := decodeStudioBase64(b64)
		if err != nil {
			if firstErr == nil {
				firstErr = fmt.Errorf("image studio: decode b64_json: %w", err)
			}
			return true
		}
		ct := studioImageEntryContentType(item)
		if ct == "" {
			ct = defaultCT
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
