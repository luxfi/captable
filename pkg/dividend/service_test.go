package dividend

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"
)

type memRepo struct {
	mu            sync.RWMutex
	declarations  map[string]*Declaration
	distributions map[string]*Distribution
}

func newMemRepo() *memRepo {
	return &memRepo{
		declarations:  make(map[string]*Declaration),
		distributions: make(map[string]*Distribution),
	}
}

func (r *memRepo) CreateDeclaration(_ context.Context, d *Declaration) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	cp := *d
	r.declarations[d.ID] = &cp
	return nil
}

func (r *memRepo) GetDeclaration(_ context.Context, id string) (*Declaration, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	d, ok := r.declarations[id]
	if !ok {
		return nil, fmt.Errorf("declaration %s not found", id)
	}
	cp := *d
	return &cp, nil
}

func (r *memRepo) UpdateDeclaration(_ context.Context, d *Declaration) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	cp := *d
	r.declarations[d.ID] = &cp
	return nil
}

func (r *memRepo) ListDeclarations(_ context.Context, companyID string) ([]*Declaration, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	var out []*Declaration
	for _, d := range r.declarations {
		if d.CompanyID == companyID {
			cp := *d
			out = append(out, &cp)
		}
	}
	return out, nil
}

func (r *memRepo) CreateDistribution(_ context.Context, d *Distribution) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	cp := *d
	r.distributions[d.ID] = &cp
	return nil
}

func (r *memRepo) GetDistribution(_ context.Context, id string) (*Distribution, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	d, ok := r.distributions[id]
	if !ok {
		return nil, fmt.Errorf("distribution %s not found", id)
	}
	cp := *d
	return &cp, nil
}

func (r *memRepo) UpdateDistribution(_ context.Context, d *Distribution) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	cp := *d
	r.distributions[d.ID] = &cp
	return nil
}

func (r *memRepo) ListDistributions(_ context.Context, declarationID string) ([]*Distribution, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	var out []*Distribution
	for _, d := range r.distributions {
		if d.DeclarationID == declarationID {
			cp := *d
			out = append(out, &cp)
		}
	}
	return out, nil
}

func (r *memRepo) ListByStakeholder(_ context.Context, stakeholderID string) ([]*Distribution, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	var out []*Distribution
	for _, d := range r.distributions {
		if d.StakeholderID == stakeholderID {
			cp := *d
			out = append(out, &cp)
		}
	}
	return out, nil
}

// stubHoldings implements HoldingsProvider for tests.
type stubHoldings struct {
	data map[string]int64
}

func (h *stubHoldings) GetHoldingsAtDate(_ context.Context, _, _ string, _ time.Time) (map[string]int64, error) {
	return h.data, nil
}

func TestDeclareValidation(t *testing.T) {
	repo := newMemRepo()
	svc := NewService(repo, nil)
	ctx := context.Background()

	tests := []struct {
		name    string
		d       Declaration
		wantErr string
	}{
		{"missing company", Declaration{ID: "d1", ShareClassID: "sc1", Type: "cash", AmountPerShare: 1, RecordDate: time.Now(), PayableDate: time.Now().Add(time.Hour)}, "company_id"},
		{"missing share class", Declaration{ID: "d1", CompanyID: "c1", Type: "cash", AmountPerShare: 1, RecordDate: time.Now(), PayableDate: time.Now().Add(time.Hour)}, "share_class_id"},
		{"missing type", Declaration{ID: "d1", CompanyID: "c1", ShareClassID: "sc1", RecordDate: time.Now(), PayableDate: time.Now().Add(time.Hour)}, "type"},
		{"cash zero amount", Declaration{ID: "d1", CompanyID: "c1", ShareClassID: "sc1", Type: "cash", RecordDate: time.Now(), PayableDate: time.Now().Add(time.Hour)}, "amount_per_share"},
		{"payable before record", Declaration{ID: "d1", CompanyID: "c1", ShareClassID: "sc1", Type: "cash", AmountPerShare: 1, RecordDate: time.Now().Add(time.Hour), PayableDate: time.Now()}, "payable_date must be after"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := svc.Declare(ctx, &tt.d)
			if err == nil {
				t.Fatal("expected error")
			}
			if !strings.Contains(err.Error(), tt.wantErr) {
				t.Fatalf("error %q does not contain %q", err.Error(), tt.wantErr)
			}
		})
	}
}

