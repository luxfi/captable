package conversion

import (
	"math"
	"strings"
	"testing"
	"time"
)

func almostEqual(a, b, epsilon float64) bool {
	return math.Abs(a-b) < epsilon
}

func TestConvertSAFECapOnly(t *testing.T) {
	// $500K SAFE with $5M cap, no discount.
	// Round price = $2.00/share, 10M pre-money shares.
	// Cap price = $5M / 10M = $0.50.
	// Effective price = $0.50 (cap is better for investor).
	safe := SAFE{
		ID:               "safe-1",
		InvestorID:       "inv-1",
		InvestmentAmount: 500_000,
		ValuationCap:     5_000_000,
		Discount:         0,
		Type:             "pre_money",
	}

	trigger := ConversionTrigger{
		Type:               "qualified_financing",
		RoundPricePerShare: 2.00,
		PreMoneyShares:     10_000_000,
	}

	result, err := ConvertSAFE(safe, trigger)
	if err != nil {
		t.Fatalf("ConvertSAFE: %v", err)
	}

	if result.Method != "cap" {
		t.Fatalf("method = %q, want cap", result.Method)
	}
	if !almostEqual(result.PricePerShare, 0.50, 0.001) {
		t.Fatalf("price_per_share = %f, want 0.50", result.PricePerShare)
	}
	// Shares = $500K / $0.50 = 1,000,000.
	if result.SharesIssued != 1_000_000 {
		t.Fatalf("shares_issued = %d, want 1000000", result.SharesIssued)
	}
	if result.InstrumentType != "safe" {
		t.Fatalf("instrument_type = %q, want safe", result.InstrumentType)
	}
	// Effective discount = (1 - 0.50/2.00) * 100 = 75%.
	if !almostEqual(result.EffectiveDiscount, 75.0, 0.1) {
		t.Fatalf("effective_discount = %f, want ~75.0", result.EffectiveDiscount)
	}
}

func TestConvertSAFEDiscountOnly(t *testing.T) {
	// $250K SAFE with no cap, 20% discount.
	// Round price = $2.00/share.
	// Discount price = $2.00 * 0.80 = $1.60.
	safe := SAFE{
		ID:               "safe-2",
		InvestorID:       "inv-2",
		InvestmentAmount: 250_000,
		ValuationCap:     0,
		Discount:         20,
		Type:             "pre_money",
	}

	trigger := ConversionTrigger{
		Type:               "qualified_financing",
		RoundPricePerShare: 2.00,
		PreMoneyShares:     10_000_000,
	}

	result, err := ConvertSAFE(safe, trigger)
	if err != nil {
		t.Fatalf("ConvertSAFE: %v", err)
	}

	if result.Method != "discount" {
		t.Fatalf("method = %q, want discount", result.Method)
	}
	if !almostEqual(result.PricePerShare, 1.60, 0.001) {
		t.Fatalf("price_per_share = %f, want 1.60", result.PricePerShare)
	}
	// Shares = $250K / $1.60 = 156,250.
	if result.SharesIssued != 156_250 {
		t.Fatalf("shares_issued = %d, want 156250", result.SharesIssued)
	}
	if !almostEqual(result.EffectiveDiscount, 20.0, 0.1) {
		t.Fatalf("effective_discount = %f, want ~20.0", result.EffectiveDiscount)
	}
}

func TestConvertSAFECapAndDiscount(t *testing.T) {
	// $500K SAFE with $8M cap and 20% discount.
	// Round price = $2.00, 10M pre-money shares.
	// Cap price = $8M / 10M = $0.80.
	// Discount price = $2.00 * 0.80 = $1.60.
	// Effective price = min($0.80, $1.60) = $0.80 (cap wins).
	safe := SAFE{
		ID:               "safe-3",
		InvestorID:       "inv-3",
		InvestmentAmount: 500_000,
		ValuationCap:     8_000_000,
		Discount:         20,
		Type:             "pre_money",
	}

	trigger := ConversionTrigger{
		Type:               "qualified_financing",
		RoundPricePerShare: 2.00,
		PreMoneyShares:     10_000_000,
	}

	result, err := ConvertSAFE(safe, trigger)
	if err != nil {
		t.Fatalf("ConvertSAFE: %v", err)
	}

	if result.Method != "cap" {
		t.Fatalf("method = %q, want cap (cap price $0.80 < discount price $1.60)", result.Method)
	}
	if !almostEqual(result.PricePerShare, 0.80, 0.001) {
		t.Fatalf("price_per_share = %f, want 0.80", result.PricePerShare)
	}
	// Shares = $500K / $0.80 = 625,000.
	if result.SharesIssued != 625_000 {
		t.Fatalf("shares_issued = %d, want 625000", result.SharesIssued)
	}
}

