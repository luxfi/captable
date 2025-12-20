package compliance

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"
)

type memRepo struct {
	mu          sync.RWMutex
	formDs      map[string]*FormD
	blueSky     map[string]*BlueSkyFiling
}

func newMemRepo() *memRepo {
	return &memRepo{
		formDs:  make(map[string]*FormD),
		blueSky: make(map[string]*BlueSkyFiling),
	}
}

func (r *memRepo) CreateFormD(_ context.Context, f *FormD) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	cp := *f
	r.formDs[f.ID] = &cp
	return nil
}

func (r *memRepo) GetFormD(_ context.Context, id string) (*FormD, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	f, ok := r.formDs[id]
	if !ok {
		return nil, fmt.Errorf("form d %s not found", id)
	}
	cp := *f
	return &cp, nil
}

func (r *memRepo) UpdateFormD(_ context.Context, f *FormD) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	cp := *f
	r.formDs[f.ID] = &cp
	return nil
}

func (r *memRepo) ListFormDs(_ context.Context, companyID string) ([]*FormD, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	var out []*FormD
	for _, f := range r.formDs {
		if f.CompanyID == companyID {
			cp := *f
			out = append(out, &cp)
		}
	}
	return out, nil
}

func (r *memRepo) CreateBlueSkyFiling(_ context.Context, b *BlueSkyFiling) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	cp := *b
	r.blueSky[b.ID] = &cp
	return nil
}

func (r *memRepo) GetBlueSkyFiling(_ context.Context, id string) (*BlueSkyFiling, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	b, ok := r.blueSky[id]
	if !ok {
		return nil, fmt.Errorf("blue sky filing %s not found", id)
	}
	cp := *b
	return &cp, nil
}

func (r *memRepo) UpdateBlueSkyFiling(_ context.Context, b *BlueSkyFiling) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	cp := *b
	r.blueSky[b.ID] = &cp
	return nil
}

func (r *memRepo) ListBlueSkyFilings(_ context.Context, companyID string) ([]*BlueSkyFiling, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	var out []*BlueSkyFiling
	for _, b := range r.blueSky {
		if b.CompanyID == companyID {
			cp := *b
			out = append(out, &cp)
		}
	}
	return out, nil
}

func (r *memRepo) ListBlueSkyByState(_ context.Context, companyID, state string) ([]*BlueSkyFiling, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	var out []*BlueSkyFiling
	for _, b := range r.blueSky {
		if b.CompanyID == companyID && b.State == state {
			cp := *b
			out = append(out, &cp)
		}
	}
	return out, nil
}

func TestCheckRegD506bCompliant(t *testing.T) {
	svc := NewService(newMemRepo())
	result := svc.CheckRegDCompliance(&FormD{
		Exemption:        "506b",
		NumInvestors:     50,
		NumAccredited:    45,
		NumNonAccredited: 5,
		TotalOffering:    5_000_000,
	})
	if !result.Compliant {
		t.Fatalf("expected compliant, got violations: %v", result.Violations)
	}
}

func TestCheckRegD506bTooManyNonAccredited(t *testing.T) {
	svc := NewService(newMemRepo())
	result := svc.CheckRegDCompliance(&FormD{
		Exemption:        "506b",
		NumInvestors:     100,
		NumAccredited:    60,
		NumNonAccredited: 40,
		TotalOffering:    5_000_000,
	})
	if result.Compliant {
		t.Fatal("expected non-compliant")
	}
	found := false
	for _, v := range result.Violations {
		if strings.Contains(v, "35") {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected 35 limit violation, got: %v", result.Violations)
	}
}

func TestCheckRegD506bWarning(t *testing.T) {
	svc := NewService(newMemRepo())
	result := svc.CheckRegDCompliance(&FormD{
		Exemption:        "506b",
		NumNonAccredited: 32,
	})
	if !result.Compliant {
		t.Fatal("expected compliant")
	}
	if len(result.Warnings) == 0 {
		t.Fatal("expected warning for approaching limit")
	}
}

func TestCheckRegD506cNonAccredited(t *testing.T) {
	svc := NewService(newMemRepo())
	result := svc.CheckRegDCompliance(&FormD{
		Exemption:        "506c",
		NumNonAccredited: 1,
	})
	if result.Compliant {
		t.Fatal("expected non-compliant — 506(c) requires all accredited")
	}
}

func TestCheckRegD506cAllAccredited(t *testing.T) {
	svc := NewService(newMemRepo())
	result := svc.CheckRegDCompliance(&FormD{
		Exemption:        "506c",
		NumInvestors:     100,
		NumAccredited:    100,
		NumNonAccredited: 0,
	})
	if !result.Compliant {
		t.Fatalf("expected compliant, got: %v", result.Violations)
	}
}

func TestCheckRegD504Exceeds10M(t *testing.T) {
	svc := NewService(newMemRepo())
	result := svc.CheckRegDCompliance(&FormD{
		Exemption:     "504",
		TotalOffering: 15_000_000,
	})
	if result.Compliant {
		t.Fatal("expected non-compliant — 504 max is $10M")
	}
}

func TestCheckRegDFormDFilingDeadline(t *testing.T) {
	svc := NewService(newMemRepo())
	pastSale := time.Now().Add(-20 * 24 * time.Hour)
	result := svc.CheckRegDCompliance(&FormD{
		Exemption:     "506b",
		FirstSaleDate: &pastSale,
		// No filing date — overdue
	})
	if result.Compliant {
		t.Fatal("expected non-compliant — Form D filing overdue")
	}
	found := false
	for _, v := range result.Violations {
		if strings.Contains(v, "15 days") {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected 15-day deadline violation, got: %v", result.Violations)
	}
}

func TestCreateFormDValidation(t *testing.T) {
	svc := NewService(newMemRepo())
	ctx := context.Background()

	tests := []struct {
		name    string
		f       FormD
		wantErr string
	}{
		{"missing company", FormD{ID: "f1", Exemption: "506b", TotalOffering: 1000}, "company_id"},
		{"missing exemption", FormD{ID: "f1", CompanyID: "c1", TotalOffering: 1000}, "exemption"},
		{"zero offering", FormD{ID: "f1", CompanyID: "c1", Exemption: "506b"}, "total_offering"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := svc.CreateFormD(ctx, &tt.f)
			if err == nil {
				t.Fatal("expected error")
			}
			if !strings.Contains(err.Error(), tt.wantErr) {
				t.Fatalf("error %q does not contain %q", err.Error(), tt.wantErr)
			}
		})
	}
}

func TestCreateFormDSuccess(t *testing.T) {
	svc := NewService(newMemRepo())
	ctx := context.Background()

	f := &FormD{
		ID:            "f1",
		CompanyID:     "c1",
		Exemption:     "506b",
		TotalOffering: 5_000_000,
		TotalSold:     1_000_000,
	}
	if err := svc.CreateFormD(ctx, f); err != nil {
		t.Fatalf("CreateFormD: %v", err)
	}
	if f.TotalRemaining != 4_000_000 {
		t.Fatalf("total_remaining = %f, want 4000000", f.TotalRemaining)
	}
	if f.Status != "draft" {
		t.Fatalf("status = %q, want draft", f.Status)
	}
}

func TestGetRequiredStates(t *testing.T) {
	svc := NewService(newMemRepo())
	states := svc.GetRequiredStates()
	if len(states) != 51 { // 50 states + DC
		t.Fatalf("states len = %d, want 51", len(states))
	}
}
