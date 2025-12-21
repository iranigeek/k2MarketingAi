package vision

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"k2MarketingAi/internal/storage"
)

// Analyzer extracts structured insights from property images.
type Analyzer interface {
	Analyze(ctx context.Context, imageURL string) (storage.VisionInsights, error)
	AnalyzeBytes(ctx context.Context, data []byte, mimeType string) (storage.VisionInsights, error)
}

// GeminiAnalyzer implements Analyzer using Google's Generative Language API.
type GeminiAnalyzer struct {
	apiKey string
	model  string
	client *http.Client
}

const (
	MaxVisionImageBytes = 7 * 1024 * 1024
	defaultVisionModel  = "gemini-1.5-flash-001"
)

// NewGeminiAnalyzer constructs a Gemini-powered image analyzer.
func NewGeminiAnalyzer(apiKey, model string, timeout time.Duration) *GeminiAnalyzer {
	model = normalizeVisionModel(model)
	if timeout <= 0 {
		timeout = 60 * time.Second
	}
	return &GeminiAnalyzer{
		apiKey: apiKey,
		model:  model,
		client: &http.Client{Timeout: timeout},
	}
}

// Analyze downloads the image and asks Gemini to describe it in structured form.
func (g *GeminiAnalyzer) Analyze(ctx context.Context, imageURL string) (storage.VisionInsights, error) {
	if strings.TrimSpace(imageURL) == "" {
		return storage.VisionInsights{}, fmt.Errorf("vision: empty image URL")
	}
	imgData, mimeType, err := g.fetchImage(ctx, imageURL)
	if err != nil {
		return storage.VisionInsights{}, err
	}
	return g.analyzeData(ctx, imgData, mimeType)
}

// AnalyzeBytes runs analysis directly on uploaded image data.
func (g *GeminiAnalyzer) AnalyzeBytes(ctx context.Context, data []byte, mimeType string) (storage.VisionInsights, error) {
	if len(data) == 0 {
		return storage.VisionInsights{}, fmt.Errorf("vision: tom bilddata")
	}
	if len(data) > MaxVisionImageBytes {
		return storage.VisionInsights{}, fmt.Errorf("vision: image exceeds %d bytes", MaxVisionImageBytes)
	}
	mime := detectMime(data, mimeType)
	return g.analyzeData(ctx, data, mime)
}

func (g *GeminiAnalyzer) analyzeData(ctx context.Context, data []byte, mimeType string) (storage.VisionInsights, error) {
	prompt := `Du är en professionell svensk bostadsexpert. Beskriv bilden kortfattat och strukturerat.
Svara ENDAST med JSON med följande struktur:
{
  "summary": "1-2 meningar om rummet/miljön",
  "room_type": "vilket typ av rum/område bilden visar",
  "style": "vilken stil eller känsla",
  "notable_details": ["lista av intressanta detaljer"],
  "color_palette": ["viktiga färger"],
  "tags": ["korta etiketter"]
}`

	payload := map[string]any{
		"contents": []map[string]any{
			{
				"role": "user",
				"parts": []map[string]any{
					{"text": prompt},
					{
						"inline_data": map[string]string{
							"mime_type": mimeType,
							"data":      base64.StdEncoding.EncodeToString(data),
						},
					},
				},
			},
		},
		"generationConfig": map[string]any{
			"temperature": 0.2,
		},
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return storage.VisionInsights{}, fmt.Errorf("vision: marshal payload: %w", err)
	}

	endpoint := fmt.Sprintf("https://generativelanguage.googleapis.com/v1beta/models/%s:generateContent?key=%s", g.model, g.apiKey)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(body))
	if err != nil {
		return storage.VisionInsights{}, fmt.Errorf("vision: request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := g.client.Do(req)
	if err != nil {
		return storage.VisionInsights{}, fmt.Errorf("vision: perform request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 300 {
		var failure struct {
			Error struct {
				Message string `json:"message"`
			} `json:"error"`
		}
		_ = json.NewDecoder(resp.Body).Decode(&failure)
		return storage.VisionInsights{}, fmt.Errorf("vision: status %d: %s", resp.StatusCode, failure.Error.Message)
	}

	var completion struct {
		Candidates []struct {
			Content struct {
				Parts []struct {
					Text string `json:"text"`
				} `json:"parts"`
			} `json:"content"`
		} `json:"candidates"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&completion); err != nil {
		return storage.VisionInsights{}, fmt.Errorf("vision: decode response: %w", err)
	}

	if len(completion.Candidates) == 0 || len(completion.Candidates[0].Content.Parts) == 0 {
		return storage.VisionInsights{}, fmt.Errorf("vision: empty response")
	}

	text := strings.TrimSpace(completion.Candidates[0].Content.Parts[0].Text)
	return parseVisionJSON(text)
}

func (g *GeminiAnalyzer) fetchImage(ctx context.Context, imageURL string) ([]byte, string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, imageURL, nil)
	if err != nil {
		return nil, "", fmt.Errorf("vision: fetch %s: %w", imageURL, err)
	}
	resp, err := g.client.Do(req)
	if err != nil {
		return nil, "", fmt.Errorf("vision: fetch image: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 300 {
		return nil, "", fmt.Errorf("vision: image status %d", resp.StatusCode)
	}

	limited := io.LimitReader(resp.Body, MaxVisionImageBytes+1)
	data, err := io.ReadAll(limited)
	if err != nil {
		return nil, "", fmt.Errorf("vision: read image: %w", err)
	}
	if len(data) > MaxVisionImageBytes {
		return nil, "", fmt.Errorf("vision: image exceeds %d bytes", MaxVisionImageBytes)
	}

	return data, detectMime(data, resp.Header.Get("Content-Type")), nil
}

func parseVisionJSON(text string) (storage.VisionInsights, error) {
	var insights storage.VisionInsights
	if err := json.Unmarshal([]byte(text), &insights); err != nil {
		start := strings.Index(text, "{")
		end := strings.LastIndex(text, "}")
		if start >= 0 && end > start {
			if err := json.Unmarshal([]byte(text[start:end+1]), &insights); err != nil {
				return storage.VisionInsights{}, fmt.Errorf("vision: parse response: %w", err)
			}
		} else {
			return storage.VisionInsights{}, fmt.Errorf("vision: parse response: %w", err)
		}
	}

	return insights, nil
}

func detectMime(data []byte, provided string) string {
	mime := strings.TrimSpace(provided)
	if mime == "" {
		mime = http.DetectContentType(data)
	}
	if !strings.Contains(mime, "image/") {
		return "image/jpeg"
	}
	return mime
}

func normalizeVisionModel(model string) string {
	clean := strings.TrimSpace(model)
	clean = strings.TrimPrefix(clean, "models/")
	clean = strings.ToLower(clean)
	clean = strings.TrimSuffix(clean, "-latest")

	switch clean {
	case "", "gemini-1.5-flash", "gemini-1_5-flash":
		return defaultVisionModel
	default:
		return clean
	}
}
