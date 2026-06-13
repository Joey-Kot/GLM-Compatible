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
	Listen                 string
	APITokens              []string
	GLMAPIKey              string
	GLMBaseURL             string
	DefaultModel           string
	ModelIDs               []string
	GLMHTTPTimeout         time.Duration
	GLMMaxIdleConns        int
	GLMMaxIdleConnsPerHost int
	GLMMaxConnsPerHost     int
	ReadHeaderTimeout      time.Duration
	IdleTimeout            time.Duration
	DebugLogBody           bool
	VerifySSL              bool
}

func Parse(args []string) (Config, error) {
	fs := flag.NewFlagSet("glm-compatible", flag.ContinueOnError)

	var apiTokenCSV string
	var modelCSV string
	var timeoutSeconds float64
	var readHeaderTimeoutSeconds float64
	var idleTimeoutSeconds float64
	cfg := Config{
		GLMMaxIdleConns:        200,
		GLMMaxIdleConnsPerHost: 100,
		VerifySSL:              true,
	}

	fs.StringVar(&cfg.Listen, "listen", ":8080", "HTTP listen address")
	fs.StringVar(&apiTokenCSV, "api-token", "", "comma-separated local bearer token list")
	fs.StringVar(&cfg.GLMAPIKey, "glm-api-key", "", "GLM upstream API key")
	fs.StringVar(&cfg.GLMBaseURL, "glm-base-url", DefaultGLMBaseURL, "GLM upstream base URL")
	fs.StringVar(&cfg.DefaultModel, "glm-model", DefaultModel, "default GLM model")
	fs.StringVar(&modelCSV, "glm-models", "", "comma-separated model IDs exposed by /v1/models")
	fs.Float64Var(&timeoutSeconds, "glm-http-timeout", 120, "GLM HTTP timeout in seconds")
	fs.IntVar(&cfg.GLMMaxIdleConns, "glm-max-idle-conns", cfg.GLMMaxIdleConns, "maximum idle upstream HTTP connections")
	fs.IntVar(&cfg.GLMMaxIdleConnsPerHost, "glm-max-idle-conns-per-host", cfg.GLMMaxIdleConnsPerHost, "maximum idle upstream HTTP connections per host")
	fs.IntVar(&cfg.GLMMaxConnsPerHost, "glm-max-conns-per-host", 0, "maximum upstream HTTP connections per host; 0 means unlimited")
	fs.Float64Var(&readHeaderTimeoutSeconds, "read-header-timeout", 10, "local HTTP read header timeout in seconds")
	fs.Float64Var(&idleTimeoutSeconds, "idle-timeout", 120, "local HTTP idle timeout in seconds")
	fs.BoolVar(&cfg.DebugLogBody, "debug-log-body", false, "log redacted request/response bodies")
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
	if timeoutSeconds <= 0 {
		return Config{}, fmt.Errorf("--glm-http-timeout must be positive")
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
	if readHeaderTimeoutSeconds <= 0 {
		return Config{}, fmt.Errorf("--read-header-timeout must be positive")
	}
	if idleTimeoutSeconds <= 0 {
		return Config{}, fmt.Errorf("--idle-timeout must be positive")
	}
	cfg.GLMHTTPTimeout = time.Duration(timeoutSeconds * float64(time.Second))
	cfg.ReadHeaderTimeout = time.Duration(readHeaderTimeoutSeconds * float64(time.Second))
	cfg.IdleTimeout = time.Duration(idleTimeoutSeconds * float64(time.Second))
	return cfg, nil
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
