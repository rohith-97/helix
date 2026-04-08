# helix

A production-grade protein folding inference server built in Go, wrapping the ESMFold API with the infrastructure layer that real ML systems need.

## What it does

helix accepts amino acid sequences and returns predicted 3D protein structures in PDB format. It provides both synchronous and asynchronous folding, a Redis-backed result cache, job queue with worker, and Prometheus observability — the full stack you'd build around any expensive ML inference endpoint.

## Why I built this

Protein structure prediction is computationally expensive and slow — ESMFold takes 15–60 seconds per sequence. Research teams and biotech startups need more than a raw API call. They need:

- **Async job queuing** so clients aren't blocked waiting for inference
- **Result caching** so repeated sequences don't hit the API twice
- **Observability** so you can see latency, throughput, and cache hit rates
- **Batch support** so you can fold multiple sequences concurrently

helix is that infrastructure layer.

## Architecture

client
│
├── POST /fold          → cache check → ESMFold API → cache store
├── POST /fold/batch    → concurrent ESMFold API → cache store
├── POST /fold/async    → cache check → Redis job queue → 202 + job ID
│                                           │
│                                       background worker
│                                           │
└── GET /fold/jobs/:id  → Redis result store
GET /metrics             → Prometheus metrics
GET /health              → health check

## Endpoints

| Method | Path | Description |
|--------|------|-------------|
| POST | `/fold` | Fold a single sequence (sync, cached) |
| POST | `/fold/batch` | Fold up to 10 sequences concurrently |
| POST | `/fold/async` | Enqueue a fold job, returns job ID immediately |
| GET | `/fold/jobs/:id` | Poll for async job result |
| GET | `/health` | Health check |
| GET | `/metrics` | Prometheus metrics |

## Metrics

- `helix_fold_requests_total` — request count by status (success / error / cache_hit)
- `helix_fold_duration_seconds` — latency histogram
- `helix_fold_sequence_length` — sequence length distribution
- `helix_batch_size` — batch size distribution

## Quick start

```bash
# Start Redis
sudo systemctl start redis

# Clone and run
git clone https://github.com/rohith-97/helix.git
cd helix
go run cmd/helix/main.go
```

Fold a sequence synchronously:

```bash
curl -X POST http://localhost:8080/fold \
  -H "Content-Type: application/json" \
  -d '{"sequence": "MKTAYIAKQRQISFVKSHFSRQLEERLGLIEVQAPILSRV"}'
```

Fold asynchronously:

```bash
# Enqueue
curl -X POST http://localhost:8080/fold/async \
  -H "Content-Type: application/json" \
  -d '{"sequence": "MKTAYIAKQRQISFVKSHFSRQLEERLGLIEVQAPILSRV"}'

# Poll result
curl http://localhost:8080/fold/jobs/<job_id>
```

## Design decisions

- **SHA256 cache keys** — sequences are hashed before storage, safe for arbitrary length inputs
- **Concurrency limit of 3** on batch requests — respects ESMFold API rate limits
- **24h cache TTL** — structures don't change, long TTL is safe
- **Graceful shutdown** — in-flight requests and worker drain cleanly on SIGTERM
- **Max 400 residues** — ESMFold API constraint, validated at the handler layer

## Stack

Go · chi · Prometheus · Redis · ESMFold API
