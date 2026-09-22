package experiment

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func testHandler(t *testing.T) (*Handler, *Service) {
	t.Helper()
	svc, _ := testTarget(t, nil)
	h := NewHandler(svc)
	mux := http.NewServeMux()
	h.RegisterRoutes(mux)
	return h, svc
}

func decodeBody(t *testing.T, res *http.Response, v any) {
	t.Helper()
	defer res.Body.Close()
	if err := json.NewDecoder(res.Body).Decode(v); err != nil {
		t.Fatalf("decode: %v", err)
	}
}

func TestHTTPMetricsLive(t *testing.T) {
	h, svc := testHandler(t)
	mux := http.NewServeMux()
	h.RegisterRoutes(mux)

	res := doRequest(t, mux, "POST", "/api/experiments",
		`{"endpoint":"test","duration_seconds":60,"requests_per_second":20,"workers":2}`)
	var created Experiment
	decodeBody(t, res, &created)
	t.Cleanup(func() { _, _ = svc.Stop(created.ID) })

	time.Sleep(2 * time.Second) // ~40 reqs @ 20rps; 5s window => RPS ≈ 8

	res = doRequest(t, mux, "GET", "/api/experiments/"+created.ID+"/metrics", "")
	if res.StatusCode != http.StatusOK {
		t.Fatalf("metrics: got %d, want 200", res.StatusCode)
	}
	var m MetricsResponse
	decodeBody(t, res, &m)

	if m.ExperimentID != created.ID || m.Status != StatusRunning {
		t.Fatalf("identity: %+v", m)
	}
	if m.TotalRequests < 30 {
		t.Fatalf("total=%d, want >= 30 after ~2s @ 20rps", m.TotalRequests)
	}
	if m.StatusCodes["200"] != m.TotalRequests {
		t.Fatalf("codes=%v total=%d", m.StatusCodes, m.TotalRequests)
	}
	if m.RPS < 5 || m.RPS > 15 {
		t.Fatalf("rps=%v, want ~8 (40 reqs in 5s window)", m.RPS)
	}
	if m.AvgLatencyMs <= 0 || m.P50LatencyMs <= 0 || m.P95LatencyMs <= 0 || m.P99LatencyMs <= 0 {
		t.Fatalf("non-positive latency: %+v", m)
	}
	if m.Samples != int(m.TotalRequests) {
		t.Fatalf("samples=%d total=%d", m.Samples, m.TotalRequests)
	}
	if m.ElapsedSeconds <= 0 {
		t.Fatalf("elapsed=%v", m.ElapsedSeconds)
	}
	// Stats probe hits the stub test-server: shape present (values may be
	// null on darwin, populated on Linux) — assert only that we got here.
}

func TestHTTPMetricsUnknown(t *testing.T) {
	h, _ := testHandler(t)
	mux := http.NewServeMux()
	h.RegisterRoutes(mux)

	res := doRequest(t, mux, "GET", "/api/experiments/nope/metrics", "")
	defer res.Body.Close()
	if res.StatusCode != http.StatusNotFound {
		t.Fatalf("got %d, want 404", res.StatusCode)
	}
}

func TestHTTPCreateListGetStop(t *testing.T) {
	h, _ := testHandler(t)
	mux := http.NewServeMux()
	h.RegisterRoutes(mux)

	// Create.
	res := doRequest(t, mux, "POST", "/api/experiments",
		`{"endpoint":"test","duration_seconds":60,"requests_per_second":5,"workers":1}`)
	if res.StatusCode != http.StatusCreated {
		t.Fatalf("create: got %d, want 201", res.StatusCode)
	}
	var created Experiment
	decodeBody(t, res, &created)
	if created.ID == "" || created.Status != StatusRunning {
		t.Fatalf("unexpected created: %+v", created)
	}

	// List contains it.
	res = doRequest(t, mux, "GET", "/api/experiments", "")
	var list struct {
		Experiments []Experiment `json:"experiments"`
	}
	decodeBody(t, res, &list)
	if len(list.Experiments) != 1 || list.Experiments[0].ID != created.ID {
		t.Fatalf("unexpected list: %+v", list)
	}

	// Get one.
	res = doRequest(t, mux, "GET", "/api/experiments/"+created.ID, "")
	var one Experiment
	decodeBody(t, res, &one)
	if one.ID != created.ID {
		t.Fatalf("unexpected get: %+v", one)
	}

	// Stop.
	res = doRequest(t, mux, "POST", "/api/experiments/"+created.ID+"/stop", "")
	var stopped Experiment
	decodeBody(t, res, &stopped)
	if stopped.Status != StatusStopped {
		t.Fatalf("unexpected stop: %+v", stopped)
	}
}

func TestHTTPErrors(t *testing.T) {
	h, _ := testHandler(t)
	mux := http.NewServeMux()
	h.RegisterRoutes(mux)

	cases := []struct {
		name   string
		method string
		path   string
		body   string
		want   int
	}{
		{"bad json", "POST", "/api/experiments", "{oops", http.StatusBadRequest},
		{"bad endpoint", "POST", "/api/experiments", `{"endpoint":"x","duration_seconds":5,"requests_per_second":5,"workers":1}`, http.StatusBadRequest},
		{"zero workers", "POST", "/api/experiments", `{"endpoint":"test","duration_seconds":5,"requests_per_second":5,"workers":0}`, http.StatusBadRequest},
		{"unknown get", "GET", "/api/experiments/nope", "", http.StatusNotFound},
		{"unknown stop", "POST", "/api/experiments/nope/stop", "", http.StatusNotFound},
	}
	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			res := doRequest(t, mux, tt.method, tt.path, tt.body)
			defer res.Body.Close()
			if res.StatusCode != tt.want {
				t.Fatalf("got %d, want %d", res.StatusCode, tt.want)
			}
			var errBody map[string]string
			if err := json.NewDecoder(res.Body).Decode(&errBody); err != nil {
				t.Fatalf("decode: %v", err)
			}
			if errBody["error"] == "" {
				t.Fatal("want JSON error body")
			}
		})
	}

	// Stop twice -> 409.
	res := doRequest(t, mux, "POST", "/api/experiments",
		`{"endpoint":"test","duration_seconds":60,"requests_per_second":1,"workers":1}`)
	var created Experiment
	decodeBody(t, res, &created)
	_ = doRequest(t, mux, "POST", "/api/experiments/"+created.ID+"/stop", "").Body.Close()
	res = doRequest(t, mux, "POST", "/api/experiments/"+created.ID+"/stop", "")
	defer res.Body.Close()
	if res.StatusCode != http.StatusConflict {
		t.Fatalf("second stop: got %d, want 409", res.StatusCode)
	}
}

func doRequest(t *testing.T, mux http.Handler, method, path, body string) *http.Response {
	t.Helper()
	var req *http.Request
	if body == "" {
		req = httptest.NewRequest(method, path, nil)
	} else {
		req = httptest.NewRequest(method, path, strings.NewReader(body))
	}
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	return rec.Result()
}
