package listings

import (
	"encoding/json"
	"net/http"
	"time"

	"k2MarketingAi/internal/storage"
)

// Handler bundles dependencies for listing endpoints.
type Handler struct {
	Store storage.Store
}

// CreateListingRequest describes inbound payload for creating a listing.
type CreateListingRequest struct {
	Address        string   `json:"address"`
	Tone           string   `json:"tone"`
	TargetAudience string   `json:"target_audience"`
	Highlights     []string `json:"highlights"`
}

// Create handles POST /api/listings.
func (h Handler) Create(w http.ResponseWriter, r *http.Request) {
	var req CreateListingRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}

	if req.Address == "" {
		http.Error(w, "address is required", http.StatusBadRequest)
		return
	}
	if req.Tone == "" {
		req.Tone = "Varm och familjär"
	}
	if req.TargetAudience == "" {
		req.TargetAudience = "Bred målgrupp"
	}

	listing, err := h.Store.CreateListing(r.Context(), storage.Listing{
		Address:        req.Address,
		Tone:           req.Tone,
		TargetAudience: req.TargetAudience,
		Highlights:     req.Highlights,
		CreatedAt:      time.Now(),
	})
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	_ = json.NewEncoder(w).Encode(listing)
}

// List handles GET /api/listings.
func (h Handler) List(w http.ResponseWriter, r *http.Request) {
	listings, err := h.Store.ListListings(r.Context())
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(listings)
}
