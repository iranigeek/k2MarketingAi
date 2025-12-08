package listings

import "k2MarketingAi/internal/storage"

// hydrateDetailsFromLegacy ensures the new Details structure mirrors the legacy fields.
func hydrateDetailsFromLegacy(listing *storage.Listing) {
	if listing == nil {
		return
	}

	// Meta defaults
	if listing.Details.Meta.DesiredWordCount == 0 {
		listing.Details.Meta.DesiredWordCount = 300
	}
	if listing.Details.Meta.Tone == "" {
		listing.Details.Meta.Tone = listing.Tone
	}
	if listing.Details.Meta.TargetAudience == "" {
		listing.Details.Meta.TargetAudience = listing.TargetAudience
	}
	if listing.Details.Meta.LanguageVariant == "" {
		listing.Details.Meta.LanguageVariant = "svenska_standard"
	}

	// Property defaults
	prop := &listing.Details.Property
	if prop.Address == "" {
		prop.Address = listing.Address
	}
	if prop.PropertyType == "" {
		prop.PropertyType = "Bostad"
	}
	if prop.Tenure == "" {
		prop.Tenure = "okänd"
	}
	if prop.Rooms == 0 && listing.Rooms > 0 {
		prop.Rooms = listing.Rooms
	}
	if prop.LivingArea == 0 && listing.LivingArea > 0 {
		prop.LivingArea = listing.LivingArea
	}
	if prop.FeePerMonth == 0 && listing.Fee > 0 {
		prop.FeePerMonth = listing.Fee
	}
	if prop.PlanSummary == "" {
		prop.PlanSummary = "Planlösning enligt specifikation."
	}
	if prop.KitchenDescription == "" {
		prop.KitchenDescription = "Kök enligt specifikation."
	}
	if prop.BedroomDescription == "" {
		prop.BedroomDescription = "Sovrum enligt planlösning."
	}
	if prop.LivingDescription == "" {
		prop.LivingDescription = "Ljust och socialt vardagsrum."
	}
	if prop.BathroomDescription == "" {
		prop.BathroomDescription = "Badrum enligt uppgift."
	}
	if prop.OutdoorDescription == "" && listing.ImageURL != "" {
		prop.OutdoorDescription = "Se bilder för uteplats/balkong."
	}

	// Advantages fallback to highlights.
	if len(listing.Details.Advantages) == 0 && len(listing.Highlights) > 0 {
		listing.Details.Advantages = append([]string(nil), listing.Highlights...)
	}
}
