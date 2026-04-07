package safe

import "context"

// Repository defines storage for SAFE operations.
type Repository interface {
	Create(ctx context.Context, s *Safe) error
	Get(ctx context.Context, id string) (*Safe, error)
	Update(ctx context.Context, s *Safe) error
	List(ctx context.Context, companyID string) ([]*Safe, error)
	ListByStakeholder(ctx context.Context, stakeholderID string) ([]*Safe, error)
	ListByStatus(ctx context.Context, companyID string, status SafeStatus) ([]*Safe, error)
}
