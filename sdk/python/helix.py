"""
helix - Python SDK for the Helix protein folding inference server
"""

import time
import requests


class HelixError(Exception):
    pass


class HelixClient:
    """
    Client for the Helix protein folding inference server.

    Usage:
        client = HelixClient("http://localhost:8080")
        result = client.fold(sequence="MKTAYIAKQRQISFVK")
        print(result["source"])   # cache | afdb | esmfold
        print(result["cost"])     # 0 = free, 1 = used ESMFold quota
    """

    def __init__(self, base_url="http://localhost:8080", timeout=130):
        self.base_url = base_url.rstrip("/")
        self.timeout = timeout
        self.session = requests.Session()

    def fold(self, sequence=None, accession=None, experiment_id=None):
        """
        Fold a single sequence or accession.
        Routes automatically: cache -> AFDB -> ESMFold.

        Args:
            sequence: amino acid sequence string
            accession: UniProt accession ID (e.g. "P00520")
            experiment_id: optional experiment label for audit trail

        Returns:
            dict with keys: pdb, sequence, source, cost, cached, elapsed_seconds
        """
        if not sequence and not accession:
            raise HelixError("sequence or accession is required")

        payload = {}
        if sequence:
            payload["sequence"] = sequence
        if accession:
            payload["accession"] = accession
        if experiment_id:
            payload["experiment_id"] = experiment_id

        return self._post("/fold", payload)

    def fold_batch(self, sequences, experiment_id=None):
        """
        Fold up to 10 sequences concurrently.

        Args:
            sequences: list of amino acid sequence strings
            experiment_id: optional experiment label

        Returns:
            dict with keys: results (list), errors (list)
        """
        if not sequences:
            raise HelixError("sequences list is required")
        if len(sequences) > 10:
            raise HelixError("max 10 sequences per batch")

        payload = {"sequences": sequences}
        if experiment_id:
            payload["experiment_id"] = experiment_id

        return self._post("/fold/batch", payload)

    def fold_async(self, sequence, experiment_id=None):
        """
        Enqueue a fold job and return immediately with a job ID.
        Use get_job() to poll for results.

        Args:
            sequence: amino acid sequence string
            experiment_id: optional experiment label

        Returns:
            dict with keys: id, status, created, updated
        """
        if not sequence:
            raise HelixError("sequence is required")

        payload = {"sequence": sequence}
        if experiment_id:
            payload["experiment_id"] = experiment_id

        return self._post("/fold/async", payload)

    def get_job(self, job_id):
        """
        Poll for the result of an async fold job.

        Args:
            job_id: job ID returned by fold_async()

        Returns:
            dict with keys: id, status, result, error, created, updated
        """
        resp = self.session.get(
            f"{self.base_url}/fold/jobs/{job_id}",
            timeout=self.timeout,
        )
        self._raise_for_status(resp)
        return resp.json()

    def fold_and_wait(self, sequence, experiment_id=None, poll_interval=2, max_wait=300):
        """
        Enqueue a fold job and wait for it to complete.

        Args:
            sequence: amino acid sequence string
            experiment_id: optional experiment label
            poll_interval: seconds between polls (default 2)
            max_wait: max seconds to wait (default 300)

        Returns:
            dict with keys: id, status, result, error
        """
        job = self.fold_async(sequence, experiment_id=experiment_id)
        job_id = job["id"]

        elapsed = 0
        while elapsed < max_wait:
            result = self.get_job(job_id)
            if result["status"] in ("done", "failed"):
                return result
            time.sleep(poll_interval)
            elapsed += poll_interval

        raise HelixError(f"job {job_id} did not complete within {max_wait}s")

    def experiment_stats(self, experiment_id):
        """
        Get cost and routing stats for an experiment.

        Args:
            experiment_id: experiment label used in fold requests

        Returns:
            dict with keys: total, total_cost, avg_elapsed_ms,
                           cache_hits, afdb_hits, esmfold_hits
        """
        resp = self.session.get(
            f"{self.base_url}/experiments/{experiment_id}/stats",
            timeout=self.timeout,
        )
        self._raise_for_status(resp)
        return resp.json()

    def experiment_log(self, experiment_id):
        """
        Get full audit log for an experiment.

        Args:
            experiment_id: experiment label

        Returns:
            list of audit entries
        """
        resp = self.session.get(
            f"{self.base_url}/experiments/{experiment_id}",
            timeout=self.timeout,
        )
        self._raise_for_status(resp)
        return resp.json()

    def health(self):
        """Check if the server is running."""
        resp = self.session.get(
            f"{self.base_url}/health",
            timeout=10,
        )
        self._raise_for_status(resp)
        return resp.json()

    def _post(self, path, payload):
        resp = self.session.post(
            f"{self.base_url}{path}",
            json=payload,
            timeout=self.timeout,
        )
        self._raise_for_status(resp)
        return resp.json()

    def _raise_for_status(self, resp):
        if not resp.ok:
            try:
                error = resp.json().get("error", resp.text)
            except Exception:
                error = resp.text
            raise HelixError(f"helix error {resp.status_code}: {error}")