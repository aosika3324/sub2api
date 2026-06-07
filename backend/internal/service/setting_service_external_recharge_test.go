//go:build unit

package service

import (
	"context"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/stretchr/testify/require"
)

func TestSettingService_GetPublicSettings_ExposesExternalRecharge(t *testing.T) {
	repo := &settingPublicRepoStub{
		values: map[string]string{
			SettingKeyExternalRechargeEnabled: "true",
			SettingKeyExternalRechargeURL:     " https://pay.example.com/topup ",
		},
	}
	svc := NewSettingService(repo, &config.Config{})

	settings, err := svc.GetPublicSettings(context.Background())
	require.NoError(t, err)
	require.True(t, settings.ExternalRechargeEnabled)
	// URL is trimmed when surfaced to the frontend.
	require.Equal(t, "https://pay.example.com/topup", settings.ExternalRechargeURL)
}

func TestSettingService_GetPublicSettings_ExternalRechargeDefaultsOff(t *testing.T) {
	svc := NewSettingService(&settingPublicRepoStub{values: map[string]string{}}, &config.Config{})

	settings, err := svc.GetPublicSettings(context.Background())
	require.NoError(t, err)
	require.False(t, settings.ExternalRechargeEnabled)
	require.Empty(t, settings.ExternalRechargeURL)
}

// Uses settingUpdateRepoStub (defined in setting_service_update_test.go), which
// records the persisted map via SetMultiple.
func TestSettingService_UpdateSettings_ExternalRecharge(t *testing.T) {
	t.Run("enabled with invalid url is rejected and nothing persisted", func(t *testing.T) {
		repo := &settingUpdateRepoStub{}
		svc := NewSettingService(repo, &config.Config{})
		err := svc.UpdateSettings(context.Background(), &SystemSettings{
			ExternalRechargeEnabled: true,
			ExternalRechargeURL:     "https://", // absolute scheme but no host → must be rejected
		})
		require.Error(t, err)
		require.Nil(t, repo.updates)
	})

	t.Run("enabled with valid url persists trimmed value", func(t *testing.T) {
		repo := &settingUpdateRepoStub{}
		svc := NewSettingService(repo, &config.Config{})
		err := svc.UpdateSettings(context.Background(), &SystemSettings{
			ExternalRechargeEnabled: true,
			ExternalRechargeURL:     "  https://pay.example.com/topup  ",
		})
		require.NoError(t, err)
		require.Equal(t, "true", repo.updates[SettingKeyExternalRechargeEnabled])
		require.Equal(t, "https://pay.example.com/topup", repo.updates[SettingKeyExternalRechargeURL])
	})

	t.Run("enabled with empty url is allowed (unconfigured)", func(t *testing.T) {
		repo := &settingUpdateRepoStub{}
		svc := NewSettingService(repo, &config.Config{})
		err := svc.UpdateSettings(context.Background(), &SystemSettings{
			ExternalRechargeEnabled: true,
			ExternalRechargeURL:     "   ",
		})
		require.NoError(t, err)
		require.Equal(t, "", repo.updates[SettingKeyExternalRechargeURL])
	})

	t.Run("disabled with garbage url is allowed (guard short-circuits)", func(t *testing.T) {
		repo := &settingUpdateRepoStub{}
		svc := NewSettingService(repo, &config.Config{})
		err := svc.UpdateSettings(context.Background(), &SystemSettings{
			ExternalRechargeEnabled: false,
			ExternalRechargeURL:     "not a url",
		})
		require.NoError(t, err)
		require.Equal(t, "false", repo.updates[SettingKeyExternalRechargeEnabled])
	})
}
