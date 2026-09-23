package report

import (
	"bytes"
	"strings"
	"testing"
	"time"
)

func testInput() Input {
	start := time.Now().UTC().Add(-30 * time.Second)
	end := time.Now().UTC()
	return Input{
		ID:                 "abc123",
		Status:             "completed",
		StartedAt:          start,
		CompletedAt:        &end,
		TargetURL:          "http://localhost:8081/api/test",
		Defense:            true,
		DurationSeconds:    30,
		RequestsPerSecond:  100,
		Workers:            10,
		TotalRequests:      3000,
		SuccessfulRequests: 2700,
		FailedRequests:     300,
		RPS:                95.5,
		AvgLatencyMs:       12.3,
		P50LatencyMs:       8.1,
		P95LatencyMs:       41.2,
		P99LatencyMs:       88.0,
		Samples:            3000,
		StatusCodes:        map[string]int64{"200": 2700, "429": 300},
	}
}

func TestBuildProducesPDF(t *testing.T) {
	pdf, err := Build(testInput())
	if err != nil {
		t.Fatal(err)
	}
	if len(pdf) == 0 {
		t.Fatal("empty PDF")
	}
	if !bytes.HasPrefix(pdf, []byte("%PDF")) {
		t.Fatal("output does not start with %PDF")
	}
}

func TestObserveDefenseEngaged(t *testing.T) {
	obs := Observe(testInput())
	joined := strings.Join(obs, "\n")
	if !strings.Contains(joined, "429") {
		t.Fatalf("want 429 observation, got:\n%s", joined)
	}
	if !strings.Contains(joined, "Tail latency") {
		t.Fatalf("want tail-latency observation (P95 %.1f >> P50 %.1f), got:\n%s", 41.2, 8.1, joined)
	}
}

func TestObserveBaselineClean(t *testing.T) {
	in := testInput()
	in.Defense = false
	in.FailedRequests = 0
	in.SuccessfulRequests = 3000
	in.TotalRequests = 3000
	in.StatusCodes = map[string]int64{"200": 3000}
	in.P50LatencyMs = 8.0
	in.P95LatencyMs = 12.0
	obs := Observe(in)
	joined := strings.Join(obs, "\n")
	for _, want := range []string{"unprotected baseline", "No failed requests", "stayed even"} {
		if !strings.Contains(joined, want) {
			t.Fatalf("want %q in:\n%s", want, joined)
		}
	}
}

func TestObserveDefenseNeverEngaged(t *testing.T) {
	in := testInput()
	in.FailedRequests = 0
	in.SuccessfulRequests = 3000
	in.TotalRequests = 3000
	in.StatusCodes = map[string]int64{"200": 3000}
	joined := strings.Join(Observe(in), "\n")
	if !strings.Contains(joined, "never engaged") {
		t.Fatalf("want never-engaged note, got:\n%s", joined)
	}
}
