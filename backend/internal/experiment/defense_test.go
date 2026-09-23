package experiment

import (
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"
)

// headerTarget serves 200 while recording whether the defense header arrived.
func headerTarget(t *testing.T, flagged *atomic.Int64, total *atomic.Int64) string {
	t.Helper()
	mux := http.NewServeMux()
	mux.HandleFunc("/api/test", func(w http.ResponseWriter, r *http.Request) {
		total.Add(1)
		if r.Header.Get(DefenseHeader) == DefenseHeaderValue {
			flagged.Add(1)
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"status":"ok"}`))
	})
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	return srv.URL
}

func TestDefenseHeaderSentWhenEnabled(t *testing.T) {
	var flagged, total atomic.Int64
	t.Setenv("TARGET_BASE_URL", headerTarget(t, &flagged, &total))
	svc := NewService(nil, nil)

	exp, err := svc.Create(Config{Endpoint: "test", Defense: true, DurationSeconds: 1, RequestsPerSecond: 10, Workers: 2})
	if err != nil {
		t.Fatal(err)
	}
	done, _ := svc.Done(exp.ID)
	waitDone(t, done, 5*time.Second)

	if total.Load() == 0 {
		t.Fatal("no requests fired")
	}
	if flagged.Load() != total.Load() {
		t.Fatalf("flagged=%d total=%d: want every request flagged", flagged.Load(), total.Load())
	}
}

func TestDefenseHeaderAbsentWhenDisabled(t *testing.T) {
	var flagged, total atomic.Int64
	t.Setenv("TARGET_BASE_URL", headerTarget(t, &flagged, &total))
	svc := NewService(nil, nil)

	exp, err := svc.Create(Config{Endpoint: "test", Defense: false, DurationSeconds: 1, RequestsPerSecond: 10, Workers: 2})
	if err != nil {
		t.Fatal(err)
	}
	done, _ := svc.Done(exp.ID)
	waitDone(t, done, 5*time.Second)

	if total.Load() == 0 {
		t.Fatal("no requests fired")
	}
	if flagged.Load() != 0 {
		t.Fatalf("flagged=%d: baseline runs must not set the header", flagged.Load())
	}
}

func TestDefenseConfigRoundTrip(t *testing.T) {
	var flagged, total atomic.Int64
	t.Setenv("TARGET_BASE_URL", headerTarget(t, &flagged, &total))
	svc := NewService(nil, nil)
	h := NewHandler(svc)
	mux := http.NewServeMux()
	h.RegisterRoutes(mux)

	// Create via HTTP with defense:true (the UI payload shape).
	res := doRequest(t, mux, "POST", "/api/experiments",
		`{"endpoint":"test","defense":true,"duration_seconds":1,"requests_per_second":5,"workers":1}`)
	if res.StatusCode != http.StatusCreated {
		t.Fatalf("create: got %d, want 201", res.StatusCode)
	}
	var created Experiment
	decodeBody(t, res, &created)
	if !created.Config.Defense {
		t.Fatal("defense flag lost in create round-trip")
	}
	done, _ := svc.Done(created.ID)
	waitDone(t, done, 5*time.Second)
	if flagged.Load() == 0 {
		t.Fatal("HTTP-created defended run sent no flagged requests")
	}
}
