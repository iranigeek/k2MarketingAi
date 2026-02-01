package geodata

import (
	"fmt"
	"strings"

	"k2MarketingAi/internal/storage"
)

// FormatSummary builds a short, human-friendly summary of the surrounding area.
func FormatSummary(geo storage.GeodataInsights) string {
	if len(geo.PointsOfInterest) == 0 && len(geo.Transit) == 0 {
		return ""
	}

	grouped := groupPOIByCategory(geo.PointsOfInterest)

	var sentences []string

	// Prioritera det som säljer: vardagsservice, kommunikation, skolor och grönska.
	if names := summarizeList(grouped["grocery"], 2); names != "" {
		sentences = append(sentences, fmt.Sprintf("Matbutiker som %s ligger nära för snabba ärenden.", names))
	}
	if len(geo.Transit) > 0 {
		var highlights []string
		for i := 0; i < len(geo.Transit) && i < 2; i++ {
			mode := strings.TrimSpace(geo.Transit[i].Mode)
			desc := strings.TrimSpace(geo.Transit[i].Description)
			switch {
			case mode != "" && desc != "":
				highlights = append(highlights, fmt.Sprintf("%s (%s)", mode, desc))
			case desc != "":
				highlights = append(highlights, desc)
			case mode != "":
				highlights = append(highlights, mode)
			}
		}
		if summary := summarizeList(highlights, len(highlights)); summary != "" {
			sentences = append(sentences, fmt.Sprintf("Kommunikationerna är smidiga med %s.", summary))
		}
	}
	if names := summarizeList(grouped["school"], 2); names != "" {
		sentences = append(sentences, fmt.Sprintf("Skolor och förskolor finns i närheten, bland annat %s.", names))
	}
	if names := summarizeList(grouped["park"], 2); names != "" {
		sentences = append(sentences, fmt.Sprintf("Gröna platser som %s ger sköna andrum.", names))
	}

	// Fyll ut med ett extra säljargument om utrymme finns.
	if len(sentences) < 3 {
		if names := summarizeList(append(grouped["restaurant"], grouped["cafe"]...), 2); names != "" {
			sentences = append(sentences, fmt.Sprintf("Restauranger och kaféer som %s finns runt knuten.", names))
		}
	}
	if len(sentences) < 3 {
		if names := summarizeList(grouped["service"], 1); names != "" {
			sentences = append(sentences, fmt.Sprintf("Service är lättillgänglig vid %s.", names))
		}
	}

	// Max tre meningar för att hålla det kompakt.
	if len(sentences) > 3 {
		sentences = sentences[:3]
	}
	return strings.TrimSpace(strings.Join(sentences, " "))
}

// FormatPromptLines renders geodata as bullet-friendly lines for prompts.
func FormatPromptLines(geo storage.GeodataInsights) string {
	grouped := groupPOIByCategory(geo.PointsOfInterest)
	var lines []string

	if names := summarizeList(grouped["grocery"], 3); names != "" {
		lines = append(lines, fmt.Sprintf("- Matbutiker: %s", names))
	}
	if names := summarizeList(append(grouped["restaurant"], grouped["cafe"]...), 3); names != "" {
		lines = append(lines, fmt.Sprintf("- Restauranger/kaféer: %s", names))
	}
	if names := summarizeList(grouped["school"], 3); names != "" {
		lines = append(lines, fmt.Sprintf("- Skolor/förskolor: %s", names))
	}
	if names := summarizeList(grouped["park"], 3); names != "" {
		lines = append(lines, fmt.Sprintf("- Parker/natur: %s", names))
	}
	if names := summarizeList(grouped["gym"], 2); names != "" {
		lines = append(lines, fmt.Sprintf("- Träning: %s", names))
	}
	if names := summarizeList(grouped["health"], 2); names != "" {
		lines = append(lines, fmt.Sprintf("- Vård/apotek: %s", names))
	}
	if names := summarizeList(grouped["service"], 3); names != "" {
		lines = append(lines, fmt.Sprintf("- Service/ärenden: %s", names))
	}
	if names := summarizeList(grouped["parking"], 1); names != "" {
		lines = append(lines, fmt.Sprintf("- Parkering/påfarter: %s", names))
	}
	if names := summarizeList(grouped["other"], 1); names != "" {
		lines = append(lines, fmt.Sprintf("- Övrigt: %s", names))
	}
	if len(geo.Transit) > 0 {
		var highlights []string
		for i := 0; i < len(geo.Transit) && i < 3; i++ {
			highlights = append(highlights, strings.TrimSpace(fmt.Sprintf("%s (%s)", geo.Transit[i].Mode, geo.Transit[i].Description)))
		}
		if summary := summarizeList(highlights, len(highlights)); summary != "" {
			lines = append(lines, fmt.Sprintf("- Kommunikationer: %s", summary))
		}
	}

	return strings.TrimSpace(strings.Join(lines, "\n"))
}

func groupPOIByCategory(pois []storage.PointOfInterest) map[string][]string {
	grouped := map[string][]string{}
	for _, poi := range pois {
		category := normalizeCategory(poi.Category)
		entry := strings.TrimSpace(strings.Join([]string{strings.TrimSpace(poi.Name), strings.TrimSpace(poi.Distance)}, " "))
		if entry != "" {
			grouped[category] = append(grouped[category], entry)
		}
	}
	return grouped
}

func normalizeCategory(category string) string {
	switch strings.ToLower(strings.TrimSpace(category)) {
	case "matbutik":
		return "grocery"
	case "restaurang":
		return "restaurant"
	case "café", "cafe", "cafÇ¸":
		return "cafe"
	case "park":
		return "park"
	case "gym":
		return "gym"
	case "apotek", "pharmacy":
		return "health"
	case "sjukhus", "hospital":
		return "health"
	case "gas_station", "bensinstation":
		return "service"
	case "parking":
		return "parking"
	case "shopping_mall", "butik", "store":
		return "service"
	case "skola", "förskola", "forskola":
		return "school"
	default:
		return "other"
	}
}

func summarizeList(items []string, limit int) string {
	filtered := make([]string, 0, len(items))
	for _, item := range items {
		if trimmed := strings.TrimSpace(item); trimmed != "" {
			filtered = append(filtered, trimmed)
		}
	}
	items = filtered

	if len(items) == 0 {
		return ""
	}
	if limit > 0 && len(items) > limit {
		items = items[:limit]
	}
	if len(items) == 1 {
		return items[0]
	}
	last := items[len(items)-1]
	return fmt.Sprintf("%s och %s", strings.Join(items[:len(items)-1], ", "), last)
}
