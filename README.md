# helix

**Never compute the same protein structure twice.**

Helix is an open-source inference server that routes protein folding requests through three layers automatically:

1. **Cache** — seen this sequence before? Return instantly (~7ms, free)
2. **AlphaFold DB** — known protein? Fetch the precomputed structure (~1s, free)
3. **ESMFold** — novel sequence? Fold it and cache the result for next time (~30s)

For most research workloads, 60–90% of sequences hit cache or AFDB.
You only spend ESMFold quota on sequences nobody has ever folded before.

## Run in 30 seconds

```bash
git clone https://github.com/rohith-97/helix.git
cd helix
docker compose up
```

Server starts at `http://localhost:8080`.

## Use from Python

```python
# pip install requests
from sdk.python.helix import HelixClient

client = HelixClient("http://localhost:8080")

# Fold by sequence — routes automatically
result = client.fold(sequence="MKTAYIAKQRQISFVKSHFSRQLEERLGLIEVQAPILSRV")
print(result["source"])           # cache | afdb | esmfold
print(result["cost"])             # 0 = free, 1 = used ESMFold quota
print(result["elapsed_seconds"])  # how long it took

# Fold by UniProt accession — hits AlphaFold DB directly
result = client.fold(accession="P00520", experiment_id="EXP-001")

# Batch fold up to 10 sequences
result = client.fold_batch(
    sequences=["MKTAYIAKQRQISFVK", "ACDEFGHIKLMNPQRSTVWY"],
    experiment_id="EXP-001"
)

# Async fold for long sequences
job = client.fold_async("MKTAYIAKQRQISFVK...")
result = client.fold_and_wait(job["id"])

# Check experiment stats
stats = client.experiment_stats("EXP-001")
print(f"Cache hits:  {stats['cache_hits']}")
print(f"AFDB hits:   {stats['afdb_hits']}")
print(f"ESMFold:     {stats['esmfold_hits']}")
print(f"Total cost:  {stats['total_cost']}")
```

## Use from curl

```bash
# Fold a sequence
curl -X POST http://localhost:8080/fold \
  -H "Content-Type: application/json" \
  -d '{"sequence": "MKTAYIAKQRQISFVKSHFSRQLEERLGLIEVQAPILSRV", "experiment_id": "EXP-001"}'

# Fold by UniProt accession
curl -X POST http://localhost:8080/fold \
  -H "Content-Type: application/json" \
  -d '{"accession": "P00520", "experiment_id": "EXP-001"}'

# Async fold
curl -X POST http://localhost:8080/fold/async \
  -H "Content-Type: application/json" \
  -d '{"sequence": "MKTAYIAKQRQISFVK..."}'

# Poll result
curl http://localhost:8080/fold/jobs/<job_id>

# Experiment stats
curl http://localhost:8080/experiments/EXP-001/stats
```

## Architecture
request
│
├── cache hit?     → return ~7ms,  free
│
├── AFDB hit?      → return ~1.7s, free, cached
│
└── ESMFold        → return ~30s,  costs quota, cached

Every request is logged to PostgreSQL with experiment ID, source, cost, and latency.

## Endpoints

| Method | Path | Description |
|--------|------|-------------|
| POST | `/fold` | Fold sequence or accession (sync) |
| POST | `/fold/batch` | Fold up to 10 sequences concurrently |
| POST | `/fold/async` | Enqueue fold job, returns job ID |
| GET | `/fold/jobs/:id` | Poll async job result |
| GET | `/experiments/:id` | Full audit log for experiment |
| GET | `/experiments/:id/stats` | Cost and routing stats |
| GET | `/metrics` | Prometheus metrics |
| GET | `/health` | Health check |

## Stack

Go · chi · Redis · PostgreSQL · Prometheus · Docker · ESMFold API · AlphaFold DB

## Constraints

- Max sequence length: 400 residues (ESMFold API limit)
- Max batch size: 10 sequences
- Cache TTL: 24 hours
- Requires: Docker, or Go + Redis + PostgreSQL locally

## Contributing

Issues and PRs welcome. Especially interested in:
- FASTA file upload support
- Sequence similarity caching
- Priority queue tiers
- Webhook notifications for async jobs