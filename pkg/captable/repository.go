package captable

import "context"

// Repository defines the storage interface that consumers must implement.
// The captable package has no database dependency — all persistence is injected.
type Repository interface {
	// Companies
	CreateCompany(ctx context.Context, c *Company) error
	GetCompany(ctx context.Context, id string) (*Company, error)
	UpdateCompany(ctx context.Context, c *Company) error
	ListCompanies(ctx context.Context, tenantID string, params ListParams) ([]*Company, error)

	// Share classes
	CreateShareClass(ctx context.Context, sc *ShareClass) error
	GetShareClass(ctx context.Context, id string) (*ShareClass, error)
	UpdateShareClass(ctx context.Context, sc *ShareClass) error
	ListShareClasses(ctx context.Context, companyID string) ([]*ShareClass, error)

	// Cap table entries
	CreateEntry(ctx context.Context, e *Entry) error
	GetEntry(ctx context.Context, id string) (*Entry, error)
	UpdateEntry(ctx context.Context, e *Entry) error
	ListEntries(ctx context.Context, companyID string, params ListParams) ([]*Entry, error)
	ListEntriesByStakeholder(ctx context.Context, companyID, stakeholderID string) ([]*Entry, error)

	// Vesting
	CreateVestingSchedule(ctx context.Context, v *VestingSchedule) error
	GetVestingSchedule(ctx context.Context, id string) (*VestingSchedule, error)
	UpdateVestingSchedule(ctx context.Context, v *VestingSchedule) error

	// Option grants
	CreateOptionGrant(ctx context.Context, og *OptionGrant) error
	GetOptionGrant(ctx context.Context, id string) (*OptionGrant, error)
	UpdateOptionGrant(ctx context.Context, og *OptionGrant) error
	ListOptionGrants(ctx context.Context, companyID string, params ListParams) ([]*OptionGrant, error)

	// Equity plans
	CreateEquityPlan(ctx context.Context, ep *EquityPlan) error
	GetEquityPlan(ctx context.Context, id string) (*EquityPlan, error)
	UpdateEquityPlan(ctx context.Context, ep *EquityPlan) error
	ListEquityPlans(ctx context.Context, companyID string) ([]*EquityPlan, error)
}
