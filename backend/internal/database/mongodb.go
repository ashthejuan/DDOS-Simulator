// Package database implements Phase 4: MongoDB persistence for experiments
// and their final metrics snapshots (PRD §11).
package database

import (
	"context"
	"time"

	"ddoslab/backend/internal/experiment"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
)

const (
	dbName          = "ddoslab"
	experimentsColl = "experiments"
	metricsColl     = "metrics"
	connectTimeout  = 10 * time.Second
	opTimeout       = 5 * time.Second
	historyLimit    = 50
)

// Store persists experiment results in MongoDB and implements
// experiment.Store so the service layer stays driver-agnostic.
type Store struct {
	experiments *mongo.Collection
	metrics     *mongo.Collection
}

// Connect dials uri, pings, and ensures indexes. Callers should pass a
// startup timeout context; degradation on failure is the caller's call.
func Connect(ctx context.Context, uri string) (*Store, error) {
	ctx, cancel := context.WithTimeout(ctx, connectTimeout)
	defer cancel()

	client, err := mongo.Connect(options.Client().ApplyURI(uri))
	if err != nil {
		return nil, err
	}
	if err := client.Ping(ctx, nil); err != nil {
		_ = client.Disconnect(context.Background())
		return nil, err
	}
	s := &Store{
		experiments: client.Database(dbName).Collection(experimentsColl),
		metrics:     client.Database(dbName).Collection(metricsColl),
	}
	if err := s.ensureIndexes(ctx); err != nil {
		_ = client.Disconnect(context.Background())
		return nil, err
	}
	return s, nil
}

func (s *Store) ensureIndexes(ctx context.Context) error {
	if _, err := s.experiments.Indexes().CreateOne(ctx, mongo.IndexModel{
		Keys: bson.D{{Key: "started_at", Value: -1}},
	}); err != nil {
		return err
	}
	if _, err := s.metrics.Indexes().CreateOne(ctx, mongo.IndexModel{
		Keys: bson.D{{Key: "experiment_id", Value: 1}, {Key: "timestamp", Value: -1}},
	}); err != nil {
		return err
	}
	return nil
}

// ExperimentDoc is the persisted form of a finished experiment (PRD §11,
// plus endpoint/target/status-code detail needed by later phases).
type ExperimentDoc struct {
	ID                 string           `bson:"_id"`
	Status             string           `bson:"status"`
	Endpoint           string           `bson:"endpoint"`
	DurationSeconds    int              `bson:"duration_seconds"`
	RequestsPerSecond  int              `bson:"requests_per_second"`
	Workers            int              `bson:"workers"`
	TargetURL          string           `bson:"target_url"`
	StartedAt          time.Time        `bson:"started_at"`
	CompletedAt        *time.Time       `bson:"completed_at,omitempty"`
	TotalRequests      int64            `bson:"total_requests"`
	SuccessfulRequests int64            `bson:"successful_requests"`
	FailedRequests     int64            `bson:"failed_requests"`
	StatusCodes        map[string]int64 `bson:"status_codes,omitempty"`
	AvgLatencyMs       float64          `bson:"average_latency_ms"`
	P50LatencyMs       float64          `bson:"p50_latency_ms"`
	P95LatencyMs       float64          `bson:"p95_latency_ms"`
	P99LatencyMs       float64          `bson:"p99_latency_ms"`
	CPUPercent         *float64         `bson:"cpu_percent,omitempty"`
	MemoryRSSBytes     *int64           `bson:"memory_rss_bytes,omitempty"`
}

// MetricsDoc is the persisted final metrics snapshot (PRD §11 metrics).
type MetricsDoc struct {
	ExperimentID   string           `bson:"experiment_id"`
	Timestamp      time.Time        `bson:"timestamp"`
	RequestsPerSec float64          `bson:"requests_per_second"`
	AverageMs      float64          `bson:"average_latency_ms"`
	P50Ms          float64          `bson:"p50_latency_ms"`
	P95Ms          float64          `bson:"p95_latency_ms"`
	P99Ms          float64          `bson:"p99_latency_ms"`
	TotalRequests  int64            `bson:"total_requests"`
	StatusCodes    map[string]int64 `bson:"status_codes,omitempty"`
	CPUPercent     *float64         `bson:"cpu_percent,omitempty"`
	MemoryRSSBytes *int64           `bson:"memory_rss_bytes,omitempty"`
}

