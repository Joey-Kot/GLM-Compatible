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
	"context"
	"log"
	"net/http"
	"regexp"
	"strconv"
	"strings"

	anthropic "glm-compatible/internal/adapters/anthropic/messages"
	gemini "glm-compatible/internal/adapters/gemini/generate"
	"glm-compatible/internal/adapters/openai/chat"
	"glm-compatible/internal/adapters/openai/responses"
	"glm-compatible/internal/adapters/openai/shared"
	"glm-compatible/internal/debuglog"
)

func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if s.cfg.DebugLogBody {
		debugWriter := newDebugResponseWriter(w)
		s.serveHTTP(debugWriter, r)
		log.Printf("debug body response method=%s path=%s status=%d body=%s", r.Method, r.URL.RequestURI(), debugWriter.statusCode(), debuglog.Body(debugWriter.bodyBytes()))
		return
	}
	s.serveHTTP(w, r)
}

func (s *Server) serveHTTP(w http.ResponseWriter, r *http.Request) {
	s.setCommonHeaders(w)
	if r.Method == http.MethodOptions {
		w.WriteHeader(http.StatusNoContent)
		return
	}
	if r.URL.Path == "/health" {
		writeJSON(w, http.StatusOK, shared.Map{"status": "ok"})
		return
	}
	if !s.authorize(w, r) {
		return
	}

	path := strings.TrimRight(r.URL.Path, "/")
	if path == "" {
		path = "/"
	}
	if s.handleDiagnostics(w, r, path) {
		return
	}
	s.store.PruneIfDue()
	switch {
	case path == "/paas/v4/chat/completions":
		s.handleGLMChatCompletions(w, r)
	case path == "/v1/messages" || path == "/v1/messages/count_tokens":
		s.handleAnthropicMessages(w, r, path)
	case strings.HasPrefix(path, "/v1beta/models/") || strings.HasPrefix(path, "/v1/models/"):
		if s.handleGeminiModels(w, r, path) {
			return
		}
		openAIError(w, http.StatusNotFound, "not found", "invalid_request_error", "")
	case r.Method == http.MethodGet && path == "/v1/models":
		s.handleModels(w, r)
	case path == "/v1/chat/completions":
		s.handleChatCompletions(w, r)
	case strings.HasPrefix(path, "/v1/chat/completions/"):
		s.handleStoredChatCompletion(w, r, strings.TrimPrefix(path, "/v1/chat/completions/"))
	case path == "/v1/responses":
		s.handleResponses(w, r)
	case path == "/v1/responses/input_tokens":
		s.handleInputTokens(w, r)
	case path == "/v1/responses/compact":
		s.handleCompact(w, r)
	case strings.HasPrefix(path, "/v1/responses/"):
		s.handleStoredResponse(w, r, strings.TrimPrefix(path, "/v1/responses/"))
	case path == "/v1/conversations":
		s.handleConversations(w, r)
	case strings.HasPrefix(path, "/v1/conversations/"):
		s.handleStoredConversation(w, r, strings.TrimPrefix(path, "/v1/conversations/"))
	default:
		openAIError(w, http.StatusNotFound, "not found", "invalid_request_error", "")
	}
}

func (s *Server) handleGLMChatCompletions(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		methodNotAllowed(w)
		return
	}
	payload, ok := s.readJSON(w, r, false)
	if !ok {
		return
	}
	if shared.BoolValue(payload["stream"]) {
		s.streamUpstreamChatCompletion(w, r, payload)
		return
	}
	payload["stream"] = false
	completion, err := s.upstream.Chat(r.Context(), payload)
	if err != nil {
		s.upstreamError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, completion)
}

