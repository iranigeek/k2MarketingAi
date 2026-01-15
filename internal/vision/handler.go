package vision

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
)

// Handler exposes HTTP endpoints for ad-hoc vision tooling.
type Handler struct {
	Analyzer Analyzer
	Designer Designer
	Renderer ImageGenerator
	Imagen   ImagenClient
}

// Analyze handles POST /api/vision/analyze.
func (h Handler) Analyze(w http.ResponseWriter, r *http.Request) {
	if h.Analyzer == nil {
		http.Error(w, "vision analysis inactive", http.StatusServiceUnavailable)
		return
	}

	ct := r.Header.Get("Content-Type")
	if strings.HasPrefix(ct, "multipart/form-data") {
		h.handleMultipartAnalyze(w, r)
		return
	}

	var req struct {
		ImageURL string `json:"image_url"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}
	imageURL := strings.TrimSpace(req.ImageURL)
	if imageURL == "" {
		http.Error(w, "image_url is required", http.StatusBadRequest)
		return
	}

	result, err := h.Analyzer.Analyze(r.Context(), imageURL)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadGateway)
		return
	}
	writeJSON(w, result)
}

func (h Handler) handleMultipartAnalyze(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseMultipartForm(MaxVisionImageBytes + (1 << 20)); err != nil {
		http.Error(w, fmt.Sprintf("could not parse form: %v", err), http.StatusBadRequest)
		return
	}

	file, header, err := r.FormFile("image_file")
	if err != nil {
		http.Error(w, "image_file is required", http.StatusBadRequest)
		return
	}
	defer file.Close()

	data, err := io.ReadAll(io.LimitReader(file, MaxVisionImageBytes+1))
	if err != nil {
		http.Error(w, "could not read file", http.StatusBadRequest)
		return
	}
	if len(data) == 0 {
		http.Error(w, "empty file", http.StatusBadRequest)
		return
	}
	if len(data) > MaxVisionImageBytes {
		http.Error(w, fmt.Sprintf("file exceeds %d bytes", MaxVisionImageBytes), http.StatusBadRequest)
		return
	}

	mime := header.Header.Get("Content-Type")
	result, err := h.Analyzer.AnalyzeBytes(r.Context(), data, mime)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadGateway)
		return
	}
	writeJSON(w, result)
}

// Design handles POST /api/vision/design.
func (h Handler) Design(w http.ResponseWriter, r *http.Request) {
	if h.Designer == nil {
		http.Error(w, "vision designer inactive", http.StatusServiceUnavailable)
		return
	}
	var req struct {
		Prompt string `json:"prompt"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}

	concept, err := h.Designer.Design(r.Context(), req.Prompt)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadGateway)
		return
	}
	writeJSON(w, concept)
}

// Render handles POST /api/vision/render by generating an illustrative image.
func (h Handler) Render(w http.ResponseWriter, r *http.Request) {
	if h.Renderer == nil && h.Imagen == nil {
		http.Error(w, "vision rendering inactive", http.StatusServiceUnavailable)
		return
	}
	var req struct {
		Prompt        string `json:"prompt"`
		BaseImageURL  string `json:"base_image_url"`
		BaseImageData string `json:"base_image_data"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}
	if strings.TrimSpace(req.Prompt) == "" {
		http.Error(w, "prompt is required", http.StatusBadRequest)
		return
	}
	if h.Imagen != nil && (strings.TrimSpace(req.BaseImageURL) != "" || strings.TrimSpace(req.BaseImageData) != "") {
		result, err := h.Imagen.Edit(r.Context(), ImagenPayload{
			Prompt:        req.Prompt,
			BaseImageURL:  strings.TrimSpace(req.BaseImageURL),
			BaseImageData: strings.TrimSpace(req.BaseImageData),
		})
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadGateway)
			return
		}
		writeJSON(w, result)
		return
	}
	result, err := h.Renderer.Generate(r.Context(), req.Prompt)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadGateway)
		return
	}
	writeJSON(w, result)
}

func writeJSON(w http.ResponseWriter, payload any) {
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(payload); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}
