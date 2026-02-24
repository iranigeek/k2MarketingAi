package brfintel

import (
	"context"
	"fmt"
	"math"
	"sort"
	"strings"
	"time"

	"k2MarketingAi/internal/llm"
	"k2MarketingAi/internal/storage"
)

// ──────────────────────────────────────────────────────────────────────
// BRF Intelligence Engine — Core Analyzer
//
// Orchestrates: extraction → financial analysis → scoring → risk
// assessment → trend analysis → LLM summaries → peer comparison.
// ──────────────────────────────────────────────────────────────────────

// Analyzer is the main entry point for BRF intelligence.
type Analyzer struct {
	llm      llm.Client
	store    storage.Store
	brfStore BRFReportStore
}

// NewAnalyzer constructs an Analyzer with its dependencies.
func NewAnalyzer(llmClient llm.Client, store storage.Store) *Analyzer {
	return &Analyzer{
		llm:   llmClient,
		store: store,
	}
}

// SetBRFStore attaches the BRF report store for peer comparison.
func (a *Analyzer) SetBRFStore(s BRFReportStore) {
	a.brfStore = s
}

// Analyze runs the full intelligence pipeline and returns a BRFReport.
func (a *Analyzer) Analyze(ctx context.Context, req AnalyzeRequest, ownerID string) (BRFReport, error) {
	now := time.Now()
	report := BRFReport{
		ID:          fmt.Sprintf("brf_%d", now.UnixMilli()),
		OwnerID:     ownerID,
		BRFName:     req.BRFName,
		OrgNumber:   req.OrgNumber,
		Municipality: req.Municipality,
		City:        req.City,
		ListingID:   req.ListingID,
		CreatedAt:   now,
		UpdatedAt:   now,
	}

	// ── Step 1: Build financials from structured or raw input ──
	financials, sourceYears, sourceDocs, err := a.buildFinancials(ctx, req)
	if err != nil {
		return report, fmt.Errorf("build financials: %w", err)
	}
	report.Financials = financials
	report.SourceYears = sourceYears
	report.SourceDocuments = sourceDocs

	// ── Step 2: Compute BRF Score (0–100) ──
	report.Score = computeScore(financials)

	// ── Step 3: Detect risk warnings ──
	report.Risks = detectRisks(financials)

	// ── Step 4: Compute economic trends ──
	report.Trends = computeTrends(financials)

	// ── Step 5: Generate LLM-powered summaries ──
	if a.llm != nil {
		buyerSummary, err := a.generateBuyerSummary(ctx, report)
		if err == nil {
			report.BuyerSummary = buyerSummary
		}

		legalView, err := a.generateLegalView(ctx, report)
		if err == nil {
			report.LegalView = legalView
		}
	}

	// ── Step 6: Peer comparison (if we have stored reports) ──
	if a.brfStore != nil {
		comparison := a.buildPeerComparison(ctx, report)
		if comparison != nil {
			report.Comparison = comparison
		}
	}

	return report, nil
}

// ──────────────────────────────────────────────────────────────────────
// Step 1 — Build Financials
// ──────────────────────────────────────────────────────────────────────

