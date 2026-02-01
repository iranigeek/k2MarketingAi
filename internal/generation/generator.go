package generation

import (
	"context"
	"encoding/json"
	"fmt"
	"regexp"
	"strings"

	"k2MarketingAi/internal/geodata"
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

	fmt.Fprintf(&b, "V?lkommen till %s", listing.Address)
	if location != "" {
		fmt.Fprintf(&b, " i %s", location)
	}
	fmt.Fprintf(&b, " ? %s %s med %s.", strings.ToLower(tone), strings.ToLower(orDefault(listing.PropertyType, "bostad")), describeRooms(listing.Rooms, listing.LivingArea))

	var points []string
	if listing.Condition != "" {
		points = append(points, fmt.Sprintf("skick: %s", strings.ToLower(listing.Condition)))
	}
	if listing.Balcony {
		points = append(points, "balkong/uteplats")
	}
	if listing.Floor != "" {
		points = append(points, fmt.Sprintf("v?ning %s", listing.Floor))
	}
	if listing.Association != "" {
		if listing.Fee > 0 {
			points = append(points, fmt.Sprintf("f?rening %s, avgift ca %d kr/m?n", listing.Association, listing.Fee))
		} else {
			points = append(points, fmt.Sprintf("f?rening %s", listing.Association))
		}
	} else if listing.Fee > 0 {
		points = append(points, fmt.Sprintf("avgift ca %d kr/m?n", listing.Fee))
	}
	if len(listing.Highlights) > 0 {
		points = append(points, fmt.Sprintf("plus: %s", strings.Join(listing.Highlights, ", ")))
	}

	if len(points) > 0 {
		fmt.Fprintf(&b, " Nycklar: %s.", strings.Join(points, "; "))
	}
	if summary := geodata.FormatSummary(listing.Insights.Geodata); strings.TrimSpace(summary) != "" {
		fmt.Fprintf(&b, " Område: %s", ensurePeriod(summary))
	}

	return sanitizeContent(strings.TrimSpace(b.String()))
}

func (heuristicGenerator) Rewrite(_ context.Context, listing storage.Listing, section storage.Section, instruction string) (storage.Section, error) {
	base := section.Content
	if strings.TrimSpace(base) == "" {
		base = buildFullAd(listing)
	}

	section.Content = ApplyLocalRewrite(base, instruction)
	section.Content = sanitizeContent(section.Content)
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
	if summary := geodata.FormatSummary(listing.Insights.Geodata); summary != "" {
		return sanitizeContent(summary)
	}
	return "Området erbjuder närhet till vardagens alla måsten – service, grönområden och kommunikationer inom bekvämt gångavstånd."
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

func ensurePeriod(text string) string {
	trimmed := strings.TrimSpace(text)
	if trimmed == "" {
		return ""
	}
	switch {
	case strings.HasSuffix(trimmed, "."):
		return trimmed
	case strings.HasSuffix(trimmed, "!"), strings.HasSuffix(trimmed, "?"):
		return trimmed
	default:
		return trimmed + "."
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
- Hoppa över självklara basfunktioner och beskriv inte vad man gör i rummen.
- Nämn aldrig att toaletten fyller sin funktion.
- Undvik banala konstateranden som att man lagar mat i köket eller umgås i vardagsrummet; lyft det som är unikt och säljande.
- Håll rumssektioner korta; om geodata finns, låt området/kommunikationen ta plats.
- Skriv kortfattat, rakt och ta bara med det som är viktigt för boendet.
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
Ta bort självklarheter (ingen text om att "umgås i vardagsrum", "laga mat i kök" eller att toalett/badrum fyller basfunktioner). Prioritera geodata/kommunikation och konkreta säljdetaljer; korta ned rumsbeskrivningar hellre än att ta bort geodata.
`, section.Title, section.Slug, section.Content, originalWords, instruction, guideline, geodata.FormatPromptLines(listing.Insights.Geodata))
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
		return sanitizeSections(envelope.Sections), nil
	}

	start := strings.Index(content, "{")
	end := strings.LastIndex(content, "}")
	if start >= 0 && end > start {
		if err := json.Unmarshal([]byte(content[start:end+1]), &envelope); err == nil && len(envelope.Sections) > 0 {
			return sanitizeSections(envelope.Sections), nil
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
		fallback.Content = sanitizeContent(resp.Content)
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
- Nämn aldrig att toaletten fyller sin funktion eller andra självklarheter om badrum/toalett.
- Undvik att konstatera självklara saker som att köket används för matlagning eller vardagsrummet för umgänge – fokusera på det som är attraktivt och särskiljande.
- Håll dig till maximalt 225 ord och använd dem på säljande fakta, geodata och kvaliteter – ingen utfyllnad; korta hellre ned rumssektioner än geodata/kommunikation. Om geodata innehåller service/skola/universitet/pendel, lyft det.
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

	return sanitizeContent(strings.TrimSpace(content)), nil
}

func desiredWordCount(meta storage.MetaInfo) int {
	if meta.DesiredWordCount > 0 {
		if meta.DesiredWordCount > 225 {
			return 225
		}
		return meta.DesiredWordCount
	}
	return 150
}

func countWords(text string) int {
	fields := strings.Fields(text)
	return len(fields)
}

func sanitizeSections(sections []storage.Section) []storage.Section {
	for i := range sections {
		sections[i].Content = sanitizeContent(sections[i].Content)
	}
	return sections
}

func sanitizeContent(text string) string {
	phrase := "Köket är praktiskt utformat för vardagens behov och badrummet följer bostadens funktionella standard."
	lower := strings.ToLower(text)
	lowerPhrase := strings.ToLower(phrase)
	for {
		idx := strings.Index(lower, lowerPhrase)
		if idx < 0 {
			break
		}
		text = text[:idx] + text[idx+len(phrase):]
		lower = strings.ToLower(text)
	}
	// Trim spaces per line to keep intentional line breaks.
	lines := strings.Split(text, "\n")
	for i, line := range lines {
		lines[i] = strings.Join(strings.Fields(line), " ")
	}
	clean := strings.TrimSpace(strings.Join(lines, "\n"))
	return addParagraphBreaks(clean)
}

var sentenceBreakRegex = regexp.MustCompile(`([.!?])\s+([A-ZÅÄÖ])`)

// addParagraphBreaks inserts blank lines sparsely (every second sentence break) without changing wording.
func addParagraphBreaks(text string) string {
	count := 0
	return sentenceBreakRegex.ReplaceAllStringFunc(text, func(match string) string {
		count++
		if len(match) == 0 {
			return match
		}
		punct := string(match[0])
		rest := strings.TrimSpace(match[1:])
		if rest == "" {
			return match
		}
		if count%2 == 0 {
			return punct + "\n\n" + rest
		}
		return punct + " " + rest
	})
}