func (s *Server) handleAnthropicMessages(w http.ResponseWriter, r *http.Request, path string) {
	if r.Method != http.MethodPost {
		methodNotAllowed(w)
		return
	}
	payload, ok := s.readJSON(w, r, false)
	if !ok {
		return
	}
	if path == "/v1/messages/count_tokens" {
		prepared, err := s.anthropic.BuildUpstreamPayload(payload)
		if err != nil {
			openAIError(w, http.StatusBadRequest, err.Error(), "invalid_request_error", "")
			return
		}
		count, err := s.countGLMTokens(r.Context(), prepared.ChatPayload)
		if err != nil {
			s.upstreamError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, shared.Map{"input_tokens": count})
		return
	}
	prepared, err := s.anthropic.BuildUpstreamPayload(payload)
	if err != nil {
		openAIError(w, http.StatusBadRequest, err.Error(), "invalid_request_error", "")
		return
	}
	if shared.BoolValue(payload["stream"]) {
		s.streamAnthropicMessage(w, r, payload, prepared.ChatPayload)
		return
	}
	prepared.ChatPayload["stream"] = false
	completion, err := s.upstream.Chat(r.Context(), prepared.ChatPayload)
	if err != nil {
		s.upstreamError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, anthropic.ResponseFromUpstream(completion, payload, s.cfg.DefaultModel))
}

func (s *Server) handleGeminiModels(w http.ResponseWriter, r *http.Request, path string) bool {
	if r.Method != http.MethodPost {
		methodNotAllowed(w)
		return true
	}
	model, action, ok := parseGeminiPath(path)
	if !ok {
		return false
	}
	payload, readOK := s.readJSON(w, r, false)
	if !readOK {
		return true
	}
	if action == "countTokens" {
		prepared, err := s.gemini.BuildUpstreamPayload(model, payload)
		if err != nil {
			openAIError(w, http.StatusBadRequest, err.Error(), "invalid_request_error", "")
			return true
		}
		count, err := s.countGLMTokens(r.Context(), prepared.ChatPayload)
		if err != nil {
			s.upstreamError(w, err)
			return true
		}
		writeJSON(w, http.StatusOK, shared.Map{"totalTokens": count})
		return true
	}
	prepared, err := s.gemini.BuildUpstreamPayload(model, payload)
	if err != nil {
		openAIError(w, http.StatusBadRequest, err.Error(), "invalid_request_error", "")
		return true
	}
	if action == "streamGenerateContent" {
		s.streamGeminiContent(w, r, model, prepared.ChatPayload)
		return true
	}
	if action != "generateContent" {
		return false
	}
	prepared.ChatPayload["stream"] = false
	completion, err := s.upstream.Chat(r.Context(), prepared.ChatPayload)
	if err != nil {
		s.upstreamError(w, err)
		return true
	}
	writeJSON(w, http.StatusOK, gemini.ResponseFromUpstream(completion, model))
	return true
}

func (s *Server) handleModels(w http.ResponseWriter, _ *http.Request) {
	data := []any{}
	for _, model := range s.cfg.ModelIDs {
		data = append(data, shared.Map{"id": model, "object": "model", "owned_by": "upstream"})
	}
	writeJSON(w, http.StatusOK, shared.Map{"object": "list", "data": data})
}

func (s *Server) handleChatCompletions(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodPost:
		payload, ok := s.readJSON(w, r, false)
		if !ok {
			return
		}
		chatPayload, requestMessages, err := s.chat.BuildUpstreamPayload(payload)
		if err != nil {
			openAIError(w, http.StatusBadRequest, err.Error(), "invalid_request_error", "")
			return
		}
		if shared.BoolValue(payload["stream"]) {
			s.streamChatCompletion(w, r, payload, chatPayload)
			return
		}
		chatPayload["stream"] = false
		completion, err := s.upstream.Chat(r.Context(), chatPayload)
		if err != nil {
			s.upstreamError(w, err)
			return
		}
		openAICompletion := chat.CompletionFromUpstream(completion, payload, s.cfg.DefaultModel)
		if shared.BoolValue(payload["store"]) {
			s.store.SaveChatCompletion(openAICompletion, chat.StoredMessages(requestMessages, openAICompletion, shared.StringValue(openAICompletion["id"])))
		}
		writeJSON(w, http.StatusOK, openAICompletion)
	case http.MethodGet:
		limit, order := paginationParams(r, 20, "asc")
		model := r.URL.Query().Get("model")
		items := s.store.ListChatCompletions(model, metadataFilters(r))
		writeJSON(w, http.StatusOK, shared.Paginate(items, r.URL.Query().Get("after"), limit, order))
	default:
		methodNotAllowed(w)
	}
}

