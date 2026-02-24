package brfintel

import "time"

// ──────────────────────────────────────────────────────────────────────
// BRF Intelligence Engine — Domain Types
//
// Structured economic intelligence from BRF årsredovisningar.
// Goes beyond simple extraction: scoring, risk assessment, trend
// analysis and peer comparison.
// ──────────────────────────────────────────────────────────────────────

// BRFReport is the full intelligence output for a single BRF.
type BRFReport struct {
	ID               string            `json:"id"`
	OwnerID          string            `json:"owner_id,omitempty"`
	BRFName          string            `json:"brf_name"`
	OrgNumber        string            `json:"org_number"`
	Municipality     string            `json:"municipality,omitempty"`
	City             string            `json:"city,omitempty"`
	Score            BRFScore          `json:"score"`
	Risks            []RiskWarning     `json:"risks"`
	Trends           EconomicTrend     `json:"trends"`
	BuyerSummary     string            `json:"buyer_summary"`
	LegalView        string            `json:"legal_view"`
	Financials       Financials        `json:"financials"`
	Comparison       *PeerComparison   `json:"comparison,omitempty"`
	SourceYears      []int             `json:"source_years"`
	SourceDocuments  []SourceDocument  `json:"source_documents,omitempty"`
	ListingID        string            `json:"listing_id,omitempty"`
	CreatedAt        time.Time         `json:"created_at"`
	UpdatedAt        time.Time         `json:"updated_at"`
}

// BRFScore is the composite 0–100 quality score with sub-dimensions.
type BRFScore struct {
	Total       int              `json:"total"`        // 0–100 composite
	Grade       string           `json:"grade"`         // A–F
	Label       string           `json:"label"`         // e.g. "Stabil förening"
	Dimensions  []ScoreDimension `json:"dimensions"`
}

// ScoreDimension breaks the score into understandable facets.
type ScoreDimension struct {
	Name        string `json:"name"`         // e.g. "Skuldsättning"
	Score       int    `json:"score"`         // 0–100
	Weight      int    `json:"weight"`        // percentage weight in composite
	Description string `json:"description"`   // human-readable explanation
}

// RiskWarning surfaces a specific concern with severity.
type RiskWarning struct {
	Severity    string `json:"severity"`     // critical | high | medium | low
	Category    string `json:"category"`     // debt | maintenance | fee | liquidity | legal | governance
	Title       string `json:"title"`        // short headline
	Description string `json:"description"`  // detailed explanation
	Metric      string `json:"metric,omitempty"` // the number behind the warning
}

// EconomicTrend captures 3-5 year directional data.
type EconomicTrend struct {
	Direction   string           `json:"direction"`   // improving | stable | declining
	Summary     string           `json:"summary"`     // prose explanation
	DataPoints  []TrendDataPoint `json:"data_points"`
}

// TrendDataPoint is a single year's snapshot of key metrics.
type TrendDataPoint struct {
	Year             int     `json:"year"`
	AvgiftPerSqm     float64 `json:"avgift_per_sqm,omitempty"`     // SEK
	SkuldPerSqm      float64 `json:"skuld_per_sqm,omitempty"`      // SEK
	Arsresultat      float64 `json:"arsresultat,omitempty"`         // SEK (net result)
	Likviditet       float64 `json:"likviditet,omitempty"`           // ratio or SEK
	Reparationsfond  float64 `json:"reparationsfond,omitempty"`     // SEK
	Rantekostnad     float64 `json:"rantekostnad,omitempty"`        // SEK
}

// Financials stores the raw extracted numbers used for analysis.
type Financials struct {
	// Identity
	PropertyDesignation string  `json:"property_designation,omitempty"`
	BuildYear           int     `json:"build_year,omitempty"`
	NumberOfUnits       int     `json:"number_of_units,omitempty"`
	BOATotal            float64 `json:"boa_total,omitempty"`         // m²
	LOATotal            float64 `json:"loa_total,omitempty"`         // m²

	// Revenue
	FeeIncome    float64 `json:"fee_income,omitempty"`     // total årsavgifter
	RentalIncome float64 `json:"rental_income,omitempty"`

	// Costs
	InterestCosts    float64 `json:"interest_costs,omitempty"`
	Depreciation     float64 `json:"depreciation,omitempty"`
	OperatingCosts   float64 `json:"operating_costs,omitempty"`
	MaintenanceCosts float64 `json:"maintenance_costs,omitempty"`

	// Balance sheet
	TotalDebt       float64 `json:"total_debt,omitempty"`
	CashAndBank     float64 `json:"cash_and_bank,omitempty"`
	RepairFund      float64 `json:"repair_fund,omitempty"`
	TotalAssets     float64 `json:"total_assets,omitempty"`

	// Per-unit / Per-sqm
	FeePerMonth     float64 `json:"fee_per_month,omitempty"`     // average
	DebtPerSqm      float64 `json:"debt_per_sqm,omitempty"`
	AvgiftPerSqm    float64 `json:"avgift_per_sqm,omitempty"`

	// Result
	NetResult float64 `json:"net_result,omitempty"`

	// Land
	LandStatus      string `json:"land_status,omitempty"`       // äganderätt | tomträtt
	LandLeaseExpiry string `json:"land_lease_expiry,omitempty"`

	// Energy
	EnergyClass       string  `json:"energy_class,omitempty"`
	EnergyConsumption float64 `json:"energy_consumption,omitempty"` // kWh/m²/år

	// Maintenance
	RenovationsDone    string `json:"renovations_done,omitempty"`
	RenovationsPlanned string `json:"renovations_planned,omitempty"`

	// Multi-year data (for trend analysis)
	YearlySnapshots []YearlySnapshot `json:"yearly_snapshots,omitempty"`
}

