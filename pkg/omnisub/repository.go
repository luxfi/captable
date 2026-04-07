package omnisub

import "context"

// Repository defines storage for omnibus and sub-account operations.
type Repository interface {
	// Omnibus accounts.
	CreateOmnibusAccount(ctx context.Context, a *OmnibusAccount) error
	GetOmnibusAccount(ctx context.Context, id string) (*OmnibusAccount, error)
	UpdateOmnibusAccount(ctx context.Context, a *OmnibusAccount) error
	ListOmnibusAccounts(ctx context.Context, companyID string) ([]*OmnibusAccount, error)

	// Sub-accounts.
	CreateSubAccount(ctx context.Context, a *SubAccount) error
	GetSubAccount(ctx context.Context, id string) (*SubAccount, error)
	UpdateSubAccount(ctx context.Context, a *SubAccount) error
	ListSubAccounts(ctx context.Context, omnibusAccountID string) ([]*SubAccount, error)
	GetSubAccountByStakeholder(ctx context.Context, omnibusAccountID, stakeholderID string) (*SubAccount, error)

	// Positions.
	UpsertPosition(ctx context.Context, p *Position) error
	GetPosition(ctx context.Context, subAccountID, securityID string) (*Position, error)
	ListPositions(ctx context.Context, subAccountID string) ([]*Position, error)

	// Orders.
	CreateOrder(ctx context.Context, o *Order) error
	GetOrder(ctx context.Context, id string) (*Order, error)
	UpdateOrder(ctx context.Context, o *Order) error
	ListOrders(ctx context.Context, subAccountID string) ([]*Order, error)

	// Corporate actions.
	CreateCorporateAction(ctx context.Context, ca *CorporateAction) error
	ListCorporateActions(ctx context.Context, securityID string) ([]*CorporateAction, error)

	// Tax lots.
	CreateTaxLot(ctx context.Context, t *TaxLot) error
	ListTaxLots(ctx context.Context, subAccountID, securityID string) ([]*TaxLot, error)

	// Tax statements.
	CreateTaxStatement(ctx context.Context, ts *TaxStatement) error
	GetTaxStatement(ctx context.Context, id string) (*TaxStatement, error)
	ListTaxStatements(ctx context.Context, subAccountID string) ([]*TaxStatement, error)
}
