package esm

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

const esmFoldAPI = "https://api.esmatlas.com/foldSequence/v1/pdb/"

type Client struct {
	http    *http.Client
}

type FoldResult struct {
	PDB       string
	Sequence  string
	Elapsed   time.Duration
}

func NewClient() *Client {
	return &Client{
		http: &http.Client{
			Timeout: 120 * time.Second,
		},
	}
}

func (c *Client) Fold(ctx context.Context, sequence string) (*FoldResult, error) {
	if len(sequence) == 0 {
		return nil, fmt.Errorf("empty sequence")
	}
	if len(sequence) > 400 {
		return nil, fmt.Errorf("sequence too long: max 400 residues, got %d", len(sequence))
	}

	start := time.Now()

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, esmFoldAPI, bytes.NewBufferString(sequence))
	if err != nil {
		return nil, fmt.Errorf("creating request: %w", err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("calling ESMFold API: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("ESMFold API returned %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("reading response: %w", err)
	}

	return &FoldResult{
		PDB:      string(body),
		Sequence: sequence,
		Elapsed:  time.Since(start),
	}, nil
}

func (c *Client) FoldBatch(ctx context.Context, sequences []string) ([]*FoldResult, []error) {
	results := make([]*FoldResult, len(sequences))
	errors := make([]error, len(sequences))

	type job struct {
		index    int
		sequence string
	}

	jobs := make(chan job, len(sequences))
	for i, seq := range sequences {
		jobs <- job{i, seq}
	}
	close(jobs)

	sem := make(chan struct{}, 3)

	done := make(chan struct{})
	pending := len(sequences)

	resultCh := make(chan struct {
		index  int
		result *FoldResult
		err    error
	}, len(sequences))

	for j := range jobs {
		j := j
		go func() {
			sem <- struct{}{}
			defer func() { <-sem }()
			r, err := c.Fold(ctx, j.sequence)
			resultCh <- struct {
				index  int
				result *FoldResult
				err    error
			}{j.index, r, err}
		}()
	}

	go func() {
		for i := 0; i < pending; i++ {
			r := <-resultCh
			results[r.index] = r.result
			errors[r.index] = r.err
		}
		close(done)
	}()

	<-done
	_ = json.Marshal // keep import
	return results, errors
}