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

package httpapi

import (
	"bytes"
	"context"
	"encoding/json"
	"log"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"glm-compatible/internal/adapters/openai/shared"
	"glm-compatible/internal/config"
	"glm-compatible/internal/state"
)

type fakeUpstream struct {
	chatFn     func(shared.Map) (shared.Map, error)
	streamFn   func(shared.Map, func(shared.Map) error) error
	tokenizeFn func(shared.Map) (shared.Map, error)
}

func (f fakeUpstream) Chat(_ context.Context, payload shared.Map) (shared.Map, error) {
	if f.chatFn == nil {
		return shared.Map{}, nil
	}
	return f.chatFn(payload)
}

func (f fakeUpstream) StreamChat(_ context.Context, payload shared.Map, handle func(shared.Map) error) error {
	if f.streamFn == nil {
		return nil
	}
	return f.streamFn(payload, handle)
}

func (f fakeUpstream) Tokenize(_ context.Context, payload shared.Map) (shared.Map, error) {
	if f.tokenizeFn == nil {
		return shared.Map{"usage": shared.Map{"total_tokens": 0}}, nil
	}
	return f.tokenizeFn(payload)
}

func testServer(up fakeUpstream) *Server {
	return New(config.Config{
		APITokens:    []string{"sk-test"},
		DefaultModel: "glm-5.1",
		ModelIDs:     []string{"glm-5.1"},
		GLMBaseURL:   "https://api.z.ai/api",
	}, up, state.New())
}

func debugTestServer(up fakeUpstream) *Server {
	return New(config.Config{
		APITokens:    []string{"sk-test"},
		DefaultModel: "glm-5.1",
		ModelIDs:     []string{"glm-5.1"},
		GLMBaseURL:   "https://api.z.ai/api",
		DebugLogBody: true,
	}, up, state.New())
}

func limitedBodyTestServer(limit int64) *Server {
	return New(config.Config{
		APITokens:           []string{"sk-test"},
		DefaultModel:        "glm-5.1",
		ModelIDs:            []string{"glm-5.1"},
		GLMBaseURL:          "https://api.z.ai/api",
		MaxRequestBodyBytes: limit,
	}, fakeUpstream{}, state.New())
}

func TestCreateResponseEndpointStoresAndRetrieves(t *testing.T) {
	server := testServer(fakeUpstream{chatFn: func(payload shared.Map) (shared.Map, error) {
		messages := payload["messages"].([]map[string]any)
		if messages[0]["content"] != "Be brief." || messages[1]["content"] != "Hello" {
			t.Fatalf("messages = %#v", messages)
		}
		return shared.Map{
			"choices": []any{map[string]any{"finish_reason": "stop", "message": map[string]any{"role": "assistant", "content": "Hi!"}}},
			"usage":   map[string]any{"prompt_tokens": 3, "completion_tokens": 2, "total_tokens": 5},
		}, nil
	}})
	body := `{"model":"glm-5.1","instructions":"Be brief.","input":"Hello"}`
	rec := request(server, http.MethodPost, "/v1/responses", body)
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
	var data map[string]any
	_ = json.Unmarshal(rec.Body.Bytes(), &data)
	if data["object"] != "response" || data["output_text"] != "Hi!" {
		t.Fatalf("response = %#v", data)
	}
	id := data["id"].(string)
	rec = request(server, http.MethodGet, "/v1/responses/"+id, "")
	if rec.Code != http.StatusOK {
		t.Fatalf("retrieve status=%d", rec.Code)
	}
	rec = request(server, http.MethodDelete, "/v1/responses/"+id, "")
	if rec.Code != http.StatusOK {
		t.Fatalf("delete status=%d", rec.Code)
	}
}

func TestStreamResponseMapsReasoningSummary(t *testing.T) {
	server := testServer(fakeUpstream{streamFn: func(payload shared.Map, handle func(shared.Map) error) error {
		if payload["stream"] != true {
			t.Fatalf("stream = %#v", payload["stream"])
		}
		chunks := []shared.Map{
			{"choices": []any{map[string]any{"delta": map[string]any{"reasoning_content": "Need "}}}},
			{"choices": []any{map[string]any{"delta": map[string]any{"reasoning_content": "answer."}}}},
			{"choices": []any{map[string]any{"delta": map[string]any{"content": "Hi!"}, "finish_reason": "stop"}}},
		}
		for _, chunk := range chunks {
			if err := handle(chunk); err != nil {
				return err
			}
		}
		return nil
	}})
	body := `{"model":"glm-5.1","input":"Hello","stream":true}`
	rec := request(server, http.MethodPost, "/v1/responses", body)
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
	text := rec.Body.String()
	if !strings.Contains(text, "response.reasoning_summary_text.delta") || !strings.Contains(text, `"delta":"Need "`) {
		t.Fatalf("missing reasoning delta: %s", text)
	}
	if !strings.Contains(text, `"type":"reasoning"`) || !strings.Contains(text, `"type":"summary_text"`) || !strings.Contains(text, `"text":"Need answer."`) {
		t.Fatalf("missing reasoning summary item: %s", text)
	}
	if !strings.Contains(text, `"output_text"`) || !strings.Contains(text, `"text":"Hi!"`) {
		t.Fatalf("missing text output: %s", text)
	}
}

