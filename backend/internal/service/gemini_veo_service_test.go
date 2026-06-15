//go:build unit

package service

import (
	"testing"

	"github.com/stretchr/testify/require"
)

// TestCalculateVeoVideoCost_BySecond 锁定 Veo 按秒计费：总价 = 单价 × 秒数 × 倍率，
// 且负数倍率被钳制为 0（与其他计费路径一致）。
func TestCalculateVeoVideoCost_BySecond(t *testing.T) {
	svc := newTestBillingService()
	pricePerSecond := 0.5

	tests := []struct {
		name       string
		price      *float64
		duration   float64
		multiplier float64
		wantTotal  float64
		wantActual float64
	}{
		{"8s @ 0.5/s, 1x", &pricePerSecond, 8, 1.0, 4.0, 4.0},
		{"8s @ 0.5/s, 2x", &pricePerSecond, 8, 2.0, 4.0, 8.0},
		{"negative multiplier clamped to 0", &pricePerSecond, 8, -1.0, 4.0, 0.0},
		{"nil price -> 0", nil, 8, 1.0, 0.0, 0.0},
		{"negative duration clamped to 0", &pricePerSecond, -5, 1.0, 0.0, 0.0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cost := svc.CalculateVeoVideoCost(tt.price, tt.duration, tt.multiplier)
			require.NotNil(t, cost)
			require.InDelta(t, tt.wantTotal, cost.TotalCost, 1e-9)
			require.InDelta(t, tt.wantActual, cost.ActualCost, 1e-9)
			require.Equal(t, string(BillingModePerRequest), cost.BillingMode)
		})
	}
}

// TestIsVeoModel 锁定 Veo 模型识别（大小写/空白不敏感）。
func TestIsVeoModel(t *testing.T) {
	tests := []struct {
		model string
		want  bool
	}{
		{"veo-3.0-generate-001", true},
		{"  Veo-3.1  ", true},
		{"VEO", true},
		{"gemini-2.5-pro", false},
		{"", false},
		{"sora-2", false},
	}
	for _, tt := range tests {
		require.Equal(t, tt.want, IsVeoModel(tt.model), tt.model)
	}
}

// TestIsVeoAction 锁定仅 predictLongRunning 被识别为 Veo 提交 action。
func TestIsVeoAction(t *testing.T) {
	require.True(t, IsVeoAction("predictLongRunning"))
	require.True(t, IsVeoAction("  predictLongRunning  "))
	require.False(t, IsVeoAction("generateContent"))
	require.False(t, IsVeoAction("streamGenerateContent"))
	require.False(t, IsVeoAction(""))
}

// TestExtractVeoDurationSeconds 验证从多种响应形态解析视频时长，解析失败时回退默认值。
func TestExtractVeoDurationSeconds(t *testing.T) {
	tests := []struct {
		name string
		body string
		want float64
	}{
		{
			name: "generatedSamples durationSeconds number",
			body: `{"response":{"generateVideoResponse":{"generatedSamples":[{"video":{"durationSeconds":8}}]}}}`,
			want: 8,
		},
		{
			name: "generatedSamples duration string 8s",
			body: `{"response":{"generateVideoResponse":{"generatedSamples":[{"video":{"duration":"8s"}}]}}}`,
			want: 8,
		},
		{
			name: "multiple samples summed",
			body: `{"response":{"generateVideoResponse":{"generatedSamples":[{"video":{"durationSeconds":8}},{"video":{"durationSeconds":4}}]}}}`,
			want: 12,
		},
		{
			name: "videos fallback path",
			body: `{"response":{"videos":[{"durationSeconds":6}]}}`,
			want: 6,
		},
		{
			name: "no duration -> default fallback",
			body: `{"response":{"generateVideoResponse":{"generatedSamples":[{"video":{}}]}}}`,
			want: veoDefaultVideoDurationSeconds,
		},
		{
			name: "empty body -> default fallback",
			body: `{}`,
			want: veoDefaultVideoDurationSeconds,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require.InDelta(t, tt.want, extractVeoDurationSeconds([]byte(tt.body)), 1e-9)
		})
	}
}
