package listings

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"

	"k2MarketingAi/internal/geodata"
	"k2MarketingAi/internal/media"
	"k2MarketingAi/internal/storage"
)

const (
	maxImageBytes = 5 * 1024 * 1024 // 5 MB
)

// Handler bundles dependencies for listing endpoints.
type Handler struct {
	Store       storage.Store
	Uploader    media.Uploader
	GeoProvider geodata.Provider
}

// CreateListingRequest describes inbound payload for creating a listing.
type CreateListingRequest struct {
	Address        string   `json:"address"`
	Tone           string   `json:"tone"`
	TargetAudience string   `json:"target_audience"`
	Highlights     []string `json:"highlights"`
	ImageURL       string   `json:"image_url,omitempty"`
	Fee            int      `json:"fee"`
	LivingArea     float64  `json:"living_area"`
	Rooms          float64  `json:"rooms"`
	Instructions   string   `json:"instructions"`
}

type uploadPayload struct {
	data        []byte
	filename    string
	contentType string
}

// Create handles POST /api/listings.
func (h Handler) Create(w http.ResponseWriter, r *http.Request) {
	var (
		req    CreateListingRequest
		upload *uploadPayload
		err    error
	)

	if strings.HasPrefix(r.Header.Get("Content-Type"), "multipart/form-data") {
		req, upload, err = parseMultipartRequest(r)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
	} else {
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "invalid request body", http.StatusBadRequest)
			return
		}
		trimCreateRequest(&req)
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

	imageURL := req.ImageURL
	if upload != nil {
		if h.Uploader == nil {
			http.Error(w, "image upload not configured", http.StatusInternalServerError)
			return
		}
		result, err := h.Uploader.Upload(r.Context(), media.UploadInput{
			Filename:    upload.filename,
			ContentType: upload.contentType,
			Body:        bytes.NewReader(upload.data),
			Size:        int64(len(upload.data)),
		})
		if err != nil {
			status := http.StatusInternalServerError
			if errors.Is(err, media.ErrUploaderDisabled) {
				status = http.StatusBadRequest
			} else {
				log.Printf("upload failed: %v", err)
			}
			http.Error(w, "could not store image", status)
			return
		}
		imageURL = result.URL
	}

	listing := storage.Listing{
		Address:        req.Address,
		Tone:           req.Tone,
		TargetAudience: req.TargetAudience,
		Highlights:     req.Highlights,
		ImageURL:       imageURL,
		Fee:            req.Fee,
		LivingArea:     req.LivingArea,
		Rooms:          req.Rooms,
		Sections:       buildDefaultSections(req, imageURL),
		Insights:       storage.Insights{},
		CreatedAt:      time.Now(),
	}

	if h.GeoProvider != nil {
		if summary, err := h.GeoProvider.Fetch(r.Context(), req.Address); err == nil {
			listing.Insights.Geodata = geodata.ToStorageInsights(summary)
		} else {
			log.Printf("geodata fetch failed: %v", err)
		}
	}

	listing, err = h.Store.CreateListing(r.Context(), listing)
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

