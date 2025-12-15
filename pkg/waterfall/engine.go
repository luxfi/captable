package waterfall

import (
	"fmt"
	"sort"
)

// Calculate runs a waterfall distribution of exit proceeds across share classes.
//
// The waterfall has four tiers:
//   1. Senior liquidation preferences (paid in seniority order, pro-rata within tier if insufficient)
//   2. Participating preferred pro-rata share of remainder (capped if ParticipationCap > 0)
//   3. Non-participating preferred as-converted comparison (converts if better than liq pref)
//   4. Common stock receives remainder
func Calculate(scenario ExitScenario, classes []ShareClassInput) (*WaterfallResult, error) {
	if scenario.TotalProceeds < 0 {
		return nil, fmt.Errorf("total_proceeds must be non-negative")
	}
	if len(classes) == 0 {
		return nil, fmt.Errorf("at least one share class is required")
	}

	// Normalize conversion ratios: 0 means 1:1.
	for i := range classes {
		if classes[i].ConversionRatio <= 0 {
			classes[i].ConversionRatio = 1.0
		}
	}

	// Separate preferred (seniority > 0) from common (seniority == 0).
	var preferred []ShareClassInput
	var common []ShareClassInput
	for _, c := range classes {
		if c.Seniority > 0 {
			preferred = append(preferred, c)
		} else {
			common = append(common, c)
		}
	}

	// Sort preferred by seniority descending (most senior first).
	sort.Slice(preferred, func(i, j int) bool {
		return preferred[i].Seniority > preferred[j].Seniority
	})

	result := &WaterfallResult{}
	remaining := scenario.TotalProceeds

	// Track cumulative payouts per class across all tiers (for return multiple calc).
	cumulative := make(map[string]float64)

	// ── Tier 1: Senior liquidation preferences ──
	tier1 := TierResult{
		Name:          "Liquidation Preferences",
		Distributions: make(map[string]ClassDistribution),
	}

	seniorityGroups := groupBySeniority(preferred)
	for _, group := range seniorityGroups {
		totalLiqPref := 0.0
		for _, c := range group {
			totalLiqPref += c.InvestmentAmount * c.LiquidationMultiple
		}

		if remaining >= totalLiqPref {
			for _, c := range group {
				payout := c.InvestmentAmount * c.LiquidationMultiple
				remaining -= payout
				cumulative[c.Name] += payout
				tier1.Distributions[c.Name] = ClassDistribution{
					LiquidationPayout: payout,
					TotalPayout:       payout,
					PerSharePayout:    safeDivide(payout, float64(c.SharesOutstanding)),
					ReturnMultiple:    safeDivide(cumulative[c.Name], c.InvestmentAmount),
				}
			}
		} else {
			// Pro-rata within this seniority group.
			for _, c := range group {
				share := safeDivide(c.InvestmentAmount*c.LiquidationMultiple, totalLiqPref)
				payout := remaining * share
				cumulative[c.Name] += payout
				tier1.Distributions[c.Name] = ClassDistribution{
					LiquidationPayout: payout,
					TotalPayout:       payout,
					PerSharePayout:    safeDivide(payout, float64(c.SharesOutstanding)),
					ReturnMultiple:    safeDivide(cumulative[c.Name], c.InvestmentAmount),
				}
			}
			remaining = 0
		}
	}
	result.Tiers = append(result.Tiers, tier1)

	// ── Tier 2: Participating preferred pro-rata ──
	tier2 := TierResult{
		Name:          "Participation",
		Distributions: make(map[string]ClassDistribution),
	}

	if remaining > 0 {
		totalAsConverted := totalAsConvertedShares(preferred, common)
		participatingClasses := filterParticipating(preferred)
		for _, c := range participatingClasses {
			asConvertedShares := float64(c.SharesOutstanding) * c.ConversionRatio
			proRataShare := safeDivide(asConvertedShares, totalAsConverted)
			payout := remaining * proRataShare

			// Apply participation cap if set.
			if c.ParticipationCap > 0 {
				maxTotal := c.InvestmentAmount * c.ParticipationCap
				alreadyPaid := cumulative[c.Name]
				maxParticipation := maxTotal - alreadyPaid
				if maxParticipation < 0 {
					maxParticipation = 0
				}
				if payout > maxParticipation {
					payout = maxParticipation
				}
			}

			cumulative[c.Name] += payout
			tier2.Distributions[c.Name] = ClassDistribution{
				ParticipationPayout: payout,
				TotalPayout:         payout,
				PerSharePayout:      safeDivide(payout, float64(c.SharesOutstanding)),
				ReturnMultiple:      safeDivide(cumulative[c.Name], c.InvestmentAmount),
			}
		}

		for _, d := range tier2.Distributions {
			remaining -= d.ParticipationPayout
		}
	}
	result.Tiers = append(result.Tiers, tier2)

	// ── Tier 3: Non-participating preferred as-converted comparison ──
	//
	// For each non-participating preferred class, compare:
	//   (a) Keep liquidation preference (already received in tier 1)
	//   (b) Convert to common and share pro-rata in the pool of
	//       (remaining + returned liq prefs from all converters)
	//
	// We evaluate all non-participating preferred at once. If converting yields
	// more than the liq pref, the class converts: its tier 1 entry is zeroed,
	// the liq pref returns to the pool, and it receives a conversion payout.
	tier3 := TierResult{
		Name:          "As-Converted Comparison",
		Distributions: make(map[string]ClassDistribution),
	}

	if remaining > 0 {
		nonParticipating := filterNonParticipating(preferred)

		// Build the hypothetical as-converted pool: remaining + all NP liq prefs returned.
		asConvertedPool := remaining
		for _, c := range nonParticipating {
			asConvertedPool += cumulative[c.Name]
		}

		// Total shares in the as-converted pool: common + all NP converted.
		totalSharesInPool := totalCommonShares(common)
		for _, c := range nonParticipating {
			totalSharesInPool += float64(c.SharesOutstanding) * c.ConversionRatio
		}

		// For each NP class, decide: convert or keep liq pref.
		for _, c := range nonParticipating {
			liqPayout := cumulative[c.Name]
			asConvertedShares := float64(c.SharesOutstanding) * c.ConversionRatio
			conversionPayout := safeDivide(asConvertedShares, totalSharesInPool) * asConvertedPool

			if conversionPayout > liqPayout {
				// Convert: zero out tier 1, record conversion in tier 3.
				remaining += liqPayout // return liq pref to pool
				remaining -= conversionPayout
				cumulative[c.Name] = conversionPayout

				if d, ok := tier1.Distributions[c.Name]; ok {
					d.LiquidationPayout = 0
					d.TotalPayout = 0
					d.PerSharePayout = 0
					d.ReturnMultiple = 0
					tier1.Distributions[c.Name] = d
				}

				tier3.Distributions[c.Name] = ClassDistribution{
					ConversionPayout: conversionPayout,
					TotalPayout:      conversionPayout,
					PerSharePayout:   safeDivide(conversionPayout, float64(c.SharesOutstanding)),
					ReturnMultiple:   safeDivide(conversionPayout, c.InvestmentAmount),
				}
			}
		}
	}
	result.Tiers = append(result.Tiers, tier3)

	// ── Tier 4: Common stock gets remainder ──
	tier4 := TierResult{
		Name:          "Common Distribution",
		Distributions: make(map[string]ClassDistribution),
	}

	if remaining > 0 {
		totalCommon := totalCommonShares(common)
		if totalCommon > 0 {
			for _, c := range common {
				share := safeDivide(float64(c.SharesOutstanding), totalCommon)
				payout := remaining * share
				cumulative[c.Name] += payout
				tier4.Distributions[c.Name] = ClassDistribution{
					TotalPayout:    payout,
					PerSharePayout: safeDivide(payout, float64(c.SharesOutstanding)),
					ReturnMultiple: safeDivide(cumulative[c.Name], c.InvestmentAmount),
				}
			}
			remaining = 0
		}
	}
	result.Tiers = append(result.Tiers, tier4)

	result.TotalDistributed = scenario.TotalProceeds - remaining
	result.Remainder = remaining
	return result, nil
}

