package brfintel

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"k2MarketingAi/internal/llm"
)

// ──────────────────────────────────────────────────────────────────────
// LLM-Powered Summary Generation
//
// Two distinct outputs:
// 1. Buyer summary  — pedagogisk, enkel svenska för köpare
// 2. Legal view     — professionell säkerhetsvy för mäklare
//
// Also houses the LLM-based financial extraction from raw text.
// ──────────────────────────────────────────────────────────────────────

// generateBuyerSummary creates a plain-language Swedish summary for apartment buyers.
func (a *Analyzer) generateBuyerSummary(ctx context.Context, report BRFReport) (string, error) {
	if a.llm == nil {
		return "", fmt.Errorf("LLM client not configured")
	}

	modelCtx := llm.WithModel(ctx, "gemini-3-pro-preview")

	systemPrompt := `Du är en ekonomisk rådgivare som hjälper bostadsköpare förstå bostadsrättsföreningars ekonomi.

Skriv en pedagogisk sammanfattning på enkel svenska som en vanlig person utan ekonomisk utbildning kan förstå.

Regler:
- Använd vardagligt språk, undvik facktermer
- Förklara vad siffrorna faktiskt betyder för köparen i praktiken
- Var ärlig om risker men inte alarmistisk
- Fokusera på: avgiftsnivå, skuldsättning, framtida avgiftsrisk, underhållsläge
- Använd konkreta jämförelser ("det är som att...", "jämfört med snittet...")
- Max 4–5 korta stycken
- Avsluta med en sammanfattande mening ("Sammanfattning: ...")
- Svara bara med texten, ingen JSON`

	userPrompt := fmt.Sprintf(`BRF-analys för %s (%s):

📊 BRF Score: %d/100 (Betyg: %s — %s)

Nyckeltal:
- Skuld per m²: %.0f kr
- Avgift per m²/år: %.0f kr
- Årsresultat: %.0f kr
- Likvida medel: %.0f kr
- Reparationsfond: %.0f kr
- Byggår: %d
- Markstatus: %s

Riskvarningar:
%s

Ekonomisk trend: %s — %s`,
		report.BRFName,
		report.OrgNumber,
		report.Score.Total,
		report.Score.Grade,
		report.Score.Label,
		report.Financials.DebtPerSqm,
		report.Financials.AvgiftPerSqm,
		report.Financials.NetResult,
		report.Financials.CashAndBank,
		report.Financials.RepairFund,
		report.Financials.BuildYear,
		nonEmpty(report.Financials.LandStatus, "Äganderätt (antas)"),
		formatRisksForPrompt(report.Risks),
		report.Trends.Direction,
		report.Trends.Summary,
	)

	reply, err := a.llm.ChatCompletion(modelCtx, []llm.ChatMessage{
		{Role: "system", Content: systemPrompt},
		{Role: "user", Content: userPrompt},
	}, 0.6)
	if err != nil {
		return "", fmt.Errorf("buyer summary LLM: %w", err)
	}

	return strings.TrimSpace(reply), nil
}

