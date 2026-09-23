// Package experiment implements Phase 2: experiment models, a ticker-paced
// worker pool that generates controlled HTTP load, and the REST handlers.
package experiment

import (
	"fmt"
	"os"
	"strings"
	"time"
)

// Statuses for an experiment's lifecycle.
const (
	StatusRunning   = "running"
	StatusCompleted = "completed"
	StatusStopped   = "stopped"
)

// Controlled-lab caps (V1 safety rails).
const (
	MaxDurationSeconds = 300
	MaxRequestsPerSec  = 1000
	MaxWorkers         = 50
)

// Config is the user-supplied traffic configuration.
type Config struct {
	// Endpoint is the test-server path choice: "test" or "slow".
	Endpoint string `json:"endpoint"`
	// Defense asks workers to flag requests for the test-server rate
	// limiter (Phase 6). False = unprotected baseline run.
	Defense           bool `json:"defense"`
	DurationSeconds   int  `json:"duration_seconds"`
	RequestsPerSecond int  `json:"requests_per_second"`
	Workers           int  `json:"workers"`
}

// DefenseHeader is set by workers on every request when Config.Defense is
// true. The test server rate-limits only flagged requests, so the same
// deployment serves both baseline and defended runs.
const DefenseHeader = "X-DDoSLab-Defense"

// DefenseHeaderValue enables limiting; any other value means unprotected.
const DefenseHeaderValue = "on"

// Validate rejects unknown endpoints and out-of-range values.
func (c Config) Validate() error {
	switch c.Endpoint {
	case "test", "slow":
	default:
		return fmt.Errorf("endpoint must be %q or %q", "test", "slow")
	}
	if c.DurationSeconds < 1 || c.DurationSeconds > MaxDurationSeconds {
		return fmt.Errorf("duration_seconds must be 1..%d", MaxDurationSeconds)
	}
	if c.RequestsPerSecond < 1 || c.RequestsPerSecond > MaxRequestsPerSec {
		return fmt.Errorf("requests_per_second must be 1..%d", MaxRequestsPerSec)
	}
	if c.Workers < 1 || c.Workers > MaxWorkers {
		return fmt.Errorf("workers must be 1..%d", MaxWorkers)
	}
	return nil
}

// Experiment is a single traffic run plus its live outcome.
type Experiment struct {
	ID        string    `json:"id"`
	Status    string    `json:"status"`
	Config    Config    `json:"config"`
	TargetURL string    `json:"target_url"`
	StartedAt time.Time `json:"started_at"`
	// CompletedAt is nil while running.
	CompletedAt *time.Time `json:"completed_at"`
	// Counters are preliminary live tallies; Phase 3 adds the metrics endpoint.
	TotalRequests      int64 `json:"total_requests"`
	SuccessfulRequests int64 `json:"successful_requests"`
	FailedRequests     int64 `json:"failed_requests"`
}

// TargetBaseURL resolves the test-server base from env.
// Compose sets TARGET_BASE_URL=http://test-server:8081; local dev defaults
// to http://localhost:8081.
func TargetBaseURL() string {
	if v := strings.TrimSpace(os.Getenv("TARGET_BASE_URL")); v != "" {
		return strings.TrimSuffix(v, "/")
	}
	return "http://localhost:8081"
}
