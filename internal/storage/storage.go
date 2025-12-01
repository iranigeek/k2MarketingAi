package storage

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

// Listing represents the metadata and generated insights for a real estate listing.
type Listing struct {
	ID             string    `json:"id"`
	Address        string    `json:"address"`
	Tone           string    `json:"tone"`
	TargetAudience string    `json:"target_audience"`
	Highlights     []string  `json:"highlights"`
	ImageURL       string    `json:"image_url,omitempty"`
	Fee            int       `json:"fee,omitempty"`
	LivingArea     float64   `json:"living_area,omitempty"`
	Rooms          float64   `json:"rooms,omitempty"`
	Sections       []Section `json:"sections,omitempty"`
	Insights       Insights  `json:"insights,omitempty"`
	CreatedAt      time.Time `json:"created_at"`
}

// Section represents an editable block of text in the listing description.
type Section struct {
	Slug    string `json:"slug"`
	Title   string `json:"title"`
	Content string `json:"content"`
}

// Insights aggregates AI/automation derived metadata for a listing.
type Insights struct {
	Geodata GeodataInsights `json:"geodata,omitempty"`
}

// GeodataInsights contains contextual information about the neighborhood.
type GeodataInsights struct {
	PointsOfInterest []PointOfInterest `json:"points_of_interest,omitempty"`
	Transit          []TransitInfo     `json:"transit,omitempty"`
}

// PointOfInterest represents a notable location near the property.
type PointOfInterest struct {
	Name     string `json:"name"`
	Category string `json:"category"`
	Distance string `json:"distance"`
}

// TransitInfo captures nearby transport options with estimated travel times.
type TransitInfo struct {
	Mode        string `json:"mode"`
	Description string `json:"description"`
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
        image_url TEXT,
        fee INTEGER,
        living_area DOUBLE PRECISION,
        rooms DOUBLE PRECISION,
        sections JSONB DEFAULT '[]'::jsonb,
        insights JSONB DEFAULT '{}'::jsonb,
        created_at TIMESTAMPTZ NOT NULL DEFAULT now()
    )`)
	if err != nil {
		return fmt.Errorf("create listings table: %w", err)
	}

	var schemaAlters = []string{
		`ALTER TABLE listings ADD COLUMN IF NOT EXISTS image_url TEXT`,
		`ALTER TABLE listings ADD COLUMN IF NOT EXISTS fee INTEGER`,
		`ALTER TABLE listings ADD COLUMN IF NOT EXISTS living_area DOUBLE PRECISION`,
		`ALTER TABLE listings ADD COLUMN IF NOT EXISTS rooms DOUBLE PRECISION`,
		`ALTER TABLE listings ADD COLUMN IF NOT EXISTS sections JSONB DEFAULT '[]'::jsonb`,
		`ALTER TABLE listings ADD COLUMN IF NOT EXISTS insights JSONB DEFAULT '{}'::jsonb`,
	}
	for _, stmt := range schemaAlters {
		if _, err := pool.Exec(ctx, stmt); err != nil {
			return fmt.Errorf("alter listings table: %w", err)
		}
	}

	return nil
}
