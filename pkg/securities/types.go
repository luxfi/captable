package securities

import "time"

// Security represents a registered security instrument.
type Security struct {
	ID              string    `json:"id"`
	CompanyID       string    `json:"company_id"`
	ShareClassID    string    `json:"share_class_id"`
	Name            string    `json:"name"`
	Type            string    `json:"type"` // equity, debt, convertible_note, safe, warrant
	CUSIP           string    `json:"cusip,omitempty"`
	ISIN            string    `json:"isin,omitempty"`
	Exemption       string    `json:"exemption,omitempty"`        // reg_d_506b, reg_d_506c, reg_s, reg_a, reg_cf
	MaxOffering     float64   `json:"max_offering,omitempty"`     // max $ amount of offering
	AmountRaised    float64   `json:"amount_raised,omitempty"`
	MinInvestment   float64   `json:"min_investment,omitempty"`
	MaxInvestors    int       `json:"max_investors,omitempty"`
	CurrentInvestors int      `json:"current_investors,omitempty"`
	Status          string    `json:"status"` // draft, offering, closed, cancelled
	OfferingDate    *time.Time `json:"offering_date,omitempty"`
	ClosingDate     *time.Time `json:"closing_date,omitempty"`
	CreatedAt       time.Time `json:"created_at"`
	UpdatedAt       time.Time `json:"updated_at"`
}

// IssuanceRequest is the input for issuing securities to a stakeholder.
type IssuanceRequest struct {
	SecurityID    string  `json:"security_id"`
	StakeholderID string  `json:"stakeholder_id"`
	Shares        int64   `json:"shares"`
	PricePerShare float64 `json:"price_per_share"`
	PaymentMethod string  `json:"payment_method,omitempty"` // wire, ach, crypto, cashless
	Consideration string  `json:"consideration,omitempty"`  // cash, ip, services
}

// TransferRequest is the input for transferring securities between stakeholders.
type TransferRequest struct {
	SecurityID       string  `json:"security_id"`
	FromStakeholder  string  `json:"from_stakeholder"`
	ToStakeholder    string  `json:"to_stakeholder"`
	Shares           int64   `json:"shares"`
	PricePerShare    float64 `json:"price_per_share"`
	TransferDate     string  `json:"transfer_date,omitempty"`
	RestrictionCheck bool    `json:"restriction_check"` // whether to enforce transfer restrictions
}

// CancellationRequest is the input for cancelling issued securities.
type CancellationRequest struct {
	SecurityID    string `json:"security_id"`
	StakeholderID string `json:"stakeholder_id"`
	Shares        int64  `json:"shares"`
	Reason        string `json:"reason"`
}

// ConversionRequest is the input for converting securities (e.g., note to equity).
type ConversionRequest struct {
	SecurityID       string  `json:"security_id"`
	StakeholderID    string  `json:"stakeholder_id"`
	TargetClassID    string  `json:"target_class_id"`    // share class to convert into
	ConversionShares int64   `json:"conversion_shares"`  // number of new shares
	ConversionPrice  float64 `json:"conversion_price"`
}

// Certificate represents a stock certificate.
type Certificate struct {
	ID              string    `json:"id"`
	SecurityID      string    `json:"security_id"`
	StakeholderID   string    `json:"stakeholder_id"`
	CertificateNo   string   `json:"certificate_no"`
	Shares          int64     `json:"shares"`
	IssueDate       time.Time `json:"issue_date"`
	Status          string    `json:"status"` // active, cancelled, transferred
	LegendText      string    `json:"legend_text,omitempty"`
}

// Ledger is an immutable audit trail of all security movements.
type LedgerEntry struct {
	ID              string    `json:"id"`
	SecurityID      string    `json:"security_id"`
	CompanyID       string    `json:"company_id"`
	FromStakeholder string    `json:"from_stakeholder,omitempty"` // empty for issuance
	ToStakeholder   string    `json:"to_stakeholder,omitempty"`   // empty for cancellation
	Shares          int64     `json:"shares"`
	PricePerShare   float64   `json:"price_per_share"`
	Action          string    `json:"action"` // issue, transfer, cancel, convert, split, reverse_split
	Timestamp       time.Time `json:"timestamp"`
	Reference       string    `json:"reference,omitempty"` // external reference (tx hash, etc.)
}
