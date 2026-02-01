package storage

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

// PostgresStore persists listings in PostgreSQL.
type PostgresStore struct {
	pool *pgxpool.Pool
}

const listingColumns = "id, owner_id, address, tone, target_audience, highlights, image_url, fee, living_area, rooms, sections, full_copy, section_history, pipeline_status, details, insights, created_at"

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
		`INSERT INTO listings (id, owner_id, address, tone, target_audience, highlights, image_url, fee, living_area, rooms, sections, full_copy, section_history, pipeline_status, details, insights, created_at) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16, $17)`,
		input.ID, input.OwnerID, input.Address, input.Tone, input.TargetAudience, input.Highlights, input.ImageURL, input.Fee, input.LivingArea, input.Rooms, sectionsJSON, input.FullCopy, historyJSON, statusJSON, detailsJSON, insightsJSON, input.CreatedAt); err != nil {
		return Listing{}, fmt.Errorf("insert listing: %w", err)
	}

	return input, nil
}

// ListListings returns a slice of the most recent listings.
func (s *PostgresStore) ListListings(ctx context.Context) ([]Listing, error) {
	return s.fetchListings(ctx, `SELECT `+listingColumns+` FROM listings ORDER BY created_at DESC LIMIT 50`)
}

// ListListingsByOwner returns recent listings for a specific owner.
func (s *PostgresStore) ListListingsByOwner(ctx context.Context, ownerID string) ([]Listing, error) {
	return s.fetchListings(ctx, `SELECT `+listingColumns+` FROM listings WHERE owner_id=$1 ORDER BY created_at DESC LIMIT 50`, ownerID)
}

// ListAllListings returns every stored listing (used for dataset exports).
func (s *PostgresStore) ListAllListings(ctx context.Context) ([]Listing, error) {
	return s.fetchListings(ctx, `SELECT `+listingColumns+` FROM listings ORDER BY created_at DESC`)
}

