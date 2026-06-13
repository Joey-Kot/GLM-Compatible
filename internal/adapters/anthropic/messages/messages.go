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

import (
	"encoding/json"
	"fmt"
	"strings"

	"glm-compatible/internal/adapters/openai/chat"
	"glm-compatible/internal/adapters/openai/shared"
)

type Adapter struct {
	DefaultModel string
}

type Prepared struct {
	ChatPayload shared.Map
	Messages    []shared.Map
}

func (a Adapter) BuildUpstreamPayload(payload shared.Map) (Prepared, error) {
	messages, err := a.normalizeMessages(payload)
	if err != nil {
		return Prepared{}, err
	}
	model := shared.StringValue(payload["model"])
	if model == "" {
		model = a.DefaultModel
	}
	chatPayload := shared.Map{
		"model":    model,
		"messages": messages,
		"stream":   shared.BoolValue(payload["stream"]),
	}
	for source, target := range map[string]string{
		"max_tokens":      "max_tokens",
		"temperature":     "temperature",
		"top_p":           "top_p",
		"stop_sequences":  "stop",
		"metadata":        "user",
		"service_tier":    "service_tier",
		"container":       "user",
		"context":         "user",
		"mcp_servers":     "user",
		"stream_options":  "stream_options",
		"top_k":           "top_k",
		"parallel_tools":  "parallel_tool_calls",
		"disable_context": "disable_context",
	} {
		if value, ok := payload[source]; ok && value != nil {
			chatPayload[target] = value
		}
	}
	if tools := NormalizeTools(payload["tools"]); len(tools) > 0 {
		chatPayload["tools"] = tools
	}
	if choice := NormalizeToolChoice(payload["tool_choice"]); choice != nil {
		chatPayload["tool_choice"] = choice
	}
	MapThinking(payload, chatPayload)
	MapOutputFormat(payload, messages, chatPayload)
	if mappedMessages, ok := chatPayload["messages"].([]shared.Map); ok {
		chatPayload["messages"] = chat.MessagesForGLMModel(mappedMessages, model)
	} else {
		chatPayload["messages"] = chat.MessagesForGLMModel(messages, model)
	}
	messages = chatPayload["messages"].([]shared.Map)
	return Prepared{ChatPayload: chatPayload, Messages: messages}, nil
}

func (a Adapter) normalizeMessages(payload shared.Map) ([]shared.Map, error) {
	rawMessages, ok := payload["messages"].([]any)
	if !ok {
		return nil, fmt.Errorf("'messages' must be an array")
	}
	messages := []shared.Map{}
	if system := SystemText(payload["system"]); system != "" {
		messages = append(messages, shared.Map{"role": "system", "content": system})
	}
	for _, raw := range rawMessages {
		msg, ok := raw.(map[string]any)
		if !ok {
			return nil, fmt.Errorf("each message must be an object")
		}
		role := shared.StringValue(msg["role"])
		if role != "user" && role != "assistant" {
			return nil, fmt.Errorf("unsupported Anthropic message role")
		}
		messages = append(messages, ContentToChatMessages(role, msg["content"])...)
	}
	if len(messages) == 0 {
		return nil, fmt.Errorf("'messages' must contain at least one message")
	}
	return messages, nil
}

func SystemText(value any) string {
	switch v := value.(type) {
	case nil:
		return ""
	case string:
		return v
	case []any:
		parts := []string{}
		for _, raw := range v {
			if block, ok := raw.(map[string]any); ok {
				if shared.StringValue(block["type"]) == "text" || block["text"] != nil {
					parts = append(parts, shared.StringValue(block["text"]))
				}
			} else {
				parts = append(parts, shared.StringValue(raw))
			}
		}
		return strings.Join(parts, "\n")
	default:
		return shared.StringValue(value)
	}
}

