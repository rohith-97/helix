package queue

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
)

const (
	jobQueue   = "helix:jobs"
	jobResults = "helix:results:"
	jobTTL     = 1 * time.Hour
)

type JobStatus string

const (
	StatusPending    JobStatus = "pending"
	StatusProcessing JobStatus = "processing"
	StatusDone       JobStatus = "done"
	StatusFailed     JobStatus = "failed"
)

type Job struct {
	ID       string    `json:"id"`
	Sequence string    `json:"sequence"`
	Status   JobStatus `json:"status"`
	Result   string    `json:"result,omitempty"`
	Error    string    `json:"error,omitempty"`
	Created  time.Time `json:"created"`
	Updated  time.Time `json:"updated"`
}

type Queue struct {
	rdb *redis.Client
}

func NewQueue(addr string) *Queue {
	rdb := redis.NewClient(&redis.Options{
		Addr: addr,
	})
	return &Queue{rdb: rdb}
}

func (q *Queue) Enqueue(ctx context.Context, job *Job) error {
	data, err := json.Marshal(job)
	if err != nil {
		return fmt.Errorf("marshaling job: %w", err)
	}

	if err := q.rdb.Set(ctx, jobResults+job.ID, data, jobTTL).Err(); err != nil {
		return fmt.Errorf("storing job: %w", err)
	}

	if err := q.rdb.LPush(ctx, jobQueue, job.ID).Err(); err != nil {
		return fmt.Errorf("enqueueing job: %w", err)
	}

	return nil
}

func (q *Queue) Dequeue(ctx context.Context) (*Job, error) {
	result, err := q.rdb.BRPop(ctx, 30*time.Second, jobQueue).Result()
	if err != nil {
		return nil, err
	}

	jobID := result[1]
	return q.GetJob(ctx, jobID)
}

func (q *Queue) GetJob(ctx context.Context, id string) (*Job, error) {
	data, err := q.rdb.Get(ctx, jobResults+id).Result()
	if err != nil {
		return nil, fmt.Errorf("getting job %s: %w", id, err)
	}

	var job Job
	if err := json.Unmarshal([]byte(data), &job); err != nil {
		return nil, fmt.Errorf("unmarshaling job: %w", err)
	}

	return &job, nil
}

func (q *Queue) UpdateJob(ctx context.Context, job *Job) error {
	job.Updated = time.Now()
	data, err := json.Marshal(job)
	if err != nil {
		return fmt.Errorf("marshaling job: %w", err)
	}

	return q.rdb.Set(ctx, jobResults+job.ID, data, jobTTL).Err()
}

func (q *Queue) Ping(ctx context.Context) error {
	return q.rdb.Ping(ctx).Err()
}
