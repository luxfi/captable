package waterfall

import (
	"math"
	"testing"
)

const tolerance = 0.01 // $0.01 tolerance for float comparison

func approxEqual(a, b float64) bool {
	return math.Abs(a-b) < tolerance
}

func assertApprox(t *testing.T, name string, got, want float64) {
	t.Helper()
	if !approxEqual(got, want) {
		t.Fatalf("%s = %.2f, want %.2f", name, got, want)
	}
}

func TestCommonOnlyExit(t *testing.T) {
	// Simple case: only common stock, $10M exit, 10M shares.
	scenario := ExitScenario{
		TotalProceeds:   10_000_000,
		TransactionType: Acquisition,
	}
	classes := []ShareClassInput{
		{
			Name:              "Common",
			SharesOutstanding: 10_000_000,
			InvestmentAmount:  100_000,
			Seniority:         0,
		},
	}

	result, err := Calculate(scenario, classes)
	if err != nil {
		t.Fatalf("Calculate: %v", err)
	}

	assertApprox(t, "TotalDistributed", result.TotalDistributed, 10_000_000)
	assertApprox(t, "Remainder", result.Remainder, 0)

	if len(result.Tiers) != 4 {
		t.Fatalf("expected 4 tiers, got %d", len(result.Tiers))
	}

	// Common gets everything in tier 4.
	tier4 := result.Tiers[3]
	if tier4.Name != "Common Distribution" {
		t.Fatalf("tier4.Name = %q, want Common Distribution", tier4.Name)
	}
	cd, ok := tier4.Distributions["Common"]
	if !ok {
		t.Fatal("no Common in tier 4")
	}
	assertApprox(t, "Common.TotalPayout", cd.TotalPayout, 10_000_000)
	assertApprox(t, "Common.PerSharePayout", cd.PerSharePayout, 1.0)
	assertApprox(t, "Common.ReturnMultiple", cd.ReturnMultiple, 100.0)
}

func TestNonParticipatingPreferredConverts(t *testing.T) {
	// Series A: 1x non-participating preferred, $5M invested, 5M shares
	// Common: 10M shares, $100k invested
	// Exit: $20M
	//
	// Tier 1: Series A gets $5M liq pref. Remaining = $15M.
	// Tier 3 as-converted comparison (all NP convert hypothetically):
	//   Pool = $15M + $5M (returned liq pref) = $20M
	//   Total shares = 5M + 10M = 15M
	//   Series A as-converted = 5M/15M * $20M = $6,666,666.67 > $5M -> convert
	// Tier 4: Common gets remainder = $20M - $6,666,666.67 = $13,333,333.33
	scenario := ExitScenario{
		TotalProceeds:   20_000_000,
		TransactionType: Acquisition,
	}
	classes := []ShareClassInput{
		{
			Name:                "Series A",
			SharesOutstanding:   5_000_000,
			InvestmentAmount:    5_000_000,
			LiquidationMultiple: 1.0,
			Participating:       false,
			Seniority:           1,
			ConversionRatio:     1.0,
		},
		{
			Name:              "Common",
			SharesOutstanding: 10_000_000,
			InvestmentAmount:  100_000,
			Seniority:         0,
		},
	}

	result, err := Calculate(scenario, classes)
	if err != nil {
		t.Fatalf("Calculate: %v", err)
	}

	assertApprox(t, "TotalDistributed", result.TotalDistributed, 20_000_000)

	// Tier 1: Series A liq pref zeroed (because it converted in tier 3).
	tier1 := result.Tiers[0]
	assertApprox(t, "Tier1.SeriesA.LiquidationPayout", tier1.Distributions["Series A"].LiquidationPayout, 0)

	// Tier 3: Series A converts.
	tier3 := result.Tiers[2]
	sa3, ok := tier3.Distributions["Series A"]
	if !ok {
		t.Fatal("Series A should appear in tier 3 (converted)")
	}
	assertApprox(t, "Tier3.SeriesA.ConversionPayout", sa3.ConversionPayout, 6_666_666.67)

	// Tier 4: Common gets remainder.
	tier4 := result.Tiers[3]
	cd := tier4.Distributions["Common"]
	assertApprox(t, "Common.TotalPayout", cd.TotalPayout, 13_333_333.33)
}

