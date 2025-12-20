package securities

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"testing"
)

// memRepo is an in-memory Repository for testing.
type memRepo struct {
	mu           sync.RWMutex
	securities   map[string]*Security
	certificates map[string]*Certificate
	ledger       []*LedgerEntry
}

func newMemRepo() *memRepo {
	return &memRepo{
		securities:   make(map[string]*Security),
		certificates: make(map[string]*Certificate),
	}
}

func (r *memRepo) CreateSecurity(_ context.Context, s *Security) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	cp := *s
	r.securities[s.ID] = &cp
	return nil
}

func (r *memRepo) GetSecurity(_ context.Context, id string) (*Security, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	s, ok := r.securities[id]
	if !ok {
		return nil, fmt.Errorf("security %s not found", id)
	}
	cp := *s
	return &cp, nil
}

func (r *memRepo) UpdateSecurity(_ context.Context, s *Security) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	cp := *s
	r.securities[s.ID] = &cp
	return nil
}

func (r *memRepo) ListSecurities(_ context.Context, companyID string) ([]*Security, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	var out []*Security
	for _, s := range r.securities {
		if s.CompanyID == companyID {
			cp := *s
			out = append(out, &cp)
		}
	}
	return out, nil
}

func (r *memRepo) CreateCertificate(_ context.Context, c *Certificate) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	cp := *c
	r.certificates[c.ID] = &cp
	return nil
}

func (r *memRepo) GetCertificate(_ context.Context, id string) (*Certificate, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	c, ok := r.certificates[id]
	if !ok {
		return nil, fmt.Errorf("certificate %s not found", id)
	}
	cp := *c
	return &cp, nil
}

func (r *memRepo) UpdateCertificate(_ context.Context, c *Certificate) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	cp := *c
	r.certificates[c.ID] = &cp
	return nil
}

func (r *memRepo) ListCertificates(_ context.Context, securityID string) ([]*Certificate, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	var out []*Certificate
	for _, c := range r.certificates {
		if c.SecurityID == securityID {
			cp := *c
			out = append(out, &cp)
		}
	}
	return out, nil
}

func (r *memRepo) AppendLedger(_ context.Context, entry *LedgerEntry) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	cp := *entry
	r.ledger = append(r.ledger, &cp)
	return nil
}

func (r *memRepo) ListLedger(_ context.Context, securityID string) ([]*LedgerEntry, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	var out []*LedgerEntry
	for _, e := range r.ledger {
		if e.SecurityID == securityID {
			cp := *e
			out = append(out, &cp)
		}
	}
	return out, nil
}

func (r *memRepo) ListLedgerByCompany(_ context.Context, companyID string) ([]*LedgerEntry, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	var out []*LedgerEntry
	for _, e := range r.ledger {
		if e.CompanyID == companyID {
			cp := *e
			out = append(out, &cp)
		}
	}
	return out, nil
}

func seedSecurity(repo *memRepo) {
	repo.CreateSecurity(context.Background(), &Security{
		ID:        "sec1",
		CompanyID: "c1",
		Name:      "Series A Preferred",
		Type:      "equity",
		Status:    "offering",
	})
}

func TestIssueSuccess(t *testing.T) {
	repo := newMemRepo()
	seedSecurity(repo)
	svc := NewService(repo)

	entry, err := svc.Issue(context.Background(), &IssuanceRequest{
		SecurityID:    "sec1",
		StakeholderID: "sh1",
		Shares:        10_000,
		PricePerShare: 1.50,
	})
	if err != nil {
		t.Fatalf("Issue: %v", err)
	}
	if entry.Action != "issue" {
		t.Fatalf("action = %q, want issue", entry.Action)
	}
	if entry.Shares != 10_000 {
		t.Fatalf("shares = %d, want 10000", entry.Shares)
	}

	// Verify security updated.
	sec, _ := repo.GetSecurity(context.Background(), "sec1")
	if sec.AmountRaised != 15_000.0 {
		t.Fatalf("amount_raised = %f, want 15000", sec.AmountRaised)
	}
	if sec.CurrentInvestors != 1 {
		t.Fatalf("current_investors = %d, want 1", sec.CurrentInvestors)
	}
}

func TestIssueValidation(t *testing.T) {
	repo := newMemRepo()
	seedSecurity(repo)
	svc := NewService(repo)

	tests := []struct {
		name    string
		req     IssuanceRequest
		wantErr string
	}{
		{"missing security", IssuanceRequest{StakeholderID: "sh1", Shares: 100, PricePerShare: 1}, "security_id"},
		{"missing stakeholder", IssuanceRequest{SecurityID: "sec1", Shares: 100, PricePerShare: 1}, "stakeholder_id"},
		{"zero shares", IssuanceRequest{SecurityID: "sec1", StakeholderID: "sh1", Shares: 0, PricePerShare: 1}, "positive"},
		{"negative price", IssuanceRequest{SecurityID: "sec1", StakeholderID: "sh1", Shares: 100, PricePerShare: -1}, "non-negative"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := svc.Issue(context.Background(), &tt.req)
			if err == nil {
				t.Fatal("expected error")
			}
			if !strings.Contains(err.Error(), tt.wantErr) {
				t.Fatalf("error %q does not contain %q", err.Error(), tt.wantErr)
			}
		})
	}
}

