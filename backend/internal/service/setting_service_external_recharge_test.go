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

func TestHasHTTPScheme(t *testing.T) {
	cases := map[string]bool{
		"http://a.com":        true,
		"https://a.com/x?y=1": true,
		"ftp://a.com":         false,
		"a.com":               false,
		"javascript:alert(1)": false,
		"":                    false,
	}
	for in, want := range cases {
		require.Equalf(t, want, hasHTTPScheme(in), "hasHTTPScheme(%q)", in)
	}
}
