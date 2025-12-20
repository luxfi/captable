package corporate

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"
)

type memRepo struct {
	mu    sync.RWMutex
	items map[string]*Action
}

func newMemRepo() *memRepo {
	return &memRepo{items: make(map[string]*Action)}
}

func (r *memRepo) CreateAction(_ context.Context, a *Action) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	cp := *a
	r.items[a.ID] = &cp
	return nil
}

func (r *memRepo) GetAction(_ context.Context, id string) (*Action, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	a, ok := r.items[id]
	if !ok {
		return nil, fmt.Errorf("action %s not found", id)
	}
	cp := *a
	return &cp, nil
}

func (r *memRepo) UpdateAction(_ context.Context, a *Action) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	cp := *a
	r.items[a.ID] = &cp
	return nil
}

func (r *memRepo) ListActions(_ context.Context, companyID string) ([]*Action, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	var out []*Action
	for _, a := range r.items {
		if a.CompanyID == companyID {
			cp := *a
			out = append(out, &cp)
		}
	}
	return out, nil
}

func TestProposeAction(t *testing.T) {
	svc := NewService(newMemRepo())
	ctx := context.Background()

	a := &Action{
		ID:            "ca1",
		CompanyID:     "c1",
		Type:          "stock_split",
		Description:   "2:1 stock split",
		EffectiveDate: time.Now().Add(30 * 24 * time.Hour),
	}
	if err := svc.ProposeAction(ctx, a); err != nil {
		t.Fatalf("ProposeAction: %v", err)
	}
	if a.Status != "proposed" {
		t.Fatalf("status = %q, want proposed", a.Status)
	}
}

func TestProposeActionValidation(t *testing.T) {
	svc := NewService(newMemRepo())
	ctx := context.Background()

	tests := []struct {
		name    string
		a       Action
		wantErr string
	}{
		{"missing company", Action{ID: "ca1", Type: "stock_split", EffectiveDate: time.Now()}, "company_id"},
		{"missing type", Action{ID: "ca1", CompanyID: "c1", EffectiveDate: time.Now()}, "type"},
		{"missing date", Action{ID: "ca1", CompanyID: "c1", Type: "stock_split"}, "effective_date"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := svc.ProposeAction(ctx, &tt.a)
			if err == nil {
				t.Fatal("expected error")
			}
			if !strings.Contains(err.Error(), tt.wantErr) {
				t.Fatalf("error %q does not contain %q", err.Error(), tt.wantErr)
			}
		})
	}
}

func TestApproveAndExecute(t *testing.T) {
	repo := newMemRepo()
	svc := NewService(repo)
	ctx := context.Background()

	repo.CreateAction(ctx, &Action{
		ID: "ca1", CompanyID: "c1", Type: "stock_split", Status: "proposed",
		EffectiveDate: time.Now(),
	})

	// Approve.
	if err := svc.ApproveAction(ctx, "ca1"); err != nil {
		t.Fatalf("ApproveAction: %v", err)
	}
	a, _ := repo.GetAction(ctx, "ca1")
	if a.Status != "approved" {
		t.Fatalf("status = %q, want approved", a.Status)
	}
	if a.ApprovedDate == nil {
		t.Fatal("approved_date should be set")
	}

	// Execute.
	if err := svc.ExecuteAction(ctx, "ca1"); err != nil {
		t.Fatalf("ExecuteAction: %v", err)
	}
	a, _ = repo.GetAction(ctx, "ca1")
	if a.Status != "executed" {
		t.Fatalf("status = %q, want executed", a.Status)
	}
}

