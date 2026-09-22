// Package server implements the bundled local test server (Phase 1).
//
// It is the only permitted traffic target: a tiny HTTP server exposing
// GET /api/test and GET /api/slow. /api/slow sleeps for a configurable
// delay so generated load has a measurable effect on latency.
package server

import (
	"encoding/json"
	"net/http"
	"time"
)

// Config tunes the test server.
type Config struct {
	// SlowDelay is how long GET /api/slow sleeps before responding.
	SlowDelay time.Duration
}

// NewMux builds the test-server routes.
func NewMux(cfg Config) *http.ServeMux {
	mux := http.NewServeMux()

	mux.HandleFunc("GET /api/health", func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
	})

	mux.HandleFunc("GET /api/test", func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
	})

	mux.HandleFunc("GET /api/slow", func(w http.ResponseWriter, _ *http.Request) {
		start := time.Now()
		time.Sleep(cfg.SlowDelay)
		writeJSON(w, http.StatusOK, map[string]any{
			"status":   "ok",
			"delay_ms": time.Since(start).Milliseconds(),
		})
	})

	return mux
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}
