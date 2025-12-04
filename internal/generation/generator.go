package generation

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"k2MarketingAi/internal/llm"
	"k2MarketingAi/internal/storage"
)

// Generator creates and rewrites listing sections.
type Generator interface {
	Generate(ctx context.Context, listing storage.Listing) ([]storage.Section, error)
	Rewrite(ctx context.Context, listing storage.Listing, section storage.Section, instruction string) (storage.Section, error)
}

// NewHeuristic returns a simple rules-based generator.
func NewHeuristic() Generator {
	return heuristicGenerator{}
}

type heuristicGenerator struct{}

func (heuristicGenerator) Generate(_ context.Context, listing storage.Listing) ([]storage.Section, error) {
	sections := []storage.Section{
		{Slug: "intro", Title: "Inledning"},
		{Slug: "hall", Title: "Hall"},
		{Slug: "kitchen", Title: "Kök"},
		{Slug: "living", Title: "Vardagsrum"},
		{Slug: "area", Title: "Området"},
	}

	for idx := range sections {
		switch sections[idx].Slug {
		case "intro":
			sections[idx].Content = buildIntroCopy(listing)
		case "hall":
			sections[idx].Content = buildHallCopy(listing)
		case "kitchen":
			sections[idx].Content = buildKitchenCopy(listing)
		case "living":
			sections[idx].Content = buildLivingCopy(listing)
		case "area":
			sections[idx].Content = buildAreaCopy(listing)
		}
	}

	return sections, nil
}

func (heuristicGenerator) Rewrite(_ context.Context, listing storage.Listing, section storage.Section, instruction string) (storage.Section, error) {
	base := section.Content
	if strings.TrimSpace(base) == "" {
		base = buildIntroCopy(listing)
	}

	var builder strings.Builder
	builder.WriteString(strings.TrimSpace(base))
	if instruction != "" {
		builder.WriteString(fmt.Sprintf("\n\nInstruktion: %s.", instruction))
	}

	section.Content = builder.String()
	return section, nil
}

func buildIntroCopy(listing storage.Listing) string {
	var b strings.Builder
	fmt.Fprintf(&b, "Välkommen till %s – där %s möter en trivsam planlösning.", listing.Address, strings.ToLower(defaultTone(listing.Tone)))
	if listing.LivingArea > 0 && listing.Rooms > 0 {
		fmt.Fprintf(&b, " Här får du ca %.1f kvm fördelade på %s ljusa rum.", listing.LivingArea, formatRooms(listing.Rooms))
	}
	if len(listing.Highlights) > 0 {
		fmt.Fprintf(&b, " Highlights: %s.", strings.Join(listing.Highlights, ", "))
	}
	return b.String()
}

func buildHallCopy(listing storage.Listing) string {
	return fmt.Sprintf("Entrén öppnar upp mot en välkomnande hall med bra förvaring och en tydlig siktlinje mot hemmets sociala delar. %s", emphasizeInstruction(listing.TargetAudience))
}

func buildKitchenCopy(listing storage.Listing) string {
	return "Köket bjuder på generösa arbetsytor, tidlösa materialval och plats för många middagar med vänner. Här finns både vardagsfunktion och det där lilla extra som får bostaden att sticka ut."
}

func buildLivingCopy(listing storage.Listing) string {
	return "Vardagsrummet är hemmets naturliga mittpunkt med stora fönsterpartier och mjukt ljusinsläpp dagen lång. Här ryms både soffgrupp, läshörna och favoritmöbeln utan att kompromissa med rymden."
}

func buildAreaCopy(listing storage.Listing) string {
	geo := listing.Insights.Geodata
	if len(geo.PointsOfInterest) == 0 && len(geo.Transit) == 0 {
		return "Området erbjuder närhet till vardagens alla måsten – service, grönområden och kommunikationer inom bekvämt gångavstånd."
	}

	grouped := map[string][]string{}
	for _, poi := range geo.PointsOfInterest {
		key := categorizePOI(poi.Category)
		entry := fmt.Sprintf("%s (%s)", poi.Name, poi.Distance)
		grouped[key] = append(grouped[key], entry)
	}

	var sentences []string
	if names := summarizeList(grouped["grocery"], 2); names != "" {
		sentences = append(sentences, fmt.Sprintf("Matbutiker som %s ligger bara några minuter bort.", names))
	}
	if names := summarizeList(append(grouped["restaurant"], grouped["cafe"]...), 2); names != "" {
		sentences = append(sentences, fmt.Sprintf("I kvarteret väntar restauranger och caféer som %s.", names))
	}
	if names := summarizeList(grouped["park"], 2); names != "" {
		sentences = append(sentences, fmt.Sprintf("För rekreation finns gröna platser som %s.", names))
	}
	if names := summarizeList(grouped["gym"], 1); names != "" {
		sentences = append(sentences, fmt.Sprintf("Den som vill träna gör det enkelt på %s.", names))
	}

	if len(geo.Transit) > 0 {
		var highlights []string
		for i := 0; i < len(geo.Transit) && i < 2; i++ {
			highlights = append(highlights, fmt.Sprintf("%s (%s)", geo.Transit[i].Mode, geo.Transit[i].Description))
		}
		if summary := summarizeList(highlights, len(highlights)); summary != "" {
			sentences = append(sentences, fmt.Sprintf("Kommunikationerna är utmärkta med %s.", summary))
		}
	}

	if len(sentences) == 0 {
		return "Området erbjuder närhet till vardagens alla måsten – service, grönområden och kommunikationer inom bekvämt gångavstånd."
	}

	return strings.Join(sentences, " ")
}

