package cache

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
)

const (
	cachePrefix = "helix:cache:"
	cacheTTL    = 24 * time.Hour
)

type Entry struct {
	PDB      string    `json:"pdb"`
	Sequence string    `json:"sequence"`
	CachedAt time.Time `json:"cached_at"`
}

type Cache struct {
	rdb *redis.Client
}

func NewCache(addr string) *Cache {
	return &Cache{
		rdb: redis.NewClient(&redis.Options{
			Addr: addr,
		}),
	}
}

func (c *Cache) key(sequence string) string {
	hash := sha256.Sum256([]byte(sequence))
	return cachePrefix + hex.EncodeToString(hash[:])
}

func (c *Cache) Get(ctx context.Context, sequence string) (*Entry, error) {
	data, err := c.rdb.Get(ctx, c.key(sequence)).Result()
	if err != nil {
		return nil, err
	}

	var entry Entry
	if err := json.Unmarshal([]byte(data), &entry); err != nil {
		return nil, fmt.Errorf("unmarshaling cache entry: %w", err)
	}

	return &entry, nil
}

func (c *Cache) Set(ctx context.Context, sequence, pdb string) error {
	entry := Entry{
		PDB:      pdb,
		Sequence: sequence,
		CachedAt: time.Now(),
	}

	data, err := json.Marshal(entry)
	if err != nil {
		return fmt.Errorf("marshaling cache entry: %w", err)
	}

	return c.rdb.Set(ctx, c.key(sequence), data, cacheTTL).Err()
}

func (c *Cache) Exists(ctx context.Context, sequence string) (bool, error) {
	n, err := c.rdb.Exists(ctx, c.key(sequence)).Result()
	return n > 0, err
}
