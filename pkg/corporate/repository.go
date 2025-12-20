package corporate

import "context"

// Repository defines storage for corporate actions.
type Repository interface {
	CreateAction(ctx context.Context, a *Action) error
	GetAction(ctx context.Context, id string) (*Action, error)
	UpdateAction(ctx context.Context, a *Action) error
	ListActions(ctx context.Context, companyID string) ([]*Action, error)
}