func (s *Server) handleStoredChatCompletion(w http.ResponseWriter, r *http.Request, rest string) {
	parts := strings.Split(rest, "/")
	id := parts[0]
	if id == "" {
		openAIError(w, http.StatusNotFound, "not found", "invalid_request_error", "")
		return
	}
	if len(parts) == 2 && parts[1] == "messages" {
		if r.Method != http.MethodGet {
			methodNotAllowed(w)
			return
		}
		messages, ok := s.store.ChatCompletionMessagesFor(id)
		if !ok {
			openAIError(w, http.StatusNotFound, "Chat completion not found: "+id, "invalid_request_error", "")
			return
		}
		limit, order := paginationParams(r, 20, "asc")
		writeJSON(w, http.StatusOK, shared.Paginate(messages, r.URL.Query().Get("after"), limit, order))
		return
	}
	if len(parts) != 1 {
		openAIError(w, http.StatusNotFound, "not found", "invalid_request_error", "")
		return
	}
	switch r.Method {
	case http.MethodGet:
		completion, ok := s.store.ChatCompletion(id)
		if !ok {
			openAIError(w, http.StatusNotFound, "Chat completion not found: "+id, "invalid_request_error", "")
			return
		}
		writeJSON(w, http.StatusOK, completion)
	case http.MethodPost:
		payload, ok := s.readJSON(w, r, false)
		if !ok {
			return
		}
		completion, ok := s.store.UpdateChatCompletion(id, payload["metadata"])
		if !ok {
			openAIError(w, http.StatusNotFound, "Chat completion not found: "+id, "invalid_request_error", "")
			return
		}
		writeJSON(w, http.StatusOK, completion)
	case http.MethodDelete:
		if !s.store.DeleteChatCompletion(id) {
			openAIError(w, http.StatusNotFound, "Chat completion not found: "+id, "invalid_request_error", "")
			return
		}
		writeJSON(w, http.StatusOK, shared.Map{"id": id, "object": "chat.completion.deleted", "deleted": true})
	default:
		methodNotAllowed(w)
	}
}

func (s *Server) handleResponses(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		methodNotAllowed(w)
		return
	}
	payload, ok := s.readJSON(w, r, false)
	if !ok {
		return
	}
	prepared, err := s.responses.Prepare(payload)
	if err != nil {
		openAIError(w, statusForLookupError(err), err.Error(), "invalid_request_error", "")
		return
	}
	chatPayload, toolNameMap := s.responses.BuildUpstreamPayload(payload, prepared.Messages)
	if shared.BoolValue(payload["stream"]) {
		s.streamResponse(w, r, payload, prepared, chatPayload, toolNameMap)
		return
	}
	chatPayload["stream"] = false
	completion, err := s.upstream.Chat(r.Context(), chatPayload)
	if err != nil {
		s.upstreamError(w, err)
		return
	}
	outputItems, outputText, finishReason, _ := s.responses.OutputItemsFromChatCompletion(completion, toolNameMap)
	status, incompleteDetails := responses.StatusFromFinishReason(finishReason)
	responseID := shared.NewID("resp")
	response := s.responses.BaseResponse(payload, responseID, shared.NowSeconds(), status, outputItems, outputText, responses.ResponseUsageFromUpstream(completion["usage"]), incompleteDetails)
	s.store.SaveResponse(response, prepared.AllItems, outputItems, payload["store"] != false, prepared.ConversationID, prepared.InputItems)
	writeJSON(w, http.StatusOK, response)
}

