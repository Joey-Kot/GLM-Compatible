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

package config

import (
	"reflect"
	"strings"
	"testing"
	"time"
)

func TestParseDefaults(t *testing.T) {
	cfg, err := Parse(nil)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Listen != ":8080" {
		t.Fatalf("Listen = %q", cfg.Listen)
	}
	if cfg.GLMBaseURL != DefaultGLMBaseURL {
		t.Fatalf("GLMBaseURL = %q", cfg.GLMBaseURL)
	}
	if cfg.DefaultModel != DefaultModel {
		t.Fatalf("DefaultModel = %q", cfg.DefaultModel)
	}
	if !reflect.DeepEqual(cfg.ModelIDs, []string{DefaultModel}) {
		t.Fatalf("ModelIDs = %#v", cfg.ModelIDs)
	}
	if cfg.GLMHTTPTimeout != 120*time.Second {
		t.Fatalf("GLMHTTPTimeout = %s", cfg.GLMHTTPTimeout)
	}
	if cfg.GLMMaxResponseBodyBytes != 32*1024*1024 {
		t.Fatalf("GLMMaxResponseBodyBytes = %d", cfg.GLMMaxResponseBodyBytes)
	}
	if cfg.StoreTTL != time.Hour {
		t.Fatalf("StoreTTL = %s", cfg.StoreTTL)
	}
	if cfg.StoreMaxResponses != 1000 {
		t.Fatalf("StoreMaxResponses = %d", cfg.StoreMaxResponses)
	}
	if cfg.StoreMaxChatCompletions != 1000 {
		t.Fatalf("StoreMaxChatCompletions = %d", cfg.StoreMaxChatCompletions)
	}
	if cfg.StoreMaxConversations != 1000 {
		t.Fatalf("StoreMaxConversations = %d", cfg.StoreMaxConversations)
	}
	if cfg.StorePruneInterval != time.Minute {
		t.Fatalf("StorePruneInterval = %s", cfg.StorePruneInterval)
	}
	if cfg.GLMMaxIdleConns != 200 {
		t.Fatalf("GLMMaxIdleConns = %d", cfg.GLMMaxIdleConns)
	}
	if cfg.GLMMaxIdleConnsPerHost != 100 {
		t.Fatalf("GLMMaxIdleConnsPerHost = %d", cfg.GLMMaxIdleConnsPerHost)
	}
	if cfg.GLMMaxConnsPerHost != 0 {
		t.Fatalf("GLMMaxConnsPerHost = %d", cfg.GLMMaxConnsPerHost)
	}
	if cfg.MaxRequestBodyBytes != 16*1024*1024 {
		t.Fatalf("MaxRequestBodyBytes = %d", cfg.MaxRequestBodyBytes)
	}
	if cfg.ReadHeaderTimeout != 10*time.Second {
		t.Fatalf("ReadHeaderTimeout = %s", cfg.ReadHeaderTimeout)
	}
	if cfg.IdleTimeout != 120*time.Second {
		t.Fatalf("IdleTimeout = %s", cfg.IdleTimeout)
	}
	if !cfg.VerifySSL {
		t.Fatal("VerifySSL should default to true")
	}
}

func TestParseCommandLineFlags(t *testing.T) {
	cfg, err := Parse([]string{
		"--listen", "127.0.0.1:9999",
		"--api-token", "sk-a, sk-b ,,",
		"--glm-api-key", "sk-upstream",
		"--glm-base-url", "https://example.test/v1",
		"--glm-model", "glm-4.7-flash",
		"--glm-models", "glm-5.1,glm-4.7-flash",
		"--store-ttl", "1800",
		"--store-max-responses", "11",
		"--store-max-chat-completions", "12",
		"--store-max-conversations", "13",
		"--store-prune-interval", "30",
		"--glm-http-timeout", "2.5",
		"--glm-max-response-body-bytes", "4096",
		"--glm-max-idle-conns", "300",
		"--glm-max-idle-conns-per-host", "150",
		"--glm-max-conns-per-host", "80",
		"--max-request-body-bytes", "2048",
		"--read-header-timeout", "3.5",
		"--idle-timeout", "45",
		"--debug-log-body=true",
		"--debug-pprof=true",
		"--verify-ssl=false",
	})
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(cfg.APITokens, []string{"sk-a", "sk-b"}) {
		t.Fatalf("APITokens = %#v", cfg.APITokens)
	}
	if cfg.GLMAPIKey != "sk-upstream" {
		t.Fatalf("GLMAPIKey = %q", cfg.GLMAPIKey)
	}
	if !reflect.DeepEqual(cfg.ModelIDs, []string{"glm-5.1", "glm-4.7-flash"}) {
		t.Fatalf("ModelIDs = %#v", cfg.ModelIDs)
	}
	if cfg.GLMHTTPTimeout != 2500*time.Millisecond {
		t.Fatalf("GLMHTTPTimeout = %s", cfg.GLMHTTPTimeout)
	}
	if cfg.GLMMaxResponseBodyBytes != 4096 {
		t.Fatalf("GLMMaxResponseBodyBytes = %d", cfg.GLMMaxResponseBodyBytes)
	}
	if cfg.StoreTTL != 1800*time.Second {
		t.Fatalf("StoreTTL = %s", cfg.StoreTTL)
	}
	if cfg.StoreMaxResponses != 11 || cfg.StoreMaxChatCompletions != 12 || cfg.StoreMaxConversations != 13 {
		t.Fatalf("store limits = %#v", cfg)
	}
	if cfg.StorePruneInterval != 30*time.Second {
		t.Fatalf("StorePruneInterval = %s", cfg.StorePruneInterval)
	}
	if cfg.GLMMaxIdleConns != 300 {
		t.Fatalf("GLMMaxIdleConns = %d", cfg.GLMMaxIdleConns)
	}
	if cfg.GLMMaxIdleConnsPerHost != 150 {
		t.Fatalf("GLMMaxIdleConnsPerHost = %d", cfg.GLMMaxIdleConnsPerHost)
	}
	if cfg.GLMMaxConnsPerHost != 80 {
		t.Fatalf("GLMMaxConnsPerHost = %d", cfg.GLMMaxConnsPerHost)
	}
	if cfg.MaxRequestBodyBytes != 2048 {
		t.Fatalf("MaxRequestBodyBytes = %d", cfg.MaxRequestBodyBytes)
	}
	if cfg.ReadHeaderTimeout != 3500*time.Millisecond {
		t.Fatalf("ReadHeaderTimeout = %s", cfg.ReadHeaderTimeout)
	}
	if cfg.IdleTimeout != 45*time.Second {
		t.Fatalf("IdleTimeout = %s", cfg.IdleTimeout)
	}
	if !cfg.DebugLogBody || !cfg.DebugPprof {
		t.Fatalf("boolean flags were not parsed: %#v", cfg)
	}
	if cfg.VerifySSL {
		t.Fatalf("VerifySSL = %t", cfg.VerifySSL)
	}
}

