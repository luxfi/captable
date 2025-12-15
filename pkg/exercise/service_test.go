package exercise

import (
	"context"
	"fmt"
	"math"
	"strings"
	"sync"
	"testing"
	"time"
)

// memRepo is an in-memory Repository for testing.
type memRepo struct {
	mu      sync.RWMutex
	results map[string]*ExerciseResult
}

func newMemRepo() *memRepo {
	return &memRepo{results: make(map[string]*ExerciseResult)}
}

func (r *memRepo) CreateResult(_ context.Context, res *ExerciseResult) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	cp := *res
	r.results[res.ID] = &cp
	return nil
}

func (r *memRepo) GetResult(_ context.Context, id string) (*ExerciseResult, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	res, ok := r.results[id]
	if !ok {
		return nil, fmt.Errorf("exercise result %s not found", id)
	}
	cp := *res
	return &cp, nil
}

func (r *memRepo) ListResults(_ context.Context, grantID string) ([]*ExerciseResult, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	var out []*ExerciseResult
	for _, res := range r.results {
		if res.GrantID == grantID {
			cp := *res
			out = append(out, &cp)
		}
	}
	return out, nil
}

// memGrantProvider is an in-memory GrantProvider for testing.
type memGrantProvider struct {
	mu     sync.RWMutex
	grants map[string]*GrantInfo
}

func newMemGrantProvider() *memGrantProvider {
	return &memGrantProvider{grants: make(map[string]*GrantInfo)}
}

func (p *memGrantProvider) addGrant(g *GrantInfo) {
	p.mu.Lock()
	defer p.mu.Unlock()
	cp := *g
	p.grants[g.ID] = &cp
}

func (p *memGrantProvider) GetGrant(_ context.Context, grantID string) (*GrantInfo, error) {
	p.mu.RLock()
	defer p.mu.RUnlock()
	g, ok := p.grants[grantID]
	if !ok {
		return nil, fmt.Errorf("grant %s not found", grantID)
	}
	cp := *g
	return &cp, nil
}

func (p *memGrantProvider) UpdateExercised(_ context.Context, grantID string, additionalExercised int64) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	g, ok := p.grants[grantID]
	if !ok {
		return fmt.Errorf("grant %s not found", grantID)
	}
	g.OptionsExercised += additionalExercised
	return nil
}

func TestNSOExerciseWithTax(t *testing.T) {
	repo := newMemRepo()
	grants := newMemGrantProvider()
	svc := NewService(repo, grants)
	ctx := context.Background()

	grants.addGrant(&GrantInfo{
		ID:               "g1",
		StakeholderID:    "sh1",
		OptionsGranted:   10000,
		OptionsExercised: 0,
		OptionsCancelled: 0,
		StrikePrice:      1.00,
		Type:             "nso",
	})

	req := &ExerciseRequest{
		GrantID:         "g1",
		StakeholderID:   "sh1",
		SharesExercised: 5000,
		ExerciseDate:    time.Date(2025, 6, 15, 0, 0, 0, 0, time.UTC),
		ExerciseType:    "cash",
		PaymentMethod:   "wire",
		CurrentFMV:      "5.00",
	}

	result, err := svc.Exercise(ctx, req)
	if err != nil {
		t.Fatalf("Exercise: %v", err)
	}

	// exercise_price = 5000 * 1.00 = 5000
	if result.ExercisePrice != 5000.0 {
		t.Fatalf("exercise_price = %f, want 5000", result.ExercisePrice)
	}

	// spread = (5.00 - 1.00) * 5000 = 20000
	if result.TaxableSpread != 20000.0 {
		t.Fatalf("taxable_spread = %f, want 20000", result.TaxableSpread)
	}

	// NSO: AMT adjustment should be 0.
	if result.AMTAdjustment != 0 {
		t.Fatalf("amt_adjustment = %f, want 0 for NSO", result.AMTAdjustment)
	}

	// NSO: tax withholding > 0.
	if result.TaxWithholding <= 0 {
		t.Fatal("expected positive tax withholding for NSO")
	}

	// Verify tax calculation.
	expectedFederal := 20000.0 * FederalRate
	expectedState := 20000.0 * StateRateCA
	expectedSS := 20000.0 * SocialSecurityRate
	expectedMedicare := 20000.0 * MedicareRate
	expectedTotal := expectedFederal + expectedState + expectedSS + expectedMedicare

	if math.Abs(result.TaxWithholding-expectedTotal) > 0.01 {
		t.Fatalf("tax_withholding = %f, want %f", result.TaxWithholding, expectedTotal)
	}

	// Net shares = all shares for cash exercise.
	if result.NetSharesIssued != 5000 {
		t.Fatalf("net_shares_issued = %d, want 5000", result.NetSharesIssued)
	}

	// 83(b) deadline = exercise_date + 30 days.
	expectedDeadline := time.Date(2025, 7, 15, 0, 0, 0, 0, time.UTC)
	if !result.Section83bDeadline.Equal(expectedDeadline) {
		t.Fatalf("section_83b_deadline = %v, want %v", result.Section83bDeadline, expectedDeadline)
	}
}

