# helix Python SDK

Simple Python client for the Helix protein folding inference server.

## Install

```bash
pip install requests
```

## Quickstart

```python
from helix import HelixClient

client = HelixClient("http://localhost:8080")

# Fold by sequence (cache -> AFDB -> ESMFold automatically)
result = client.fold(sequence="MKTAYIAKQRQISFVKSHFSRQLEERLGLIEVQAPILSRV")
print(result["source"])          # cache | afdb | esmfold
print(result["cost"])            # 0 = free, 1 = used ESMFold quota
print(result["elapsed_seconds"]) # how long it took

# Fold by UniProt accession (hits AlphaFold DB directly)
result = client.fold(accession="P00520", experiment_id="EXP-001")
print(result["source"])          # afdb

# Fold a batch of sequences
result = client.fold_batch(
    sequences=[
        "MKTAYIAKQRQISFVK",
        "ACDEFGHIKLMNPQRSTVWY",
    ],
    experiment_id="EXP-001"
)
for r in result["results"]:
    print(r["source"], r["cost"])

# Async fold — enqueue and poll
job = client.fold_async("MKTAYIAKQRQISFVK", experiment_id="EXP-001")
print(job["id"])

result = client.get_job(job["id"])
print(result["status"])  # pending | processing | done | failed

# Or just wait for it
result = client.fold_and_wait("MKTAYIAKQRQISFVK", experiment_id="EXP-001")
print(result["status"])  # done

# Check experiment stats
stats = client.experiment_stats("EXP-001")
print(f"Total folds: {stats['total']}")
print(f"Cache hits:  {stats['cache_hits']}")
print(f"AFDB hits:   {stats['afdb_hits']}")
print(f"ESMFold:     {stats['esmfold_hits']}")
print(f"Total cost:  {stats['total_cost']}")
```

## Why helix

Protein folding is expensive. Helix routes every request through three layers:

1. **Cache** — if this sequence was folded before, return instantly (~7ms, free)
2. **AlphaFold DB** — if the protein is known, fetch the precomputed structure (~1s, free)
3. **ESMFold** — only for genuinely novel sequences (~30s, costs quota)

For most research workloads, 60–90% of sequences hit cache or AFDB.
You only pay ESMFold quota for sequences nobody has ever folded before.

## Running the server

```bash
git clone https://github.com/rohith-97/helix.git
cd helix
docker compose up
```

Server starts at `http://localhost:8080`.