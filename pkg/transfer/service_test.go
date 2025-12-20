package transfer

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"
)

type memRepo struct {
	mu    sync.RWMutex
	items map[string]*Restriction
}

func newMemRepo() *memRepo {
	return &memRepo{items: make(map[string]*Restriction)}
}

func (r *memRepo) CreateRestriction(_ context.Context, rest *Restriction) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	cp := *rest
	r.items[rest.ID] = &cp
	return nil
}

func (r *memRepo) GetRestriction(_ context.Context, id string) (*Restriction, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	rest, ok := r.items[id]
	if !ok {
		return nil, fmt.Errorf("restriction %s not found", id)
	}
	cp := *rest
	return &cp, nil
}

func (r *memRepo) UpdateRestriction(_ context.Context, rest *Restriction) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	cp := *rest
	r.items[rest.ID] = &cp
	return nil
}

func (r *memRepo) ListRestrictions(_ context.Context, companyID string) ([]*Restriction, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	var out []*Restriction
	for _, rest := range r.items {
		if rest.CompanyID == companyID {
			cp := *rest
			out = append(out, &cp)
		}
	}
	return out, nil
}

func (r *memRepo) ListByStakeholder(_ context.Context, companyID, stakeholderID string) ([]*Restriction, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	var out []*Restriction
	for _, rest := range r.items {
		if rest.CompanyID == companyID && rest.StakeholderID == stakeholderID {
			cp := *rest
			out = append(out, &cp)
		}
	}
	return out, nil
}

func (r *memRepo) ListByShareClass(_ context.Context, companyID, shareClassID string) ([]*Restriction, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	var out []*Restriction
	for _, rest := range r.items {
		if rest.CompanyID == companyID && rest.ShareClassID == shareClassID {
			cp := *rest
			out = append(out, &cp)
		}
	}
	return out, nil
}

func TestCheckTransferNoRestrictions(t *testing.T) {
	svc := NewService(newMemRepo())
	result, err := svc.CheckTransfer(context.Background(), &CheckRequest{
		CompanyID:       "c1",
		FromStakeholder: "sh1",
		ToStakeholder:   "sh2",
		ShareClassID:    "sc1",
		Shares:          1000,
	})
	if err != nil {
		t.Fatalf("CheckTransfer: %v", err)
	}
	if !result.Allowed {
		t.Fatal("expected allowed with no restrictions")
	}
}

func TestCheckTransferBlocked(t *testing.T) {
	repo := newMemRepo()
	svc := NewService(repo)
	ctx := context.Background()

	future := time.Now().Add(365 * 24 * time.Hour)
	repo.CreateRestriction(ctx, &Restriction{
		ID:        "r1",
		CompanyID: "c1",
		Type:      "lockup",
		StartDate: time.Now().Add(-30 * 24 * time.Hour),
		EndDate:   &future,
		Status:    "active",
		Conditions: "180-day post-issuance lockup",
	})

	result, err := svc.CheckTransfer(ctx, &CheckRequest{
		CompanyID:       "c1",
		FromStakeholder: "sh1",
		ToStakeholder:   "sh2",
		ShareClassID:    "sc1",
		Shares:          1000,
	})
	if err != nil {
		t.Fatalf("CheckTransfer: %v", err)
	}
	if result.Allowed {
		t.Fatal("expected blocked by lockup")
	}
	if len(result.Restrictions) != 1 {
		t.Fatalf("restrictions len = %d, want 1", len(result.Restrictions))
	}
}

func TestCheckTransferExpiredRestriction(t *testing.T) {
	repo := newMemRepo()
	svc := NewService(repo)
	ctx := context.Background()

	past := time.Now().Add(-24 * time.Hour)
	repo.CreateRestriction(ctx, &Restriction{
		ID:        "r1",
		CompanyID: "c1",
		Type:      "lockup",
		StartDate: time.Now().Add(-365 * 24 * time.Hour),
		EndDate:   &past,
		Status:    "active",
	})

	result, err := svc.CheckTransfer(ctx, &CheckRequest{
		CompanyID:       "c1",
		FromStakeholder: "sh1",
		ToStakeholder:   "sh2",
		ShareClassID:    "sc1",
		Shares:          1000,
	})
	if err != nil {
		t.Fatalf("CheckTransfer: %v", err)
	}
	if !result.Allowed {
		t.Fatal("expected allowed — restriction expired")
	}
}

