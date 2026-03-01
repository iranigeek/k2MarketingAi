package brfintel

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

// ──────────────────────────────────────────────────────────────────────
// PostgreSQL storage for BRF Intelligence Reports
//
// Works alongside the existing storage.PostgresStore.
// The table is created lazily (idempotent DDL, no migration framework).
// ──────────────────────────────────────────────────────────────────────

// PostgresBRFStore wraps a pgx pool for BRF report persistence.
type PostgresBRFStore struct {
	pool *pgxpool.Pool
}

// NewPostgresBRFStore creates the store. The brf_reports table must already exist.
func NewPostgresBRFStore(pool *pgxpool.Pool) *PostgresBRFStore {
	return &PostgresBRFStore{pool: pool}
}

// SaveBRFReport upserts a BRF intelligence report.
func (s *PostgresBRFStore) SaveBRFReport(ctx context.Context, report BRFReport) error {
	scoreJSON, _ := json.Marshal(report.Score)
	risksJSON, _ := json.Marshal(report.Risks)
	trendsJSON, _ := json.Marshal(report.Trends)
	financialsJSON, _ := json.Marshal(report.Financials)
	sourceDocsJSON, _ := json.Marshal(report.SourceDocuments)

	var comparisonJSON []byte
	if report.Comparison != nil {
		comparisonJSON, _ = json.Marshal(report.Comparison)
	}

	now := time.Now()
	_, err := s.pool.Exec(ctx, `
		INSERT INTO brf_reports (
			id, owner_id, brf_name, org_number, municipality, city, listing_id,
			score, risks, trends, buyer_summary, legal_view, ad_text,
			financials, comparison, source_years, source_documents,
			created_at, updated_at
		) VALUES (
			$1, $2, $3, $4, $5, $6, $7,
			$8, $9, $10, $11, $12, $13,
			$14, $15, $16, $17,
			$18, $19
		)
		ON CONFLICT (id) DO UPDATE SET
			brf_name = EXCLUDED.brf_name,
			org_number = EXCLUDED.org_number,
			municipality = EXCLUDED.municipality,
			city = EXCLUDED.city,
			listing_id = EXCLUDED.listing_id,
			score = EXCLUDED.score,
			risks = EXCLUDED.risks,
			trends = EXCLUDED.trends,
			buyer_summary = EXCLUDED.buyer_summary,
			legal_view = EXCLUDED.legal_view,
			ad_text = EXCLUDED.ad_text,
			financials = EXCLUDED.financials,
			comparison = EXCLUDED.comparison,
			source_years = EXCLUDED.source_years,
			source_documents = EXCLUDED.source_documents,
			updated_at = EXCLUDED.updated_at`,
		report.ID, report.OwnerID, report.BRFName, report.OrgNumber,
		report.Municipality, report.City, report.ListingID,
		scoreJSON, risksJSON, trendsJSON, report.BuyerSummary, report.LegalView, report.AdText,
		financialsJSON, comparisonJSON, report.SourceYears, sourceDocsJSON,
		now, now,
	)
	if err != nil {
		return fmt.Errorf("save brf report: %w", err)
	}
	return nil
}

// GetBRFReport retrieves a single BRF report by ID.
func (s *PostgresBRFStore) GetBRFReport(ctx context.Context, id string) (BRFReport, error) {
	var r BRFReport
	var scoreJSON, risksJSON, trendsJSON, financialsJSON, sourceDocsJSON string
	var comparisonJSON *string

	err := s.pool.QueryRow(ctx, `
		SELECT id, owner_id, brf_name, org_number, municipality, city, listing_id,
			score::text, risks::text, trends::text, buyer_summary, legal_view, ad_text,
			financials::text, comparison::text, source_years, source_documents::text,
			created_at, updated_at
		FROM brf_reports WHERE id = $1`, id).Scan(
		&r.ID, &r.OwnerID, &r.BRFName, &r.OrgNumber, &r.Municipality, &r.City, &r.ListingID,
		&scoreJSON, &risksJSON, &trendsJSON, &r.BuyerSummary, &r.LegalView, &r.AdText,
		&financialsJSON, &comparisonJSON, &r.SourceYears, &sourceDocsJSON,
		&r.CreatedAt, &r.UpdatedAt,
	)
	if err != nil {
		return BRFReport{}, fmt.Errorf("get brf report %s: %w", id, err)
	}

	_ = json.Unmarshal([]byte(scoreJSON), &r.Score)
	_ = json.Unmarshal([]byte(risksJSON), &r.Risks)
	_ = json.Unmarshal([]byte(trendsJSON), &r.Trends)
	_ = json.Unmarshal([]byte(financialsJSON), &r.Financials)
	_ = json.Unmarshal([]byte(sourceDocsJSON), &r.SourceDocuments)
	if comparisonJSON != nil {
		var comp PeerComparison
		if err := json.Unmarshal([]byte(*comparisonJSON), &comp); err == nil {
			r.Comparison = &comp
		}
	}

	return r, nil
}