func TestConvertSAFECapAndDiscountDiscountWins(t *testing.T) {
	// Verify discount wins when cap price is higher.
	// $500K SAFE with $20M cap and 20% discount.
	// Round price = $2.00, 10M pre-money shares.
	// Cap price = $20M / 10M = $2.00.
	// Discount price = $2.00 * 0.80 = $1.60.
	// Effective price = min($2.00, $1.60) = $1.60 (discount wins).
	safe := SAFE{
		ID:               "safe-4",
		InvestorID:       "inv-4",
		InvestmentAmount: 500_000,
		ValuationCap:     20_000_000,
		Discount:         20,
		Type:             "pre_money",
	}

	trigger := ConversionTrigger{
		Type:               "qualified_financing",
		RoundPricePerShare: 2.00,
		PreMoneyShares:     10_000_000,
	}

	result, err := ConvertSAFE(safe, trigger)
	if err != nil {
		t.Fatalf("ConvertSAFE: %v", err)
	}

	if result.Method != "discount" {
		t.Fatalf("method = %q, want discount (discount price $1.60 < cap price $2.00)", result.Method)
	}
	if !almostEqual(result.PricePerShare, 1.60, 0.001) {
		t.Fatalf("price_per_share = %f, want 1.60", result.PricePerShare)
	}
}

func TestConvertSAFEPostMoney(t *testing.T) {
	// $1M post-money SAFE with $10M cap.
	// Post-money shares = 10M. Cap price = $10M / 10M = $1.00.
	// Round price = $2.00.
	safe := SAFE{
		ID:               "safe-5",
		InvestorID:       "inv-5",
		InvestmentAmount: 1_000_000,
		ValuationCap:     10_000_000,
		Discount:         0,
		Type:             "post_money",
	}

	trigger := ConversionTrigger{
		Type:               "qualified_financing",
		RoundPricePerShare: 2.00,
		PostMoneyShares:    10_000_000,
		PreMoneyShares:     8_000_000,
	}

	result, err := ConvertSAFE(safe, trigger)
	if err != nil {
		t.Fatalf("ConvertSAFE: %v", err)
	}

	if !almostEqual(result.PricePerShare, 1.00, 0.001) {
		t.Fatalf("price_per_share = %f, want 1.00", result.PricePerShare)
	}
	// Shares = $1M / $1.00 = 1,000,000.
	if result.SharesIssued != 1_000_000 {
		t.Fatalf("shares_issued = %d, want 1000000", result.SharesIssued)
	}
}

func TestConvertNoteWithAccruedInterest(t *testing.T) {
	// $100K note at 8% annual, 12 months accrued.
	// Accrued = $100K * 0.08 * (365/365) = $8,000.
	// Converted amount = $108,000.
	// Cap = $5M, 10M pre-money shares. Cap price = $0.50.
	// Round price = $2.00. Discount = 0.
	issueDate := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
	conversionDate := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)

	note := ConvertibleNote{
		ID:              "note-1",
		InvestorID:      "inv-10",
		PrincipalAmount: 100_000,
		InterestRate:    8.0,
		MaturityDate:    time.Date(2027, 1, 1, 0, 0, 0, 0, time.UTC),
		ValuationCap:    5_000_000,
		Discount:        0,
		IssueDate:       issueDate,
	}

	trigger := ConversionTrigger{
		Type:               "qualified_financing",
		RoundPricePerShare: 2.00,
		PreMoneyShares:     10_000_000,
		ConversionDate:     conversionDate,
	}

	result, err := ConvertNote(note, trigger)
	if err != nil {
		t.Fatalf("ConvertNote: %v", err)
	}

	// Accrued interest = $100K * 0.08 * (365/365) = $8,000.
	if !almostEqual(result.ConvertedAmount, 108_000, 100) {
		t.Fatalf("converted_amount = %f, want ~108000", result.ConvertedAmount)
	}
	if result.OriginalInvestment != 100_000 {
		t.Fatalf("original_investment = %f, want 100000", result.OriginalInvestment)
	}

	// Cap price = $0.50. Shares = $108,000 / $0.50 = 216,000.
	if !almostEqual(result.PricePerShare, 0.50, 0.001) {
		t.Fatalf("price_per_share = %f, want 0.50", result.PricePerShare)
	}
	if result.SharesIssued != 216_000 {
		t.Fatalf("shares_issued = %d, want 216000", result.SharesIssued)
	}
	if result.InstrumentType != "note" {
		t.Fatalf("instrument_type = %q, want note", result.InstrumentType)
	}
	if result.Method != "cap" {
		t.Fatalf("method = %q, want cap", result.Method)
	}
}