func ContentToChatMessages(role string, content any) []shared.Map {
	if text, ok := content.(string); ok {
		return []shared.Map{{"role": role, "content": text}}
	}
	blocks, ok := content.([]any)
	if !ok {
		return []shared.Map{{"role": role, "content": shared.StringValue(content)}}
	}
	messages := []shared.Map{}
	contentParts := []any{}
	flushContent := func() {
		if len(contentParts) > 0 {
			messages = append(messages, shared.Map{"role": role, "content": contentParts})
			contentParts = nil
		}
	}
	appendText := func(text string) {
		if text != "" {
			contentParts = append(contentParts, shared.Map{"type": "text", "text": text})
		}
	}
	for _, raw := range blocks {
		block, ok := raw.(map[string]any)
		if !ok {
			appendText(shared.StringValue(raw))
			continue
		}
		switch shared.StringValue(block["type"]) {
		case "text":
			appendText(shared.StringValue(block["text"]))
		case "thinking":
			if role == "assistant" {
				flushContent()
				messages = append(messages, shared.Map{"role": "assistant", "content": "", "reasoning_content": shared.StringValue(block["thinking"])})
			}
		case "tool_use":
			flushContent()
			args := shared.JSONString(block["input"])
			if args == "null" {
				args = "{}"
			}
			messages = append(messages, shared.Map{
				"role":    "assistant",
				"content": "",
				"tool_calls": []any{shared.Map{
					"id":   valueOrDefault(block["id"], shared.NewID("toolu")),
					"type": "function",
					"function": shared.Map{
						"name":      block["name"],
						"arguments": args,
					},
				}},
			})
		case "tool_result":
			flushContent()
			messages = append(messages, shared.Map{"role": "tool", "tool_call_id": block["tool_use_id"], "content": BlockText(block["content"])})
		case "image":
			if part := AnthropicImagePart(block); part != nil && role == "user" {
				contentParts = append(contentParts, part)
			} else {
				appendText(DescribeUnsupportedBlock(block))
			}
		case "document":
			if part := AnthropicDocumentPart(block); part != nil && role == "user" {
				contentParts = append(contentParts, part)
			} else {
				appendText(DescribeUnsupportedBlock(block))
			}
		case "search_result", "web_search_tool_result", "web_fetch_tool_result", "code_execution_tool_result":
			appendText(DescribeUnsupportedBlock(block))
		default:
			appendText(BlockText(block))
		}
	}
	flushContent()
	if len(messages) == 0 {
		return []shared.Map{{"role": role, "content": ""}}
	}
	return messages
}

func AnthropicImagePart(block map[string]any) shared.Map {
	url := AnthropicSourceURL(block["source"])
	if url == "" {
		return nil
	}
	return shared.Map{"type": "image_url", "image_url": shared.Map{"url": url}}
}

func AnthropicDocumentPart(block map[string]any) shared.Map {
	url := AnthropicSourceURL(block["source"])
	if url == "" {
		return nil
	}
	return shared.Map{"type": "file_url", "file_url": shared.Map{"url": url}}
}

func AnthropicSourceURL(value any) string {
	source, ok := value.(map[string]any)
	if !ok {
		return ""
	}
	switch shared.StringValue(source["type"]) {
	case "url":
		return shared.StringValue(source["url"])
	case "base64":
		data := shared.StringValue(source["data"])
		if data == "" {
			return ""
		}
		mediaType := shared.StringValue(source["media_type"])
		if mediaType == "" {
			return data
		}
		return "data:" + mediaType + ";base64," + data
	default:
		return shared.StringValue(source["url"])
	}
}

func BlockText(value any) string {
	switch v := value.(type) {
	case nil:
		return ""
	case string:
		return v
	case []any:
		parts := []string{}
		for _, item := range v {
			parts = append(parts, BlockText(item))
		}
		return strings.Join(parts, "")
	case map[string]any:
		if text := shared.StringValue(v["text"]); text != "" {
			return text
		}
		return shared.JSONString(v)
	default:
		return shared.StringValue(value)
	}
}

func DescribeUnsupportedBlock(block map[string]any) string {
	t := shared.StringValue(block["type"])
	if t == "" {
		t = "content"
	}
	return "\n[" + t + " block omitted by GLM compatibility layer: " + shared.JSONString(block) + "]\n"
}

