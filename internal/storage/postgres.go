package storage

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// PostgresStore persists listings in PostgreSQL.
type PostgresStore struct {
	pool *pgxpool.Pool
}

// CreateListing stores the provided listing in PostgreSQL.
func (s *PostgresStore) CreateListing(ctx context.Context, input Listing) (Listing, error) {
	if input.ID == "" {
		input.ID = uuid.NewString()
	}

	if _, err := s.pool.Exec(ctx,
		`INSERT INTO listings (id, address, tone, target_audience, highlights, created_at) VALUES ($1, $2, $3, $4, $5, $6)`,
		input.ID, input.Address, input.Tone, input.TargetAudience, input.Highlights, input.CreatedAt); err != nil {
		return Listing{}, fmt.Errorf("insert listing: %w", err)
	}

	return input, nil
}

// ListListings returns a slice of the most recent listings.
func (s *PostgresStore) ListListings(ctx context.Context) ([]Listing, error) {
	rows, err := s.pool.Query(ctx, `SELECT id, address, tone, target_audience, highlights, created_at FROM listings ORDER BY created_at DESC LIMIT 50`)
	if err != nil {
		return nil, fmt.Errorf("query listings: %w", err)
	}
	defer rows.Close()

	listings := []Listing{}
	for rows.Next() {
		var item Listing
		if err := rows.Scan(&item.ID, &item.Address, &item.Tone, &item.TargetAudience, &item.Highlights, &item.CreatedAt); err != nil {
			if err == pgx.ErrNoRows {
				break
			}
			return nil, fmt.Errorf("scan listing: %w", err)
		}
		listings = append(listings, item)
	}

	return listings, nil
}

// Close releases database resources.
func (s *PostgresStore) Close() {
	if s.pool != nil {
		s.pool.Close()
	}
}
