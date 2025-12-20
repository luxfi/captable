package dividend

import "context"

// Repository defines storage for dividend operations.
type Repository interface {
	CreateDeclaration(ctx context.Context, d *Declaration) error
	GetDeclaration(ctx context.Context, id string) (*Declaration, error)
	UpdateDeclaration(ctx context.Context, d *Declaration) error
	ListDeclarations(ctx context.Context, companyID string) ([]*Declaration, error)

	CreateDistribution(ctx context.Context, d *Distribution) error
	GetDistribution(ctx context.Context, id string) (*Distribution, error)
	UpdateDistribution(ctx context.Context, d *Distribution) error
	ListDistributions(ctx context.Context, declarationID string) ([]*Distribution, error)
	ListByStakeholder(ctx context.Context, stakeholderID string) ([]*Distribution, error)
}
