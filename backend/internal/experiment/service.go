package experiment

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"sort"
	"sync"
	"sync/atomic"
	"time"

	"ddoslab/backend/internal/server"
)

// Service owns the in-memory experiment registry and runs worker pools
// against the test server. A Store (Phase 4, nil = disabled) persists
// finished runs and supplies history across restarts.
type Service struct {
	mu          sync.RWMutex
	experiments map[string]*run
	allowed     []string
	client      *http.Client
	store       Store
	stats       func() server.ProcStats
}

type run struct {
	exp     Experiment
	metrics Metrics
	final   *Result
	cancel  context.CancelFunc
	done    chan struct{}
}

// Result bundles an experiment with its metrics view and target-side
// telemetry. Historical marks store-loaded results (post-restart reads).
type Result struct {
	Exp            Experiment
	Metrics        View
	CPUPercent     *float64
	MemoryRSSBytes *int64
	Historical     bool
}

// Store persists finished results and serves history. All methods must be
// safe for concurrent use.
type Store interface {
	SaveResult(ctx context.Context, r Result) error
	LoadResult(ctx context.Context, id string) (Result, error)
	History(ctx context.Context) ([]Result, error)
}

// NewService builds a Service. allowedHosts gates target URLs (see
// server.AllowedTarget, nil = defaults); store persists finished runs
// (nil = in-memory only).
func NewService(allowedHosts []string, store Store) *Service {
	if allowedHosts == nil {
		allowedHosts = server.DefaultAllowedHosts
	}
	return &Service{
		experiments: make(map[string]*run),
		allowed:     allowedHosts,
		client:      &http.Client{Timeout: 10 * time.Second},
		store:       store,
		stats:       defaultStatsProbe,
	}
}

// defaultStatsProbe fetches the test-server's self telemetry. Any failure
// yields empty stats — telemetry must never break the caller.
func defaultStatsProbe() server.ProcStats {
	client := &http.Client{Timeout: time.Second}
	res, err := client.Get(TargetBaseURL() + "/api/stats")
	if err != nil {
		return server.ProcStats{}
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		return server.ProcStats{}
	}
	var st server.ProcStats
	if err := json.NewDecoder(res.Body).Decode(&st); err != nil {
		return server.ProcStats{}
	}
	return st
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
		return nil, ErrNotFound{ID: id}
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if r.exp.Status != StatusRunning {
		return nil, ErrConflict{Msg: fmt.Sprintf("experiment %s is %s", id, r.exp.Status)}
	}
	// Mark stopped synchronously so the response reflects the stop;
	// the worker loop exits on ctx.Done and finish() becomes a no-op.
	now := time.Now().UTC()
	r.exp.Status = StatusStopped
	r.exp.CompletedAt = &now
	r.cancel()
	return s.snapshotLocked(r), nil
}

// Get returns one experiment: live copy, else stored history (post-restart).
func (s *Service) Get(id string) (*Experiment, error) {
	s.mu.RLock()
	r, ok := s.experiments[id]
	s.mu.RUnlock()
	if ok {
		return s.snapshot(r), nil
	}
	if s.store == nil {
		return nil, ErrNotFound{ID: id}
	}
	res, err := s.store.LoadResult(context.Background(), id)
	if err != nil {
		return nil, err
	}
	return &res.Exp, nil
}

// List returns in-memory experiments unioned with stored history
// (in-memory wins on ID conflict), newest first. Store failures degrade
// to in-memory only and are logged.
func (s *Service) List(ctx context.Context) []Experiment {
	s.mu.RLock()
	seen := make(map[string]bool, len(s.experiments))
	out := make([]Experiment, 0, len(s.experiments))
	for id, r := range s.experiments {
		seen[id] = true
		out = append(out, *s.snapshotLocked(r))
	}
	s.mu.RUnlock()

	if s.store != nil {
		history, err := s.store.History(ctx)
		if err != nil {
			log.Printf("history unavailable, serving in-memory only: %v", err)
		}
		for _, h := range history {
			if !seen[h.Exp.ID] {
				out = append(out, h.Exp)
			}
		}
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

	// Workers drained: counters are final. Capture, persist, then report.
	final := s.finalize(r)
	if s.store != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		if err := s.store.SaveResult(ctx, final); err != nil {
			// Graceful degradation: the run is complete in memory;
			// history just won't survive a restart.
			log.Printf("persist experiment %s failed (in-memory copy kept): %v", r.exp.ID, err)
		}
		cancel()
	}
	s.mu.Lock()
	r.final = &final
	status := r.exp.Status
	s.mu.Unlock()
	log.Printf("experiment %s %s: total=%d ok=%d fail=%d",
		r.exp.ID, status,
		atomic.LoadInt64(&r.exp.TotalRequests),
		atomic.LoadInt64(&r.exp.SuccessfulRequests),
		atomic.LoadInt64(&r.exp.FailedRequests))
}