func (s *PostgresStore) fetchListings(ctx context.Context, query string, args ...any) ([]Listing, error) {
	rows, err := s.pool.Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("query listings: %w", err)
	}
	defer rows.Close()

	var listings []Listing
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
	row := s.pool.QueryRow(ctx, `SELECT `+listingColumns+` FROM listings WHERE id=$1`, id)
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

	row := s.pool.QueryRow(ctx, `UPDATE listings SET sections=$2, full_copy=$3, section_history=$4, pipeline_status=$5 WHERE id=$1 RETURNING `+listingColumns, id, payload, fullCopy, historyJSON, statusJSON)
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

	row := s.pool.QueryRow(ctx, `UPDATE listings SET details=$2, image_url=$3 WHERE id=$1 RETURNING `+listingColumns, id, payload, imageURL)
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

	row := s.pool.QueryRow(ctx, `UPDATE listings SET insights=$2, pipeline_status=$3 WHERE id=$1 RETURNING `+listingColumns, id, insightsJSON, statusJSON)
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
		INSERT INTO style_profiles (id, name, description, tone, guidelines, example_texts, forbidden_words, custom_model, dataset_uri, last_trained_at, created_at, updated_at)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,COALESCE($11, now()), $12)
		ON CONFLICT (id) DO UPDATE SET
			name=EXCLUDED.name,
			description=EXCLUDED.description,
			tone=EXCLUDED.tone,
			guidelines=EXCLUDED.guidelines,
			example_texts=EXCLUDED.example_texts,
			forbidden_words=EXCLUDED.forbidden_words,
			custom_model=EXCLUDED.custom_model,
			dataset_uri=EXCLUDED.dataset_uri,
			last_trained_at=EXCLUDED.last_trained_at,
			updated_at=EXCLUDED.updated_at
	`, profile.ID, profile.Name, profile.Description, profile.Tone, profile.Guidelines, profile.ExampleTexts, profile.ForbiddenWords, nullString(profile.CustomModel), nullString(profile.DatasetURI), profile.LastTrainedAt, nullableTime(profile.CreatedAt), profile.UpdatedAt); err != nil {
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

func nullString(value string) any {
	if strings.TrimSpace(value) == "" {
		return nil
	}
	return value
}

// ListStyleProfiles returns all stored style profiles.
func (s *PostgresStore) ListStyleProfiles(ctx context.Context) ([]StyleProfile, error) {
	rows, err := s.pool.Query(ctx, `SELECT id, name, description, tone, guidelines, example_texts, forbidden_words, custom_model, dataset_uri, last_trained_at, created_at, updated_at FROM style_profiles ORDER BY name`)
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
	row := s.pool.QueryRow(ctx, `SELECT id, name, description, tone, guidelines, example_texts, forbidden_words, custom_model, dataset_uri, last_trained_at, created_at, updated_at FROM style_profiles WHERE id=$1`, id)
	profile, err := scanStyleProfile(row)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return StyleProfile{}, ErrNotFound
		}
		return StyleProfile{}, err
	}
	return profile, nil
}

// CreateUser stores a new user account.
func (s *PostgresStore) CreateUser(ctx context.Context, user User) (User, error) {
	if user.ID == "" {
		user.ID = uuid.NewString()
	}
	if user.CreatedAt.IsZero() {
		user.CreatedAt = time.Now()
	}
	user.Email = strings.ToLower(strings.TrimSpace(user.Email))
	if _, err := s.pool.Exec(ctx, `INSERT INTO users (id, email, password_hash, created_at) VALUES ($1, $2, $3, $4)`, user.ID, user.Email, user.PasswordHash, user.CreatedAt); err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			return User{}, ErrUserExists
		}
		return User{}, fmt.Errorf("insert user: %w", err)
	}
	return user, nil
}

// GetUserByEmail fetches a user by their email address.
func (s *PostgresStore) GetUserByEmail(ctx context.Context, email string) (User, error) {
	row := s.pool.QueryRow(ctx, `SELECT id, email, password_hash, created_at FROM users WHERE email=$1`, strings.ToLower(strings.TrimSpace(email)))
	return scanUser(row)
}

// GetUserByID fetches a user by ID.
func (s *PostgresStore) GetUserByID(ctx context.Context, id string) (User, error) {
	row := s.pool.QueryRow(ctx, `SELECT id, email, password_hash, created_at FROM users WHERE id=$1`, id)
	return scanUser(row)
}

type rowScanner interface {
	Scan(dest ...any) error
}

func scanListing(row rowScanner) (Listing, error) {
	var (
		item         Listing
		ownerID      sql.NullString
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
	if err := row.Scan(&item.ID, &ownerID, &item.Address, &item.Tone, &item.TargetAudience, &item.Highlights, &imageURL, &fee, &livingArea, &rooms, &sectionsJSON, &fullCopy, &historyJSON, &statusJSON, &detailsJSON, &insightsJSON, &item.CreatedAt); err != nil {
		return Listing{}, fmt.Errorf("scan listing: %w", err)
	}
	if ownerID.Valid {
		item.OwnerID = ownerID.String
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
	var (
		profile     StyleProfile
		customModel sql.NullString
		datasetURI  sql.NullString
		lastTrained sql.NullTime
	)
	if err := row.Scan(&profile.ID, &profile.Name, &profile.Description, &profile.Tone, &profile.Guidelines, &profile.ExampleTexts, &profile.ForbiddenWords, &customModel, &datasetURI, &lastTrained, &profile.CreatedAt, &profile.UpdatedAt); err != nil {
		return StyleProfile{}, fmt.Errorf("scan style profile: %w", err)
	}
	if customModel.Valid {
		profile.CustomModel = customModel.String
	}
	if datasetURI.Valid {
		profile.DatasetURI = datasetURI.String
	}
	if lastTrained.Valid {
		t := lastTrained.Time
		profile.LastTrainedAt = &t
	}
	return profile, nil
}

func scanUser(row rowScanner) (User, error) {
	var user User
	if err := row.Scan(&user.ID, &user.Email, &user.PasswordHash, &user.CreatedAt); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return User{}, ErrNotFound
		}
		return User{}, fmt.Errorf("scan user: %w", err)
	}
	return user, nil
}
