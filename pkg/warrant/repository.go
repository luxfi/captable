package warrant

import "context"

// Repository defines storage for warrant operations.
type Repository interface {
	Create(ctx context.Context, w *Warrant) error
	Get(ctx context.Context, id string) (*Warrant, error)
	Update(ctx context.Context, w *Warrant) error
	List(ctx context.Context, companyID string) ([]*Warrant, error)
	ListByStakeholder(ctx context.Context, stakeholderID string) ([]*Warrant, error)
	ListByStatus(ctx context.Context, companyID string, status WarrantStatus) ([]*Warrant, error)
}