func TestConvertNoteAtMaturity(t *testing.T) {
	// Note at maturity with no qualified financing.
	// Uses cap price at maturity date for interest calculation.
	issueDate := time.Date(2024, 6, 1, 0, 0, 0, 0, time.UTC)
	maturityDate := time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC)

	note := ConvertibleNote{
		ID:                "note-2",
		InvestorID:        "inv-11",
		PrincipalAmount:   200_000,
		InterestRate:      6.0,
		MaturityDate:      maturityDate,
		ValuationCap:      8_000_000,
		Discount:          20,
		ConversionTrigger: "maturity",
		IssueDate:         issueDate,
	}

	trigger := ConversionTrigger{
		Type:               "maturity",
		RoundPricePerShare: 1.00,
		PreMoneyShares:     10_000_000,
		ConversionDate:     maturityDate,
	}

	result, err := ConvertNote(note, trigger)
	if err != nil {
		t.Fatalf("ConvertNote: %v", err)
	}

	// 2 years of interest: $200K * 0.06 * (731/365) ~= $24,032.88
	days := maturityDate.Sub(issueDate).Hours() / 24.0
	expectedInterest := 200_000 * 0.06 * (days / 365.0)
	expectedConverted := 200_000 + expectedInterest

	if !almostEqual(result.ConvertedAmount, expectedConverted, 10) {
		t.Fatalf("converted_amount = %f, want ~%f", result.ConvertedAmount, expectedConverted)
	}

	// Cap price = $8M / 10M = $0.80.
	// Discount price = $1.00 * 0.80 = $0.80.
	// Both are $0.80 — cap wins (checked first).
	if !almostEqual(result.PricePerShare, 0.80, 0.001) {
		t.Fatalf("price_per_share = %f, want 0.80", result.PricePerShare)
	}
}

func TestConvertNoteDiscountOnly(t *testing.T) {
	// Note with discount but no cap.
	issueDate := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
	conversionDate := time.Date(2025, 7, 1, 0, 0, 0, 0, time.UTC)

	note := ConvertibleNote{
		ID:              "note-3",
		InvestorID:      "inv-12",
		PrincipalAmount: 50_000,
		InterestRate:    5.0,
		MaturityDate:    time.Date(2027, 1, 1, 0, 0, 0, 0, time.UTC),
		ValuationCap:    0,
		Discount:        25,
		IssueDate:       issueDate,
	}

	trigger := ConversionTrigger{
		Type:               "qualified_financing",
		RoundPricePerShare: 4.00,
		PreMoneyShares:     10_000_000,
		ConversionDate:     conversionDate,
	}

	result, err := ConvertNote(note, trigger)
	if err != nil {
		t.Fatalf("ConvertNote: %v", err)
	}

	if result.Method != "discount" {
		t.Fatalf("method = %q, want discount", result.Method)
	}

	// Discount price = $4.00 * 0.75 = $3.00.
	if !almostEqual(result.PricePerShare, 3.00, 0.001) {
		t.Fatalf("price_per_share = %f, want 3.00", result.PricePerShare)
	}

	// ~6 months accrued: $50K * 0.05 * (181/365) ~= $1,239.73
	days := conversionDate.Sub(issueDate).Hours() / 24.0
	expectedInterest := 50_000 * 0.05 * (days / 365.0)
	expectedConverted := 50_000 + expectedInterest

	if !almostEqual(result.ConvertedAmount, expectedConverted, 5) {
		t.Fatalf("converted_amount = %f, want ~%f", result.ConvertedAmount, expectedConverted)
	}
}

