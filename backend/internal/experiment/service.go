package experiment

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"log"
	"net/http"
	"sort"
	"sync"
	"sync/atomic"
	"time"

	"ddoslab/backend/internal/server"
)

// Service owns the in-memory experiment registry (Mongo persistence
// arrives in Phase 4) and runs worker pools against the test server.
type Service struct {
	mu          sync.RWMutex
	experiments map[string]*run
	allowed     []string
	client      *http.Client
}

type run struct {
	exp    Experiment
	cancel context.CancelFunc
	done   chan struct{}
}

// NewService builds a Service. allowedHosts gates target URLs (see
// server.AllowedTarget); pass nil to use server.DefaultAllowedHosts.
func NewService(allowedHosts []string) *Service {
	if allowedHosts == nil {
		allowedHosts = server.DefaultAllowedHosts
	}
	return &Service{
		experiments: make(map[string]*run),
		allowed:     allowedHosts,
		client:      &http.Client{Timeout: 10 * time.Second},
	}
}

// Create validates cfg, resolves the server-side target URL, registers the
// experiment and starts its worker pool.
func (s *Service) Create(cfg Config) (*Experiment, error) {
	if err := cfg.Validate(); err != nil {
		return nil, err
	}
	target := TargetBaseURL() + "/api/" + cfg.Endpoint
	if !server.AllowedTarget(target, s.allowed) {
		return nil, fmt.Errorf("target %q rejected by allowlist", target)
	}

	id := newID()
	ctx, cancel := context.WithCancel(context.Background())
	r := &run{
		exp: Experiment{
			ID:        id,
			Status:    StatusRunning,
			Config:    cfg,
			TargetURL: target,
			StartedAt: time.Now().UTC(),
		},
		cancel: cancel,
		done:   make(chan struct{}),
	}

	s.mu.Lock()
	s.experiments[id] = r
	s.mu.Unlock()

	go s.execute(ctx, r)
	return s.snapshot(r), nil
}

// Stop cancels a running experiment. Stopping a finished one is a 409-style
// error (see handler).
func (s *Service) Stop(id string) (*Experiment, error) {
	s.mu.RLock()
	r, ok := s.experiments[id]
	s.mu.RUnlock()
	if !ok {
		return nil, errNotFound{id}
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if r.exp.Status != StatusRunning {
		return nil, errConflict{fmt.Sprintf("experiment %s is %s", id, r.exp.Status)}
	}
	// Mark stopped synchronously so the response reflects the stop;
	// the worker loop exits on ctx.Done and finish() becomes a no-op.
	now := time.Now().UTC()
	r.exp.Status = StatusStopped
	r.exp.CompletedAt = &now
	r.cancel()
	return s.snapshotLocked(r), nil
}

// Get returns a copy of one experiment.
func (s *Service) Get(id string) (*Experiment, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	r, ok := s.experiments[id]
	if !ok {
		return nil, errNotFound{id}
	}
	return s.snapshotLocked(r), nil
}

// List returns copies of all experiments, newest first.
func (s *Service) List() []Experiment {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]Experiment, 0, len(s.experiments))
	for _, r := range s.experiments {
		out = append(out, *s.snapshotLocked(r))
	}
	sort.Slice(out, func(i, j int) bool { return out[i].StartedAt.After(out[j].StartedAt) })
	return out
}

// execute paces cfg.RequestsPerSecond tokens through cfg.Workers goroutines
// until the duration timer fires (completed) or Stop cancels (stopped).
func (s *Service) execute(ctx context.Context, r *run) {
	defer close(r.done)

	cfg := r.exp.Config
	timer := time.NewTimer(time.Duration(cfg.DurationSeconds) * time.Second)
	defer timer.Stop()

	interval := time.Second / time.Duration(cfg.RequestsPerSecond)
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	tokens := make(chan struct{}, cfg.Workers*2)
	var wg sync.WaitGroup
	for range cfg.Workers {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for range tokens {
				s.fire(ctx, r)
			}
		}()
	}

loop:
	for {
		select {
		case <-ctx.Done():
			s.finish(r, StatusStopped)
			break loop
		case <-timer.C:
			s.finish(r, StatusCompleted)
			break loop
		case <-ticker.C:
			select {
			case tokens <- struct{}{}:
			case <-ctx.Done():
				s.finish(r, StatusStopped)
				break loop
			case <-timer.C:
				s.finish(r, StatusCompleted)
				break loop
			}
		}
	}

	close(tokens)
	wg.Wait()

	s.mu.RLock()
	status := r.exp.Status
	s.mu.RUnlock()
	log.Printf("experiment %s %s: total=%d ok=%d fail=%d",
		r.exp.ID, status,
		atomic.LoadInt64(&r.exp.TotalRequests),
		atomic.LoadInt64(&r.exp.SuccessfulRequests),
		atomic.LoadInt64(&r.exp.FailedRequests))
}

// fire performs one request and bumps the atomic counters.
func (s *Service) fire(ctx context.Context, r *run) {
	atomic.AddInt64(&r.exp.TotalRequests, 1)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, r.exp.TargetURL, nil)
	if err != nil {
		atomic.AddInt64(&r.exp.FailedRequests, 1)
		return
	}
	res, err := s.client.Do(req)
	if err != nil {
		atomic.AddInt64(&r.exp.FailedRequests, 1)
		return
	}
	defer res.Body.Close()
	// Drain minimally so connections are reusable; body content unused in V1.
	if res.StatusCode >= 200 && res.StatusCode < 300 {
		atomic.AddInt64(&r.exp.SuccessfulRequests, 1)
	} else {
		atomic.AddInt64(&r.exp.FailedRequests, 1)
	}
}

func (s *Service) finish(r *run, status string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if r.exp.Status != StatusRunning {
		return
	}
	now := time.Now().UTC()
	r.exp.Status = status
	r.exp.CompletedAt = &now
}

func (s *Service) snapshot(r *run) *Experiment {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.snapshotLocked(r)
}

func (s *Service) snapshotLocked(r *run) *Experiment {
	// Non-counter fields are only mutated under the write lock; counters
	// are atomic and must be loaded (never plain-copied) to stay race-free.
	return &Experiment{
		ID:                 r.exp.ID,
		Status:             r.exp.Status,
		Config:             r.exp.Config,
		TargetURL:          r.exp.TargetURL,
		StartedAt:          r.exp.StartedAt,
		CompletedAt:        r.exp.CompletedAt,
		TotalRequests:      atomic.LoadInt64(&r.exp.TotalRequests),
		SuccessfulRequests: atomic.LoadInt64(&r.exp.SuccessfulRequests),
		FailedRequests:     atomic.LoadInt64(&r.exp.FailedRequests),
	}
}

// Done returns a channel closed when the experiment finishes (for tests).
func (s *Service) Done(id string) (<-chan struct{}, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	r, ok := s.experiments[id]
	if !ok {
		return nil, false
	}
	return r.done, true
}

func newID() string {
	var b [8]byte
	if _, err := rand.Read(b[:]); err != nil {
		panic(err)
	}
	return hex.EncodeToString(b[:])
}

// errNotFound / errConflict let the handler map to 404 / 409.
type errNotFound struct{ id string }

func (e errNotFound) Error() string { return fmt.Sprintf("experiment %s not found", e.id) }

type errConflict struct{ msg string }

func (e errConflict) Error() string { return e.msg }