func TestCheckTransferStakeholderSpecific(t *testing.T) {
	repo := newMemRepo()
	svc := NewService(repo)
	ctx := context.Background()

	future := time.Now().Add(365 * 24 * time.Hour)
	// Restriction only applies to sh1.
	repo.CreateRestriction(ctx, &Restriction{
		ID:            "r1",
		CompanyID:     "c1",
		StakeholderID: "sh1",
		Type:          "lockup",
		StartDate:     time.Now(),
		EndDate:       &future,
		Status:        "active",
	})

	// sh2 should not be blocked.
	result, err := svc.CheckTransfer(ctx, &CheckRequest{
		CompanyID:       "c1",
		FromStakeholder: "sh2",
		ToStakeholder:   "sh3",
		ShareClassID:    "sc1",
		Shares:          1000,
	})
	if err != nil {
		t.Fatalf("CheckTransfer: %v", err)
	}
	if !result.Allowed {
		t.Fatal("expected allowed — restriction targets sh1, not sh2")
	}

	// sh1 should be blocked.
	result2, err := svc.CheckTransfer(ctx, &CheckRequest{
		CompanyID:       "c1",
		FromStakeholder: "sh1",
		ToStakeholder:   "sh3",
		ShareClassID:    "sc1",
		Shares:          1000,
	})
	if err != nil {
		t.Fatalf("CheckTransfer: %v", err)
	}
	if result2.Allowed {
		t.Fatal("expected blocked for sh1")
	}
}

func TestRule144ReportingIssuer(t *testing.T) {
	svc := NewService(newMemRepo())

	result := svc.CheckRule144(&Rule144Check{
		StakeholderID: "sh1",
		IsAffiliate:   false,
		AcquiredDate:  time.Now().Add(-7 * 30 * 24 * time.Hour), // 7 months ago
		IsReporting:   true,
		SharesHeld:    100_000,
		SharesToSell:  10_000,
	})

	if !result.Eligible {
		t.Fatalf("expected eligible: %s", result.Reason)
	}
	if result.HoldingPeriod != 6 {
		t.Fatalf("holding_period = %d, want 6", result.HoldingPeriod)
	}
}

func TestRule144NonReportingInsufficientHolding(t *testing.T) {
	svc := NewService(newMemRepo())

	result := svc.CheckRule144(&Rule144Check{
		StakeholderID: "sh1",
		IsAffiliate:   false,
		AcquiredDate:  time.Now().Add(-9 * 30 * 24 * time.Hour), // 9 months
		IsReporting:   false,
		SharesHeld:    100_000,
		SharesToSell:  10_000,
	})

	if result.Eligible {
		t.Fatal("expected not eligible — non-reporting needs 12 months")
	}
	if result.HoldingPeriod != 12 {
		t.Fatalf("holding_period = %d, want 12", result.HoldingPeriod)
	}
}

func TestRule144AffiliateVolumeLimit(t *testing.T) {
	svc := NewService(newMemRepo())

	result := svc.CheckRule144(&Rule144Check{
		StakeholderID: "sh1",
		IsAffiliate:   true,
		AcquiredDate:  time.Now().Add(-13 * 30 * 24 * time.Hour), // 13 months
		IsReporting:   true,
		SharesHeld:    1_000_000,
		SharesToSell:  50_000, // 5% — exceeds 1% volume limit
		AvgWeeklyVol:  5_000,
	})

	if result.Eligible {
		t.Fatal("expected not eligible — volume limit exceeded")
	}
	// 1% of 1M = 10,000; avg weekly vol = 5,000; max = 10,000
	if result.VolumeLimit != 10_000 {
		t.Fatalf("volume_limit = %d, want 10000", result.VolumeLimit)
	}
}

func TestRule144AffiliateWithinVolumeLimit(t *testing.T) {
	svc := NewService(newMemRepo())

	result := svc.CheckRule144(&Rule144Check{
		StakeholderID: "sh1",
		IsAffiliate:   true,
		AcquiredDate:  time.Now().Add(-13 * 30 * 24 * time.Hour),
		IsReporting:   true,
		SharesHeld:    1_000_000,
		SharesToSell:  5_000, // under 1% limit of 10,000
		AvgWeeklyVol:  2_000,
	})

	if !result.Eligible {
		t.Fatalf("expected eligible: %s", result.Reason)
	}
}

func TestWaiveRestriction(t *testing.T) {
	repo := newMemRepo()
	svc := NewService(repo)
	ctx := context.Background()

	future := time.Now().Add(365 * 24 * time.Hour)
	repo.CreateRestriction(ctx, &Restriction{
		ID: "r1", CompanyID: "c1", Type: "board_approval",
		StartDate: time.Now(), EndDate: &future, Status: "active",
	})

	if err := svc.WaiveRestriction(ctx, "r1"); err != nil {
		t.Fatalf("WaiveRestriction: %v", err)
	}

	r, _ := repo.GetRestriction(ctx, "r1")
	if r.Status != "waived" {
		t.Fatalf("status = %q, want waived", r.Status)
	}
}
