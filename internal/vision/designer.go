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
	if err := json.Unmarshal([]byte(content), &concept); err == nil {
		return concept, nil
	}
	start := strings.Index(content, "{")
	end := strings.LastIndex(content, "}")
	if start >= 0 && end > start {
		if err := json.Unmarshal([]byte(content[start:end+1]), &concept); err == nil {
			return concept, nil
		}
	}
	return DesignConcept{}, fmt.Errorf("vision: could not parse design response")
}
