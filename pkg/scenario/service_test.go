package scenario

import (
	"math"
	"strings"
	"testing"
)

// almostEqual checks that two floats are within epsilon.
func almostEqual(a, b, epsilon float64) bool {
	return math.Abs(a-b) < epsilon
}

func TestModelRoundSeriesA(t *testing.T) {
	// Founders 80%, Angels 20% of 10M shares.
	// Series A: $5M investment at $20M pre-money.
	existing := []OwnershipRow{
		{Name: "Founders", SharesBefore: 8_000_000},
		{Name: "Angels", SharesBefore: 2_000_000},
	}
	round := FundingRound{
		Name:              "Series A",
		PreMoneyValuation: 20_000_000,
		InvestmentAmount:  5_000_000,
		ShareClassType:    "preferred",
	}

	result, err := ModelRound(existing, round, 0)
	if err != nil {
		t.Fatalf("ModelRound: %v", err)
	}

	// Price per share = $20M / 10M shares = $2.00.
	if !almostEqual(result.PricePerShare, 2.0, 0.001) {
		t.Fatalf("price_per_share = %f, want 2.0", result.PricePerShare)
	}

	// New shares = $5M / $2.00 = 2,500,000.
	if result.NewSharesIssued != 2_500_000 {
		t.Fatalf("new_shares_issued = %d, want 2500000", result.NewSharesIssued)
	}

	// Post-money = $25M.
	if result.PostMoneyValuation != 25_000_000 {
		t.Fatalf("post_money_valuation = %d, want 25000000", result.PostMoneyValuation)
	}

	// Fully diluted = 10M + 2.5M = 12.5M.
	if result.FullyDilutedShares != 12_500_000 {
		t.Fatalf("fully_diluted_shares = %d, want 12500000", result.FullyDilutedShares)
	}

	// Founders: 8M/12.5M = 64%.
	founders := result.Ownership[0]
	if !almostEqual(founders.PercentAfter, 64.0, 0.1) {
		t.Fatalf("founders percent_after = %f, want ~64.0", founders.PercentAfter)
	}

	// Angels: 2M/12.5M = 16%.
	angels := result.Ownership[1]
	if !almostEqual(angels.PercentAfter, 16.0, 0.1) {
		t.Fatalf("angels percent_after = %f, want ~16.0", angels.PercentAfter)
	}

	// Investor: 2.5M/12.5M = 20%.
	investor := result.Ownership[2]
	if !almostEqual(investor.PercentAfter, 20.0, 0.1) {
		t.Fatalf("investor percent_after = %f, want ~20.0", investor.PercentAfter)
	}

	// Dilution: Founders were 80%, now 64% => 16pp dilution.
	if !almostEqual(founders.Dilution, 16.0, 0.1) {
		t.Fatalf("founders dilution = %f, want ~16.0", founders.Dilution)
	}

	// Angels were 20%, now 16% => 4pp dilution.
	if !almostEqual(angels.Dilution, 4.0, 0.1) {
		t.Fatalf("angels dilution = %f, want ~4.0", angels.Dilution)
	}
}

func TestModelRoundOptionPoolExpansion(t *testing.T) {
	// Start with 10M shares, expand option pool to 15% post-money.
	existing := []OwnershipRow{
		{Name: "Founders", SharesBefore: 8_000_000},
		{Name: "Angels", SharesBefore: 2_000_000},
	}
	round := FundingRound{
		Name:              "Series A",
		PreMoneyValuation: 20_000_000,
		InvestmentAmount:  5_000_000,
		ShareClassType:    "preferred",
	}

	result, err := ModelRound(existing, round, 15.0)
	if err != nil {
		t.Fatalf("ModelRound: %v", err)
	}

	if result.OptionPoolIncrease <= 0 {
		t.Fatal("expected option pool increase > 0")
	}

	// Verify option pool is ~15% of fully diluted.
	poolPct := float64(result.OptionPoolShares) / float64(result.FullyDilutedShares) * 100.0
	if !almostEqual(poolPct, 15.0, 0.1) {
		t.Fatalf("option pool percent = %f, want ~15.0", poolPct)
	}

	// Total percent should sum to ~100%.
	var totalPct float64
	for _, row := range result.Ownership {
		totalPct += row.PercentAfter
	}
	if !almostEqual(totalPct, 100.0, 0.1) {
		t.Fatalf("total percent = %f, want ~100.0", totalPct)
	}
}

