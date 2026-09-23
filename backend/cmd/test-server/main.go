// Command test-server runs the bundled local test target (Phase 1).
//
// It is the only permitted traffic destination for experiments.
// Listens on :8081 (override with TEST_SERVER_PORT); GET /api/slow
// sleeps SLOW_DELAY_MS (default 200) so load is measurable.
package main

import (
	"log"
	"net/http"
	"os"
	"strconv"
	"time"

	"ddoslab/backend/internal/server"
)

func main() {
	port := os.Getenv("TEST_SERVER_PORT")
	if port == "" {
		port = "8081"
	}

	delay := 200 * time.Millisecond
	if raw := os.Getenv("SLOW_DELAY_MS"); raw != "" {
		if ms, err := strconv.Atoi(raw); err == nil && ms >= 0 {
			delay = time.Duration(ms) * time.Millisecond
		} else {
			log.Printf("invalid SLOW_DELAY_MS=%q, using %v", raw, delay)
		}
	}

	// Defense rate limit for flagged requests (Phase 6). Default 50/s;
	// 0 disables limiting.
	rateLimit := 50.0
	if raw := os.Getenv("RATE_LIMIT_RPS"); raw != "" {
		if n, err := strconv.ParseFloat(raw, 64); err == nil && n >= 0 {
			rateLimit = n
		} else {
			log.Printf("invalid RATE_LIMIT_RPS=%q, using %v", raw, rateLimit)
		}
	}

	mux := server.NewMux(server.Config{SlowDelay: delay, RateLimitRPS: rateLimit})

	addr := ":" + port
	log.Printf("ddoslab test-server listening on %s (slow delay: %v, rate limit: %v/s, 0=off)", addr, delay, rateLimit)
	if err := http.ListenAndServe(addr, mux); err != nil {
		log.Fatal(err)
	}
}
