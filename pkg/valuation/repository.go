package valuation

import "context"

// Repository defines storage for 409A valuation operations.
type Repository interface {
	Create(ctx context.Context, v *Valuation409A) error
	Get(ctx context.Context, id string) (*Valuation409A, error)
	GetLatest(ctx context.Context, companyID, shareClassID string) (*Valuation409A, error)
	List(ctx context.Context, companyID string) ([]*Valuation409A, error)
	Update(ctx context.Context, v *Valuation409A) error
}