func (a *Analyzer) buildFinancials(ctx context.Context, req AnalyzeRequest) (Financials, []int, []SourceDocument, error) {
	var fin Financials
	var years []int
	var docs []SourceDocument

	// If we have structured reports from annual report extraction
	if len(req.Reports) > 0 {
		for _, r := range req.Reports {
			snap := parseYearlySnapshot(r.FiscalYear, r.Data)
			fin.YearlySnapshots = append(fin.YearlySnapshots, snap)
			years = append(years, r.FiscalYear)
			docs = append(docs, SourceDocument{
				FileName:   r.FileName,
				FiscalYear: r.FiscalYear,
				PageCount:  r.PageCount,
				CharCount:  r.CharCount,
				InputKind:  "structured",
			})
		}
		// Sort snapshots by year
		sort.Slice(fin.YearlySnapshots, func(i, j int) bool {
			return fin.YearlySnapshots[i].Year < fin.YearlySnapshots[j].Year
		})
		// Use latest year as the primary financials
		if len(fin.YearlySnapshots) > 0 {
			latest := fin.YearlySnapshots[len(fin.YearlySnapshots)-1]
			applySnapshotToFinancials(&fin, latest)
		}
		// Merge identity fields from any report data
		for _, r := range req.Reports {
			mergeIdentityFields(&fin, r.Data)
		}
		return fin, years, docs, nil
	}

	// If we have raw text, use LLM to extract structured data
	if req.RawText != "" && a.llm != nil {
		extracted, err := a.extractFinancialsFromText(ctx, req.RawText)
		if err != nil {
			return fin, nil, nil, fmt.Errorf("extract from text: %w", err)
		}
		fin = extracted
		docs = append(docs, SourceDocument{
			FileName:  req.FileName,
			CharCount: len(req.RawText),
			InputKind: "text-client",
		})
		if len(fin.YearlySnapshots) > 0 {
			for _, s := range fin.YearlySnapshots {
				years = append(years, s.Year)
			}
		}
		return fin, years, docs, nil
	}

	// If attached to a listing, read from existing annual report
	if req.ListingID != "" && a.store != nil {
		listing, err := a.store.GetListing(ctx, req.ListingID)
		if err != nil {
			return fin, nil, nil, fmt.Errorf("get listing: %w", err)
		}
		if ar := listing.Details.Association.AnnualReport; ar != nil {
			fin = financialsFromAnnualReport(ar)
			docs = append(docs, SourceDocument{
				FileName:  ar.FileName,
				PageCount: ar.SourcePageCount,
				CharCount: ar.CharactersAnalysed,
				InputKind: "listing-import",
			})
		}
		// Also merge association-level data
		if listing.Details.Association.DebtPerSquareMeter > 0 {
			fin.DebtPerSqm = float64(listing.Details.Association.DebtPerSquareMeter)
		}
		if listing.Details.Property.FeePerMonth > 0 {
			fin.FeePerMonth = float64(listing.Details.Property.FeePerMonth)
		}
		if listing.Details.Property.LivingArea > 0 {
			fin.BOATotal = listing.Details.Property.LivingArea
		}
		return fin, years, docs, nil
	}

	return fin, nil, nil, fmt.Errorf("ingen data tillgänglig: ange årsredovisningar, råtext eller listing-ID")
}

func parseYearlySnapshot(year int, data map[string]any) YearlySnapshot {
	s := YearlySnapshot{Year: year}
	s.FeeIncome = floatVal(data, "fee_income")
	s.InterestCosts = floatVal(data, "interest_costs")
	s.OperatingCosts = floatVal(data, "operating_costs")
	s.MaintenanceCosts = floatVal(data, "maintenance_costs")
	s.NetResult = floatVal(data, "net_result")
	s.TotalDebt = floatVal(data, "total_debt")
	s.CashAndBank = floatVal(data, "cash_and_bank")
	s.RepairFund = floatVal(data, "repair_fund")
	s.DebtPerSqm = floatVal(data, "debt_per_sqm")
	s.AvgiftPerSqm = floatVal(data, "avgift_per_sqm")
	return s
}

func applySnapshotToFinancials(fin *Financials, snap YearlySnapshot) {
	fin.FeeIncome = snap.FeeIncome
	fin.InterestCosts = snap.InterestCosts
	fin.OperatingCosts = snap.OperatingCosts
	fin.MaintenanceCosts = snap.MaintenanceCosts
	fin.NetResult = snap.NetResult
	fin.TotalDebt = snap.TotalDebt
	fin.CashAndBank = snap.CashAndBank
	fin.RepairFund = snap.RepairFund
	fin.DebtPerSqm = snap.DebtPerSqm
	fin.AvgiftPerSqm = snap.AvgiftPerSqm
}

