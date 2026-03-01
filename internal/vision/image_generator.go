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

	// Build a strong wrapper that hammers the two immutable constraints
	// (camera angle + windows) and makes everything else freely changeable.
	fullPrompt := "" +
		"⚠️ CRITICAL RULE — READ BEFORE ANYTHING ELSE ⚠️\n" +
		"NEVER remove, add, resize, move, or obscure ANY window. Every window visible in the input photo MUST appear in the output at the EXACT same position, size, and shape. This rule overrides ALL other instructions.\n\n" +

		"════════ 🔒 LOCKED — NEVER CHANGE ════════\n" +
		"Only TWO things are locked:\n" +
		"1. CAMERA — Keep the EXACT camera angle, perspective, lens distortion, focal length, composition, and crop. Do not rotate, pan, tilt, zoom, or reframe even slightly.\n" +
		"2. WINDOWS — Keep every window EXACTLY as-is: same count, same size, same position, same glass, same frame. If the photo shows 1 window → output has 1 window in the same spot. If it shows 4 → output has 4 in the same spots. NEVER remove, add, cover, or resize a window.\n\n" +

		"════════ ✅ FREELY CHANGEABLE ════════\n" +
		"You MAY (and SHOULD) dramatically change ALL of the following:\n" +
		"• WALL COLOR — repaint walls any color (white, dark, accent wall, etc.)\n" +
		"• FLOOR — change floor color, finish, add/replace rugs\n" +
		"• ALL FURNITURE — replace every sofa, bed, table, chair, shelf, cabinet with completely new ones\n" +
		"• CURTAINS & BLINDS — replace with new style/color (but the window behind must still be visible/intact)\n" +
		"• LIGHTING — replace all lamps, pendants, sconces\n" +
		"• TEXTILES — new rugs, cushions, throws, bedding\n" +
		"• WALL DECOR — new art, mirrors, shelves\n" +
		"• PLANTS & ACCESSORIES — new plants, books, candles, vases\n" +
		"• DOORS — may change door color/style\n" +
		"• CEILING — may change ceiling color\n\n" +

		"════════ HOW TO EXECUTE ════════\n" +
		"Step 1: Mentally EMPTY the room — remove every movable object.\n" +
		"Step 2: Verify all windows are still present and untouched.\n" +
		"Step 3: FILL the room with a completely new interior. Every item must be different from the original.\n" +
		"Step 4: Before outputting, COUNT the windows in your result and CONFIRM the count matches the input.\n\n" +

		"MINIMUM new items: main furniture (sofa/bed/dining table), secondary furniture (coffee table, side tables, bookshelf/console), large area rug, curtains, 3+ light sources (pendant + floor lamp + table lamp), throw pillows, 2-3 art pieces, 1-2 plants, decorative accessories.\n\n" +

		"The change must be DRAMATIC — like a professional before/after home renovation. Magazine-quality staging.\n" +
		"OUTPUT: Single photorealistic image, 4K quality, natural lighting.\n\n" +

		"⚠️ FINAL CHECK: Does the output have the SAME number of windows in the SAME positions as the input? If not, REDO it. ⚠️\n\n" +

		"════════ STYLE INSTRUCTIONS FROM USER ════════\n" +
		prompt

	contents := []*genai.Content{
		{
			Role: "user",
			Parts: []*genai.Part{
				{InlineData: &genai.Blob{MIMEType: mimeType, Data: imageData}},
				{Text: fullPrompt},
			},
		},
	}

	resp, err := client.Models.GenerateContent(childCtx, g.model, contents, &genai.GenerateContentConfig{
		ResponseModalities: []string{"IMAGE", "TEXT"},
		Temperature:        floatPtr(0.9),
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
