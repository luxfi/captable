package note

import (
	"context"
	"fmt"
	"time"
)

// Service handles convertible note lifecycle management.
type Service struct {
	repo Repository
}

// NewService creates a convertible note service backed by the given repository.
func NewService(repo Repository) *Service {
	return &Service{repo: repo}
}

// Create validates and persists a new convertible note.
func (s *Service) Create(ctx context.Context, n *ConvertibleNote) error {
	if n.CompanyID == "" {
		return fmt.Errorf("company_id is required")
	}
	if n.StakeholderID == "" {
		return fmt.Errorf("stakeholder_id is required")
	}
	if n.Capital <= 0 {
		return fmt.Errorf("capital must be positive")
	}
	if n.Type == "" {
		return fmt.Errorf("type is required")
	}
	if n.InterestRate != nil && *n.InterestRate < 0 {
		return fmt.Errorf("interest_rate must be non-negative")
	}
	now := time.Now().UTC()
	n.CreatedAt = now
	n.UpdatedAt = now
	if n.Status == "" {
		n.Status = StatusDraft
	}
	return s.repo.Create(ctx, n)
}

// Get retrieves a convertible note by ID.
func (s *Service) Get(ctx context.Context, id string) (*ConvertibleNote, error) {
	if id == "" {
		return nil, fmt.Errorf("note id is required")
	}
	return s.repo.Get(ctx, id)
}

// List returns all convertible notes for a company.
func (s *Service) List(ctx context.Context, companyID string) ([]*ConvertibleNote, error) {
	if companyID == "" {
		return nil, fmt.Errorf("company_id is required")
	}
	return s.repo.List(ctx, companyID)
}

// Update modifies an existing convertible note.
func (s *Service) Update(ctx context.Context, n *ConvertibleNote) error {
	if n.ID == "" {
		return fmt.Errorf("note id is required")
	}
	n.UpdatedAt = time.Now().UTC()
	return s.repo.Update(ctx, n)
}

// MarkConverted transitions a note to converted status.
func (s *Service) MarkConverted(ctx context.Context, id string) error {
	n, err := s.repo.Get(ctx, id)
	if err != nil {
		return fmt.Errorf("get note: %w", err)
	}
	if n.Status != StatusActive {
		return fmt.Errorf("note %s is not active (status=%s)", n.ID, n.Status)
	}
	n.Status = StatusConverted
	n.UpdatedAt = time.Now().UTC()
	return s.repo.Update(ctx, n)
}

// Cancel marks a convertible note as cancelled.
func (s *Service) Cancel(ctx context.Context, id string) error {
	n, err := s.repo.Get(ctx, id)
	if err != nil {
		return fmt.Errorf("get note: %w", err)
	}
	if n.Status == StatusConverted {
		return fmt.Errorf("cannot cancel a converted note")
	}
	n.Status = StatusCancelled
	n.UpdatedAt = time.Now().UTC()
	return s.repo.Update(ctx, n)
}
