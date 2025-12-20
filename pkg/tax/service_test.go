package tax

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"
)

type memRepo struct {
	mu       sync.RWMutex
	divs     map[string]*Form1099DIV
	bs       map[string]*Form1099B
	k1s      map[string]*ScheduleK1
}

func newMemRepo() *memRepo {
	return &memRepo{
		divs: make(map[string]*Form1099DIV),
		bs:   make(map[string]*Form1099B),
		k1s:  make(map[string]*ScheduleK1),
	}
}

func (r *memRepo) CreateForm1099DIV(_ context.Context, f *Form1099DIV) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	cp := *f
	r.divs[f.ID] = &cp
	return nil
}

func (r *memRepo) GetForm1099DIV(_ context.Context, id string) (*Form1099DIV, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	f, ok := r.divs[id]
	if !ok {
		return nil, fmt.Errorf("1099-DIV %s not found", id)
	}
	cp := *f
	return &cp, nil
}

func (r *memRepo) ListForms1099DIV(_ context.Context, _ int, _ string) ([]*Form1099DIV, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	var out []*Form1099DIV
	for _, f := range r.divs {
		cp := *f
		out = append(out, &cp)
	}
	return out, nil
}

func (r *memRepo) CreateForm1099B(_ context.Context, f *Form1099B) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	cp := *f
	r.bs[f.ID] = &cp
	return nil
}

func (r *memRepo) GetForm1099B(_ context.Context, id string) (*Form1099B, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	f, ok := r.bs[id]
	if !ok {
		return nil, fmt.Errorf("1099-B %s not found", id)
	}
	cp := *f
	return &cp, nil
}

func (r *memRepo) ListForms1099B(_ context.Context, _ int, _ string) ([]*Form1099B, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	var out []*Form1099B
	for _, f := range r.bs {
		cp := *f
		out = append(out, &cp)
	}
	return out, nil
}

func (r *memRepo) CreateScheduleK1(_ context.Context, k *ScheduleK1) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	cp := *k
	r.k1s[k.ID] = &cp
	return nil
}

func (r *memRepo) GetScheduleK1(_ context.Context, id string) (*ScheduleK1, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	k, ok := r.k1s[id]
	if !ok {
		return nil, fmt.Errorf("K-1 %s not found", id)
	}
	cp := *k
	return &cp, nil
}

func (r *memRepo) ListScheduleK1s(_ context.Context, _ int, _ string) ([]*ScheduleK1, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	var out []*ScheduleK1
	for _, k := range r.k1s {
		cp := *k
		out = append(out, &cp)
	}
	return out, nil
}

// stubDividends implements DividendProvider for tests.
type stubDividends struct {
	data map[string]float64
}

func (d *stubDividends) GetDividendsByYear(_ context.Context, _ string, _ int) (map[string]float64, error) {
	return d.data, nil
}

func TestGenerate1099DIV(t *testing.T) {
	repo := newMemRepo()
	divProv := &stubDividends{data: map[string]float64{
		"sh1": 5000.00,
		"sh2": 250.00,
		"sh3": 5.00, // below $10 threshold
	}}
	svc := NewService(repo, divProv)

	recipients := map[string]RecipientInfo{
		"sh1": {Name: "Alice", TIN: "123-45-6789", Address: "123 Main St"},
		"sh2": {Name: "Bob", TIN: "987-65-4321", Address: "456 Oak Ave"},
		"sh3": {Name: "Charlie", TIN: "111-22-3333", Address: "789 Pine Rd"},
	}

	result, err := svc.Generate1099DIV(context.Background(), &GenerationRequest{
		CompanyID: "c1",
		TaxYear:   2025,
	}, "Acme Inc.", "12-3456789", recipients)
	if err != nil {
		t.Fatalf("Generate1099DIV: %v", err)
	}
	if result.FormsCreated != 2 { // sh3 below threshold
		t.Fatalf("forms_created = %d, want 2", result.FormsCreated)
	}
	if result.Errors != 0 {
		t.Fatalf("errors = %d, want 0", result.Errors)
	}
}

func TestGenerate1099DIVMissingRecipient(t *testing.T) {
	repo := newMemRepo()
	divProv := &stubDividends{data: map[string]float64{
		"sh1": 500.00,
		"sh_missing": 200.00, // no recipient info
	}}
	svc := NewService(repo, divProv)

	recipients := map[string]RecipientInfo{
		"sh1": {Name: "Alice", TIN: "123-45-6789"},
	}

	result, err := svc.Generate1099DIV(context.Background(), &GenerationRequest{
		CompanyID: "c1", TaxYear: 2025,
	}, "Acme", "12-3456789", recipients)
	if err != nil {
		t.Fatalf("Generate1099DIV: %v", err)
	}
	if result.FormsCreated != 1 {
		t.Fatalf("forms_created = %d, want 1", result.FormsCreated)
	}
	if result.Errors != 1 {
		t.Fatalf("errors = %d, want 1 (missing recipient)", result.Errors)
	}
}

func TestGenerate1099DIVValidation(t *testing.T) {
	svc := NewService(newMemRepo(), nil)

	_, err := svc.Generate1099DIV(context.Background(), &GenerationRequest{
		TaxYear: 2025,
	}, "Acme", "12-3456789", nil)
	if err == nil {
		t.Fatal("expected error for missing company_id")
	}

	_, err = svc.Generate1099DIV(context.Background(), &GenerationRequest{
		CompanyID: "c1",
	}, "Acme", "12-3456789", nil)
	if err == nil {
		t.Fatal("expected error for missing tax_year")
	}
}

func TestCompute1099BShortTerm(t *testing.T) {
	svc := NewService(newMemRepo(), nil)

	acquired := time.Date(2025, 6, 1, 0, 0, 0, 0, time.UTC)
	sold := time.Date(2025, 11, 1, 0, 0, 0, 0, time.UTC) // ~5 months
	form := svc.Compute1099B(15000, 10000, acquired, sold)

	if form.GainLoss != 5000 {
		t.Fatalf("gain_loss = %f, want 5000", form.GainLoss)
	}
	if form.ShortTermLongTerm != "short" {
		t.Fatalf("term = %q, want short", form.ShortTermLongTerm)
	}
	if form.Proceeds != 15000 {
		t.Fatalf("proceeds = %f, want 15000", form.Proceeds)
	}
	if form.CostBasis != 10000 {
		t.Fatalf("cost_basis = %f, want 10000", form.CostBasis)
	}
}

func TestCompute1099BLongTerm(t *testing.T) {
	svc := NewService(newMemRepo(), nil)

	acquired := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)
	sold := time.Date(2025, 6, 1, 0, 0, 0, 0, time.UTC) // ~17 months
	form := svc.Compute1099B(8000, 10000, acquired, sold)

	if form.GainLoss != -2000 {
		t.Fatalf("gain_loss = %f, want -2000", form.GainLoss)
	}
	if form.ShortTermLongTerm != "long" {
		t.Fatalf("term = %q, want long", form.ShortTermLongTerm)
	}
}

func TestCompute1099BBreakeven(t *testing.T) {
	svc := NewService(newMemRepo(), nil)

	acquired := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
	sold := time.Date(2025, 3, 1, 0, 0, 0, 0, time.UTC)
	form := svc.Compute1099B(5000, 5000, acquired, sold)

	if form.GainLoss != 0 {
		t.Fatalf("gain_loss = %f, want 0", form.GainLoss)
	}
}
