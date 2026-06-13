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

package messages

import "testing"

func TestBuildUpstreamPayloadMapsAnthropicToolsAndThinking(t *testing.T) {
	adapter := Adapter{DefaultModel: "glm-5.1"}
	payload := map[string]any{
		"model":      "glm-5.1",
		"system":     "Be brief.",
		"max_tokens": 32,
		"messages": []any{
			map[string]any{"role": "user", "content": []any{map[string]any{"type": "text", "text": "Weather?"}}},
			map[string]any{"role": "assistant", "content": []any{map[string]any{"type": "tool_use", "id": "toolu_1", "name": "get_weather", "input": map[string]any{"city": "Hangzhou"}}}},
			map[string]any{"role": "user", "content": []any{map[string]any{"type": "tool_result", "tool_use_id": "toolu_1", "content": "24C"}}},
		},
		"tools":       []any{map[string]any{"name": "get_weather", "description": "Get weather.", "input_schema": map[string]any{"type": "object"}}},
		"tool_choice": map[string]any{"type": "tool", "name": "get_weather"},
		"thinking":    map[string]any{"type": "enabled", "budget_tokens": 1024},
	}

	prepared, err := adapter.BuildUpstreamPayload(payload)
	if err != nil {
		t.Fatal(err)
	}
	messages := prepared.ChatPayload["messages"].([]map[string]any)
	if messages[0]["role"] != "system" || messages[0]["content"] != "Be brief." {
		t.Fatalf("system message = %#v", messages[0])
	}
	if messages[2]["role"] != "assistant" {
		t.Fatalf("assistant tool message = %#v", messages[2])
	}
	if messages[3]["role"] != "tool" || messages[3]["tool_call_id"] != "toolu_1" {
		t.Fatalf("tool result = %#v", messages[3])
	}
	tools := prepared.ChatPayload["tools"].([]map[string]any)
	if tools[0]["function"].(map[string]any)["name"] != "get_weather" {
		t.Fatalf("tools = %#v", tools)
	}
	choice := prepared.ChatPayload["tool_choice"].(map[string]any)["function"].(map[string]any)
	if choice["name"] != "get_weather" {
		t.Fatalf("tool_choice = %#v", choice)
	}
	if prepared.ChatPayload["thinking"].(map[string]any)["type"] != "enabled" {
		t.Fatalf("thinking = %#v", prepared.ChatPayload["thinking"])
	}
}

func TestResponseFromUpstreamMapsAnthropicContent(t *testing.T) {
	completion := map[string]any{
		"id":    "chat_1",
		"model": "glm-5.1",
		"choices": []any{map[string]any{
			"finish_reason": "tool_calls",
			"message": map[string]any{
				"role":              "assistant",
				"reasoning_content": "Need tool.",
				"content":           "Checking.",
				"tool_calls":        []any{map[string]any{"id": "call_1", "type": "function", "function": map[string]any{"name": "get_weather", "arguments": "{\"city\":\"Hangzhou\"}"}}},
			},
		}},
		"usage": map[string]any{"prompt_tokens": 5, "completion_tokens": 3},
	}
	response := ResponseFromUpstream(completion, nil, "glm-5.1")
	if response["type"] != "message" || response["stop_reason"] != "tool_use" {
		t.Fatalf("response = %#v", response)
	}
	blocks := response["content"].([]any)
	if blocks[0].(map[string]any)["type"] != "thinking" {
		t.Fatalf("blocks = %#v", blocks)
	}
	if blocks[2].(map[string]any)["type"] != "tool_use" {
		t.Fatalf("tool block = %#v", blocks[2])
	}
}

func TestAnthropicMultimodalInputMapsToGLMVisionContent(t *testing.T) {
	adapter := Adapter{DefaultModel: "glm-5.1"}
	payload := map[string]any{
		"model": "glm-4.6v-flash",
		"messages": []any{map[string]any{
			"role": "user",
			"content": []any{
				map[string]any{"type": "text", "text": "describe"},
				map[string]any{
					"type": "image",
					"source": map[string]any{
						"type":       "base64",
						"media_type": "image/png",
						"data":       "abc",
					},
				},
			},
		}},
	}

	prepared, err := adapter.BuildUpstreamPayload(payload)
	if err != nil {
		t.Fatal(err)
	}
	messages := prepared.ChatPayload["messages"].([]map[string]any)
	parts := messages[0]["content"].([]any)
	if got := parts[1].(map[string]any)["image_url"].(map[string]any)["url"]; got != "data:image/png;base64,abc" {
		t.Fatalf("image url = %v", got)
	}
}

func TestAnthropicMultimodalInputFallsBackForTextOnlyGLMModel(t *testing.T) {
	adapter := Adapter{DefaultModel: "glm-5.1"}
	payload := map[string]any{
		"model": "glm-5.1",
		"messages": []any{map[string]any{
			"role": "user",
			"content": []any{
				map[string]any{"type": "text", "text": "describe"},
				map[string]any{
					"type":   "image",
					"source": map[string]any{"type": "url", "url": "https://example.com/a.png"},
				},
			},
		}},
	}

	prepared, err := adapter.BuildUpstreamPayload(payload)
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