func parseMultipartRequest(r *http.Request) (CreateListingRequest, *uploadPayload, error) {
	const maxFormMemory = maxImageBytes + (1 << 20)
	if err := r.ParseMultipartForm(maxFormMemory); err != nil {
		return CreateListingRequest{}, nil, fmt.Errorf("invalid multipart payload: %w", err)
	}

	req := CreateListingRequest{
		Address:        strings.TrimSpace(r.FormValue("address")),
		Tone:           strings.TrimSpace(r.FormValue("tone")),
		TargetAudience: strings.TrimSpace(r.FormValue("target_audience")),
		ImageURL:       strings.TrimSpace(r.FormValue("image_url")),
		Instructions:   strings.TrimSpace(r.FormValue("instructions")),
	}

	if highlightsRaw := strings.TrimSpace(r.FormValue("highlights")); highlightsRaw != "" {
		req.Highlights = splitHighlights(highlightsRaw)
	}

	if feeStr := strings.TrimSpace(r.FormValue("fee")); feeStr != "" {
		fee, err := strconv.Atoi(feeStr)
		if err != nil {
			return req, nil, fmt.Errorf("ogiltig avgift")
		}
		req.Fee = fee
	}
	if areaStr := strings.TrimSpace(r.FormValue("living_area")); areaStr != "" {
		area, err := strconv.ParseFloat(areaStr, 64)
		if err != nil {
			return req, nil, fmt.Errorf("ogiltig boarea")
		}
		req.LivingArea = area
	}
	if roomsStr := strings.TrimSpace(r.FormValue("rooms")); roomsStr != "" {
		rooms, err := strconv.ParseFloat(roomsStr, 64)
		if err != nil {
			return req, nil, fmt.Errorf("ogiltigt antal rum")
		}
		req.Rooms = rooms
	}

	file, header, err := r.FormFile("photo")
	if err != nil {
		if errors.Is(err, http.ErrMissingFile) {
			return req, nil, nil
		}
		return req, nil, fmt.Errorf("could not read image: %w", err)
	}
	defer file.Close()

	data, err := io.ReadAll(io.LimitReader(file, maxImageBytes+1))
	if err != nil {
		return req, nil, fmt.Errorf("read image: %w", err)
	}
	if len(data) > maxImageBytes {
		return req, nil, fmt.Errorf("bilden är för stor (max %d MB)", maxImageBytes/(1024*1024))
	}
	if len(data) == 0 {
		return req, nil, nil
	}

	contentType := header.Header.Get("Content-Type")
	if contentType == "" {
		contentType = http.DetectContentType(data)
	}

	return req, &uploadPayload{
		data:        data,
		filename:    header.Filename,
		contentType: contentType,
	}, nil
}

func splitHighlights(raw string) []string {
	chunks := strings.Split(raw, ",")
	values := make([]string, 0, len(chunks))
	for _, c := range chunks {
		if trimmed := strings.TrimSpace(c); trimmed != "" {
			values = append(values, trimmed)
		}
	}
	return values
}

func trimCreateRequest(req *CreateListingRequest) {
	req.Address = strings.TrimSpace(req.Address)
	req.Tone = strings.TrimSpace(req.Tone)
	req.TargetAudience = strings.TrimSpace(req.TargetAudience)
	req.ImageURL = strings.TrimSpace(req.ImageURL)
	req.Instructions = strings.TrimSpace(req.Instructions)
}

func buildDefaultSections(req CreateListingRequest, imageURL string) []storage.Section {
	return []storage.Section{
		{Slug: "intro", Title: "Inledning", Content: buildIntro(req, imageURL)},
		{Slug: "hall", Title: "Hall", Content: "Rymlig hall med gott om plats för förvaring och ett välkomnande första intryck."},
		{Slug: "kitchen", Title: "Kök", Content: "Stilrent kök med bra arbetsytor och harmonisk koppling till matplatsen."},
		{Slug: "living", Title: "Vardagsrum", Content: "Socialt vardagsrum med fina ljusinsläpp och plats för både soffgrupp och läshörna."},
		{Slug: "area", Title: "Området", Content: "Området bjuder på närhet till service, kommunikationer och rekreation."},
	}
}

func buildIntro(req CreateListingRequest, imageURL string) string {
	var b strings.Builder
	fmt.Fprintf(&b, "Välkommen till %s – en bostad som kombinerar ", req.Address)
	if req.Tone != "" {
		fmt.Fprintf(&b, "%s känsla och ", strings.ToLower(req.Tone))
	}
	fmt.Fprint(&b, "ett omsorgsfullt ljusinsläpp.")
	if imageURL != "" {
		fmt.Fprint(&b, " Hjältebilden ger en hint om atmosfären redan innan visningen.")
	}
	if len(req.Highlights) > 0 {
		fmt.Fprintf(&b, " Highlights: %s.", strings.Join(req.Highlights, ", "))
	}
	return b.String()
}
