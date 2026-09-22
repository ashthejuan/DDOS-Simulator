package experiment

import (
	"encoding/json"
	"errors"
	"net/http"
)

// Handler exposes the experiment REST API (Phase 2).
type Handler struct {
	svc *Service
}

// NewHandler builds a Handler around svc.
func NewHandler(svc *Service) *Handler { return &Handler{svc: svc} }

// RegisterRoutes mounts the experiment endpoints on mux.
func (h *Handler) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("POST /api/experiments", h.create)
	mux.HandleFunc("GET /api/experiments", h.list)
	mux.HandleFunc("GET /api/experiments/{id}", h.get)
	mux.HandleFunc("POST /api/experiments/{id}/stop", h.stop)
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
