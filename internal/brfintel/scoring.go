package brfintel

import (
	"fmt"
	"math"
	"strings"
)

// ──────────────────────────────────────────────────────────────────────
// BRF Score Engine — 0–100 composite scoring
//
// Dimensions (weights sum to 100):
//   30% Skuldsättning (debt level)
//   20% Likviditet (liquidity / cash position)
//   20% Underhåll (maintenance/repair fund)
//   15% Avgiftsnivå (fee reasonableness)
//   15% Resultat (annual result stability)
//
// Each dimension scores 0–100 independently, then the weighted average
// becomes the composite. Grades: A (80+), B (65–79), C (50–64),
// D (35–49), E (20–34), F (<20).
// ──────────────────────────────────────────────────────────────────────

func computeScore(fin Financials) BRFScore {
	dims := []ScoreDimension{
		scoreDebt(fin),
		scoreLiquidity(fin),
		scoreMaintenance(fin),
		scoreFee(fin),
		scoreResult(fin),
	}

	var totalWeighted float64
	var totalWeight int
	for _, d := range dims {
		totalWeighted += float64(d.Score) * float64(d.Weight)
		totalWeight += d.Weight
	}

	composite := 50 // default if no data
	if totalWeight > 0 {
		composite = int(math.Round(totalWeighted / float64(totalWeight)))
	}
	composite = clamp(composite, 0, 100)

	return BRFScore{
		Total:      composite,
		Grade:      grade(composite),
		Label:      label(composite),
		Dimensions: dims,
	}
}

// ── Debt scoring (30%) ──
// Swedish BRF benchmark: < 5 000 kr/m² = excellent, > 15 000 = concerning
func scoreDebt(fin Financials) ScoreDimension {
	d := ScoreDimension{
		Name:   "Skuldsättning",
		Weight: 30,
	}

	debtPerSqm := fin.DebtPerSqm
	if debtPerSqm == 0 && fin.TotalDebt > 0 && fin.BOATotal > 0 {
		debtPerSqm = fin.TotalDebt / fin.BOATotal
	}

	if debtPerSqm == 0 {
		d.Score = 50
		d.Description = "Uppgift om skuldsättning saknas."
		return d
	}

	// Scoring curve: 0 debt = 100, 5000 = 85, 10000 = 60, 15000 = 35, 20000+ = 10
	switch {
	case debtPerSqm <= 0:
		d.Score = 100
		d.Description = "Föreningen är skuldfri — utmärkt."
	case debtPerSqm <= 3000:
		d.Score = 95
		d.Description = "Mycket låg skuldsättning."
	case debtPerSqm <= 5000:
		d.Score = 85
		d.Description = "Låg skuldsättning, under genomsnittet."
	case debtPerSqm <= 8000:
		d.Score = 70
		d.Description = "Genomsnittlig skuldsättning."
	case debtPerSqm <= 10000:
		d.Score = 55
		d.Description = "Något förhöjd skuldsättning."
	case debtPerSqm <= 13000:
		d.Score = 40
		d.Description = "Hög skuldsättning, kan medföra avgiftshöjningar."
	case debtPerSqm <= 16000:
		d.Score = 25
		d.Description = "Mycket hög skuldsättning — risk för kraftiga avgiftshöjningar."
	default:
		d.Score = 10
		d.Description = "Extremt hög skuldsättning — allvarlig risk."
	}

	d.Description += formatMetric(" Skuld: %.0f kr/m².", debtPerSqm)
	return d
}

