package stakeholder

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
	items map[string]*Stakeholder
}

func newMemRepo() *memRepo {
	return &memRepo{items: make(map[string]*Stakeholder)}
}

func (r *memRepo) Create(_ context.Context, s *Stakeholder) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	cp := *s
	r.items[s.ID] = &cp
	return nil
}

func (r *memRepo) Get(_ context.Context, id string) (*Stakeholder, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	s, ok := r.items[id]
	if !ok {
		return nil, fmt.Errorf("stakeholder %s not found", id)
	}
	cp := *s
	return &cp, nil
}

func (r *memRepo) Update(_ context.Context, s *Stakeholder) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	cp := *s
	r.items[s.ID] = &cp
	return nil
}

func (r *memRepo) List(_ context.Context, companyID string) ([]*Stakeholder, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	var out []*Stakeholder
	for _, s := range r.items {
		if s.CompanyID == companyID {
			cp := *s
			out = append(out, &cp)
		}
	}
	return out, nil
}

func (r *memRepo) ListByTenant(_ context.Context, tenantID string) ([]*Stakeholder, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	var out []*Stakeholder
	for _, s := range r.items {
		if s.TenantID == tenantID {
			cp := *s
			out = append(out, &cp)
		}
	}
	return out, nil
}

func (r *memRepo) GetByEmail(_ context.Context, companyID, email string) (*Stakeholder, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	for _, s := range r.items {
		if s.CompanyID == companyID && s.Email == email {
			cp := *s
			return &cp, nil
		}
	}
	return nil, fmt.Errorf("stakeholder with email %s not found", email)
}

func TestCreateStakeholderSuccess(t *testing.T) {
	svc := NewService(newMemRepo())
	ctx := context.Background()

	sh := &Stakeholder{
		ID:        "sh1",
		CompanyID: "c1",
		Name:      "Jane Doe",
		Type:      "individual",
		Role:      "founder",
		Email:     "jane@example.com",
	}
	if err := svc.Create(ctx, sh); err != nil {
		t.Fatalf("Create: %v", err)
	}
	if sh.Status != "active" {
		t.Fatalf("status = %q, want active", sh.Status)
	}
	if sh.CreatedAt.IsZero() {
		t.Fatal("created_at should be set")
	}

	got, err := svc.Get(ctx, "sh1")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if got.Name != "Jane Doe" {
		t.Fatalf("name = %q, want Jane Doe", got.Name)
	}
}

func TestCreateStakeholderValidation(t *testing.T) {
	svc := NewService(newMemRepo())
	ctx := context.Background()

	tests := []struct {
		name    string
		sh      Stakeholder
		wantErr string
	}{
		{"missing company", Stakeholder{ID: "sh1", Name: "X", Type: "individual"}, "company_id"},
		{"missing name", Stakeholder{ID: "sh1", CompanyID: "c1", Type: "individual"}, "name"},
		{"missing type", Stakeholder{ID: "sh1", CompanyID: "c1", Name: "X"}, "type is required"},
		{"bad type", Stakeholder{ID: "sh1", CompanyID: "c1", Name: "X", Type: "alien"}, "must be"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := svc.Create(ctx, &tt.sh)
			if err == nil {
				t.Fatal("expected error")
			}
			if !strings.Contains(err.Error(), tt.wantErr) {
				t.Fatalf("error %q does not contain %q", err.Error(), tt.wantErr)
			}
		})
	}
}

func TestIsAccreditedTrue(t *testing.T) {
	repo := newMemRepo()
	svc := NewService(repo)
	ctx := context.Background()

	future := time.Now().Add(365 * 24 * time.Hour)
	repo.Create(ctx, &Stakeholder{
		ID: "sh1", CompanyID: "c1", Name: "Rich", Type: "individual",
		Accreditation: &Accreditation{
			IsAccredited: true,
			Method:       "income",
			ExpiresAt:    &future,
		},
	})

	ok, err := svc.IsAccredited(ctx, "sh1")
	if err != nil {
		t.Fatalf("IsAccredited: %v", err)
	}
	if !ok {
		t.Fatal("expected accredited")
	}
}

func TestIsAccreditedExpired(t *testing.T) {
	repo := newMemRepo()
	svc := NewService(repo)
	ctx := context.Background()

	past := time.Now().Add(-24 * time.Hour)
	repo.Create(ctx, &Stakeholder{
		ID: "sh1", CompanyID: "c1", Name: "Expired", Type: "individual",
		Accreditation: &Accreditation{
			IsAccredited: true,
			ExpiresAt:    &past,
		},
	})

	ok, err := svc.IsAccredited(ctx, "sh1")
	if err != nil {
		t.Fatalf("IsAccredited: %v", err)
	}
	if ok {
		t.Fatal("expected not accredited (expired)")
	}
}

func TestIsAccreditedNoAccreditation(t *testing.T) {
	repo := newMemRepo()
	svc := NewService(repo)
	ctx := context.Background()

	repo.Create(ctx, &Stakeholder{
		ID: "sh1", CompanyID: "c1", Name: "None", Type: "individual",
	})

	ok, err := svc.IsAccredited(ctx, "sh1")
	if err != nil {
		t.Fatalf("IsAccredited: %v", err)
	}
	if ok {
		t.Fatal("expected not accredited")
	}
}

func TestGetNotFound(t *testing.T) {
	svc := NewService(newMemRepo())
	_, err := svc.Get(context.Background(), "nonexistent")
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestGetEmptyID(t *testing.T) {
	svc := NewService(newMemRepo())
	_, err := svc.Get(context.Background(), "")
	if err == nil {
		t.Fatal("expected error")
	}
}