// finalize builds the immutable end-of-run result (snapshot + view +
// target telemetry). Call after workers drain.
func (s *Service) finalize(r *run) Result {
	exp := s.snapshot(r)
	st := s.stats()
	return Result{
		Exp:            *exp,
		Metrics:        r.metrics.Snapshot(),
		CPUPercent:     st.CPUPercent,
		MemoryRSSBytes: st.MemoryRSSBytes,
	}
}

// fire performs one request, bumps the atomic counters and records the
// outcome (status code 0 = no response: timeout / connection error).
func (s *Service) fire(ctx context.Context, r *run) {
	start := time.Now()
	atomic.AddInt64(&r.exp.TotalRequests, 1)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, r.exp.TargetURL, nil)
	if err != nil {
		atomic.AddInt64(&r.exp.FailedRequests, 1)
		r.metrics.Record(0, time.Since(start))
		return
	}
	// Flag the request for the test-server limiter when this run is
	// defended. Unflagged runs are never limited (baseline behavior).
	if r.exp.Config.Defense {
		req.Header.Set(DefenseHeader, DefenseHeaderValue)
	}
	res, err := s.client.Do(req)
	if err != nil {
		atomic.AddInt64(&r.exp.FailedRequests, 1)
		r.metrics.Record(0, time.Since(start))
		return
	}
	defer res.Body.Close()
	// Drain minimally so connections are reusable; body content unused in V1.
	if res.StatusCode >= 200 && res.StatusCode < 300 {
		atomic.AddInt64(&r.exp.SuccessfulRequests, 1)
	} else {
		atomic.AddInt64(&r.exp.FailedRequests, 1)
	}
	r.metrics.Record(res.StatusCode, time.Since(start))
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

// Metrics returns the live result for a running experiment (fresh target
// probe) or the stored/final result for a finished one. Post-restart,
// finished experiments resolve from the store.
func (s *Service) Metrics(id string) (Result, error) {
	s.mu.RLock()
	r, ok := s.experiments[id]
	s.mu.RUnlock()
	if ok {
		s.mu.RLock()
		finished := r.exp.Status != StatusRunning
		final := r.final
		s.mu.RUnlock()
		if finished && final != nil {
			return *final, nil
		}
		exp := s.snapshot(r)
		st := s.stats()
		return Result{
			Exp:            *exp,
			Metrics:        r.metrics.Snapshot(),
			CPUPercent:     st.CPUPercent,
			MemoryRSSBytes: st.MemoryRSSBytes,
		}, nil
	}
	if s.store == nil {
		return Result{}, ErrNotFound{ID: id}
	}
	res, err := s.store.LoadResult(context.Background(), id)
	if err != nil {
		return Result{}, err
	}
	res.Historical = true
	return res, nil
}

// Snapshot returns the experiment copy plus its computed metrics view.
// Prefer Metrics for handler use (includes telemetry + history fallback).
func (s *Service) Snapshot(id string) (*Experiment, View, error) {
	res, err := s.Metrics(id)
	if err != nil {
		return nil, View{}, err
	}
	return &res.Exp, res.Metrics, nil
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

// ErrNotFound / ErrConflict let the handler map to 404 / 409 and let the
// database layer signal missing documents.
type ErrNotFound struct{ ID string }

func (e ErrNotFound) Error() string { return fmt.Sprintf("experiment %s not found", e.ID) }

type ErrConflict struct{ Msg string }

func (e ErrConflict) Error() string { return e.Msg }
