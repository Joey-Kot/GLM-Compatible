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
	"expvar"
	"net/http"
	"net/http/pprof"
	"runtime"
	"strings"

	"glm-compatible/internal/adapters/openai/shared"
	"glm-compatible/internal/debuglog"
)

func (s *Server) handleDiagnostics(w http.ResponseWriter, r *http.Request, path string) bool {
	if r.Method == http.MethodGet && path == "/healthz/memory" {
		s.writeMemoryHealth(w)
		return true
	}
	if !s.cfg.DebugPprof {
		return false
	}
	switch {
	case path == "/debug/vars" && r.Method == http.MethodGet:
		expvar.Handler().ServeHTTP(w, r)
		return true
	case path == "/debug/pprof":
		s.servePprof(w, r, "")
		return true
	case strings.HasPrefix(path, "/debug/pprof/"):
		s.servePprof(w, r, strings.TrimPrefix(path, "/debug/pprof/"))
		return true
	default:
		return false
	}
}

func (s *Server) writeMemoryHealth(w http.ResponseWriter) {
	var mem runtime.MemStats
	runtime.ReadMemStats(&mem)
	writeJSON(w, http.StatusOK, shared.Map{
		"alloc":      mem.Alloc,
		"sys":        mem.Sys,
		"num_gc":     mem.NumGC,
		"goroutines": runtime.NumGoroutine(),
		"store":      s.store.Stats(),
	})
}

func (s *Server) servePprof(w http.ResponseWriter, r *http.Request, name string) {
	switch name {
	case "":
		pprof.Index(w, r)
	case "cmdline":
		pprof.Cmdline(w, r)
	case "profile":
		pprof.Profile(w, r)
	case "symbol":
		pprof.Symbol(w, r)
	case "trace":
		pprof.Trace(w, r)
	default:
		pprof.Handler(name).ServeHTTP(w, r)
	}
}

type debugResponseWriter struct {
	http.ResponseWriter
	status int
	body   bytes.Buffer
}

func newDebugResponseWriter(w http.ResponseWriter) *debugResponseWriter {
	return &debugResponseWriter{ResponseWriter: w}
}

func (w *debugResponseWriter) WriteHeader(status int) {
	if w.status == 0 {
		w.status = status
	}
	w.ResponseWriter.WriteHeader(status)
}

func (w *debugResponseWriter) Write(data []byte) (int, error) {
	if w.status == 0 {
		w.status = http.StatusOK
	}
	remaining := debuglog.MaxBodyBytes + 1 - w.body.Len()
	if remaining > 0 {
		if len(data) > remaining {
			w.body.Write(data[:remaining])
		} else {
			w.body.Write(data)
		}
	}
	return w.ResponseWriter.Write(data)
}

func (w *debugResponseWriter) Flush() {
	if flusher, ok := w.ResponseWriter.(http.Flusher); ok {
		flusher.Flush()
	}
}

func (w *debugResponseWriter) statusCode() int {
	if w.status == 0 {
		return http.StatusOK
	}
	return w.status
}

func (w *debugResponseWriter) bodyBytes() []byte {
	return w.body.Bytes()
}
