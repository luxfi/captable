package disclosure

import (
	"context"
	"fmt"
	"time"
)

// Service provides investor disclosure operations.
type Service struct {
	repo Repository
}

func NewService(repo Repository) *Service {
	return &Service{repo: repo}
}

// Create saves a new disclosure document.
func (s *Service) Create(ctx context.Context, d *Disclosure) error {
	if d.Name == "" {
		return fmt.Errorf("name required")
	}
	if d.CompanyID == "" {
		return fmt.Errorf("company_id required")
	}
	if d.Type == "" {
		return fmt.Errorf("type required")
	}
	if d.CreatedAt.IsZero() {
		d.CreatedAt = time.Now().UTC()
	}
	return s.repo.Create(ctx, d)
}

// Get returns a disclosure by ID.
func (s *Service) Get(ctx context.Context, id string) (*Disclosure, error) {
	return s.repo.Get(ctx, id)
}

// List returns disclosures matching the filter.
func (s *Service) List(ctx context.Context, filter DisclosureFilter) ([]*Disclosure, error) {
	return s.repo.List(ctx, filter)
}

// Deliver marks a disclosure as delivered to a stakeholder.
func (s *Service) Deliver(ctx context.Context, id, stakeholderID string) error {
	return s.repo.MarkDelivered(ctx, id, stakeholderID)
}

// Acknowledge marks a disclosure as acknowledged by a stakeholder.
func (s *Service) Acknowledge(ctx context.Context, id, stakeholderID string) error {
	return s.repo.Acknowledge(ctx, id, stakeholderID)
}
