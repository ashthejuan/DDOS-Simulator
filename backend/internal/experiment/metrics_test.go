package experiment

import (
	"testing"
	"time"
)

func TestPercentile(t *testing.T) {
	vals := []float64{1, 2, 3, 4, 5, 6, 7, 8, 9, 10}
	if got := percentile(vals, 50); got != 5 {
		t.Fatalf("p50=%v, want 5", got)
	}
	if got := percentile(vals, 95); got != 10 {
		t.Fatalf("p95=%v, want 10", got)
	}
	if got := percentile(vals, 99); got != 10 {
		t.Fatalf("p99=%v, want 10", got)
	}
	if got := percentile(nil, 50); got != 0 {
		t.Fatalf("empty p50=%v, want 0", got)
	}
	if got := percentile([]float64{7}, 99); got != 7 {
		t.Fatalf("single p99=%v, want 7", got)
	}
}

func TestRecordAndSnapshot(t *testing.T) {
	var m Metrics
	for i := 0; i < 100; i++ {
		m.Record(200, 10*time.Millisecond)
	}
	for i := 0; i < 5; i++ {
		m.Record(500, 50*time.Millisecond)
	}

	v := m.Snapshot()
	if v.TotalRequests != 105 {
		t.Fatalf("total=%d", v.TotalRequests)
	}
	if v.StatusCodes["200"] != 100 || v.StatusCodes["500"] != 5 {
		t.Fatalf("codes=%v", v.StatusCodes)
	}
	if v.RPS < 1 {
		t.Fatalf("rps=%v, want > 0 right after recording", v.RPS)
	}
	// avg = (100*10 + 5*50)/105 ≈ 11.9
	if v.AvgLatencyMs < 11 || v.AvgLatencyMs > 13 {
		t.Fatalf("avg=%v", v.AvgLatencyMs)
	}
	if v.P50LatencyMs != 10 || v.P99LatencyMs != 50 {
		t.Fatalf("p50=%v p99=%v", v.P50LatencyMs, v.P99LatencyMs)
	}
	if v.Samples != 105 {
		t.Fatalf("samples=%d", v.Samples)
	}
}

func TestReservoirBoundsMemory(t *testing.T) {
	var m Metrics
	for i := 0; i < 3*maxSamples; i++ {
		m.Record(200, time.Millisecond)
	}
	v := m.Snapshot()
	if v.Samples != maxSamples {
		t.Fatalf("samples=%d, want cap %d", v.Samples, maxSamples)
	}
	if v.AvgLatencyMs != 1 {
		t.Fatalf("avg=%v, want 1 (uniform input)", v.AvgLatencyMs)
	}
}

func TestEmptySnapshot(t *testing.T) {
	var m Metrics
	v := m.Snapshot()
	if v.RPS != 0 || v.AvgLatencyMs != 0 || v.P99LatencyMs != 0 || len(v.StatusCodes) != 0 {
		t.Fatalf("non-zero empty snapshot: %+v", v)
	}
}
