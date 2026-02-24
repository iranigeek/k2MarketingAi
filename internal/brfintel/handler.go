package brfintel

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"

	"k2MarketingAi/internal/auth"
	"k2MarketingAi/internal/storage"
)

// ──────────────────────────────────────────────────────────────────────
// HTTP Handler — BRF Intelligence API
// ──────────────────────────────────────────────────────────────────────

// Handler bundles dependencies for BRF intelligence endpoints.
type Handler struct {
	Analyzer *Analyzer
	Store    storage.Store
	BRFStore BRFReportStore
}

// Analyze handles POST /api/brf-intel/analyze.
// Accepts structured annual report data or raw text and returns a full BRF intelligence report.
func (h Handler) Analyze(w http.ResponseWriter, r *http.Request) {
	user, ok := userFromRequest(w, r)
	if !ok {
		return
	}

	var req AnalyzeRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "ogiltig JSON-payload: "+err.Error(), http.StatusBadRequest)
		return
	}

	if req.BRFName == "" {
		http.Error(w, "brf_name krävs", http.StatusBadRequest)
		return
	}

	report, err := h.Analyzer.Analyze(r.Context(), req, user.ID)
	if err != nil {
		status := http.StatusInternalServerError
		if strings.Contains(err.Error(), "ingen data") {
			status = http.StatusBadRequest
		}
		http.Error(w, fmt.Sprintf("analys misslyckades: %v", err), status)
		return
	}

	// Persist report
	if h.BRFStore != nil {
		if err := h.BRFStore.SaveBRFReport(r.Context(), report); err != nil {
			// Log but don't fail the response
			fmt.Printf("brfintel: failed to save report %s: %v\n", report.ID, err)
		}
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(AnalyzeResponse{Report: report})
}

