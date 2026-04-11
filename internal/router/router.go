package router

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/yourusername/helix/internal/afdb"
	"github.com/yourusername/helix/internal/cache"
	"github.com/yourusername/helix/internal/esm"
	"github.com/yourusername/helix/internal/metrics"
)

type Source string

const (
	SourceCache   Source = "cache"
	SourceAFDB    Source = "afdb"
	SourceESMFold Source = "esmfold"
)

type FoldRequest struct {
	Sequence  string
	Accession string // optional UniProt accession
}

type FoldResult struct {
	PDB        string
	Sequence   string
	Accession  string
	Source     Source
	Confidence float64
	Elapsed    time.Duration
	Cost       int // 0=free, 1=costs quota
}

type Router struct {
	cache *cache.Cache
	afdb  *afdb.Client
	esm   *esm.Client
}

func NewRouter(c *cache.Cache, a *afdb.Client, e *esm.Client) *Router {
	return &Router{
		cache: c,
		afdb:  a,
		esm:   e,
	}
}

func (r *Router) Fold(ctx context.Context, req FoldRequest) (*FoldResult, error) {
	start := time.Now()

	// 1. cache check by sequence if provided
	if req.Sequence != "" {
		if entry, err := r.cache.Get(ctx, req.Sequence); err == nil {
			metrics.FoldRequestsTotal.WithLabelValues("cache_hit").Inc()
			log.Printf("cache hit sequence_len=%d", len(req.Sequence))
			return &FoldResult{
				PDB:      entry.PDB,
				Sequence: entry.Sequence,
				Source:   SourceCache,
				Elapsed:  time.Since(start),
				Cost:     0,
			}, nil
		}
	}

	// 2. cache check by accession if provided
	if req.Accession != "" {
		if entry, err := r.cache.Get(ctx, req.Accession); err == nil {
			metrics.FoldRequestsTotal.WithLabelValues("cache_hit").Inc()
			log.Printf("cache hit accession=%s", req.Accession)
			return &FoldResult{
				PDB:      entry.PDB,
				Sequence: entry.Sequence,
				Source:   SourceCache,
				Elapsed:  time.Since(start),
				Cost:     0,
			}, nil
		}
	}

	// 3. AFDB if accession provided
	if req.Accession != "" {
		result, err := r.afdb.Fold(ctx, req.Accession)
		if err == nil {
			sequence := req.Sequence
			if sequence == "" {
				sequence = result.Sequence
			}
			_ = r.cache.Set(ctx, sequence, result.PDB)
			_ = r.cache.Set(ctx, req.Accession, result.PDB)
			metrics.FoldRequestsTotal.WithLabelValues("afdb").Inc()
			metrics.FoldDuration.WithLabelValues("afdb").Observe(result.Elapsed.Seconds())
			log.Printf("afdb hit accession=%s elapsed=%.2fs", req.Accession, result.Elapsed.Seconds())
			return &FoldResult{
				PDB:        result.PDB,
				Sequence:   sequence,
				Accession:  result.Accession,
				Source:     SourceAFDB,
				Confidence: result.Confidence,
				Elapsed:    time.Since(start),
				Cost:       0,
			}, nil
		}
		log.Printf("afdb miss accession=%s err=%v falling back to esmfold", req.Accession, err)
	}

	// 4. ESMFold fallback
	if req.Sequence == "" {
		return nil, fmt.Errorf("sequence required when accession not found in AFDB")
	}

	metrics.FoldSequenceLength.Observe(float64(len(req.Sequence)))

	result, err := r.esm.Fold(ctx, req.Sequence)
	if err != nil {
		metrics.FoldRequestsTotal.WithLabelValues("error").Inc()
		return nil, err
	}

	_ = r.cache.Set(ctx, req.Sequence, result.PDB)
	metrics.FoldRequestsTotal.WithLabelValues("esmfold").Inc()
	metrics.FoldDuration.WithLabelValues("esmfold").Observe(result.Elapsed.Seconds())
	log.Printf("esmfold hit sequence_len=%d elapsed=%.2fs", len(req.Sequence), result.Elapsed.Seconds())

	return &FoldResult{
		PDB:      result.PDB,
		Sequence: result.Sequence,
		Source:   SourceESMFold,
		Elapsed:  time.Since(start),
		Cost:     1,
	}, nil
}
