package experiment

import (
	"math/rand"
	"sort"
	"strconv"
	"sync"
	"time"
)

// Collector tuning.
const (
	// maxSamples bounds memory: reservoir sampling keeps an unbiased
	// subset no matter how long the run (1000 RPS x 300s = 300k reqs).
	maxSamples = 8192
	// rpsWindow is the sliding window for the reported request rate.
	rpsWindow = 5 * time.Second
	// tsKeep bounds the timestamp ring to a bit more than the window.
	tsKeep = 10 * time.Second
)

// Metrics accumulates per-request outcomes behind one mutex. At lab scale
// (~1000 RPS max) a single lock per request is negligible.
type Metrics struct {
	mu         sync.Mutex
	statusCode map[int]int64
	samples    []float64 // reservoir of latency ms
	seen       int64     // total samples offered (for reservoir math)
	sumMs      float64
	timestamps []int64 // UnixNano of recent requests, trimmed to tsKeep
}

// Record adds one completed request outcome.
func (m *Metrics) Record(statusCode int, latency time.Duration) {
	ms := float64(latency) / float64(time.Millisecond)
	now := time.Now().UnixNano()

	m.mu.Lock()
	defer m.mu.Unlock()

	if m.statusCode == nil {
		m.statusCode = make(map[int]int64)
	}
	m.statusCode[statusCode]++

	// Reservoir sampling (algorithm R): uniform subset of the full stream.
	m.seen++
	if len(m.samples) < maxSamples {
		m.samples = append(m.samples, ms)
	} else if j := rand.Intn(int(m.seen)); j < maxSamples {
		m.samples[j] = ms
	}
	m.sumMs += ms

	m.timestamps = append(m.timestamps, now)
	cutoff := now - tsKeep.Nanoseconds()
	keep := 0
	for keep < len(m.timestamps) && m.timestamps[keep] < cutoff {
		keep++
	}
	if keep > 0 {
		m.timestamps = append([]int64(nil), m.timestamps[keep:]...)
	}
}

// View is a computed point-in-time snapshot for JSON responses.
type View struct {
	TotalRequests int64            `json:"total_requests"`
	StatusCodes   map[string]int64 `json:"status_codes"`
	RPS           float64          `json:"rps"`
	AvgLatencyMs  float64          `json:"avg_latency_ms"`
	P50LatencyMs  float64          `json:"p50_latency_ms"`
	P95LatencyMs  float64          `json:"p95_latency_ms"`
	P99LatencyMs  float64          `json:"p99_latency_ms"`
	Samples       int              `json:"samples"`
}

// Snapshot computes rates and percentiles (nearest-rank on a sorted copy).
// TotalRequests counts completed outcomes (in-flight requests are tallied
// in Experiment.TotalRequests once they finish).
func (m *Metrics) Snapshot() View {
	m.mu.Lock()
	defer m.mu.Unlock()

	v := View{TotalRequests: m.seen, StatusCodes: make(map[string]int64, len(m.statusCode))}
	for code, n := range m.statusCode {
		v.StatusCodes[strconv.Itoa(code)] = n
	}

	now := time.Now().UnixNano()
	cutoff := now - rpsWindow.Nanoseconds()
	inWindow := 0
	for _, ts := range m.timestamps {
		if ts >= cutoff {
			inWindow++
		}
	}
	v.RPS = float64(inWindow) / rpsWindow.Seconds()

	if m.seen > 0 {
		v.AvgLatencyMs = m.sumMs / float64(m.seen)
	}
	v.Samples = len(m.samples)
	if len(m.samples) > 0 {
		sorted := append([]float64(nil), m.samples...)
		sort.Float64s(sorted)
		v.P50LatencyMs = percentile(sorted, 50)
		v.P95LatencyMs = percentile(sorted, 95)
		v.P99LatencyMs = percentile(sorted, 99)
	}
	return v
}

// percentile returns the nearest-rank percentile of sorted values.
func percentile(sorted []float64, p float64) float64 {
	if len(sorted) == 0 {
		return 0
	}
	rank := int((p/100)*float64(len(sorted)) + 0.5)
	if rank < 1 {
		rank = 1
	}
	if rank > len(sorted) {
		rank = len(sorted)
	}
	return sorted[rank-1]
}
