package brfintel

import (
	"fmt"
	"math"
	"strings"
)

// ──────────────────────────────────────────────────────────────────────
// Risk Detection Engine
//
// Scans financials for specific warning signals and returns
// categorised, severity-ranked risk warnings.
// ──────────────────────────────────────────────────────────────────────

func detectRisks(fin Financials) []RiskWarning {
	var risks []RiskWarning

	risks = append(risks, checkDebtRisk(fin)...)
	risks = append(risks, checkLiquidityRisk(fin)...)
	risks = append(risks, checkFeeRisk(fin)...)
	risks = append(risks, checkMaintenanceRisk(fin)...)
	risks = append(risks, checkLandRisk(fin)...)
	risks = append(risks, checkResultRisk(fin)...)
	risks = append(risks, checkInterestRisk(fin)...)

	// Sort by severity (critical first)
	severityOrder := map[string]int{"critical": 0, "high": 1, "medium": 2, "low": 3}
	for i := range risks {
		for j := i + 1; j < len(risks); j++ {
			if severityOrder[risks[j].Severity] < severityOrder[risks[i].Severity] {
				risks[i], risks[j] = risks[j], risks[i]
			}
		}
	}

	return risks
}

func checkDebtRisk(fin Financials) []RiskWarning {
	var risks []RiskWarning

	debtPerSqm := fin.DebtPerSqm
	if debtPerSqm == 0 && fin.TotalDebt > 0 && fin.BOATotal > 0 {
		debtPerSqm = fin.TotalDebt / fin.BOATotal
	}

	if debtPerSqm > 15000 {
		risks = append(risks, RiskWarning{
			Severity:    "critical",
			Category:    "debt",
			Title:       "Extremt hög skuldsättning",
			Description: "Föreningens skuld per kvadratmeter överstiger 15 000 kr, vilket är långt över det normala. Detta innebär hög risk för kraftiga avgiftshöjningar, särskilt vid ränteuppgång.",
			Metric:      fmt.Sprintf("%.0f kr/m²", debtPerSqm),
		})
	} else if debtPerSqm > 10000 {
		risks = append(risks, RiskWarning{
			Severity:    "high",
			Category:    "debt",
			Title:       "Hög skuldsättning",
			Description: "Skulden per kvadratmeter är över 10 000 kr. Föreningen kan behöva höja avgifterna, särskilt om lån ska omsättas till högre ränta.",
			Metric:      fmt.Sprintf("%.0f kr/m²", debtPerSqm),
		})
	} else if debtPerSqm > 7000 {
		risks = append(risks, RiskWarning{
			Severity:    "medium",
			Category:    "debt",
			Title:       "Skuldsättningen är ovanför genomsnittet",
			Description: "Skulden per kvadratmeter är högre än snittet för svenska bostadsrättsföreningar.",
			Metric:      fmt.Sprintf("%.0f kr/m²", debtPerSqm),
		})
	}

	// Check for rising debt trend
	if len(fin.YearlySnapshots) >= 2 {
		first := fin.YearlySnapshots[0]
		last := fin.YearlySnapshots[len(fin.YearlySnapshots)-1]
		if first.TotalDebt > 0 && last.TotalDebt > 0 {
			change := (last.TotalDebt - first.TotalDebt) / first.TotalDebt
			if change > 0.15 {
				risks = append(risks, RiskWarning{
					Severity:    "high",
					Category:    "debt",
					Title:       "Skulden ökar",
					Description: fmt.Sprintf("Föreningens skuld har ökat med %.0f%% under perioden %d–%d.", change*100, first.Year, last.Year),
					Metric:      fmt.Sprintf("+%.0f%%", change*100),
				})
			}
		}
	}

	return risks
}