func TestNonParticipatingPreferredKeepsLiqPref(t *testing.T) {
	// When exit is low, non-participating preferred keeps liq pref.
	// Series A: 1x non-participating, $5M invested, 5M shares
	// Common: 10M shares
	// Exit: $8M
	//
	// Tier 1: $5M liq pref. Remaining = $3M.
	// Tier 3: Pool = $3M + $5M = $8M. Shares = 15M.
	//   As-converted = 5M/15M * $8M = $2,666,666.67 < $5M -> keep liq pref.
	// Tier 4: Common gets $3M.
	scenario := ExitScenario{
		TotalProceeds:   8_000_000,
		TransactionType: Acquisition,
	}
	classes := []ShareClassInput{
		{
			Name:                "Series A",
			SharesOutstanding:   5_000_000,
			InvestmentAmount:    5_000_000,
			LiquidationMultiple: 1.0,
			Participating:       false,
			Seniority:           1,
			ConversionRatio:     1.0,
		},
		{
			Name:              "Common",
			SharesOutstanding: 10_000_000,
			InvestmentAmount:  100_000,
			Seniority:         0,
		},
	}

	result, err := Calculate(scenario, classes)
	if err != nil {
		t.Fatalf("Calculate: %v", err)
	}

	// Tier 1: Series A keeps $5M liq pref.
	tier1 := result.Tiers[0]
	d := tier1.Distributions["Series A"]
	assertApprox(t, "Tier1.SeriesA.LiquidationPayout", d.LiquidationPayout, 5_000_000)

	// Tier 3: No conversion.
	tier3 := result.Tiers[2]
	if _, ok := tier3.Distributions["Series A"]; ok {
		t.Fatal("Series A should NOT convert when liq pref is better")
	}

	// Tier 4: Common gets remainder ($3M).
	tier4 := result.Tiers[3]
	cd := tier4.Distributions["Common"]
	assertApprox(t, "Common.TotalPayout", cd.TotalPayout, 3_000_000)
}

func TestParticipatingPreferred(t *testing.T) {
	// Series A: 2x participating preferred (uncapped), $5M invested, 5M shares
	// Common: 10M shares, $100k invested
	// Exit: $30M
	//
	// Tier 1: Series A gets 2x = $10M. Remaining = $20M.
	// Tier 2: Series A participates pro-rata on as-converted basis.
	//   Total as-converted = 5M + 10M = 15M.
	//   Series A share = 5M/15M * $20M = $6,666,666.67.
	// Tier 4: Common gets $20M - $6,666,666.67 = $13,333,333.33.
	scenario := ExitScenario{
		TotalProceeds:   30_000_000,
		TransactionType: Acquisition,
	}
	classes := []ShareClassInput{
		{
			Name:                "Series A",
			SharesOutstanding:   5_000_000,
			InvestmentAmount:    5_000_000,
			LiquidationMultiple: 2.0,
			Participating:       true,
			Seniority:           1,
			ConversionRatio:     1.0,
		},
		{
			Name:              "Common",
			SharesOutstanding: 10_000_000,
			InvestmentAmount:  100_000,
			Seniority:         0,
		},
	}

	result, err := Calculate(scenario, classes)
	if err != nil {
		t.Fatalf("Calculate: %v", err)
	}

	assertApprox(t, "TotalDistributed", result.TotalDistributed, 30_000_000)

	// Tier 1: $10M liq pref.
	tier1 := result.Tiers[0]
	sa1 := tier1.Distributions["Series A"]
	assertApprox(t, "Tier1.SeriesA.LiquidationPayout", sa1.LiquidationPayout, 10_000_000)

	// Tier 2: Participation.
	tier2 := result.Tiers[1]
	sa2 := tier2.Distributions["Series A"]
	assertApprox(t, "Tier2.SeriesA.ParticipationPayout", sa2.ParticipationPayout, 6_666_666.67)

	// Total Series A: $10M + $6.67M = $16.67M.
	totalSA := sa1.LiquidationPayout + sa2.ParticipationPayout
	assertApprox(t, "SeriesA.Total", totalSA, 16_666_666.67)

	// Tier 4: Common gets remainder.
	tier4 := result.Tiers[3]
	cd := tier4.Distributions["Common"]
	assertApprox(t, "Common.TotalPayout", cd.TotalPayout, 13_333_333.33)
}

