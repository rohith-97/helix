package metrics

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

var (
	FoldRequestsTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "helix_fold_requests_total",
			Help: "Total number of fold requests",
		},
		[]string{"status"},
	)

	FoldDuration = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "helix_fold_duration_seconds",
			Help:    "Duration of fold requests in seconds",
			Buckets: []float64{1, 5, 10, 30, 60, 90, 120},
		},
		[]string{"status"},
	)

	FoldSequenceLength = promauto.NewHistogram(
		prometheus.HistogramOpts{
			Name:    "helix_fold_sequence_length",
			Help:    "Length of sequences submitted for folding",
			Buckets: []float64{50, 100, 150, 200, 250, 300, 350, 400},
		},
	)

	BatchSize = promauto.NewHistogram(
		prometheus.HistogramOpts{
			Name:    "helix_batch_size",
			Help:    "Number of sequences per batch request",
			Buckets: []float64{1, 2, 3, 5, 10},
		},
	)
)
