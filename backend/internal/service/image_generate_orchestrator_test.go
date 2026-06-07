//go:build unit

package service

import (
	"bytes"
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

// newImageGenTestContext builds a gin.Context bound to an /v1/images/generations
// POST request carrying body, mirroring the existing image service tests.
func newImageGenTestContext(t *testing.T, body []byte) *gin.Context {
	t.Helper()
	gin.SetMode(gin.TestMode)
	req := httptest.NewRequest(http.MethodPost, "/v1/images/generations", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = req
	return c
}

// stubOpenAIImageScheduler is a minimal OpenAIAccountScheduler used to drive
// GenerateImages' selection + failover loop without the real DB-backed
// load-balancer. Each Select returns the next account whose ID is not excluded.
type stubOpenAIImageScheduler struct {
	accounts    []*Account
	selectCalls int32
	switchCalls int32
	reportCalls int32
}

func (s *stubOpenAIImageScheduler) Select(_ context.Context, req OpenAIAccountScheduleRequest) (*AccountSelectionResult, OpenAIAccountScheduleDecision, error) {
	atomic.AddInt32(&s.selectCalls, 1)
	for _, account := range s.accounts {
		if req.ExcludedIDs != nil {
			if _, excluded := req.ExcludedIDs[account.ID]; excluded {
				continue
			}
		}
		return &AccountSelectionResult{Account: account, Acquired: true}, OpenAIAccountScheduleDecision{Layer: openAIAccountScheduleLayerLoadBalance}, nil
	}
	return nil, OpenAIAccountScheduleDecision{}, noAvailableOpenAISelectionError(req.RequestedModel, false)
}

func (s *stubOpenAIImageScheduler) ReportResult(int64, bool, *int) {
	atomic.AddInt32(&s.reportCalls, 1)
}
func (s *stubOpenAIImageScheduler) ReportSwitch() { atomic.AddInt32(&s.switchCalls, 1) }
func (s *stubOpenAIImageScheduler) SnapshotMetrics() OpenAIAccountSchedulerMetricsSnapshot {
	return OpenAIAccountSchedulerMetricsSnapshot{}
}

// newImageOrchestratorService wires a service whose SelectAccountWithSchedulerForImages
// reaches the injected stub scheduler (by enabling the advanced scheduler flag and
// pre-setting s.openaiScheduler so the sync.Once leaves it intact).
func newImageOrchestratorService(t *testing.T, scheduler OpenAIAccountScheduler, upstream HTTPUpstream) *OpenAIGatewayService {
	t.Helper()
	// Enable the advanced scheduler via the cached setting so getOpenAIAccountScheduler
	// returns our pre-set stub instead of falling back to the DB load-aware path.
	openAIAdvancedSchedulerSettingCache.Store(&cachedOpenAIAdvancedSchedulerSetting{
		enabled:   true,
		expiresAt: time.Now().Add(time.Hour).UnixNano(),
	})
	t.Cleanup(resetOpenAIAdvancedSchedulerSettingCacheForTest)

	svc := &OpenAIGatewayService{
		httpUpstream: upstream,
		cfg:          &config.Config{},
	}
	svc.openaiScheduler = scheduler // sync.Once.Do leaves a non-nil scheduler intact
	return svc
}

func newImageAPIKeyAccount(id int64) *Account {
	return &Account{
		ID:       id,
		Name:     "openai-apikey",
		Platform: PlatformOpenAI,
		Type:     AccountTypeAPIKey,
		Credentials: map[string]any{
			"api_key": "sk-test",
		},
	}
}

func parseImageGenRequest(t *testing.T, body []byte) *OpenAIImagesRequest {
	t.Helper()
	c := newImageGenTestContext(t, body)
	svc := &OpenAIGatewayService{}
	parsed, err := svc.ParseOpenAIImagesRequest(c, body)
	require.NoError(t, err)
	return parsed
}

func okImageResponse() *http.Response {
	return &http.Response{
		StatusCode: http.StatusOK,
		Header: http.Header{
			"Content-Type": []string{"application/json"},
			"X-Request-Id": []string{"req_ok"},
		},
		Body: io.NopCloser(strings.NewReader(
			`{"created":1710000000,"data":[{"b64_json":"aW1n"}],"usage":{"input_tokens":5,"output_tokens":0}}`,
		)),
	}
}

func failoverImageResponse() *http.Response {
	return &http.Response{
		StatusCode: http.StatusInternalServerError, // 500 -> shouldFailoverUpstreamError == true
		Header: http.Header{
			"Content-Type": []string{"application/json"},
			"X-Request-Id": []string{"req_500"},
		},
		Body: io.NopCloser(strings.NewReader(
			`{"error":{"type":"server_error","message":"upstream boom"}}`,
		)),
	}
}

// TestGenerateImages_Success verifies the happy path: the orchestrator selects an
// account, forwards, and returns the forward result + selected account with no
// switches. It must NOT record usage (that stays with the caller).
func TestGenerateImages_Success(t *testing.T) {
	body := []byte(`{"model":"gpt-image-2","prompt":"a cat","size":"1024x1024"}`)
	parsed := parseImageGenRequest(t, body)

	account := newImageAPIKeyAccount(1)
	scheduler := &stubOpenAIImageScheduler{accounts: []*Account{account}}
	upstream := &httpUpstreamRecorder{resp: okImageResponse()}
	svc := newImageOrchestratorService(t, scheduler, upstream)

	c := newImageGenTestContext(t, body)

	res, err := svc.GenerateImages(context.Background(), ImageGenInput{
		OpsContext:         c,
		APIKey:             &APIKey{ID: 42},
		Parsed:             parsed,
		Body:               body,
		RequestModel:       parsed.Model,
		RequiredCapability: parsed.RequiredCapability,
		MaxAccountSwitches: 3,
	})

	require.NoError(t, err)
	require.NotNil(t, res)
	require.NotNil(t, res.Result)
	require.NotNil(t, res.Account)
	require.Equal(t, int64(1), res.Account.ID)
	require.Equal(t, 0, res.SwitchCount)
	require.Equal(t, int32(1), atomic.LoadInt32(&scheduler.selectCalls))
}

// TestGenerateImages_FailoverSwitchesAccountsUpToCap verifies that an
// UpstreamFailoverError switches to the next account, and that the loop respects
// the max-switch cap, returning a failover-exhausted terminal error and invoking
// the OnFailoverExhausted hook exactly once.
func TestGenerateImages_FailoverSwitchesAccountsUpToCap(t *testing.T) {
	body := []byte(`{"model":"gpt-image-2","prompt":"a cat","size":"1024x1024"}`)
	parsed := parseImageGenRequest(t, body)

	// 4 accounts so the cap (not account exhaustion) is the terminator.
	accounts := []*Account{
		newImageAPIKeyAccount(1),
		newImageAPIKeyAccount(2),
		newImageAPIKeyAccount(3),
		newImageAPIKeyAccount(4),
	}
	scheduler := &stubOpenAIImageScheduler{accounts: accounts}
	// Every forward fails with a failover (500) error.
	upstream := &httpUpstreamRecorder{
		responses: []*http.Response{
			failoverImageResponse(),
			failoverImageResponse(),
			failoverImageResponse(),
			failoverImageResponse(),
		},
	}
	svc := newImageOrchestratorService(t, scheduler, upstream)

	c := newImageGenTestContext(t, body)

	const maxSwitches = 2
	var exhaustedCalls int
	var lastFailoverStatus int

	res, err := svc.GenerateImages(context.Background(), ImageGenInput{
		OpsContext:         c,
		APIKey:             &APIKey{ID: 42},
		Parsed:             parsed,
		Body:               body,
		RequestModel:       parsed.Model,
		RequiredCapability: parsed.RequiredCapability,
		MaxAccountSwitches: maxSwitches,
		Hooks: ImageGenHooks{
			OnFailoverExhausted: func(failoverErr *UpstreamFailoverError) {
				exhaustedCalls++
				if failoverErr != nil {
					lastFailoverStatus = failoverErr.StatusCode
				}
			},
		},
	})

	require.Nil(t, res)
	require.Error(t, err)

	var terminal *imageGenTerminalError
	require.ErrorAs(t, err, &terminal)
	require.Equal(t, imageGenKindFailoverExhausted, terminal.Kind)

	// Hook fired exactly once with the upstream failover status.
	require.Equal(t, 1, exhaustedCalls)
	require.Equal(t, http.StatusInternalServerError, lastFailoverStatus)

	// Switch cap (faithful to the original inline loop): account 1 forward fails
	// (switchCount 0 < cap 2 -> increment to 1), account 2 forward fails
	// (1 < 2 -> increment to 2), account 3 forward fails (switchCount 2 >= cap 2
	// -> exhausted, return). RecordOpenAIAccountSwitch is invoked on every
	// failover BEFORE the cap check, so it fires 3 times across 3 forwards.
	require.Equal(t, 3, len(upstream.requests))
	require.Equal(t, int32(3), atomic.LoadInt32(&scheduler.selectCalls))
	require.Equal(t, int32(3), atomic.LoadInt32(&scheduler.switchCalls))
}

// TestGenerateImages_SelectionExhaustedNoAccounts verifies that when the
// scheduler has no account to offer on the very first selection, GenerateImages
// returns the no-available-accounts terminal error and fires the matching hooks.
func TestGenerateImages_SelectionExhaustedNoAccounts(t *testing.T) {
	body := []byte(`{"model":"gpt-image-2","prompt":"a cat","size":"1024x1024"}`)
	parsed := parseImageGenRequest(t, body)

	scheduler := &stubOpenAIImageScheduler{accounts: nil} // Select always errors out.
	upstream := &httpUpstreamRecorder{resp: okImageResponse()}
	svc := newImageOrchestratorService(t, scheduler, upstream)

	c := newImageGenTestContext(t, body)

	var noAvailCalls int
	var markIfNoAvailCalls int

	res, err := svc.GenerateImages(context.Background(), ImageGenInput{
		OpsContext:         c,
		APIKey:             &APIKey{ID: 42},
		Parsed:             parsed,
		Body:               body,
		RequestModel:       parsed.Model,
		RequiredCapability: parsed.RequiredCapability,
		MaxAccountSwitches: 3,
		Hooks: ImageGenHooks{
			OnNoAvailableAccounts:               func() { noAvailCalls++ },
			MarkRoutingCapacityLimitedIfNoAvail: func(error) { markIfNoAvailCalls++ },
		},
	})

	require.Nil(t, res)
	require.Error(t, err)

	var terminal *imageGenTerminalError
	require.ErrorAs(t, err, &terminal)
	require.Equal(t, imageGenKindNoAvailableAccounts, terminal.Kind)

	require.Equal(t, 1, noAvailCalls)
	require.Equal(t, 1, markIfNoAvailCalls)
	require.Equal(t, 0, len(upstream.requests), "no forward should happen when selection is empty")
}

// TestGenerateImages_AccountSlotUnavailableStops verifies that when the
// AcquireAccountSlot hook reports failure (handler path: response already
// written), the loop stops with the account-slot terminal error and does not
// forward.
func TestGenerateImages_AccountSlotUnavailableStops(t *testing.T) {
	body := []byte(`{"model":"gpt-image-2","prompt":"a cat","size":"1024x1024"}`)
	parsed := parseImageGenRequest(t, body)

	account := newImageAPIKeyAccount(1)
	scheduler := &stubOpenAIImageScheduler{accounts: []*Account{account}}
	upstream := &httpUpstreamRecorder{resp: okImageResponse()}
	svc := newImageOrchestratorService(t, scheduler, upstream)

	c := newImageGenTestContext(t, body)

	res, err := svc.GenerateImages(context.Background(), ImageGenInput{
		OpsContext:         c,
		APIKey:             &APIKey{ID: 42},
		Parsed:             parsed,
		Body:               body,
		RequestModel:       parsed.Model,
		RequiredCapability: parsed.RequiredCapability,
		MaxAccountSwitches: 3,
		Hooks: ImageGenHooks{
			AcquireAccountSlot: func(*AccountSelectionResult, string) (func(), bool) {
				return nil, false
			},
		},
	})

	require.Nil(t, res)
	require.Error(t, err)
	require.True(t, errors.Is(err, ErrImageGenAccountSlotUnavailable))
	require.Equal(t, 0, len(upstream.requests), "no forward should happen when slot acquisition fails")
}
