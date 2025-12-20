package captable

import "time"

// Company represents a legal entity whose equity is tracked on the cap table.
type Company struct {
	ID           string            `json:"id"`
	TenantID     string            `json:"tenant_id"`
	Name         string            `json:"name"`
	LegalName    string            `json:"legal_name"`
	Jurisdiction string            `json:"jurisdiction"`
	EntityType   string            `json:"entity_type"` // corporation, llc, lp, trust
	EIN          string            `json:"ein,omitempty"`
	StateOfInc   string            `json:"state_of_inc,omitempty"`
	Status       string            `json:"status"` // active, dissolved, merged
	Meta         map[string]string `json:"meta,omitempty"`
	CreatedAt    time.Time         `json:"created_at"`
	UpdatedAt    time.Time         `json:"updated_at"`
}

// ShareClass defines a class of equity or convertible instrument.
type ShareClass struct {
	ID                    string  `json:"id"`
	CompanyID             string  `json:"company_id"`
	Name                  string  `json:"name"`
	Symbol                string  `json:"symbol,omitempty"`
	AuthorizedShares      int64   `json:"authorized_shares"`
	IssuedShares          int64   `json:"issued_shares"`
	OutstandingShares     int64   `json:"outstanding_shares"`
	ParValue              float64 `json:"par_value"`
	PricePerShare         float64 `json:"price_per_share"`
	VotingRights          bool    `json:"voting_rights"`
	VotesPerShare         int     `json:"votes_per_share"`
	LiquidationPreference float64 `json:"liquidation_preference"`
	LiquidationMultiple   float64 `json:"liquidation_multiple"`
	Participating         bool    `json:"participating"`
	DividendRate          float64 `json:"dividend_rate,omitempty"`
	ConversionRatio       float64 `json:"conversion_ratio,omitempty"`
	AntiDilution          string  `json:"anti_dilution,omitempty"` // broad_based, narrow_based, full_ratchet
	Seniority             int     `json:"seniority"`              // higher = more senior
	Type                  string  `json:"type"`                   // common, preferred_a, preferred_b, convertible_note, safe, warrant
	Status                string  `json:"status"`                 // active, retired, converted
	CreatedAt             time.Time `json:"created_at"`
	UpdatedAt             time.Time `json:"updated_at"`
}

// Entry is a single record on the cap table representing an ownership position.
type Entry struct {
	ID              string    `json:"id"`
	CompanyID       string    `json:"company_id"`
	StakeholderID   string    `json:"stakeholder_id"`
	ShareClassID    string    `json:"share_class_id"`
	CertificateNo   string   `json:"certificate_no,omitempty"`
	Shares          int64     `json:"shares"`
	PricePerShare   float64   `json:"price_per_share"`
	TotalValue      float64   `json:"total_value"`
	IssueDate       time.Time `json:"issue_date"`
	VestingStartDate *time.Time `json:"vesting_start_date,omitempty"`
	Type            string    `json:"type"`   // issuance, transfer_in, transfer_out, cancellation, exercise, conversion
	Status          string    `json:"status"` // active, cancelled, transferred, converted
	LedgerRef       string    `json:"ledger_ref,omitempty"` // reference to on-chain or external ledger
	Notes           string    `json:"notes,omitempty"`
	CreatedAt       time.Time `json:"created_at"`
	UpdatedAt       time.Time `json:"updated_at"`
}

// Summary is a computed snapshot of a company's cap table.
type Summary struct {
	CompanyID          string          `json:"company_id"`
	TotalAuthorized    int64           `json:"total_authorized"`
	TotalIssued        int64           `json:"total_issued"`
	TotalOutstanding   int64           `json:"total_outstanding"`
	TotalStakeholders  int             `json:"total_stakeholders"`
	FullyDilutedShares int64           `json:"fully_diluted_shares"`
	ByClass            []ClassSummary  `json:"by_class"`
	AsOf               time.Time       `json:"as_of"`
}

// ClassSummary is the per-class breakdown within a Summary.
type ClassSummary struct {
	ShareClassID      string  `json:"share_class_id"`
	Name              string  `json:"name"`
	Type              string  `json:"type"`
	Authorized        int64   `json:"authorized"`
	Issued            int64   `json:"issued"`
	Outstanding       int64   `json:"outstanding"`
	OwnershipPercent  float64 `json:"ownership_percent"`
	FullyDilutedPct   float64 `json:"fully_diluted_pct"`
}

// VestingSchedule defines how shares vest over time.
type VestingSchedule struct {
	ID              string    `json:"id"`
	EntryID         string    `json:"entry_id"`
	TotalShares     int64     `json:"total_shares"`
	VestedShares    int64     `json:"vested_shares"`
	CliffMonths     int       `json:"cliff_months"`
	VestingMonths   int       `json:"vesting_months"`
	StartDate       time.Time `json:"start_date"`
	CliffDate       time.Time `json:"cliff_date"`
	EndDate         time.Time `json:"end_date"`
	Accelerated     bool      `json:"accelerated"`
	SingleTrigger   bool      `json:"single_trigger"`
	DoubleTrigger   bool      `json:"double_trigger"`
}

// OptionGrant represents a stock option grant.
type OptionGrant struct {
	ID              string     `json:"id"`
	CompanyID       string     `json:"company_id"`
	StakeholderID   string     `json:"stakeholder_id"`
	PlanID          string     `json:"plan_id"`
	GrantDate       time.Time  `json:"grant_date"`
	ExpirationDate  time.Time  `json:"expiration_date"`
	OptionsGranted  int64      `json:"options_granted"`
	OptionsExercised int64     `json:"options_exercised"`
	OptionsCancelled int64     `json:"options_cancelled"`
	OptionsVested   int64      `json:"options_vested"`
	StrikePrice     float64    `json:"strike_price"`
	FMVAtGrant      float64    `json:"fmv_at_grant"`
	Type            string     `json:"type"`   // iso, nso
	Status          string     `json:"status"` // active, exercised, expired, cancelled
	VestingID       string     `json:"vesting_id,omitempty"`
}

// EquityPlan is a stock option or equity incentive plan (e.g., 2024 EIP).
type EquityPlan struct {
	ID              string    `json:"id"`
	CompanyID       string    `json:"company_id"`
	Name            string    `json:"name"`
	SharesReserved  int64     `json:"shares_reserved"`
	SharesGranted   int64     `json:"shares_granted"`
	SharesExercised int64     `json:"shares_exercised"`
	SharesAvailable int64     `json:"shares_available"`
	ShareClassID    string    `json:"share_class_id"`
	BoardApprovalDate time.Time `json:"board_approval_date"`
	Status          string    `json:"status"` // active, terminated, expired
	CreatedAt       time.Time `json:"created_at"`
}

// ListParams controls pagination and filtering for list operations.
type ListParams struct {
	CompanyID string `json:"company_id,omitempty"`
	Limit     int    `json:"limit,omitempty"`
	Offset    int    `json:"offset,omitempty"`
	Status    string `json:"status,omitempty"`
}