// ListBRFReports returns all reports owned by a user.
func (s *PostgresBRFStore) ListBRFReports(ctx context.Context, ownerID string) ([]BRFReport, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT id, owner_id, brf_name, org_number, municipality, city, listing_id,
			score::text, risks::text, trends::text, buyer_summary, legal_view, ad_text,
			financials::text, comparison::text, source_years, source_documents::text,
			created_at, updated_at
		FROM brf_reports
		WHERE owner_id = $1
		ORDER BY updated_at DESC`, ownerID)
	if err != nil {
		return nil, fmt.Errorf("list brf reports: %w", err)
	}
	defer rows.Close()

	var reports []BRFReport
	for rows.Next() {
		var r BRFReport
		var scoreJSON, risksJSON, trendsJSON, financialsJSON, sourceDocsJSON string
		var comparisonJSON *string

		if err := rows.Scan(
			&r.ID, &r.OwnerID, &r.BRFName, &r.OrgNumber, &r.Municipality, &r.City, &r.ListingID,
			&scoreJSON, &risksJSON, &trendsJSON, &r.BuyerSummary, &r.LegalView, &r.AdText,
			&financialsJSON, &comparisonJSON, &r.SourceYears, &sourceDocsJSON,
			&r.CreatedAt, &r.UpdatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan brf report: %w", err)
		}

		_ = json.Unmarshal([]byte(scoreJSON), &r.Score)
		_ = json.Unmarshal([]byte(risksJSON), &r.Risks)
		_ = json.Unmarshal([]byte(trendsJSON), &r.Trends)
		_ = json.Unmarshal([]byte(financialsJSON), &r.Financials)
		_ = json.Unmarshal([]byte(sourceDocsJSON), &r.SourceDocuments)
		if comparisonJSON != nil {
			var comp PeerComparison
			if err := json.Unmarshal([]byte(*comparisonJSON), &comp); err == nil {
				r.Comparison = &comp
			}
		}

		reports = append(reports, r)
	}

	return reports, nil
}

// DeleteBRFReport removes a report by ID.
func (s *PostgresBRFStore) DeleteBRFReport(ctx context.Context, id string) error {
	_, err := s.pool.Exec(ctx, `DELETE FROM brf_reports WHERE id = $1`, id)
	if err != nil {
		return fmt.Errorf("delete brf report: %w", err)
	}
	return nil
}

// GetBRFReportByListing retrieves the most recent BRF report linked to a listing.
func (s *PostgresBRFStore) GetBRFReportByListing(ctx context.Context, listingID string) (BRFReport, error) {
	var r BRFReport
	var scoreJSON, risksJSON, trendsJSON, financialsJSON, sourceDocsJSON string
	var comparisonJSON *string

	err := s.pool.QueryRow(ctx, `
		SELECT id, owner_id, brf_name, org_number, municipality, city, listing_id,
			score::text, risks::text, trends::text, buyer_summary, legal_view, ad_text,
			financials::text, comparison::text, source_years, source_documents::text,
			created_at, updated_at
		FROM brf_reports WHERE listing_id = $1
		ORDER BY updated_at DESC LIMIT 1`, listingID).Scan(
		&r.ID, &r.OwnerID, &r.BRFName, &r.OrgNumber, &r.Municipality, &r.City, &r.ListingID,
		&scoreJSON, &risksJSON, &trendsJSON, &r.BuyerSummary, &r.LegalView, &r.AdText,
		&financialsJSON, &comparisonJSON, &r.SourceYears, &sourceDocsJSON,
		&r.CreatedAt, &r.UpdatedAt,
	)
	if err != nil {
		return BRFReport{}, fmt.Errorf("get brf report by listing %s: %w", listingID, err)
	}

	_ = json.Unmarshal([]byte(scoreJSON), &r.Score)
	_ = json.Unmarshal([]byte(risksJSON), &r.Risks)
	_ = json.Unmarshal([]byte(trendsJSON), &r.Trends)
	_ = json.Unmarshal([]byte(financialsJSON), &r.Financials)
	_ = json.Unmarshal([]byte(sourceDocsJSON), &r.SourceDocuments)
	if comparisonJSON != nil {
		var comp PeerComparison
		if err := json.Unmarshal([]byte(*comparisonJSON), &comp); err == nil {
			r.Comparison = &comp
		}
	}

	return r, nil
}
