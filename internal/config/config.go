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
	"flag"
	"fmt"
	"strings"
	"time"
)

const (
	DefaultGLMBaseURL = "https://api.z.ai/api"
	DefaultModel      = "glm-5.1"
)

type Config struct {
	Listen                  string
	APITokens               []string
	GLMAPIKey               string
	GLMBaseURL              string
	DefaultModel            string
	ModelIDs                []string
	StoreTTL                time.Duration
	StoreMaxResponses       int
	StoreMaxChatCompletions int
	StoreMaxConversations   int
	StorePruneInterval      time.Duration
	GLMHTTPTimeout          time.Duration
	GLMMaxResponseBodyBytes int64
	GLMMaxIdleConns         int
	GLMMaxIdleConnsPerHost  int
	GLMMaxConnsPerHost      int
	MaxRequestBodyBytes     int64
	ReadHeaderTimeout       time.Duration
	IdleTimeout             time.Duration
	DebugLogBody            bool
	DebugPprof              bool
	VerifySSL               bool
}

func Parse(args []string) (Config, error) {
	fs := flag.NewFlagSet("glm-compatible", flag.ContinueOnError)
	fs.Usage = func() {
		fmt.Fprint(fs.Output(), usage())
	}

	var apiTokenCSV string
	var modelCSV string
	var storeTTLSeconds float64
	var storePruneIntervalSeconds float64
	var timeoutSeconds float64
	var glmMaxResponseBodyBytes int64
	var maxRequestBodyBytes int64
	var readHeaderTimeoutSeconds float64
	var idleTimeoutSeconds float64
	cfg := Config{
		StoreMaxResponses:       1000,
		StoreMaxChatCompletions: 1000,
		StoreMaxConversations:   1000,
		GLMMaxResponseBodyBytes: 32 * 1024 * 1024,
		GLMMaxIdleConns:         200,
		GLMMaxIdleConnsPerHost:  100,
		MaxRequestBodyBytes:     16 * 1024 * 1024,
		VerifySSL:               true,
	}

	fs.StringVar(&cfg.Listen, "listen", ":8080", "HTTP listen address")
	fs.StringVar(&apiTokenCSV, "api-token", "", "comma-separated local bearer token list")
	fs.StringVar(&cfg.GLMAPIKey, "glm-api-key", "", "GLM upstream API key")
	fs.StringVar(&cfg.GLMBaseURL, "glm-base-url", DefaultGLMBaseURL, "GLM upstream base URL")
	fs.StringVar(&cfg.DefaultModel, "glm-model", DefaultModel, "default GLM model")
	fs.StringVar(&modelCSV, "glm-models", "", "comma-separated model IDs exposed by /v1/models")
	fs.Float64Var(&storeTTLSeconds, "store-ttl", 3600, "local store TTL in seconds after last access; 0 disables TTL")
	fs.IntVar(&cfg.StoreMaxResponses, "store-max-responses", cfg.StoreMaxResponses, "maximum stored Responses; 0 means unlimited")
	fs.IntVar(&cfg.StoreMaxChatCompletions, "store-max-chat-completions", cfg.StoreMaxChatCompletions, "maximum stored Chat Completions; 0 means unlimited")
	fs.IntVar(&cfg.StoreMaxConversations, "store-max-conversations", cfg.StoreMaxConversations, "maximum stored Conversations; 0 means unlimited")
	fs.Float64Var(&storePruneIntervalSeconds, "store-prune-interval", 60, "minimum interval between request-path store prune checks in seconds")
	fs.Float64Var(&timeoutSeconds, "glm-http-timeout", 120, "GLM HTTP timeout in seconds")
	fs.Int64Var(&glmMaxResponseBodyBytes, "glm-max-response-body-bytes", cfg.GLMMaxResponseBodyBytes, "maximum GLM upstream non-streaming or error response body size in bytes; 0 means unlimited")
	fs.IntVar(&cfg.GLMMaxIdleConns, "glm-max-idle-conns", cfg.GLMMaxIdleConns, "maximum idle upstream HTTP connections")
	fs.IntVar(&cfg.GLMMaxIdleConnsPerHost, "glm-max-idle-conns-per-host", cfg.GLMMaxIdleConnsPerHost, "maximum idle upstream HTTP connections per host")
	fs.IntVar(&cfg.GLMMaxConnsPerHost, "glm-max-conns-per-host", 0, "maximum upstream HTTP connections per host; 0 means unlimited")
	fs.Int64Var(&maxRequestBodyBytes, "max-request-body-bytes", cfg.MaxRequestBodyBytes, "maximum local request body bytes; 0 means unlimited")
	fs.Float64Var(&readHeaderTimeoutSeconds, "read-header-timeout", 10, "local HTTP read header timeout in seconds")
	fs.Float64Var(&idleTimeoutSeconds, "idle-timeout", 120, "local HTTP idle timeout in seconds")
	fs.BoolVar(&cfg.DebugLogBody, "debug-log-body", false, "log redacted request/response bodies")
	fs.BoolVar(&cfg.DebugPprof, "debug-pprof", false, "enable authenticated /debug/pprof and /debug/vars endpoints")
	fs.BoolVar(&cfg.VerifySSL, "verify-ssl", true, "verify GLM upstream TLS certificates")

	if err := fs.Parse(args); err != nil {
		return Config{}, err
	}

	cfg.APITokens = splitCSV(apiTokenCSV)
	cfg.ModelIDs = splitCSV(modelCSV)
	if cfg.DefaultModel == "" {
		cfg.DefaultModel = DefaultModel
	}
	if len(cfg.ModelIDs) == 0 {
		cfg.ModelIDs = []string{cfg.DefaultModel}
	} else if !contains(cfg.ModelIDs, cfg.DefaultModel) {
		cfg.ModelIDs = append([]string{cfg.DefaultModel}, cfg.ModelIDs...)
	}
	if cfg.GLMBaseURL == "" {
		cfg.GLMBaseURL = DefaultGLMBaseURL
	}
	if storeTTLSeconds < 0 {
		return Config{}, fmt.Errorf("--store-ttl must be non-negative")
	}
	if cfg.StoreMaxResponses < 0 {
		return Config{}, fmt.Errorf("--store-max-responses must be non-negative")
	}
	if cfg.StoreMaxChatCompletions < 0 {
		return Config{}, fmt.Errorf("--store-max-chat-completions must be non-negative")
	}
	if cfg.StoreMaxConversations < 0 {
		return Config{}, fmt.Errorf("--store-max-conversations must be non-negative")
	}
	if storePruneIntervalSeconds < 0 {
		return Config{}, fmt.Errorf("--store-prune-interval must be non-negative")
	}
	if timeoutSeconds <= 0 {
		return Config{}, fmt.Errorf("--glm-http-timeout must be positive")
	}
	if glmMaxResponseBodyBytes < 0 {
		return Config{}, fmt.Errorf("--glm-max-response-body-bytes must be non-negative")
	}
	if cfg.GLMMaxIdleConns < 0 {
		return Config{}, fmt.Errorf("--glm-max-idle-conns must be non-negative")
	}
	if cfg.GLMMaxIdleConnsPerHost < 0 {
		return Config{}, fmt.Errorf("--glm-max-idle-conns-per-host must be non-negative")
	}
	if cfg.GLMMaxConnsPerHost < 0 {
		return Config{}, fmt.Errorf("--glm-max-conns-per-host must be non-negative")
	}
	if maxRequestBodyBytes < 0 {
		return Config{}, fmt.Errorf("--max-request-body-bytes must be non-negative")
	}
	if readHeaderTimeoutSeconds <= 0 {
		return Config{}, fmt.Errorf("--read-header-timeout must be positive")
	}
	if idleTimeoutSeconds <= 0 {
		return Config{}, fmt.Errorf("--idle-timeout must be positive")
	}
	cfg.StoreTTL = time.Duration(storeTTLSeconds * float64(time.Second))
	cfg.StorePruneInterval = time.Duration(storePruneIntervalSeconds * float64(time.Second))
	cfg.GLMHTTPTimeout = time.Duration(timeoutSeconds * float64(time.Second))
	cfg.GLMMaxResponseBodyBytes = glmMaxResponseBodyBytes
	cfg.MaxRequestBodyBytes = maxRequestBodyBytes
	cfg.ReadHeaderTimeout = time.Duration(readHeaderTimeoutSeconds * float64(time.Second))
	cfg.IdleTimeout = time.Duration(idleTimeoutSeconds * float64(time.Second))
	return cfg, nil
}

