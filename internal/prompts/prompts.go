package prompts

import (
	"encoding/json"
	"fmt"
	"strings"

	"k2MarketingAi/internal/storage"
)

const systemPrompt = "Du är en prisbelönt svensk copywriter för fastighetsmäklare. Du skriver på svenska, använder geodata när den finns och beskriver kommunikationer (buss/tåg/tunnelbana) konkret. Hitta inte på fakta. Hoppa över självklara basfunktioner och allt som beskriver vad man gör i rummen. Ta bara med det som är relevant och viktigt för boendet och håll texterna så korta som möjligt (max 225 ord totalt). Lyft alltid området (service, skolor/förskolor, natur, kommunikationer) när data finns. Om kunden har en stilprofil måste du följa den strikt."

const userPromptTemplate = `Returnera JSON {"sections":[{"slug":"","title":"","content":"","highlights":["..."]}, ...]}.
Krav:
- Skapa sektioner enligt "sections" i datan (intro, hall, kök, vardagsrum, sovrum/bad, område, avslutning).
- 1 mening per sektion. Skriv enkelt och rakt så att endast det absolut relevanta återstår.
- "highlights" ska innehålla 1–2 punktlistor med de starkaste argumenten för sektionen.
- Ta inte med självklara basfunktioner eller vad man gör i rummen; fokusera på det som verkligen säljer (läge, skick, material/ytskikt, ljus, utsikt, förvaring, förening, avgift, uteplats/balkong, energieffektivitet, geodata).
- Total text: max 225 ord (alla sektioner tillsammans).
- I område-sektionen: använd geodata/Transit för att nämna matbutiker, parker, träning, skolor/förskolor och kommunikationer (buss/tåg/tunnelbana) med uppskattade tider om de finns; undvik att konstatera självklarheter som att toaletten fyller sin funktion.
- Respektera ton, målgrupp och detaljer i datan. Om något saknas: skriv professionellt och generellt utan att hitta på.
Data:
%s`

type structuredSection struct {
	Slug  string `json:"slug"`
	Title string `json:"title"`
}

// BuildGenerationPrompts composes the system + user prompt pair used for text generation.
func BuildGenerationPrompts(listing storage.Listing) (string, string, error) {
	payload, err := buildStructuredPayload(listing)
	if err != nil {
		return "", "", err
	}

	userPrompt := fmt.Sprintf(userPromptTemplate, payload)
	if profile := FormatStyleProfile(listing.StyleProfile); profile != "" {
		userPrompt = fmt.Sprintf("%s\n\n%s", userPrompt, profile)
	}
	return systemPrompt, userPrompt, nil
}

// SystemPrompt returns the canonical system instruction used for all annonser.
func SystemPrompt() string {
	return systemPrompt
}

func buildStructuredPayload(listing storage.Listing) (string, error) {
	payload, err := json.Marshal(struct {
		Address        string              `json:"address"`
		Neighborhood   string              `json:"neighborhood"`
		City           string              `json:"city"`
		PropertyType   string              `json:"property_type"`
		Condition      string              `json:"condition"`
		Balcony        bool                `json:"balcony"`
		Floor          string              `json:"floor"`
		Association    string              `json:"association"`
		Length         string              `json:"length"`
		Tone           string              `json:"tone"`
		TargetAudience string              `json:"target_audience"`
		Highlights     []string            `json:"highlights"`
		Fee            int                 `json:"fee"`
		LivingArea     float64             `json:"living_area"`
		Rooms          float64             `json:"rooms"`
		Insights       storage.Insights    `json:"insights"`
		Details        storage.Details     `json:"details"`
		Sections       []structuredSection `json:"sections"`
		StyleProfileID string              `json:"style_profile_id"`
	}{
		Address:        listing.Address,
		Neighborhood:   listing.Neighborhood,
		City:           listing.City,
		PropertyType:   listing.PropertyType,
		Condition:      listing.Condition,
		Balcony:        listing.Balcony,
		Floor:          listing.Floor,
		Association:    listing.Association,
		Length:         listing.Length,
		Tone:           listing.Tone,
		TargetAudience: listing.TargetAudience,
		Highlights:     listing.Highlights,
		Fee:            listing.Fee,
		LivingArea:     listing.LivingArea,
		Rooms:          listing.Rooms,
		Insights:       listing.Insights,
		Details:        listing.Details,
		StyleProfileID: strings.TrimSpace(listing.Details.Meta.StyleProfileID),
		Sections: []structuredSection{
			{Slug: "intro", Title: "Inledning"},
			{Slug: "hall", Title: "Hall"},
			{Slug: "kitchen", Title: "Kök"},
			{Slug: "living", Title: "Vardagsrum"},
			{Slug: "sleep", Title: "Sovrum & bad"},
			{Slug: "area", Title: "Område & kommunikation"},
			{Slug: "closing", Title: "Sammanfattning"},
		},
	})
	if err != nil {
		return "", err
	}
	return string(payload), nil
}

// FormatStyleProfile renders a style profile as prompt instructions.
func FormatStyleProfile(profile *storage.StyleProfile) string {
	if profile == nil {
		return ""
	}
	var b strings.Builder
	fmt.Fprintf(&b, "Kundens stilprofil \"%s\":", profile.Name)
	if profile.Description != "" {
		fmt.Fprintf(&b, "\n- Beskrivning: %s", profile.Description)
	}
	if profile.Tone != "" {
		fmt.Fprintf(&b, "\n- Önskad ton: %s", profile.Tone)
	}
	if profile.Guidelines != "" {
		fmt.Fprintf(&b, "\n- Riktlinjer: %s", profile.Guidelines)
	}
	if len(profile.ExampleTexts) > 0 {
		fmt.Fprintf(&b, "\n- Förebilder (imitera rytm/ordval):")
		for i, ex := range profile.ExampleTexts {
			if trimmed := strings.TrimSpace(ex); trimmed != "" {
				fmt.Fprintf(&b, "\n  %d) %s", i+1, trimmed)
			}
		}
	}
	if len(profile.ForbiddenWords) > 0 {
		fmt.Fprintf(&b, "\n- Undvik orden: %s", strings.Join(profile.ForbiddenWords, ", "))
	}
	if profile.CustomModel != "" {
		fmt.Fprintf(&b, "\n- Denna kund tränas mot modellen: %s", profile.CustomModel)
	}
	return b.String()
}
