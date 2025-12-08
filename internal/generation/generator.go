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
	Generate(ctx context.Context, listing storage.Listing) (Result, error)
	Rewrite(ctx context.Context, listing storage.Listing, section storage.Section, instruction string) (storage.Section, error)
}

// Result represents the output from a generator run.
type Result struct {
	Sections []storage.Section
	FullCopy string
}

// NewHeuristic returns a simple rules-based generator.
func NewHeuristic() Generator {
	return heuristicGenerator{}
}

type heuristicGenerator struct{}

func (heuristicGenerator) Generate(_ context.Context, listing storage.Listing) (Result, error) {
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

	return Result{
		Sections: sections,
		FullCopy: composeFullCopyFromSections(sections),
	}, nil
}

func (heuristicGenerator) Rewrite(_ context.Context, listing storage.Listing, section storage.Section, instruction string) (storage.Section, error) {
	base := section.Content
	if strings.TrimSpace(base) == "" {
		base = buildIntroCopy(listing)
	}

	section.Content = applyLocalRewrite(base, instruction)
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

func ApplyLocalRewrite(base, instruction string) string {
	cleaned := strings.TrimSpace(base)
	lower := strings.ToLower(instruction)

	// Shorten by keeping the first two sentences when asked for brevity.
	if strings.Contains(lower, "kort") || strings.Contains(lower, "short") {
		sentences := splitSentences(cleaned)
		if len(sentences) > 0 {
			if len(sentences) > 2 {
				sentences = sentences[:2]
			}
			cleaned = strings.Join(sentences, " ")
		}
	}

	// Light tone adjustments for fallback behaviour when no LLM is available.
	var tweaks []string
	switch {
	case strings.Contains(lower, "sälj"):
		tweaks = append(tweaks, "Texten är tonad mer säljande och engagerande")
	case strings.Contains(lower, "formell"):
		tweaks = append(tweaks, "Texten är mer formell och rak")
	case strings.Contains(lower, "tydlig") || strings.Contains(lower, "klar"):
		tweaks = append(tweaks, "Språket är förtydligat utan utfyllnad")
	case strings.Contains(lower, "längre"):
		tweaks = append(tweaks, "Texten är utvecklad med mer kontext")
	}

	if len(tweaks) == 0 {
		return cleaned
	}

	return fmt.Sprintf("%s\n\n%s.", cleaned, strings.Join(tweaks, ". "))
}

func splitSentences(text string) []string {
	fields := strings.FieldsFunc(text, func(r rune) bool {
		switch r {
		case '.', '!', '?':
			return true
		default:
			return false
		}
	})

	var sentences []string
	for _, part := range fields {
		if trimmed := strings.TrimSpace(part); trimmed != "" {
			sentences = append(sentences, trimmed)
		}
	}
	return sentences
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

var sectionGuidelines = map[string]string{
	"intro":   "Sätt scenen med adress, känsla och viktigaste argument.",
	"hall":    "Beskriv entréns intryck och funktion (ljus, förvaring, koppling till övriga ytor).",
	"kitchen": "Lyft material, vitvaror, förvaring och social matplats.",
	"living":  "Fokusera på rymd, ljus, utsikt och hur rummet används för umgänge.",
	"area":    "Summera service, rekreation och kommunikation från geodata.",
}

func (g *openAIGenerator) Generate(ctx context.Context, listing storage.Listing) (Result, error) {
	if hasPremiumDetails(listing.Details) {
		text, err := g.generatePremiumAd(ctx, listing)
		if err == nil {
			return Result{
				Sections: []storage.Section{{Slug: "ad", Title: "Annons", Content: text}},
				FullCopy: text,
			}, nil
		}
	}

	payload, _ := json.Marshal(struct {
		Address        string   `json:"address"`
		Tone           string   `json:"tone"`
		TargetAudience string   `json:"target_audience"`
		Highlights     []string `json:"highlights"`
		Fee            int      `json:"fee"`
		LivingArea     float64  `json:"living_area"`
		Rooms          float64  `json:"rooms"`
		Geodata        string   `json:"geodata"`
		Sections       []string `json:"sections"`
	}{
		Address:        listing.Address,
		Tone:           listing.Tone,
		TargetAudience: listing.TargetAudience,
		Highlights:     listing.Highlights,
		Fee:            listing.Fee,
		LivingArea:     listing.LivingArea,
		Rooms:          listing.Rooms,
		Geodata:        joinGeoInsights(listing.Insights.Geodata),
		Sections:       []string{"intro", "hall", "kitchen", "living", "area"},
	})

	systemPrompt := `Du är en erfaren svensk copywriter som skriver för premium-mäklare. Stilen ska kännas engagerande och målande utan klyschor som "ljus och fräsch".
- 2–4 meningar per sektion.
- Lyft faktiska detaljer (material, funktion, läge), väv in livsstil och känsla.
- Undvik upprepningar, använd varierat språk som i professionella bostadsannonser.
- Skriv allt på svenska.`
	userPrompt := fmt.Sprintf(`Generera JSON med fältet "sections" (lista av objekt med "slug","title","content") för sektionerna intro, hall, kitchen, living, area. Följ instruktionerna:
intro: måla upp bostadens själ, adress och tonalitet, nämn livsstil och ev. höjdpunkt (balkong, utsikt etc).
hall: beskriv entréns känsla och funktion (förvaring, ljus, första intryck).
kitchen: fokusera på matlagning, material och sociala ytor.
living: beskriv rymd, ljus, utsikt och känslan av samvaro.
area: använd geodata och sammanfatta mat/service, rekreation och kommunikation.

Exempelstil: "Här möter stads- puls Mälarens lugn..." (se payload).

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
	return Result{
		Sections: sections,
		FullCopy: composeFullCopyFromSections(sections),
	}, nil
}

func (g *openAIGenerator) Rewrite(ctx context.Context, listing storage.Listing, section storage.Section, instruction string) (storage.Section, error) {
	guideline := sectionGuidelines[strings.ToLower(section.Slug)]
	if guideline == "" {
		guideline = "Håll samma struktur men förbättra språk och tydlighet."
	}

	systemPrompt := `Du är en skicklig svensk copywriter. Polera text för en given sektion i en bostadsannons.
- 2–3 meningar, inga överdrifter eller klyschor.
- Behåll fakta men gör texten mer målande och säljande.
- Returnera JSON {"title":"...","content":"..."}.
`
	userPrompt := fmt.Sprintf(`Sektion: %s (%s)
Originaltext: """%s"""
Mäklarens instruktion: "%s"
Sektionens syfte: %s
Geodata: %s
`, section.Title, section.Slug, section.Content, instruction, guideline, joinGeoInsights(listing.Insights.Geodata))

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

func composeFullCopyFromSections(sections []storage.Section) string {
	var parts []string
	for _, section := range sections {
		if strings.TrimSpace(section.Content) == "" {
			continue
		}
		if section.Title != "" {
			parts = append(parts, fmt.Sprintf("%s\n%s", section.Title, section.Content))
		} else {
			parts = append(parts, section.Content)
		}
	}
	return strings.Join(parts, "\n\n")
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

func hasPremiumDetails(details storage.Details) bool {
	if details.Property.Address != "" {
		return true
	}
	if len(details.Advantages) > 0 {
		return true
	}
	if details.Meta.DesiredWordCount > 0 || details.Meta.Tone != "" {
		return true
	}
	return false
}

func (g *openAIGenerator) generatePremiumAd(ctx context.Context, listing storage.Listing) (string, error) {
	payload, _ := json.Marshal(struct {
		Meta        storage.MetaInfo        `json:"meta"`
		Property    storage.PropertyInfo    `json:"property"`
		Association storage.AssociationInfo `json:"association"`
		Area        storage.AreaInfo        `json:"area"`
		Advantages  []string                `json:"advantages"`
	}{
		Meta:        listing.Details.Meta,
		Property:    listing.Details.Property,
		Association: listing.Details.Association,
		Area:        listing.Details.Area,
		Advantages:  listing.Details.Advantages,
	})

	systemPrompt := `Du är en mycket skicklig svensk copywriter som skriver bostadsannonser åt mäklare.

- Skriv alltid på svenska.
- Variera språk, meningslängd och struktur i varje text.
- Anpassa ton och ordval efter målgruppen i datan.
- Undvik återkommande klyschor; texten ska kännas skriven av en människa.
- Presentera bostaden i ett sammanhållet flöde och avsluta gärna med en kort varierad punktlista.`

	userPrompt := fmt.Sprintf(`Skapa en unik bostadsannons baserat på JSON-datan nedan.
Följande ska uppnås:
- Textlängd ca %d ord.
- Ton som harmoniserar med "%s".
- Använd strukturen (pitch, bostad, kök, sovrum, badrum, uteplats, förening, område, punktlista) men ändra ordning/stil vid behov.

Data:
%s
`, desiredWordCount(listing.Details.Meta), listing.Details.Meta.Tone, string(payload))

	content, err := g.client.ChatCompletion(ctx, []llm.ChatMessage{
		{Role: "system", Content: systemPrompt},
		{Role: "user", Content: userPrompt},
	}, 0.9)
	if err != nil {
		return "", err
	}

	return strings.TrimSpace(content), nil
}

func desiredWordCount(meta storage.MetaInfo) int {
	if meta.DesiredWordCount > 0 {
		return meta.DesiredWordCount
	}
	return 280
}