func checkLiquidityRisk(fin Financials) []RiskWarning {
	var risks []RiskWarning

	if fin.CashAndBank <= 0 {
		return risks
	}

	if fin.OperatingCosts > 0 {
		monthsRunway := (fin.CashAndBank / fin.OperatingCosts) * 12
		if monthsRunway < 1 {
			risks = append(risks, RiskWarning{
				Severity:    "critical",
				Category:    "liquidity",
				Title:       "Akut likviditetsbrist",
				Description: "Föreningens likvida medel räcker inte ens till en månads drift. Risk för akuta insatser eller nödlån.",
				Metric:      fmt.Sprintf("%.1f månaders drift", monthsRunway),
			})
		} else if monthsRunway < 3 {
			risks = append(risks, RiskWarning{
				Severity:    "high",
				Category:    "liquidity",
				Title:       "Låg likviditet",
				Description: "Föreningen har mindre än tre månaders driftkostnader i kassa.",
				Metric:      fmt.Sprintf("%.1f månaders drift", monthsRunway),
			})
		}
	}

	return risks
}

func checkFeeRisk(fin Financials) []RiskWarning {
	var risks []RiskWarning

	feePerSqm := fin.AvgiftPerSqm
	if feePerSqm == 0 && fin.FeePerMonth > 0 && fin.BOATotal > 0 {
		feePerSqm = (fin.FeePerMonth * 12) / fin.BOATotal
	}

	if feePerSqm > 1200 {
		risks = append(risks, RiskWarning{
			Severity:    "high",
			Category:    "fee",
			Title:       "Mycket hög avgift",
			Description: "Avgiften per kvadratmeter och år är betydligt högre än snittet. Det kan finnas underliggande ekonomiska problem.",
			Metric:      fmt.Sprintf("%.0f kr/m²/år", feePerSqm),
		})
	} else if feePerSqm > 1000 {
		risks = append(risks, RiskWarning{
			Severity:    "medium",
			Category:    "fee",
			Title:       "Hög avgift",
			Description: "Avgiftsnivån är högre än genomsnittet, men inte alarmerande.",
			Metric:      fmt.Sprintf("%.0f kr/m²/år", feePerSqm),
		})
	}

	// Check for rapid fee increases
	if len(fin.YearlySnapshots) >= 2 {
		first := fin.YearlySnapshots[0]
		last := fin.YearlySnapshots[len(fin.YearlySnapshots)-1]
		if first.AvgiftPerSqm > 0 && last.AvgiftPerSqm > 0 {
			yearsSpan := float64(last.Year - first.Year)
			if yearsSpan > 0 {
				annualGrowth := math.Pow(last.AvgiftPerSqm/first.AvgiftPerSqm, 1/yearsSpan) - 1
				if annualGrowth > 0.05 {
					risks = append(risks, RiskWarning{
						Severity:    "medium",
						Category:    "fee",
						Title:       "Avgifterna stiger snabbt",
						Description: fmt.Sprintf("Avgifterna har ökat med i snitt %.1f%% per år under %d–%d.", annualGrowth*100, first.Year, last.Year),
						Metric:      fmt.Sprintf("%.1f%%/år", annualGrowth*100),
					})
				}
			}
		}
	}

	return risks
}

func checkMaintenanceRisk(fin Financials) []RiskWarning {
	var risks []RiskWarning

	if fin.RepairFund > 0 && fin.BOATotal > 0 {
		repairPerSqm := fin.RepairFund / fin.BOATotal
		if repairPerSqm < 100 {
			risks = append(risks, RiskWarning{
				Severity:    "high",
				Category:    "maintenance",
				Title:       "Underdimensionerad reparationsfond",
				Description: fmt.Sprintf("Reparationsfonden uppgår till %.0f kr/m², vilket är lågt. Kan innebära oväntade extra uttaxeringar vid behov av reparationer.", repairPerSqm),
				Metric:      fmt.Sprintf("%.0f kr/m²", repairPerSqm),
			})
		}
	}

	if fin.RenovationsPlanned == "" && fin.BuildYear > 0 && fin.BuildYear < 1980 {
		risks = append(risks, RiskWarning{
			Severity:    "medium",
			Category:    "maintenance",
			Title:       "Äldre fastighet utan redovisad underhållsplan",
			Description: fmt.Sprintf("Fastigheten är byggd %d och ingen kommande underhållsplan nämns. Äldre fastigheter kräver ofta stambyte, fönsterbyte eller takarbeten.", fin.BuildYear),
			Metric:      fmt.Sprintf("Byggår %d", fin.BuildYear),
		})
	}

	return risks
}

