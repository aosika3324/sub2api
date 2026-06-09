package service

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/tidwall/gjson"
)

// ParseOpenAIChatImagesRequest converts image-scene Chat Completions requests
// into the internal Images API shape used by /v1/images/generations and
// /v1/images/edits. It returns ok=false for ordinary chat requests.
func (s *OpenAIGatewayService) ParseOpenAIChatImagesRequest(body []byte) (*OpenAIImagesRequest, bool, error) {
	if len(body) == 0 || !gjson.ValidBytes(body) {
		return nil, false, nil
	}

	model := strings.TrimSpace(gjson.GetBytes(body, "model").String())
	if !isOpenAIChatImagesIntent(body, model) {
		return nil, false, nil
	}

	stream := false
	if streamResult := gjson.GetBytes(body, "stream"); streamResult.Exists() {
		if streamResult.Type != gjson.True && streamResult.Type != gjson.False {
			return nil, true, fmt.Errorf("invalid stream field type")
		}
		stream = streamResult.Bool()
	}

	n := 1
	if nResult := gjson.GetBytes(body, "n"); nResult.Exists() {
		if nResult.Type != gjson.Number {
			return nil, true, fmt.Errorf("invalid n field type")
		}
		n = int(nResult.Int())
		if n <= 0 {
			return nil, true, fmt.Errorf("n must be greater than 0")
		}
	}

	explicitImageModel := isOpenAIImageGenerationModel(model)
	if !explicitImageModel {
		model = firstOpenAIChatImagesToolModel(body)
		explicitImageModel = strings.TrimSpace(model) != ""
	}
	if strings.TrimSpace(model) == "" {
		model = "gpt-image-2"
	}

	prompt, imageURLs := extractOpenAIChatImagesPromptAndURLs(body)
	if strings.TrimSpace(prompt) == "" {
		return nil, true, fmt.Errorf("prompt is required")
	}

	endpoint := openAIImagesGenerationsEndpoint
	if len(imageURLs) > 0 {
		endpoint = openAIImagesEditsEndpoint
	}

	req := &OpenAIImagesRequest{
		Endpoint:       endpoint,
		ContentType:    "application/json",
		Model:          model,
		ExplicitModel:  explicitImageModel,
		Prompt:         prompt,
		Stream:         stream,
		N:              n,
		Size:           strings.TrimSpace(gjson.GetBytes(body, "size").String()),
		ResponseFormat: strings.ToLower(strings.TrimSpace(gjson.GetBytes(body, "response_format").String())),
		Quality:        strings.TrimSpace(gjson.GetBytes(body, "quality").String()),
		Background:     strings.TrimSpace(gjson.GetBytes(body, "background").String()),
		OutputFormat:   strings.TrimSpace(gjson.GetBytes(body, "output_format").String()),
		Moderation:     strings.TrimSpace(gjson.GetBytes(body, "moderation").String()),
		InputFidelity:  strings.TrimSpace(gjson.GetBytes(body, "input_fidelity").String()),
		Style:          strings.TrimSpace(gjson.GetBytes(body, "style").String()),
		InputImageURLs: imageURLs,
	}
	req.ExplicitSize = req.Size != ""

	if outputCompression := gjson.GetBytes(body, "output_compression"); outputCompression.Exists() {
		if outputCompression.Type != gjson.Number {
			return nil, true, fmt.Errorf("invalid output_compression field type")
		}
		v := int(outputCompression.Int())
		req.OutputCompression = &v
	}
	if partialImages := gjson.GetBytes(body, "partial_images"); partialImages.Exists() {
		if partialImages.Type != gjson.Number {
			return nil, true, fmt.Errorf("invalid partial_images field type")
		}
		v := int(partialImages.Int())
		req.PartialImages = &v
	}
	if maskImageURL := strings.TrimSpace(gjson.GetBytes(body, "mask.image_url").String()); maskImageURL != "" {
		req.MaskImageURL = maskImageURL
		req.HasMask = true
	}
	req.HasNativeOptions = hasOpenAINativeImageOptions(func(path string) bool {
		return gjson.GetBytes(body, path).Exists()
	})

	applyOpenAIImagesDefaults(req)
	if err := validateOpenAIImagesModel(req.Model); err != nil {
		return nil, true, err
	}
	req.SizeTier = normalizeOpenAIImageSizeTier(req.Size)
	req.RequiredCapability = classifyOpenAIImagesCapability(req)
	req.Body = buildOpenAIImagesJSONBody(req)
	if len(req.Body) > 0 {
		sum := sha256.Sum256(req.Body)
		req.bodyHash = hex.EncodeToString(sum[:8])
	}
	return req, true, nil
}

func isOpenAIChatImagesIntent(body []byte, model string) bool {
	if isOpenAIImageGenerationModel(model) {
		return true
	}
	return openAIChatHasImageGenerationTool(body)
}

func openAIChatHasImageGenerationTool(body []byte) bool {
	tools := gjson.GetBytes(body, "tools")
	if tools.IsArray() {
		for _, item := range tools.Array() {
			if strings.EqualFold(strings.TrimSpace(item.Get("type").String()), "image_generation") {
				return true
			}
		}
	}
	toolChoice := gjson.GetBytes(body, "tool_choice")
	if toolChoice.IsObject() && strings.EqualFold(strings.TrimSpace(toolChoice.Get("type").String()), "image_generation") {
		return true
	}
	if toolChoice.Type == gjson.String && strings.EqualFold(strings.TrimSpace(toolChoice.String()), "image_generation") {
		return true
	}
	return false
}

