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
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"

	"k2MarketingAi/internal/auth"
	"k2MarketingAi/internal/events"
	"k2MarketingAi/internal/generation"
	"k2MarketingAi/internal/geodata"
	"k2MarketingAi/internal/llm"
	"k2MarketingAi/internal/media"
	"k2MarketingAi/internal/storage"
	"k2MarketingAi/internal/vision"
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
	Vision      vision.Analyzer
	Events      *events.Broker
}

// CreateListingRequest describes inbound payload for creating a listing.
type CreateListingRequest struct {
	Address        string               `json:"address"`
	Neighborhood   string               `json:"neighborhood"`
	City           string               `json:"city"`
	PropertyType   string               `json:"property_type"`
	Condition      string               `json:"condition"`
	Balcony        bool                 `json:"balcony"`
	Floor          string               `json:"floor"`
	Association    string               `json:"association"`
	Length         string               `json:"length"`
	Tone           string               `json:"tone"`
	TargetAudience string               `json:"target_audience"`
	Highlights     []string             `json:"highlights"`
	ImageURL       string               `json:"image_url,omitempty"`
	Fee            int                  `json:"fee"`
	LivingArea     float64              `json:"living_area"`
	Rooms          float64              `json:"rooms"`
	Instructions   string               `json:"instructions"`
	Sections       []SectionInput       `json:"sections"`
	Images         []storage.ImageAsset `json:"images"`
	StyleProfileID string               `json:"style_profile_id"`
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

func currentUser(w http.ResponseWriter, r *http.Request) (storage.User, bool) {
	user, ok := auth.UserFromContext(r.Context())
	if !ok {
		http.Error(w, "logga in f\u00f6rst", http.StatusUnauthorized)
		return storage.User{}, false
	}
	return user, true
}

func (h Handler) fetchListingForUser(ctx context.Context, id string, userID string) (storage.Listing, error) {
	listing, err := h.Store.GetListing(ctx, id)
	if err != nil {
		return storage.Listing{}, err
	}
	if listing.OwnerID == "" || listing.OwnerID != userID {
		return storage.Listing{}, storage.ErrNotFound
	}
	return listing, nil
}

func logUploadEvent(format string, args ...interface{}) {
	f, err := os.OpenFile("upload_debug.log", os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		log.Printf("upload debug log open failed: %v", err)
		return
	}
	defer f.Close()
	fmt.Fprintf(f, "%s: %s\n", time.Now().Format(time.RFC3339), fmt.Sprintf(format, args...))
}

// Create handles POST /api/listings.
func (h Handler) Create(w http.ResponseWriter, r *http.Request) {
	var (
		req    CreateListingRequest
		upload *uploadPayload
		err    error
	)
	user, ok := currentUser(w, r)
	if !ok {
		return
	}

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
		if upload.filename == "" {
			upload.filename = "photo.jpg"
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
		newAsset := storage.ImageAsset{
			URL:    result.URL,
			Key:    result.Key,
			Label:  upload.filename,
			Source: "user",
			Kind:   "photo",
			Cover:  true,
		}
		req.Images = append([]storage.ImageAsset{newAsset}, req.Images...)
	}

	listing := storage.Listing{
		OwnerID:        user.ID,
		Address:        req.Address,
		Neighborhood:   req.Neighborhood,
		City:           req.City,
		PropertyType:   req.PropertyType,
		Condition:      req.Condition,
		Balcony:        req.Balcony,
		Floor:          req.Floor,
		Association:    req.Association,
		Length:         req.Length,
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
	listing.Details.Meta.StyleProfileID = strings.TrimSpace(req.StyleProfileID)
	applyImagesToListing(&listing, req.Images)
	hydrateDetailsFromLegacy(&listing)
	h.attachStyleProfiles(r.Context(), []*storage.Listing{&listing})

	if h.GeoProvider != nil {
		searchAddress := combineAddressCity(req.Address, req.City)
		if summary, err := h.GeoProvider.Fetch(r.Context(), searchAddress); err == nil {
			listing.Insights.Geodata = geodata.ToStorageInsights(summary)
		} else {
			log.Printf("geodata fetch failed: %v", err)
		}
	}

	if h.Generator != nil {
		genCtx := r.Context()
		if listing.StyleProfile != nil && listing.StyleProfile.CustomModel != "" {
			genCtx = llm.WithModel(genCtx, listing.StyleProfile.CustomModel)
		}
		result, genErr := h.Generator.Generate(genCtx, listing)
		if genErr != nil {
			log.Printf("generator failed: %v", genErr)
			http.Error(w, fmt.Sprintf("text generation failed: %v", genErr), http.StatusBadGateway)
			return
		}
		listing.Sections = result.Sections
		if strings.TrimSpace(result.FullCopy) != "" {
			listing.FullCopy = result.FullCopy
		}
	}
	recordHistoryForAll(&listing, "generate", historyContext{
		Tone:           listing.Tone,
		TargetAudience: listing.TargetAudience,
		Highlights:     listing.Highlights,
	})
	if listing.FullCopy == "" {
		listing.FullCopy = composeFullCopy(listing.Sections)
	}
	deriveStatus(&listing)

	listing, err = h.Store.CreateListing(r.Context(), listing)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	h.attachStyleProfiles(r.Context(), []*storage.Listing{&listing})
	h.publishListing(listing)
	go h.runPipeline(listing)

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	_ = json.NewEncoder(w).Encode(listing)
}

// List handles GET /api/listings.
func (h Handler) List(w http.ResponseWriter, r *http.Request) {
	user, ok := currentUser(w, r)
	if !ok {
		return
	}
	listings, err := h.Store.ListListingsByOwner(r.Context(), user.ID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	pointers := make([]*storage.Listing, len(listings))
	for i := range listings {
		hydrateDetailsFromLegacy(&listings[i])
		pointers[i] = &listings[i]
	}
	h.attachStyleProfiles(r.Context(), pointers)

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(listings)
}

// Get returns a single listing by id.
func (h Handler) Get(w http.ResponseWriter, r *http.Request) {
	user, ok := currentUser(w, r)
	if !ok {
		return
	}
	id := chi.URLParam(r, "id")
	if id == "" {
		http.Error(w, "id is required", http.StatusBadRequest)
		return
	}

	listing, err := h.fetchListingForUser(r.Context(), id, user.ID)
	if err != nil {
		if errors.Is(err, storage.ErrNotFound) {
			http.NotFound(w, r)
			return
		}
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	hydrateDetailsFromLegacy(&listing)
	h.attachStyleProfiles(r.Context(), []*storage.Listing{&listing})
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(listing)
}

// RewriteSection accepts instructions and rewrites a section using the generator.
func (h Handler) RewriteSection(w http.ResponseWriter, r *http.Request) {
	user, ok := currentUser(w, r)
	if !ok {
		return
	}
	id := chi.URLParam(r, "id")
	slug := normalizeSlug(chi.URLParam(r, "slug"))
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

	listing, err := h.fetchListingForUser(r.Context(), id, user.ID)
	if err != nil {
		if errors.Is(err, storage.ErrNotFound) {
			http.NotFound(w, r)
			return
		}
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	h.attachStyleProfiles(r.Context(), []*storage.Listing{&listing})
	idx := findSectionIndex(listing.Sections, slug)
	if idx == -1 && slug == "main" && len(listing.Sections) > 0 {
		idx = 0
		slug = normalizeSlug(listing.Sections[idx].Slug)
	}
	if idx == -1 {
		http.Error(w, "section not found", http.StatusNotFound)
		return
	}

	section := listing.Sections[idx]
	fallbackUsed := false
	if h.Generator != nil {
		genCtx := r.Context()
		if listing.StyleProfile != nil && listing.StyleProfile.CustomModel != "" {
			genCtx = llm.WithModel(genCtx, listing.StyleProfile.CustomModel)
		}
		updated, genErr := h.Generator.Rewrite(genCtx, listing, section, req.Instruction)
		if genErr != nil {
			log.Printf("rewrite failed: %v", genErr)
			http.Error(w, fmt.Sprintf("text rewrite failed: %v", genErr), http.StatusBadGateway)
			return
		}
		section = updated
	} else if strings.TrimSpace(req.Instruction) != "" {
		fallbackUsed = true
		section.Content = generation.ApplyLocalRewrite(section.Content, req.Instruction)
	}

	listing.Sections[idx] = section
	rewriteCtx := historyContext{
		Instruction:    strings.TrimSpace(req.Instruction),
		Tone:           listing.Tone,
		TargetAudience: listing.TargetAudience,
		Highlights:     listing.Highlights,
	}
	if fallbackUsed {
		rewriteCtx.Notes = "lokal fallback rewriter"
	}
	addHistoryEntry(&listing, section, "rewrite", rewriteCtx)
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
	h.attachStyleProfiles(r.Context(), []*storage.Listing{&updated})
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(updated)
	h.publishListing(updated)
}

// UpdateSection saves manual edits for a section.
func (h Handler) UpdateSection(w http.ResponseWriter, r *http.Request) {
	user, ok := currentUser(w, r)
	if !ok {
		return
	}
	id := chi.URLParam(r, "id")
	slug := normalizeSlug(chi.URLParam(r, "slug"))
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

	listing, err := h.fetchListingForUser(r.Context(), id, user.ID)
	if err != nil {
		if errors.Is(err, storage.ErrNotFound) {
			http.NotFound(w, r)
			return
		}
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	idx := findSectionIndex(listing.Sections, slug)
	if idx == -1 && slug == "main" && len(listing.Sections) > 0 {
		idx = 0
	}
	if idx == -1 {
		newSection := storage.Section{
			Slug:    slug,
			Title:   req.Title,
			Content: req.Content,
		}
		listing.Sections = append(listing.Sections, newSection)
		addHistoryEntry(&listing, newSection, "manual", historyContext{
			Tone:           listing.Tone,
			TargetAudience: listing.TargetAudience,
			Highlights:     listing.Highlights,
		})
	} else {
		if req.Title != "" {
			listing.Sections[idx].Title = req.Title
		}
		listing.Sections[idx].Content = req.Content
		addHistoryEntry(&listing, listing.Sections[idx], "manual", historyContext{
			Tone:           listing.Tone,
			TargetAudience: listing.TargetAudience,
			Highlights:     listing.Highlights,
			Notes:          "manuell uppdatering",
		})
	}

	listing.FullCopy = composeFullCopy(listing.Sections)
	deriveStatus(&listing)
	updated, err := h.Store.UpdateListingSections(r.Context(), id, listing.Sections, listing.FullCopy, listing.History, listing.Status)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	hydrateDetailsFromLegacy(&updated)
	h.attachStyleProfiles(r.Context(), []*storage.Listing{&updated})
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(updated)
	h.publishListing(updated)
}

// DeleteSection removes a section by slug.
func (h Handler) DeleteSection(w http.ResponseWriter, r *http.Request) {
	user, ok := currentUser(w, r)
	if !ok {
		return
	}
	id := chi.URLParam(r, "id")
	slug := normalizeSlug(chi.URLParam(r, "slug"))
	if id == "" || slug == "" {
		http.Error(w, "id and slug are required", http.StatusBadRequest)
		return
	}

	listing, err := h.fetchListingForUser(r.Context(), id, user.ID)
	if err != nil {
		if errors.Is(err, storage.ErrNotFound) {
			http.NotFound(w, r)
			return
		}
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	idx := findSectionIndex(listing.Sections, slug)
	if idx == -1 && slug == "main" && len(listing.Sections) > 0 {
		idx = 0
	}
	if idx == -1 {
		http.Error(w, "section not found", http.StatusNotFound)
		return
	}

	addHistoryEntry(&listing, listing.Sections[idx], "delete", historyContext{
		Tone:           listing.Tone,
		TargetAudience: listing.TargetAudience,
		Highlights:     listing.Highlights,
		Notes:          "sektionen togs bort",
	})

	listing.Sections = append(listing.Sections[:idx], listing.Sections[idx+1:]...)
	listing.FullCopy = composeFullCopy(listing.Sections)
	deriveStatus(&listing)

	updated, err := h.Store.UpdateListingSections(r.Context(), id, listing.Sections, listing.FullCopy, listing.History, listing.Status)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	hydrateDetailsFromLegacy(&updated)
	h.attachStyleProfiles(r.Context(), []*storage.Listing{&updated})
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(updated)
	h.publishListing(updated)
}

// ExportFullCopy returns the listing text in different formats (text/html).
func (h Handler) ExportFullCopy(w http.ResponseWriter, r *http.Request) {
	user, ok := currentUser(w, r)
	if !ok {
		return
	}
	id := chi.URLParam(r, "id")
	if id == "" {
		http.Error(w, "id is required", http.StatusBadRequest)
		return
	}

	listing, err := h.fetchListingForUser(r.Context(), id, user.ID)
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
	user, ok := currentUser(w, r)
	if !ok {
		return
	}
	id := chi.URLParam(r, "id")
	if id == "" {
		http.Error(w, "id is required", http.StatusBadRequest)
		return
	}

	listing, err := h.fetchListingForUser(r.Context(), id, user.ID)
	if err != nil {
		if errors.Is(err, storage.ErrNotFound) {
			http.NotFound(w, r)
			return
		}
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	if err := h.Store.DeleteListing(r.Context(), listing.ID); err != nil {
		if errors.Is(err, storage.ErrNotFound) {
			http.NotFound(w, r)
			return
		}
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	h.publishDeletion(listing.ID, listing.OwnerID)
	w.WriteHeader(http.StatusNoContent)
}

// UploadMedia handles raw file uploads to the configured uploader (S3).
func (h Handler) UploadMedia(w http.ResponseWriter, r *http.Request) {
	if h.Uploader == nil {
		http.Error(w, "uploads disabled", http.StatusNotImplemented)
		return
	}
	logUploadEvent("start UploadMedia")
	if err := r.ParseMultipartForm(maxImageBytes + (1 << 20)); err != nil {
		http.Error(w, fmt.Sprintf("invalid upload payload: %v", err), http.StatusBadRequest)
		return
	}
	file, header, err := r.FormFile("file")
	if err != nil {
		http.Error(w, "file is required", http.StatusBadRequest)
		return
	}
	defer file.Close()

	data, err := io.ReadAll(io.LimitReader(file, maxImageBytes+1))
	if err != nil {
		http.Error(w, "could not read file", http.StatusBadRequest)
		return
	}
	if len(data) == 0 {
		http.Error(w, "file was empty", http.StatusBadRequest)
		return
	}
	if len(data) > maxImageBytes {
		http.Error(w, fmt.Sprintf("file exceeds %d MB", maxImageBytes/(1024*1024)), http.StatusBadRequest)
		return
	}

	contentType := header.Header.Get("Content-Type")
	if contentType == "" {
		contentType = http.DetectContentType(data)
	}

	uploadCtx, cancel := context.WithTimeout(r.Context(), 30*time.Second)
	defer cancel()

	result, err := h.Uploader.Upload(uploadCtx, media.UploadInput{
		Filename:    header.Filename,
		ContentType: contentType,
		Body:        bytes.NewReader(data),
		Size:        int64(len(data)),
	})
	if err != nil {
		status := http.StatusInternalServerError
		if errors.Is(err, media.ErrUploaderDisabled) {
			status = http.StatusBadRequest
		} else if errors.Is(err, context.DeadlineExceeded) {
			status = http.StatusGatewayTimeout
		} else {
			log.Printf("upload failed: %v", err)
		}
		logUploadEvent("upload failed: %v", err)
		http.Error(w, "could not upload file", status)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	logUploadEvent("upload ok: key=%s size=%d", result.Key, len(data))
	_ = json.NewEncoder(w).Encode(result)
}

// AttachImage wires an uploaded image to an existing listing.
func (h Handler) AttachImage(w http.ResponseWriter, r *http.Request) {
	user, ok := currentUser(w, r)
	if !ok {
		return
	}
	id := chi.URLParam(r, "id")
	if id == "" {
		http.Error(w, "id is required", http.StatusBadRequest)
		return
	}

	var req struct {
		URL    string `json:"url"`
		Key    string `json:"key"`
		Label  string `json:"label"`
		Source string `json:"source"`
		Kind   string `json:"kind"`
		Cover  bool   `json:"cover"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}
	if strings.TrimSpace(req.URL) == "" {
		http.Error(w, "url is required", http.StatusBadRequest)
		return
	}

	listing, err := h.fetchListingForUser(r.Context(), id, user.ID)
	if err != nil {
		if errors.Is(err, storage.ErrNotFound) {
			http.NotFound(w, r)
			return
		}
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	asset := storage.ImageAsset{
		URL:       strings.TrimSpace(req.URL),
		Key:       strings.TrimSpace(req.Key),
		Label:     strings.TrimSpace(req.Label),
		Source:    strings.TrimSpace(req.Source),
		Kind:      strings.TrimSpace(req.Kind),
		Cover:     req.Cover,
		CreatedAt: time.Now(),
	}
	if asset.Source == "" {
		asset.Source = "user"
	}
	if asset.Kind == "" {
		asset.Kind = "photo"
	}

	listing.Details.Media.Images = append(listing.Details.Media.Images, asset)
	if asset.Cover || listing.ImageURL == "" {
		listing.ImageURL = asset.URL
		for i := range listing.Details.Media.Images {
			listing.Details.Media.Images[i].Cover = listing.Details.Media.Images[i].URL == asset.URL
		}
	}

	updated, err := h.Store.UpdateListingDetails(r.Context(), id, listing.Details, listing.ImageURL)
	if err != nil {
		if errors.Is(err, storage.ErrNotFound) {
			http.NotFound(w, r)
			return
		}
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	hydrateDetailsFromLegacy(&updated)
	h.attachStyleProfiles(r.Context(), []*storage.Listing{&updated})
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(updated)
	h.publishListing(updated)
}

// StreamEvents streams status updates over SSE.
func (h Handler) StreamEvents(w http.ResponseWriter, r *http.Request) {
	user, ok := currentUser(w, r)
	if !ok {
		return
	}
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
			if evt.OwnerID != "" && evt.OwnerID != user.ID {
				continue
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

// ListStyleProfiles returns all stored style profiles.
func (h Handler) ListStyleProfiles(w http.ResponseWriter, r *http.Request) {
	profiles, err := h.Store.ListStyleProfiles(r.Context())
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(profiles)
}

// SaveStyleProfile creates or updates a style profile.
func (h Handler) SaveStyleProfile(w http.ResponseWriter, r *http.Request) {
	var payload storage.StyleProfile
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}

	payload.Name = strings.TrimSpace(payload.Name)
	payload.Description = strings.TrimSpace(payload.Description)
	payload.Tone = strings.TrimSpace(payload.Tone)
	payload.Guidelines = strings.TrimSpace(payload.Guidelines)
	payload.ExampleTexts = normalizeList(payload.ExampleTexts)
	payload.ForbiddenWords = normalizeList(payload.ForbiddenWords)

	if payload.Name == "" {
		http.Error(w, "name is required", http.StatusBadRequest)
		return
	}
	if len(payload.ExampleTexts) == 0 {
		http.Error(w, "at least one example_texts entry is required", http.StatusBadRequest)
		return
	}

	profile, err := h.Store.SaveStyleProfile(r.Context(), payload)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(profile)
}

func normalizeList(values []string) []string {
	var result []string
	for _, v := range values {
		if trimmed := strings.TrimSpace(v); trimmed != "" {
			result = append(result, trimmed)
		}
	}
	return result
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
		StyleProfileID: strings.TrimSpace(r.FormValue("style_profile_id")),
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
	req.Neighborhood = strings.TrimSpace(req.Neighborhood)
	req.City = strings.TrimSpace(req.City)
	req.PropertyType = strings.TrimSpace(req.PropertyType)
	req.Condition = strings.TrimSpace(req.Condition)
	req.Floor = strings.TrimSpace(req.Floor)
	req.Association = strings.TrimSpace(req.Association)
	req.Length = strings.TrimSpace(req.Length)
	req.Tone = strings.TrimSpace(req.Tone)
	req.TargetAudience = strings.TrimSpace(req.TargetAudience)
	req.ImageURL = strings.TrimSpace(req.ImageURL)
	req.Instructions = strings.TrimSpace(req.Instructions)
}

func applyImagesToListing(listing *storage.Listing, assets []storage.ImageAsset) {
	normalized := normalizeAssets(assets)
	if len(normalized) == 0 {
		return
	}
	listing.Details.Media.Images = append(listing.Details.Media.Images, normalized...)
	ensureCoverImage(listing)
}

func normalizeAssets(assets []storage.ImageAsset) []storage.ImageAsset {
	if len(assets) == 0 {
		return nil
	}
	now := time.Now()
	normalized := make([]storage.ImageAsset, 0, len(assets))
	for _, asset := range assets {
		asset.URL = strings.TrimSpace(asset.URL)
		if asset.URL == "" {
			continue
		}
		if asset.Source == "" {
			asset.Source = "user"
		}
		if asset.Kind == "" {
			asset.Kind = "photo"
		}
		if asset.CreatedAt.IsZero() {
			asset.CreatedAt = now
		}
		normalized = append(normalized, asset)
	}
	return normalized
}

func ensureCoverImage(listing *storage.Listing) {
	images := listing.Details.Media.Images
	if len(images) == 0 {
		return
	}
	if listing.ImageURL != "" {
		for i := range images {
			images[i].Cover = images[i].URL == listing.ImageURL
		}
		return
	}

	for i := range images {
		if images[i].Cover && images[i].URL != "" {
			listing.ImageURL = images[i].URL
			return
		}
	}

	images[0].Cover = true
	listing.ImageURL = images[0].URL
	listing.Details.Media.Images = images
}

func buildDefaultSections(req CreateListingRequest, imageURL string) []storage.Section {
	return []storage.Section{
		{Slug: "main", Title: "Annons", Content: buildMinimalAd(req, imageURL)},
	}
}

func buildSectionsFromInput(req CreateListingRequest, imageURL string) []storage.Section {
	if len(req.Sections) == 0 {
		return buildDefaultSections(req, imageURL)
	}

	sections := make([]storage.Section, 0, len(req.Sections))
	seen := map[string]bool{}

	for _, s := range req.Sections {
		slug := normalizeSlug(s.Slug)
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
		if content == "" {
			content = buildMinimalAd(req, imageURL)
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

func buildMinimalAd(req CreateListingRequest, imageURL string) string {
	var b strings.Builder
	location := strings.TrimSpace(strings.Join([]string{req.Neighborhood, req.City}, ", "))
	tone := req.Tone
	if tone == "" {
		tone = "Neutral"
	}

	fmt.Fprintf(&b, "Välkommen till %s", req.Address)
	if location != "" {
		fmt.Fprintf(&b, " i %s", location)
	}
	fmt.Fprintf(&b, " – en %s %s med %s.", strings.ToLower(tone), strings.ToLower(orDefault(req.PropertyType, "bostad")), describeRooms(req.Rooms, req.LivingArea))

	if req.Balcony {
		fmt.Fprint(&b, " Balkong eller uteplats finns för sköna stunder utomhus.")
	}
	if req.Condition != "" {
		fmt.Fprintf(&b, " Skicket upplevs som %s, vilket ger en trygg bas att flytta in i.", strings.ToLower(req.Condition))
	}
	if req.Floor != "" {
		fmt.Fprintf(&b, " Bostaden ligger på våning %s.", req.Floor)
	}
	if req.Association != "" {
		fmt.Fprintf(&b, " Föreningen heter %s.", req.Association)
	}
	if len(req.Highlights) > 0 {
		fmt.Fprintf(&b, " Fördelar: %s.", strings.Join(req.Highlights, ", "))
	}
	if imageURL != "" {
		fmt.Fprint(&b, " Bildunderlaget ger en bra känsla redan innan visningen.")
	}

	fmt.Fprintf(&b, "\n\nPlanlösningen nyttjar ytan väl med sociala och privata delar som flyter ihop naturligt. ")
	fmt.Fprint(&b, "Köket och vardagsrummet binder ihop hemmet för både vardag och umgänge, medan sovdelarna ger ro och balans. ")
	fmt.Fprint(&b, "Badrumsdelen beskrivs generellt och utan påhittade detaljer för att hålla fakta korrekt. ")

	if req.Association != "" {
		fmt.Fprint(&b, "Föreningen presenteras kort utan att lägga till fakta som inte finns i inmatningen. ")
	}
	fmt.Fprint(&b, "Området kan beskrivas generellt med närhet till service, natur eller kommunikationer utan att hitta på specifika namn.")

	fmt.Fprint(&b, "\n\nSammanfattningsvis får köparen en inbjudande bostad med balanserad ton, redo för nästa kapitel.")
	return strings.TrimSpace(b.String())
}

func combineAddressCity(address, city string) string {
	var parts []string
	if trimmed := strings.TrimSpace(address); trimmed != "" {
		parts = append(parts, trimmed)
	}
	if trimmed := strings.TrimSpace(city); trimmed != "" {
		parts = append(parts, trimmed)
	}
	return strings.Join(parts, ", ")
}

func orDefault(value, fallback string) string {
	if strings.TrimSpace(value) == "" {
		return fallback
	}
	return value
}

func describeRooms(rooms float64, area float64) string {
	switch {
	case rooms > 0 && area > 0:
		return fmt.Sprintf("%s fördelat över ca %.0f kvm", formatRooms(rooms), area)
	case rooms > 0:
		return fmt.Sprintf("%s med flexibel planlösning", formatRooms(rooms))
	case area > 0:
		return fmt.Sprintf("funktionella %.0f kvm", area)
	default:
		return "välavvägd planlösning"
	}
}

func findSectionIndex(sections []storage.Section, slug string) int {
	target := normalizeSlug(slug)
	for i, section := range sections {
		sectionSlug := normalizeSlug(section.Slug)
		if sectionSlug == target {
			return i
		}
		if sectionSlug == "" && target == normalizeSlug(section.Title) {
			return i
		}
	}
	return -1
}

func normalizeSlug(value string) string {
	slug := strings.ToLower(strings.TrimSpace(value))
	slug = strings.Trim(slug, "/")
	slug = strings.ReplaceAll(slug, "_", "-")
	slug = strings.Join(strings.FieldsFunc(slug, func(r rune) bool {
		return r == ' ' || r == '\t' || r == '\n'
	}), "-")
	slug = strings.Trim(slug, "-")
	return slug
}

func formatRooms(rooms float64) string {
	if rooms == 0 {
		return ""
	}
	if rooms == float64(int(rooms)) {
		return fmt.Sprintf("%d", int(rooms))
	}
	return fmt.Sprintf("%.1f", rooms)
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

func (h Handler) attachStyleProfiles(ctx context.Context, listings []*storage.Listing) {
	if h.Store == nil {
		return
	}
	cache := make(map[string]*storage.StyleProfile)
	for _, item := range listings {
		if item == nil {
			continue
		}
		styleID := strings.TrimSpace(item.Details.Meta.StyleProfileID)
		if styleID == "" {
			item.StyleProfile = nil
			continue
		}
		if cached, ok := cache[styleID]; ok {
			item.StyleProfile = cached
			continue
		}
		profile, err := h.Store.GetStyleProfile(ctx, styleID)
		if err != nil {
			if !errors.Is(err, storage.ErrNotFound) {
				log.Printf("style profile fetch failed: %v", err)
			}
			continue
		}
		cache[styleID] = &profile
		item.StyleProfile = &profile
	}
}

func recordHistoryForAll(listing *storage.Listing, source string, ctx historyContext) {
	for _, section := range listing.Sections {
		addHistoryEntry(listing, section, source, ctx)
	}
}

type historyContext struct {
	Instruction    string
	Tone           string
	TargetAudience string
	Highlights     []string
	Notes          string
}

func addHistoryEntry(listing *storage.Listing, section storage.Section, source string, ctx historyContext) {
	if listing.History == nil {
		listing.History = storage.History{}
	}
	if section.Slug == "" {
		return
	}
	entry := storage.SectionVersion{
		Title:          section.Title,
		Content:        section.Content,
		Source:         source,
		Instruction:    strings.TrimSpace(ctx.Instruction),
		Tone:           ctx.Tone,
		TargetAudience: ctx.TargetAudience,
		Highlights:     append([]string(nil), ctx.Highlights...),
		Notes:          strings.TrimSpace(ctx.Notes),
		Timestamp:      time.Now(),
	}
	if len(entry.Highlights) == 0 {
		entry.Highlights = nil
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
	} else if listing.Insights.Vision.Summary != "" && status.Vision == "" {
		status.Vision = "completed"
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
	listing := initial

	if initial.ImageURL == "" {
		if status.Vision == "" {
			status.Vision = "skipped"
			_ = h.Store.UpdateStatus(ctx, initial.ID, status)
			h.publishStatus(initial.ID, initial.OwnerID, status)
		}
	} else if status.Vision != "completed" {
		if h.Vision == nil {
			status.Vision = "skipped"
			_ = h.Store.UpdateStatus(ctx, initial.ID, status)
			h.publishStatus(initial.ID, initial.OwnerID, status)
		} else {
			status.Vision = "in_progress"
			_ = h.Store.UpdateStatus(ctx, initial.ID, status)
			h.publishStatus(initial.ID, initial.OwnerID, status)

			insights, err := h.Vision.Analyze(ctx, initial.ImageURL)
			if err != nil {
				log.Printf("vision analysis failed: %v", err)
				status.Vision = "failed"
				_ = h.Store.UpdateStatus(ctx, initial.ID, status)
				h.publishStatus(initial.ID, initial.OwnerID, status)
			} else {
				listing.Insights.Vision = insights
				status.Vision = "completed"
				updated, err := h.Store.UpdateInsights(ctx, initial.ID, listing.Insights, status)
				if err != nil {
					log.Printf("store vision insights failed: %v", err)
					_ = h.Store.UpdateStatus(ctx, initial.ID, status)
					h.publishStatus(initial.ID, initial.OwnerID, status)
				} else {
					listing = updated
					status = updated.Status
					h.publishListing(updated)
				}
			}
		}
	}

	if status.Geodata != "completed" {
		status.Geodata = "in_progress"
		_ = h.Store.UpdateStatus(ctx, initial.ID, status)
		h.publishStatus(initial.ID, initial.OwnerID, status)
		time.Sleep(1200 * time.Millisecond)
		status.Geodata = "completed"
		_ = h.Store.UpdateStatus(ctx, initial.ID, status)
		h.publishStatus(initial.ID, initial.OwnerID, status)
	}

	if status.Text != "completed" {
		status.Text = "in_progress"
		_ = h.Store.UpdateStatus(ctx, initial.ID, status)
		h.publishStatus(initial.ID, initial.OwnerID, status)
		time.Sleep(800 * time.Millisecond)
		status.Text = "completed"
		_ = h.Store.UpdateStatus(ctx, initial.ID, status)
		h.publishStatus(initial.ID, initial.OwnerID, status)
	}

	status.Data = "completed"
	_ = h.Store.UpdateStatus(ctx, initial.ID, status)
	h.publishStatus(initial.ID, initial.OwnerID, status)
}

func (h Handler) publishListing(listing storage.Listing) {
	if h.Events == nil {
		return
	}
	h.Events.Publish(events.Event{
		ListingID: listing.ID,
		OwnerID:   listing.OwnerID,
		Status:    listing.Status,
	})
}

func (h Handler) publishStatus(listingID, ownerID string, status storage.Status) {
	if h.Events == nil {
		return
	}
	h.Events.Publish(events.Event{
		ListingID: listingID,
		OwnerID:   ownerID,
		Status:    status,
	})
}

func (h Handler) publishDeletion(id, ownerID string) {
	if h.Events == nil {
		return
	}
	h.Events.Publish(events.Event{
		ListingID: id,
		OwnerID:   ownerID,
		Status:    storage.Status{Data: "deleted"},
	})
}