func TestConvertAllMultipleInstruments(t *testing.T) {
	issueDate := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
	conversionDate := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)

	safes := []SAFE{
		{
			ID:               "safe-a",
			InvestorID:       "inv-a",
			InvestmentAmount: 500_000,
			ValuationCap:     10_000_000,
			Discount:         0,
			Type:             "pre_money",
		},
		{
			ID:               "safe-b",
			InvestorID:       "inv-b",
			InvestmentAmount: 250_000,
			ValuationCap:     0,
			Discount:         20,
			Type:             "pre_money",
		},
	}

	notes := []ConvertibleNote{
		{
			ID:              "note-a",
			InvestorID:      "inv-c",
			PrincipalAmount: 100_000,
			InterestRate:    8.0,
			MaturityDate:    time.Date(2027, 1, 1, 0, 0, 0, 0, time.UTC),
			ValuationCap:    8_000_000,
			Discount:        15,
			IssueDate:       issueDate,
		},
	}

	trigger := ConversionTrigger{
		Type:               "qualified_financing",
		RoundPricePerShare: 2.00,
		PreMoneyShares:     10_000_000,
		ConversionDate:     conversionDate,
	}

	results, err := ConvertAll(safes, notes, trigger)
	if err != nil {
		t.Fatalf("ConvertAll: %v", err)
	}

	if len(results) != 3 {
		t.Fatalf("len(results) = %d, want 3", len(results))
	}

	// First SAFE: cap price = $10M/10M = $1.00, round price $2.00, no discount. Method = cap.
	if results[0].InstrumentID != "safe-a" {
		t.Fatalf("results[0].instrument_id = %q, want safe-a", results[0].InstrumentID)
	}
	if results[0].Method != "cap" {
		t.Fatalf("results[0].method = %q, want cap", results[0].Method)
	}
	if results[0].SharesIssued != 500_000 {
		t.Fatalf("results[0].shares = %d, want 500000", results[0].SharesIssued)
	}

	// Second SAFE: no cap, 20% discount. Discount price = $1.60.
	if results[1].InstrumentID != "safe-b" {
		t.Fatalf("results[1].instrument_id = %q, want safe-b", results[1].InstrumentID)
	}
	if results[1].Method != "discount" {
		t.Fatalf("results[1].method = %q, want discount", results[1].Method)
	}
	if !almostEqual(results[1].PricePerShare, 1.60, 0.001) {
		t.Fatalf("results[1].price = %f, want 1.60", results[1].PricePerShare)
	}

	// Note: cap price = $8M/10M = $0.80, discount price = $2.00 * 0.85 = $1.70. Cap wins.
	if results[2].InstrumentID != "note-a" {
		t.Fatalf("results[2].instrument_id = %q, want note-a", results[2].InstrumentID)
	}
	if results[2].InstrumentType != "note" {
		t.Fatalf("results[2].type = %q, want note", results[2].InstrumentType)
	}
	if results[2].Method != "cap" {
		t.Fatalf("results[2].method = %q, want cap", results[2].Method)
	}
	if !almostEqual(results[2].PricePerShare, 0.80, 0.001) {
		t.Fatalf("results[2].price = %f, want 0.80", results[2].PricePerShare)
	}
	// Converted amount should include interest.
	if results[2].ConvertedAmount <= results[2].OriginalInvestment {
		t.Fatalf("converted_amount %f should be > original %f (has accrued interest)",
			results[2].ConvertedAmount, results[2].OriginalInvestment)
	}
}

func TestConvertSAFEValidation(t *testing.T) {
	tests := []struct {
		name    string
		safe    SAFE
		trigger ConversionTrigger
		wantErr string
	}{
		{
			"zero investment",
			SAFE{InvestmentAmount: 0, ValuationCap: 1000},
			ConversionTrigger{RoundPricePerShare: 1.0, PreMoneyShares: 100},
			"investment_amount",
		},
		{
			"no price or cap",
			SAFE{InvestmentAmount: 1000, ValuationCap: 0, Discount: 0},
			ConversionTrigger{RoundPricePerShare: 0, PreMoneyShares: 100},
			"either round_price_per_share or valuation_cap",
		},
		{
			"post-money missing shares",
			SAFE{InvestmentAmount: 1000, ValuationCap: 5000, Type: "post_money"},
			ConversionTrigger{RoundPricePerShare: 1.0, PostMoneyShares: 0},
			"post_money_shares",
		},
		{
			"pre-money missing shares",
			SAFE{InvestmentAmount: 1000, ValuationCap: 5000, Type: "pre_money"},
			ConversionTrigger{RoundPricePerShare: 1.0, PreMoneyShares: 0},
			"pre_money_shares",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := ConvertSAFE(tt.safe, tt.trigger)
			if err == nil {
				t.Fatal("expected error")
			}
			if !strings.Contains(err.Error(), tt.wantErr) {
				t.Fatalf("error %q does not contain %q", err.Error(), tt.wantErr)
			}
		})
	}
}

func TestConvertNoteValidation(t *testing.T) {
	tests := []struct {
		name    string
		note    ConvertibleNote
		trigger ConversionTrigger
		wantErr string
	}{
		{
			"zero principal",
			ConvertibleNote{PrincipalAmount: 0, ValuationCap: 1000},
			ConversionTrigger{RoundPricePerShare: 1.0, PreMoneyShares: 100},
			"principal_amount",
		},
		{
			"no price or cap",
			ConvertibleNote{PrincipalAmount: 1000, ValuationCap: 0, Discount: 0},
			ConversionTrigger{RoundPricePerShare: 0, PreMoneyShares: 100},
			"either round_price_per_share or valuation_cap",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := ConvertNote(tt.note, tt.trigger)
			if err == nil {
				t.Fatal("expected error")
			}
			if !strings.Contains(err.Error(), tt.wantErr) {
				t.Fatalf("error %q does not contain %q", err.Error(), tt.wantErr)
			}
		})
	}
}
