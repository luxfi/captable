package stakeholder

import (
	"context"
	"fmt"
	"time"
)

// Service handles stakeholder (investor/shareholder) management.
type Service struct {
	repo Repository
}

// NewService creates a stakeholder service backed by the given repository.
func NewService(repo Repository) *Service {
	return &Service{repo: repo}
}

// Create validates and persists a new stakeholder.
func (s *Service) Create(ctx context.Context, sh *Stakeholder) error {
	if sh.CompanyID == "" {
		return fmt.Errorf("company_id is required")
	}
	if sh.Name == "" {
		return fmt.Errorf("name is required")
	}
	if sh.Type == "" {
		return fmt.Errorf("type is required")
	}
	if sh.Type != "individual" && sh.Type != "entity" && sh.Type != "trust" {
		return fmt.Errorf("type must be individual, entity, or trust")
	}
	now := time.Now().UTC()
	sh.CreatedAt = now
	sh.UpdatedAt = now
	if sh.Status == "" {
		sh.Status = "active"
	}
	return s.repo.Create(ctx, sh)
}

// Get retrieves a stakeholder by ID.
func (s *Service) Get(ctx context.Context, id string) (*Stakeholder, error) {
	if id == "" {
		return nil, fmt.Errorf("stakeholder id is required")
	}
	return s.repo.Get(ctx, id)
}

// Update modifies an existing stakeholder.
func (s *Service) Update(ctx context.Context, sh *Stakeholder) error {
	if sh.ID == "" {
		return fmt.Errorf("stakeholder id is required")
	}
	sh.UpdatedAt = time.Now().UTC()
	return s.repo.Update(ctx, sh)
}

// List returns all stakeholders for a company.
func (s *Service) List(ctx context.Context, companyID string) ([]*Stakeholder, error) {
	if companyID == "" {
		return nil, fmt.Errorf("company_id is required")
	}
	return s.repo.List(ctx, companyID)
}

// IsAccredited checks whether a stakeholder has valid accreditation.
func (s *Service) IsAccredited(ctx context.Context, id string) (bool, error) {
	sh, err := s.repo.Get(ctx, id)
	if err != nil {
		return false, fmt.Errorf("get stakeholder: %w", err)
	}
	if sh.Accreditation == nil {
		return false, nil
	}
	if !sh.Accreditation.IsAccredited {
		return false, nil
	}
	// Check expiration if set.
	if sh.Accreditation.ExpiresAt != nil && sh.Accreditation.ExpiresAt.Before(time.Now()) {
		return false, nil
	}
	return true, nil
}