// AnalyzeFromListing handles POST /api/brf-intel/analyze-listing/{id}.
// Pulls data from an existing listing's attached annual report.
func (h Handler) AnalyzeFromListing(w http.ResponseWriter, r *http.Request) {
	user, ok := userFromRequest(w, r)
	if !ok {
		return
	}

	listingID := chi.URLParam(r, "id")
	if listingID == "" {
		http.Error(w, "listing-id saknas", http.StatusBadRequest)
		return
	}

	// Get the listing
	listing, err := h.Store.GetListing(r.Context(), listingID)
	if err != nil {
		http.Error(w, "kunde inte hitta objektet", http.StatusNotFound)
		return
	}
	if listing.OwnerID != "" && listing.OwnerID != user.ID {
		http.Error(w, "åtkomst nekad", http.StatusForbidden)
		return
	}

	brfName := listing.Details.Association.Name
	if brfName == "" {
		brfName = listing.Association
	}
	if brfName == "" {
		brfName = "Okänd förening"
	}

	req := AnalyzeRequest{
		ListingID:    listingID,
		BRFName:      brfName,
		Municipality: listing.Details.Property.Municipality,
		City:         listing.Details.Property.City,
	}

	report, err := h.Analyzer.Analyze(r.Context(), req, user.ID)
	if err != nil {
		http.Error(w, fmt.Sprintf("analys misslyckades: %v", err), http.StatusInternalServerError)
		return
	}

	// Persist
	if h.BRFStore != nil {
		if err := h.BRFStore.SaveBRFReport(r.Context(), report); err != nil {
			fmt.Printf("brfintel: failed to save report %s: %v\n", report.ID, err)
		}
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(AnalyzeResponse{Report: report})
}

// Get handles GET /api/brf-intel/reports/{id}.
func (h Handler) Get(w http.ResponseWriter, r *http.Request) {
	user, ok := userFromRequest(w, r)
	if !ok {
		return
	}

	reportID := chi.URLParam(r, "id")
	if h.BRFStore == nil {
		http.Error(w, "BRF-rapporter stöds inte av denna lagringsbackend", http.StatusNotImplemented)
		return
	}

	report, err := h.BRFStore.GetBRFReport(r.Context(), reportID)
	if err != nil {
		http.Error(w, "rapport hittades inte", http.StatusNotFound)
		return
	}
	if report.OwnerID != "" && report.OwnerID != user.ID {
		http.Error(w, "åtkomst nekad", http.StatusForbidden)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(report)
}

// List handles GET /api/brf-intel/reports.
func (h Handler) List(w http.ResponseWriter, r *http.Request) {
	user, ok := userFromRequest(w, r)
	if !ok {
		return
	}

	brfStore := h.BRFStore
	if brfStore == nil {
		http.Error(w, "BRF-rapporter stöds inte av denna lagringsbackend", http.StatusNotImplemented)
		return
	}

	reports, err := brfStore.ListBRFReports(r.Context(), user.ID)
	if err != nil {
		http.Error(w, "kunde inte hämta rapporter", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(reports)
}

// Delete handles DELETE /api/brf-intel/reports/{id}.
func (h Handler) Delete(w http.ResponseWriter, r *http.Request) {
	user, ok := userFromRequest(w, r)
	if !ok {
		return
	}

	reportID := chi.URLParam(r, "id")
	brfStore := h.BRFStore
	if brfStore == nil {
		http.Error(w, "BRF-rapporter stöds inte av denna lagringsbackend", http.StatusNotImplemented)
		return
	}

	report, err := brfStore.GetBRFReport(r.Context(), reportID)
	if err != nil {
		http.Error(w, "rapport hittades inte", http.StatusNotFound)
		return
	}
	if report.OwnerID != "" && report.OwnerID != user.ID {
		http.Error(w, "åtkomst nekad", http.StatusForbidden)
		return
	}

	if err := brfStore.DeleteBRFReport(r.Context(), reportID); err != nil {
		http.Error(w, "kunde inte radera rapporten", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// ScoreQuick handles POST /api/brf-intel/score-quick.
// Returns just the score + risks without LLM summaries (fast, no AI calls).
func (h Handler) ScoreQuick(w http.ResponseWriter, r *http.Request) {
	if _, ok := userFromRequest(w, r); !ok {
		return
	}

	var fin Financials
	if err := json.NewDecoder(r.Body).Decode(&fin); err != nil {
		http.Error(w, "ogiltig JSON: "+err.Error(), http.StatusBadRequest)
		return
	}

	score := computeScore(fin)
	risks := detectRisks(fin)
	trends := computeTrends(fin)

	type quickResponse struct {
		Score  BRFScore      `json:"score"`
		Risks  []RiskWarning `json:"risks"`
		Trends EconomicTrend `json:"trends"`
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(quickResponse{
		Score:  score,
		Risks:  risks,
		Trends: trends,
	})
}

// ──────────────────────────────────────────────────────────────────────
// Storage interface for BRF reports
// ──────────────────────────────────────────────────────────────────────

// BRFReportStore extends storage.Store with BRF intelligence persistence.
type BRFReportStore interface {
	SaveBRFReport(ctx context.Context, report BRFReport) error
	GetBRFReport(ctx context.Context, id string) (BRFReport, error)
	ListBRFReports(ctx context.Context, ownerID string) ([]BRFReport, error)
	DeleteBRFReport(ctx context.Context, id string) error
}

// ──────────────────────────────────────────────────────────────────────
// In-memory BRF report store (for development / fallback)
// ──────────────────────────────────────────────────────────────────────

// InMemoryBRFStore provides thread-safe in-memory BRF report storage.
// It wraps a base storage.Store to satisfy both interfaces.
type InMemoryBRFStore struct {
	storage.Store
	reports map[string]BRFReport
}

// NewInMemoryBRFStore wraps an existing store with in-memory BRF report capabilities.
func NewInMemoryBRFStore(base storage.Store) *InMemoryBRFStore {
	return &InMemoryBRFStore{
		Store:   base,
		reports: make(map[string]BRFReport),
	}
}

func (s *InMemoryBRFStore) SaveBRFReport(_ context.Context, report BRFReport) error {
	report.UpdatedAt = time.Now()
	if report.CreatedAt.IsZero() {
		report.CreatedAt = report.UpdatedAt
	}
	s.reports[report.ID] = report
	return nil
}

func (s *InMemoryBRFStore) GetBRFReport(_ context.Context, id string) (BRFReport, error) {
	r, ok := s.reports[id]
	if !ok {
		return BRFReport{}, fmt.Errorf("BRF-rapport %s hittades inte", id)
	}
	return r, nil
}

func (s *InMemoryBRFStore) ListBRFReports(_ context.Context, ownerID string) ([]BRFReport, error) {
	var result []BRFReport
	for _, r := range s.reports {
		if ownerID == "" || r.OwnerID == ownerID {
			result = append(result, r)
		}
	}
	return result, nil
}

func (s *InMemoryBRFStore) DeleteBRFReport(_ context.Context, id string) error {
	delete(s.reports, id)
	return nil
}

// ──────────────────────────────────────────────────────────────────────
// Helpers
// ──────────────────────────────────────────────────────────────────────

func userFromRequest(w http.ResponseWriter, r *http.Request) (storage.User, bool) {
	user, ok := auth.UserFromContext(r.Context())
	if !ok {
		http.Error(w, "logga in först", http.StatusUnauthorized)
		return storage.User{}, false
	}
	return user, true
}