// SaveResult upserts the experiment doc and inserts its final metrics
// snapshot. Safe to call once per finished run; retries overwrite.
func (s *Store) SaveResult(ctx context.Context, r experiment.Result) error {
	ctx, cancel := context.WithTimeout(ctx, opTimeout)
	defer cancel()

	ts := time.Now().UTC()
	if r.Exp.CompletedAt != nil {
		ts = *r.Exp.CompletedAt
	}
	expDoc := ExperimentDoc{
		ID:                 r.Exp.ID,
		Status:             r.Exp.Status,
		Endpoint:           r.Exp.Config.Endpoint,
		DurationSeconds:    r.Exp.Config.DurationSeconds,
		RequestsPerSecond:  r.Exp.Config.RequestsPerSecond,
		Workers:            r.Exp.Config.Workers,
		TargetURL:          r.Exp.TargetURL,
		StartedAt:          r.Exp.StartedAt,
		CompletedAt:        r.Exp.CompletedAt,
		TotalRequests:      r.Exp.TotalRequests,
		SuccessfulRequests: r.Exp.SuccessfulRequests,
		FailedRequests:     r.Exp.FailedRequests,
		StatusCodes:        r.Metrics.StatusCodes,
		AvgLatencyMs:       r.Metrics.AvgLatencyMs,
		P50LatencyMs:       r.Metrics.P50LatencyMs,
		P95LatencyMs:       r.Metrics.P95LatencyMs,
		P99LatencyMs:       r.Metrics.P99LatencyMs,
		CPUPercent:         r.CPUPercent,
		MemoryRSSBytes:     r.MemoryRSSBytes,
	}
	if _, err := s.experiments.ReplaceOne(ctx,
		bson.D{{Key: "_id", Value: expDoc.ID}},
		expDoc, options.Replace().SetUpsert(true)); err != nil {
		return err
	}
	_, err := s.metrics.InsertOne(ctx, MetricsDoc{
		ExperimentID:   r.Exp.ID,
		Timestamp:      ts,
		RequestsPerSec: r.Metrics.RPS,
		AverageMs:      r.Metrics.AvgLatencyMs,
		P50Ms:          r.Metrics.P50LatencyMs,
		P95Ms:          r.Metrics.P95LatencyMs,
		P99Ms:          r.Metrics.P99LatencyMs,
		TotalRequests:  r.Metrics.TotalRequests,
		StatusCodes:    r.Metrics.StatusCodes,
		CPUPercent:     r.CPUPercent,
		MemoryRSSBytes: r.MemoryRSSBytes,
	})
	return err
}

// LoadResult fetches one experiment plus its latest metrics snapshot.
func (s *Store) LoadResult(ctx context.Context, id string) (experiment.Result, error) {
	ctx, cancel := context.WithTimeout(ctx, opTimeout)
	defer cancel()

	var doc ExperimentDoc
	if err := s.experiments.FindOne(ctx, bson.D{{Key: "_id", Value: id}}).Decode(&doc); err != nil {
		if err == mongo.ErrNoDocuments {
			return experiment.Result{}, experiment.ErrNotFound{ID: id}
		}
		return experiment.Result{}, err
	}
	res := docToResult(doc)

	var m MetricsDoc
	err := s.metrics.FindOne(ctx,
		bson.D{{Key: "experiment_id", Value: id}},
		options.FindOne().SetSort(bson.D{{Key: "timestamp", Value: -1}}),
	).Decode(&m)
	if err != nil && err != mongo.ErrNoDocuments {
		return experiment.Result{}, err
	}
	if err == nil {
		res.Metrics = experiment.View{
			TotalRequests: m.TotalRequests,
			StatusCodes:   m.StatusCodes,
			RPS:           m.RequestsPerSec,
			AvgLatencyMs:  m.AverageMs,
			P50LatencyMs:  m.P50Ms,
			P95LatencyMs:  m.P95Ms,
			P99LatencyMs:  m.P99Ms,
			Samples:       int(m.TotalRequests),
		}
	}
	return res, nil
}

// History returns the newest stored results first (bounded).
func (s *Store) History(ctx context.Context) ([]experiment.Result, error) {
	ctx, cancel := context.WithTimeout(ctx, opTimeout)
	defer cancel()

	cur, err := s.experiments.Find(ctx, bson.D{},
		options.Find().SetSort(bson.D{{Key: "started_at", Value: -1}}).SetLimit(historyLimit))
	if err != nil {
		return nil, err
	}
	defer cur.Close(ctx)

	var out []experiment.Result
	for cur.Next(ctx) {
		var doc ExperimentDoc
		if err := cur.Decode(&doc); err != nil {
			return nil, err
		}
		out = append(out, docToResult(doc))
	}
	if err := cur.Err(); err != nil {
		return nil, err
	}
	return out, nil
}

func docToResult(doc ExperimentDoc) experiment.Result {
	return experiment.Result{
		Exp: experiment.Experiment{
			ID:                 doc.ID,
			Status:             doc.Status,
			Config:             experiment.Config{Endpoint: doc.Endpoint, DurationSeconds: doc.DurationSeconds, RequestsPerSecond: doc.RequestsPerSecond, Workers: doc.Workers},
			TargetURL:          doc.TargetURL,
			StartedAt:          doc.StartedAt,
			CompletedAt:        doc.CompletedAt,
			TotalRequests:      doc.TotalRequests,
			SuccessfulRequests: doc.SuccessfulRequests,
			FailedRequests:     doc.FailedRequests,
		},
		Metrics: experiment.View{
			TotalRequests: doc.TotalRequests,
			StatusCodes:   doc.StatusCodes,
			AvgLatencyMs:  doc.AvgLatencyMs,
			P50LatencyMs:  doc.P50LatencyMs,
			P95LatencyMs:  doc.P95LatencyMs,
			P99LatencyMs:  doc.P99LatencyMs,
			Samples:       int(doc.TotalRequests),
		},
		CPUPercent:     doc.CPUPercent,
		MemoryRSSBytes: doc.MemoryRSSBytes,
		Historical:     true,
	}
}