func TestUsageDocumentsFlagsAndEndpoints(t *testing.T) {
	out := usage()
	for _, want := range []string{
		"Usage:\n  glm-compatible [flags]",
		"Example:\n  glm-compatible --listen :8080 --api-token sk-local-test --glm-api-key sk-your-glm-key",
		"--api-token string",
		"--debug-log-body",
		"--debug-pprof",
		"--glm-api-key string",
		"--glm-base-url string",
		"--glm-http-timeout float",
		"--glm-max-conns-per-host int",
		"--glm-max-idle-conns int",
		"--glm-max-idle-conns-per-host int",
		"--glm-max-response-body-bytes int",
		"--glm-model string",
		"--glm-models string",
		"--idle-timeout float",
		"--listen string",
		"--max-request-body-bytes int",
		"--read-header-timeout float",
		"--store-max-chat-completions int",
		"--store-max-conversations int",
		"--store-max-responses int",
		"--store-prune-interval float",
		"--store-ttl float",
		"--verify-ssl",
		"docker-entrypoint.sh maps environment variables to the same flags. See docker.env.example.",
		"GLM Chat Completions:    POST /paas/v4/chat/completions",
		"OpenAI Chat Completions: /v1/chat/completions",
		"OpenAI Responses:        /v1/responses",
		"OpenAI Conversations:    /v1/conversations",
		"Anthropic Messages:      /v1/messages",
		"Gemini Generate Content: /v1beta/models/{model}:generateContent, /v1/models/{model}:generateContent",
		"Common endpoints:        /v1/models, /health, /healthz/memory",
	} {
		if !strings.Contains(out, want) {
			t.Fatalf("usage() missing %q\n%s", want, out)
		}
	}
}

func TestParsePrependsDefaultModelWhenMissingFromModelList(t *testing.T) {
	cfg, err := Parse([]string{"--glm-model", "glm-5", "--glm-models", "glm-4.6"})
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(cfg.ModelIDs, []string{"glm-5", "glm-4.6"}) {
		t.Fatalf("ModelIDs = %#v", cfg.ModelIDs)
	}
}

func TestParseRejectsNonPositiveTimeout(t *testing.T) {
	if _, err := Parse([]string{"--glm-http-timeout", "0"}); err == nil {
		t.Fatal("expected error for zero timeout")
	}
}

func TestParseRejectsInvalidConnectionLimits(t *testing.T) {
	for _, flag := range []string{"--glm-max-idle-conns", "--glm-max-idle-conns-per-host", "--glm-max-conns-per-host"} {
		if _, err := Parse([]string{flag, "-1"}); err == nil {
			t.Fatalf("expected error for %s", flag)
		}
	}
}

func TestParseRejectsInvalidStoreLimits(t *testing.T) {
	for _, flag := range []string{"--store-max-responses", "--store-max-chat-completions", "--store-max-conversations"} {
		if _, err := Parse([]string{flag, "-1"}); err == nil {
			t.Fatalf("expected error for %s", flag)
		}
	}
	for _, flag := range []string{"--store-ttl", "--store-prune-interval", "--glm-max-response-body-bytes", "--max-request-body-bytes"} {
		if _, err := Parse([]string{flag, "-1"}); err == nil {
			t.Fatalf("expected error for %s", flag)
		}
	}
}

func TestParseRejectsNonPositiveServerTimeouts(t *testing.T) {
	for _, flag := range []string{"--read-header-timeout", "--idle-timeout"} {
		if _, err := Parse([]string{flag, "0"}); err == nil {
			t.Fatalf("expected error for %s", flag)
		}
	}
}