func TestParticipatingPreferredWithCap(t *testing.T) {
	// Series A: 1x participating, capped at 3x total return, $5M invested, 5M shares
	// Common: 10M shares
	// Exit: $50M
	//
	// Tier 1: $5M liq pref. Remaining = $45M.
	// Tier 2: Pro-rata = 5M/15M * $45M = $15M.
	//   But cap = 3x * $5M = $15M total. Already got $5M. Max participation = $10M.
	// Tier 4: Common gets $50M - $5M - $10M = $35M.
	scenario := ExitScenario{
		TotalProceeds:   50_000_000,
		TransactionType: Acquisition,
	}
	classes := []ShareClassInput{
		{
			Name:                "Series A",
			SharesOutstanding:   5_000_000,
			InvestmentAmount:    5_000_000,
			LiquidationMultiple: 1.0,
			Participating:       true,
			ParticipationCap:    3.0,
			Seniority:           1,
			ConversionRatio:     1.0,
		},
		{
			Name:              "Common",
			SharesOutstanding: 10_000_000,
			InvestmentAmount:  100_000,
			Seniority:         0,
		},
	}

	result, err := Calculate(scenario, classes)
	if err != nil {
		t.Fatalf("Calculate: %v", err)
	}

	// Tier 1: $5M.
	tier1 := result.Tiers[0]
	sa1 := tier1.Distributions["Series A"]
	assertApprox(t, "Tier1.SeriesA", sa1.LiquidationPayout, 5_000_000)

	// Tier 2: Capped at $10M participation.
	tier2 := result.Tiers[1]
	sa2 := tier2.Distributions["Series A"]
	assertApprox(t, "Tier2.SeriesA.ParticipationPayout", sa2.ParticipationPayout, 10_000_000)

	// Common gets remainder.
	tier4 := result.Tiers[3]
	cd := tier4.Distributions["Common"]
	assertApprox(t, "Common.TotalPayout", cd.TotalPayout, 35_000_000)
}