func TestISOExercise(t *testing.T) {
	repo := newMemRepo()
	grants := newMemGrantProvider()
	svc := NewService(repo, grants)
	ctx := context.Background()

	grants.addGrant(&GrantInfo{
		ID:               "g1",
		StakeholderID:    "sh1",
		OptionsGranted:   10000,
		OptionsExercised: 0,
		OptionsCancelled: 0,
		StrikePrice:      2.00,
		Type:             "iso",
	})

	req := &ExerciseRequest{
		GrantID:         "g1",
		StakeholderID:   "sh1",
		SharesExercised: 10000,
		ExerciseDate:    time.Date(2025, 6, 15, 0, 0, 0, 0, time.UTC),
		ExerciseType:    "cash",
		PaymentMethod:   "wire",
		CurrentFMV:      "10.00",
	}

	result, err := svc.Exercise(ctx, req)
	if err != nil {
		t.Fatalf("Exercise: %v", err)
	}

	// exercise_price = 10000 * 2.00 = 20000
	if result.ExercisePrice != 20000.0 {
		t.Fatalf("exercise_price = %f, want 20000", result.ExercisePrice)
	}

	// spread = (10.00 - 2.00) * 10000 = 80000
	if result.TaxableSpread != 80000.0 {
		t.Fatalf("taxable_spread = %f, want 80000", result.TaxableSpread)
	}

	// ISO: no ordinary income, no withholding.
	if result.TaxWithholding != 0 {
		t.Fatalf("tax_withholding = %f, want 0 for ISO", result.TaxWithholding)
	}

	// ISO: AMT adjustment = spread.
	if result.AMTAdjustment != 80000.0 {
		t.Fatalf("amt_adjustment = %f, want 80000", result.AMTAdjustment)
	}
}

func TestPartialExercise(t *testing.T) {
	repo := newMemRepo()
	grants := newMemGrantProvider()
	svc := NewService(repo, grants)
	ctx := context.Background()

	grants.addGrant(&GrantInfo{
		ID:               "g1",
		StakeholderID:    "sh1",
		OptionsGranted:   100,
		OptionsExercised: 0,
		OptionsCancelled: 0,
		StrikePrice:      1.00,
		Type:             "nso",
	})

	// Exercise 50 of 100.
	req := &ExerciseRequest{
		GrantID:         "g1",
		StakeholderID:   "sh1",
		SharesExercised: 50,
		ExerciseDate:    time.Date(2025, 6, 15, 0, 0, 0, 0, time.UTC),
		ExerciseType:    "cash",
		PaymentMethod:   "ach",
		CurrentFMV:      "3.00",
	}

	result, err := svc.Exercise(ctx, req)
	if err != nil {
		t.Fatalf("Exercise: %v", err)
	}

	if result.SharesExercised != 50 {
		t.Fatalf("shares_exercised = %d, want 50", result.SharesExercised)
	}
	if result.ExercisePrice != 50.0 {
		t.Fatalf("exercise_price = %f, want 50", result.ExercisePrice)
	}

	// Verify grant was updated: now 50 exercised.
	g, _ := grants.GetGrant(ctx, "g1")
	if g.OptionsExercised != 50 {
		t.Fatalf("grant options_exercised = %d, want 50", g.OptionsExercised)
	}

	// Exercise remaining 50.
	req2 := &ExerciseRequest{
		GrantID:         "g1",
		StakeholderID:   "sh1",
		SharesExercised: 50,
		ExerciseDate:    time.Date(2025, 7, 15, 0, 0, 0, 0, time.UTC),
		ExerciseType:    "cash",
		PaymentMethod:   "ach",
		CurrentFMV:      "4.00",
	}

	result2, err := svc.Exercise(ctx, req2)
	if err != nil {
		t.Fatalf("Exercise second batch: %v", err)
	}
	if result2.SharesExercised != 50 {
		t.Fatalf("second batch shares_exercised = %d, want 50", result2.SharesExercised)
	}
}