// groupBySeniority groups classes by their seniority level, sorted descending.
func groupBySeniority(classes []ShareClassInput) [][]ShareClassInput {
	if len(classes) == 0 {
		return nil
	}

	var groups [][]ShareClassInput
	var current []ShareClassInput
	currentSeniority := -1

	for _, c := range classes {
		if c.Seniority != currentSeniority {
			if len(current) > 0 {
				groups = append(groups, current)
			}
			current = []ShareClassInput{c}
			currentSeniority = c.Seniority
		} else {
			current = append(current, c)
		}
	}
	if len(current) > 0 {
		groups = append(groups, current)
	}
	return groups
}

// filterParticipating returns preferred classes with Participating == true.
func filterParticipating(classes []ShareClassInput) []ShareClassInput {
	var out []ShareClassInput
	for _, c := range classes {
		if c.Participating {
			out = append(out, c)
		}
	}
	return out
}

// filterNonParticipating returns preferred classes with Participating == false.
func filterNonParticipating(classes []ShareClassInput) []ShareClassInput {
	var out []ShareClassInput
	for _, c := range classes {
		if !c.Participating {
			out = append(out, c)
		}
	}
	return out
}

// totalCommonShares sums shares outstanding across common classes.
func totalCommonShares(common []ShareClassInput) float64 {
	var total float64
	for _, c := range common {
		total += float64(c.SharesOutstanding)
	}
	return total
}

// totalAsConvertedShares computes total shares on as-converted basis.
func totalAsConvertedShares(preferred, common []ShareClassInput) float64 {
	total := totalCommonShares(common)
	for _, c := range preferred {
		total += float64(c.SharesOutstanding) * c.ConversionRatio
	}
	return total
}

// safeDivide returns a/b, or 0 if b is zero.
func safeDivide(a, b float64) float64 {
	if b == 0 {
		return 0
	}
	return a / b
}