func TestCannotApproveNonProposed(t *testing.T) {
	repo := newMemRepo()
	svc := NewService(repo)
	ctx := context.Background()

	repo.CreateAction(ctx, &Action{ID: "ca1", Status: "executed"})
	err := svc.ApproveAction(ctx, "ca1")
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestCannotExecuteNonApproved(t *testing.T) {
	repo := newMemRepo()
	svc := NewService(repo)
	ctx := context.Background()

	repo.CreateAction(ctx, &Action{ID: "ca1", Status: "proposed"})
	err := svc.ExecuteAction(ctx, "ca1")
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestCannotCancelExecuted(t *testing.T) {
	repo := newMemRepo()
	svc := NewService(repo)
	ctx := context.Background()

	repo.CreateAction(ctx, &Action{ID: "ca1", Status: "executed"})
	err := svc.CancelAction(ctx, "ca1")
	if err == nil {
		t.Fatal("expected error for cancelling executed action")
	}
}

func TestCancelProposed(t *testing.T) {
	repo := newMemRepo()
	svc := NewService(repo)
	ctx := context.Background()

	repo.CreateAction(ctx, &Action{ID: "ca1", CompanyID: "c1", Status: "proposed"})
	if err := svc.CancelAction(ctx, "ca1"); err != nil {
		t.Fatalf("CancelAction: %v", err)
	}
	a, _ := repo.GetAction(ctx, "ca1")
	if a.Status != "cancelled" {
		t.Fatalf("status = %q, want cancelled", a.Status)
	}
}

func TestCalculateSplit2to1(t *testing.T) {
	svc := NewService(newMemRepo())

	holdings := map[string]int64{
		"sh1": 10_000,
		"sh2": 5_000,
		"sh3": 1,
	}

	results, err := svc.CalculateSplit(&StockSplit{
		ShareClassID: "sc1",
		Ratio:        2.0,
	}, holdings)
	if err != nil {
		t.Fatalf("CalculateSplit: %v", err)
	}
	if len(results) != 3 {
		t.Fatalf("results len = %d, want 3", len(results))
	}

	found := make(map[string]SplitResult)
	for _, r := range results {
		found[r.StakeholderID] = r
	}

	if found["sh1"].SharesAfter != 20_000 {
		t.Fatalf("sh1 after = %d, want 20000", found["sh1"].SharesAfter)
	}
	if found["sh2"].SharesAfter != 10_000 {
		t.Fatalf("sh2 after = %d, want 10000", found["sh2"].SharesAfter)
	}
	if found["sh3"].SharesAfter != 2 {
		t.Fatalf("sh3 after = %d, want 2", found["sh3"].SharesAfter)
	}
}

func TestCalculateSplitFractional(t *testing.T) {
	svc := NewService(newMemRepo())

	holdings := map[string]int64{
		"sh1": 3, // 3 * 1.5 = 4.5 -> 4 shares + 0.5 fractional
	}

	results, err := svc.CalculateSplit(&StockSplit{
		ShareClassID: "sc1",
		Ratio:        1.5,
	}, holdings)
	if err != nil {
		t.Fatalf("CalculateSplit: %v", err)
	}
	if results[0].SharesAfter != 4 {
		t.Fatalf("shares_after = %d, want 4", results[0].SharesAfter)
	}
	if results[0].Fractional < 0.49 || results[0].Fractional > 0.51 {
		t.Fatalf("fractional = %f, want ~0.5", results[0].Fractional)
	}
}

func TestCalculateSplitZeroRatio(t *testing.T) {
	svc := NewService(newMemRepo())
	_, err := svc.CalculateSplit(&StockSplit{ShareClassID: "sc1", Ratio: 0}, map[string]int64{"sh1": 100})
	if err == nil {
		t.Fatal("expected error for zero ratio")
	}
}

func TestCalculateReverseSplit(t *testing.T) {
	svc := NewService(newMemRepo())

	results, err := svc.CalculateSplit(&StockSplit{
		ShareClassID: "sc1",
		Ratio:        0.1, // 1:10 reverse split
	}, map[string]int64{
		"sh1": 100_000,
	})
	if err != nil {
		t.Fatalf("CalculateSplit: %v", err)
	}
	if results[0].SharesAfter != 10_000 {
		t.Fatalf("shares_after = %d, want 10000", results[0].SharesAfter)
	}
}
