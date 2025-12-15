package comms

import (
	"context"
	"fmt"
	"time"
)

// Service provides investor communication operations.
type Service struct {
	repo Repository
}

func NewService(repo Repository) *Service {
	return &Service{repo: repo}
}

// Create saves a new investor notice.
func (s *Service) Create(ctx context.Context, n *Notice) error {
	if n.Subject == "" {
		return fmt.Errorf("subject required")
	}
	if n.CompanyID == "" {
		return fmt.Errorf("company_id required")
	}
	if n.Type == "" {
		n.Type = "general"
	}
	if n.CreatedAt.IsZero() {
		n.CreatedAt = time.Now().UTC()
	}
	return s.repo.CreateNotice(ctx, n)
}

// Get returns a notice by ID.
func (s *Service) Get(ctx context.Context, id string) (*Notice, error) {
	return s.repo.GetNotice(ctx, id)
}

// List returns notices matching the filter.
func (s *Service) List(ctx context.Context, filter NoticeFilter) ([]*Notice, error) {
	return s.repo.ListNotices(ctx, filter)
}

// Send marks a notice as sent.
func (s *Service) Send(ctx context.Context, id string) error {
	return s.repo.MarkSent(ctx, id)
}