func TestDeclareSuccess(t *testing.T) {
	repo := newMemRepo()
	svc := NewService(repo, nil)
	ctx := context.Background()

	d := &Declaration{
		ID:             "d1",
		CompanyID:      "c1",
		ShareClassID:   "sc1",
		Type:           "cash",
		AmountPerShare: 0.25,
		RecordDate:     time.Now().Add(30 * 24 * time.Hour),
		PayableDate:    time.Now().Add(60 * 24 * time.Hour),
	}
	if err := svc.Declare(ctx, d); err != nil {
		t.Fatalf("Declare: %v", err)
	}
	if d.Status != "declared" {
		t.Fatalf("status = %q, want declared", d.Status)
	}
	if d.ExDividendDate.IsZero() {
		t.Fatal("ex_dividend_date should be set")
	}
}

func TestProcessRecordDate(t *testing.T) {
	repo := newMemRepo()
	holdings := &stubHoldings{data: map[string]int64{
		"sh1": 10_000,
		"sh2": 5_000,
		"sh3": 0, // should be skipped
	}}
	svc := NewService(repo, holdings)
	ctx := context.Background()

	repo.CreateDeclaration(ctx, &Declaration{
		ID:             "d1",
		CompanyID:      "c1",
		ShareClassID:   "sc1",
		Type:           "cash",
		AmountPerShare: 0.50,
		RecordDate:     time.Now(),
		PayableDate:    time.Now().Add(30 * 24 * time.Hour),
		Status:         "declared",
	})

	count, err := svc.ProcessRecordDate(ctx, "d1")
	if err != nil {
		t.Fatalf("ProcessRecordDate: %v", err)
	}
	if count != 2 {
		t.Fatalf("count = %d, want 2", count)
	}

	// Verify declaration updated.
	decl, _ := repo.GetDeclaration(ctx, "d1")
	if decl.Status != "record_set" {
		t.Fatalf("status = %q, want record_set", decl.Status)
	}
	if decl.TotalAmount != 7_500.0 { // (10000 + 5000) * 0.50
		t.Fatalf("total_amount = %f, want 7500", decl.TotalAmount)
	}

	// Verify distributions created.
	dists, _ := repo.ListDistributions(ctx, "d1")
	if len(dists) != 2 {
		t.Fatalf("distributions len = %d, want 2", len(dists))
	}
}

func TestProcessRecordDateAlreadyProcessed(t *testing.T) {
	repo := newMemRepo()
	svc := NewService(repo, nil)
	ctx := context.Background()

	repo.CreateDeclaration(ctx, &Declaration{
		ID:     "d1",
		Status: "record_set", // already processed
	})

	_, err := svc.ProcessRecordDate(ctx, "d1")
	if err == nil {
		t.Fatal("expected error for non-declared state")
	}
}

func TestGetSummary(t *testing.T) {
	repo := newMemRepo()
	svc := NewService(repo, nil)
	ctx := context.Background()

	repo.CreateDistribution(ctx, &Distribution{
		ID: "dist1", DeclarationID: "d1", GrossAmount: 5000, NetAmount: 4500, TaxWithholding: 500, Status: "paid",
	})
	repo.CreateDistribution(ctx, &Distribution{
		ID: "dist2", DeclarationID: "d1", GrossAmount: 2500, NetAmount: 2250, TaxWithholding: 250, Status: "pending",
	})

	summary, err := svc.GetSummary(ctx, "d1")
	if err != nil {
		t.Fatalf("GetSummary: %v", err)
	}
	if summary.RecipientsCount != 2 {
		t.Fatalf("recipients = %d, want 2", summary.RecipientsCount)
	}
	if summary.TotalGross != 7500 {
		t.Fatalf("total_distributed = %f, want 7500", summary.TotalGross)
	}
	if summary.PaidCount != 1 {
		t.Fatalf("paid = %d, want 1", summary.PaidCount)
	}
	if summary.PendingCount != 1 {
		t.Fatalf("pending = %d, want 1", summary.PendingCount)
	}
}

func TestMarkPaid(t *testing.T) {
	repo := newMemRepo()
	svc := NewService(repo, nil)
	ctx := context.Background()

	repo.CreateDistribution(ctx, &Distribution{
		ID: "dist1", DeclarationID: "d1", GrossAmount: 5000, NetAmount: 5000, Status: "pending",
	})

	if err := svc.MarkPaid(ctx, "dist1"); err != nil {
		t.Fatalf("MarkPaid: %v", err)
	}

	d, _ := repo.GetDistribution(ctx, "dist1")
	if d.Status != "paid" {
		t.Fatalf("status = %q, want paid", d.Status)
	}
	if d.PaidAt == nil {
		t.Fatal("paid_at should be set")
	}
}
