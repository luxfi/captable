package captable

import (
	"context"
	"fmt"
	"time"
)

// Service provides cap table CRUD operations for companies, share classes, and entries.
type Service struct {
	repo Repository
}

// NewService creates a cap table service backed by the given repository.
func NewService(repo Repository) *Service {
	return &Service{repo: repo}
}

// CreateCompany validates and persists a new company.
func (s *Service) CreateCompany(ctx context.Context, c *Company) error {
	if c.TenantID == "" {
		return fmt.Errorf("tenant_id is required")
	}
	if c.LegalName == "" {
		return fmt.Errorf("legal_name is required")
	}
	if c.Jurisdiction == "" {
		return fmt.Errorf("jurisdiction is required")
	}
	if c.EntityType == "" {
		return fmt.Errorf("entity_type is required")
	}
	now := time.Now().UTC()
	c.CreatedAt = now
	c.UpdatedAt = now
	if c.Status == "" {
		c.Status = "active"
	}
	return s.repo.CreateCompany(ctx, c)
}

// GetCompany retrieves a company by ID.
func (s *Service) GetCompany(ctx context.Context, id string) (*Company, error) {
	if id == "" {
		return nil, fmt.Errorf("company id is required")
	}
	return s.repo.GetCompany(ctx, id)
}

// ListCompanies returns companies for a tenant.
func (s *Service) ListCompanies(ctx context.Context, tenantID string, params ListParams) ([]*Company, error) {
	if tenantID == "" {
		return nil, fmt.Errorf("tenant_id is required")
	}
	return s.repo.ListCompanies(ctx, tenantID, params)
}

// CreateShareClass validates and persists a new share class.
func (s *Service) CreateShareClass(ctx context.Context, sc *ShareClass) error {
	if sc.CompanyID == "" {
		return fmt.Errorf("company_id is required")
	}
	if sc.Name == "" {
		return fmt.Errorf("name is required")
	}
	if sc.AuthorizedShares <= 0 {
		return fmt.Errorf("authorized_shares must be positive")
	}
	if sc.Type == "" {
		return fmt.Errorf("type is required")
	}
	now := time.Now().UTC()
	sc.CreatedAt = now
	sc.UpdatedAt = now
	if sc.Status == "" {
		sc.Status = "active"
	}
	if sc.VotesPerShare == 0 && sc.VotingRights {
		sc.VotesPerShare = 1
	}
	return s.repo.CreateShareClass(ctx, sc)
}

// GetShareClass retrieves a share class by ID.
func (s *Service) GetShareClass(ctx context.Context, id string) (*ShareClass, error) {
	if id == "" {
		return nil, fmt.Errorf("share class id is required")
	}
	return s.repo.GetShareClass(ctx, id)
}

// ListShareClasses returns all share classes for a company.
func (s *Service) ListShareClasses(ctx context.Context, companyID string) ([]*ShareClass, error) {
	if companyID == "" {
		return nil, fmt.Errorf("company_id is required")
	}
	return s.repo.ListShareClasses(ctx, companyID)
}

// IssueShares creates a new cap table entry for share issuance and updates the share class counters.
func (s *Service) IssueShares(ctx context.Context, e *Entry) error {
	if e.CompanyID == "" {
		return fmt.Errorf("company_id is required")
	}
	if e.StakeholderID == "" {
		return fmt.Errorf("stakeholder_id is required")
	}
	if e.ShareClassID == "" {
		return fmt.Errorf("share_class_id is required")
	}
	if e.Shares <= 0 {
		return fmt.Errorf("shares must be positive")
	}

	// Load the share class and check authorized capacity.
	sc, err := s.repo.GetShareClass(ctx, e.ShareClassID)
	if err != nil {
		return fmt.Errorf("get share class: %w", err)
	}
	if sc.CompanyID != e.CompanyID {
		return fmt.Errorf("share class %s does not belong to company %s", e.ShareClassID, e.CompanyID)
	}
	if sc.IssuedShares+e.Shares > sc.AuthorizedShares {
		return fmt.Errorf("issuance of %d shares would exceed authorized %d (currently issued %d)",
			e.Shares, sc.AuthorizedShares, sc.IssuedShares)
	}

	now := time.Now().UTC()
	e.Type = "issuance"
	e.Status = "active"
	e.TotalValue = float64(e.Shares) * e.PricePerShare
	e.CreatedAt = now
	e.UpdatedAt = now
	if e.IssueDate.IsZero() {
		e.IssueDate = now
	}

	if err := s.repo.CreateEntry(ctx, e); err != nil {
		return fmt.Errorf("create entry: %w", err)
	}

	// Update share class counters.
	sc.IssuedShares += e.Shares
	sc.OutstandingShares += e.Shares
	sc.UpdatedAt = now
	if err := s.repo.UpdateShareClass(ctx, sc); err != nil {
		return fmt.Errorf("update share class: %w", err)
	}

	return nil
}

// GetSummary computes a cap table summary for a company.
func (s *Service) GetSummary(ctx context.Context, companyID string) (*Summary, error) {
	if companyID == "" {
		return nil, fmt.Errorf("company_id is required")
	}

	classes, err := s.repo.ListShareClasses(ctx, companyID)
	if err != nil {
		return nil, fmt.Errorf("list share classes: %w", err)
	}

	entries, err := s.repo.ListEntries(ctx, companyID, ListParams{})
	if err != nil {
		return nil, fmt.Errorf("list entries: %w", err)
	}

	var totalAuth, totalIssued, totalOutstanding int64
	stakeholders := make(map[string]bool)
	classSummaries := make([]ClassSummary, 0, len(classes))

	for _, sc := range classes {
		totalAuth += sc.AuthorizedShares
		totalIssued += sc.IssuedShares
		totalOutstanding += sc.OutstandingShares
		classSummaries = append(classSummaries, ClassSummary{
			ShareClassID: sc.ID,
			Name:         sc.Name,
			Type:         sc.Type,
			Authorized:   sc.AuthorizedShares,
			Issued:       sc.IssuedShares,
			Outstanding:  sc.OutstandingShares,
		})
	}

	for _, e := range entries {
		if e.Status == "active" {
			stakeholders[e.StakeholderID] = true
		}
	}

	// Compute ownership percentages.
	if totalOutstanding > 0 {
		for i := range classSummaries {
			classSummaries[i].OwnershipPercent = float64(classSummaries[i].Outstanding) / float64(totalOutstanding) * 100
			classSummaries[i].FullyDilutedPct = float64(classSummaries[i].Outstanding) / float64(totalOutstanding) * 100
		}
	}

	return &Summary{
		CompanyID:          companyID,
		TotalAuthorized:    totalAuth,
		TotalIssued:        totalIssued,
		TotalOutstanding:   totalOutstanding,
		TotalStakeholders:  len(stakeholders),
		FullyDilutedShares: totalOutstanding,
		ByClass:            classSummaries,
		AsOf:               time.Now().UTC(),
	}, nil
}