// YearlySnapshot captures one fiscal year's numbers for trend analysis.
type YearlySnapshot struct {
	Year             int     `json:"year"`
	FeeIncome        float64 `json:"fee_income,omitempty"`
	InterestCosts    float64 `json:"interest_costs,omitempty"`
	OperatingCosts   float64 `json:"operating_costs,omitempty"`
	MaintenanceCosts float64 `json:"maintenance_costs,omitempty"`
	NetResult        float64 `json:"net_result,omitempty"`
	TotalDebt        float64 `json:"total_debt,omitempty"`
	CashAndBank      float64 `json:"cash_and_bank,omitempty"`
	RepairFund       float64 `json:"repair_fund,omitempty"`
	DebtPerSqm       float64 `json:"debt_per_sqm,omitempty"`
	AvgiftPerSqm     float64 `json:"avgift_per_sqm,omitempty"`
}

// PeerComparison shows how this BRF stacks up against similar ones.
type PeerComparison struct {
	PeerGroupLabel    string          `json:"peer_group_label"`    // e.g. "BRF i Stockholm innerstad, 1960-tal"
	PeerCount         int             `json:"peer_count"`
	Percentile        int             `json:"percentile"`          // 0–100, where this BRF ranks
	MedianDebtPerSqm  float64         `json:"median_debt_per_sqm"`
	MedianFeePerSqm   float64         `json:"median_fee_per_sqm"`
	ComparisonMetrics []CompareMetric `json:"comparison_metrics"`
}

// CompareMetric shows a single metric vs the peer group.
type CompareMetric struct {
	Name       string  `json:"name"`
	ThisBRF    float64 `json:"this_brf"`
	PeerMedian float64 `json:"peer_median"`
	Unit       string  `json:"unit"`
	Better     bool    `json:"better"` // true if this BRF is better than median
}

// SourceDocument records a document used for the analysis.
type SourceDocument struct {
	FileName    string `json:"file_name"`
	FiscalYear  int    `json:"fiscal_year"`
	PageCount   int    `json:"page_count"`
	CharCount   int    `json:"char_count"`
	InputKind   string `json:"input_kind"` // pdf-upload | text-client | listing-import
}

// ────────────────────────────────────────────────────────────────
// Request / Response types for the API layer
// ────────────────────────────────────────────────────────────────

// AnalyzeRequest is the inbound payload for creating a BRF intelligence report.
type AnalyzeRequest struct {
	// Attach to existing listing (optional)
	ListingID string `json:"listing_id,omitempty"`

	// Manual BRF identification
	BRFName      string `json:"brf_name"`
	OrgNumber    string `json:"org_number,omitempty"`
	Municipality string `json:"municipality,omitempty"`
	City         string `json:"city,omitempty"`

	// Structured data (from prior annual report extraction)
	Reports []AnnualReportInput `json:"reports,omitempty"`

	// Raw text if no structured input is given
	RawText  string `json:"raw_text,omitempty"`
	FileName string `json:"file_name,omitempty"`
}

// AnnualReportInput carries extracted data for one fiscal year.
type AnnualReportInput struct {
	FiscalYear int                `json:"fiscal_year"`
	Data       map[string]any     `json:"data"` // flexible key-value from extraction
	FileName   string             `json:"file_name,omitempty"`
	PageCount  int                `json:"page_count,omitempty"`
	CharCount  int                `json:"char_count,omitempty"`
}

// AnalyzeResponse is the API envelope wrapping a BRFReport.
type AnalyzeResponse struct {
	Report BRFReport `json:"report"`
}
