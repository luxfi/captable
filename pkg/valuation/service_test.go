package valuation

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"sync"
	"testing"
	"time"
)

// memRepo is an in-memory Repository for testing.
type memRepo struct {
	mu   sync.RWMutex
	vals map[string]*Valuation409A
}

func newMemRepo() *memRepo {
	return &memRepo{vals: make(map[string]*Valuation409A)}
}

func (r *memRepo) Create(_ context.Context, v *Valuation409A) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, exists := r.vals[v.ID]; exists {
		return fmt.Errorf("valuation %s already exists", v.ID)
	}
	cp := *v
	r.vals[v.ID] = &cp
	return nil
}

func (r *memRepo) Get(_ context.Context, id string) (*Valuation409A, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	v, ok := r.vals[id]
	if !ok {
		return nil, fmt.Errorf("valuation %s not found", id)
	}
	cp := *v
	return &cp, nil
}

func (r *memRepo) GetLatest(_ context.Context, companyID, shareClassID string) (*Valuation409A, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	var latest *Valuation409A
	for _, v := range r.vals {
		if v.CompanyID == companyID && v.ShareClassID == shareClassID && v.Status != "expired" {
			if latest == nil || v.EffectiveDate.After(latest.EffectiveDate) {
				cp := *v
				latest = &cp
			}
		}
	}
	if latest == nil {
		return nil, fmt.Errorf("no valuation found for company %s share class %s", companyID, shareClassID)
	}
	return latest, nil
}

func (r *memRepo) List(_ context.Context, companyID string) ([]*Valuation409A, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	var out []*Valuation409A
	for _, v := range r.vals {
		if v.CompanyID == companyID {
			cp := *v
			out = append(out, &cp)
		}
	}
	return out, nil
}

func (r *memRepo) Update(_ context.Context, v *Valuation409A) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, exists := r.vals[v.ID]; !exists {
		return fmt.Errorf("valuation %s not found", v.ID)
	}
	cp := *v
	r.vals[v.ID] = &cp
	return nil
}

func TestCreateValuationSuccess(t *testing.T) {
	svc := NewService(newMemRepo())
	ctx := context.Background()

	v := &Valuation409A{
		ID:              "v1",
		CompanyID:       "c1",
		ShareClassID:    "sc1",
		EffectiveDate:   time.Now().UTC(),
		FairMarketValue: "1.25",
		Method:          "dcf",
		Provider:        "ValuCo",
		IsSafeHarbor:    true,
	}
	if err := svc.CreateValuation(ctx, v); err != nil {
		t.Fatalf("CreateValuation: %v", err)
	}
	if v.Status != "draft" {
		t.Fatalf("status = %q, want draft", v.Status)
	}
	if v.CreatedAt.IsZero() {
		t.Fatal("created_at should be set")
	}
	if v.ExpirationDate.IsZero() {
		t.Fatal("expiration_date should default to 12 months from effective date")
	}
	// Verify expiration is ~12 months from effective date.
	expected := v.EffectiveDate.AddDate(0, 12, 0)
	if !v.ExpirationDate.Equal(expected) {
		t.Fatalf("expiration_date = %v, want %v", v.ExpirationDate, expected)
	}
}

func TestCreateValuationValidation(t *testing.T) {
	svc := NewService(newMemRepo())
	ctx := context.Background()

	tests := []struct {
		name    string
		val     Valuation409A
		wantErr string
	}{
		{"missing company", Valuation409A{ID: "v1", ShareClassID: "sc1", EffectiveDate: time.Now(), FairMarketValue: "1.00", Method: "dcf"}, "company_id"},
		{"missing share class", Valuation409A{ID: "v1", CompanyID: "c1", EffectiveDate: time.Now(), FairMarketValue: "1.00", Method: "dcf"}, "share_class_id"},
		{"missing effective date", Valuation409A{ID: "v1", CompanyID: "c1", ShareClassID: "sc1", FairMarketValue: "1.00", Method: "dcf"}, "effective_date"},
		{"missing fmv", Valuation409A{ID: "v1", CompanyID: "c1", ShareClassID: "sc1", EffectiveDate: time.Now(), Method: "dcf"}, "fair_market_value"},
		{"zero fmv", Valuation409A{ID: "v1", CompanyID: "c1", ShareClassID: "sc1", EffectiveDate: time.Now(), FairMarketValue: "0", Method: "dcf"}, "positive number"},
		{"negative fmv", Valuation409A{ID: "v1", CompanyID: "c1", ShareClassID: "sc1", EffectiveDate: time.Now(), FairMarketValue: "-5.00", Method: "dcf"}, "positive number"},
		{"invalid fmv", Valuation409A{ID: "v1", CompanyID: "c1", ShareClassID: "sc1", EffectiveDate: time.Now(), FairMarketValue: "abc", Method: "dcf"}, "positive number"},
		{"missing method", Valuation409A{ID: "v1", CompanyID: "c1", ShareClassID: "sc1", EffectiveDate: time.Now(), FairMarketValue: "1.00"}, "method"},
		{"invalid method", Valuation409A{ID: "v1", CompanyID: "c1", ShareClassID: "sc1", EffectiveDate: time.Now(), FairMarketValue: "1.00", Method: "guessing"}, "method must be one of"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := svc.CreateValuation(ctx, &tt.val)
			if err == nil {
				t.Fatal("expected error")
			}
			if !strings.Contains(err.Error(), tt.wantErr) {
				t.Fatalf("error %q does not contain %q", err.Error(), tt.wantErr)
			}
		})
	}
}

