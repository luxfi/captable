package warrant

import "time"

// WarrantStatus tracks the lifecycle of a warrant.
type WarrantStatus string

const (
	StatusDraft     WarrantStatus = "draft"
	StatusActive    WarrantStatus = "active"
	StatusExercised WarrantStatus = "exercised"
	StatusExpired   WarrantStatus = "expired"
	StatusCancelled WarrantStatus = "cancelled"
)

// ExerciseType is the method of warrant exercise.
type ExerciseType string

const (
	ExerciseCash     ExerciseType = "cash"
	ExerciseCashless ExerciseType = "cashless"
	ExerciseNet      ExerciseType = "net" // net exercise (same as cashless)
)

// Warrant represents a right to purchase equity at a fixed price.
type Warrant struct {
	ID              string        `json:"id"`
	PublicID        string        `json:"public_id"` // e.g. W-01
	CompanyID       string        `json:"company_id"`
	StakeholderID   string        `json:"stakeholder_id"`
	ShareClassID    string        `json:"share_class_id"`
	Status          WarrantStatus `json:"status"`
	ExerciseType    ExerciseType  `json:"exercise_type"`
	ExercisePrice   float64       `json:"exercise_price"`
	Quantity        int64         `json:"quantity"`          // number of shares purchasable
	QuantityExercised int64       `json:"quantity_exercised"`
	IssueDate       time.Time     `json:"issue_date"`
	ExpirationDate  time.Time     `json:"expiration_date"`
	BoardApprovalDate time.Time   `json:"board_approval_date"`
	CreatedAt       time.Time     `json:"created_at"`
	UpdatedAt       time.Time     `json:"updated_at"`
}

// ExerciseRequest is the input for exercising a warrant.
type ExerciseRequest struct {
	WarrantID    string       `json:"warrant_id"`
	Quantity     int64        `json:"quantity"`
	ExerciseType ExerciseType `json:"exercise_type"`
	FMVAtExercise float64    `json:"fmv_at_exercise,omitempty"` // required for cashless
}

// ExerciseResult holds the outcome of a warrant exercise.
type ExerciseResult struct {
	WarrantID     string  `json:"warrant_id"`
	SharesIssued  int64   `json:"shares_issued"`
	CashPaid      float64 `json:"cash_paid,omitempty"`
	Remaining     int64   `json:"remaining"` // unexercised quantity
}
