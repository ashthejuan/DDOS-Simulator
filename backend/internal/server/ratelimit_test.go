package server

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestLimiterAllowsBurstThenDenies(t *testing.T) {
	l := NewLimiter(1000, 2) // fast refill, burst of 2
	if !l.Allow() || !l.Allow() {
		t.Fatal("want first two requests allowed")
	}
	if l.Allow() {
		t.Fatal("want third request denied (bucket empty)")
	}
}

func TestLimiterRefillsOverTime(t *testing.T) {
	l := NewLimiter(10, 1)
	if !l.Allow() {
		t.Fatal("want initial token")
	}
	if l.Allow() {
		t.Fatal("want denial while empty")
	}
	time.Sleep(300 * time.Millisecond) // ~3 tokens at 10/s
	if !l.Allow() {
		t.Fatal("want token after refill")
	}
}

func TestDefenseMiddlewareFlagsOnly(t *testing.T) {
	next := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	// Tiny bucket: 1 burst, negligible refill.
	h := DefenseMiddleware(NewLimiter(0.001, 1), next)

	// Unflagged requests always pass, even with an empty bucket.
	for range 5 {
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, httptest.NewRequest("GET", "/api/test", nil))
		if rec.Code != http.StatusOK {
			t.Fatalf("unflagged: got %d, want 200", rec.Code)
		}
	}

	// Flagged: first passes, rest get 429.
	rec := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/api/test", nil)
	req.Header.Set("X-DDoSLab-Defense", "on")
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("flagged first: got %d, want 200", rec.Code)
	}
	rec = httptest.NewRecorder()
	req = httptest.NewRequest("GET", "/api/test", nil)
	req.Header.Set("X-DDoSLab-Defense", "on")
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusTooManyRequests {
		t.Fatalf("flagged excess: got %d, want 429", rec.Code)
	}
	if rec.Header().Get("Retry-After") == "" {
		t.Fatal("want Retry-After header on 429")
	}
}

func TestDefenseMiddlewareNilLimiterPasses(t *testing.T) {
	next := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	h := DefenseMiddleware(nil, next)
	req := httptest.NewRequest("GET", "/api/test", nil)
	req.Header.Set("X-DDoSLab-Defense", "on")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("got %d, want 200 (nil limiter disables limiting)", rec.Code)
	}
}

func TestMuxLimitsFlaggedTraffic(t *testing.T) {
	mux := NewMux(Config{RateLimitRPS: 1})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	flagged := func() *http.Response {
		req, _ := http.NewRequest("GET", srv.URL+"/api/test", nil)
		req.Header.Set("X-DDoSLab-Defense", "on")
		res, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		res.Body.Close()
		return res
	}
	if code := flagged().StatusCode; code != http.StatusOK {
		t.Fatalf("first flagged: got %d, want 200", code)
	}
	if code := flagged().StatusCode; code != http.StatusTooManyRequests {
		t.Fatalf("second flagged: got %d, want 429", code)
	}
	// Unflagged baseline unaffected.
	res, err := http.Get(srv.URL + "/api/test")
	if err != nil {
		t.Fatal(err)
	}
	res.Body.Close()
	if res.StatusCode != http.StatusOK {
		t.Fatalf("unflagged: got %d, want 200", res.StatusCode)
	}
}
