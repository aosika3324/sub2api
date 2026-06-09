package service

import (
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"
)

func TestParseOpenAIChatImagesRequest_ImageModelGenerations(t *testing.T) {
	svc := &OpenAIGatewayService{}
	body := []byte(`{
		"model":"gpt-image-2",
		"messages":[{"role":"user","content":"draw a cat"}],
		"n":2,
		"size":"1024x1024",
		"quality":"high",
		"response_format":"b64_json"
	}`)

	parsed, ok, err := svc.ParseOpenAIChatImagesRequest(body)

	require.NoError(t, err)
	require.True(t, ok)
	require.Equal(t, openAIImagesGenerationsEndpoint, parsed.Endpoint)
	require.Equal(t, "gpt-image-2", parsed.Model)
	require.Equal(t, "draw a cat", parsed.Prompt)
	require.Equal(t, 2, parsed.N)
	require.Equal(t, "1024x1024", parsed.Size)
	require.Equal(t, "high", parsed.Quality)
	require.Equal(t, OpenAIImagesCapabilityNative, parsed.RequiredCapability)
	require.JSONEq(t, `{
		"model":"gpt-image-2",
		"prompt":"draw a cat",
		"n":2,
		"size":"1024x1024",
		"quality":"high",
		"response_format":"b64_json"
	}`, string(parsed.Body))
}

func TestParseOpenAIChatImagesRequest_ImageURLBecomesEdits(t *testing.T) {
	svc := &OpenAIGatewayService{}
	body := []byte(`{
		"model":"gpt-image-2",
		"messages":[{
			"role":"user",
			"content":[
				{"type":"text","text":"turn this into watercolor"},
				{"type":"image_url","image_url":{"url":"data:image/png;base64,AAAA"}}
			]
		}]
	}`)

	parsed, ok, err := svc.ParseOpenAIChatImagesRequest(body)

	require.NoError(t, err)
	require.True(t, ok)
	require.Equal(t, openAIImagesEditsEndpoint, parsed.Endpoint)
	require.Equal(t, "turn this into watercolor", parsed.Prompt)
	require.Equal(t, []string{"data:image/png;base64,AAAA"}, parsed.InputImageURLs)
	require.Equal(t, "data:image/png;base64,AAAA", gjson.GetBytes(parsed.Body, "images.0.image_url").String())
}

func TestParseOpenAIChatImagesRequest_TopLevelImagesBecomeEdits(t *testing.T) {
	svc := &OpenAIGatewayService{}
	body := []byte(`{
		"model":"gpt-image-2",
		"messages":[{"role":"user","content":"change the background"}],
		"images":[{"image_url":"https://example.com/source.png"}]
	}`)

	parsed, ok, err := svc.ParseOpenAIChatImagesRequest(body)

	require.NoError(t, err)
	require.True(t, ok)
	require.Equal(t, openAIImagesEditsEndpoint, parsed.Endpoint)
	require.Equal(t, []string{"https://example.com/source.png"}, parsed.InputImageURLs)
	require.Equal(t, "https://example.com/source.png", gjson.GetBytes(parsed.Body, "images.0.image_url").String())
}

func TestParseOpenAIChatImagesRequest_ImageGenerationToolDefaultsModel(t *testing.T) {
	svc := &OpenAIGatewayService{}
	body := []byte(`{
		"model":"gpt-5",
		"messages":[{"role":"user","content":"draw a city skyline"}],
		"tools":[{"type":"image_generation"}],
		"tool_choice":{"type":"image_generation"}
	}`)

	parsed, ok, err := svc.ParseOpenAIChatImagesRequest(body)

	require.NoError(t, err)
	require.True(t, ok)
	require.Equal(t, openAIImagesGenerationsEndpoint, parsed.Endpoint)
	require.Equal(t, "gpt-image-2", parsed.Model)
	require.Equal(t, "draw a city skyline", parsed.Prompt)
}

