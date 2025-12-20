package captable

import (
	"context"
	"strings"
	"testing"
)

func TestCreateCompanyValidation(t *testing.T) {
	svc := NewService(newMemRepo())
	ctx := context.Background()

	tests := []struct {
		name    string
		company Company
		wantErr string
	}{
		{"missing tenant", Company{ID: "c1", LegalName: "Acme", Jurisdiction: "DE", EntityType: "corporation"}, "tenant_id"},
		{"missing legal name", Company{ID: "c1", TenantID: "t1", Jurisdiction: "DE", EntityType: "corporation"}, "legal_name"},
		{"missing jurisdiction", Company{ID: "c1", TenantID: "t1", LegalName: "Acme", EntityType: "corporation"}, "jurisdiction"},
		{"missing entity type", Company{ID: "c1", TenantID: "t1", LegalName: "Acme", Jurisdiction: "DE"}, "entity_type"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := svc.CreateCompany(ctx, &tt.company)
			if err == nil {
				t.Fatal("expected error")
			}
			if !strings.Contains(err.Error(), tt.wantErr) {
				t.Fatalf("error %q does not contain %q", err.Error(), tt.wantErr)
			}
		})
	}
}

func TestCreateCompanySuccess(t *testing.T) {
	svc := NewService(newMemRepo())
	ctx := context.Background()

	c := &Company{
		ID:           "c1",
		TenantID:     "t1",
		LegalName:    "Acme Inc.",
		Name:         "Acme",
		Jurisdiction: "DE",
		EntityType:   "corporation",
	}
	if err := svc.CreateCompany(ctx, c); err != nil {
		t.Fatalf("CreateCompany: %v", err)
	}
	if c.Status != "active" {
		t.Fatalf("status = %q, want active", c.Status)
	}
	if c.CreatedAt.IsZero() {
		t.Fatal("created_at should be set")
	}

	got, err := svc.GetCompany(ctx, "c1")
	if err != nil {
		t.Fatalf("GetCompany: %v", err)
	}
	if got.LegalName != "Acme Inc." {
		t.Fatalf("legal_name = %q, want Acme Inc.", got.LegalName)
	}
}

func TestCreateShareClassValidation(t *testing.T) {
	svc := NewService(newMemRepo())
	ctx := context.Background()

	tests := []struct {
		name    string
		sc      ShareClass
		wantErr string
	}{
		{"missing company", ShareClass{ID: "sc1", Name: "Common", AuthorizedShares: 1000, Type: "common"}, "company_id"},
		{"missing name", ShareClass{ID: "sc1", CompanyID: "c1", AuthorizedShares: 1000, Type: "common"}, "name"},
		{"zero authorized", ShareClass{ID: "sc1", CompanyID: "c1", Name: "Common", AuthorizedShares: 0, Type: "common"}, "authorized_shares"},
		{"missing type", ShareClass{ID: "sc1", CompanyID: "c1", Name: "Common", AuthorizedShares: 1000}, "type"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := svc.CreateShareClass(ctx, &tt.sc)
			if err == nil {
				t.Fatal("expected error")
			}
			if !strings.Contains(err.Error(), tt.wantErr) {
				t.Fatalf("error %q does not contain %q", err.Error(), tt.wantErr)
			}
		})
	}
}

func TestCreateShareClassSuccess(t *testing.T) {
	svc := NewService(newMemRepo())
	ctx := context.Background()

	sc := &ShareClass{
		ID:               "sc1",
		CompanyID:        "c1",
		Name:             "Common Stock",
		AuthorizedShares: 10_000_000,
		ParValue:         0.001,
		VotingRights:     true,
		Type:             "common",
	}
	if err := svc.CreateShareClass(ctx, sc); err != nil {
		t.Fatalf("CreateShareClass: %v", err)
	}
	if sc.Status != "active" {
		t.Fatalf("status = %q, want active", sc.Status)
	}
	if sc.VotesPerShare != 1 {
		t.Fatalf("votes_per_share = %d, want 1 (default for voting)", sc.VotesPerShare)
	}
}

func TestIssueSharesSuccess(t *testing.T) {
	repo := newMemRepo()
	svc := NewService(repo)
	ctx := context.Background()

	// Seed share class directly in repo.
	sc := &ShareClass{
		ID:               "sc1",
		CompanyID:        "c1",
		Name:             "Common",
		AuthorizedShares: 10_000_000,
		Type:             "common",
		Status:           "active",
	}
	if err := repo.CreateShareClass(ctx, sc); err != nil {
		t.Fatal(err)
	}

	entry := &Entry{
		ID:            "e1",
		CompanyID:     "c1",
		StakeholderID: "sh1",
		ShareClassID:  "sc1",
		Shares:        100_000,
		PricePerShare: 0.10,
	}
	if err := svc.IssueShares(ctx, entry); err != nil {
		t.Fatalf("IssueShares: %v", err)
	}
	if entry.Type != "issuance" {
		t.Fatalf("type = %q, want issuance", entry.Type)
	}
	if entry.TotalValue != 10_000.0 {
		t.Fatalf("total_value = %f, want 10000", entry.TotalValue)
	}

	// Verify share class updated.
	updated, _ := repo.GetShareClass(ctx, "sc1")
	if updated.IssuedShares != 100_000 {
		t.Fatalf("issued_shares = %d, want 100000", updated.IssuedShares)
	}
	if updated.OutstandingShares != 100_000 {
		t.Fatalf("outstanding_shares = %d, want 100000", updated.OutstandingShares)
	}
}

