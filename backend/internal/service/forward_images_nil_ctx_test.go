//go:build unit

package service

import (
	"context"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

// TestForwardImagesNilGinContext verifies that the ops/metrics writes and
// header-passthrough reads inside the ForwardImages call chain do not panic
// when the *gin.Context argument is nil (non-HTTP / image-studio path).
//
// We test the two leaf helpers that previously dereferenced c.Request.Header
// without a nil guard:
//
//   - buildOpenAIImagesRequest  (openai_images.go)
//   - buildUpstreamRequest       (openai_gateway_service.go)
func TestForwardImagesNilGinContext_BuildOpenAIImagesRequest(t *testing.T) {
	gin.SetMode(gin.TestMode)

	svc := &OpenAIGatewayService{}
	body := []byte(`{"model":"gpt-image-1","prompt":"cat"}`)

	// c is intentionally nil — must not panic.
	require.NotPanics(t, func() {
		_, _ = svc.buildOpenAIImagesRequest(
			context.Background(),
			nil, // <- nil *gin.Context
			&Account{
				Type:        AccountTypeAPIKey,
				Credentials: map[string]any{"api_key": "sk-test"},
			},
			body,
			"application/json",
			"sk-test",
			openAIImagesGenerationsEndpoint,
		)
	}, "buildOpenAIImagesRequest must not panic when gin.Context is nil")
}

func TestForwardImagesNilGinContext_BuildUpstreamRequest(t *testing.T) {
	gin.SetMode(gin.TestMode)

	svc := &OpenAIGatewayService{}
	body := []byte(`{"model":"gpt-image-1","prompt":"cat","stream":true}`)

	// c is intentionally nil — must not panic.
	require.NotPanics(t, func() {
		_, _ = svc.buildUpstreamRequest(
			context.Background(),
			nil, // <- nil *gin.Context
			&Account{
				Type:        AccountTypeOAuth,
				Credentials: map[string]any{"api_key": "tok-test"},
			},
			body,
			"tok-test",
			true,
			"",
			false,
		)
	}, "buildUpstreamRequest must not panic when gin.Context is nil")
}

// TestForwardImagesNilGinContext_OpsWrites verifies that the ops helpers used
// by forwardOpenAIImagesAPIKey / forwardOpenAIImagesOAuth are nil-safe.
// (These already guard c == nil internally; the test documents and locks that.)
func TestForwardImagesNilGinContext_OpsWrites(t *testing.T) {
	require.NotPanics(t, func() {
		SetOpsLatencyMs(nil, OpsUpstreamLatencyMsKey, 100)
	}, "SetOpsLatencyMs must not panic with nil context")

	require.NotPanics(t, func() {
		setOpsUpstreamError(nil, 0, "some error", "")
	}, "setOpsUpstreamError must not panic with nil context")

	require.NotPanics(t, func() {
		appendOpsUpstreamError(nil, OpsUpstreamErrorEvent{
			Platform: "openai",
			Kind:     "request_error",
			Message:  "test",
		})
	}, "appendOpsUpstreamError must not panic with nil context")
}