func usage() string {
	return `Usage:
  glm-compatible [flags]

Example:
  glm-compatible --listen :8080 --api-token sk-local-test --glm-api-key sk-your-glm-key

Flags:
  --api-token string
      comma-separated local bearer token list
  --debug-log-body
      log redacted request/response bodies (default false)
  --debug-pprof
      enable authenticated /debug/pprof/ and /debug/vars endpoints (default false)
  --glm-api-key string
      GLM upstream API key
  --glm-base-url string
      GLM upstream base URL (default https://api.z.ai/api)
  --glm-http-timeout float
      GLM HTTP timeout in seconds (default 120)
  --glm-max-conns-per-host int
      maximum upstream HTTP connections per host; 0 means unlimited (default 0)
  --glm-max-idle-conns int
      maximum idle upstream HTTP connections (default 200)
  --glm-max-idle-conns-per-host int
      maximum idle upstream HTTP connections per host (default 100)
  --glm-max-response-body-bytes int
      maximum GLM upstream non-streaming or error response body size in bytes; 0 means unlimited (default 33554432)
  --glm-model string
      default GLM model (default glm-5.1)
  --glm-models string
      comma-separated model IDs exposed by /v1/models
  --idle-timeout float
      local HTTP idle timeout in seconds (default 120)
  --listen string
      HTTP listen address (default :8080)
  --max-request-body-bytes int
      maximum local request body size in bytes; 0 means unlimited (default 16777216)
  --read-header-timeout float
      local HTTP read header timeout in seconds (default 10)
  --store-max-chat-completions int
      maximum locally stored Chat Completions; 0 means unlimited (default 1000)
  --store-max-conversations int
      maximum locally stored Conversations; 0 means unlimited (default 1000)
  --store-max-responses int
      maximum locally stored Responses; 0 means unlimited (default 1000)
  --store-prune-interval float
      minimum interval between request-path store prune checks in seconds (default 60)
  --store-ttl float
      local store TTL in seconds after last access; 0 disables TTL (default 3600)
  --verify-ssl
      verify GLM upstream TLS certificates (default true)

Container deployment:
  docker-entrypoint.sh maps environment variables to the same flags. See docker.env.example.

Compatible APIs:
  GLM Chat Completions:    POST /paas/v4/chat/completions
  OpenAI Chat Completions: /v1/chat/completions
  OpenAI Responses:        /v1/responses
  OpenAI Conversations:    /v1/conversations
  Anthropic Messages:      /v1/messages
  Gemini Generate Content: /v1beta/models/{model}:generateContent, /v1/models/{model}:generateContent
  Common endpoints:        /v1/models, /health, /healthz/memory
`
}

func splitCSV(value string) []string {
	if strings.TrimSpace(value) == "" {
		return nil
	}
	parts := strings.Split(value, ",")
	out := make([]string, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part != "" {
			out = append(out, part)
		}
	}
	return out
}

func contains(values []string, want string) bool {
	for _, value := range values {
		if value == want {
			return true
		}
	}
	return false
}
