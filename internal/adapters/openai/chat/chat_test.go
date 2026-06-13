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

import "testing"

func TestBuildUpstreamPayloadMapsChatCompletion(t *testing.T) {
	adapter := Adapter{DefaultModel: "glm-5.1"}
	payload := map[string]any{
		"model": "glm-5.1",
		"messages": []any{
			map[string]any{"role": "developer", "content": "Be brief."},
			map[string]any{"role": "user", "content": "Hi"},
		},
		"max_completion_tokens": 32,
		"temperature":           0.2,
		"functions":             []any{map[string]any{"name": "legacy_fn", "parameters": map[string]any{"type": "object", "properties": map[string]any{}}}},
		"function_call":         map[string]any{"name": "legacy_fn"},
		"response_format": map[string]any{
			"type": "json_schema",
			"json_schema": map[string]any{
				"name":   "answer",
				"schema": map[string]any{"type": "object", "properties": map[string]any{"ok": map[string]any{"type": "boolean"}}},
			},
		},
		"reasoning_effort": "xhigh",
	}

	chatPayload, _, err := adapter.BuildUpstreamPayload(payload)
	if err != nil {
		t.Fatal(err)
	}
	messages := chatPayload["messages"].([]map[string]any)
	if got := messages[0]["role"]; got != "system" {
		t.Fatalf("schema instruction role = %v", got)
	}
	if got := messages[1]["role"]; got != "system" {
		t.Fatalf("developer role = %v", got)
	}
	if got := chatPayload["max_tokens"]; got != 32 {
		t.Fatalf("max_tokens = %v", got)
	}
	tools := chatPayload["tools"].([]map[string]any)
	fn := tools[0]["function"].(map[string]any)
	if got := fn["name"]; got != "legacy_fn" {
		t.Fatalf("tool name = %v", got)
	}
	choice := chatPayload["tool_choice"].(map[string]any)["function"].(map[string]any)
	if got := choice["name"]; got != "legacy_fn" {
		t.Fatalf("tool choice = %v", got)
	}
	if got := chatPayload["response_format"].(map[string]any)["type"]; got != "json_object" {
		t.Fatalf("response_format = %v", got)
	}
	thinking := chatPayload["thinking"].(map[string]any)
	if got := thinking["type"]; got != "enabled" {
		t.Fatalf("thinking.type = %v", got)
	}
	if _, ok := chatPayload["reasoning_effort"]; ok {
		t.Fatalf("reasoning_effort should not be forwarded: %#v", chatPayload)
	}
}

func TestChatMultimodalContentRespectsGLMModelCapabilities(t *testing.T) {
	adapter := Adapter{DefaultModel: "glm-5.1"}
	payload := map[string]any{
		"model": "glm-4.6v-flash",
		"messages": []any{map[string]any{
			"role": "user",
			"content": []any{
				map[string]any{"type": "text", "text": "describe"},
				map[string]any{"type": "image_url", "image_url": map[string]any{"url": "data:image/png;base64,abc"}},
			},
		}},
	}

	chatPayload, _, err := adapter.BuildUpstreamPayload(payload)
	if err != nil {
		t.Fatal(err)
	}
	messages := chatPayload["messages"].([]map[string]any)
	parts := messages[0]["content"].([]any)
	if got := parts[1].(map[string]any)["image_url"].(map[string]any)["url"]; got != "data:image/png;base64,abc" {
		t.Fatalf("image url = %v", got)
	}

	payload["model"] = "glm-5.1"
	chatPayload, _, err = adapter.BuildUpstreamPayload(payload)
	if err != nil {
		t.Fatal(err)
	}
	messages = chatPayload["messages"].([]map[string]any)
	if _, ok := messages[0]["content"].([]any); ok {
		t.Fatalf("text-only model should not receive multimodal parts: %#v", messages[0]["content"])
	}
}