func (s *Server) handleStoredResponse(w http.ResponseWriter, r *http.Request, rest string) {
	parts := strings.Split(rest, "/")
	id := parts[0]
	if id == "" {
		openAIError(w, http.StatusNotFound, "not found", "invalid_request_error", "")
		return
	}
	if len(parts) == 2 && parts[1] == "input_items" {
		if r.Method != http.MethodGet {
			methodNotAllowed(w)
			return
		}
		items, ok := s.store.ResponseInput(id)
		if !ok {
			openAIError(w, http.StatusNotFound, "Response not found: "+id, "invalid_request_error", "")
			return
		}
		limit, order := paginationParams(r, 20, "desc")
		writeJSON(w, http.StatusOK, shared.Paginate(items, r.URL.Query().Get("after"), limit, order))
		return
	}
	if len(parts) == 2 && parts[1] == "cancel" {
		if r.Method != http.MethodPost {
			methodNotAllowed(w)
			return
		}
		response, ok := s.store.UpdateResponse(id, func(item shared.Map) shared.Map {
			status := shared.StringValue(item["status"])
			if status == "queued" || status == "in_progress" {
				item["status"] = "cancelled"
				item["completed_at"] = shared.NowSeconds()
			}
			return item
		})
		if !ok {
			openAIError(w, http.StatusNotFound, "Response not found: "+id, "invalid_request_error", "")
			return
		}
		if shared.StringValue(response["status"]) != "cancelled" {
			openAIError(w, http.StatusBadRequest, "Only in-progress background responses can be cancelled", "invalid_request_error", "")
			return
		}
		writeJSON(w, http.StatusOK, response)
		return
	}
	if len(parts) != 1 {
		openAIError(w, http.StatusNotFound, "not found", "invalid_request_error", "")
		return
	}
	switch r.Method {
	case http.MethodGet:
		response, ok := s.store.Response(id)
		if !ok {
			openAIError(w, http.StatusNotFound, "Response not found: "+id, "invalid_request_error", "")
			return
		}
		writeJSON(w, http.StatusOK, response)
	case http.MethodDelete:
		if !s.store.DeleteResponse(id) {
			openAIError(w, http.StatusNotFound, "Response not found: "+id, "invalid_request_error", "")
			return
		}
		writeJSON(w, http.StatusOK, shared.Map{"id": id, "object": "response", "deleted": true})
	default:
		methodNotAllowed(w)
	}
}

func (s *Server) handleInputTokens(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		methodNotAllowed(w)
		return
	}
	payload, ok := s.readJSON(w, r, false)
	if !ok {
		return
	}
	prepared, err := s.responses.Prepare(payload)
	if err != nil {
		openAIError(w, statusForLookupError(err), err.Error(), "invalid_request_error", "")
		return
	}
	model := modelForTokenize(payload, s.cfg.DefaultModel)
	count, err := s.countGLMTokens(r.Context(), shared.Map{"model": model, "messages": chat.MessagesForGLMModel(prepared.Messages, model)})
	if err != nil {
		s.upstreamError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, shared.Map{"object": "response.input_tokens", "input_tokens": count})
}

func (s *Server) handleCompact(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		methodNotAllowed(w)
		return
	}
	payload, ok := s.readJSON(w, r, false)
	if !ok {
		return
	}
	compactPayload := shared.CloneMap(payload)
	if shared.StringValue(compactPayload["instructions"]) == "" {
		compactPayload["instructions"] = "Compact the provided conversation into a concise context summary for future turns."
	}
	if compactPayload["text"] == nil {
		compactPayload["text"] = shared.Map{"format": shared.Map{"type": "text"}}
	}
	prepared, err := s.responses.Prepare(compactPayload)
	if err != nil {
		openAIError(w, statusForLookupError(err), err.Error(), "invalid_request_error", "")
		return
	}
	chatPayload, toolNameMap := s.responses.BuildUpstreamPayload(compactPayload, prepared.Messages)
	chatPayload["stream"] = false
	completion, err := s.upstream.Chat(r.Context(), chatPayload)
	if err != nil {
		s.upstreamError(w, err)
		return
	}
	outputItems, _, finishReason, _ := s.responses.OutputItemsFromChatCompletion(completion, toolNameMap)
	status, _ := responses.StatusFromFinishReason(finishReason)
	writeJSON(w, http.StatusOK, shared.Map{
		"id":         shared.NewID("comp"),
		"created_at": shared.NowSeconds(),
		"object":     "response.compaction",
		"status":     status,
		"output":     shared.PublicItems(outputItems),
		"usage":      responses.ResponseUsageFromUpstream(completion["usage"]),
	})
}

func (s *Server) handleConversations(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		methodNotAllowed(w)
		return
	}
	payload, ok := s.readJSON(w, r, true)
	if !ok {
		return
	}
	id := shared.NewID("conv")
	conversation := shared.Map{"id": id, "object": "conversation", "created_at": shared.NowSeconds(), "metadata": metadataOrEmpty(payload["metadata"])}
	items, err := s.responses.NormalizeInputItems(payload["items"])
	if err != nil {
		openAIError(w, http.StatusBadRequest, err.Error(), "invalid_request_error", "")
		return
	}
	s.store.SaveConversation(conversation, items)
	writeJSON(w, http.StatusOK, conversation)
}