func checkLandRisk(fin Financials) []RiskWarning {
	var risks []RiskWarning

	status := strings.ToLower(fin.LandStatus)
	if strings.Contains(status, "tomträtt") || strings.Contains(status, "arrende") {
		sev := "medium"
		desc := "Föreningen äger inte marken utan har tomträtt. Vid omförhandling kan avgälden öka kraftigt."
		if fin.LandLeaseExpiry != "" {
			desc += " Upplåtelse utgår: " + fin.LandLeaseExpiry + "."
			sev = "high" // more urgent when expiry is known
		}
		risks = append(risks, RiskWarning{
			Severity:    sev,
			Category:    "legal",
			Title:       "Tomträtt — marken ägs ej",
			Description: desc,
			Metric:      fin.LandLeaseExpiry,
		})
	}

	return risks
}

func checkResultRisk(fin Financials) []RiskWarning {
	var risks []RiskWarning

	result := fin.NetResult
	if result == 0 && len(fin.YearlySnapshots) > 0 {
		result = fin.YearlySnapshots[len(fin.YearlySnapshots)-1].NetResult
	}

	if result < 0 {
		if fin.FeeIncome > 0 {
			ratio := result / fin.FeeIncome
			if ratio < -0.15 {
				risks = append(risks, RiskWarning{
					Severity:    "high",
					Category:    "governance",
					Title:       "Kraftigt negativt årsresultat",
					Description: fmt.Sprintf("Årsresultatet motsvarar %.0f%% av avgiftsintäkterna. Föreningen förbrukar sin ekonomiska buffert snabbt.", math.Abs(ratio)*100),
					Metric:      fmt.Sprintf("%.0f kr (%.0f%% av intäkter)", result, math.Abs(ratio)*100),
				})
			}
		} else if result < -500_000 {
			risks = append(risks, RiskWarning{
				Severity:    "high",
				Category:    "governance",
				Title:       "Negativt årsresultat",
				Description: "Föreningen redovisar ett betydande negativt resultat.",
				Metric:      fmt.Sprintf("%.0f kr", result),
			})
		}
	}

	// Check for consecutive negative results
	if len(fin.YearlySnapshots) >= 3 {
		negCount := 0
		for _, s := range fin.YearlySnapshots {
			if s.NetResult < 0 {
				negCount++
			}
		}
		if negCount == len(fin.YearlySnapshots) && negCount >= 3 {
			risks = append(risks, RiskWarning{
				Severity:    "high",
				Category:    "governance",
				Title:       "Kroniskt negativt resultat",
				Description: fmt.Sprintf("Alla %d redovisade år visar negativt resultat. Avgifterna täcker sannolikt inte kostnaderna.", negCount),
				Metric:      fmt.Sprintf("%d år med förlust", negCount),
			})
		}
	}

	return risks
}

func checkInterestRisk(fin Financials) []RiskWarning {
	var risks []RiskWarning

	if fin.InterestCosts > 0 && fin.FeeIncome > 0 {
		interestRatio := fin.InterestCosts / fin.FeeIncome
		if interestRatio > 0.40 {
			risks = append(risks, RiskWarning{
				Severity:    "critical",
				Category:    "debt",
				Title:       "Räntor upptar stor del av intäkterna",
				Description: fmt.Sprintf("Räntekostnaderna motsvarar %.0f%% av avgiftsintäkterna. Vid ränteuppgång riskerar föreningen allvarligt underskott.", interestRatio*100),
				Metric:      fmt.Sprintf("%.0f%% av avgiftsintäkter", interestRatio*100),
			})
		} else if interestRatio > 0.25 {
			risks = append(risks, RiskWarning{
				Severity:    "medium",
				Category:    "debt",
				Title:       "Betydande räntekostnader",
				Description: fmt.Sprintf("Räntekostnaderna motsvarar %.0f%% av avgiftsintäkterna. Känsligt vid ränteförändringar.", interestRatio*100),
				Metric:      fmt.Sprintf("%.0f%% av avgiftsintäkter", interestRatio*100),
			})
		}
	}

	return risks
}