// ── Liquidity scoring (20%) ──
// cash_and_bank relative to operating costs or absolute thresholds.
func scoreLiquidity(fin Financials) ScoreDimension {
	d := ScoreDimension{
		Name:   "Likviditet",
		Weight: 20,
	}

	if fin.CashAndBank == 0 {
		d.Score = 50
		d.Description = "Uppgift om likvida medel saknas."
		return d
	}

	// If we know operating costs, measure months of runway
	if fin.OperatingCosts > 0 {
		monthsRunway := (fin.CashAndBank / fin.OperatingCosts) * 12
		switch {
		case monthsRunway >= 12:
			d.Score = 95
			d.Description = "Mycket god likviditet, över 12 månaders driftreserv."
		case monthsRunway >= 6:
			d.Score = 80
			d.Description = "God likviditet med god driftreserv."
		case monthsRunway >= 3:
			d.Score = 60
			d.Description = "Acceptabel likviditet, men begränsad reserv."
		case monthsRunway >= 1:
			d.Score = 35
			d.Description = "Låg likviditet — föreningen kan behöva höja avgifter eller låna."
		default:
			d.Score = 15
			d.Description = "Kritiskt låg likviditet — risk för betalningssvårigheter."
		}
		d.Description += formatMetric(" Kassa: %.0f kr (ca %.1f månaders drift).", fin.CashAndBank, monthsRunway)
		return d
	}

	// Absolute fallback
	switch {
	case fin.CashAndBank >= 5_000_000:
		d.Score = 90
		d.Description = "Stora likvida medel."
	case fin.CashAndBank >= 2_000_000:
		d.Score = 75
		d.Description = "Goda likvida medel."
	case fin.CashAndBank >= 500_000:
		d.Score = 55
		d.Description = "Begränsade likvida medel."
	default:
		d.Score = 30
		d.Description = "Låga likvida medel."
	}

	d.Description += formatMetric(" Kassa: %.0f kr.", fin.CashAndBank)
	return d
}

// ── Maintenance scoring (20%) ──
// Checks repair fund adequacy and planned maintenance transparency.
func scoreMaintenance(fin Financials) ScoreDimension {
	d := ScoreDimension{
		Name:   "Underhåll & reparationsfond",
		Weight: 20,
	}

	score := 50 // neutral start
	reasons := []string{}

	// Repair fund per m²
	if fin.RepairFund > 0 && fin.BOATotal > 0 {
		repairPerSqm := fin.RepairFund / fin.BOATotal
		switch {
		case repairPerSqm >= 500:
			score += 25
			reasons = append(reasons, formatMetric("Reparationsfond: %.0f kr/m² (god).", repairPerSqm))
		case repairPerSqm >= 200:
			score += 10
			reasons = append(reasons, formatMetric("Reparationsfond: %.0f kr/m² (acceptabel).", repairPerSqm))
		default:
			score -= 10
			reasons = append(reasons, formatMetric("Reparationsfond: %.0f kr/m² (låg).", repairPerSqm))
		}
	} else if fin.RepairFund > 0 {
		score += 10
		reasons = append(reasons, "Reparationsfond finns.")
	} else {
		reasons = append(reasons, "Uppgift om reparationsfond saknas.")
	}

	// Planned maintenance mention
	if fin.RenovationsPlanned != "" {
		score += 10
		reasons = append(reasons, "Underhållsplan redovisas.")
	} else {
		score -= 5
		reasons = append(reasons, "Ingen underhållsplan nämnd.")
	}

	// Done renovations bonus
	if fin.RenovationsDone != "" {
		score += 5
		reasons = append(reasons, "Genomförda renoveringar dokumenterade.")
	}

	d.Score = clamp(score, 0, 100)
	d.Description = joinReasons(reasons)
	return d
}

