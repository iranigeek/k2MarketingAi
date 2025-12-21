package vision

import (
	"context"
	"encoding/base64"
	"fmt"
	"strings"
	"time"

	"google.golang.org/genai"
)

// ImageGenerator returns rendered interiors based on prompts.
type ImageGenerator interface {
	Generate(ctx context.Context, prompt string) (ImageResult, error)
}

// ImageResult represents a rendered image payload.
type ImageResult struct {
	Data string `json:"data"`
	MIME string `json:"mime"`
}

// GeminiImageGenerator renders interiors via Gemini image outputs.
type GeminiImageGenerator struct {
	apiKey  string
	model   string
	timeout time.Duration
}

const defaultImageModel = "gemini-2.5-flash-image"

// NewGeminiImageGenerator constructs a generator able to request inline images.
func NewGeminiImageGenerator(apiKey, model string, timeout time.Duration) *GeminiImageGenerator {
	if strings.TrimSpace(model) == "" {
		model = defaultImageModel
	}
	model = strings.TrimPrefix(strings.TrimSpace(model), "models/")
	if timeout <= 0 {
		timeout = 60 * time.Second
	}
	return &GeminiImageGenerator{
		apiKey:  apiKey,
		model:   model,
		timeout: timeout,
	}
}

// Generate requests a photorealistic image for the given prompt.
func (g *GeminiImageGenerator) Generate(ctx context.Context, prompt string) (ImageResult, error) {
	if g == nil || strings.TrimSpace(g.apiKey) == "" {
		return ImageResult{}, fmt.Errorf("vision: image generator unavailable")
	}
	if strings.TrimSpace(prompt) == "" {
		return ImageResult{}, fmt.Errorf("vision: tom prompt för rendering")
	}

	childCtx, cancel := context.WithTimeout(ctx, g.timeout)
	defer cancel()

	client, err := genai.NewClient(childCtx, &genai.ClientConfig{
		APIKey: g.apiKey,
	})
	if err != nil {
		return ImageResult{}, fmt.Errorf("vision: skapa genai-klient: %w", err)
	}

	resp, err := client.Models.GenerateContent(childCtx, g.model, genai.Text(prompt), nil)
	if err != nil {
		return ImageResult{}, fmt.Errorf("vision: render misslyckades: %w", err)
	}
	if len(resp.Candidates) == 0 || len(resp.Candidates[0].Content.Parts) == 0 {
		return ImageResult{}, fmt.Errorf("vision: render saknar kandidater")
	}

	for _, part := range resp.Candidates[0].Content.Parts {
		if part.InlineData == nil || len(part.InlineData.Data) == 0 {
			continue
		}
		mime := part.InlineData.MIMEType
		if strings.TrimSpace(mime) == "" {
			mime = "image/png"
		}
		encoded := base64.StdEncoding.EncodeToString(part.InlineData.Data)
		return ImageResult{
			Data: encoded,
			MIME: mime,
		}, nil
	}
	return ImageResult{}, fmt.Errorf("vision: render gav ingen bilddata")
}
