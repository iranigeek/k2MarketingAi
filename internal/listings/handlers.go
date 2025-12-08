package listings

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"

	"k2MarketingAi/internal/events"
	"k2MarketingAi/internal/generation"
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
	Generator   generation.Generator
	Events      *events.Broker
}

// CreateListingRequest describes inbound payload for creating a listing.
type CreateListingRequest struct {
	Address        string         `json:"address"`
	Tone           string         `json:"tone"`
	TargetAudience string         `json:"target_audience"`
	Highlights     []string       `json:"highlights"`
	ImageURL       string         `json:"image_url,omitempty"`
	Fee            int            `json:"fee"`
	LivingArea     float64        `json:"living_area"`
	Rooms          float64        `json:"rooms"`
	Instructions   string         `json:"instructions"`
	Sections       []SectionInput `json:"sections"`
}

// SectionInput allows custom section configuration from the client.
type SectionInput struct {
	Slug    string `json:"slug"`
	Title   string `json:"title"`
	Content string `json:"content"`
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
		Sections:       buildSectionsFromInput(req, imageURL),
		History:        storage.History{},
		Insights:       storage.Insights{},
		CreatedAt:      time.Now(),
	}
	hydrateDetailsFromLegacy(&listing)

	if h.GeoProvider != nil {
		if summary, err := h.GeoProvider.Fetch(r.Context(), req.Address); err == nil {
			listing.Insights.Geodata = geodata.ToStorageInsights(summary)
		} else {
			log.Printf("geodata fetch failed: %v", err)
		}
	}

	if h.Generator != nil {
		if result, genErr := h.Generator.Generate(r.Context(), listing); genErr == nil {
			listing.Sections = result.Sections
			if strings.TrimSpace(result.FullCopy) != "" {
				listing.FullCopy = result.FullCopy
			}
		} else {
			log.Printf("generator failed, using fallback sections: %v", genErr)
		}
	}
	recordHistoryForAll(&listing, "generate")
	if listing.FullCopy == "" {
		listing.FullCopy = composeFullCopy(listing.Sections)
	}
	deriveStatus(&listing)

	listing, err = h.Store.CreateListing(r.Context(), listing)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	h.publishListing(listing)
	go h.runPipeline(listing)

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

	for i := range listings {
		hydrateDetailsFromLegacy(&listings[i])
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(listings)
}

