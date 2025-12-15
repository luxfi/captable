package exercise

import "time"

// ExerciseRequest represents a request to exercise stock options.
type ExerciseRequest struct {
	GrantID         string    `json:"grant_id"`
	StakeholderID   string    `json:"stakeholder_id"`
	SharesExercised int64     `json:"shares_exercised"`
	ExerciseDate    time.Time `json:"exercise_date"`
	ExerciseType    string    `json:"exercise_type"`  // cash, cashless, net
	PaymentMethod   string    `json:"payment_method"` // wire, ach, check, stock_swap
	CurrentFMV      string    `json:"current_fmv"`    // per share, string for precision
}

// ExerciseResult represents the outcome of an option exercise.
type ExerciseResult struct {
	ID                string    `json:"id"`
	GrantID           string    `json:"grant_id"`
	SharesExercised   int64     `json:"shares_exercised"`
	ExercisePrice     float64   `json:"exercise_price"`      // total cost = shares * strike
	FMVAtExercise     float64   `json:"fmv_at_exercise"`     // per share
	TotalCost         float64   `json:"total_cost"`           // exercise_price for cash; varies for cashless
	TaxableSpread     float64   `json:"taxable_spread"`       // (FMV - strike) * shares
	AMTAdjustment     float64   `json:"amt_adjustment"`       // ISO only: spread is AMT preference item
	TaxWithholding    float64   `json:"tax_withholding"`      // NSO: computed withholding
	NetSharesIssued   int64     `json:"net_shares_issued"`    // shares after withholding (cashless)
	Section83bElection bool    `json:"section_83b_election"`
	Section83bDeadline time.Time `json:"section_83b_deadline"` // exercise_date + 30 days
	ExerciseDate      time.Time `json:"exercise_date"`
	CreatedAt         time.Time `json:"created_at"`
}

// TaxCalculation breaks down the tax implications of an option exercise.
type TaxCalculation struct {
	ExerciseType              string  `json:"exercise_type"` // iso, nso
	Spread                    float64 `json:"spread"`
	OrdinaryIncome            float64 `json:"ordinary_income"`
	AMTPreferenceItem         float64 `json:"amt_preference_item"`
	FederalWithholding        float64 `json:"federal_withholding"`
	StateWithholding          float64 `json:"state_withholding"`
	SocialSecurityWithholding float64 `json:"social_security_withholding"`
	MedicareWithholding       float64 `json:"medicare_withholding"`
	TotalWithholding          float64 `json:"total_withholding"`
}
