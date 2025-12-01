package storage

import (
	"context"
	"database/sql"
	"encoding/json"
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

	sectionsJSON, err := json.Marshal(input.Sections)
	if err != nil {
		return Listing{}, fmt.Errorf("marshal sections: %w", err)
	}

	insightsJSON, err := json.Marshal(input.Insights)
	if err != nil {
		return Listing{}, fmt.Errorf("marshal insights: %w", err)
	}

	if _, err := s.pool.Exec(ctx,
		`INSERT INTO listings (id, address, tone, target_audience, highlights, image_url, fee, living_area, rooms, sections, insights, created_at) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12)`,
		input.ID, input.Address, input.Tone, input.TargetAudience, input.Highlights, input.ImageURL, input.Fee, input.LivingArea, input.Rooms, sectionsJSON, insightsJSON, input.CreatedAt); err != nil {
		return Listing{}, fmt.Errorf("insert listing: %w", err)
	}

	return input, nil
}

// ListListings returns a slice of the most recent listings.
func (s *PostgresStore) ListListings(ctx context.Context) ([]Listing, error) {
	rows, err := s.pool.Query(ctx, `SELECT id, address, tone, target_audience, highlights, image_url, fee, living_area, rooms, sections, insights, created_at FROM listings ORDER BY created_at DESC LIMIT 50`)
	if err != nil {
		return nil, fmt.Errorf("query listings: %w", err)
	}
	defer rows.Close()

	listings := []Listing{}
	for rows.Next() {
		var (
			item         Listing
			imageURL     sql.NullString
			fee          sql.NullInt64
			livingArea   sql.NullFloat64
			rooms        sql.NullFloat64
			sectionsJSON []byte
			insightsJSON []byte
		)
		if err := rows.Scan(&item.ID, &item.Address, &item.Tone, &item.TargetAudience, &item.Highlights, &imageURL, &fee, &livingArea, &rooms, &sectionsJSON, &insightsJSON, &item.CreatedAt); err != nil {
			if err == pgx.ErrNoRows {
				break
			}
			return nil, fmt.Errorf("scan listing: %w", err)
		}
		if imageURL.Valid {
			item.ImageURL = imageURL.String
		}
		if fee.Valid {
			item.Fee = int(fee.Int64)
		}
		if livingArea.Valid {
			item.LivingArea = livingArea.Float64
		}
		if rooms.Valid {
			item.Rooms = rooms.Float64
		}
		if len(sectionsJSON) > 0 {
			if err := json.Unmarshal(sectionsJSON, &item.Sections); err != nil {
				return nil, fmt.Errorf("unmarshal sections: %w", err)
			}
		}
		if len(insightsJSON) > 0 {
			if err := json.Unmarshal(insightsJSON, &item.Insights); err != nil {
				return nil, fmt.Errorf("unmarshal insights: %w", err)
			}
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
