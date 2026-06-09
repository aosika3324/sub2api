package handler

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"
)

func TestOpenAIChatImagesStreamWriterWrapsImageEventsAsChatChunks(t *testing.T) {
	gin.SetMode(gin.TestMode)
	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	writer := newOpenAIChatImagesStreamWriter(c.Writer, "gpt-image-2")

	_, err := writer.WriteString("event: image_generation.completed\n")
	require.NoError(t, err)
	_, err = writer.WriteString(`data: {"type":"image_generation.completed","b64_json":"Zm9v","model":"gpt-image-2"}` + "\n\n")
	require.NoError(t, err)
	writer.finish()

	events := parseOpenAIChatImagesTestStreamEvents(rec.Body.String())
	require.Len(t, events, 3)
	require.Equal(t, "chat.completion.chunk", gjson.Get(events[0], "object").String())
	require.Equal(t, "gpt-image-2", gjson.Get(events[0], "model").String())
	require.Equal(t, "assistant", gjson.Get(events[0], "choices.0.delta.role").String())
	require.Equal(t, "![image_1](data:image/png;base64,Zm9v)", gjson.Get(events[0], "choices.0.delta.content").String())
	require.Equal(t, "chat.completion.chunk", gjson.Get(events[1], "object").String())
	require.Equal(t, "stop", gjson.Get(events[1], "choices.0.finish_reason").String())
	require.Equal(t, "[DONE]", events[2])
}

func TestOpenAIChatImagesStreamWriterWrapsMultipleDataPayloads(t *testing.T) {
	gin.SetMode(gin.TestMode)
	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	writer := newOpenAIChatImagesStreamWriter(c.Writer, "gpt-image-2")

	_, err := writer.WriteString("event: image_generation.completed\n")
	require.NoError(t, err)
	_, err = writer.WriteString(`data: {"type":"image_generation.completed","b64_json":"YQ=="}` + "\n")
	require.NoError(t, err)
	_, err = writer.WriteString(`data: {"type":"image_generation.completed","b64_json":"Yg=="}` + "\n\n")
	require.NoError(t, err)
	writer.finish()

	events := parseOpenAIChatImagesTestStreamEvents(rec.Body.String())
	require.Len(t, events, 4)
	require.Equal(t, "![image_1](data:image/png;base64,YQ==)", gjson.Get(events[0], "choices.0.delta.content").String())
	require.Equal(t, "![image_1](data:image/png;base64,Yg==)", gjson.Get(events[1], "choices.0.delta.content").String())
	require.Equal(t, "stop", gjson.Get(events[2], "choices.0.finish_reason").String())
	require.Equal(t, "[DONE]", events[3])
}

func TestOpenAIChatImagesStreamWriterWrapsImagesAPIDataArray(t *testing.T) {
	gin.SetMode(gin.TestMode)
	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	writer := newOpenAIChatImagesStreamWriter(c.Writer, "gpt-image-1.5")

	_, err := writer.WriteString(`data: {"created":1710000001,"data":[{"b64_json":"YQ=="},{"url":"https://example.com/b.png"}]}` + "\n\n")
	require.NoError(t, err)
	writer.finish()

	events := parseOpenAIChatImagesTestStreamEvents(rec.Body.String())
	require.Len(t, events, 3)
	content := gjson.Get(events[0], "choices.0.delta.content").String()
	require.Contains(t, content, "![image_1](data:image/png;base64,YQ==)")
	require.Contains(t, content, "![image_2](https://example.com/b.png)")
}

func TestOpenAIChatImagesStreamWriterKeepsImageErrorsAsSSEError(t *testing.T) {
	gin.SetMode(gin.TestMode)
	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	writer := newOpenAIChatImagesStreamWriter(c.Writer, "gpt-image-2")

	_, err := writer.WriteString(`event: error` + "\n")
	require.NoError(t, err)
	_, err = writer.WriteString(`data: {"type":"error","error":{"type":"upstream_error","message":"boom"}}` + "\n\n")
	require.NoError(t, err)
	writer.finish()

	body := rec.Body.String()
	require.Contains(t, body, "event: error\n")
	require.Contains(t, body, `"message":"boom"`)
	require.Contains(t, body, `"error":{"type":"upstream_error","message":"boom"}`)
	require.NotContains(t, body, "chat.completion.chunk")
	require.NotContains(t, body, `"type":"error","error"`)
}