func TestResponsesInputTokensUsesGLMTokenizer(t *testing.T) {
	server := testServer(fakeUpstream{tokenizeFn: func(payload shared.Map) (shared.Map, error) {
		if payload["model"] != "glm-5.1" {
			t.Fatalf("tokenize payload = %#v", payload)
		}
		if payload["stream"] != nil {
			t.Fatalf("stream should not be sent to tokenizer: %#v", payload)
		}
		return shared.Map{"usage": shared.Map{"total_tokens": 17}}, nil
	}})
	body := `{"model":"glm-5.1","instructions":"Be brief.","input":"Hello!"}`
	rec := request(server, http.MethodPost, "/v1/responses/input_tokens", body)
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
	var data map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &data); err != nil {
		t.Fatal(err)
	}
	if data["object"] != "response.input_tokens" || int(data["input_tokens"].(float64)) != 17 {
		t.Fatalf("input tokens response = %#v", data)
	}
}

func TestGLMChatCompletionPassthrough(t *testing.T) {
	server := testServer(fakeUpstream{chatFn: func(payload shared.Map) (shared.Map, error) {
		if payload["max_tokens"] != json.Number("8") {
			t.Fatalf("max_tokens=%#v", payload["max_tokens"])
		}
		if payload["temperature"] != json.Number("0.2") {
			t.Fatalf("temperature=%#v", payload["temperature"])
		}
		if payload["custom_glm_field"] != "kept" {
			t.Fatalf("custom field missing: %#v", payload)
		}
		return shared.Map{
			"id":      "glm_1",
			"object":  "chat.completion",
			"model":   payload["model"],
			"choices": []any{map[string]any{"index": 0, "finish_reason": "stop", "message": map[string]any{"role": "assistant", "content": "Hi!"}}},
		}, nil
	}})
	body := `{"model":"glm-5.1","messages":[{"role":"user","content":"Hello"}],"max_tokens":8,"temperature":0.2,"custom_glm_field":"kept"}`
	rec := request(server, http.MethodPost, "/paas/v4/chat/completions", body)
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), `"id":"glm_1"`) || !strings.Contains(rec.Body.String(), `"content":"Hi!"`) {
		t.Fatalf("body=%s", rec.Body.String())
	}
}

func TestGLMChatCompletionStreamPassthrough(t *testing.T) {
	server := testServer(fakeUpstream{streamFn: func(payload shared.Map, handle func(shared.Map) error) error {
		if payload["stream"] != true {
			t.Fatalf("stream=%v", payload["stream"])
		}
		return handle(shared.Map{"id": "chunk_1", "choices": []any{map[string]any{"delta": map[string]any{"content": "Hi"}}}})
	}})
	body := `{"model":"glm-5.1","messages":[{"role":"user","content":"Hello"}],"stream":true}`
	rec := request(server, http.MethodPost, "/paas/v4/chat/completions", body)
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), `"id":"chunk_1"`) || !strings.Contains(rec.Body.String(), `"content":"Hi"`) || !strings.Contains(rec.Body.String(), "data: [DONE]") {
		t.Fatalf("stream body=%s", rec.Body.String())
	}
}

func TestCreateChatCompletionStoresAndLists(t *testing.T) {
	server := testServer(fakeUpstream{chatFn: func(payload shared.Map) (shared.Map, error) {
		if payload["max_tokens"] != json.Number("8") {
			t.Fatalf("max_tokens=%#v", payload["max_tokens"])
		}
		return shared.Map{
			"id":      "glm_chat_1",
			"created": 123,
			"model":   payload["model"],
			"choices": []any{map[string]any{"index": 0, "finish_reason": "stop", "message": map[string]any{"role": "assistant", "content": "Hi!", "reasoning_content": "short thought"}}},
			"usage":   map[string]any{"prompt_tokens": 3, "completion_tokens": 2, "total_tokens": 5, "prompt_cache_hit_tokens": 1},
		}, nil
	}})
	body := `{"model":"glm-5.1","messages":[{"role":"user","content":"Hello"}],"max_tokens":8,"metadata":{"topic":"demo"},"store":true}`
	rec := request(server, http.MethodPost, "/v1/chat/completions", body)
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), `"reasoning_content":"short thought"`) {
		t.Fatalf("reasoning_content not preserved: %s", rec.Body.String())
	}
	rec = request(server, http.MethodGet, "/v1/chat/completions?metadata[topic]=demo", "")
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), "glm_chat_1") {
		t.Fatalf("list status=%d body=%s", rec.Code, rec.Body.String())
	}
	rec = request(server, http.MethodGet, "/v1/chat/completions/glm_chat_1/messages", "")
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), "assistant") {
		t.Fatalf("messages status=%d body=%s", rec.Code, rec.Body.String())
	}
}