func TestParseOpenAIChatImagesRequest_ImageGenerationToolOptions(t *testing.T) {
	svc := &OpenAIGatewayService{}
	body := []byte(`{
		"model":"gpt-5",
		"messages":[{"role":"user","content":"draw a city skyline"}],
		"tools":[{
			"type":"image_generation",
			"model":"gpt-image-1.5",
			"n":3,
			"size":"2048x1152",
			"quality":"medium",
			"output_format":"webp",
			"partial_images":2
		}],
		"tool_choice":{"type":"image_generation"}
	}`)

	parsed, ok, err := svc.ParseOpenAIChatImagesRequest(body)

	require.NoError(t, err)
	require.True(t, ok)
	require.Equal(t, openAIImagesGenerationsEndpoint, parsed.Endpoint)
	require.Equal(t, "gpt-image-1.5", parsed.Model)
	require.Equal(t, 3, parsed.N)
	require.Equal(t, "2048x1152", parsed.Size)
	require.Equal(t, "medium", parsed.Quality)
	require.Equal(t, "webp", parsed.OutputFormat)
	require.NotNil(t, parsed.PartialImages)
	require.Equal(t, 2, *parsed.PartialImages)
	require.Equal(t, "gpt-image-1.5", gjson.GetBytes(parsed.Body, "model").String())
	require.Equal(t, int64(3), gjson.GetBytes(parsed.Body, "n").Int())
	require.Equal(t, "2048x1152", gjson.GetBytes(parsed.Body, "size").String())
	require.Equal(t, "webp", gjson.GetBytes(parsed.Body, "output_format").String())
}

func TestParseOpenAIChatImagesRequest_TopLevelOptionsOverrideToolOptions(t *testing.T) {
	svc := &OpenAIGatewayService{}
	body := []byte(`{
		"model":"gpt-5",
		"messages":[{"role":"user","content":"draw a city skyline"}],
		"n":1,
		"size":"1024x1024",
		"quality":"high",
		"tools":[{
			"type":"image_generation",
			"model":"gpt-image-2",
			"n":4,
			"size":"2048x1152",
			"quality":"low"
		}],
		"tool_choice":{"type":"image_generation"}
	}`)

	parsed, ok, err := svc.ParseOpenAIChatImagesRequest(body)

	require.NoError(t, err)
	require.True(t, ok)
	require.Equal(t, 1, parsed.N)
	require.Equal(t, "1024x1024", parsed.Size)
	require.Equal(t, "high", parsed.Quality)
	require.Equal(t, int64(1), gjson.GetBytes(parsed.Body, "n").Int())
	require.Equal(t, "1024x1024", gjson.GetBytes(parsed.Body, "size").String())
	require.Equal(t, "high", gjson.GetBytes(parsed.Body, "quality").String())
}

func TestParseOpenAIChatImagesRequest_CodexAliasPreserved(t *testing.T) {
	svc := &OpenAIGatewayService{}
	body := []byte(`{
		"model":"codex-gpt-image-2",
		"messages":[{"role":"user","content":"draw a cat"}]
	}`)

	parsed, ok, err := svc.ParseOpenAIChatImagesRequest(body)

	require.NoError(t, err)
	require.True(t, ok)
	require.Equal(t, "codex-gpt-image-2", parsed.Model)
	require.Equal(t, "codex-gpt-image-2", gjson.GetBytes(parsed.Body, "model").String())
}

func TestParseOpenAIChatImagesRequest_OrdinaryChatIgnored(t *testing.T) {
	svc := &OpenAIGatewayService{}
	body := []byte(`{
		"model":"gpt-5",
		"messages":[{"role":"user","content":"hello"}]
	}`)

	parsed, ok, err := svc.ParseOpenAIChatImagesRequest(body)

	require.NoError(t, err)
	require.False(t, ok)
	require.Nil(t, parsed)
}

func TestParseOpenAIChatImagesRequest_ModalitiesImageIgnoredWithoutImageIntent(t *testing.T) {
	svc := &OpenAIGatewayService{}
	body := []byte(`{
		"model":"gpt-5",
		"modalities":["image"],
		"messages":[{"role":"user","content":"hello"}]
	}`)

	parsed, ok, err := svc.ParseOpenAIChatImagesRequest(body)

	require.NoError(t, err)
	require.False(t, ok)
	require.Nil(t, parsed)
}
