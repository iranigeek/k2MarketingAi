package generation

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"k2MarketingAi/internal/llm"
	"k2MarketingAi/internal/prompts"
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
	content := buildFullAd(listing)
	sections := []storage.Section{{Slug: "main", Title: "Annons", Content: content}}
	return Result{
		Sections: sections,
		FullCopy: composeFullCopyFromSections(sections),
	}, nil
}

func buildFullAd(listing storage.Listing) string {
	var b strings.Builder
	location := strings.TrimSpace(strings.Join([]string{listing.Neighborhood, listing.City}, ", "))
	tone := defaultTone(listing.Tone)

	fmt.Fprintf(&b, "Välkommen till %s", listing.Address)
	if location != "" {
		fmt.Fprintf(&b, " i %s", location)
	}
	fmt.Fprintf(&b, " – en %s %s som levererar %s.", strings.ToLower(tone), strings.ToLower(orDefault(listing.PropertyType, "bostad")), describeRooms(listing.Rooms, listing.LivingArea))

	if listing.Balcony {
		fmt.Fprint(&b, " Här finns balkong eller uteplats för naturligt ljus och frisk luft.")
	}
	if listing.Condition != "" {
		fmt.Fprintf(&b, " Skicket upplevs som %s vilket gör det enkelt att flytta in.", strings.ToLower(listing.Condition))
	}
	if listing.Floor != "" {
		fmt.Fprintf(&b, " Våning: %s.", listing.Floor)
	}
	if listing.Association != "" {
		fmt.Fprintf(&b, " Förening: %s.", listing.Association)
	}
	if len(listing.Highlights) > 0 {
		fmt.Fprintf(&b, " Fördelar: %s.", strings.Join(listing.Highlights, ", "))
	}

	fmt.Fprint(&b, "\n\nPlanlösningen nyttjar ytan smart mellan sociala och privata delar. Köket och vardagsrummet bjuder in till både vardag och middagar, medan sovrummen ger ro. Badrumsdelen beskrivs neutralt utan påhittade detaljer för att hålla fakta korrekt.")

	fmt.Fprint(&b, "\n\nOmrådet kan beskrivas generellt med närhet till service, natur eller kommunikationer beroende på vad som faktiskt finns tillgängligt. Texten undviker att hitta på namn eller siffror som inte finns i underlaget.")

	fmt.Fprint(&b, "\n\nAvslutningen sammanfattar bostadens känsla och inbjuder till visning med ett språk som känns unikt, varierat och professionellt.")

	return strings.TrimSpace(b.String())
}

