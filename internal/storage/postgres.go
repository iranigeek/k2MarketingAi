package storage

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"time"

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
	historyJSON, err := json.Marshal(input.History)
	if err != nil {
		return Listing{}, fmt.Errorf("marshal history: %w", err)
	}

	statusJSON, err := json.Marshal(input.Status)
	if err != nil {
		return Listing{}, fmt.Errorf("marshal status: %w", err)
	}

	detailsJSON, err := json.Marshal(input.Details)
	if err != nil {
		return Listing{}, fmt.Errorf("marshal details: %w", err)
	}

	if _, err := s.pool.Exec(ctx,
		`INSERT INTO listings (id, address, tone, target_audience, highlights, image_url, fee, living_area, rooms, sections, full_copy, section_history, pipeline_status, details, insights, created_at) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16)`,
		input.ID, input.Address, input.Tone, input.TargetAudience, input.Highlights, input.ImageURL, input.Fee, input.LivingArea, input.Rooms, sectionsJSON, input.FullCopy, historyJSON, statusJSON, detailsJSON, insightsJSON, input.CreatedAt); err != nil {
		return Listing{}, fmt.Errorf("insert listing: %w", err)
	}

	return input, nil
}

// ListListings returns a slice of the most recent listings.
func (s *PostgresStore) ListListings(ctx context.Context) ([]Listing, error) {
	rows, err := s.pool.Query(ctx, `SELECT id, address, tone, target_audience, highlights, image_url, fee, living_area, rooms, sections, full_copy, section_history, pipeline_status, details, insights, created_at FROM listings ORDER BY created_at DESC LIMIT 50`)
	if err != nil {
		return nil, fmt.Errorf("query listings: %w", err)
	}
	defer rows.Close()

	listings := []Listing{}
	for rows.Next() {
		item, err := scanListing(rows)
		if err != nil {
			return nil, err
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

// GetListing fetches a single listing by ID.
func (s *PostgresStore) GetListing(ctx context.Context, id string) (Listing, error) {
	row := s.pool.QueryRow(ctx, `SELECT id, address, tone, target_audience, highlights, image_url, fee, living_area, rooms, sections, full_copy, section_history, pipeline_status, details, insights, created_at FROM listings WHERE id=$1`, id)
	item, err := scanListing(row)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return Listing{}, ErrNotFound
		}
		return Listing{}, err
	}
	return item, nil
}

// UpdateListingSections replaces the sections JSONB for a listing and returns the updated row.
func (s *PostgresStore) UpdateListingSections(ctx context.Context, id string, sections []Section, fullCopy string, history History, status Status) (Listing, error) {
	payload, err := json.Marshal(sections)
	if err != nil {
		return Listing{}, fmt.Errorf("marshal sections: %w", err)
	}

	historyJSON, err := json.Marshal(history)
	if err != nil {
		return Listing{}, fmt.Errorf("marshal history: %w", err)
	}

	statusJSON, err := json.Marshal(status)
	if err != nil {
		return Listing{}, fmt.Errorf("marshal status: %w", err)
	}

	row := s.pool.QueryRow(ctx, `UPDATE listings SET sections=$2, full_copy=$3, section_history=$4, pipeline_status=$5 WHERE id=$1 RETURNING id, address, tone, target_audience, highlights, image_url, fee, living_area, rooms, sections, full_copy, section_history, pipeline_status, details, insights, created_at`, id, payload, fullCopy, historyJSON, statusJSON)
	item, scanErr := scanListing(row)
	if scanErr != nil {
		if errors.Is(scanErr, pgx.ErrNoRows) {
			return Listing{}, ErrNotFound
		}
		return Listing{}, scanErr
	}
	return item, nil
}

// UpdateListingDetails replaces the details JSONB (including media) and optionally the cover image.
func (s *PostgresStore) UpdateListingDetails(ctx context.Context, id string, details Details, imageURL string) (Listing, error) {
	payload, err := json.Marshal(details)
	if err != nil {
		return Listing{}, fmt.Errorf("marshal details: %w", err)
	}

	row := s.pool.QueryRow(ctx, `UPDATE listings SET details=$2, image_url=$3 WHERE id=$1 RETURNING id, address, tone, target_audience, highlights, image_url, fee, living_area, rooms, sections, full_copy, section_history, pipeline_status, details, insights, created_at`, id, payload, imageURL)
	item, scanErr := scanListing(row)
	if scanErr != nil {
		if errors.Is(scanErr, pgx.ErrNoRows) {
			return Listing{}, ErrNotFound
		}
		return Listing{}, scanErr
	}
	return item, nil
}

// UpdateInsights replaces insights JSON and optionally pipeline status.
func (s *PostgresStore) UpdateInsights(ctx context.Context, id string, insights Insights, status Status) (Listing, error) {
	insightsJSON, err := json.Marshal(insights)
	if err != nil {
		return Listing{}, fmt.Errorf("marshal insights: %w", err)
	}

	statusJSON, err := json.Marshal(status)
	if err != nil {
		return Listing{}, fmt.Errorf("marshal status: %w", err)
	}

	row := s.pool.QueryRow(ctx, `UPDATE listings SET insights=$2, pipeline_status=$3 WHERE id=$1 RETURNING id, address, tone, target_audience, highlights, image_url, fee, living_area, rooms, sections, full_copy, section_history, pipeline_status, details, insights, created_at`, id, insightsJSON, statusJSON)
	item, scanErr := scanListing(row)
	if scanErr != nil {
		if errors.Is(scanErr, pgx.ErrNoRows) {
			return Listing{}, ErrNotFound
		}
		return Listing{}, scanErr
	}
	return item, nil
}

// DeleteListing removes a listing entirely.
func (s *PostgresStore) DeleteListing(ctx context.Context, id string) error {
	result, err := s.pool.Exec(ctx, `DELETE FROM listings WHERE id=$1`, id)
	if err != nil {
		return fmt.Errorf("delete listing: %w", err)
	}
	if result.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

// UpdateStatus updates only the pipeline status column.
func (s *PostgresStore) UpdateStatus(ctx context.Context, id string, status Status) error {
	payload, err := json.Marshal(status)
	if err != nil {
		return fmt.Errorf("marshal status: %w", err)
	}

	tag, err := s.pool.Exec(ctx, `UPDATE listings SET pipeline_status=$2 WHERE id=$1`, id, payload)
	if err != nil {
		return fmt.Errorf("update status: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

// SaveStyleProfile creates or updates a style profile.
func (s *PostgresStore) SaveStyleProfile(ctx context.Context, profile StyleProfile) (StyleProfile, error) {
	now := time.Now()
	if profile.ID == "" {
		profile.ID = uuid.NewString()
		profile.CreatedAt = now
	}
	profile.UpdatedAt = now
	if _, err := s.pool.Exec(ctx, `
		INSERT INTO style_profiles (id, name, description, tone, guidelines, example_texts, forbidden_words, created_at, updated_at)
		VALUES ($1,$2,$3,$4,$5,$6,$7,COALESCE($8, now()), $9)
		ON CONFLICT (id) DO UPDATE SET
			name=EXCLUDED.name,
			description=EXCLUDED.description,
			tone=EXCLUDED.tone,
			guidelines=EXCLUDED.guidelines,
			example_texts=EXCLUDED.example_texts,
			forbidden_words=EXCLUDED.forbidden_words,
			updated_at=EXCLUDED.updated_at
	`, profile.ID, profile.Name, profile.Description, profile.Tone, profile.Guidelines, profile.ExampleTexts, profile.ForbiddenWords, nullableTime(profile.CreatedAt), profile.UpdatedAt); err != nil {
		return StyleProfile{}, fmt.Errorf("save style profile: %w", err)
	}
	return s.GetStyleProfile(ctx, profile.ID)
}

func nullableTime(t time.Time) *time.Time {
	if t.IsZero() {
		return nil
	}
	return &t
}

// ListStyleProfiles returns all stored style profiles.
func (s *PostgresStore) ListStyleProfiles(ctx context.Context) ([]StyleProfile, error) {
	rows, err := s.pool.Query(ctx, `SELECT id, name, description, tone, guidelines, example_texts, forbidden_words, created_at, updated_at FROM style_profiles ORDER BY name`)
	if err != nil {
		return nil, fmt.Errorf("list style profiles: %w", err)
	}
	defer rows.Close()

	var profiles []StyleProfile
	for rows.Next() {
		profile, err := scanStyleProfile(rows)
		if err != nil {
			return nil, err
		}
		profiles = append(profiles, profile)
	}
	return profiles, nil
}

// GetStyleProfile fetches a profile by ID.
func (s *PostgresStore) GetStyleProfile(ctx context.Context, id string) (StyleProfile, error) {
	row := s.pool.QueryRow(ctx, `SELECT id, name, description, tone, guidelines, example_texts, forbidden_words, created_at, updated_at FROM style_profiles WHERE id=$1`, id)
	profile, err := scanStyleProfile(row)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return StyleProfile{}, ErrNotFound
		}
		return StyleProfile{}, err
	}
	return profile, nil
}

type rowScanner interface {
	Scan(dest ...any) error
}

func scanListing(row rowScanner) (Listing, error) {
	var (
		item         Listing
		imageURL     sql.NullString
		fee          sql.NullInt64
		livingArea   sql.NullFloat64
		rooms        sql.NullFloat64
		sectionsJSON []byte
		fullCopy     sql.NullString
		historyJSON  []byte
		statusJSON   []byte
		detailsJSON  []byte
		insightsJSON []byte
	)
	if err := row.Scan(&item.ID, &item.Address, &item.Tone, &item.TargetAudience, &item.Highlights, &imageURL, &fee, &livingArea, &rooms, &sectionsJSON, &fullCopy, &historyJSON, &statusJSON, &detailsJSON, &insightsJSON, &item.CreatedAt); err != nil {
		return Listing{}, fmt.Errorf("scan listing: %w", err)
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
			return Listing{}, fmt.Errorf("unmarshal sections: %w", err)
		}
	}
	if fullCopy.Valid {
		item.FullCopy = fullCopy.String
	}
	if len(historyJSON) > 0 {
		if err := json.Unmarshal(historyJSON, &item.History); err != nil {
			return Listing{}, fmt.Errorf("unmarshal history: %w", err)
		}
	}
	if len(statusJSON) > 0 {
		if err := json.Unmarshal(statusJSON, &item.Status); err != nil {
			return Listing{}, fmt.Errorf("unmarshal status: %w", err)
		}
	}
	if len(detailsJSON) > 0 {
		if err := json.Unmarshal(detailsJSON, &item.Details); err != nil {
			return Listing{}, fmt.Errorf("unmarshal details: %w", err)
		}
	}
	if len(insightsJSON) > 0 {
		if err := json.Unmarshal(insightsJSON, &item.Insights); err != nil {
			return Listing{}, fmt.Errorf("unmarshal insights: %w", err)
		}
	}
	return item, nil
}

func scanStyleProfile(row rowScanner) (StyleProfile, error) {
	var profile StyleProfile
	if err := row.Scan(&profile.ID, &profile.Name, &profile.Description, &profile.Tone, &profile.Guidelines, &profile.ExampleTexts, &profile.ForbiddenWords, &profile.CreatedAt, &profile.UpdatedAt); err != nil {
		return StyleProfile{}, fmt.Errorf("scan style profile: %w", err)
	}
	return profile, nil
}
