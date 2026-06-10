package service

import (
	"encoding/json"
	"strings"
	"testing"
)

func mustParseClaudeMaxRequest(t *testing.T, model string, system any, messages []any, topLevelCache bool) *ParsedRequest {
	t.Helper()
	body := map[string]any{
		"model":    model,
		"messages": messages,
	}
	if system != nil {
		body["system"] = system
	}
	if topLevelCache {
		body["cache_control"] = map[string]any{"type": "ephemeral"}
	}

	raw, err := json.Marshal(body)
	if err != nil {
		t.Fatalf("marshal request: %v", err)
	}
	parsed, err := ParseGatewayRequest(NewRequestBodyRef(raw), "")
	if err != nil {
		t.Fatalf("ParseGatewayRequest failed: %v", err)
	}
	return parsed
}

func TestProjectUsageToClaudeMax1H_Conservation(t *testing.T) {
	usage := &ClaudeUsage{
		InputTokens:              1200,
		CacheCreationInputTokens: 0,
		CacheCreation5mTokens:    0,
		CacheCreation1hTokens:    0,
	}
	parsed := mustParseClaudeMaxRequest(t, "claude-sonnet-4-5", nil, []any{
		map[string]any{
			"role": "user",
			"content": []any{
				map[string]any{
					"type":          "text",
					"text":          strings.Repeat("cached context ", 200),
					"cache_control": map[string]any{"type": "ephemeral"},
				},
				map[string]any{
					"type": "text",
					"text": "summarize quickly",
				},
			},
		},
	}, false)

	changed := projectUsageToClaudeMax1H(usage, parsed)
	if !changed {
		t.Fatalf("expected usage to be projected")
	}

	total := usage.InputTokens + usage.CacheCreation5mTokens + usage.CacheCreation1hTokens
	if total != 1200 {
		t.Fatalf("total tokens changed: got=%d want=%d", total, 1200)
	}
	if usage.CacheCreation5mTokens != 0 {
		t.Fatalf("cache_creation_5m should be 0, got=%d", usage.CacheCreation5mTokens)
	}
	if usage.InputTokens <= 0 || usage.InputTokens >= 1200 {
		t.Fatalf("simulated input out of range, got=%d", usage.InputTokens)
	}
	if usage.InputTokens > 100 {
		t.Fatalf("simulated input should stay near cache breakpoint tail, got=%d", usage.InputTokens)
	}
	if usage.CacheCreation1hTokens <= 0 {
		t.Fatalf("cache_creation_1h should be > 0, got=%d", usage.CacheCreation1hTokens)
	}
	if usage.CacheCreationInputTokens != usage.CacheCreation1hTokens {
		t.Fatalf("cache_creation_input_tokens mismatch: got=%d want=%d", usage.CacheCreationInputTokens, usage.CacheCreation1hTokens)
	}
}

func TestComputeClaudeMaxProjectedInputTokens_Deterministic(t *testing.T) {
	parsed := mustParseClaudeMaxRequest(t, "claude-opus-4-5", nil, []any{
		map[string]any{
			"role": "user",
			"content": []any{
				map[string]any{
					"type":          "text",
					"text":          "build context",
					"cache_control": map[string]any{"type": "ephemeral"},
				},
				map[string]any{
					"type": "text",
					"text": "what is failing now",
				},
			},
		},
	}, false)

	got1 := computeClaudeMaxProjectedInputTokens(4096, parsed)
	got2 := computeClaudeMaxProjectedInputTokens(4096, parsed)
	if got1 != got2 {
		t.Fatalf("non-deterministic input tokens: %d != %d", got1, got2)
	}
}

func TestShouldSimulateClaudeMaxUsage(t *testing.T) {
	group := &Group{
		Platform:                 PlatformAnthropic,
		SimulateClaudeMaxEnabled: true,
	}
	input := &RecordUsageInput{
		Result: &ForwardResult{
			Model: "claude-sonnet-4-5",
			Usage: ClaudeUsage{
				InputTokens:              3000,
				CacheCreationInputTokens: 0,
				CacheCreation5mTokens:    0,
				CacheCreation1hTokens:    0,
			},
		},
		ParsedRequest: mustParseClaudeMaxRequest(t, "claude-sonnet-4-5", nil, []any{
			map[string]any{
				"role": "user",
				"content": []any{
					map[string]any{
						"type":          "text",
						"text":          "cached",
						"cache_control": map[string]any{"type": "ephemeral"},
					},
					map[string]any{
						"type": "text",
						"text": "tail",
					},
				},
			},
		}, false),
		APIKey: &APIKey{Group: group},
	}

	if !shouldSimulateClaudeMaxUsage(input) {
		t.Fatalf("expected simulate=true for claude group with cache signal")
	}

	input.ParsedRequest = mustParseClaudeMaxRequest(t, "claude-sonnet-4-5", nil, []any{
		map[string]any{"role": "user", "content": "no cache signal"},
	}, false)
	if shouldSimulateClaudeMaxUsage(input) {
		t.Fatalf("expected simulate=false when request has no cache signal")
	}

	input.ParsedRequest = mustParseClaudeMaxRequest(t, "claude-sonnet-4-5", nil, []any{
		map[string]any{
			"role": "user",
			"content": []any{
				map[string]any{
					"type":          "text",
					"text":          "cached",
					"cache_control": map[string]any{"type": "ephemeral"},
				},
			},
		},
	}, false)
	input.Result.Usage.CacheCreationInputTokens = 100
	if shouldSimulateClaudeMaxUsage(input) {
		t.Fatalf("expected simulate=false when cache creation already exists")
	}
}