func TestMultipleSeniorityTiers(t *testing.T) {
	// Series C (seniority 3): $10M invested, 5M shares, 1x NP
	// Series B (seniority 2): $7M invested, 5M shares, 1x NP
	// Series A (seniority 1): $3M invested, 5M shares, 1x NP
	// Common: 20M shares
	// Exit: $50M
	//
	// Tier 1: C=$10M, B=$7M, A=$3M. Remaining=$30M.
	// Tier 3: All NP. Hypothetical pool = $30M + $10M + $7M + $3M = $50M.
	//   Total shares = 5M+5M+5M+20M = 35M.
	//   C: 5M/35M * $50M = $7,142,857.14 < $10M -> keep liq pref
	//   B: 5M/35M * $50M = $7,142,857.14 > $7M -> convert
	//   A: 5M/35M * $50M = $7,142,857.14 > $3M -> convert
	//
	// After B and A convert: remaining = $30M + $7M + $3M - 2*$7,142,857.14 = $25,714,285.71
	// Tier 4: Common gets $25,714,285.71.
	//
	// Check: C=$10M + B=$7.14M + A=$7.14M + Common=$25.71M = $50M.
	scenario := ExitScenario{
		TotalProceeds:   50_000_000,
		TransactionType: Acquisition,
	}
	classes := []ShareClassInput{
		{
			Name:                "Series C",
			SharesOutstanding:   5_000_000,
			InvestmentAmount:    10_000_000,
			LiquidationMultiple: 1.0,
			Participating:       false,
			Seniority:           3,
			ConversionRatio:     1.0,
		},
		{
			Name:                "Series B",
			SharesOutstanding:   5_000_000,
			InvestmentAmount:    7_000_000,
			LiquidationMultiple: 1.0,
			Participating:       false,
			Seniority:           2,
			ConversionRatio:     1.0,
		},
		{
			Name:                "Series A",
			SharesOutstanding:   5_000_000,
			InvestmentAmount:    3_000_000,
			LiquidationMultiple: 1.0,
			Participating:       false,
			Seniority:           1,
			ConversionRatio:     1.0,
		},
		{
			Name:              "Common",
			SharesOutstanding: 20_000_000,
			InvestmentAmount:  200_000,
			Seniority:         0,
		},
	}

	result, err := Calculate(scenario, classes)
	if err != nil {
		t.Fatalf("Calculate: %v", err)
	}

	assertApprox(t, "TotalDistributed", result.TotalDistributed, 50_000_000)

	// Tier 1: C keeps liq pref; B and A get zeroed (they convert).
	tier1 := result.Tiers[0]
	assertApprox(t, "Tier1.SeriesC", tier1.Distributions["Series C"].LiquidationPayout, 10_000_000)
	assertApprox(t, "Tier1.SeriesB.zeroed", tier1.Distributions["Series B"].LiquidationPayout, 0)
	assertApprox(t, "Tier1.SeriesA.zeroed", tier1.Distributions["Series A"].LiquidationPayout, 0)

	// Tier 3: B and A convert. C does not.
	tier3 := result.Tiers[2]
	if _, ok := tier3.Distributions["Series C"]; ok {
		t.Fatal("Series C should NOT convert")
	}

	sbConv, ok := tier3.Distributions["Series B"]
	if !ok {
		t.Fatal("Series B should convert")
	}
	assertApprox(t, "Tier3.SeriesB.ConversionPayout", sbConv.ConversionPayout, 7_142_857.14)

	saConv, ok := tier3.Distributions["Series A"]
	if !ok {
		t.Fatal("Series A should convert")
	}
	assertApprox(t, "Tier3.SeriesA.ConversionPayout", saConv.ConversionPayout, 7_142_857.14)

	// Tier 4: Common gets remainder.
	tier4 := result.Tiers[3]
	cd := tier4.Distributions["Common"]
	// $50M - $10M - $7.14M - $7.14M = $25,714,285.71
	assertApprox(t, "Common.TotalPayout", cd.TotalPayout, 25_714_285.71)
}