func TestChatCompletionStreamReturnsChunks(t *testing.T) {
	server := testServer(fakeUpstream{streamFn: func(payload shared.Map, handle func(shared.Map) error) error {
		if payload["stream"] != true {
			t.Fatalf("stream=%v", payload["stream"])
		}
		if err := handle(shared.Map{"id": "chunk_1", "created": 123, "choices": []any{map[string]any{"index": 0, "delta": map[string]any{"role": "assistant", "content": "Hi"}, "finish_reason": nil}}, "usage": nil}); err != nil {
			return err
		}
		return handle(shared.Map{"id": "chunk_1", "created": 123, "choices": []any{}, "usage": map[string]any{"prompt_tokens": 1, "completion_tokens": 1, "total_tokens": 2}})
	}})
	body := `{"model":"glm-5.1","messages":[{"role":"user","content":"Hello"}],"stream":true}`
	rec := request(server, http.MethodPost, "/v1/chat/completions", body)
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "data: [DONE]") || !strings.Contains(rec.Body.String(), `"object":"chat.completion.chunk"`) {
		t.Fatalf("stream body=%s", rec.Body.String())
	}
}

func TestAnthropicMessageEndpointUsesXAPIKey(t *testing.T) {
	server := testServer(fakeUpstream{chatFn: func(payload shared.Map) (shared.Map, error) {
		messages := payload["messages"].([]map[string]any)
		if messages[0]["role"] != "system" || messages[1]["content"] != "Hello" {
			t.Fatalf("messages = %#v", messages)
		}
		return shared.Map{
			"id":      "anthropic_1",
			"model":   payload["model"],
			"choices": []any{map[string]any{"finish_reason": "stop", "message": map[string]any{"role": "assistant", "content": "Hi!"}}},
			"usage":   map[string]any{"prompt_tokens": 3, "completion_tokens": 2},
		}, nil
	}})
	body := `{"model":"glm-5.1","system":"Be brief.","max_tokens":8,"messages":[{"role":"user","content":"Hello"}]}`
	rec := requestWithHeader(server, http.MethodPost, "/v1/messages", body, "x-api-key", "sk-test")
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), `"type":"message"`) || !strings.Contains(rec.Body.String(), `"text":"Hi!"`) {
		t.Fatalf("body=%s", rec.Body.String())
	}
}

func TestAnthropicCountTokens(t *testing.T) {
	server := testServer(fakeUpstream{})
	body := `{"model":"glm-5.1","messages":[{"role":"user","content":"Hello world"}]}`
	rec := requestWithHeader(server, http.MethodPost, "/v1/messages/count_tokens", body, "x-api-key", "sk-test")
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), "input_tokens") {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
}

func TestGeminiCountTokens(t *testing.T) {
	server := testServer(fakeUpstream{})
	body := `{"contents":[{"role":"user","parts":[{"text":"Hello world"}]}]}`
	rec := requestWithHeader(server, http.MethodPost, "/v1beta/models/gemini-3.5-flash:countTokens", body, "x-goog-api-key", "sk-test")
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), "totalTokens") {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
}

func TestGeminiGenerateContentEndpointUsesXGoogAPIKey(t *testing.T) {
	server := testServer(fakeUpstream{chatFn: func(payload shared.Map) (shared.Map, error) {
		messages := payload["messages"].([]map[string]any)
		if messages[0]["content"] != "Hello" {
			t.Fatalf("messages = %#v", messages)
		}
		return shared.Map{
			"id":      "gemini_1",
			"model":   payload["model"],
			"choices": []any{map[string]any{"finish_reason": "stop", "message": map[string]any{"role": "assistant", "content": "Hi!"}}},
			"usage":   map[string]any{"prompt_tokens": 1, "completion_tokens": 1, "total_tokens": 2},
		}, nil
	}})
	body := `{"contents":[{"role":"user","parts":[{"text":"Hello"}]}],"generationConfig":{"maxOutputTokens":8}}`
	rec := requestWithHeader(server, http.MethodPost, "/v1beta/models/gemini-3.5-flash:generateContent", body, "x-goog-api-key", "sk-test")
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), `"candidates"`) || !strings.Contains(rec.Body.String(), `"text":"Hi!"`) {
		t.Fatalf("body=%s", rec.Body.String())
	}
}

