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

package main

import (
	"log"
	"net/http"
	"os"

	"glm-compatible/internal/config"
	"glm-compatible/internal/httpapi"
	"glm-compatible/internal/state"
	"glm-compatible/internal/upstream/glm"
)

func main() {
	cfg, err := config.Parse(os.Args[1:])
	if err != nil {
		log.Fatal(err)
	}
	upstream := glm.NewWithTransportConfig(cfg.GLMBaseURL, cfg.GLMAPIKey, cfg.GLMHTTPTimeout, cfg.VerifySSL, glm.TransportConfig{
		MaxIdleConns:        cfg.GLMMaxIdleConns,
		MaxIdleConnsPerHost: cfg.GLMMaxIdleConnsPerHost,
		MaxConnsPerHost:     cfg.GLMMaxConnsPerHost,
	})
	defer upstream.CloseIdleConnections()
	upstream.DebugLogBody = cfg.DebugLogBody
	upstream.MaxResponseBodyBytes = cfg.GLMMaxResponseBodyBytes
	handler := httpapi.New(cfg, upstream, state.NewWithLimits(state.Limits{
		MaxResponses:       cfg.StoreMaxResponses,
		MaxChatCompletions: cfg.StoreMaxChatCompletions,
		MaxConversations:   cfg.StoreMaxConversations,
		TTL:                cfg.StoreTTL,
		PruneInterval:      cfg.StorePruneInterval,
	}))
	server := newHTTPServer(cfg, handler)
	log.Printf("listening on %s", cfg.Listen)
	if err := server.ListenAndServe(); err != nil {
		log.Fatal(err)
	}
}

func newHTTPServer(cfg config.Config, handler http.Handler) *http.Server {
	return &http.Server{
		Addr:              cfg.Listen,
		Handler:           handler,
		ReadHeaderTimeout: cfg.ReadHeaderTimeout,
		IdleTimeout:       cfg.IdleTimeout,
	}
}
