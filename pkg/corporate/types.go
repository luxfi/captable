package corporate

import "time"

// Action represents a corporate action that affects the cap table.
type Action struct {
	ID           string    `json:"id"`
	CompanyID    string    `json:"company_id"`
	Type         string    `json:"type"`   // stock_split, reverse_split, merger, spinoff, reclassification, recapitalization
	Status       string    `json:"status"` // proposed, approved, executed, cancelled
	Description  string    `json:"description"`
	EffectiveDate time.Time `json:"effective_date"`
	ApprovedDate *time.Time `json:"approved_date,omitempty"`
	ExecutedDate *time.Time `json:"executed_date,omitempty"`
	CreatedAt    time.Time  `json:"created_at"`
}

// StockSplit defines the parameters for a stock split or reverse split.
type StockSplit struct {
	ActionID     string `json:"action_id"`
	ShareClassID string `json:"share_class_id"`
	Ratio        float64 `json:"ratio"` // e.g., 2.0 for 2:1 split, 0.1 for 1:10 reverse split
	NewAuthorized int64  `json:"new_authorized,omitempty"` // if authorized shares change
}

// Merger defines merger/acquisition parameters.
type Merger struct {
	ActionID         string  `json:"action_id"`
	TargetCompanyID  string  `json:"target_company_id,omitempty"`
	AcquirerID       string  `json:"acquirer_id,omitempty"`
	ExchangeRatio    float64 `json:"exchange_ratio,omitempty"` // shares of acquirer per target share
	CashConsideration float64 `json:"cash_consideration,omitempty"`
	Type             string  `json:"type"` // acquisition, merger_of_equals, reverse_merger
}

// Reclassification changes the terms of an existing share class.
type Reclassification struct {
	ActionID         string `json:"action_id"`
	FromShareClassID string `json:"from_share_class_id"`
	ToShareClassID   string `json:"to_share_class_id"`
	ConversionRatio  float64 `json:"conversion_ratio"`
}

// SplitResult captures the before/after state for each affected stakeholder.
type SplitResult struct {
	StakeholderID string `json:"stakeholder_id"`
	SharesBefore  int64  `json:"shares_before"`
	SharesAfter   int64  `json:"shares_after"`
	Fractional    float64 `json:"fractional,omitempty"` // fractional share remainder
}