func (heuristicGenerator) Rewrite(_ context.Context, listing storage.Listing, section storage.Section, instruction string) (storage.Section, error) {
	base := section.Content
	if strings.TrimSpace(base) == "" {
		base = buildFullAd(listing)
	}

	section.Content = ApplyLocalRewrite(base, instruction)
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

func orDefault(value, fallback string) string {
	if strings.TrimSpace(value) == "" {
		return fallback
	}
	return value
}

func describeRooms(rooms float64, area float64) string {
	switch {
	case rooms > 0 && area > 0:
		return fmt.Sprintf("%s över ca %.0f kvm", formatRooms(rooms), area)
	case rooms > 0:
		return fmt.Sprintf("%s med flexibel yta", formatRooms(rooms))
	case area > 0:
		return fmt.Sprintf("funktionella %.0f kvm", area)
	default:
		return "välbalanserad planlösning"
	}
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

// NewLLM wires the generator to any chat-completion capable client.
func NewLLM(client llm.Client) Generator {
	return &llmGenerator{client: client}
}

type llmGenerator struct {
	client llm.Client
}

var sectionGuidelines = map[string]string{
	"intro":   "Sätt scenen med adress, känsla och viktigaste argument.",
	"hall":    "Beskriv entréns intryck och funktion (ljus, förvaring, koppling till övriga ytor).",
	"kitchen": "Lyft material, vitvaror, förvaring och social matplats.",
	"living":  "Fokusera på rymd, ljus, utsikt och hur rummet används för umgänge.",
	"area":    "Summera service, rekreation och kommunikation från geodata.",
}

func (g *llmGenerator) Generate(ctx context.Context, listing storage.Listing) (Result, error) {
	if hasPremiumDetails(listing.Details) {
		text, err := g.generatePremiumAd(ctx, listing)
		if err == nil {
			return Result{
				Sections: []storage.Section{{Slug: "ad", Title: "Annons", Content: text}},
				FullCopy: text,
			}, nil
		}
	}

	systemPrompt, userPrompt, err := prompts.BuildGenerationPrompts(listing)
	if err != nil {
		return Result{}, err
	}

	content, err := g.client.ChatCompletion(ctx, []llm.ChatMessage{
		{Role: "system", Content: systemPrompt},
		{Role: "user", Content: userPrompt},
	}, 0.4)
	if err != nil {
		return Result{}, err
	}

	sections, parseErr := parseSections(content)
	if parseErr != nil {
		return Result{}, parseErr
	}
	return Result{
		Sections: sections,
		FullCopy: composeFullCopyFromSections(sections),
	}, nil
}

func (g *llmGenerator) Rewrite(ctx context.Context, listing storage.Listing, section storage.Section, instruction string) (storage.Section, error) {
	guideline := sectionGuidelines[strings.ToLower(section.Slug)]
	if guideline == "" {
		guideline = "Håll samma struktur men förbättra språk och tydlighet."
	}

	originalWords := countWords(section.Content)
	systemPrompt := `Du är en skicklig svensk copywriter. Polera text för en given sektion i en bostadsannons.
- Undvik klyschor och överdrifter.
- Behåll fakta men gör texten mer målande och säljande.
- Matcha ursprunglig längd (minst 85 % av originalet) eller gör den något längre.
- Följ kundens stilprofil om den finns.
- Returnera JSON {"title":"...","content":"..."}.
`
	userPrompt := fmt.Sprintf(`Sektion: %s (%s)
Originaltext: """%s"""
Originalets längd: %d ord (matcha denna längd, ±15%%)
Mäklarens instruktion: "%s"
Sektionens syfte: %s
Geodata: %s
`, section.Title, section.Slug, section.Content, originalWords, instruction, guideline, joinGeoInsights(listing.Insights.Geodata))
	if profile := prompts.FormatStyleProfile(listing.StyleProfile); profile != "" {
		userPrompt = fmt.Sprintf(`%s

%s`, userPrompt, profile)
	}

	content, err := g.client.ChatCompletion(ctx, []llm.ChatMessage{
		{Role: "system", Content: systemPrompt},
		{Role: "user", Content: userPrompt},
	}, 0.5)
	if err != nil {
		return storage.Section{}, err
	}

	updated, parseErr := parseSection(content, section)
	if parseErr != nil {
		return storage.Section{}, parseErr
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

func (g *llmGenerator) generatePremiumAd(ctx context.Context, listing storage.Listing) (string, error) {
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

func joinGeoInsights(geo storage.GeodataInsights) string {
	if len(geo.PointsOfInterest) == 0 && len(geo.Transit) == 0 {
		return ""
	}

	var parts []string
	if len(geo.PointsOfInterest) > 0 {
		var poiSummaries []string
		for _, poi := range geo.PointsOfInterest {
			name := strings.TrimSpace(poi.Name)
			category := strings.TrimSpace(poi.Category)
			distance := strings.TrimSpace(poi.Distance)

			var poiParts []string
			if name != "" {
				poiParts = append(poiParts, name)
			}
			if category != "" {
				poiParts = append(poiParts, strings.ToLower(category))
			}
			if distance != "" {
				poiParts = append(poiParts, distance)
			}

			if summary := strings.Join(poiParts, ", "); summary != "" {
				poiSummaries = append(poiSummaries, summary)
			}
		}
		if len(poiSummaries) > 0 {
			parts = append(parts, fmt.Sprintf("POI: %s", strings.Join(poiSummaries, "; ")))
		}
	}

	if len(geo.Transit) > 0 {
		var transitSummaries []string
		for _, transit := range geo.Transit {
			mode := strings.TrimSpace(transit.Mode)
			desc := strings.TrimSpace(transit.Description)

			switch {
			case mode != "" && desc != "":
				transitSummaries = append(transitSummaries, fmt.Sprintf("%s (%s)", mode, desc))
			case desc != "":
				transitSummaries = append(transitSummaries, desc)
			case mode != "":
				transitSummaries = append(transitSummaries, mode)
			}
		}
		if len(transitSummaries) > 0 {
			parts = append(parts, fmt.Sprintf("Transit: %s", strings.Join(transitSummaries, "; ")))
		}
	}

	return strings.Join(parts, " | ")
}

func countWords(text string) int {
	fields := strings.Fields(text)
	return len(fields)
}
