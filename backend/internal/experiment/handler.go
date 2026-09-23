package experiment

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"time"

	"ddoslab/backend/internal/report"
)

// Handler exposes the experiment REST API (Phases 2–4).
type Handler struct {
	svc *Service
}

// NewHandler builds a Handler around svc.
func NewHandler(svc *Service) *Handler {
	return &Handler{svc: svc}
}

// RegisterRoutes mounts the experiment endpoints on mux.
func (h *Handler) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("POST /api/experiments", h.create)
	mux.HandleFunc("GET /api/experiments", h.list)
	mux.HandleFunc("GET /api/experiments/{id}", h.get)
	mux.HandleFunc("POST /api/experiments/{id}/stop", h.stop)
	mux.HandleFunc("GET /api/experiments/{id}/metrics", h.metrics)
	mux.HandleFunc("GET /api/experiments/{id}/report.pdf", h.reportPDF)
}

func (h *Handler) create(w http.ResponseWriter, r *http.Request) {
	var cfg Config
	if err := json.NewDecoder(r.Body).Decode(&cfg); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}
	exp, err := h.svc.Create(cfg)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusCreated, exp)
}

func (h *Handler) list(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{"experiments": h.svc.List(r.Context())})
}

func (h *Handler) get(w http.ResponseWriter, r *http.Request) {
	exp, err := h.svc.Get(r.PathValue("id"))
	if err != nil {
		writeServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, exp)
}

func (h *Handler) stop(w http.ResponseWriter, r *http.Request) {
	exp, err := h.svc.Stop(r.PathValue("id"))
	if err != nil {
		writeServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, exp)
}

// MetricsResponse is the live poll payload: worker-side rates/latency plus
// target-side process telemetry (null where unavailable).
type MetricsResponse struct {
	ExperimentID   string  `json:"experiment_id"`
	Status         string  `json:"status"`
	ElapsedSeconds float64 `json:"elapsed_seconds"`
	View
	CPUPercent         *float64 `json:"cpu_percent"`
	MemoryRSSBytes     *int64   `json:"memory_rss_bytes"`
	SuccessfulRequests int64    `json:"successful_requests"`
	FailedRequests     int64    `json:"failed_requests"`
}

func (h *Handler) metrics(w http.ResponseWriter, r *http.Request) {
	res, err := h.svc.Metrics(r.PathValue("id"))
	if err != nil {
		writeServiceError(w, err)
		return
	}
	end := time.Now().UTC()
	if res.Exp.CompletedAt != nil {
		end = *res.Exp.CompletedAt
	}
	writeJSON(w, http.StatusOK, MetricsResponse{
		ExperimentID:       res.Exp.ID,
		Status:             res.Exp.Status,
		ElapsedSeconds:     end.Sub(res.Exp.StartedAt).Seconds(),
		View:               res.Metrics,
		CPUPercent:         res.CPUPercent,
		MemoryRSSBytes:     res.MemoryRSSBytes,
		SuccessfulRequests: res.Exp.SuccessfulRequests,
		FailedRequests:     res.Exp.FailedRequests,
	})
}

// reportPDF streams the Phase 6 PDF report for one experiment.
func (h *Handler) reportPDF(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	res, err := h.svc.Metrics(id)
	if err != nil {
		writeServiceError(w, err)
		return
	}
	pdf, err := report.Build(report.Input{
		ID:                 res.Exp.ID,
		Status:             res.Exp.Status,
		StartedAt:          res.Exp.StartedAt,
		CompletedAt:        res.Exp.CompletedAt,
		TargetURL:          res.Exp.TargetURL,
		Defense:            res.Exp.Config.Defense,
		DurationSeconds:    res.Exp.Config.DurationSeconds,
		RequestsPerSecond:  res.Exp.Config.RequestsPerSecond,
		Workers:            res.Exp.Config.Workers,
		TotalRequests:      res.Metrics.TotalRequests,
		SuccessfulRequests: res.Exp.SuccessfulRequests,
		FailedRequests:     res.Exp.FailedRequests,
		RPS:                res.Metrics.RPS,
		AvgLatencyMs:       res.Metrics.AvgLatencyMs,
		P50LatencyMs:       res.Metrics.P50LatencyMs,
		P95LatencyMs:       res.Metrics.P95LatencyMs,
		P99LatencyMs:       res.Metrics.P99LatencyMs,
		Samples:            res.Metrics.Samples,
		StatusCodes:        res.Metrics.StatusCodes,
		CPUPercent:         res.CPUPercent,
		MemoryRSSBytes:     res.MemoryRSSBytes,
	})
	if err != nil {
		writeError(w, http.StatusInternalServerError, fmt.Sprintf("render report: %v", err))
		return
	}
	w.Header().Set("Content-Type", "application/pdf")
	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=\"ddoslab-experiment-%s.pdf\"", id))
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(pdf)
}

func writeServiceError(w http.ResponseWriter, err error) {
	var nf ErrNotFound
	if errors.As(err, &nf) {
		writeError(w, http.StatusNotFound, err.Error())
		return
	}
	var cf ErrConflict
	if errors.As(err, &cf) {
		writeError(w, http.StatusConflict, err.Error())
		return
	}
	writeError(w, http.StatusBadRequest, err.Error())
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, map[string]string{"error": msg})
}
