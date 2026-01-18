package dataset

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"k2MarketingAi/internal/prompts"
	"k2MarketingAi/internal/storage"
)

// Example represents a single prompt/completion pair for fine-tuning.
type Example struct {
	ListingID      string `json:"listing_id"`
	StyleProfileID string `json:"style_profile_id,omitempty"`
	StyleProfile   string `json:"style_profile_name,omitempty"`
	InputText      string `json:"input_text"`
	OutputText     string `json:"output_text"`
	SectionCount   int    `json:"section_count"`
}

// Options control which listings are exported.
type Options struct {
	MinSections int
	MinWords    int
}

// BuildExamples converts listings to a consistent JSONL-friendly dataset.
func BuildExamples(listings []storage.Listing, opts Options) ([]Example, error) {
	if opts.MinSections <= 0 {
		opts.MinSections = 3
	}
	if opts.MinWords <= 0 {
		opts.MinWords = 120
	}

	var examples []Example
	for _, listing := range listings {
		if len(listing.Sections) < opts.MinSections {
			continue
		}
		output := strings.TrimSpace(listing.FullCopy)
		if output == "" {
			output = composeFullCopy(listing.Sections)
		}
		if wordCount(output) < opts.MinWords {
			continue
		}
		systemPrompt, userPrompt, err := prompts.BuildGenerationPrompts(listing)
		if err != nil {
			return nil, fmt.Errorf("build prompt for listing %s: %w", listing.ID, err)
		}
		prompt := strings.TrimSpace(systemPrompt + "\n\n" + userPrompt)
		example := Example{
			ListingID:    listing.ID,
			InputText:    prompt,
			OutputText:   output,
			SectionCount: len(listing.Sections),
		}
		styleID := strings.TrimSpace(listing.Details.Meta.StyleProfileID)
		if styleID != "" {
			example.StyleProfileID = styleID
		}
		if listing.StyleProfile != nil {
			example.StyleProfile = listing.StyleProfile.Name
		}
		examples = append(examples, example)
	}
	return examples, nil
}

func composeFullCopy(sections []storage.Section) string {
	var parts []string
	for _, section := range sections {
		content := strings.TrimSpace(section.Content)
		if content == "" {
			continue
		}
		title := strings.TrimSpace(section.Title)
		if title != "" {
			parts = append(parts, fmt.Sprintf("%s\n%s", title, content))
			continue
		}
		parts = append(parts, content)
	}
	return strings.Join(parts, "\n\n")
}

func wordCount(text string) int {
	return len(strings.Fields(text))
}

// WriteJSONL serializes examples to disk as JSON Lines.
func WriteJSONL(path string, examples []Example) error {
	file, err := os.Create(path)
	if err != nil {
		return err
	}
	defer file.Close()

	enc := json.NewEncoder(file)
	for _, ex := range examples {
		if err := enc.Encode(ex); err != nil {
			return err
		}
	}
	return nil
}
