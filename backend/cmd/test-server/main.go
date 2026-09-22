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

	mux := server.NewMux(server.Config{SlowDelay: delay})

	addr := ":" + port
	log.Printf("ddoslab test-server listening on %s (slow delay: %v)", addr, delay)
	if err := http.ListenAndServe(addr, mux); err != nil {
		log.Fatal(err)
	}
}
