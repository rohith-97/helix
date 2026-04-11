package api

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/yourusername/helix/internal/queue"
	"github.com/yourusername/helix/internal/router"
)

type Handler struct {
	router *router.Router
	queue  *queue.Queue
}

func NewHandler(r *router.Router, q *queue.Queue) *Handler {
	return &Handler{router: r, queue: q}
}

type FoldRequest struct {
	Sequence  string `json:"sequence"`
	Accession string `json:"accession,omitempty"`
}

type FoldResponse struct {
	PDB        string        `json:"pdb"`
	Sequence   string        `json:"sequence"`
	Accession  string        `json:"accession,omitempty"`
	Source     router.Source `json:"source"`
	Confidence float64       `json:"confidence,omitempty"`
	ElapsedS   float64       `json:"elapsed_seconds"`
	Cost       int           `json:"cost"`
	Cached     bool          `json:"cached"`
}

type BatchFoldRequest struct {
	Sequences []string `json:"sequences"`
}

type BatchFoldResponse struct {
	Results []FoldResponse `json:"results"`
	Errors  []string       `json:"errors,omitempty"`
}

type JobResponse struct {
	ID      string          `json:"id"`
	Status  queue.JobStatus `json:"status"`
	Result  string          `json:"result,omitempty"`
	Error   string          `json:"error,omitempty"`
	Created time.Time       `json:"created"`
	Updated time.Time       `json:"updated"`
}

type ErrorResponse struct {
	Error string `json:"error"`
}

func (h *Handler) Fold(w http.ResponseWriter, r *http.Request) {
	var req FoldRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, "invalid request body", http.StatusBadRequest)
		return
	}

	if req.Sequence == "" && req.Accession == "" {
		writeError(w, "sequence or accession is required", http.StatusBadRequest)
		return
	}

	result, err := h.router.Fold(r.Context(), router.FoldRequest{
		Sequence:  req.Sequence,
		Accession: req.Accession,
	})
	if err != nil {
		writeError(w, err.Error(), http.StatusInternalServerError)
		return
	}

	writeJSON(w, FoldResponse{
		PDB:        result.PDB,
		Sequence:   result.Sequence,
		Accession:  result.Accession,
		Source:     result.Source,
		Confidence: result.Confidence,
		ElapsedS:   result.Elapsed.Seconds(),
		Cost:       result.Cost,
		Cached:     result.Source == router.SourceCache,
	}, http.StatusOK)
}

func (h *Handler) BatchFold(w http.ResponseWriter, r *http.Request) {
	var req BatchFoldRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, "invalid request body", http.StatusBadRequest)
		return
	}

	if len(req.Sequences) == 0 {
		writeError(w, "sequences are required", http.StatusBadRequest)
		return
	}

	if len(req.Sequences) > 10 {
		writeError(w, "max 10 sequences per batch", http.StatusBadRequest)
		return
	}

	var response BatchFoldResponse
	for _, seq := range req.Sequences {
		result, err := h.router.Fold(r.Context(), router.FoldRequest{Sequence: seq})
		if err != nil {
			response.Errors = append(response.Errors, err.Error())
			continue
		}
		response.Results = append(response.Results, FoldResponse{
			PDB:      result.PDB,
			Sequence: result.Sequence,
			Source:   result.Source,
			ElapsedS: result.Elapsed.Seconds(),
			Cost:     result.Cost,
			Cached:   result.Source == router.SourceCache,
		})
	}

	writeJSON(w, response, http.StatusOK)
}

func (h *Handler) EnqueueFold(w http.ResponseWriter, r *http.Request) {
	var req FoldRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, "invalid request body", http.StatusBadRequest)
		return
	}

	if req.Sequence == "" && req.Accession == "" {
		writeError(w, "sequence or accession is required", http.StatusBadRequest)
		return
	}

	// cache check before enqueuing
	if req.Sequence != "" {
		result, err := h.router.Fold(r.Context(), router.FoldRequest{
			Sequence:  req.Sequence,
			Accession: req.Accession,
		})
		if err == nil && result.Source == router.SourceCache {
			writeJSON(w, FoldResponse{
				PDB:      result.PDB,
				Sequence: result.Sequence,
				Source:   result.Source,
				ElapsedS: result.Elapsed.Seconds(),
				Cost:     result.Cost,
				Cached:   true,
			}, http.StatusOK)
			return
		}
	}

	job := &queue.Job{
		ID:       uuid.New().String(),
		Sequence: req.Sequence,
		Status:   queue.StatusPending,
		Created:  time.Now(),
		Updated:  time.Now(),
	}

	if err := h.queue.Enqueue(r.Context(), job); err != nil {
		writeError(w, "failed to enqueue job", http.StatusInternalServerError)
		return
	}

	writeJSON(w, JobResponse{
		ID:      job.ID,
		Status:  job.Status,
		Created: job.Created,
		Updated: job.Updated,
	}, http.StatusAccepted)
}

func (h *Handler) GetJob(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if id == "" {
		writeError(w, "job id is required", http.StatusBadRequest)
		return
	}

	job, err := h.queue.GetJob(r.Context(), id)
	if err != nil {
		writeError(w, "job not found", http.StatusNotFound)
		return
	}

	writeJSON(w, JobResponse{
		ID:      job.ID,
		Status:  job.Status,
		Result:  job.Result,
		Error:   job.Error,
		Created: job.Created,
		Updated: job.Updated,
	}, http.StatusOK)
}

func (h *Handler) Health(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, map[string]string{"status": "ok"}, http.StatusOK)
}

func writeJSON(w http.ResponseWriter, v any, status int) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, msg string, status int) {
	writeJSON(w, ErrorResponse{Error: msg}, status)
}