func TestGeminiStreamGenerateContent(t *testing.T) {
	server := testServer(fakeUpstream{streamFn: func(payload shared.Map, handle func(shared.Map) error) error {
		if payload["stream"] != true {
			t.Fatalf("stream=%v", payload["stream"])
		}
		return handle(shared.Map{"id": "chunk_1", "choices": []any{map[string]any{"delta": map[string]any{"content": "Hi"}, "finish_reason": nil}}, "usage": nil})
	}})
	body := `{"contents":[{"role":"user","parts":[{"text":"Hello"}]}]}`
	rec := requestWithHeader(server, http.MethodPost, "/v1beta/models/gemini-3.5-flash:streamGenerateContent?alt=sse", body, "x-goog-api-key", "sk-test")
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "data:") || !strings.Contains(rec.Body.String(), `"candidates"`) || !strings.Contains(rec.Body.String(), `"text":"Hi"`) {
		t.Fatalf("body=%s", rec.Body.String())
	}
}

func TestDebugLogBodyLogsRedactedRequestAndResponse(t *testing.T) {
	var logs bytes.Buffer
	previousOutput := log.Writer()
	previousFlags := log.Flags()
	log.SetOutput(&logs)
	log.SetFlags(0)
	defer log.SetOutput(previousOutput)
	defer log.SetFlags(previousFlags)

	server := debugTestServer(fakeUpstream{chatFn: func(payload shared.Map) (shared.Map, error) {
		return shared.Map{
			"id":      "chat_1",
			"model":   payload["model"],
			"choices": []any{map[string]any{"finish_reason": "stop", "message": map[string]any{"role": "assistant", "content": "Hi!"}}},
		}, nil
	}})
	body := `{"model":"glm-5.1","api_key":"secret-value","messages":[{"role":"user","content":"Hello"}]}`
	rec := request(server, http.MethodPost, "/v1/chat/completions", body)
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
	text := logs.String()
	if !strings.Contains(text, "debug body request") || !strings.Contains(text, "debug body response") {
		t.Fatalf("missing debug body logs: %s", text)
	}
	if strings.Contains(text, "secret-value") {
		t.Fatalf("debug logs leaked secret: %s", text)
	}
	if !strings.Contains(text, `"api_key":"[REDACTED]"`) || !strings.Contains(text, `"content":"Hi!"`) {
		t.Fatalf("debug logs = %s", text)
	}
}

func TestReadJSONRejectsOversizedBody(t *testing.T) {
	server := limitedBodyTestServer(8)
	rec := request(server, http.MethodPost, "/v1/chat/completions", `{"model":"glm-5.1"}`)
	if rec.Code != http.StatusRequestEntityTooLarge {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
}

func TestReadJSONRejectsTrailingContent(t *testing.T) {
	server := testServer(fakeUpstream{})
	rec := request(server, http.MethodPost, "/v1/chat/completions", `{"model":"glm-5.1"} trailing`)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
}

func TestMemoryHealthReportsStoreStats(t *testing.T) {
	store := state.New()
	store.SaveResponse(shared.Map{"id": "resp_1"}, nil, []shared.Map{{"id": "msg_1"}}, true, "", nil)
	server := New(config.Config{
		APITokens:    []string{"sk-test"},
		DefaultModel: "glm-5.1",
		ModelIDs:     []string{"glm-5.1"},
		GLMBaseURL:   "https://api.z.ai/api",
	}, fakeUpstream{}, store)

	rec := request(server, http.MethodGet, "/healthz/memory", "")
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
	var data map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &data); err != nil {
		t.Fatal(err)
	}
	storeStats, ok := data["store"].(map[string]any)
	if !ok {
		t.Fatalf("store stats missing: %#v", data)
	}
	if int(storeStats["responses"].(float64)) != 1 || int(storeStats["items"].(float64)) != 1 {
		t.Fatalf("store stats = %#v", storeStats)
	}
}

func TestDebugPprofRequiresFlag(t *testing.T) {
	server := testServer(fakeUpstream{})
	rec := request(server, http.MethodGet, "/debug/pprof/", "")
	if rec.Code != http.StatusNotFound {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}

	server.cfg.DebugPprof = true
	rec = request(server, http.MethodGet, "/debug/pprof/", "")
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
}

func request(server *Server, method, path, body string) *httptest.ResponseRecorder {
	return requestWithHeader(server, method, path, body, "Authorization", "Bearer sk-test")
}

func requestWithHeader(server *Server, method, path, body, header, value string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(method, path, bytes.NewBufferString(body))
	req.Header.Set(header, value)
	if body != "" {
		req.Header.Set("Content-Type", "application/json")
	}
	rec := httptest.NewRecorder()
	server.ServeHTTP(rec, req)
	return rec
}
