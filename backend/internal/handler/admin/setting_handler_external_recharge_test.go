package admin

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

// TestSettingsPUT_ExternalRecharge_Persists is a regression guard for the fork's
// external-recharge entry (feat ea781a068). The UpdateSettingsRequest DTO field
// and its mapping into SystemSettings were silently dropped during upstream's
// setting_handler.go split refactor (bb5d2e84a), so the admin toggle never saved.
// This test fails loudly if either the DTO binding or the handler mapping is lost
// again on a future upstream merge.
func TestSettingsPUT_ExternalRecharge_Persists(t *testing.T) {
	gin.SetMode(gin.TestMode)
	repo := &settingHandlerRepoStub{values: map[string]string{}}
	svc := service.NewSettingService(repo, &config.Config{Default: config.DefaultConfig{UserConcurrency: 5}})
	handler := NewSettingHandler(svc, nil, nil, nil, nil, nil, nil)

	body := map[string]any{
		"external_recharge_enabled": true,
		"external_recharge_url":     "https://recharge.example.com",
	}
	rawBody, err := json.Marshal(body)
	require.NoError(t, err)

	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodPut, "/api/v1/admin/settings", bytes.NewReader(rawBody))
	c.Request.Header.Set("Content-Type", "application/json")

	handler.UpdateSettings(c)

	require.Equal(t, http.StatusOK, rec.Code, "update should succeed")
	require.Equal(t, "true", repo.lastUpdates[service.SettingKeyExternalRechargeEnabled],
		"external_recharge_enabled must be persisted from the request")
	require.Equal(t, "https://recharge.example.com", repo.lastUpdates[service.SettingKeyExternalRechargeURL],
		"external_recharge_url must be persisted from the request")
}
