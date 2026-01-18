package dataset

import (
	"fmt"
	"regexp"
	"strings"

	"k2MarketingAi/internal/prompts"
)

// RawOptions defines how docx/text snippets should be converted into training examples.
type RawOptions struct {
	StyleProfileID   string
	StyleProfileName string
	Tone             string
	MaxBullets       int
}

var sentenceEndings = regexp.MustCompile(`[.!?]+`)

// BuildExamplesFromRawEntries converts plain-text annonser to prompt/completion pairs.
func BuildExamplesFromRawEntries(entries []string, opts RawOptions) ([]Example, error) {
	if opts.MaxBullets <= 0 {
		opts.MaxBullets = 8
	}
	if strings.TrimSpace(opts.Tone) == "" {
		opts.Tone = "professionell och varm"
	}

	var examples []Example
	for idx, entry := range entries {
		entry = strings.TrimSpace(entry)
		if entry == "" {
			continue
		}
		sentences := splitSentences(entry)
		if len(sentences) < 2 {
			continue
		}
		intro := sentences[0]
		detailSentences := sentences[1:]
		if len(detailSentences) > opts.MaxBullets {
			detailSentences = detailSentences[:opts.MaxBullets]
		}
		bullets := make([]string, 0, len(detailSentences))
		for _, sentence := range detailSentences {
			line := strings.TrimSpace(sentence)
			if line == "" {
				continue
			}
			bullets = append(bullets, line)
		}
		if len(bullets) == 0 {
			bullets = []string{intro}
		}
		facts := "- " + strings.Join(bullets, "\n- ")
		propertyType := guessPropertyType(entry)
		userPrompt := fmt.Sprintf(`Skapa en komplett svensk bostadsannons med 5–7 sektioner (rubrik + stycke) och avslutande sammanfattning.
Adress eller ingång: %s
Bostadstyp: %s
Ton: %s
Nyckeldetaljer:
%s

Inkludera konkreta exempel och variera språket så att texten känns handskriven.`, intro, propertyType, opts.Tone, facts)

		input := strings.TrimSpace(prompts.SystemPrompt() + "\n\n" + userPrompt)
		example := Example{
			ListingID:    fmt.Sprintf("docx-%03d", idx+1),
			InputText:    input,
			OutputText:   entry,
			SectionCount: 6,
		}
		if opts.StyleProfileID != "" {
			example.StyleProfileID = opts.StyleProfileID
		}
		if opts.StyleProfileName != "" {
			example.StyleProfile = opts.StyleProfileName
		}
		examples = append(examples, example)
	}
	return examples, nil
}

func splitSentences(text string) []string {
	text = strings.TrimSpace(text)
	if text == "" {
		return nil
	}
	var sentences []string
	var current strings.Builder
	for _, r := range text {
		current.WriteRune(r)
		if strings.ContainsRune(".!?", r) {
			chunk := strings.TrimSpace(current.String())
			if chunk != "" {
				sentences = append(sentences, chunk)
			}
			current.Reset()
		}
	}
	if tail := strings.TrimSpace(current.String()); tail != "" {
		sentences = append(sentences, tail)
	}
	return sentences
}

func guessPropertyType(text string) string {
	lower := strings.ToLower(text)
	switch {
	case strings.Contains(lower, "radhus"):
		return "radhus"
	case strings.Contains(lower, "parhus"):
		return "parhus"
	case strings.Contains(lower, "villa"):
		return "villa"
	case strings.Contains(lower, "studio"):
		return "studio"
	case strings.Contains(lower, "lägenhet"), strings.Contains(lower, "tvårummare"), strings.Contains(lower, "tredrum"):
		return "lägenhet"
	case strings.Contains(lower, "parhus"):
		return "parhus"
	default:
		return "bostad"
	}
}