// ──────────────────────────────────────────────────────────────────────
// Trend Analysis — 3-5 year economic direction
// ──────────────────────────────────────────────────────────────────────

func computeTrends(fin Financials) EconomicTrend {
	trend := EconomicTrend{
		Direction: "stable",
		Summary:   "Otillräckliga data för trendanalys.",
	}

	if len(fin.YearlySnapshots) < 2 {
		// Build a single data point from current financials if available
		if fin.DebtPerSqm > 0 || fin.NetResult != 0 {
			trend.DataPoints = []TrendDataPoint{{
				SkuldPerSqm:  fin.DebtPerSqm,
				AvgiftPerSqm: fin.AvgiftPerSqm,
				Arsresultat:  fin.NetResult,
				Likviditet:   fin.CashAndBank,
			}}
		}
		return trend
	}

	// Build data points
	for _, s := range fin.YearlySnapshots {
		trend.DataPoints = append(trend.DataPoints, TrendDataPoint{
			Year:            s.Year,
			AvgiftPerSqm:    s.AvgiftPerSqm,
			SkuldPerSqm:     s.DebtPerSqm,
			Arsresultat:     s.NetResult,
			Likviditet:      s.CashAndBank,
			Reparationsfond: s.RepairFund,
			Rantekostnad:    s.InterestCosts,
		})
	}

	// Evaluate direction based on key signals
	signals := 0 // positive = improving, negative = declining

	first := fin.YearlySnapshots[0]
	last := fin.YearlySnapshots[len(fin.YearlySnapshots)-1]

	// Debt direction
	if first.TotalDebt > 0 && last.TotalDebt > 0 {
		debtChange := (last.TotalDebt - first.TotalDebt) / first.TotalDebt
		if debtChange < -0.05 {
			signals += 2 // debt reducing is good
		} else if debtChange > 0.10 {
			signals -= 2
		}
	}
	if first.DebtPerSqm > 0 && last.DebtPerSqm > 0 {
		if last.DebtPerSqm < first.DebtPerSqm*0.95 {
			signals++
		} else if last.DebtPerSqm > first.DebtPerSqm*1.10 {
			signals--
		}
	}

	// Result direction
	if last.NetResult > first.NetResult+100_000 {
		signals++
	} else if last.NetResult < first.NetResult-200_000 {
		signals--
	}

	// Cash direction
	if first.CashAndBank > 0 && last.CashAndBank > 0 {
		if last.CashAndBank > first.CashAndBank*1.20 {
			signals++
		} else if last.CashAndBank < first.CashAndBank*0.50 {
			signals -= 2
		}
	}

	// Interest costs direction
	if first.InterestCosts > 0 && last.InterestCosts > 0 {
		if last.InterestCosts < first.InterestCosts*0.90 {
			signals++
		} else if last.InterestCosts > first.InterestCosts*1.30 {
			signals--
		}
	}

	switch {
	case signals >= 3:
		trend.Direction = "improving"
		trend.Summary = fmt.Sprintf("Föreningens ekonomi visar en positiv trend under %d–%d. Skuldsättning och/eller kostnader minskar.", first.Year, last.Year)
	case signals <= -3:
		trend.Direction = "declining"
		trend.Summary = fmt.Sprintf("Föreningens ekonomi försämras under %d–%d. Var uppmärksam på ökande skulder, sjunkande likviditet eller stigande kostnader.", first.Year, last.Year)
	case signals >= 1:
		trend.Direction = "improving"
		trend.Summary = fmt.Sprintf("Ekonomin visar svagt förbättrad trend %d–%d.", first.Year, last.Year)
	case signals <= -1:
		trend.Direction = "declining"
		trend.Summary = fmt.Sprintf("Ekonomin visar svagt försämrad trend %d–%d.", first.Year, last.Year)
	default:
		trend.Direction = "stable"
		trend.Summary = fmt.Sprintf("Ekonomin är stabil under perioden %d–%d.", first.Year, last.Year)
	}

	return trend
}
