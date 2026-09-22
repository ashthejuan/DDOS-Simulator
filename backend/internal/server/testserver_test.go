package server

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestHealth(t *testing.T) {
	srv := httptest.NewServer(NewMux(Config{}))
	defer srv.Close()

	var body map[string]string
	if err := getJSON(t, srv.URL+"/api/health", &body); err != nil {
		t.Fatal(err)
	}
	if body["status"] != "ok" {
		t.Fatalf("got %v, want status=ok", body)
	}
}

func TestTestEndpoint(t *testing.T) {
	srv := httptest.NewServer(NewMux(Config{}))
	defer srv.Close()

	var body map[string]string
	if err := getJSON(t, srv.URL+"/api/test", &body); err != nil {
		t.Fatal(err)
	}
	if body["status"] != "ok" {
		t.Fatalf("got %v, want status=ok", body)
	}
}

func TestSlowEndpointSleeps(t *testing.T) {
	const delay = 50 * time.Millisecond
	srv := httptest.NewServer(NewMux(Config{SlowDelay: delay}))
	defer srv.Close()

	start := time.Now()
	var body map[string]any
	if err := getJSON(t, srv.URL+"/api/slow", &body); err != nil {
		t.Fatal(err)
	}
	elapsed := time.Since(start)

	if body["status"] != "ok" {
		t.Fatalf("got %v, want status=ok", body)
	}
	if elapsed < delay {
		t.Fatalf("elapsed %v < delay %v", elapsed, delay)
	}
	if got, ok := body["delay_ms"].(float64); !ok || got < float64(delay.Milliseconds()) {
		t.Fatalf("delay_ms=%v, want >= %v", body["delay_ms"], delay.Milliseconds())
	}
}

func TestMethodNotAllowed(t *testing.T) {
	srv := httptest.NewServer(NewMux(Config{}))
	defer srv.Close()

	res, err := http.Post(srv.URL+"/api/test", "application/json", nil)
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusMethodNotAllowed {
		t.Fatalf("got %d, want 405", res.StatusCode)
	}
}

func TestNotFound(t *testing.T) {
	srv := httptest.NewServer(NewMux(Config{}))
	defer srv.Close()

	res, err := http.Get(srv.URL + "/api/nope")
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusNotFound {
		t.Fatalf("got %d, want 404", res.StatusCode)
	}
}

func getJSON(t *testing.T, url string, v any) error {
	t.Helper()
	res, err := http.Get(url)
	if err != nil {
		return err
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		t.Fatalf("GET %s: got %d, want 200", url, res.StatusCode)
	}
	return json.NewDecoder(res.Body).Decode(v)
}
