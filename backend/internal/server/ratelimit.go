// Rate limiting for Phase 6 (defense mode).
//
// The limiter is a token bucket shared by the test-server traffic routes.
// It engages ONLY for requests carrying the defense header
// (X-DDoSLab-Defense: on), which workers set when the experiment enables
// defense. Unflagged runs are never limited, so one deployment serves both
// the unprotected baseline and the defended run back-to-back.
//
// Over-limit requests are rejected with HTTP 429 (counted client-side in
// the metrics status-code map) instead of consuming server work.
package server

import (
	"encoding/json"
	"net/http"
	"sync"
	"time"
)

// DefenseHeader mirrors the worker-side signal (kept as literals here so
// this package stays importable without the experiment package).
const defenseHeader = "X-DDoSLab-Defense"
const defenseHeaderValue = "on"

// Limiter is a mutex-guarded token bucket: capacity tokens max, refilled
// at rate tokens/second. Zero value denies everything; use NewLimiter.
type Limiter struct {
	mu       sync.Mutex
	rate     float64
	capacity float64
	tokens   float64
	last     time.Time
}

// NewLimiter builds a bucket that sustains rate requests/second with bursts
// up to capacity (capacity <= 0 defaults to rate).
func NewLimiter(rate float64, capacity float64) *Limiter {
	if capacity <= 0 {
		capacity = rate
	}
	return &Limiter{rate: rate, capacity: capacity, tokens: capacity, last: time.Now()}
}

// Allow reports whether one request may proceed, consuming a token.
func (l *Limiter) Allow() bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	now := time.Now()
	l.tokens += now.Sub(l.last).Seconds() * l.rate
	if l.tokens > l.capacity {
		l.tokens = l.capacity
	}
	l.last = now
	if l.tokens < 1 {
		return false
	}
	l.tokens--
	return true
}

// DefenseMiddleware wraps traffic routes: flagged requests must pass the
// limiter (429 on excess); unflagged requests always pass through.
// A nil limiter disables limiting entirely (useful in tests).
func DefenseMiddleware(limiter *Limiter, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if limiter != nil && r.Header.Get(defenseHeader) == defenseHeaderValue {
			if !limiter.Allow() {
				w.Header().Set("Content-Type", "application/json")
				w.Header().Set("Retry-After", "1")
				w.WriteHeader(http.StatusTooManyRequests)
				_ = json.NewEncoder(w).Encode(map[string]string{
					"status": "rate_limited",
					"error":  "defense rate limiter engaged: slow down",
				})
				return
			}
		}
		next.ServeHTTP(w, r)
	})
}