// generateLegalView creates a professional risk/safety assessment for real estate agents.
func (a *Analyzer) generateLegalView(ctx context.Context, report BRFReport) (string, error) {
	if a.llm == nil {
		return "", fmt.Errorf("LLM client not configured")
	}

	modelCtx := llm.WithModel(ctx, "gemini-3-pro-preview")

	systemPrompt := `Du är en juridisk och ekonomisk rådgivare som hjälper fastighetsmäklare med due diligence av bostadsrättsföreningar.

Skriv en professionell säkerhetsvy som en mäklare kan använda i sin bedömning och dokumentation.

Regler:
- Använd professionellt men tydligt språk
- Strukturera med tydliga rubriker
- Fokusera på: juridiska risker, ekonomisk stabilitet, potentiella informationsbrister, rekommenderade kontrollpunkter
- Hänvisa till relevanta lagar/regler om tillämpligt (Bostadsrättslagen, God mäklarsed)
- Markera särskilt saker mäklaren BÖR informera köparen om
- Indikera om BRF:en klarar normala kreditprövningskrav
- Max 5–6 strukturerade stycken
- Svara bara med texten, ingen JSON`

	userPrompt := fmt.Sprintf(`BRF-analys för %s (org.nr: %s):

BRF Score: %d/100 (Betyg: %s)

Finansiella nyckeltal:
- Skuld per m²: %.0f kr (rikssnitt ~5 000–7 000)
- Avgift per m²/år: %.0f kr (rikssnitt ~600–800)
- Årsresultat: %.0f kr
- Totala lån: %.0f kr
- Kassa/bank: %.0f kr
- Reparationsfond: %.0f kr
- Räntekostnader: %.0f kr
- Avskrivningar: %.0f kr

Fastighetsdata:
- Byggår: %d
- Markstatus: %s
- Tomträttsutgång: %s
- Energiklass: %s

Underhåll:
- Genomfört: %s
- Planerat: %s

Riskvarningar (%d st):
%s

Ekonomisk trend: %s`,
		report.BRFName,
		report.OrgNumber,
		report.Score.Total,
		report.Score.Grade,
		report.Financials.DebtPerSqm,
		report.Financials.AvgiftPerSqm,
		report.Financials.NetResult,
		report.Financials.TotalDebt,
		report.Financials.CashAndBank,
		report.Financials.RepairFund,
		report.Financials.InterestCosts,
		report.Financials.Depreciation,
		report.Financials.BuildYear,
		nonEmpty(report.Financials.LandStatus, "Ej angivet"),
		nonEmpty(report.Financials.LandLeaseExpiry, "Ej tillämpligt"),
		nonEmpty(report.Financials.EnergyClass, "Ej angivet"),
		nonEmpty(report.Financials.RenovationsDone, "Ej angivet"),
		nonEmpty(report.Financials.RenovationsPlanned, "Ej angivet"),
		len(report.Risks),
		formatRisksForPrompt(report.Risks),
		report.Trends.Summary,
	)

	reply, err := a.llm.ChatCompletion(modelCtx, []llm.ChatMessage{
		{Role: "system", Content: systemPrompt},
		{Role: "user", Content: userPrompt},
	}, 0.4)
	if err != nil {
		return "", fmt.Errorf("legal view LLM: %w", err)
	}

	return strings.TrimSpace(reply), nil
}

// extractFinancialsFromText uses the LLM to extract structured financials from raw text.
func (a *Analyzer) extractFinancialsFromText(ctx context.Context, text string) (Financials, error) {
	if a.llm == nil {
		return Financials{}, fmt.Errorf("LLM client not configured")
	}

	modelCtx := llm.WithModel(ctx, "gemini-3-pro-preview")

	// Truncate if needed
	sanitized := text
	if len(sanitized) > 20000 {
		sanitized = sanitized[:20000]
	}

	systemPrompt := `Du är en svensk ekonom som analyserar årsredovisningar från bostadsrättsföreningar.

Extrahera alla tillgängliga ekonomiska nyckeltal. Om data finns för flera år, inkludera alla.
Svara ENBART med JSON, ingen annan text. Använd 0 för saknade numeriska värden.`

	userPrompt := fmt.Sprintf(`Text från årsredovisning:
"""
%s
"""

Returnera JSON med denna exakta struktur:
{
  "property_designation": "",
  "build_year": 0,
  "number_of_units": 0,
  "boa_total": 0,
  "loa_total": 0,
  "fee_income": 0,
  "rental_income": 0,
  "interest_costs": 0,
  "depreciation": 0,
  "operating_costs": 0,
  "maintenance_costs": 0,
  "total_debt": 0,
  "cash_and_bank": 0,
  "repair_fund": 0,
  "total_assets": 0,
  "fee_per_month": 0,
  "debt_per_sqm": 0,
  "avgift_per_sqm": 0,
  "net_result": 0,
  "land_status": "",
  "land_lease_expiry": "",
  "energy_class": "",
  "energy_consumption": 0,
  "renovations_done": "",
  "renovations_planned": "",
  "yearly_snapshots": [
    {
      "year": 2024,
      "fee_income": 0,
      "interest_costs": 0,
      "operating_costs": 0,
      "maintenance_costs": 0,
      "net_result": 0,
      "total_debt": 0,
      "cash_and_bank": 0,
      "repair_fund": 0,
      "debt_per_sqm": 0,
      "avgift_per_sqm": 0
    }
  ]
}

Inkludera yearly_snapshots för varje år du hittar data. Fyll i alla fält du kan hitta.`, sanitized)

	reply, err := a.llm.ChatCompletion(modelCtx, []llm.ChatMessage{
		{Role: "system", Content: systemPrompt},
		{Role: "user", Content: userPrompt},
	}, 0.15)
	if err != nil {
		return Financials{}, fmt.Errorf("extract financials LLM: %w", err)
	}

	// Clean JSON
	clean := cleanLLMJSON(reply)

	var fin Financials
	if err := json.Unmarshal([]byte(clean), &fin); err != nil {
		// Try via map for flexible parsing
		var m map[string]any
		if uerr := json.Unmarshal([]byte(clean), &m); uerr != nil {
			return Financials{}, fmt.Errorf("parse LLM response: %w (snippet: %s)", uerr, firstN(clean, 200))
		}
		fin = financialsFromMap(m)
	}

	return fin, nil
}