func mergeIdentityFields(fin *Financials, data map[string]any) {
	if v := strVal(data, "property_designation"); v != "" && fin.PropertyDesignation == "" {
		fin.PropertyDesignation = v
	}
	if v := intVal(data, "build_year"); v > 0 && fin.BuildYear == 0 {
		fin.BuildYear = v
	}
	if v := intVal(data, "number_of_units"); v > 0 && fin.NumberOfUnits == 0 {
		fin.NumberOfUnits = v
	}
	if v := floatVal(data, "boa_total"); v > 0 && fin.BOATotal == 0 {
		fin.BOATotal = v
	}
	if v := floatVal(data, "loa_total"); v > 0 && fin.LOATotal == 0 {
		fin.LOATotal = v
	}
	if v := strVal(data, "land_status"); v != "" && fin.LandStatus == "" {
		fin.LandStatus = v
	}
	if v := strVal(data, "land_lease_expiry"); v != "" && fin.LandLeaseExpiry == "" {
		fin.LandLeaseExpiry = v
	}
	if v := strVal(data, "energy_class"); v != "" && fin.EnergyClass == "" {
		fin.EnergyClass = v
	}
	if v := floatVal(data, "energy_consumption"); v > 0 && fin.EnergyConsumption == 0 {
		fin.EnergyConsumption = v
	}
	if v := strVal(data, "renovations_done"); v != "" && fin.RenovationsDone == "" {
		fin.RenovationsDone = v
	}
	if v := strVal(data, "renovations_planned"); v != "" && fin.RenovationsPlanned == "" {
		fin.RenovationsPlanned = v
	}
}

func financialsFromAnnualReport(ar *storage.AnnualReportSummary) Financials {
	fin := Financials{
		PropertyDesignation: ar.PropertyDesignation,
		LandStatus:          ar.LandStatus,
		LandLeaseExpiry:     ar.LandLeaseExpiry,
		EnergyClass:         ar.EnergyClass,
		RenovationsDone:     ar.RenovationsDone,
		RenovationsPlanned:  ar.RenovationsPlanned,
	}
	fin.BuildYear = parseIntFromString(ar.BuildYear)
	fin.FeeIncome = parseFloatFromString(ar.FeeIncome)
	fin.RentalIncome = parseFloatFromString(ar.RentalIncome)
	fin.InterestCosts = parseFloatFromString(ar.InterestCosts)
	fin.Depreciation = parseFloatFromString(ar.Depreciation)
	fin.TotalDebt = parseFloatFromString(ar.TotalDebt)
	fin.CashAndBank = parseFloatFromString(ar.CashAndBank)
	fin.NetResult = parseFloatFromString(ar.NetResult)
	fin.DebtPerSqm = parseFloatFromString(ar.DebtPerSqm)
	fin.FeePerMonth = parseFloatFromString(ar.FeePerMonth)
	fin.BOATotal = parseFloatFromString(ar.BoaTotal)
	fin.LOATotal = parseFloatFromString(ar.LoaTotal)
	fin.EnergyConsumption = parseFloatFromString(ar.EnergyConsumption)
	return fin
}

// ──────────────────────────────────────────────────────────────────────
// Helper parsers
// ──────────────────────────────────────────────────────────────────────

func floatVal(m map[string]any, key string) float64 {
	v, ok := m[key]
	if !ok {
		return 0
	}
	switch t := v.(type) {
	case float64:
		return t
	case int:
		return float64(t)
	case string:
		return parseFloatFromString(t)
	}
	return 0
}

func intVal(m map[string]any, key string) int {
	v, ok := m[key]
	if !ok {
		return 0
	}
	switch t := v.(type) {
	case float64:
		return int(t)
	case int:
		return t
	case string:
		return parseIntFromString(t)
	}
	return 0
}

func strVal(m map[string]any, key string) string {
	v, ok := m[key]
	if !ok {
		return ""
	}
	s, _ := v.(string)
	return strings.TrimSpace(s)
}

func parseFloatFromString(s string) float64 {
	clean := strings.TrimSpace(s)
	if clean == "" || strings.EqualFold(clean, "okänd") {
		return 0
	}
	// Remove non-numeric chars except dot, minus and comma
	var buf strings.Builder
	for _, r := range clean {
		if r >= '0' && r <= '9' || r == '-' || r == '.' || r == ',' {
			buf.WriteRune(r)
		}
	}
	numStr := buf.String()
	// Swedish: comma as decimal separator
	numStr = strings.ReplaceAll(numStr, ",", ".")
	var val float64
	fmt.Sscanf(numStr, "%f", &val)
	return val
}

func parseIntFromString(s string) int {
	return int(parseFloatFromString(s))
}

// abs returns the absolute value of a float64.
func abs(v float64) float64 {
	return math.Abs(v)
}
