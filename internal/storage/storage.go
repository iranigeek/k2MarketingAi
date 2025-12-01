package storage

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

// Listing represents the minimal metadata for a real estate listing.
type Listing struct {
	ID             string    `json:"id"`
	Address        string    `json:"address"`
	Tone           string    `json:"tone"`
	TargetAudience string    `json:"target_audience"`
	Highlights     []string  `json:"highlights"`
	CreatedAt      time.Time `json:"created_at"`
}

// Store defines the persistence behaviors the application relies on.
type Store interface {
	CreateListing(ctx context.Context, input Listing) (Listing, error)
	ListListings(ctx context.Context) ([]Listing, error)
	Close()
}

// NewStore selects a backing store based on whether a database URL is provided.
func NewStore(ctx context.Context, databaseURL string) (Store, error) {
	if databaseURL == "" {
		return NewInMemoryStore(), nil
	}

	pool, err := pgxpool.New(ctx, databaseURL)
	if err != nil {
		return nil, fmt.Errorf("create pool: %w", err)
	}

	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("ping database: %w", err)
	}

	if err := ensureSchema(ctx, pool); err != nil {
		pool.Close()
		return nil, err
	}

	return &PostgresStore{pool: pool}, nil
}

func ensureSchema(ctx context.Context, pool *pgxpool.Pool) error {
	_, err := pool.Exec(ctx, `CREATE TABLE IF NOT EXISTS listings (
        id TEXT PRIMARY KEY,
        address TEXT NOT NULL,
        tone TEXT NOT NULL,
        target_audience TEXT NOT NULL,
        highlights TEXT[],
        created_at TIMESTAMPTZ NOT NULL DEFAULT now()
    )`)
	if err != nil {
		return fmt.Errorf("create listings table: %w", err)
	}

	return nil
}
