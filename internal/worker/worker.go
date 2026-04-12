package worker

import (
	"context"
	"log"
	"time"

	"github.com/redis/go-redis/v9"
	"github.com/yourusername/helix/internal/cache"
	"github.com/yourusername/helix/internal/esm"
	"github.com/yourusername/helix/internal/metrics"
	"github.com/yourusername/helix/internal/queue"
)

type Worker struct {
	queue  *queue.Queue
	client *esm.Client
	cache  *cache.Cache
}

func NewWorker(q *queue.Queue, client *esm.Client, c *cache.Cache) *Worker {
	return &Worker{
		queue:  q,
		client: client,
		cache:  c,
	}
}

func (w *Worker) Run(ctx context.Context) {
	log.Println("worker started")
	for {
		select {
		case <-ctx.Done():
			log.Println("worker stopped")
			return
		default:
			if err := w.processNext(ctx); err != nil {
				if err == redis.Nil {
					continue
				}
				log.Printf("worker error: %v", err)
				time.Sleep(1 * time.Second)
			}
		}
	}
}

func (w *Worker) processNext(ctx context.Context) error {
	job, err := w.queue.Dequeue(ctx)
	if err != nil {
		return err
	}

	log.Printf("processing job %s sequence_len=%d", job.ID, len(job.Sequence))

	job.Status = queue.StatusProcessing
	if err := w.queue.UpdateJob(ctx, job); err != nil {
		return err
	}

	metrics.FoldSequenceLength.Observe(float64(len(job.Sequence)))

	result, err := w.client.Fold(ctx, job.Sequence)
	if err != nil {
		job.Status = queue.StatusFailed
		job.Error = err.Error()
		metrics.FoldRequestsTotal.WithLabelValues("error").Inc()
		log.Printf("job %s failed: %v", job.ID, err)
	} else {
		job.Status = queue.StatusDone
		job.Result = result.PDB
		metrics.FoldRequestsTotal.WithLabelValues("success").Inc()
		metrics.FoldDuration.WithLabelValues("success").Observe(result.Elapsed.Seconds())
		log.Printf("job %s done elapsed=%.2fs", job.ID, result.Elapsed.Seconds())

		// write back to cache so future sync requests return instantly
		if err := w.cache.Set(ctx, job.Sequence, result.PDB); err != nil {
			log.Printf("cache writeback failed for job %s: %v", job.ID, err)
		} else {
			log.Printf("cache writeback success for job %s", job.ID)
		}
	}

	return w.queue.UpdateJob(ctx, job)
}