func TestModelRoundMultipleRounds(t *testing.T) {
	// Seed: Founders 10M shares, $2M at $8M pre.
	existing := []OwnershipRow{
		{Name: "Founders", SharesBefore: 10_000_000},
	}

	seedRound := FundingRound{
		Name:              "Seed",
		PreMoneyValuation: 8_000_000,
		InvestmentAmount:  2_000_000,
		ShareClassType:    "preferred",
	}

	seedResult, err := ModelRound(existing, seedRound, 0)
	if err != nil {
		t.Fatalf("Seed ModelRound: %v", err)
	}

	// Series A: $5M at $20M pre using the post-seed ownership.
	seriesARound := FundingRound{
		Name:              "Series A",
		PreMoneyValuation: 20_000_000,
		InvestmentAmount:  5_000_000,
		ShareClassType:    "preferred",
	}

	// Convert seed result ownership rows to input for next round.
	postSeedShares := make([]OwnershipRow, 0, len(seedResult.Ownership))
	for _, row := range seedResult.Ownership {
		if row.SharesAfter > 0 {
			postSeedShares = append(postSeedShares, OwnershipRow{
				Name:         row.Name,
				SharesBefore: row.SharesAfter,
			})
		}
	}

	seriesAResult, err := ModelRound(postSeedShares, seriesARound, 0)
	if err != nil {
		t.Fatalf("Series A ModelRound: %v", err)
	}

	// Founders should be diluted across both rounds.
	var foundersAfterA float64
	for _, row := range seriesAResult.Ownership {
		if row.Name == "Founders" {
			foundersAfterA = row.PercentAfter
			break
		}
	}

	// Founders started at 100%, after seed ~80%, after A should be further diluted.
	if foundersAfterA >= 80.0 {
		t.Fatalf("founders after Series A = %f, should be < 80%%", foundersAfterA)
	}
	if foundersAfterA <= 0 {
		t.Fatalf("founders after Series A = %f, should be > 0", foundersAfterA)
	}

	// Post-money valuation = $25M.
	if seriesAResult.PostMoneyValuation != 25_000_000 {
		t.Fatalf("post_money = %d, want 25000000", seriesAResult.PostMoneyValuation)
	}
}

func TestModelRoundDilutionCalculations(t *testing.T) {
	existing := []OwnershipRow{
		{Name: "Founder A", SharesBefore: 6_000_000},
		{Name: "Founder B", SharesBefore: 3_000_000},
		{Name: "Angel", SharesBefore: 1_000_000},
	}

	round := FundingRound{
		Name:              "Series A",
		PreMoneyValuation: 30_000_000,
		InvestmentAmount:  10_000_000,
		ShareClassType:    "preferred",
	}

	result, err := ModelRound(existing, round, 0)
	if err != nil {
		t.Fatalf("ModelRound: %v", err)
	}

	// Each existing holder should have dilution = percentBefore - percentAfter.
	for _, row := range result.Ownership {
		if row.SharesBefore > 0 {
			expectedDilution := row.PercentBefore - row.PercentAfter
			if !almostEqual(row.Dilution, expectedDilution, 0.001) {
				t.Fatalf("%s dilution = %f, want %f", row.Name, row.Dilution, expectedDilution)
			}
			if row.Dilution < 0 {
				t.Fatalf("%s has negative dilution %f", row.Name, row.Dilution)
			}
		}
	}

	// All dilutions should be proportional (same percentage for each holder).
	if len(result.Ownership) >= 2 {
		d0 := result.Ownership[0].Dilution / result.Ownership[0].PercentBefore
		d1 := result.Ownership[1].Dilution / result.Ownership[1].PercentBefore
		if !almostEqual(d0, d1, 0.001) {
			t.Fatalf("dilution not proportional: %f vs %f", d0, d1)
		}
	}
}

