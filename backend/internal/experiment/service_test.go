package experiment

import (
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"ddoslab/backend/internal/server"
)

// testTarget spins up the real Phase 1 test server and points the service
// at it (127.0.0.1 is allowlisted). Returns the service and a hit counter.
func testTarget(t *testing.T, allowed []string) (*Service, *atomic.Int64) {
	t.Helper()
	var hits atomic.Int64
	mux := server.NewMux(server.Config{})
	wrapped := http.NewServeMux()
	wrapped.Handle("/api/test", countMiddleware(&hits, mux))
	wrapped.Handle("/api/slow", countMiddleware(&hits, mux))
	wrapped.Handle("/api/health", mux)
	wrapped.Handle("/api/stats", mux)
	srv := httptest.NewServer(wrapped)
	t.Cleanup(srv.Close)

	t.Setenv("TARGET_BASE_URL", srv.URL)
	return NewService(allowed), &hits
}

func countMiddleware(hits *atomic.Int64, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits.Add(1)
		next.ServeHTTP(w, r)
	})
}

func waitDone(t *testing.T, done <-chan struct{}, timeout time.Duration) {
	t.Helper()
	select {
	case <-done:
	case <-time.After(timeout):
		t.Fatal("timed out waiting for experiment to finish")
	}
}

func TestRunCompletesAndHitsTarget(t *testing.T) {
	svc, hits := testTarget(t, nil)

	exp, err := svc.Create(Config{Endpoint: "test", DurationSeconds: 1, RequestsPerSecond: 10, Workers: 2})
	if err != nil {
		t.Fatal(err)
	}
	if exp.Status != StatusRunning {
		t.Fatalf("status=%q, want running", exp.Status)
	}
	if exp.TargetURL == "" {
		t.Fatal("empty target URL")
	}

	done, ok := svc.Done(exp.ID)
	if !ok {
		t.Fatal("missing done channel")
	}
	waitDone(t, done, 5*time.Second)

	got, err := svc.Get(exp.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got.Status != StatusCompleted {
		t.Fatalf("status=%q, want completed", got.Status)
	}
	if got.CompletedAt == nil {
		t.Fatal("completed_at not set")
	}
	if got.TotalRequests < 5 {
		t.Fatalf("total=%d, want >= 5 (1s @ 10rps)", got.TotalRequests)
	}
	if got.SuccessfulRequests != got.TotalRequests || got.FailedRequests != 0 {
		t.Fatalf("ok=%d fail=%d total=%d", got.SuccessfulRequests, got.FailedRequests, got.TotalRequests)
	}
	if hits.Load() != got.TotalRequests {
		t.Fatalf("target saw %d hits, experiment counted %d", hits.Load(), got.TotalRequests)
	}
}

func TestStopFinishesCleanly(t *testing.T) {
	svc, _ := testTarget(t, nil)

	exp, err := svc.Create(Config{Endpoint: "slow", DurationSeconds: 300, RequestsPerSecond: 5, Workers: 1})
	if err != nil {
		t.Fatal(err)
	}
	time.Sleep(300 * time.Millisecond) // let workers start

	stopped, err := svc.Stop(exp.ID)
	if err != nil {
		t.Fatal(err)
	}
	if stopped.Status != StatusStopped {
		t.Fatalf("status=%q, want stopped", stopped.Status)
	}

	done, _ := svc.Done(exp.ID)
	waitDone(t, done, 5*time.Second)

	// Second stop must fail: already finished.
	if _, err := svc.Stop(exp.ID); err == nil {
		t.Fatal("second stop should fail")
	}
}

func TestCreateRejectsBadConfig(t *testing.T) {
	svc, _ := testTarget(t, nil)
	if _, err := svc.Create(Config{Endpoint: "nuke", DurationSeconds: 10, RequestsPerSecond: 10, Workers: 1}); err == nil {
		t.Fatal("bad endpoint should be rejected")
	}
	if _, err := svc.Create(Config{Endpoint: "test", DurationSeconds: 0, RequestsPerSecond: 10, Workers: 1}); err == nil {
		t.Fatal("zero duration should be rejected")
	}
}

func TestCreateRejectsDisallowedTarget(t *testing.T) {
	svc, _ := testTarget(t, []string{"test-server"}) // no loopback
	if _, err := svc.Create(Config{Endpoint: "test", DurationSeconds: 5, RequestsPerSecond: 5, Workers: 1}); err == nil {
		t.Fatal("loopback target should be rejected under narrow allowlist")
	}
}

func TestGetUnknown(t *testing.T) {
	svc, _ := testTarget(t, nil)
	if _, err := svc.Get("nope"); err == nil {
		t.Fatal("want not-found error")
	}
	if _, err := svc.Stop("nope"); err == nil {
		t.Fatal("want not-found error")
	}
}

func TestListNewestFirst(t *testing.T) {
	svc, _ := testTarget(t, nil)
	a, _ := svc.Create(Config{Endpoint: "test", DurationSeconds: 60, RequestsPerSecond: 1, Workers: 1})
	time.Sleep(10 * time.Millisecond)
	b, _ := svc.Create(Config{Endpoint: "test", DurationSeconds: 60, RequestsPerSecond: 1, Workers: 1})
	t.Cleanup(func() { _, _ = svc.Stop(a.ID); _, _ = svc.Stop(b.ID) })

	list := svc.List()
	if len(list) != 2 || list[0].ID != b.ID || list[1].ID != a.ID {
		t.Fatalf("want newest-first [%s %s], got %v", b.ID, a.ID, list)
	}
}
