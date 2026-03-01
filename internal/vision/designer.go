package vision

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"k2MarketingAi/internal/llm"
)

// Designer creates interior concepts based on prompts.
type Designer interface {
	Design(ctx context.Context, prompt string) (DesignConcept, error)
}

// DesignConcept represents a structured space proposal.
type DesignConcept struct {
	Summary  string   `json:"summary"`
	Mood     string   `json:"mood"`
	Layout   string   `json:"layout"`
	Items    []string `json:"items"`
	Palette  []string `json:"palette"`
	Lighting string   `json:"lighting"`
	Notes    []string `json:"notes"`
}

// GeminiDesigner wraps the chat client for design prompts.
type GeminiDesigner struct {
	client llm.Client
}

// NewGeminiDesigner constructs a designer backed by the given chat client.
func NewGeminiDesigner(client llm.Client) *GeminiDesigner {
	return &GeminiDesigner{client: client}
}

// Design generates a concept using Gemini.
func (d *GeminiDesigner) Design(ctx context.Context, prompt string) (DesignConcept, error) {
	if d == nil || d.client == nil {
		return DesignConcept{}, fmt.Errorf("vision: designer unavailable")
	}
	if strings.TrimSpace(prompt) == "" {
		return DesignConcept{}, fmt.Errorf("vision: instructions required")
	}

	systemPrompt := `Du är en svensk inredningsarkitekt som tar fram kreativa men genomförbara designförslag.
- Beskriv lösningen kort men konkret.
- Hitta inte på fakta om bostaden, utgå endast från instruktionen.
- Svara alltid som JSON med fälten: summary, mood, layout, items (lista), palette (lista), lighting, notes (lista).`
	userPrompt := fmt.Sprintf(`Ta fram en designplan för följande önskemål:
%s
`, prompt)

	content, err := d.client.ChatCompletion(ctx, []llm.ChatMessage{
		{Role: "system", Content: systemPrompt},
		{Role: "user", Content: userPrompt},
	}, 0.3)
	if err != nil {
		return DesignConcept{}, err
	}

	return parseDesignConcept(content)
}

func parseDesignConcept(content string) (DesignConcept, error) {
	var concept DesignConcept

	// Try raw JSON first.
	if err := json.Unmarshal([]byte(content), &concept); err == nil {
		return concept, nil
	}

	// Strip markdown code fences (```json ... ``` or ``` ... ```).
	cleaned := content
	if idx := strings.Index(cleaned, "```"); idx >= 0 {
		cleaned = cleaned[idx+3:]
		// Strip closing fence first.
		if end := strings.LastIndex(cleaned, "```"); end >= 0 {
			cleaned = cleaned[:end]
		}
		// Remove optional language tag ("json", "JSON", etc.) before the actual JSON.
		// The tag might be on the same line as the opening brace or on its own line.
		cleaned = strings.TrimSpace(cleaned)
		for _, prefix := range []string{"json", "JSON", "Json"} {
			if strings.HasPrefix(cleaned, prefix) {
				cleaned = strings.TrimSpace(cleaned[len(prefix):])
				break
			}
		}
		cleaned = strings.TrimSpace(cleaned)
		if err := json.Unmarshal([]byte(cleaned), &concept); err == nil {
			return concept, nil
		}
	}

	// Try extracting the outermost { ... } block.
	start := strings.Index(content, "{")
	end := strings.LastIndex(content, "}")
	if start >= 0 && end > start {
		if err := json.Unmarshal([]byte(content[start:end+1]), &concept); err == nil {
			return concept, nil
		}
	}
	return DesignConcept{}, fmt.Errorf("vision: could not parse design response: %s", truncate(content, 200))
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "…"
}
