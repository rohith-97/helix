import requests

class HelixClient:
    def __init__(self, base_url="http://localhost:8080"):
        self.base_url = base_url

    def fold(self, sequence=None, accession=None, experiment_id=None):
        payload = {}
        if sequence:
            payload["sequence"] = sequence
        if accession:
            payload["accession"] = accession
        if experiment_id:
            payload["experiment_id"] = experiment_id
        r = requests.post(f"{self.base_url}/fold", json=payload)
        r.raise_for_status()
        return r.json()

    def fold_batch(self, sequences, experiment_id=None):
        payload = {"sequences": sequences}
        if experiment_id:
            payload["experiment_id"] = experiment_id
        r = requests.post(f"{self.base_url}/fold/batch", json=payload)
        r.raise_for_status()
        return r.json()

    def fold_async(self, sequence, experiment_id=None):
        payload = {"sequence": sequence}
        if experiment_id:
            payload["experiment_id"] = experiment_id
        r = requests.post(f"{self.base_url}/fold/async", json=payload)
        r.raise_for_status()
        return r.json()

    def get_job(self, job_id):
        r = requests.get(f"{self.base_url}/fold/jobs/{job_id}")
        r.raise_for_status()
        return r.json()

    def experiment_stats(self, experiment_id):
        r = requests.get(f"{self.base_url}/experiments/{experiment_id}/stats")
        r.raise_for_status()
        return r.json()