func NormalizeTools(value any) []shared.Map {
	rawTools, ok := value.([]any)
	if !ok {
		return nil
	}
	out := []shared.Map{}
	for _, raw := range rawTools {
		tool, ok := raw.(map[string]any)
		if !ok {
			continue
		}
		name := shared.StringValue(tool["name"])
		if name == "" {
			continue
		}
		function := shared.Map{
			"name":        shared.SafeUpstreamToolName(name),
			"description": tool["description"],
			"parameters":  valueOrDefault(tool["input_schema"], shared.Map{}),
		}
		if tool["strict"] != nil {
			function["strict"] = tool["strict"]
		}
		out = append(out, shared.Map{"type": "function", "function": function})
	}
	return out
}

func NormalizeToolChoice(value any) any {
	switch v := value.(type) {
	case string:
		switch v {
		case "auto", "none":
			return v
		case "any":
			return "required"
		}
	case map[string]any:
		switch shared.StringValue(v["type"]) {
		case "auto":
			return "auto"
		case "none":
			return "none"
		case "any":
			return "required"
		case "tool":
			if name := shared.StringValue(v["name"]); name != "" {
				return shared.Map{"type": "function", "function": shared.Map{"name": shared.SafeUpstreamToolName(name)}}
			}
		}
	}
	return nil
}

func MapThinking(payload shared.Map, chatPayload shared.Map) {
	thinking, ok := payload["thinking"].(map[string]any)
	if !ok {
		return
	}
	switch shared.StringValue(thinking["type"]) {
	case "enabled":
		chatPayload["thinking"] = shared.Map{"type": "enabled"}
	case "disabled":
		chatPayload["thinking"] = shared.Map{"type": "disabled"}
	}
}

func MapOutputFormat(payload shared.Map, messages []shared.Map, chatPayload shared.Map) {
	var format map[string]any
	if outputConfig, ok := payload["output_config"].(map[string]any); ok {
		format, _ = outputConfig["format"].(map[string]any)
		if effort := shared.StringValue(outputConfig["effort"]); effort != "" {
			switch mapEffort(effort) {
			case "none", "minimal":
				chatPayload["thinking"] = shared.Map{"type": "disabled"}
			default:
				chatPayload["thinking"] = shared.Map{"type": "enabled"}
			}
		}
	}
	if format == nil {
		format, _ = payload["response_format"].(map[string]any)
	}
	if format == nil {
		return
	}
	t := shared.StringValue(format["type"])
	if t == "json_object" || t == "json_schema" {
		chatPayload["response_format"] = shared.Map{"type": "json_object"}
	}
	if t == "json_schema" && format["schema"] != nil {
		instruction := chat.JSONSchemaInstruction(format)
		if instruction != "" {
			chatPayload["messages"] = append([]shared.Map{{"role": "system", "content": instruction}}, messages...)
		}
	}
}

func mapEffort(effort string) string {
	switch effort {
	case "xhigh":
		return "max"
	case "low", "medium":
		return "high"
	default:
		return effort
	}
}

func ResponseFromUpstream(completion shared.Map, requestPayload shared.Map, defaultModel string) shared.Map {
	choices, _ := completion["choices"].([]any)
	var choice map[string]any
	if len(choices) > 0 {
		choice, _ = choices[0].(map[string]any)
	}
	message, _ := choice["message"].(map[string]any)
	contentBlocks := ContentBlocksFromMessage(message)
	model := shared.StringValue(completion["model"])
	if model == "" {
		model = shared.StringValue(requestPayload["model"])
	}
	if model == "" {
		model = defaultModel
	}
	return shared.Map{
		"id":            valueOrDefault(completion["id"], shared.NewID("msg")),
		"type":          "message",
		"role":          "assistant",
		"model":         model,
		"content":       contentBlocks,
		"stop_reason":   StopReason(shared.StringValue(choice["finish_reason"]), contentBlocks),
		"stop_sequence": nil,
		"usage":         UsageFromUpstream(completion["usage"]),
	}
}