func TestInsufficientProceeds(t *testing.T) {
	// Exit is only $3M but liq prefs total $8M.
	// Series B (seniority 2): $5M invested, 1x liq pref
	// Series A (seniority 1): $3M invested, 1x liq pref
	// Common: 10M shares
	// Series B gets all $3M (most senior). Series A and Common get $0.
	scenario := ExitScenario{
		TotalProceeds:   3_000_000,
		TransactionType: Dissolution,
	}
	classes := []ShareClassInput{
		{
			Name:                "Series B",
			SharesOutstanding:   3_000_000,
			InvestmentAmount:    5_000_000,
			LiquidationMultiple: 1.0,
			Participating:       false,
			Seniority:           2,
			ConversionRatio:     1.0,
		},
		{
			Name:                "Series A",
			SharesOutstanding:   2_000_000,
			InvestmentAmount:    3_000_000,
			LiquidationMultiple: 1.0,
			Participating:       false,
			Seniority:           1,
			ConversionRatio:     1.0,
		},
		{
			Name:              "Common",
			SharesOutstanding: 10_000_000,
			InvestmentAmount:  100_000,
			Seniority:         0,
		},
	}

	result, err := Calculate(scenario, classes)
	if err != nil {
		t.Fatalf("Calculate: %v", err)
	}

	assertApprox(t, "TotalDistributed", result.TotalDistributed, 3_000_000)
	assertApprox(t, "Remainder", result.Remainder, 0)

	if len(result.Tiers) != 4 {
		t.Fatalf("expected 4 tiers, got %d", len(result.Tiers))
	}

	// Tier 1: Series B gets all $3M (needs $5M but only $3M available).
	tier1 := result.Tiers[0]
	assertApprox(t, "Tier1.SeriesB", tier1.Distributions["Series B"].LiquidationPayout, 3_000_000)
	assertApprox(t, "Tier1.SeriesB.ReturnMultiple", tier1.Distributions["Series B"].ReturnMultiple, 0.60)

	// Series A gets nothing (remaining=0 after Series B took it all).
	if d, ok := tier1.Distributions["Series A"]; ok {
		assertApprox(t, "Tier1.SeriesA", d.LiquidationPayout, 0)
	}

	// Common gets nothing.
	tier4 := result.Tiers[3]
	if len(tier4.Distributions) != 0 {
		if d, ok := tier4.Distributions["Common"]; ok {
			assertApprox(t, "Common.TotalPayout", d.TotalPayout, 0)
		}
	}
}

func TestInsufficientProceedsProRataWithinTier(t *testing.T) {
	// Two classes at same seniority, insufficient proceeds.
	// Series A1 (seniority 1): $4M invested, 1x liq pref
	// Series A2 (seniority 1): $6M invested, 1x liq pref
	// Exit: $5M
	// Pro-rata: A1 gets 4/10 * $5M = $2M, A2 gets 6/10 * $5M = $3M.
	scenario := ExitScenario{
		TotalProceeds:   5_000_000,
		TransactionType: Dissolution,
	}
	classes := []ShareClassInput{
		{
			Name:                "Series A1",
			SharesOutstanding:   4_000_000,
			InvestmentAmount:    4_000_000,
			LiquidationMultiple: 1.0,
			Seniority:           1,
			ConversionRatio:     1.0,
		},
		{
			Name:                "Series A2",
			SharesOutstanding:   6_000_000,
			InvestmentAmount:    6_000_000,
			LiquidationMultiple: 1.0,
			Seniority:           1,
			ConversionRatio:     1.0,
		},
		{
			Name:              "Common",
			SharesOutstanding: 10_000_000,
			InvestmentAmount:  100_000,
			Seniority:         0,
		},
	}

	result, err := Calculate(scenario, classes)
	if err != nil {
		t.Fatalf("Calculate: %v", err)
	}

	tier1 := result.Tiers[0]
	assertApprox(t, "Tier1.A1", tier1.Distributions["Series A1"].LiquidationPayout, 2_000_000)
	assertApprox(t, "Tier1.A2", tier1.Distributions["Series A2"].LiquidationPayout, 3_000_000)
	assertApprox(t, "A1.ReturnMultiple", tier1.Distributions["Series A1"].ReturnMultiple, 0.50)
	assertApprox(t, "A2.ReturnMultiple", tier1.Distributions["Series A2"].ReturnMultiple, 0.50)
}

func TestValidationErrors(t *testing.T) {
	_, err := Calculate(ExitScenario{TotalProceeds: -1}, []ShareClassInput{{Name: "A"}})
	if err == nil {
		t.Fatal("expected error for negative proceeds")
	}

	_, err = Calculate(ExitScenario{TotalProceeds: 100}, nil)
	if err == nil {
		t.Fatal("expected error for empty classes")
	}
}