func emphasizeInstruction(target string) string {
	if target == "" {
		return ""
	}
	return fmt.Sprintf("Perfekt anpassat för %s.", strings.ToLower(target))
}

func defaultTone(tone string) string {
	if tone == "" {
		return "varm och familjär"
	}
	return tone
}

func formatRooms(rooms float64) string {
	if rooms == 0 {
		return ""
	}
	if rooms == float64(int(rooms)) {
		return fmt.Sprintf("%d", int(rooms))
	}
	return fmt.Sprintf("%.1f", rooms)
}

func categorizePOI(category string) string {
	switch strings.ToLower(category) {
	case "matbutik", "butik":
		return "grocery"
	case "restaurang":
		return "restaurant"
	case "café":
		return "cafe"
	case "park":
		return "park"
	case "gym":
		return "gym"
	default:
		return "other"
	}
}

func summarizeList(items []string, limit int) string {
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

// NewOpenAI wires the generator to OpenAI's chat completions.
func NewOpenAI(client *llm.OpenAIClient) Generator {
	return &openAIGenerator{
		client:   client,
		fallback: heuristicGenerator{},
	}
}

type openAIGenerator struct {
	client   *llm.OpenAIClient
	fallback Generator
}

func (g *openAIGenerator) Generate(ctx context.Context, listing storage.Listing) ([]storage.Section, error) {
	payload, _ := json.Marshal(struct {
		Address        string           `json:"address"`
		Tone           string           `json:"tone"`
		TargetAudience string           `json:"target_audience"`
		Highlights     []string         `json:"highlights"`
		Fee            int              `json:"fee"`
		LivingArea     float64          `json:"living_area"`
		Rooms          float64          `json:"rooms"`
		Insights       storage.Insights `json:"insights"`
		Sections       []string         `json:"sections"`
	}{
		Address:        listing.Address,
		Tone:           listing.Tone,
		TargetAudience: listing.TargetAudience,
		Highlights:     listing.Highlights,
		Fee:            listing.Fee,
		LivingArea:     listing.LivingArea,
		Rooms:          listing.Rooms,
		Insights:       listing.Insights,
		Sections:       []string{"intro", "hall", "kitchen", "living", "area"},
	})

	systemPrompt := "Du är en prisbelönt svensk copywriter för fastighetsmäklare. Du skriver korrekta, inspirerande texter på svenska, med fokus på fakta och känsla."
	userPrompt := fmt.Sprintf(`Generera JSON med fältet "sections" som innehåller en lista av objekt { "slug": "intro", "title": "Inledning", "content": "..." } för varje sektion i ordningen intro, hall, kitchen, living, area. Håll varje content 2-4 meningar.
Data:
%s`, string(payload))

	content, err := g.client.ChatCompletion(ctx, []llm.ChatMessage{
		{Role: "system", Content: systemPrompt},
		{Role: "user", Content: userPrompt},
	}, 0.4)
	if err != nil {
		return g.fallback.Generate(ctx, listing)
	}

	sections, parseErr := parseSections(content)
	if parseErr != nil {
		return g.fallback.Generate(ctx, listing)
	}
	return sections, nil
}

func (g *openAIGenerator) Rewrite(ctx context.Context, listing storage.Listing, section storage.Section, instruction string) (storage.Section, error) {
	payload, _ := json.Marshal(struct {
		Section     storage.Section `json:"section"`
		Instruction string          `json:"instruction"`
		Listing     storage.Listing `json:"listing"`
	}{
		Section:     section,
		Instruction: instruction,
		Listing:     listing,
	})

	systemPrompt := "Du är en svensk copywriter som förbättrar en specifik sektion i en bostadsbeskrivning."
	userPrompt := fmt.Sprintf(`Skriv om följande sektion i JSON-format {"title":"...", "content":"..."} med samma språk men följ instruktionerna. Behåll titel om ingen förbättring behövs.
Data:
%s`, string(payload))

	content, err := g.client.ChatCompletion(ctx, []llm.ChatMessage{
		{Role: "system", Content: systemPrompt},
		{Role: "user", Content: userPrompt},
	}, 0.5)
	if err != nil {
		return g.fallback.Rewrite(ctx, listing, section, instruction)
	}

	updated, parseErr := parseSection(content, section)
	if parseErr != nil {
		return g.fallback.Rewrite(ctx, listing, section, instruction)
	}
	return updated, nil
}

func parseSections(content string) ([]storage.Section, error) {
	var envelope struct {
		Sections []storage.Section `json:"sections"`
	}
	if err := json.Unmarshal([]byte(content), &envelope); err == nil && len(envelope.Sections) > 0 {
		return envelope.Sections, nil
	}

	start := strings.Index(content, "{")
	end := strings.LastIndex(content, "}")
	if start >= 0 && end > start {
		if err := json.Unmarshal([]byte(content[start:end+1]), &envelope); err == nil && len(envelope.Sections) > 0 {
			return envelope.Sections, nil
		}
	}
	return nil, fmt.Errorf("could not parse sections from response")
}

func parseSection(content string, fallback storage.Section) (storage.Section, error) {
	var resp struct {
		Title   string `json:"title"`
		Content string `json:"content"`
	}
	if err := json.Unmarshal([]byte(content), &resp); err != nil {
		start := strings.Index(content, "{")
		end := strings.LastIndex(content, "}")
		if start >= 0 && end > start {
			if err := json.Unmarshal([]byte(content[start:end+1]), &resp); err != nil {
				return storage.Section{}, err
			}
		} else {
			return storage.Section{}, err
		}
	}

	if resp.Title != "" {
		fallback.Title = resp.Title
	}
	if resp.Content != "" {
		fallback.Content = resp.Content
	}
	return fallback, nil
}