func TestInsufficientShares(t *testing.T) {
	repo := newMemRepo()
	grants := newMemGrantProvider()
	svc := NewService(repo, grants)
	ctx := context.Background()

	grants.addGrant(&GrantInfo{
		ID:               "g1",
		StakeholderID:    "sh1",
		OptionsGranted:   100,
		OptionsExercised: 80,
		OptionsCancelled: 10,
		StrikePrice:      1.00,
		Type:             "nso",
	})

	// Only 10 available (100 - 80 - 10), trying to exercise 20.
	req := &ExerciseRequest{
		GrantID:         "g1",
		StakeholderID:   "sh1",
		SharesExercised: 20,
		ExerciseDate:    time.Date(2025, 6, 15, 0, 0, 0, 0, time.UTC),
		ExerciseType:    "cash",
		PaymentMethod:   "wire",
		CurrentFMV:      "5.00",
	}

	_, err := svc.Exercise(ctx, req)
	if err == nil {
		t.Fatal("expected error for insufficient shares")
	}
	if !strings.Contains(err.Error(), "insufficient shares") {
		t.Fatalf("error %q does not mention insufficient shares", err.Error())
	}
}

func TestSection83bDeadline(t *testing.T) {
	repo := newMemRepo()
	grants := newMemGrantProvider()
	svc := NewService(repo, grants)
	ctx := context.Background()

	grants.addGrant(&GrantInfo{
		ID:               "g1",
		StakeholderID:    "sh1",
		OptionsGranted:   1000,
		OptionsExercised: 0,
		OptionsCancelled: 0,
		StrikePrice:      0.50,
		Type:             "iso",
	})

	exerciseDate := time.Date(2025, 3, 1, 0, 0, 0, 0, time.UTC)
	req := &ExerciseRequest{
		GrantID:         "g1",
		StakeholderID:   "sh1",
		SharesExercised: 1000,
		ExerciseDate:    exerciseDate,
		ExerciseType:    "cash",
		PaymentMethod:   "check",
		CurrentFMV:      "2.00",
	}

	result, err := svc.Exercise(ctx, req)
	if err != nil {
		t.Fatalf("Exercise: %v", err)
	}

	expectedDeadline := time.Date(2025, 3, 31, 0, 0, 0, 0, time.UTC)
	if !result.Section83bDeadline.Equal(expectedDeadline) {
		t.Fatalf("section_83b_deadline = %v, want %v", result.Section83bDeadline, expectedDeadline)
	}
}