func TestModelRoundValidation(t *testing.T) {
	tests := []struct {
		name    string
		shares  []OwnershipRow
		round   FundingRound
		pool    float64
		wantErr string
	}{
		{
			"zero pre-money",
			[]OwnershipRow{{Name: "F", SharesBefore: 100}},
			FundingRound{PreMoneyValuation: 0, InvestmentAmount: 100},
			0,
			"pre_money_valuation",
		},
		{
			"zero investment",
			[]OwnershipRow{{Name: "F", SharesBefore: 100}},
			FundingRound{PreMoneyValuation: 100, InvestmentAmount: 0},
			0,
			"investment_amount",
		},
		{
			"empty shares",
			nil,
			FundingRound{PreMoneyValuation: 100, InvestmentAmount: 100},
			0,
			"existing_shares",
		},
		{
			"negative pool",
			[]OwnershipRow{{Name: "F", SharesBefore: 100}},
			FundingRound{PreMoneyValuation: 100, InvestmentAmount: 100},
			-1,
			"option_pool_percent",
		},
		{
			"pool 100",
			[]OwnershipRow{{Name: "F", SharesBefore: 100}},
			FundingRound{PreMoneyValuation: 100, InvestmentAmount: 100},
			100,
			"option_pool_percent",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := ModelRound(tt.shares, tt.round, tt.pool)
			if err == nil {
				t.Fatal("expected error")
			}
			if !strings.Contains(err.Error(), tt.wantErr) {
				t.Fatalf("error %q does not contain %q", err.Error(), tt.wantErr)
			}
		})
	}
}

func TestModelExitProRata(t *testing.T) {
	// 3 holders, $100M exit.
	shares := []OwnershipRow{
		{Name: "Founders", SharesAfter: 6_000_000},
		{Name: "Seed Investor", SharesAfter: 2_000_000},
		{Name: "Series A Investor", SharesAfter: 2_000_000},
	}

	result, err := ModelExit(shares, 100_000_000)
	if err != nil {
		t.Fatalf("ModelExit: %v", err)
	}

	if result.ExitValuation != 100_000_000 {
		t.Fatalf("exit_valuation = %d, want 100000000", result.ExitValuation)
	}

	// Founders: 60% of $100M = $60M.
	if !almostEqual(result.Ownership[0].Proceeds, 60_000_000, 1.0) {
		t.Fatalf("founders proceeds = %f, want ~60000000", result.Ownership[0].Proceeds)
	}
	if !almostEqual(result.Ownership[0].PercentOfExit, 60.0, 0.1) {
		t.Fatalf("founders percent_of_exit = %f, want ~60.0", result.Ownership[0].PercentOfExit)
	}

	// Each investor: 20% of $100M = $20M.
	if !almostEqual(result.Ownership[1].Proceeds, 20_000_000, 1.0) {
		t.Fatalf("seed proceeds = %f, want ~20000000", result.Ownership[1].Proceeds)
	}
	if !almostEqual(result.Ownership[2].Proceeds, 20_000_000, 1.0) {
		t.Fatalf("series a proceeds = %f, want ~20000000", result.Ownership[2].Proceeds)
	}

	// Total proceeds should equal exit valuation.
	var total float64
	for _, row := range result.Ownership {
		total += row.Proceeds
	}
	if !almostEqual(total, 100_000_000, 1.0) {
		t.Fatalf("total proceeds = %f, want ~100000000", total)
	}
}

func TestModelExitValidation(t *testing.T) {
	tests := []struct {
		name    string
		shares  []OwnershipRow
		exit    int64
		wantErr string
	}{
		{"zero exit", []OwnershipRow{{SharesAfter: 100}}, 0, "exit_valuation"},
		{"negative exit", []OwnershipRow{{SharesAfter: 100}}, -1, "exit_valuation"},
		{"empty shares", nil, 100, "shares must not be empty"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := ModelExit(tt.shares, tt.exit)
			if err == nil {
				t.Fatal("expected error")
			}
			if !strings.Contains(err.Error(), tt.wantErr) {
				t.Fatalf("error %q does not contain %q", err.Error(), tt.wantErr)
			}
		})
	}
}