func (s *Server) handleStoredConversation(w http.ResponseWriter, r *http.Request, id string) {
	if id == "" || strings.Contains(id, "/") {
		openAIError(w, http.StatusNotFound, "not found", "invalid_request_error", "")
		return
	}
	switch r.Method {
	case http.MethodGet:
		conversation, ok := s.store.Conversation(id)
		if !ok {
			openAIError(w, http.StatusNotFound, "Conversation not found: "+id, "invalid_request_error", "")
			return
		}
		writeJSON(w, http.StatusOK, conversation)
	case http.MethodPost, http.MethodPatch:
		payload, ok := s.readJSON(w, r, false)
		if !ok {
			return
		}
		conversation, ok := s.store.UpdateConversation(id, payload["metadata"])
		if !ok {
			openAIError(w, http.StatusNotFound, "Conversation not found: "+id, "invalid_request_error", "")
			return
		}
		writeJSON(w, http.StatusOK, conversation)
	case http.MethodDelete:
		if !s.store.DeleteConversation(id) {
			openAIError(w, http.StatusNotFound, "Conversation not found: "+id, "invalid_request_error", "")
			return
		}
		writeJSON(w, http.StatusOK, shared.Map{"id": id, "deleted": true, "object": "conversation.deleted"})
	default:
		methodNotAllowed(w)
	}
}

func (s *Server) countGLMTokens(ctx context.Context, payload shared.Map) (int, error) {
	tokenPayload := shared.CloneMap(payload)
	delete(tokenPayload, "stream")
	result, err := s.upstream.Tokenize(ctx, tokenPayload)
	if err != nil {
		return 0, err
	}
	usage, _ := result["usage"].(map[string]any)
	if usage == nil {
		return 0, nil
	}
	return shared.IntValue(usage["total_tokens"], shared.IntValue(usage["prompt_tokens"], 0)), nil
}

func modelForTokenize(payload shared.Map, defaultModel string) string {
	model := shared.StringValue(payload["model"])
	if model == "" {
		model = defaultModel
	}
	return model
}

func parseGeminiPath(path string) (string, string, bool) {
	const betaPrefix = "/v1beta/models/"
	const v1Prefix = "/v1/models/"
	rest := ""
	switch {
	case strings.HasPrefix(path, betaPrefix):
		rest = strings.TrimPrefix(path, betaPrefix)
	case strings.HasPrefix(path, v1Prefix):
		rest = strings.TrimPrefix(path, v1Prefix)
	default:
		return "", "", false
	}
	model, action, ok := strings.Cut(rest, ":")
	if !ok || model == "" || action == "" || strings.Contains(action, "/") {
		return "", "", false
	}
	return model, action, true
}

func errorTypeForStatus(status int) string {
	if status == http.StatusUnauthorized {
		return "authentication_error"
	}
	if status >= 500 {
		return "server_error"
	}
	return "invalid_request_error"
}

func statusForLookupError(err error) int {
	text := strings.ToLower(err.Error())
	if strings.Contains(text, "not found") {
		return http.StatusNotFound
	}
	return http.StatusBadRequest
}

func paginationParams(r *http.Request, defaultLimit int, defaultOrder string) (int, string) {
	limit := defaultLimit
	if raw := r.URL.Query().Get("limit"); raw != "" {
		if parsed, err := strconv.Atoi(raw); err == nil {
			limit = parsed
		}
	}
	order := r.URL.Query().Get("order")
	if order != "asc" && order != "desc" {
		order = defaultOrder
	}
	return limit, order
}

var metadataFilterRe = regexp.MustCompile(`^metadata\[([^\]]+)\]$`)

func metadataFilters(r *http.Request) map[string]string {
	out := map[string]string{}
	for key, values := range r.URL.Query() {
		match := metadataFilterRe.FindStringSubmatch(key)
		if len(match) == 2 && len(values) > 0 {
			out[match[1]] = values[0]
		}
	}
	return out
}

func metadataOrEmpty(value any) any {
	if value == nil {
		return shared.Map{}
	}
	return value
}
