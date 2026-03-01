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

// ImageEditor extends generation with reference-image editing.
type ImageEditor interface {
	EditImage(ctx context.Context, prompt string, imageData []byte, mimeType string) (ImageResult, error)
}

// ImageResult represents a rendered image payload.
type ImageResult struct {
	Data string `json:"data,omitempty"`
	MIME string `json:"mime,omitempty"`
	URL  string `json:"url,omitempty"`
	Key  string `json:"key,omitempty"`
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

	resp, err := client.Models.GenerateContent(childCtx, g.model, genai.Text(prompt), &genai.GenerateContentConfig{
		ResponseModalities: []string{"IMAGE", "TEXT"},
	})
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

// EditImage edits an existing room image using Gemini, preserving the camera
// angle and architecture while only changing furniture and decor.
func (g *GeminiImageGenerator) EditImage(ctx context.Context, prompt string, imageData []byte, mimeType string) (ImageResult, error) {
	if g == nil || strings.TrimSpace(g.apiKey) == "" {
		return ImageResult{}, fmt.Errorf("vision: image generator unavailable")
	}
	if strings.TrimSpace(prompt) == "" {
		return ImageResult{}, fmt.Errorf("vision: tom prompt för redigering")
	}
	if len(imageData) == 0 {
		return ImageResult{}, fmt.Errorf("vision: referensbild saknas")
	}

	childCtx, cancel := context.WithTimeout(ctx, g.timeout)
	defer cancel()

	client, err := genai.NewClient(childCtx, &genai.ClientConfig{
		APIKey: g.apiKey,
	})
	if err != nil {
		return ImageResult{}, fmt.Errorf("vision: skapa genai-klient: %w", err)
	}

	if strings.TrimSpace(mimeType) == "" {
		mimeType = "image/jpeg"
	}

	// ── System instruction: laser-focused, ultra-short. ──
	// Long instructions cause the model to lose track of constraints.
	// Keep it to the absolute minimum so windows/camera are not diluted.
	systemConstraint := &genai.Content{
		Parts: []*genai.Part{
			{Text: "You edit room photos. Rules:\n" +
				"- NEVER move, remove, resize or add windows. Windows stay pixel-identical.\n" +
				"- NEVER change the camera angle, perspective or crop.\n" +
				"- ONLY replace furniture, decor, wall paint, floor finish and textiles."},
		},
	}

	// ── User message: image FIRST so the model anchors on it, then a short edit instruction. ──
	// The shorter the edit instruction, the more faithfully the model preserves structure.
	editInstruction := "Edit this photo: replace all furniture and decor with new " + strings.TrimSpace(prompt) +
		"\nKeep every window exactly where it is. Same camera angle."

	contents := []*genai.Content{
		{
			Role: "user",
			Parts: []*genai.Part{
				// Image first — model sees the room before reading what to change.
				{InlineData: &genai.Blob{MIMEType: mimeType, Data: imageData}},
				{Text: editInstruction},
			},
		},
	}

	resp, err := client.Models.GenerateContent(childCtx, g.model, contents, &genai.GenerateContentConfig{
		SystemInstruction:  systemConstraint,
		ResponseModalities: []string{"IMAGE", "TEXT"},
		Temperature:        floatPtr(0.1),
	})
	if err != nil {
		return ImageResult{}, fmt.Errorf("vision: redigering misslyckades: %w", err)
	}
	if len(resp.Candidates) == 0 || len(resp.Candidates[0].Content.Parts) == 0 {
		return ImageResult{}, fmt.Errorf("vision: redigering saknar kandidater")
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
	return ImageResult{}, fmt.Errorf("vision: redigering gav ingen bilddata")
}

func floatPtr(f float32) *float32 { return &f }
