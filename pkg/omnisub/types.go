package omnisub

import "time"

// AccountStatus tracks omnibus/sub-account lifecycle.
type AccountStatus string

const (
	StatusActive   AccountStatus = "active"
	StatusSuspended AccountStatus = "suspended"
	StatusClosed   AccountStatus = "closed"
)

// OrderSide is buy or sell.
type OrderSide string

const (
	SideBuy  OrderSide = "buy"
	SideSell OrderSide = "sell"
)

// OrderStatus tracks order lifecycle.
type OrderStatus string

const (
	OrderPending   OrderStatus = "pending"
	OrderFilled    OrderStatus = "filled"
	OrderPartial   OrderStatus = "partial"
	OrderCancelled OrderStatus = "cancelled"
	OrderRejected  OrderStatus = "rejected"
)

// CorporateActionType classifies corporate actions.
type CorporateActionType string

const (
	ActionDividend     CorporateActionType = "dividend"
	ActionSplit        CorporateActionType = "split"
	ActionReverseSplit CorporateActionType = "reverse_split"
	ActionMerger       CorporateActionType = "merger"
	ActionSpinoff      CorporateActionType = "spinoff"
)

// OmnibusAccount is the master account that holds positions on behalf of sub-accounts.
type OmnibusAccount struct {
	ID         string        `json:"id"`
	CompanyID  string        `json:"company_id"`
	Name       string        `json:"name"`
	CustodianID string       `json:"custodian_id,omitempty"` // external custodian reference
	Status     AccountStatus `json:"status"`
	CreatedAt  time.Time     `json:"created_at"`
	UpdatedAt  time.Time     `json:"updated_at"`
}

// SubAccount is an individual stakeholder's account within an omnibus account.
type SubAccount struct {
	ID               string        `json:"id"`
	OmnibusAccountID string        `json:"omnibus_account_id"`
	StakeholderID    string        `json:"stakeholder_id"`
	AccountNumber    string        `json:"account_number"`
	Status           AccountStatus `json:"status"`
	CreatedAt        time.Time     `json:"created_at"`
	UpdatedAt        time.Time     `json:"updated_at"`
}

// Position represents a security holding within a sub-account.
type Position struct {
	ID           string    `json:"id"`
	SubAccountID string    `json:"sub_account_id"`
	SecurityID   string    `json:"security_id"`
	Quantity     int64     `json:"quantity"`
	CostBasis    float64   `json:"cost_basis"`
	MarketValue  float64   `json:"market_value"`
	UpdatedAt    time.Time `json:"updated_at"`
}

// Order is a buy/sell order within a sub-account.
type Order struct {
	ID           string      `json:"id"`
	SubAccountID string      `json:"sub_account_id"`
	SecurityID   string      `json:"security_id"`
	Side         OrderSide   `json:"side"`
	Quantity     int64       `json:"quantity"`
	FilledQty    int64       `json:"filled_qty"`
	Price        float64     `json:"price"`
	Status       OrderStatus `json:"status"`
	SubmittedAt  time.Time   `json:"submitted_at"`
	FilledAt     *time.Time  `json:"filled_at,omitempty"`
	CreatedAt    time.Time   `json:"created_at"`
	UpdatedAt    time.Time   `json:"updated_at"`
}

// CorporateAction records a corporate event applied to positions.
type CorporateAction struct {
	ID           string              `json:"id"`
	SecurityID   string              `json:"security_id"`
	Type         CorporateActionType `json:"type"`
	Description  string              `json:"description"`
	RecordDate   time.Time           `json:"record_date"`
	EffectiveDate time.Time          `json:"effective_date"`
	Ratio        *float64            `json:"ratio,omitempty"`         // for splits
	AmountPerShare *float64          `json:"amount_per_share,omitempty"` // for dividends
	CreatedAt    time.Time           `json:"created_at"`
}

// TaxLot tracks cost basis for a specific acquisition of shares.
type TaxLot struct {
	ID           string    `json:"id"`
	SubAccountID string    `json:"sub_account_id"`
	SecurityID   string    `json:"security_id"`
	Quantity     int64     `json:"quantity"`
	CostPerShare float64   `json:"cost_per_share"`
	AcquiredDate time.Time `json:"acquired_date"`
	CreatedAt    time.Time `json:"created_at"`
}

// TaxStatement is a yearly tax summary for a sub-account.
type TaxStatement struct {
	ID           string    `json:"id"`
	SubAccountID string    `json:"sub_account_id"`
	TaxYear      int       `json:"tax_year"`
	FormType     string    `json:"form_type"` // 1099-B, 1099-DIV, K-1
	TotalGains   float64   `json:"total_gains"`
	TotalLosses  float64   `json:"total_losses"`
	TotalDividends float64 `json:"total_dividends"`
	DocumentID   string    `json:"document_id,omitempty"` // generated PDF reference
	GeneratedAt  time.Time `json:"generated_at"`
	CreatedAt    time.Time `json:"created_at"`
}
