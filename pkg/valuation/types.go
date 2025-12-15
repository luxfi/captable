package valuation

import "time"

// Valuation409A represents a 409A fair market value determination.
type Valuation409A struct {
	ID                string    `json:"id"`
	CompanyID         string    `json:"company_id"`
	EffectiveDate     time.Time `json:"effective_date"`
	ExpirationDate    time.Time `json:"expiration_date"`
	FairMarketValue   string    `json:"fair_market_value"` // per share, stored as string for precision
	ShareClassID      string    `json:"share_class_id"`
	Method            string    `json:"method"`   // dcf, market_comparable, asset_based, backsolve, opm
	Provider          string    `json:"provider"` // valuation firm name
	ReportURL         string    `json:"report_url,omitempty"`
	Status            string    `json:"status"` // draft, final, expired
	BoardApprovalDate time.Time `json:"board_approval_date"`
	IsSafeHarbor      bool      `json:"is_safe_harbor"`
	Notes             string    `json:"notes,omitempty"`
	CreatedAt         time.Time `json:"created_at"`
	UpdatedAt         time.Time `json:"updated_at"`
}

// ValuationHistory is a list of 409A valuations for a company, sorted by effective date descending.
type ValuationHistory struct {
	CompanyID  string         `json:"company_id"`
	Valuations []Valuation409A `json:"valuations"`
}