func TestOpenAIChatImagesStreamWriterPassesThroughNonSSEJSON(t *testing.T) {
	gin.SetMode(gin.TestMode)
	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	writer := newOpenAIChatImagesStreamWriter(c.Writer, "gpt-image-2")

	_, err := writer.WriteString(`{"error":{"type":"api_error","message":"No available accounts"}}`)
	require.NoError(t, err)
	writer.finish()

	require.JSONEq(t, `{"error":{"type":"api_error","message":"No available accounts"}}`, rec.Body.String())
	require.NotContains(t, rec.Body.String(), "chat.completion.chunk")
}

func TestOpenAIChatImagesStreamWriterWrapsNonSSEImageJSON(t *testing.T) {
	gin.SetMode(gin.TestMode)
	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	writer := newOpenAIChatImagesStreamWriter(c.Writer, "gpt-image-2")

	_, err := writer.WriteString(`{"created":1710000001,"data":[{"b64_json":"YQ=="}]}`)
	require.NoError(t, err)
	writer.finish()

	events := parseOpenAIChatImagesTestStreamEvents(rec.Body.String())
	require.Len(t, events, 3)
	require.Equal(t, "chat.completion.chunk", gjson.Get(events[0], "object").String())
	require.Equal(t, "![image_1](data:image/png;base64,YQ==)", gjson.Get(events[0], "choices.0.delta.content").String())
	require.Equal(t, "[DONE]", events[2])
}

func TestOpenAIChatImagesStreamWriterWrapsSplitNonSSEImageJSON(t *testing.T) {
	gin.SetMode(gin.TestMode)
	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	writer := newOpenAIChatImagesStreamWriter(c.Writer, "gpt-image-2")

	_, err := writer.WriteString(`{"created":1710000001,`)
	require.NoError(t, err)
	_, err = writer.WriteString(`"data":[{"b64_json":"YQ==","output_format":"webp"}]}`)
	require.NoError(t, err)
	writer.finish()

	events := parseOpenAIChatImagesTestStreamEvents(rec.Body.String())
	require.Len(t, events, 3)
	require.Equal(t, "![image_1](data:image/webp;base64,YQ==)", gjson.Get(events[0], "choices.0.delta.content").String())
	require.Equal(t, "[DONE]", events[2])
}

func TestOpenAIChatImagesStreamWriterNormalizesImageErrorPayload(t *testing.T) {
	gin.SetMode(gin.TestMode)
	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	writer := newOpenAIChatImagesStreamWriter(c.Writer, "gpt-image-2")

	_, err := writer.WriteString(`event: error` + "\n")
	require.NoError(t, err)
	_, err = writer.WriteString(`data: {"type":"error","error":{"type":"upstream_error","message":"boom"}}` + "\n\n")
	require.NoError(t, err)
	writer.finish()

	body := rec.Body.String()
	require.Contains(t, body, "event: error\n")
	require.Contains(t, body, `"message":"boom"`)
	require.NotContains(t, body, `"type":"error","error"`)
}

func TestOpenAIChatImagesStreamWriterReturnsWriteError(t *testing.T) {
	gin.SetMode(gin.TestMode)
	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	writer := newOpenAIChatImagesStreamWriter(&failingChatImagesResponseWriter{ResponseWriter: c.Writer}, "gpt-image-2")

	_, err := writer.WriteString(`data: {"created":1710000001,"data":[{"b64_json":"YQ=="}]}` + "\n\n")
	require.Error(t, err)
	require.Contains(t, err.Error(), "write failed")
}

func parseOpenAIChatImagesTestStreamEvents(body string) []string {
	blocks := strings.Split(body, "\n\n")
	events := make([]string, 0, len(blocks))
	for _, block := range blocks {
		block = strings.TrimSpace(block)
		if block == "" {
			continue
		}
		for _, line := range strings.Split(block, "\n") {
			line = strings.TrimSpace(line)
			if strings.HasPrefix(line, "data: ") {
				events = append(events, strings.TrimSpace(strings.TrimPrefix(line, "data: ")))
			}
		}
	}
	return events
}

type failingChatImagesResponseWriter struct {
	gin.ResponseWriter
}

func (w *failingChatImagesResponseWriter) Write([]byte) (int, error) {
	return 0, errors.New("write failed")
}

func (w *failingChatImagesResponseWriter) Header() http.Header {
	return w.ResponseWriter.Header()
}
