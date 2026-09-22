package database

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"

	"ddoslab/backend/internal/experiment"
)

// TestIntegrationRoundTrip exercises Save/Load/History against a real
// MongoDB. It only runs when MONGO_TEST_URI is set (e.g. the compose
// mongodb on localhost:27017), so `go test ./...` stays dependency-free.
//
//	MONGO_TEST_URI=mongodb://localhost:27017 go test ./internal/database/ -run Integration -v
func TestIntegrationRoundTrip(t *testing.T) {
	uri := os.Getenv("MONGO_TEST_URI")
	if uri == "" {
		t.Skip("MONGO_TEST_URI unset; skipping mongo integration test")
	}
	ctx := context.Background()
	store, err := Connect(ctx, uri)
	if err != nil {
		t.Skipf("mongo unreachable at %s: %v", uri, err)
	}

	id := fmt.Sprintf("integ-%d", time.Now().UnixNano())
	cpu := 3.5
	rss := int64(8 << 20)
	want := experiment.Result{
		Exp: experiment.Experiment{
			ID:                 id,
			Status:             experiment.StatusCompleted,
			Config:             experiment.Config{Endpoint: "test", DurationSeconds: 2, RequestsPerSecond: 10, Workers: 1},
			TargetURL:          "http://test-server:8081/api/test",
			StartedAt:          time.Now().UTC().Add(-2 * time.Second),
			TotalRequests:      19,
			SuccessfulRequests: 19,
		},
		Metrics: experiment.View{
			TotalRequests: 19,
			StatusCodes:   map[string]int64{"200": 19},
			RPS:           9.5,
			AvgLatencyMs:  2.1,
			P50LatencyMs:  2.0,
			P95LatencyMs:  3.0,
			P99LatencyMs:  4.0,
			Samples:       19,
		},
		CPUPercent:     &cpu,
		MemoryRSSBytes: &rss,
	}
	now := time.Now().UTC()
	want.Exp.CompletedAt = &now

	t.Cleanup(func() {
		_, _ = store.experiments.DeleteOne(ctx, map[string]string{"_id": id})
		_, _ = store.metrics.DeleteMany(ctx, map[string]string{"experiment_id": id})
	})

	if err := store.SaveResult(ctx, want); err != nil {
		t.Fatalf("save: %v", err)
	}

	got, err := store.LoadResult(ctx, id)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if got.Exp.ID != id || got.Exp.Status != experiment.StatusCompleted {
		t.Fatalf("load mismatch: %+v", got.Exp)
	}
	if got.Metrics.TotalRequests != 19 || got.Metrics.P95LatencyMs != 3.0 {
		t.Fatalf("metrics mismatch: %+v", got.Metrics)
	}
	if got.CPUPercent == nil || *got.CPUPercent != 3.5 {
		t.Fatalf("cpu mismatch: %+v", got)
	}

	hist, err := store.History(ctx)
	if err != nil {
		t.Fatalf("history: %v", err)
	}
	found := false
	for _, h := range hist {
		if h.Exp.ID == id {
			found = true
		}
	}
	if !found {
		t.Fatal("saved experiment missing from history")
	}

	if _, err := store.LoadResult(ctx, "does-not-exist"); err == nil {
		t.Fatal("want not-found error")
	} else if _, ok := err.(experiment.ErrNotFound); !ok {
		t.Fatalf("want ErrNotFound, got %T", err)
	}
}
