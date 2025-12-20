package transfer

import "context"

// Repository defines storage for transfer restrictions.
type Repository interface {
	CreateRestriction(ctx context.Context, r *Restriction) error
	GetRestriction(ctx context.Context, id string) (*Restriction, error)
	UpdateRestriction(ctx context.Context, r *Restriction) error
	ListRestrictions(ctx context.Context, companyID string) ([]*Restriction, error)
	ListByStakeholder(ctx context.Context, companyID, stakeholderID string) ([]*Restriction, error)
	ListByShareClass(ctx context.Context, companyID, shareClassID string) ([]*Restriction, error)
}
