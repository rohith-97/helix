# helix

A high-performance protein folding inference server built in Go, wrapping the ESMFold API with production-grade infrastructure.

## What it does

helix accepts amino acid sequences and returns predicted 3D protein structures in PDB format. It routes requests through Meta's ESMFold API and adds the infrastructure layer that production ML systems need: async handling, batch support, observability, and graceful shutdown.

## Why I built this

Protein structure prediction is computationally expensive. Research teams and biotech startups running ESMFold need more than a raw API call — they need request batching, latency tracking, and a server that behaves predictably under load. helix is that layer.

## Architecture

client → chi router → handler → esm client → ESMFold API
↓
prometheus metrics

## Endpoints

| Method | Path | Description |
|--------|------|-------------|
| POST | `/fold` | Fold a single sequence |
| POST | `/fold/batch` | Fold up to 10 sequences concurrently |
| GET | `/health` | Health check |
| GET | `/metrics` | Prometheus metrics |

## Metrics

- `helix_fold_requests_total` — request count by status
- `helix_fold_duration_seconds` — latency histogram
- `helix_fold_sequence_length` — sequence length distribution
- `helix_batch_size` — batch size distribution

## Quick start

```bash
git clone https://github.com/rohith-97/helix.git
cd helix
go run cmd/helix/main.go
```

Fold a sequence:

```bash
curl -X POST http://localhost:8080/fold \
  -H "Content-Type: application/json" \
  -d '{"sequence": "MKTAYIAKQRQISFVKSHFSRQLEERLGLIEVQAPILSRV"}'
```

## Stack

Go · chi · Prometheus · ESMFold API

## Constraints and design decisions

- Max sequence length: 400 residues (ESMFold API limit)
- Max batch size: 10 sequences
- Batch concurrency: 3 parallel requests (rate limit aware)
- Request timeout: 120 seconds