func TestCashlessExercise(t *testing.T) {
	repo := newMemRepo()
	grants := newMemGrantProvider()
	svc := NewService(repo, grants)
	ctx := context.Background()

	grants.addGrant(&GrantInfo{
		ID:               "g1",
		StakeholderID:    "sh1",
		OptionsGranted:   1000,
		OptionsExercised: 0,
		OptionsCancelled: 0,
		StrikePrice:      2.00,
		Type:             "nso",
	})

	req := &ExerciseRequest{
		GrantID:         "g1",
		StakeholderID:   "sh1",
		SharesExercised: 1000,
		ExerciseDate:    time.Date(2025, 6, 15, 0, 0, 0, 0, time.UTC),
		ExerciseType:    "cashless",
		PaymentMethod:   "stock_swap",
		CurrentFMV:      "10.00",
	}

	result, err := svc.Exercise(ctx, req)
	if err != nil {
		t.Fatalf("Exercise: %v", err)
	}

	// Cashless: net_shares = (FMV - strike) * shares / FMV = (10 - 2) * 1000 / 10 = 800.
	if result.NetSharesIssued != 800 {
		t.Fatalf("net_shares_issued = %d, want 800", result.NetSharesIssued)
	}

	// No cash outlay for cashless exercise.
	if result.TotalCost != 0 {
		t.Fatalf("total_cost = %f, want 0 for cashless", result.TotalCost)
	}
}

func TestCalculateTaxNSO(t *testing.T) {
	svc := NewService(newMemRepo(), newMemGrantProvider())

	spread := 50000.0
	tax := svc.CalculateTax("nso", spread, 5000)

	if tax.OrdinaryIncome != spread {
		t.Fatalf("ordinary_income = %f, want %f", tax.OrdinaryIncome, spread)
	}
	if tax.AMTPreferenceItem != 0 {
		t.Fatalf("amt_preference_item = %f, want 0 for NSO", tax.AMTPreferenceItem)
	}

	expectedFederal := spread * FederalRate
	if math.Abs(tax.FederalWithholding-expectedFederal) > 0.01 {
		t.Fatalf("federal = %f, want %f", tax.FederalWithholding, expectedFederal)
	}

	expectedState := spread * StateRateCA
	if math.Abs(tax.StateWithholding-expectedState) > 0.01 {
		t.Fatalf("state = %f, want %f", tax.StateWithholding, expectedState)
	}

	expectedSS := spread * SocialSecurityRate
	if math.Abs(tax.SocialSecurityWithholding-expectedSS) > 0.01 {
		t.Fatalf("social_security = %f, want %f", tax.SocialSecurityWithholding, expectedSS)
	}

	expectedMedicare := spread * MedicareRate
	if math.Abs(tax.MedicareWithholding-expectedMedicare) > 0.01 {
		t.Fatalf("medicare = %f, want %f", tax.MedicareWithholding, expectedMedicare)
	}

	expectedTotal := expectedFederal + expectedState + expectedSS + expectedMedicare
	if math.Abs(tax.TotalWithholding-expectedTotal) > 0.01 {
		t.Fatalf("total = %f, want %f", tax.TotalWithholding, expectedTotal)
	}
}

func TestCalculateTaxISO(t *testing.T) {
	svc := NewService(newMemRepo(), newMemGrantProvider())

	spread := 80000.0
	tax := svc.CalculateTax("iso", spread, 10000)

	if tax.OrdinaryIncome != 0 {
		t.Fatalf("ordinary_income = %f, want 0 for ISO", tax.OrdinaryIncome)
	}
	if tax.AMTPreferenceItem != spread {
		t.Fatalf("amt_preference_item = %f, want %f", tax.AMTPreferenceItem, spread)
	}
	if tax.TotalWithholding != 0 {
		t.Fatalf("total_withholding = %f, want 0 for ISO", tax.TotalWithholding)
	}
}

