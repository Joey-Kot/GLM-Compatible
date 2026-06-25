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
	"encoding/json"
	"errors"
	"io"
	"log"
	"net/http"

	"glm-compatible/internal/adapters/openai/shared"
	"glm-compatible/internal/debuglog"
	"glm-compatible/internal/upstream/glm"
)

func (s *Server) readJSON(w http.ResponseWriter, r *http.Request, optional bool) (shared.Map, bool) {
	if optional && r.ContentLength == 0 {
		return shared.Map{}, true
	}
	defer r.Body.Close()
	reader := io.Reader(r.Body)
	if s.cfg.MaxRequestBodyBytes > 0 {
		reader = http.MaxBytesReader(w, r.Body, s.cfg.MaxRequestBodyBytes)
	}
	body, err := io.ReadAll(reader)
	if err != nil {
		var maxBytesErr *http.MaxBytesError
		if errors.As(err, &maxBytesErr) {
			openAIError(w, http.StatusRequestEntityTooLarge, "Request body is too large", "invalid_request_error", "")
			return nil, false
		}
		openAIError(w, http.StatusBadRequest, "Request body could not be read", "invalid_request_error", "")
		return nil, false
	}
	if s.cfg.DebugLogBody {
		log.Printf("debug body request method=%s path=%s body=%s", r.Method, r.URL.RequestURI(), debuglog.Body(body))
	}
	if optional && len(body) == 0 {
		return shared.Map{}, true
	}
	decoder := json.NewDecoder(bytes.NewReader(body))
	decoder.UseNumber()
	var payload any
	if err := decoder.Decode(&payload); err != nil {
		if optional && errors.Is(err, io.EOF) {
			return shared.Map{}, true
		}
		openAIError(w, http.StatusBadRequest, "Request body must be JSON", "invalid_request_error", "")
		return nil, false
	}
	var extra any
	if err := decoder.Decode(&extra); !errors.Is(err, io.EOF) {
		openAIError(w, http.StatusBadRequest, "Request body must be JSON", "invalid_request_error", "")
		return nil, false
	}
	result, ok := payload.(map[string]any)
	if !ok {
		openAIError(w, http.StatusBadRequest, "request body must be a JSON object", "invalid_request_error", "")
		return nil, false
	}
	return result, true
}

func (s *Server) upstreamError(w http.ResponseWriter, err error) {
	status := statusFromError(err)
	openAIError(w, status, err.Error(), errorTypeForStatus(status), "")
}

func statusFromError(err error) int {
	var upstream glm.HTTPError
	if errors.As(err, &upstream) {
		return upstream.StatusCode
	}
	return http.StatusBadGateway
}

func writeJSON(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(payload); err != nil {
		log.Printf("write json response: %v", err)
	}
}

func openAIError(w http.ResponseWriter, status int, message, typ, param string) {
	writeJSON(w, status, errorPayload(message, typ))
}

func errorPayload(message, typ string) shared.Map {
	return shared.Map{"error": shared.Map{"message": message, "type": typ, "param": nil, "code": nil}}
}

func methodNotAllowed(w http.ResponseWriter) {
	openAIError(w, http.StatusMethodNotAllowed, "method not allowed", "invalid_request_error", "")
}

func setSSEHeaders(w http.ResponseWriter) {
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
}