func TestIssueClosedSecurity(t *testing.T) {
	repo := newMemRepo()
	repo.CreateSecurity(context.Background(), &Security{
		ID: "sec2", CompanyID: "c1", Name: "Closed", Type: "equity", Status: "closed",
	})
	svc := NewService(repo)

	_, err := svc.Issue(context.Background(), &IssuanceRequest{
		SecurityID: "sec2", StakeholderID: "sh1", Shares: 100, PricePerShare: 1,
	})
	if err == nil {
		t.Fatal("expected error for closed security")
	}
	if !strings.Contains(err.Error(), "not in an issuable state") {
		t.Fatalf("error %q unexpected", err.Error())
	}
}

func TestTransferSuccess(t *testing.T) {
	repo := newMemRepo()
	seedSecurity(repo)
	svc := NewService(repo)

	entry, err := svc.Transfer(context.Background(), &TransferRequest{
		SecurityID:      "sec1",
		FromStakeholder: "sh1",
		ToStakeholder:   "sh2",
		Shares:          5_000,
		PricePerShare:   2.00,
	})
	if err != nil {
		t.Fatalf("Transfer: %v", err)
	}
	if entry.Action != "transfer" {
		t.Fatalf("action = %q, want transfer", entry.Action)
	}
}

func TestTransferSameStakeholder(t *testing.T) {
	repo := newMemRepo()
	seedSecurity(repo)
	svc := NewService(repo)

	_, err := svc.Transfer(context.Background(), &TransferRequest{
		SecurityID:      "sec1",
		FromStakeholder: "sh1",
		ToStakeholder:   "sh1",
		Shares:          100,
	})
	if err == nil {
		t.Fatal("expected error for same stakeholder transfer")
	}
}

func TestCancelSuccess(t *testing.T) {
	repo := newMemRepo()
	seedSecurity(repo)
	svc := NewService(repo)

	entry, err := svc.Cancel(context.Background(), &CancellationRequest{
		SecurityID:    "sec1",
		StakeholderID: "sh1",
		Shares:        1_000,
		Reason:        "employee termination",
	})
	if err != nil {
		t.Fatalf("Cancel: %v", err)
	}
	if entry.Action != "cancel" {
		t.Fatalf("action = %q, want cancel", entry.Action)
	}
	if entry.Reference != "employee termination" {
		t.Fatalf("reference = %q, want reason", entry.Reference)
	}
}

func TestCancelMissingReason(t *testing.T) {
	repo := newMemRepo()
	seedSecurity(repo)
	svc := NewService(repo)

	_, err := svc.Cancel(context.Background(), &CancellationRequest{
		SecurityID:    "sec1",
		StakeholderID: "sh1",
		Shares:        100,
	})
	if err == nil {
		t.Fatal("expected error for missing reason")
	}
}

func TestConvertSuccess(t *testing.T) {
	repo := newMemRepo()
	seedSecurity(repo)
	svc := NewService(repo)

	entry, err := svc.Convert(context.Background(), &ConversionRequest{
		SecurityID:       "sec1",
		StakeholderID:    "sh1",
		TargetClassID:    "sc_common",
		ConversionShares: 20_000,
		ConversionPrice:  0.75,
	})
	if err != nil {
		t.Fatalf("Convert: %v", err)
	}
	if entry.Action != "convert" {
		t.Fatalf("action = %q, want convert", entry.Action)
	}
	if !strings.Contains(entry.Reference, "target_class=sc_common") {
		t.Fatalf("reference %q missing target class", entry.Reference)
	}
}

func TestGetLedger(t *testing.T) {
	repo := newMemRepo()
	seedSecurity(repo)
	svc := NewService(repo)

	svc.Issue(context.Background(), &IssuanceRequest{
		SecurityID: "sec1", StakeholderID: "sh1", Shares: 1000, PricePerShare: 1,
	})
	svc.Transfer(context.Background(), &TransferRequest{
		SecurityID: "sec1", FromStakeholder: "sh1", ToStakeholder: "sh2", Shares: 500, PricePerShare: 1,
	})

	ledger, err := svc.GetLedger(context.Background(), "sec1")
	if err != nil {
		t.Fatalf("GetLedger: %v", err)
	}
	if len(ledger) != 2 {
		t.Fatalf("ledger len = %d, want 2", len(ledger))
	}
	if ledger[0].Action != "issue" {
		t.Fatalf("first entry action = %q, want issue", ledger[0].Action)
	}
	if ledger[1].Action != "transfer" {
		t.Fatalf("second entry action = %q, want transfer", ledger[1].Action)
	}
}
