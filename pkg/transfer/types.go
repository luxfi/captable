package transfer

import "time"

// Restriction defines a transfer restriction applied to shares.
type Restriction struct {
	ID            string     `json:"id"`
	CompanyID     string     `json:"company_id"`
	ShareClassID  string     `json:"share_class_id,omitempty"` // empty = applies to all classes
	StakeholderID string     `json:"stakeholder_id,omitempty"` // empty = applies to all holders
	Type          string     `json:"type"`                     // rule_144, lockup, rofr, board_approval, legend
	StartDate     time.Time  `json:"start_date"`
	EndDate       *time.Time `json:"end_date,omitempty"`       // nil = indefinite
	Status        string     `json:"status"`                   // active, expired, waived
	Conditions    string     `json:"conditions,omitempty"`     // human-readable restriction description
	CreatedAt     time.Time  `json:"created_at"`
}

// Rule144Check contains the inputs for a Rule 144 holding period check.
type Rule144Check struct {
	StakeholderID string    `json:"stakeholder_id"`
	IsAffiliate   bool      `json:"is_affiliate"`
	AcquiredDate  time.Time `json:"acquired_date"`
	IsReporting   bool      `json:"is_reporting"` // whether the issuer is SEC-reporting
	SharesHeld    int64     `json:"shares_held"`
	SharesToSell  int64     `json:"shares_to_sell"`
	AvgWeeklyVol  int64     `json:"avg_weekly_volume,omitempty"` // for volume limit calc
}

// Rule144Result is the outcome of a Rule 144 eligibility check.
type Rule144Result struct {
	Eligible       bool   `json:"eligible"`
	HoldingPeriod  int    `json:"holding_period_months"` // required holding period
	MonthsHeld     int    `json:"months_held"`
	VolumeLimit    int64  `json:"volume_limit,omitempty"` // max shares sellable per 90 days
	Reason         string `json:"reason,omitempty"`       // if not eligible, why
}

// LockupPeriod defines a post-IPO or post-issuance lockup.
type LockupPeriod struct {
	ID            string     `json:"id"`
	CompanyID     string     `json:"company_id"`
	StakeholderID string     `json:"stakeholder_id,omitempty"`
	ShareClassID  string     `json:"share_class_id,omitempty"`
	StartDate     time.Time  `json:"start_date"`
	EndDate       time.Time  `json:"end_date"`
	Type          string     `json:"type"` // ipo_lockup, contractual, regulatory
	Status        string     `json:"status"`
}

// CheckRequest is the input for evaluating whether a transfer is permitted.
type CheckRequest struct {
	CompanyID       string `json:"company_id"`
	FromStakeholder string `json:"from_stakeholder"`
	ToStakeholder   string `json:"to_stakeholder"`
	ShareClassID    string `json:"share_class_id"`
	Shares          int64  `json:"shares"`
}

// CheckResult is the outcome of a transfer restriction check.
type CheckResult struct {
	Allowed      bool     `json:"allowed"`
	Restrictions []string `json:"restrictions,omitempty"` // active restriction descriptions
	Errors       []string `json:"errors,omitempty"`
}
