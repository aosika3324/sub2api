//go:build unit

package service

import (
	"context"
	"encoding/base64"
	"testing"

	"github.com/stretchr/testify/require"
)

// TestStudioImageDecode_SingleB64 feeds a synthetic captured non-streaming body
// (the b64_json shape the studio forces) through the decode helper and asserts
// the decoded bytes + default png content type. This isolates the testable core
// of the capturing adapter without a real upstream.
func TestStudioImageDecode_SingleB64(t *testing.T) {
	raw := []byte("hello-image-bytes-\x00\x01\x02")
	b64 := base64.StdEncoding.EncodeToString(raw)
	body := []byte(`{"created":123,"data":[{"b64_json":"` + b64 + `"}]}`)

	images, err := decodeStudioImagesFromCapturedBody(body, "application/json")
	require.NoError(t, err)
	require.Len(t, images, 1)
	require.Equal(t, raw, images[0].Data)
	require.Equal(t, "image/png", images[0].ContentType)
}

// TestStudioImageDecode_MultipleAndContentType verifies n>1 decoding and that a
// per-entry content_type/output_format overrides the png default.
func TestStudioImageDecode_MultipleAndContentType(t *testing.T) {
	a := []byte("first")
	b := []byte("second")
	b64a := base64.StdEncoding.EncodeToString(a)
	b64b := base64.StdEncoding.EncodeToString(b)
	body := []byte(`{"data":[` +
		`{"b64_json":"` + b64a + `","content_type":"image/webp"},` +
		`{"b64_json":"` + b64b + `","output_format":"jpeg"}` +
		`]}`)

	images, err := decodeStudioImagesFromCapturedBody(body, "")
	require.NoError(t, err)
	require.Len(t, images, 2)
	require.Equal(t, a, images[0].Data)
	require.Equal(t, "image/webp", images[0].ContentType)
	require.Equal(t, b, images[1].Data)
	require.Equal(t, "image/jpeg", images[1].ContentType)
}

// TestStudioImageDecode_DataURLAndPadding verifies data: URL stripping and
// tolerant padding in the base64 decode path.
func TestStudioImageDecode_DataURLAndPadding(t *testing.T) {
	raw := []byte("padme")
	// StdEncoding of "padme" needs padding; strip it and wrap in a data URL.
	b64 := base64.StdEncoding.EncodeToString(raw)
	stripped := b64
	for len(stripped) > 0 && stripped[len(stripped)-1] == '=' {
		stripped = stripped[:len(stripped)-1]
	}
	body := []byte(`{"data":[{"b64_json":"data:image/png;base64,` + stripped + `"}]}`)

	images, err := decodeStudioImagesFromCapturedBody(body, "")
	require.NoError(t, err)
	require.Len(t, images, 1)
	require.Equal(t, raw, images[0].Data)
}

// TestStudioImageDecode_NoImages covers empty, non-JSON, and empty-data bodies.
func TestStudioImageDecode_NoImages(t *testing.T) {
	t.Run("empty body", func(t *testing.T) {
		_, err := decodeStudioImagesFromCapturedBody(nil, "")
		require.ErrorIs(t, err, ErrImageStudioNoImages)
	})
	t.Run("not json", func(t *testing.T) {
		_, err := decodeStudioImagesFromCapturedBody([]byte("not json at all"), "")
		require.Error(t, err)
	})
	t.Run("empty data array", func(t *testing.T) {
		_, err := decodeStudioImagesFromCapturedBody([]byte(`{"data":[]}`), "")
		require.ErrorIs(t, err, ErrImageStudioNoImages)
	})
	t.Run("entry missing b64", func(t *testing.T) {
		_, err := decodeStudioImagesFromCapturedBody([]byte(`{"data":[{"url":"http://x/y.png"}]}`), "")
		require.Error(t, err)
	})
}

func TestStudioImageDecode_URLOnlyWithDownloader(t *testing.T) {
	body := []byte(`{"data":[{"url":"https://cdn.example.com/image.webp"}]}`)

	images, err := decodeStudioImagesFromCapturedBodyWithDownloader(
		context.Background(),
		body,
		"",
		func(ctx context.Context, rawURL string) ([]byte, string, error) {
			require.Equal(t, "https://cdn.example.com/image.webp", rawURL)
			return []byte("downloaded-image"), "image/webp", nil
		},
	)

	require.NoError(t, err)
	require.Len(t, images, 1)
	require.Equal(t, []byte("downloaded-image"), images[0].Data)
	require.Equal(t, "image/webp", images[0].ContentType)
}

func TestStudioImageDecode_DataURL(t *testing.T) {
	raw := []byte("inline-url-image")
	body := []byte(`{"data":[{"url":"data:image/jpeg;base64,` + base64.StdEncoding.EncodeToString(raw) + `"}]}`)

	images, err := decodeStudioImagesFromCapturedBody(body, "")

	require.NoError(t, err)
	require.Len(t, images, 1)
	require.Equal(t, raw, images[0].Data)
	require.Equal(t, "image/jpeg", images[0].ContentType)
}

// TestStudioCaptureWriter_CapturesBodyAndContentType verifies the capturing
// gin.ResponseWriter records body + content type the way c.Data would drive it.
func TestStudioCaptureWriter_CapturesBodyAndContentType(t *testing.T) {
	w := newCaptureResponseWriter()
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(200)
	payload := []byte(`{"data":[]}`)
	n, err := w.Write(payload)
	require.NoError(t, err)
	require.Equal(t, len(payload), n)
	require.Equal(t, payload, w.Body())
	require.Equal(t, 200, w.Status())
	require.Equal(t, "application/json", w.ContentType())
	require.True(t, w.Written())
}
