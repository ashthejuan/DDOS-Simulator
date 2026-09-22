package experiment

import (
	"encoding/json"
	"errors"
	"net/http"
	"time"

	"ddoslab/backend/internal/server"
)

// Handler exposes the experiment REST API (Phases 2–3).
type Handler struct {
	svc        *Service
	statsProbe *http.Client
}

// NewHandler builds a Handler around svc.
func NewHandler(svc *Service) *Handler {
	return &Handler{svc: svc, statsProbe: &http.Client{Timeout: 2 * time.Second}}
}

// RegisterRoutes mounts the experiment endpoints on mux.
func (h *Handler) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("POST /api/experiments", h.create)
	mux.HandleFunc("GET /api/experiments", h.list)
	mux.HandleFunc("GET /api/experiments/{id}", h.get)
	mux.HandleFunc("POST /api/experiments/{id}/stop", h.stop)
	mux.HandleFunc("GET /api/experiments/{id}/metrics", h.metrics)
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

func (h *Handler) list(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{"experiments": h.svc.List()})
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
	exp, view, err := h.svc.Snapshot(r.PathValue("id"))
	if err != nil {
		writeServiceError(w, err)
		return
	}
	end := time.Now().UTC()
	if exp.CompletedAt != nil {
		end = *exp.CompletedAt
	}
	stats := h.probeTargetStats()
	writeJSON(w, http.StatusOK, MetricsResponse{
		ExperimentID:       exp.ID,
		Status:             exp.Status,
		ElapsedSeconds:     end.Sub(exp.StartedAt).Seconds(),
		View:               view,
		CPUPercent:         stats.CPUPercent,
		MemoryRSSBytes:     stats.MemoryRSSBytes,
		SuccessfulRequests: exp.SuccessfulRequests,
		FailedRequests:     exp.FailedRequests,
	})
}

// probeTargetStats fetches the test-server's self telemetry. Any failure
// (unreachable, timeout, bad JSON) yields empty stats — metrics must never
// fail just because telemetry is unavailable.
func (h *Handler) probeTargetStats() server.ProcStats {
	res, err := h.statsProbe.Get(TargetBaseURL() + "/api/stats")
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

func writeServiceError(w http.ResponseWriter, err error) {
	var nf errNotFound
	if errors.As(err, &nf) {
		writeError(w, http.StatusNotFound, err.Error())
		return
	}
	var cf errConflict
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