func TestGetCurrentFMV(t *testing.T) {
	repo := newMemRepo()
	svc := NewService(repo)
	ctx := context.Background()

	// Create a current (non-expired) valuation.
	now := time.Now().UTC()
	v := &Valuation409A{
		ID:              "v1",
		CompanyID:       "c1",
		ShareClassID:    "sc1",
		EffectiveDate:   now.AddDate(0, -3, 0), // 3 months ago
		ExpirationDate:  now.AddDate(0, 9, 0),  // 9 months from now
		FairMarketValue: "2.50",
		Method:          "dcf",
		Status:          "final",
	}
	if err := repo.Create(ctx, v); err != nil {
		t.Fatal(err)
	}

	fmv, err := svc.GetCurrentFMV(ctx, "c1", "sc1")
	if err != nil {
		t.Fatalf("GetCurrentFMV: %v", err)
	}
	if fmv != "2.50" {
		t.Fatalf("fmv = %q, want 2.50", fmv)
	}
}

func TestGetCurrentFMVExpired(t *testing.T) {
	repo := newMemRepo()
	svc := NewService(repo)
	ctx := context.Background()

	// Create an expired valuation.
	now := time.Now().UTC()
	v := &Valuation409A{
		ID:              "v1",
		CompanyID:       "c1",
		ShareClassID:    "sc1",
		EffectiveDate:   now.AddDate(-2, 0, 0), // 2 years ago
		ExpirationDate:  now.AddDate(-1, 0, 0), // 1 year ago
		FairMarketValue: "1.00",
		Method:          "dcf",
		Status:          "final",
	}
	if err := repo.Create(ctx, v); err != nil {
		t.Fatal(err)
	}

	_, err := svc.GetCurrentFMV(ctx, "c1", "sc1")
	if err == nil {
		t.Fatal("expected error for expired valuation")
	}
	if !strings.Contains(err.Error(), "expired") {
		t.Fatalf("error %q does not mention expired", err.Error())
	}
}

func TestIsExpired(t *testing.T) {
	svc := NewService(newMemRepo())
	now := time.Now().UTC()

	tests := []struct {
		name    string
		val     Valuation409A
		expired bool
	}{
		{
			"current valuation",
			Valuation409A{EffectiveDate: now.AddDate(0, -6, 0), ExpirationDate: now.AddDate(0, 6, 0)},
			false,
		},
		{
			"expired by expiration date",
			Valuation409A{EffectiveDate: now.AddDate(-2, 0, 0), ExpirationDate: now.AddDate(-1, 0, 0)},
			true,
		},
		{
			"expired by 12 month rule (no explicit expiration)",
			Valuation409A{EffectiveDate: now.AddDate(-1, -1, 0)},
			true,
		},
		{
			"not expired within 12 months (no explicit expiration)",
			Valuation409A{EffectiveDate: now.AddDate(0, -11, 0)},
			false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := svc.IsExpired(&tt.val)
			if got != tt.expired {
				t.Fatalf("IsExpired = %v, want %v", got, tt.expired)
			}
		})
	}
}

func TestListHistoryOrdering(t *testing.T) {
	repo := newMemRepo()
	svc := NewService(repo)
	ctx := context.Background()

	now := time.Now().UTC()
	// Insert in non-chronological order.
	dates := []time.Time{
		now.AddDate(0, -6, 0),
		now.AddDate(0, -1, 0),
		now.AddDate(-1, 0, 0),
		now.AddDate(0, -3, 0),
	}
	for i, d := range dates {
		v := &Valuation409A{
			ID:              fmt.Sprintf("v%d", i),
			CompanyID:       "c1",
			ShareClassID:    "sc1",
			EffectiveDate:   d,
			ExpirationDate:  d.AddDate(0, 12, 0),
			FairMarketValue: fmt.Sprintf("%d.00", i+1),
			Method:          "dcf",
			Status:          "final",
		}
		if err := repo.Create(ctx, v); err != nil {
			t.Fatal(err)
		}
	}

	history, err := svc.ListHistory(ctx, "c1")
	if err != nil {
		t.Fatalf("ListHistory: %v", err)
	}
	if len(history.Valuations) != 4 {
		t.Fatalf("len = %d, want 4", len(history.Valuations))
	}

	// Verify descending order by effective date.
	if !sort.SliceIsSorted(history.Valuations, func(i, j int) bool {
		return history.Valuations[i].EffectiveDate.After(history.Valuations[j].EffectiveDate)
	}) {
		t.Fatal("history is not sorted by effective date descending")
	}

	// Most recent should be dates[1] (1 month ago).
	if history.Valuations[0].ID != "v1" {
		t.Fatalf("first entry ID = %q, want v1 (most recent)", history.Valuations[0].ID)
	}
}

func TestListHistoryEmpty(t *testing.T) {
	svc := NewService(newMemRepo())
	ctx := context.Background()

	history, err := svc.ListHistory(ctx, "c_nonexistent")
	if err != nil {
		t.Fatalf("ListHistory: %v", err)
	}
	if len(history.Valuations) != 0 {
		t.Fatalf("len = %d, want 0", len(history.Valuations))
	}
}

func TestCreateValuationAllMethods(t *testing.T) {
	svc := NewService(newMemRepo())
	ctx := context.Background()

	methods := []string{"dcf", "market_comparable", "asset_based", "backsolve", "opm"}
	for _, m := range methods {
		v := &Valuation409A{
			ID:              "v_" + m,
			CompanyID:       "c1",
			ShareClassID:    "sc1",
			EffectiveDate:   time.Now().UTC(),
			FairMarketValue: "5.00",
			Method:          m,
		}
		if err := svc.CreateValuation(ctx, v); err != nil {
			t.Fatalf("CreateValuation with method %s: %v", m, err)
		}
	}
}