func TestIssueSharesExceedsAuthorized(t *testing.T) {
	repo := newMemRepo()
	svc := NewService(repo)
	ctx := context.Background()

	sc := &ShareClass{
		ID:               "sc1",
		CompanyID:        "c1",
		AuthorizedShares: 1000,
		IssuedShares:     900,
		Type:             "common",
		Name:             "Common",
		Status:           "active",
	}
	if err := repo.CreateShareClass(ctx, sc); err != nil {
		t.Fatal(err)
	}

	entry := &Entry{
		ID:            "e1",
		CompanyID:     "c1",
		StakeholderID: "sh1",
		ShareClassID:  "sc1",
		Shares:        200, // would bring total to 1100 > 1000
		PricePerShare: 1.0,
	}
	err := svc.IssueShares(ctx, entry)
	if err == nil {
		t.Fatal("expected error for exceeding authorized shares")
	}
	if !strings.Contains(err.Error(), "exceed authorized") {
		t.Fatalf("error %q does not mention authorized", err.Error())
	}
}

func TestIssueSharesWrongCompany(t *testing.T) {
	repo := newMemRepo()
	svc := NewService(repo)
	ctx := context.Background()

	sc := &ShareClass{
		ID:               "sc1",
		CompanyID:        "c2", // different company
		AuthorizedShares: 1000,
		Type:             "common",
		Name:             "Common",
	}
	if err := repo.CreateShareClass(ctx, sc); err != nil {
		t.Fatal(err)
	}

	entry := &Entry{
		ID:            "e1",
		CompanyID:     "c1", // mismatched
		StakeholderID: "sh1",
		ShareClassID:  "sc1",
		Shares:        100,
		PricePerShare: 1.0,
	}
	err := svc.IssueShares(ctx, entry)
	if err == nil {
		t.Fatal("expected error for company mismatch")
	}
	if !strings.Contains(err.Error(), "does not belong") {
		t.Fatalf("error %q does not mention mismatch", err.Error())
	}
}

func TestGetSummary(t *testing.T) {
	repo := newMemRepo()
	svc := NewService(repo)
	ctx := context.Background()

	// Create two share classes.
	repo.CreateShareClass(ctx, &ShareClass{
		ID: "sc1", CompanyID: "c1", Name: "Common", Type: "common",
		AuthorizedShares: 10_000_000, IssuedShares: 1_000_000, OutstandingShares: 1_000_000,
	})
	repo.CreateShareClass(ctx, &ShareClass{
		ID: "sc2", CompanyID: "c1", Name: "Series A", Type: "preferred_a",
		AuthorizedShares: 2_000_000, IssuedShares: 500_000, OutstandingShares: 500_000,
	})

	// Create entries.
	repo.CreateEntry(ctx, &Entry{ID: "e1", CompanyID: "c1", StakeholderID: "sh1", ShareClassID: "sc1", Status: "active"})
	repo.CreateEntry(ctx, &Entry{ID: "e2", CompanyID: "c1", StakeholderID: "sh2", ShareClassID: "sc2", Status: "active"})
	repo.CreateEntry(ctx, &Entry{ID: "e3", CompanyID: "c1", StakeholderID: "sh1", ShareClassID: "sc2", Status: "active"})

	summary, err := svc.GetSummary(ctx, "c1")
	if err != nil {
		t.Fatalf("GetSummary: %v", err)
	}
	if summary.TotalAuthorized != 12_000_000 {
		t.Fatalf("total_authorized = %d, want 12000000", summary.TotalAuthorized)
	}
	if summary.TotalOutstanding != 1_500_000 {
		t.Fatalf("total_outstanding = %d, want 1500000", summary.TotalOutstanding)
	}
	if summary.TotalStakeholders != 2 {
		t.Fatalf("total_stakeholders = %d, want 2", summary.TotalStakeholders)
	}
	if len(summary.ByClass) != 2 {
		t.Fatalf("by_class len = %d, want 2", len(summary.ByClass))
	}
}

func TestGetCompanyNotFound(t *testing.T) {
	svc := NewService(newMemRepo())
	_, err := svc.GetCompany(context.Background(), "nonexistent")
	if err == nil {
		t.Fatal("expected error for nonexistent company")
	}
}

func TestGetCompanyEmptyID(t *testing.T) {
	svc := NewService(newMemRepo())
	_, err := svc.GetCompany(context.Background(), "")
	if err == nil {
		t.Fatal("expected error for empty id")
	}
}

func TestListCompaniesEmptyTenant(t *testing.T) {
	svc := NewService(newMemRepo())
	_, err := svc.ListCompanies(context.Background(), "", ListParams{})
	if err == nil {
		t.Fatal("expected error for empty tenant")
	}
}