// Get returns a single listing by id.
func (h Handler) Get(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if id == "" {
		http.Error(w, "id is required", http.StatusBadRequest)
		return
	}

	listing, err := h.Store.GetListing(r.Context(), id)
	if err != nil {
		if errors.Is(err, storage.ErrNotFound) {
			http.NotFound(w, r)
			return
		}
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	hydrateDetailsFromLegacy(&listing)
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(listing)
}

// RewriteSection accepts instructions and rewrites a section using the generator.
func (h Handler) RewriteSection(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	slug := chi.URLParam(r, "slug")
	if id == "" || slug == "" {
		http.Error(w, "id and slug are required", http.StatusBadRequest)
		return
	}

	var req struct {
		Instruction string `json:"instruction"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}

	listing, err := h.Store.GetListing(r.Context(), id)
	if err != nil {
		if errors.Is(err, storage.ErrNotFound) {
			http.NotFound(w, r)
			return
		}
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	idx := findSectionIndex(listing.Sections, slug)
	if idx == -1 {
		http.Error(w, "section not found", http.StatusNotFound)
		return
	}

	section := listing.Sections[idx]
	fallbackUsed := false
	if h.Generator != nil {
		if updated, genErr := h.Generator.Rewrite(r.Context(), listing, section, req.Instruction); genErr == nil {
			section = updated
		} else {
			log.Printf("rewrite fallback: %v", genErr)
			fallbackUsed = true
			section.Content = generation.ApplyLocalRewrite(section.Content, req.Instruction)
		}
	} else if strings.TrimSpace(req.Instruction) != "" {
		fallbackUsed = true
		section.Content = generation.ApplyLocalRewrite(section.Content, req.Instruction)
	}

	listing.Sections[idx] = section
	addHistoryEntry(&listing, section, "rewrite")
	listing.FullCopy = composeFullCopy(listing.Sections)
	deriveStatus(&listing)
	updated, err := h.Store.UpdateListingSections(r.Context(), id, listing.Sections, listing.FullCopy, listing.History, listing.Status)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	if fallbackUsed {
		w.Header().Set("X-Generator-Fallback", "1")
	}
	hydrateDetailsFromLegacy(&updated)
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(updated)
	h.publishListing(updated)
}

// UpdateSection saves manual edits for a section.
func (h Handler) UpdateSection(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	slug := chi.URLParam(r, "slug")
	if id == "" || slug == "" {
		http.Error(w, "id and slug are required", http.StatusBadRequest)
		return
	}

	var req struct {
		Title   string `json:"title"`
		Content string `json:"content"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}
	req.Title = strings.TrimSpace(req.Title)
	req.Content = strings.TrimSpace(req.Content)
	if req.Content == "" {
		http.Error(w, "content cannot be empty", http.StatusBadRequest)
		return
	}

	listing, err := h.Store.GetListing(r.Context(), id)
	if err != nil {
		if errors.Is(err, storage.ErrNotFound) {
			http.NotFound(w, r)
			return
		}
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	idx := findSectionIndex(listing.Sections, slug)
	if idx == -1 {
		newSection := storage.Section{
			Slug:    slug,
			Title:   req.Title,
			Content: req.Content,
		}
		listing.Sections = append(listing.Sections, newSection)
		addHistoryEntry(&listing, newSection, "manual")
	} else {
		if req.Title != "" {
			listing.Sections[idx].Title = req.Title
		}
		listing.Sections[idx].Content = req.Content
		addHistoryEntry(&listing, listing.Sections[idx], "manual")
	}

	listing.FullCopy = composeFullCopy(listing.Sections)
	deriveStatus(&listing)
	updated, err := h.Store.UpdateListingSections(r.Context(), id, listing.Sections, listing.FullCopy, listing.History, listing.Status)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	hydrateDetailsFromLegacy(&updated)
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(updated)
	h.publishListing(updated)
}

// DeleteSection removes a section by slug.
func (h Handler) DeleteSection(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	slug := chi.URLParam(r, "slug")
	if id == "" || slug == "" {
		http.Error(w, "id and slug are required", http.StatusBadRequest)
		return
	}

	listing, err := h.Store.GetListing(r.Context(), id)
	if err != nil {
		if errors.Is(err, storage.ErrNotFound) {
			http.NotFound(w, r)
			return
		}
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	idx := findSectionIndex(listing.Sections, slug)
	if idx == -1 {
		http.Error(w, "section not found", http.StatusNotFound)
		return
	}

	listing.History[slug] = append([]storage.SectionVersion{{
		Title:     listing.Sections[idx].Title,
		Content:   listing.Sections[idx].Content,
		Source:    "delete",
		Timestamp: time.Now(),
	}}, listing.History[slug]...)

	listing.Sections = append(listing.Sections[:idx], listing.Sections[idx+1:]...)
	listing.FullCopy = composeFullCopy(listing.Sections)
	deriveStatus(&listing)

	updated, err := h.Store.UpdateListingSections(r.Context(), id, listing.Sections, listing.FullCopy, listing.History, listing.Status)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(updated)
	h.publishListing(updated)
}

// ExportFullCopy returns the listing text in different formats (text/html).
func (h Handler) ExportFullCopy(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if id == "" {
		http.Error(w, "id is required", http.StatusBadRequest)
		return
	}

	listing, err := h.Store.GetListing(r.Context(), id)
	if err != nil {
		if errors.Is(err, storage.ErrNotFound) {
			http.NotFound(w, r)
			return
		}
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	fullCopy := listing.FullCopy
	if fullCopy == "" && len(listing.Sections) > 0 {
		fullCopy = composeFullCopy(listing.Sections)
	}

	format := r.URL.Query().Get("format")
	switch strings.ToLower(format) {
	case "html":
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		for _, block := range strings.Split(fullCopy, "\n\n") {
			block = strings.TrimSpace(block)
			if block == "" {
				continue
			}
			_, _ = fmt.Fprintf(w, "<p>%s</p>\n", strings.ReplaceAll(block, "\n", " "))
		}
	default:
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		_, _ = w.Write([]byte(fullCopy))
	}
}

// DeleteListing removes an entire listing.
func (h Handler) DeleteListing(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if id == "" {
		http.Error(w, "id is required", http.StatusBadRequest)
		return
	}

	if err := h.Store.DeleteListing(r.Context(), id); err != nil {
		if errors.Is(err, storage.ErrNotFound) {
			http.NotFound(w, r)
			return
		}
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	h.publishDeletion(id)
	w.WriteHeader(http.StatusNoContent)
}

// StreamEvents streams status updates over SSE.
func (h Handler) StreamEvents(w http.ResponseWriter, r *http.Request) {
	if h.Events == nil {
		http.Error(w, "event streaming disabled", http.StatusNotImplemented)
		return
	}

	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "streaming unsupported", http.StatusInternalServerError)
		return
	}

	ch := h.Events.Subscribe()
	defer h.Events.Unsubscribe(ch)

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")

	for {
		select {
		case <-r.Context().Done():
			return
		case evt, ok := <-ch:
			if !ok {
				return
			}
			payload, err := json.Marshal(evt)
			if err != nil {
				continue
			}
			_, _ = fmt.Fprintf(w, "event: status\ndata: %s\n\n", payload)
			flusher.Flush()
		}
	}
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

	if sectionsRaw := strings.TrimSpace(r.FormValue("sections")); sectionsRaw != "" {
		var sections []SectionInput
		if err := json.Unmarshal([]byte(sectionsRaw), &sections); err != nil {
			return req, nil, fmt.Errorf("invalid sections payload: %w", err)
		}
		req.Sections = sections
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

func buildSectionsFromInput(req CreateListingRequest, imageURL string) []storage.Section {
	if len(req.Sections) == 0 {
		return buildDefaultSections(req, imageURL)
	}

	sections := make([]storage.Section, 0, len(req.Sections))
	seen := map[string]bool{}

	for _, s := range req.Sections {
		slug := strings.TrimSpace(s.Slug)
		if slug == "" {
			continue
		}
		if seen[slug] {
			continue
		}
		seen[slug] = true

		title := strings.TrimSpace(s.Title)
		if title == "" {
			title = strings.Title(slug)
		}
		content := strings.TrimSpace(s.Content)
		if content == "" && slug == "intro" {
			content = buildIntro(req, imageURL)
		} else if content == "" {
			content = "Text genereras vid behov."
		}

		sections = append(sections, storage.Section{
			Slug:    slug,
			Title:   title,
			Content: content,
		})
	}

	if len(sections) == 0 {
		return buildDefaultSections(req, imageURL)
	}

	return sections
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

func findSectionIndex(sections []storage.Section, slug string) int {
	for i, section := range sections {
		if section.Slug == slug {
			return i
		}
	}
	return -1
}

func composeFullCopy(sections []storage.Section) string {
	parts := make([]string, 0, len(sections))
	for _, section := range sections {
		content := strings.TrimSpace(section.Content)
		if content == "" {
			continue
		}
		title := strings.TrimSpace(section.Title)
		if title != "" {
			parts = append(parts, fmt.Sprintf("%s\n%s", title, content))
		} else {
			parts = append(parts, content)
		}
	}
	return strings.Join(parts, "\n\n")
}

func recordHistoryForAll(listing *storage.Listing, source string) {
	for _, section := range listing.Sections {
		addHistoryEntry(listing, section, source)
	}
}

func addHistoryEntry(listing *storage.Listing, section storage.Section, source string) {
	if listing.History == nil {
		listing.History = storage.History{}
	}
	if section.Slug == "" {
		return
	}
	entry := storage.SectionVersion{
		Title:     section.Title,
		Content:   section.Content,
		Source:    source,
		Timestamp: time.Now(),
	}
	entries := listing.History[section.Slug]
	entries = append([]storage.SectionVersion{entry}, entries...)
	if len(entries) > 5 {
		entries = entries[:5]
	}
	listing.History[section.Slug] = entries
}

func deriveStatus(listing *storage.Listing) {
	status := listing.Status
	if status.Data == "" {
		status.Data = "completed"
	}

	if listing.ImageURL == "" {
		status.Vision = "skipped"
	} else if status.Vision == "" {
		status.Vision = "pending"
	}

	if len(listing.Insights.Geodata.PointsOfInterest) > 0 {
		status.Geodata = "completed"
	} else if status.Geodata == "" {
		status.Geodata = "pending"
	}

	fullCopy := strings.TrimSpace(listing.FullCopy)
	if fullCopy != "" {
		status.Text = "completed"
	} else if status.Text == "" {
		status.Text = "pending"
	}

	listing.Status = status
}

func (h Handler) runPipeline(initial storage.Listing) {
	if h.Store == nil {
		return
	}

	ctx := context.Background()
	status := initial.Status

	if initial.ImageURL != "" && status.Vision != "completed" {
		status.Vision = "in_progress"
		_ = h.Store.UpdateStatus(ctx, initial.ID, status)
		h.publishStatus(initial.ID, status)
		time.Sleep(1500 * time.Millisecond)
		status.Vision = "completed"
	}

	if status.Geodata != "completed" {
		status.Geodata = "in_progress"
		_ = h.Store.UpdateStatus(ctx, initial.ID, status)
		h.publishStatus(initial.ID, status)
		time.Sleep(1200 * time.Millisecond)
		status.Geodata = "completed"
	}

	if status.Text != "completed" {
		status.Text = "in_progress"
		_ = h.Store.UpdateStatus(ctx, initial.ID, status)
		h.publishStatus(initial.ID, status)
		time.Sleep(800 * time.Millisecond)
		status.Text = "completed"
	}

	status.Data = "completed"
	_ = h.Store.UpdateStatus(ctx, initial.ID, status)
	h.publishStatus(initial.ID, status)
}

func (h Handler) publishListing(listing storage.Listing) {
	if h.Events == nil {
		return
	}
	h.Events.Publish(events.Event{
		ListingID: listing.ID,
		Status:    listing.Status,
	})
}

func (h Handler) publishStatus(listingID string, status storage.Status) {
	if h.Events == nil {
		return
	}
	h.Events.Publish(events.Event{
		ListingID: listingID,
		Status:    status,
	})
}

func (h Handler) publishDeletion(id string) {
	if h.Events == nil {
		return
	}
	h.Events.Publish(events.Event{
		ListingID: id,
		Status:    storage.Status{Data: "deleted"},
	})
}
