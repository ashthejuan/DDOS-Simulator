// Command server is the DDoSLab API.
//
// It serves the static Oat UI from web/, exposes GET /api/health and the
// experiment API (Phase 2). Listens on :8080 (override with PORT env).
package main

import (
	"encoding/json"
	"log"
	"net/http"
	"os"

	"ddoslab/backend/internal/experiment"
	"ddoslab/backend/internal/server"
)

func main() {
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	webDir := resolveWebDir()

	mux := http.NewServeMux()

	mux.HandleFunc("GET /api/health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
	})

	// Experiment API (Phase 2): workers hit TARGET_BASE_URL (test-server).
	svc := experiment.NewService(server.AllowedHostsFromEnv())
	experiment.NewHandler(svc).RegisterRoutes(mux)

	// Static UI: served from web/ at /. Must be registered last (catch-all).
	fileServer := http.FileServer(http.Dir(webDir))
	mux.Handle("GET /", fileServer)

	addr := ":" + port
	log.Printf("ddoslab api listening on %s (web dir: %s)", addr, webDir)
	if err := http.ListenAndServe(addr, withLogging(mux)); err != nil {
		log.Fatal(err)
	}
}

// resolveWebDir finds the web/ directory both for local `go run`
// (cwd = backend/) and for Docker (cwd = /app).
func resolveWebDir() string {
	if dir := os.Getenv("WEB_DIR"); dir != "" {
		return dir
	}
	candidates := []string{"web", "../web", "/app/web", "./web"}
	for _, c := range candidates {
		if st, err := os.Stat(c); err == nil && st.IsDir() {
			return c
		}
	}
	// Default; http.FileServer will 404 with a clear log line.
	return "web"
}

func withLogging(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		log.Printf("%s %s", r.Method, r.URL.Path)
		next.ServeHTTP(w, r)
	})
}
