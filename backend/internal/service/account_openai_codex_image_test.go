package service

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestAccountSupportsCodexImageGenerationRequiresPaidChatGPTPlan(t *testing.T) {
	tests := []struct {
		name        string
		credentials map[string]any
		want        bool
	}{
		{
			name:        "plus",
			credentials: map[string]any{"plan_type": "plus"},
			want:        true,
		},
		{
			name:        "team from chatgpt plan type",
			credentials: map[string]any{"chatgpt_plan_type": "team"},
			want:        true,
		},
		{
			name:        "pro display name",
			credentials: map[string]any{"plan_type": "chatgpt_pro"},
			want:        true,
		},
		{
			name:        "free",
			credentials: map[string]any{"plan_type": "free"},
			want:        false,
		},
		{
			name:        "missing",
			credentials: nil,
			want:        false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			account := &Account{
				Platform:    PlatformOpenAI,
				Type:        AccountTypeOAuth,
				Credentials: tt.credentials,
			}
			require.Equal(t, tt.want, account.SupportsCodexImageGeneration())
		})
	}
}
