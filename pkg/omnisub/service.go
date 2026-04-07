package omnisub

import (
	"context"
	"fmt"
	"time"
)

// Service handles omnibus account and sub-account management.
type Service struct {
	repo Repository
}

// NewService creates an omnisub service backed by the given repository.
func NewService(repo Repository) *Service {
	return &Service{repo: repo}
}

// CreateOmnibusAccount validates and persists a new omnibus account.
func (s *Service) CreateOmnibusAccount(ctx context.Context, a *OmnibusAccount) error {
	if a.CompanyID == "" {
		return fmt.Errorf("company_id is required")
	}
	if a.Name == "" {
		return fmt.Errorf("name is required")
	}
	now := time.Now().UTC()
	a.CreatedAt = now
	a.UpdatedAt = now
	if a.Status == "" {
		a.Status = StatusActive
	}
	return s.repo.CreateOmnibusAccount(ctx, a)
}

// GetOmnibusAccount retrieves an omnibus account by ID.
func (s *Service) GetOmnibusAccount(ctx context.Context, id string) (*OmnibusAccount, error) {
	if id == "" {
		return nil, fmt.Errorf("omnibus account id is required")
	}
	return s.repo.GetOmnibusAccount(ctx, id)
}

// CreateSubAccount validates and persists a new sub-account.
func (s *Service) CreateSubAccount(ctx context.Context, a *SubAccount) error {
	if a.OmnibusAccountID == "" {
		return fmt.Errorf("omnibus_account_id is required")
	}
	if a.StakeholderID == "" {
		return fmt.Errorf("stakeholder_id is required")
	}
	if a.AccountNumber == "" {
		return fmt.Errorf("account_number is required")
	}
	now := time.Now().UTC()
	a.CreatedAt = now
	a.UpdatedAt = now
	if a.Status == "" {
		a.Status = StatusActive
	}
	return s.repo.CreateSubAccount(ctx, a)
}

// GetSubAccount retrieves a sub-account by ID.
func (s *Service) GetSubAccount(ctx context.Context, id string) (*SubAccount, error) {
	if id == "" {
		return nil, fmt.Errorf("sub-account id is required")
	}
	return s.repo.GetSubAccount(ctx, id)
}

// CloseSubAccount marks a sub-account as closed.
func (s *Service) CloseSubAccount(ctx context.Context, id string) error {
	a, err := s.repo.GetSubAccount(ctx, id)
	if err != nil {
		return fmt.Errorf("get sub-account: %w", err)
	}
	if a.Status == StatusClosed {
		return fmt.Errorf("sub-account %s is already closed", a.ID)
	}
	a.Status = StatusClosed
	a.UpdatedAt = time.Now().UTC()
	return s.repo.UpdateSubAccount(ctx, a)
}

// SubmitOrder validates and persists a new order.
func (s *Service) SubmitOrder(ctx context.Context, o *Order) error {
	if o.SubAccountID == "" {
		return fmt.Errorf("sub_account_id is required")
	}
	if o.SecurityID == "" {
		return fmt.Errorf("security_id is required")
	}
	if o.Quantity <= 0 {
		return fmt.Errorf("quantity must be positive")
	}
	if o.Price < 0 {
		return fmt.Errorf("price must be non-negative")
	}
	if o.Side == "" {
		return fmt.Errorf("side is required")
	}
	now := time.Now().UTC()
	o.SubmittedAt = now
	o.CreatedAt = now
	o.UpdatedAt = now
	if o.Status == "" {
		o.Status = OrderPending
	}
	return s.repo.CreateOrder(ctx, o)
}

// ListPositions returns all positions for a sub-account.
func (s *Service) ListPositions(ctx context.Context, subAccountID string) ([]*Position, error) {
	if subAccountID == "" {
		return nil, fmt.Errorf("sub_account_id is required")
	}
	return s.repo.ListPositions(ctx, subAccountID)
}

// RecordCorporateAction persists a corporate action event.
func (s *Service) RecordCorporateAction(ctx context.Context, ca *CorporateAction) error {
	if ca.SecurityID == "" {
		return fmt.Errorf("security_id is required")
	}
	if ca.Type == "" {
		return fmt.Errorf("type is required")
	}
	ca.CreatedAt = time.Now().UTC()
	return s.repo.CreateCorporateAction(ctx, ca)
}

// ListTaxStatements returns all tax statements for a sub-account.
func (s *Service) ListTaxStatements(ctx context.Context, subAccountID string) ([]*TaxStatement, error) {
	if subAccountID == "" {
		return nil, fmt.Errorf("sub_account_id is required")
	}
	return s.repo.ListTaxStatements(ctx, subAccountID)
}