func ContentBlocksFromMessage(message map[string]any) []any {
	blocks := []any{}
	if reasoning := shared.StringValue(message["reasoning_content"]); reasoning != "" {
		blocks = append(blocks, shared.Map{"type": "thinking", "thinking": reasoning, "signature": ""})
	}
	if text := shared.ContentToText(message["content"], false); text != "" {
		blocks = append(blocks, shared.Map{"type": "text", "text": text})
	}
	if calls, ok := message["tool_calls"].([]any); ok {
		for _, raw := range calls {
			call, ok := raw.(map[string]any)
			if !ok {
				continue
			}
			function, _ := call["function"].(map[string]any)
			var input any = shared.Map{}
			if args := shared.StringValue(function["arguments"]); args != "" {
				var parsed any
				if err := json.Unmarshal([]byte(args), &parsed); err == nil {
					input = parsed
				} else {
					input = shared.Map{"input": args}
				}
			}
			blocks = append(blocks, shared.Map{
				"type":  "tool_use",
				"id":    valueOrDefault(call["id"], shared.NewID("toolu")),
				"name":  function["name"],
				"input": input,
			})
		}
	}
	if len(blocks) == 0 {
		blocks = append(blocks, shared.Map{"type": "text", "text": ""})
	}
	return blocks
}

func StopReason(finish string, blocks []any) any {
	for _, raw := range blocks {
		if block, ok := raw.(shared.Map); ok && block["type"] == "tool_use" {
			return "tool_use"
		}
		if block, ok := raw.(map[string]any); ok && block["type"] == "tool_use" {
			return "tool_use"
		}
	}
	switch finish {
	case "length":
		return "max_tokens"
	case "tool_calls":
		return "tool_use"
	case "stop", "":
		return "end_turn"
	default:
		return finish
	}
}

func UsageFromUpstream(value any) shared.Map {
	usage, ok := value.(map[string]any)
	if !ok {
		return shared.Map{"input_tokens": 0, "output_tokens": 0}
	}
	input := shared.IntValue(usage["prompt_tokens"], 0)
	output := shared.IntValue(usage["completion_tokens"], 0)
	out := shared.Map{"input_tokens": input, "output_tokens": output}
	if cached := shared.IntValue(usage["prompt_cache_hit_tokens"], 0); cached > 0 {
		out["cache_read_input_tokens"] = cached
	}
	if reasoning := shared.IntValue(usage["reasoning_tokens"], 0); reasoning > 0 {
		out["thinking_tokens"] = reasoning
	}
	return out
}

func CountTokens(payload shared.Map, defaultModel string) (shared.Map, error) {
	adapter := Adapter{DefaultModel: defaultModel}
	prepared, err := adapter.BuildUpstreamPayload(payload)
	if err != nil {
		return nil, err
	}
	return shared.Map{"input_tokens": shared.EstimateTokensFromMessages(prepared.Messages)}, nil
}

func StreamStart(completionID any, requestPayload shared.Map, defaultModel string) shared.Map {
	model := shared.StringValue(requestPayload["model"])
	if model == "" {
		model = defaultModel
	}
	return shared.Map{
		"type": "message_start",
		"message": shared.Map{
			"id":            valueOrDefault(completionID, shared.NewID("msg")),
			"type":          "message",
			"role":          "assistant",
			"model":         model,
			"content":       []any{},
			"stop_reason":   nil,
			"stop_sequence": nil,
			"usage":         shared.Map{"input_tokens": 0, "output_tokens": 0},
		},
	}
}

func TextDelta(text string) shared.Map {
	return shared.Map{"type": "content_block_delta", "index": 0, "delta": shared.Map{"type": "text_delta", "text": text}}
}

func ThinkingDelta(text string) shared.Map {
	return shared.Map{"type": "content_block_delta", "index": 0, "delta": shared.Map{"type": "thinking_delta", "thinking": text}}
}

func MessageDelta(stopReason any, usage any) shared.Map {
	if usage == nil {
		usage = shared.Map{"output_tokens": 0}
	}
	return shared.Map{"type": "message_delta", "delta": shared.Map{"stop_reason": stopReason, "stop_sequence": nil}, "usage": usage}
}

func valueOrDefault(value any, fallback any) any {
	if value == nil {
		return fallback
	}
	return value
}