// ── Fee scoring (15%) ──
// avgift per m² benchmarked against Swedish norms.
func scoreFee(fin Financials) ScoreDimension {
	d := ScoreDimension{
		Name:   "Avgiftsnivå",
		Weight: 15,
	}

	feePerSqm := fin.AvgiftPerSqm
	if feePerSqm == 0 && fin.FeePerMonth > 0 && fin.BOATotal > 0 {
		feePerSqm = (fin.FeePerMonth * 12) / fin.BOATotal
	}

	if feePerSqm == 0 {
		d.Score = 50
		d.Description = "Uppgift om avgift saknas."
		return d
	}

	// Swedish norm: 500–800 kr/m²/år is typical.
	// < 400 = very low, > 1000 = high, > 1200 = worrying
	switch {
	case feePerSqm <= 400:
		d.Score = 95
		d.Description = "Mycket låg avgift."
	case feePerSqm <= 600:
		d.Score = 85
		d.Description = "Låg avgift, under snittet."
	case feePerSqm <= 800:
		d.Score = 70
		d.Description = "Normal avgiftsnivå."
	case feePerSqm <= 1000:
		d.Score = 50
		d.Description = "Något hög avgift."
	case feePerSqm <= 1200:
		d.Score = 35
		d.Description = "Hög avgift — risk för att den förblivit hög."
	default:
		d.Score = 15
		d.Description = "Mycket hög avgift — kan indikera ekonomiska problem."
	}

	d.Description += formatMetric(" Avgift: %.0f kr/m²/år.", feePerSqm)
	return d
}

// ── Result scoring (15%) ──
// Net result stability and direction.
func scoreResult(fin Financials) ScoreDimension {
	d := ScoreDimension{
		Name:   "Årsresultat",
		Weight: 15,
	}

	if fin.NetResult == 0 && len(fin.YearlySnapshots) == 0 {
		d.Score = 50
		d.Description = "Uppgift om årsresultat saknas."
		return d
	}

	result := fin.NetResult
	if result == 0 && len(fin.YearlySnapshots) > 0 {
		result = fin.YearlySnapshots[len(fin.YearlySnapshots)-1].NetResult
	}

	// Positive result = good, negative = concerning
	// For BRFs, a slight negative isn't necessarily bad (planned depreciation)
	// but large negatives signal trouble
	if fin.FeeIncome > 0 {
		// Normalize by income
		ratio := result / fin.FeeIncome
		switch {
		case ratio >= 0.05:
			d.Score = 90
			d.Description = "Starkt positivt resultat."
		case ratio >= 0:
			d.Score = 75
			d.Description = "Positivt eller nollresultat."
		case ratio >= -0.05:
			d.Score = 60
			d.Description = "Svagt negativt resultat (inom normal variation)."
		case ratio >= -0.15:
			d.Score = 40
			d.Description = "Negativt resultat — kan kräva avgiftshöjning på sikt."
		default:
			d.Score = 20
			d.Description = "Kraftigt negativt resultat — oroväckande."
		}
	} else {
		// Absolute judgment
		switch {
		case result > 500_000:
			d.Score = 90
			d.Description = "Starkt positivt resultat."
		case result >= 0:
			d.Score = 70
			d.Description = "Positivt resultat."
		case result >= -200_000:
			d.Score = 50
			d.Description = "Svagt negativt resultat."
		case result >= -1_000_000:
			d.Score = 30
			d.Description = "Negativt resultat."
		default:
			d.Score = 15
			d.Description = "Kraftigt negativt resultat."
		}
	}

	d.Description += formatMetric(" Årsresultat: %.0f kr.", result)
	return d
}

// ──────────────────────────────────────────────────────────────────────
// Scoring helpers
// ──────────────────────────────────────────────────────────────────────

func grade(score int) string {
	switch {
	case score >= 80:
		return "A"
	case score >= 65:
		return "B"
	case score >= 50:
		return "C"
	case score >= 35:
		return "D"
	case score >= 20:
		return "E"
	default:
		return "F"
	}
}

func label(score int) string {
	switch {
	case score >= 80:
		return "Stabil och välskött förening"
	case score >= 65:
		return "Bra förening med enstaka noteringar"
	case score >= 50:
		return "Acceptabel förening"
	case score >= 35:
		return "Förening med förbättringsbehov"
	case score >= 20:
		return "Förening med påtagliga risker"
	default:
		return "Förening med allvarliga ekonomiska problem"
	}
}

func clamp(v, lo, hi int) int {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}

func formatMetric(format string, args ...any) string {
	return " " + fmt.Sprintf(format, args...)
}

func joinReasons(reasons []string) string {
	return strings.Join(reasons, " ")
}
