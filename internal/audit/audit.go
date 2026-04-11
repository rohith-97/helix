package audit

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"log"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

type Entry struct {
	SequenceHash string
	Accession    string
	SequenceLen  int
	Source       string
	Cost         int
	ElapsedMs    int
	ExperimentID string
}

type Logger struct {
	pool *pgxpool.Pool
}

func NewLogger(ctx context.Context, connString string) (*Logger, error) {
	pool, err := pgxpool.New(ctx, connString)
	if err != nil {
		return nil, err
	}

	if err := pool.Ping(ctx); err != nil {
		return nil, err
	}

	return &Logger{pool: pool}, nil
}

func (l *Logger) Log(ctx context.Context, entry Entry) {
	_, err := l.pool.Exec(ctx,
		`INSERT INTO fold_audit
			(sequence_hash, accession, sequence_len, source, cost, elapsed_ms, experiment_id, created_at)
		VALUES
			($1, $2, $3, $4, $5, $6, $7, $8)`,
		entry.SequenceHash,
		entry.Accession,
		entry.SequenceLen,
		entry.Source,
		entry.Cost,
		entry.ElapsedMs,
		nullableString(entry.ExperimentID),
		time.Now(),
	)
	if err != nil {
		log.Printf("audit log error: %v", err)
	}
}

func (l *Logger) Query(ctx context.Context, experimentID string) ([]Entry, error) {
	rows, err := l.pool.Query(ctx,
		`SELECT sequence_hash, accession, sequence_len, source, cost, elapsed_ms, experiment_id
		 FROM fold_audit
		 WHERE experiment_id = $1
		 ORDER BY created_at DESC`,
		experimentID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var entries []Entry
	for rows.Next() {
		var e Entry
		var accession, experimentID *string
		if err := rows.Scan(
			&e.SequenceHash,
			&accession,
			&e.SequenceLen,
			&e.Source,
			&e.Cost,
			&e.ElapsedMs,
			&experimentID,
		); err != nil {
			return nil, err
		}
		if accession != nil {
			e.Accession = *accession
		}
		if experimentID != nil {
			e.ExperimentID = *experimentID
		}
		entries = append(entries, e)
	}

	return entries, nil
}

func (l *Logger) Stats(ctx context.Context, experimentID string) (map[string]any, error) {
	row := l.pool.QueryRow(ctx,
		`SELECT
			COUNT(*) as total,
			SUM(cost) as total_cost,
			AVG(elapsed_ms) as avg_elapsed_ms,
			COUNT(CASE WHEN source = 'cache' THEN 1 END) as cache_hits,
			COUNT(CASE WHEN source = 'afdb' THEN 1 END) as afdb_hits,
			COUNT(CASE WHEN source = 'esmfold' THEN 1 END) as esmfold_hits
		FROM fold_audit
		WHERE experiment_id = $1`,
		experimentID,
	)

	var total, totalCost, cacheHits, afdbHits, esmfoldHits int
	var avgElapsedMs float64
	if err := row.Scan(&total, &totalCost, &avgElapsedMs, &cacheHits, &afdbHits, &esmfoldHits); err != nil {
		return nil, err
	}

	return map[string]any{
		"total":          total,
		"total_cost":     totalCost,
		"avg_elapsed_ms": avgElapsedMs,
		"cache_hits":     cacheHits,
		"afdb_hits":      afdbHits,
		"esmfold_hits":   esmfoldHits,
	}, nil
}

func HashSequence(sequence string) string {
	hash := sha256.Sum256([]byte(sequence))
	return hex.EncodeToString(hash[:])
}

func nullableString(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}
