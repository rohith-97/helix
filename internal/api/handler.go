package api

import (
	"encoding/json"
	"net/http"

	"github.com/yourusername/helix/internal/esm"
	"github.com/yourusername/helix/internal/metrics"
)

type Handler struct {
	esm *esm.Client
}

func NewHandler(client *esm.Client) *Handler {
	return &Handler{esm: client}
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
