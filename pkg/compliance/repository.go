package compliance

import "context"

// Repository defines storage for compliance filings.
type Repository interface {
	CreateFormD(ctx context.Context, f *FormD) error
	GetFormD(ctx context.Context, id string) (*FormD, error)
	UpdateFormD(ctx context.Context, f *FormD) error
	ListFormDs(ctx context.Context, companyID string) ([]*FormD, error)

	CreateBlueSkyFiling(ctx context.Context, b *BlueSkyFiling) error
	GetBlueSkyFiling(ctx context.Context, id string) (*BlueSkyFiling, error)
	UpdateBlueSkyFiling(ctx context.Context, b *BlueSkyFiling) error
	ListBlueSkyFilings(ctx context.Context, companyID string) ([]*BlueSkyFiling, error)
	ListBlueSkyByState(ctx context.Context, companyID, state string) ([]*BlueSkyFiling, error)
}