// ──────────────────────────────────────────────────────────────────────
// Helpers
// ──────────────────────────────────────────────────────────────────────

func formatRisksForPrompt(risks []RiskWarning) string {
	if len(risks) == 0 {
		return "Inga riskvarningar identifierade."
	}
	var b strings.Builder
	for _, r := range risks {
		emoji := "ℹ️"
		switch r.Severity {
		case "critical":
			emoji = "🔴"
		case "high":
			emoji = "🟠"
		case "medium":
			emoji = "🟡"
		case "low":
			emoji = "🟢"
		}
		fmt.Fprintf(&b, "%s [%s] %s: %s", emoji, strings.ToUpper(r.Severity), r.Title, r.Description)
		if r.Metric != "" {
			fmt.Fprintf(&b, " (%s)", r.Metric)
		}
		b.WriteString("\n")
	}
	return b.String()
}

func nonEmpty(s, fallback string) string {
	s = strings.TrimSpace(s)
	if s == "" || strings.EqualFold(s, "okänd") {
		return fallback
	}
	return s
}

func cleanLLMJSON(s string) string {
	trim := strings.TrimSpace(s)
	if strings.HasPrefix(trim, "```") {
		trim = strings.TrimPrefix(trim, "```json")
		trim = strings.TrimPrefix(trim, "```")
		trim = strings.TrimSuffix(trim, "```")
	}
	return strings.TrimSpace(trim)
}

func firstN(s string, n int) string {
	runes := []rune(s)
	if len(runes) <= n {
		return s
	}
	return string(runes[:n])
}

func financialsFromMap(m map[string]any) Financials {
	fin := Financials{
		PropertyDesignation: strVal(m, "property_designation"),
		BuildYear:           intVal(m, "build_year"),
		NumberOfUnits:       intVal(m, "number_of_units"),
		BOATotal:            floatVal(m, "boa_total"),
		LOATotal:            floatVal(m, "loa_total"),
		FeeIncome:           floatVal(m, "fee_income"),
		RentalIncome:        floatVal(m, "rental_income"),
		InterestCosts:       floatVal(m, "interest_costs"),
		Depreciation:        floatVal(m, "depreciation"),
		OperatingCosts:      floatVal(m, "operating_costs"),
		MaintenanceCosts:    floatVal(m, "maintenance_costs"),
		TotalDebt:           floatVal(m, "total_debt"),
		CashAndBank:         floatVal(m, "cash_and_bank"),
		RepairFund:          floatVal(m, "repair_fund"),
		TotalAssets:         floatVal(m, "total_assets"),
		FeePerMonth:         floatVal(m, "fee_per_month"),
		DebtPerSqm:          floatVal(m, "debt_per_sqm"),
		AvgiftPerSqm:        floatVal(m, "avgift_per_sqm"),
		NetResult:           floatVal(m, "net_result"),
		LandStatus:          strVal(m, "land_status"),
		LandLeaseExpiry:     strVal(m, "land_lease_expiry"),
		EnergyClass:         strVal(m, "energy_class"),
		EnergyConsumption:   floatVal(m, "energy_consumption"),
		RenovationsDone:     strVal(m, "renovations_done"),
		RenovationsPlanned:  strVal(m, "renovations_planned"),
	}

	// Parse yearly_snapshots if present
	if snapshots, ok := m["yearly_snapshots"]; ok {
		if arr, ok := snapshots.([]any); ok {
			for _, item := range arr {
				if sm, ok := item.(map[string]any); ok {
					fin.YearlySnapshots = append(fin.YearlySnapshots, parseYearlySnapshot(intVal(sm, "year"), sm))
				}
			}
		}
	}

	return fin
}