func firstOpenAIChatImagesToolModel(body []byte) string {
	tools := gjson.GetBytes(body, "tools")
	if !tools.IsArray() {
		return ""
	}
	for _, item := range tools.Array() {
		if !strings.EqualFold(strings.TrimSpace(item.Get("type").String()), "image_generation") {
			continue
		}
		model := strings.TrimSpace(item.Get("model").String())
		if isOpenAIImageGenerationModel(model) {
			return model
		}
	}
	return ""
}

func extractOpenAIChatImagesPromptAndURLs(body []byte) (string, []string) {
	directPrompt := strings.TrimSpace(gjson.GetBytes(body, "prompt").String())
	var promptParts []string
	if directPrompt != "" {
		promptParts = append(promptParts, directPrompt)
	}
	var imageURLs []string
	imageURLs = append(imageURLs, extractOpenAIChatImagesTopLevelURLs(body)...)

	messages := gjson.GetBytes(body, "messages")
	if messages.IsArray() {
		for _, message := range messages.Array() {
			if role := strings.TrimSpace(message.Get("role").String()); role != "" && !strings.EqualFold(role, "user") {
				continue
			}
			text, urls := extractOpenAIChatImagesContent(message.Get("content"))
			if text != "" {
				promptParts = append(promptParts, text)
			}
			imageURLs = append(imageURLs, urls...)
		}
	}

	prompt := strings.TrimSpace(strings.Join(promptParts, "\n"))
	return prompt, dedupeTrimmedStrings(imageURLs)
}

func extractOpenAIChatImagesTopLevelURLs(body []byte) []string {
	var imageURLs []string
	for _, path := range []string{"image", "images", "input_image"} {
		value := gjson.GetBytes(body, path)
		if !value.Exists() {
			continue
		}
		if value.IsArray() {
			for _, item := range value.Array() {
				imageURLs = append(imageURLs, extractOpenAIChatImageURL(item)...)
			}
			continue
		}
		imageURLs = append(imageURLs, extractOpenAIChatImageURL(value)...)
	}
	return imageURLs
}

func extractOpenAIChatImagesContent(content gjson.Result) (string, []string) {
	switch {
	case content.Type == gjson.String:
		return strings.TrimSpace(content.String()), nil
	case content.IsArray():
		var promptParts []string
		var imageURLs []string
		for _, part := range content.Array() {
			partType := strings.ToLower(strings.TrimSpace(part.Get("type").String()))
			switch partType {
			case "text", "input_text":
				if text := strings.TrimSpace(part.Get("text").String()); text != "" {
					promptParts = append(promptParts, text)
				}
			case "image_url":
				imageURLs = append(imageURLs, extractOpenAIChatImageURL(part.Get("image_url"))...)
			case "input_image":
				imageURLs = append(imageURLs, extractOpenAIChatImageURL(part)...)
			case "image":
				imageURLs = append(imageURLs, extractOpenAIChatImageURL(part)...)
			}
		}
		return strings.TrimSpace(strings.Join(promptParts, "\n")), imageURLs
	default:
		return "", nil
	}
}

func extractOpenAIChatImageURL(value gjson.Result) []string {
	var out []string
	if value.Type == gjson.String {
		if url := strings.TrimSpace(value.String()); url != "" {
			out = append(out, url)
		}
	}
	for _, path := range []string{"url", "image_url", "image_url.url"} {
		candidate := value.Get(path)
		if candidate.Type != gjson.String {
			continue
		}
		if url := strings.TrimSpace(candidate.String()); url != "" {
			out = append(out, url)
		}
	}
	return out
}

func dedupeTrimmedStrings(values []string) []string {
	if len(values) == 0 {
		return nil
	}
	seen := make(map[string]struct{}, len(values))
	out := make([]string, 0, len(values))
	for _, value := range values {
		trimmed := strings.TrimSpace(value)
		if trimmed == "" {
			continue
		}
		if _, ok := seen[trimmed]; ok {
			continue
		}
		seen[trimmed] = struct{}{}
		out = append(out, trimmed)
	}
	return out
}

func buildOpenAIImagesJSONBody(req *OpenAIImagesRequest) []byte {
	if req == nil {
		return nil
	}
	payload := map[string]any{
		"model":  req.Model,
		"prompt": req.Prompt,
		"n":      req.N,
	}
	if strings.TrimSpace(req.Size) != "" {
		payload["size"] = req.Size
	}
	if strings.TrimSpace(req.Quality) != "" {
		payload["quality"] = req.Quality
	}
	if strings.TrimSpace(req.ResponseFormat) != "" {
		payload["response_format"] = req.ResponseFormat
	}
	if req.IsEdits() && len(req.InputImageURLs) > 0 {
		images := make([]map[string]string, 0, len(req.InputImageURLs))
		for _, imageURL := range req.InputImageURLs {
			if trimmed := strings.TrimSpace(imageURL); trimmed != "" {
				images = append(images, map[string]string{"image_url": trimmed})
			}
		}
		payload["images"] = images
	}
	if maskImageURL := strings.TrimSpace(req.MaskImageURL); maskImageURL != "" {
		payload["mask"] = map[string]string{"image_url": maskImageURL}
	}
	for _, field := range []struct {
		key   string
		value string
	}{
		{key: "background", value: req.Background},
		{key: "output_format", value: req.OutputFormat},
		{key: "moderation", value: req.Moderation},
		{key: "input_fidelity", value: req.InputFidelity},
		{key: "style", value: req.Style},
	} {
		if trimmed := strings.TrimSpace(field.value); trimmed != "" {
			payload[field.key] = trimmed
		}
	}
	if req.OutputCompression != nil {
		payload["output_compression"] = *req.OutputCompression
	}
	if req.PartialImages != nil {
		payload["partial_images"] = *req.PartialImages
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return nil
	}
	return body
}
