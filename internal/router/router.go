package router

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/yourusername/helix/internal/afdb"
	"github.com/yourusername/helix/internal/audit"
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
	Sequence     string
	Accession    string
	ExperimentID string
}

type FoldResult struct {
	PDB        string
	Sequence   string
	Accession  string
	Source     Source
	Confidence float64
	Elapsed    time.Duration
	Cost       int
}

type Router struct {
	cache *cache.Cache
	afdb  *afdb.Client
	esm   *esm.Client
	audit *audit.Logger
}

func NewRouter(c *cache.Cache, a *afdb.Client, e *esm.Client, al *audit.Logger) *Router {
	return &Router{
		cache: c,
		afdb:  a,
		esm:   e,
		audit: al,
	}
}

func (r *Router) Fold(ctx context.Context, req FoldRequest) (*FoldResult, error) {
	start := time.Now()

	// 1. cache check by sequence
	if req.Sequence != "" {
		if entry, err := r.cache.Get(ctx, req.Sequence); err == nil {
			metrics.FoldRequestsTotal.WithLabelValues("cache_hit").Inc()
			log.Printf("cache hit sequence_len=%d", len(req.Sequence))
			result := &FoldResult{
				PDB:      entry.PDB,
				Sequence: entry.Sequence,
				Source:   SourceCache,
				Elapsed:  time.Since(start),
				Cost:     0,
			}
			r.logAudit(ctx, req, result)
			return result, nil
		}
	}

	// 2. cache check by accession
	if req.Accession != "" {
		if entry, err := r.cache.Get(ctx, req.Accession); err == nil {
			metrics.FoldRequestsTotal.WithLabelValues("cache_hit").Inc()
			log.Printf("cache hit accession=%s", req.Accession)
			result := &FoldResult{
				PDB:      entry.PDB,
				Sequence: entry.Sequence,
				Source:   SourceCache,
				Elapsed:  time.Since(start),
				Cost:     0,
			}
			r.logAudit(ctx, req, result)
			return result, nil
		}
	}

	// 3. AFDB if accession provided
	if req.Accession != "" {
		afdbResult, err := r.afdb.Fold(ctx, req.Accession)
		if err == nil {
			sequence := req.Sequence
			if sequence == "" {
				sequence = afdbResult.Sequence
			}
			_ = r.cache.Set(ctx, sequence, afdbResult.PDB)
			_ = r.cache.Set(ctx, req.Accession, afdbResult.PDB)
			metrics.FoldRequestsTotal.WithLabelValues("afdb").Inc()
			metrics.FoldDuration.WithLabelValues("afdb").Observe(afdbResult.Elapsed.Seconds())
			log.Printf("afdb hit accession=%s elapsed=%.2fs", req.Accession, afdbResult.Elapsed.Seconds())
			result := &FoldResult{
				PDB:        afdbResult.PDB,
				Sequence:   sequence,
				Accession:  afdbResult.Accession,
				Source:     SourceAFDB,
				Confidence: afdbResult.Confidence,
				Elapsed:    time.Since(start),
				Cost:       0,
			}
			r.logAudit(ctx, req, result)
			return result, nil
		}
		log.Printf("afdb miss accession=%s err=%v falling back to esmfold", req.Accession, err)
	}

	// 4. ESMFold fallback
	if req.Sequence == "" {
		return nil, fmt.Errorf("sequence required when accession not found in AFDB")
	}

	metrics.FoldSequenceLength.Observe(float64(len(req.Sequence)))

	esmResult, err := r.esm.Fold(ctx, req.Sequence)
	if err != nil {
		metrics.FoldRequestsTotal.WithLabelValues("error").Inc()
		return nil, err
	}

	_ = r.cache.Set(ctx, req.Sequence, esmResult.PDB)
	metrics.FoldRequestsTotal.WithLabelValues("esmfold").Inc()
	metrics.FoldDuration.WithLabelValues("esmfold").Observe(esmResult.Elapsed.Seconds())
	log.Printf("esmfold hit sequence_len=%d elapsed=%.2fs", len(req.Sequence), esmResult.Elapsed.Seconds())

	result := &FoldResult{
		PDB:      esmResult.PDB,
		Sequence: esmResult.Sequence,
		Source:   SourceESMFold,
		Elapsed:  time.Since(start),
		Cost:     1,
	}
	r.logAudit(ctx, req, result)
	return result, nil
}

func (r *Router) logAudit(ctx context.Context, req FoldRequest, result *FoldResult) {
	if r.audit == nil {
		return
	}
	r.audit.Log(ctx, audit.Entry{
		SequenceHash: audit.HashSequence(result.Sequence),
		Accession:    result.Accession,
		SequenceLen:  len(result.Sequence),
		Source:       string(result.Source),
		Cost:         result.Cost,
		ElapsedMs:    int(result.Elapsed.Milliseconds()),
		ExperimentID: req.ExperimentID,
	})
}
