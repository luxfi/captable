package safe

import (
	"context"
	"fmt"
	"time"
)

// Service handles SAFE lifecycle management.
type Service struct {
	repo Repository
}

// NewService creates a SAFE service backed by the given repository.
func NewService(repo Repository) *Service {
	return &Service{repo: repo}
}

// Create validates and persists a new SAFE.
func (s *Service) Create(ctx context.Context, sf *Safe) error {
	if sf.CompanyID == "" {
		return fmt.Errorf("company_id is required")
	}
	if sf.StakeholderID == "" {
		return fmt.Errorf("stakeholder_id is required")
	}
	if sf.Capital <= 0 {
		return fmt.Errorf("capital must be positive")
	}
	if sf.Type == "" {
		return fmt.Errorf("type is required")
	}
	now := time.Now().UTC()
	sf.CreatedAt = now
	sf.UpdatedAt = now
	if sf.Status == "" {
		sf.Status = StatusDraft
	}
	return s.repo.Create(ctx, sf)
}

// Get retrieves a SAFE by ID.
func (s *Service) Get(ctx context.Context, id string) (*Safe, error) {
	if id == "" {
		return nil, fmt.Errorf("safe id is required")
	}
	return s.repo.Get(ctx, id)
}

// List returns all SAFEs for a company.
func (s *Service) List(ctx context.Context, companyID string) ([]*Safe, error) {
	if companyID == "" {
		return nil, fmt.Errorf("company_id is required")
	}
	return s.repo.List(ctx, companyID)
}

// Update modifies an existing SAFE.
func (s *Service) Update(ctx context.Context, sf *Safe) error {
	if sf.ID == "" {
		return fmt.Errorf("safe id is required")
	}
	sf.UpdatedAt = time.Now().UTC()
	return s.repo.Update(ctx, sf)
}

// MarkConverted transitions a SAFE to converted status.
func (s *Service) MarkConverted(ctx context.Context, id string) error {
	sf, err := s.repo.Get(ctx, id)
	if err != nil {
		return fmt.Errorf("get safe: %w", err)
	}
	if sf.Status != StatusActive {
		return fmt.Errorf("safe %s is not active (status=%s)", sf.ID, sf.Status)
	}
	sf.Status = StatusConverted
	sf.UpdatedAt = time.Now().UTC()
	return s.repo.Update(ctx, sf)
}

// Cancel marks a SAFE as cancelled.
func (s *Service) Cancel(ctx context.Context, id string) error {
	sf, err := s.repo.Get(ctx, id)
	if err != nil {
		return fmt.Errorf("get safe: %w", err)
	}
	if sf.Status == StatusConverted {
		return fmt.Errorf("cannot cancel a converted safe")
	}
	sf.Status = StatusCancelled
	sf.UpdatedAt = time.Now().UTC()
	return s.repo.Update(ctx, sf)
}
