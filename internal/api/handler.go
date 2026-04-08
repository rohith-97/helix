package api

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/yourusername/helix/internal/esm"
	"github.com/yourusername/helix/internal/metrics"
	"github.com/yourusername/helix/internal/queue"
)

type Handler struct {
	esm   *esm.Client
	queue *queue.Queue
}

func NewHandler(client *esm.Client, q *queue.Queue) *Handler {
	return &Handler{esm: client, queue: q}
}

type FoldRequest struct {
	Sequence string `json:"sequence"`
}

type FoldResponse struct {
	PDB      string  `json:"pdb"`
	Sequence string  `json:"sequence"`
	ElapsedS float64 `json:"elapsed_seconds"`
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

	if req.Sequence == "" {
		writeError(w, "sequence is required", http.StatusBadRequest)
		return
	}

	metrics.FoldSequenceLength.Observe(float64(len(req.Sequence)))

	result, err := h.esm.Fold(r.Context(), req.Sequence)
	if err != nil {
		metrics.FoldRequestsTotal.WithLabelValues("error").Inc()
		metrics.FoldDuration.WithLabelValues("error").Observe(0)
		writeError(w, err.Error(), http.StatusInternalServerError)
		return
	}

	metrics.FoldRequestsTotal.WithLabelValues("success").Inc()
	metrics.FoldDuration.WithLabelValues("success").Observe(result.Elapsed.Seconds())

	writeJSON(w, FoldResponse{
		PDB:      result.PDB,
		Sequence: result.Sequence,
		ElapsedS: result.Elapsed.Seconds(),
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

	metrics.BatchSize.Observe(float64(len(req.Sequences)))

	results, errs := h.esm.FoldBatch(r.Context(), req.Sequences)

	var response BatchFoldResponse
	for i, result := range results {
		if errs[i] != nil {
			response.Errors = append(response.Errors, errs[i].Error())
			continue
		}
		metrics.FoldRequestsTotal.WithLabelValues("success").Inc()
		metrics.FoldDuration.WithLabelValues("success").Observe(result.Elapsed.Seconds())
		response.Results = append(response.Results, FoldResponse{
			PDB:      result.PDB,
			Sequence: result.Sequence,
			ElapsedS: result.Elapsed.Seconds(),
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

	if req.Sequence == "" {
		writeError(w, "sequence is required", http.StatusBadRequest)
		return
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
