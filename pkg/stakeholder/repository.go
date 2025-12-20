package stakeholder

import "context"

// Repository defines storage for stakeholder operations.
type Repository interface {
	Create(ctx context.Context, s *Stakeholder) error
	Get(ctx context.Context, id string) (*Stakeholder, error)
	Update(ctx context.Context, s *Stakeholder) error
	List(ctx context.Context, companyID string) ([]*Stakeholder, error)
	ListByTenant(ctx context.Context, tenantID string) ([]*Stakeholder, error)
	GetByEmail(ctx context.Context, companyID, email string) (*Stakeholder, error)
}
