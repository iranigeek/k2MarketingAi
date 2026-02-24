package brfintel

import (
	"context"
	"math"
	"sort"
)

// ──────────────────────────────────────────────────────────────────────
// Peer Comparison Engine
//
// Compares this BRF against all previously analysed BRF reports
// stored in the system. Groups by city/municipality + building era.
// ──────────────────────────────────────────────────────────────────────

func (a *Analyzer) buildPeerComparison(ctx context.Context, report BRFReport) *PeerComparison {
	peers := a.findPeers(ctx, report)
	if len(peers) < 2 {
		return nil
	}

	peerLabel := buildPeerGroupLabel(report)

	// Collect metrics from peers
	var peerDebts, peerFees, peerResults, peerLiq []float64
	for _, p := range peers {
		if p.Financials.DebtPerSqm > 0 {
			peerDebts = append(peerDebts, p.Financials.DebtPerSqm)
		}
		if p.Financials.AvgiftPerSqm > 0 {
			peerFees = append(peerFees, p.Financials.AvgiftPerSqm)
		}
		if p.Financials.NetResult != 0 {
			peerResults = append(peerResults, p.Financials.NetResult)
		}
		if p.Financials.CashAndBank > 0 {
			peerLiq = append(peerLiq, p.Financials.CashAndBank)
		}
	}

	comp := &PeerComparison{
		PeerGroupLabel:   peerLabel,
		PeerCount:        len(peers),
		MedianDebtPerSqm: median(peerDebts),
		MedianFeePerSqm:  median(peerFees),
	}

	// Build comparison metrics
	if len(peerDebts) > 0 && report.Financials.DebtPerSqm > 0 {
		medD := median(peerDebts)
		comp.ComparisonMetrics = append(comp.ComparisonMetrics, CompareMetric{
			Name:       "Skuld per m²",
			ThisBRF:    report.Financials.DebtPerSqm,
			PeerMedian: medD,
			Unit:       "kr/m²",
			Better:     report.Financials.DebtPerSqm < medD,
		})
	}

	if len(peerFees) > 0 && report.Financials.AvgiftPerSqm > 0 {
		medF := median(peerFees)
		comp.ComparisonMetrics = append(comp.ComparisonMetrics, CompareMetric{
			Name:       "Avgift per m²/år",
			ThisBRF:    report.Financials.AvgiftPerSqm,
			PeerMedian: medF,
			Unit:       "kr/m²/år",
			Better:     report.Financials.AvgiftPerSqm < medF,
		})
	}

	if len(peerResults) > 0 && report.Financials.NetResult != 0 {
		medR := median(peerResults)
		comp.ComparisonMetrics = append(comp.ComparisonMetrics, CompareMetric{
			Name:       "Årsresultat",
			ThisBRF:    report.Financials.NetResult,
			PeerMedian: medR,
			Unit:       "kr",
			Better:     report.Financials.NetResult > medR,
		})
	}

	if len(peerLiq) > 0 && report.Financials.CashAndBank > 0 {
		medL := median(peerLiq)
		comp.ComparisonMetrics = append(comp.ComparisonMetrics, CompareMetric{
			Name:       "Likvida medel",
			ThisBRF:    report.Financials.CashAndBank,
			PeerMedian: medL,
			Unit:       "kr",
			Better:     report.Financials.CashAndBank > medL,
		})
	}

	// Calculate percentile for debt (lower = better)
	if len(peerDebts) > 0 && report.Financials.DebtPerSqm > 0 {
		sort.Float64s(peerDebts)
		betterCount := 0
		for _, d := range peerDebts {
			if report.Financials.DebtPerSqm <= d {
				betterCount++
			}
		}
		comp.Percentile = int(math.Round(float64(betterCount) / float64(len(peerDebts)) * 100))
	}

	return comp
}

// findPeers retrieves stored BRF reports that match the same region/era.
func (a *Analyzer) findPeers(ctx context.Context, report BRFReport) []BRFReport {
	if a.brfStore == nil {
		return nil
	}

	all, err := a.brfStore.ListBRFReports(ctx, report.OwnerID)
	if err != nil {
		return nil
	}

	var peers []BRFReport
	for _, p := range all {
		if p.ID == report.ID {
			continue
		}
		if matchesPeerGroup(report, p) {
			peers = append(peers, p)
		}
	}
	return peers
}

// matchesPeerGroup determines if two BRF reports belong to the same comparison group.
func matchesPeerGroup(a, b BRFReport) bool {
	// Same city/municipality
	if a.City != "" && b.City != "" && a.City != b.City {
		return false
	}
	if a.Municipality != "" && b.Municipality != "" && a.Municipality != b.Municipality {
		return false
	}

	// Similar building era (within 20 years)
	if a.Financials.BuildYear > 0 && b.Financials.BuildYear > 0 {
		diff := a.Financials.BuildYear - b.Financials.BuildYear
		if diff < 0 {
			diff = -diff
		}
		if diff > 20 {
			return false
		}
	}

	return true
}

func buildPeerGroupLabel(report BRFReport) string {
	label := "BRF"
	if report.City != "" {
		label += " i " + report.City
	} else if report.Municipality != "" {
		label += " i " + report.Municipality
	}
	if report.Financials.BuildYear > 0 {
		decade := (report.Financials.BuildYear / 10) * 10
		label += ", " + itoa(decade) + "-tal"
	}
	return label
}

func median(vals []float64) float64 {
	if len(vals) == 0 {
		return 0
	}
	sorted := make([]float64, len(vals))
	copy(sorted, vals)
	sort.Float64s(sorted)
	n := len(sorted)
	if n%2 == 0 {
		return (sorted[n/2-1] + sorted[n/2]) / 2
	}
	return sorted[n/2]
}

func itoa(v int) string {
	// Small helper to avoid importing strconv for a single use
	if v == 0 {
		return "0"
	}
	s := ""
	neg := false
	if v < 0 {
		neg = true
		v = -v
	}
	for v > 0 {
		s = string(rune('0'+v%10)) + s
		v /= 10
	}
	if neg {
		s = "-" + s
	}
	return s
}
