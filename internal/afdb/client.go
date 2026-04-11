package afdb

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

const (
	afdbAPI = "https://alphafold.ebi.ac.uk/api/prediction/"
)

type Client struct {
	http *http.Client
}

type Prediction struct {
	UniprotAccession string  `json:"uniprotAccession"`
	UniprotID        string  `json:"uniprotId"`
	Gene             string  `json:"gene"`
	Organism         string  `json:"organismScientificName"`
	Sequence         string  `json:"sequence"`
	PDBUrl           string  `json:"pdbUrl"`
	GlobalMetric     float64 `json:"globalMetricValue"`
	ModelCreatedDate string  `json:"modelCreatedDate"`
	LatestVersion    int     `json:"latestVersion"`
}

type FoldResult struct {
	PDB        string
	Accession  string
	Sequence   string
	Confidence float64
	Source     string
	Elapsed    time.Duration
}

func NewClient() *Client {
	return &Client{
		http: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

func (c *Client) Fold(ctx context.Context, accession string) (*FoldResult, error) {
	start := time.Now()

	pred, err := c.getPrediction(ctx, accession)
	if err != nil {
		return nil, err
	}

	pdb, err := c.fetchPDB(ctx, pred.PDBUrl)
	if err != nil {
		return nil, fmt.Errorf("fetching PDB: %w", err)
	}

	return &FoldResult{
		PDB:        pdb,
		Accession:  pred.UniprotAccession,
		Sequence:   pred.Sequence,
		Confidence: pred.GlobalMetric,
		Source:     "afdb",
		Elapsed:    time.Since(start),
	}, nil
}

func (c *Client) getPrediction(ctx context.Context, accession string) (*Prediction, error) {
	url := afdbAPI + accession

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("creating request: %w", err)
	}

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("calling AFDB API: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return nil, fmt.Errorf("accession %s not found in AFDB", accession)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("AFDB API returned %d", resp.StatusCode)
	}

	var predictions []Prediction
	if err := json.NewDecoder(resp.Body).Decode(&predictions); err != nil {
		return nil, fmt.Errorf("decoding response: %w", err)
	}

	if len(predictions) == 0 {
		return nil, fmt.Errorf("no predictions found for %s", accession)
	}

	return &predictions[0], nil
}

func (c *Client) fetchPDB(ctx context.Context, url string) (string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return "", fmt.Errorf("creating request: %w", err)
	}

	resp, err := c.http.Do(req)
	if err != nil {
		return "", fmt.Errorf("fetching PDB: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("PDB fetch returned %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("reading PDB: %w", err)
	}

	return string(body), nil
}