func TestEarlyExercise(t *testing.T) {
	repo := newMemRepo()
	grants := newMemGrantProvider()
	svc := NewService(repo, grants)
	ctx := context.Background()

	grants.addGrant(&GrantInfo{
		ID:               "g1",
		StakeholderID:    "sh1",
		OptionsGranted:   5000,
		OptionsExercised: 0,
		OptionsCancelled: 0,
		StrikePrice:      0.10,
		Type:             "iso",
	})

	exerciseDate := time.Date(2025, 1, 15, 0, 0, 0, 0, time.UTC)
	req := &ExerciseRequest{
		GrantID:         "g1",
		StakeholderID:   "sh1",
		SharesExercised: 5000,
		ExerciseDate:    exerciseDate,
		ExerciseType:    "cash",
		PaymentMethod:   "wire",
		CurrentFMV:      "0.10", // at FMV, no spread
	}

	result, err := svc.EarlyExercise(ctx, req)
	if err != nil {
		t.Fatalf("EarlyExercise: %v", err)
	}

	if !result.Section83bElection {
		t.Fatal("section_83b_election should be true for early exercise")
	}

	expectedDeadline := time.Date(2025, 2, 14, 0, 0, 0, 0, time.UTC)
	if !result.Section83bDeadline.Equal(expectedDeadline) {
		t.Fatalf("section_83b_deadline = %v, want %v", result.Section83bDeadline, expectedDeadline)
	}

	// At-FMV exercise: spread = 0.
	if result.TaxableSpread != 0 {
		t.Fatalf("taxable_spread = %f, want 0", result.TaxableSpread)
	}
}

func TestExerciseValidation(t *testing.T) {
	svc := NewService(newMemRepo(), newMemGrantProvider())
	ctx := context.Background()

	tests := []struct {
		name    string
		req     ExerciseRequest
		wantErr string
	}{
		{"missing grant", ExerciseRequest{StakeholderID: "sh1", SharesExercised: 100, ExerciseDate: time.Now(), ExerciseType: "cash", CurrentFMV: "5.00"}, "grant_id"},
		{"missing stakeholder", ExerciseRequest{GrantID: "g1", SharesExercised: 100, ExerciseDate: time.Now(), ExerciseType: "cash", CurrentFMV: "5.00"}, "stakeholder_id"},
		{"zero shares", ExerciseRequest{GrantID: "g1", StakeholderID: "sh1", SharesExercised: 0, ExerciseDate: time.Now(), ExerciseType: "cash", CurrentFMV: "5.00"}, "shares_exercised"},
		{"missing date", ExerciseRequest{GrantID: "g1", StakeholderID: "sh1", SharesExercised: 100, ExerciseType: "cash", CurrentFMV: "5.00"}, "exercise_date"},
		{"missing type", ExerciseRequest{GrantID: "g1", StakeholderID: "sh1", SharesExercised: 100, ExerciseDate: time.Now(), CurrentFMV: "5.00"}, "exercise_type"},
		{"missing fmv", ExerciseRequest{GrantID: "g1", StakeholderID: "sh1", SharesExercised: 100, ExerciseDate: time.Now(), ExerciseType: "cash"}, "current_fmv"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := svc.Exercise(ctx, &tt.req)
			if err == nil {
				t.Fatal("expected error")
			}
			if !strings.Contains(err.Error(), tt.wantErr) {
				t.Fatalf("error %q does not contain %q", err.Error(), tt.wantErr)
			}
		})
	}
}

func TestStakeholderMismatch(t *testing.T) {
	repo := newMemRepo()
	grants := newMemGrantProvider()
	svc := NewService(repo, grants)
	ctx := context.Background()

	grants.addGrant(&GrantInfo{
		ID:               "g1",
		StakeholderID:    "sh1",
		OptionsGranted:   1000,
		OptionsExercised: 0,
		OptionsCancelled: 0,
		StrikePrice:      1.00,
		Type:             "nso",
	})

	req := &ExerciseRequest{
		GrantID:         "g1",
		StakeholderID:   "sh_wrong",
		SharesExercised: 100,
		ExerciseDate:    time.Date(2025, 6, 15, 0, 0, 0, 0, time.UTC),
		ExerciseType:    "cash",
		PaymentMethod:   "wire",
		CurrentFMV:      "5.00",
	}

	_, err := svc.Exercise(ctx, req)
	if err == nil {
		t.Fatal("expected error for stakeholder mismatch")
	}
	if !strings.Contains(err.Error(), "does not own") {
		t.Fatalf("error %q does not mention ownership mismatch", err.Error())
	}
}
