package experiment

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"
)

// fakeStore is an in-memory Store for persistence tests.
type fakeStore struct {
	mu      sync.Mutex
	saved   []Result
	saveErr error
	hist    []Result
	histErr error
}

func (f *fakeStore) SaveResult(_ context.Context, r Result) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.saveErr != nil {
		return f.saveErr
	}
	f.saved = append(f.saved, r)
	return nil
}

func (f *fakeStore) LoadResult(_ context.Context, id string) (Result, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	for _, r := range append(append([]Result(nil), f.saved...), f.hist...) {
		if r.Exp.ID == id {
			r.Historical = true
			return r, nil
		}
	}
	return Result{}, ErrNotFound{ID: id}
}

func (f *fakeStore) History(_ context.Context) ([]Result, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.histErr != nil {
		return nil, f.histErr
	}
	return append([]Result(nil), f.hist...), nil
}

func testServiceWithStore(t *testing.T, store *fakeStore) *Service {
	t.Helper()
	useTestTarget(t) // spins a stub test server + sets TARGET_BASE_URL
	return NewService(nil, store)
}

func TestFinishPersistsResult(t *testing.T) {
	fake := &fakeStore{}
	svc := testServiceWithStore(t, fake)

	exp, err := svc.Create(Config{Endpoint: "test", DurationSeconds: 1, RequestsPerSecond: 10, Workers: 2})
	if err != nil {
		t.Fatal(err)
	}
	done, _ := svc.Done(exp.ID)
	waitDone(t, done, 10*time.Second)

	fake.mu.Lock()
	defer fake.mu.Unlock()
	if len(fake.saved) != 1 {
		t.Fatalf("saved=%d, want 1", len(fake.saved))
	}
	got := fake.saved[0]
	if got.Exp.ID != exp.ID || got.Exp.Status != StatusCompleted {
		t.Fatalf("unexpected saved: %+v", got.Exp)
	}
	if got.Exp.TotalRequests < 5 || got.Metrics.TotalRequests < 5 {
		t.Fatalf("counts too low: %+v", got)
	}
	if got.Metrics.AvgLatencyMs <= 0 || len(got.Metrics.StatusCodes) == 0 {
		t.Fatalf("metrics not captured: %+v", got.Metrics)
	}
}

func TestPersistFailureDegrades(t *testing.T) {
	svc := testServiceWithStore(t, &fakeStore{saveErr: fmt.Errorf("mongo down")})

	exp, err := svc.Create(Config{Endpoint: "test", DurationSeconds: 1, RequestsPerSecond: 5, Workers: 1})
	if err != nil {
		t.Fatal(err)
	}
	done, _ := svc.Done(exp.ID)
	waitDone(t, done, 10*time.Second)

	got, err := svc.Get(exp.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got.Status != StatusCompleted {
		t.Fatalf("status=%q, want completed despite persist failure", got.Status)
	}
}

func TestListUnionsHistory(t *testing.T) {
	old := Result{Exp: Experiment{
		ID: "old-run", Status: StatusCompleted,
		Config:    Config{Endpoint: "test", DurationSeconds: 5, RequestsPerSecond: 5, Workers: 1},
		StartedAt: time.Now().UTC().Add(-time.Hour),
	}}
	svc := testServiceWithStore(t, &fakeStore{hist: []Result{old}})

	live, err := svc.Create(Config{Endpoint: "test", DurationSeconds: 60, RequestsPerSecond: 1, Workers: 1})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _, _ = svc.Stop(live.ID) })

	list := svc.List(context.Background())
	if len(list) != 2 {
		t.Fatalf("list=%v, want 2 (live + history)", list)
	}
	if list[0].ID != live.ID || list[1].ID != "old-run" {
		t.Fatalf("wrong order/union: %v", list)
	}
}

func TestListStoreErrorDegrades(t *testing.T) {
	svc := testServiceWithStore(t, &fakeStore{histErr: fmt.Errorf("mongo down")})

	live, err := svc.Create(Config{Endpoint: "test", DurationSeconds: 60, RequestsPerSecond: 1, Workers: 1})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _, _ = svc.Stop(live.ID) })

	list := svc.List(context.Background())
	if len(list) != 1 || list[0].ID != live.ID {
		t.Fatalf("want in-memory only, got %v", list)
	}
}

func TestMetricsHistoricalFallback(t *testing.T) {
	// Simulate a restart: run finishes on one service, a fresh service with
	// the same store serves its metrics as historical.
	fake := &fakeStore{}
	svc1 := testServiceWithStore(t, fake)
	exp, err := svc1.Create(Config{Endpoint: "test", DurationSeconds: 1, RequestsPerSecond: 10, Workers: 1})
	if err != nil {
		t.Fatal(err)
	}
	done, _ := svc1.Done(exp.ID)
	waitDone(t, done, 10*time.Second)

	svc2 := NewService(nil, fake)
	res, err := svc2.Metrics(exp.ID)
	if err != nil {
		t.Fatal(err)
	}
	if !res.Historical {
		t.Fatal("want historical result from store")
	}
	if res.Exp.Status != StatusCompleted || res.Metrics.TotalRequests < 5 {
		t.Fatalf("unexpected historical: %+v", res)
	}

	if _, err := svc2.Metrics("missing"); err == nil {
		t.Fatal("want not-found for unknown id")
	}
}
