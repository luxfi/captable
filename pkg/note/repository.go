package note

import "context"

// Repository defines storage for convertible note operations.
type Repository interface {
	Create(ctx context.Context, n *ConvertibleNote) error
	Get(ctx context.Context, id string) (*ConvertibleNote, error)
	Update(ctx context.Context, n *ConvertibleNote) error
	List(ctx context.Context, companyID string) ([]*ConvertibleNote, error)
	ListByStakeholder(ctx context.Context, stakeholderID string) ([]*ConvertibleNote, error)
	ListByStatus(ctx context.Context, companyID string, status NoteStatus) ([]*ConvertibleNote, error)
}
