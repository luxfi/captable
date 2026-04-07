package note

import "time"

// NoteType classifies the convertible instrument.
type NoteType string

const (
	TypeCCD  NoteType = "ccd"  // compulsory convertible debenture
	TypeOCD  NoteType = "ocd"  // optionally convertible debenture
	TypeNote NoteType = "note" // simple convertible note
)

// NoteStatus tracks the lifecycle of a convertible note.
type NoteStatus string

const (
	StatusDraft     NoteStatus = "draft"
	StatusActive    NoteStatus = "active"
	StatusPending   NoteStatus = "pending"
	StatusConverted NoteStatus = "converted"
	StatusExpired   NoteStatus = "expired"
	StatusCancelled NoteStatus = "cancelled"
)

// InterestMethod is simple or compound.
type InterestMethod string

const (
	InterestSimple   InterestMethod = "simple"
	InterestCompound InterestMethod = "compound"
)

// AccrualFrequency defines how often interest accrues.
type AccrualFrequency string

const (
	AccrualDaily        AccrualFrequency = "daily"
	AccrualMonthly      AccrualFrequency = "monthly"
	AccrualSemiAnnually AccrualFrequency = "semi_annually"
	AccrualAnnually     AccrualFrequency = "annually"
	AccrualContinuously AccrualFrequency = "continuously"
)

// PaymentSchedule defines when interest is paid.
type PaymentSchedule string

const (
	PaymentDeferred    PaymentSchedule = "deferred"
	PaymentAtMaturity  PaymentSchedule = "pay_at_maturity"
)

// ConversionTrigger defines what causes automatic conversion.
type ConversionTrigger string

const (
	TriggerQualifiedFinancing ConversionTrigger = "qualified_financing"
	TriggerMaturity           ConversionTrigger = "maturity"
	TriggerChangeOfControl    ConversionTrigger = "change_of_control"
	TriggerIPO                ConversionTrigger = "ipo"
	TriggerOptional           ConversionTrigger = "optional"
)

// ConvertibleNote represents a convertible debt instrument.
type ConvertibleNote struct {
	ID              string     `json:"id"`
	PublicID        string     `json:"public_id"` // e.g. CN-01
	CompanyID       string     `json:"company_id"`
	StakeholderID   string     `json:"stakeholder_id"`
	Type            NoteType   `json:"type"`
	Status          NoteStatus `json:"status"`
	Capital         float64    `json:"capital"` // principal amount

	ConversionCap   *float64 `json:"conversion_cap,omitempty"`
	DiscountRate    *float64 `json:"discount_rate,omitempty"`
	MFN             bool     `json:"mfn"`
	AdditionalTerms string   `json:"additional_terms,omitempty"`

	InterestRate            *float64          `json:"interest_rate,omitempty"`    // e.g. 0.06 for 6%
	InterestMethod          InterestMethod    `json:"interest_method,omitempty"`
	InterestAccrual         AccrualFrequency  `json:"interest_accrual,omitempty"`
	InterestPaymentSchedule PaymentSchedule   `json:"interest_payment_schedule,omitempty"`

	MaturityDate      *time.Time `json:"maturity_date,omitempty"`
	ConversionTrigger ConversionTrigger `json:"conversion_trigger,omitempty"`

	IssueDate         time.Time `json:"issue_date"`
	BoardApprovalDate time.Time `json:"board_approval_date"`
	CreatedAt         time.Time `json:"created_at"`
	UpdatedAt         time.Time `json:"updated_at"`
}

// AccruedInterest is the computed interest on a note at a point in time.
type AccruedInterest struct {
	NoteID    string    `json:"note_id"`
	Principal float64   `json:"principal"`
	Accrued   float64   `json:"accrued"`
	Total     float64   `json:"total"` // principal + accrued
	AsOf      time.Time `json:"as_of"`
}

// InterestCalculator computes accrued interest on a convertible note.
type InterestCalculator interface {
	Calculate(n *ConvertibleNote, asOf time.Time) (*AccruedInterest, error)
}
