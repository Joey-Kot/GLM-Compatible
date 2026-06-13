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

package generate

import "testing"

func TestBuildUpstreamPayloadMapsGeminiGenerateContent(t *testing.T) {
	adapter := Adapter{DefaultModel: "glm-5.1"}
	payload := map[string]any{
		"systemInstruction": map[string]any{"parts": []any{map[string]any{"text": "Be brief."}}},
		"contents": []any{
			map[string]any{"role": "user", "parts": []any{map[string]any{"text": "Weather?"}}},
			map[string]any{"role": "model", "parts": []any{map[string]any{"functionCall": map[string]any{"id": "call_1", "name": "get_weather", "args": map[string]any{"city": "Hangzhou"}}}}},
			map[string]any{"role": "user", "parts": []any{map[string]any{"functionResponse": map[string]any{"id": "call_1", "name": "get_weather", "response": map[string]any{"temp": "24C"}}}}},
		},
		"generationConfig": map[string]any{"maxOutputTokens": 32, "responseMimeType": "application/json", "responseSchema": map[string]any{"type": "object"}},
		"tools":            []any{map[string]any{"functionDeclarations": []any{map[string]any{"name": "get_weather", "parameters": map[string]any{"type": "object"}}}}},
		"toolConfig":       map[string]any{"functionCallingConfig": map[string]any{"mode": "ANY", "allowedFunctionNames": []any{"get_weather"}}},
	}
	prepared, err := adapter.BuildUpstreamPayload("gemini-3.5-flash", payload)
	if err != nil {
		t.Fatal(err)
	}
	messages := prepared.ChatPayload["messages"].([]map[string]any)
	if messages[0]["role"] != "system" {
		t.Fatalf("messages = %#v", messages)
	}
	if messages[2]["role"] != "user" || messages[2]["content"] != "Weather?" {
		t.Fatalf("user message = %#v", messages[2])
	}
	if messages[3]["role"] != "assistant" {
		t.Fatalf("function call message = %#v", messages[3])
	}
	if messages[4]["role"] != "tool" {
		t.Fatalf("function response = %#v", messages[4])
	}
	if prepared.ChatPayload["response_format"].(map[string]any)["type"] != "json_object" {
		t.Fatalf("response_format = %#v", prepared.ChatPayload["response_format"])
	}
	if prepared.ChatPayload["tool_choice"].(map[string]any)["function"].(map[string]any)["name"] != "get_weather" {
		t.Fatalf("tool_choice = %#v", prepared.ChatPayload["tool_choice"])
	}
}

func TestResponseFromUpstreamMapsGeminiParts(t *testing.T) {
	completion := map[string]any{
		"id": "resp_1",
		"choices": []any{map[string]any{
			"finish_reason": "tool_calls",
			"message": map[string]any{
				"content":           "Checking.",
				"reasoning_content": "Need tool.",
				"tool_calls":        []any{map[string]any{"id": "call_1", "function": map[string]any{"name": "get_weather", "arguments": "{\"city\":\"Hangzhou\"}"}}},
			},
		}},
		"usage": map[string]any{"prompt_tokens": 2, "completion_tokens": 3, "total_tokens": 5},
	}
	response := ResponseFromUpstream(completion, "gemini-3.5-flash")
	candidates := response["candidates"].([]any)
	parts := candidates[0].(map[string]any)["content"].(map[string]any)["parts"].([]any)
	if parts[0].(map[string]any)["thought"] != true {
		t.Fatalf("thought part = %#v", parts[0])
	}
	if _, ok := parts[2].(map[string]any)["functionCall"]; !ok {
		t.Fatalf("functionCall part = %#v", parts[2])
	}
}

func TestGeminiMultimodalInputMapsToGLMVisionContent(t *testing.T) {
	adapter := Adapter{DefaultModel: "glm-5.1"}
	payload := map[string]any{
		"contents": []any{map[string]any{
			"role": "user",
			"parts": []any{
				map[string]any{"text": "describe"},
				map[string]any{"inlineData": map[string]any{"mimeType": "image/png", "data": "abc"}},
			},
		}},
	}

	prepared, err := adapter.BuildUpstreamPayload("glm-4.6v-flash", payload)
	if err != nil {
		t.Fatal(err)
	}
	messages := prepared.ChatPayload["messages"].([]map[string]any)
	parts := messages[0]["content"].([]any)
	if got := parts[1].(map[string]any)["image_url"].(map[string]any)["url"]; got != "data:image/png;base64,abc" {
		t.Fatalf("image url = %v", got)
	}
}

func TestGeminiMultimodalInputFallsBackForTextOnlyGLMModel(t *testing.T) {
	adapter := Adapter{DefaultModel: "glm-5.1"}
	payload := map[string]any{
		"contents": []any{map[string]any{
			"role": "user",
			"parts": []any{
				map[string]any{"text": "describe"},
				map[string]any{"fileData": map[string]any{"mimeType": "image/png", "fileUri": "https://example.com/a.png"}},
			},
		}},
	}

	prepared, err := adapter.BuildUpstreamPayload("glm-5.1", payload)
	if err != nil {
		t.Fatal(err)
	}
	messages := prepared.ChatPayload["messages"].([]map[string]any)
	content, ok := messages[0]["content"].(string)
	if !ok {
		t.Fatalf("content should be text fallback: %#v", messages[0]["content"])
	}
	if content != "describe\n[Image input omitted: selected GLM model does not support multimodal input.]" {
		t.Fatalf("fallback content = %q", content)
	}
}
