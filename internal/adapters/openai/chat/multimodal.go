// Copyright (C) 2026 Joey Kot <joey.kot.x@gmail.com>
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed WITHOUT ANY WARRANTY; without even the
// implied warranty of MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.
// See <https://www.gnu.org/licenses/> for more details.

package chat

import (
	"strings"

	"glm-compatible/internal/adapters/openai/shared"
)

func SupportsGLMMultimodalModel(model string) bool {
	m := strings.ToLower(model)
	for _, marker := range []string{
		"glm-5v",
		"glm-4.6v",
		"glm-4.5v",
		"autoglm-phone-multilingual",
	} {
		if strings.Contains(m, marker) {
			return true
		}
	}
	return false
}

func MessagesForGLMModel(messages []shared.Map, model string) []shared.Map {
	supportsMultimodal := SupportsGLMMultimodalModel(model)
	out := make([]shared.Map, 0, len(messages))
	for _, message := range messages {
		normalized := shared.CloneMap(message)
		role := shared.StringValue(normalized["role"])
		normalized["content"] = ContentForGLMModel(normalized["content"], role, supportsMultimodal)
		out = append(out, normalized)
	}
	return out
}

func ContentForGLMModel(content any, role string, supportsMultimodal bool) any {
	if role != "user" {
		return shared.ContentToText(content, role == "assistant")
	}
	parts := contentParts(content)
	if len(parts) == 0 {
		return shared.ContentToText(content, false)
	}
	if supportsMultimodal {
		glmParts := []any{}
		for _, part := range parts {
			glmParts = append(glmParts, glmContentPart(part)...)
		}
		if len(glmParts) > 0 {
			return glmParts
		}
	}
	return textContentWithUnsupportedParts(parts)
}

func contentParts(content any) []shared.Map {
	switch c := content.(type) {
	case []any:
		parts := make([]shared.Map, 0, len(c))
		for _, raw := range c {
			if part, ok := raw.(map[string]any); ok {
				parts = append(parts, part)
			} else if text := shared.StringValue(raw); text != "" {
				parts = append(parts, shared.Map{"type": "text", "text": text})
			}
		}
		return parts
	case []shared.Map:
		return c
	default:
		return nil
	}
}

func glmContentPart(part shared.Map) []any {
	switch shared.StringValue(part["type"]) {
	case "input_text", "text":
		if text := shared.StringValue(part["text"]); text != "" {
			return []any{shared.Map{"type": "text", "text": text}}
		}
	case "input_image", "image_url":
		if url := partURL(part, "image_url"); url != "" {
			return []any{shared.Map{"type": "image_url", "image_url": shared.Map{"url": url}}}
		}
	case "input_video", "video_url":
		if url := partURL(part, "video_url"); url != "" {
			return []any{shared.Map{"type": "video_url", "video_url": shared.Map{"url": url}}}
		}
	case "input_file", "file_url":
		if url := partURL(part, "file_url"); url != "" {
			return []any{shared.Map{"type": "file_url", "file_url": shared.Map{"url": url}}}
		}
	}
	return nil
}

func partURL(part shared.Map, key string) string {
	switch v := part[key].(type) {
	case string:
		return v
	case map[string]any:
		return shared.StringValue(v["url"])
	}
	return ""
}

func textContentWithUnsupportedParts(parts []shared.Map) string {
	lines := []string{}
	for _, part := range parts {
		if text := shared.ContentPartToText(part, false); text != "" {
			lines = append(lines, text)
			continue
		}
		switch shared.StringValue(part["type"]) {
		case "input_image", "image_url":
			lines = append(lines, "[Image input omitted: selected GLM model does not support multimodal input.]")
		case "input_video", "video_url":
			lines = append(lines, "[Video input omitted: selected GLM model does not support multimodal input.]")
		case "input_file", "file_url":
			lines = append(lines, "[File input omitted: selected GLM model does not support multimodal input.]")
		default:
			lines = append(lines, "[Unsupported input part omitted.]")
		}
	}
	return strings.Join(lines, "\n")
}
