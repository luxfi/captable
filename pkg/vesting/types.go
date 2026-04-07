package vesting

import "time"

// ScheduleType distinguishes standard vs custom vesting.
type ScheduleType string

const (
	ScheduleStandard ScheduleType = "standard"
	ScheduleCustom   ScheduleType = "custom"
)

// VestingType is time-based or milestone-based.
type VestingType string

const (
	VestingTimeBased  VestingType = "time_based"
	VestingMilestone  VestingType = "milestone"
)

// Frequency defines how often vesting occurs.
type Frequency string

const (
	FrequencyDaily      Frequency = "every_day"
	FrequencyWeekly     Frequency = "every_week"
	FrequencyMonthly    Frequency = "every_month"
	FrequencyBiMonthly  Frequency = "every_2_months"
	FrequencyQuarterly  Frequency = "every_3_months"
	FrequencySemiAnnual Frequency = "every_6_months"
	FrequencyAnnual     Frequency = "every_12_months"
)

// OccursDay defines which day within the period vesting occurs.
type OccursDay string

const (
	OccursSameDay       OccursDay = "same_day"
	OccursSameDayMinus1 OccursDay = "same_day_minus_one"
	OccursFirstDay      OccursDay = "first_day"
	OccursLastDay       OccursDay = "last_day"
)

// CliffUnit is the unit for cliff length.
type CliffUnit string

const (
	CliffDays   CliffUnit = "days"
	CliffMonths CliffUnit = "months"
	CliffYears  CliffUnit = "years"
)

// PerformanceConditionType classifies the performance metric.
type PerformanceConditionType string

const (
	ConditionMarketBased    PerformanceConditionType = "market_based"
	ConditionNonMarketBased PerformanceConditionType = "non_market_based"
)

// Schedule defines a vesting schedule with optional cliff and tranches.
type Schedule struct {
	ID            string       `json:"id"`
	CompanyID     string       `json:"company_id"`
	EquityPlanID  string       `json:"equity_plan_id,omitempty"`
	ScheduleName  string       `json:"schedule_name"`
	VestingTerm   string       `json:"vesting_term"` // displayed in board consent
	ScheduleType  ScheduleType `json:"schedule_type"`

	// Standard schedule fields.
	VestingOccurs    Frequency `json:"vesting_occurs,omitempty"`
	VestingOccursDay OccursDay `json:"vesting_occurs_day,omitempty"`
	LengthOfSchedule *int      `json:"length_of_schedule,omitempty"` // months

	// Cliff fields.
	IncludeCliff    bool      `json:"include_cliff"`
	CliffPercentage *float64  `json:"cliff_percentage,omitempty"` // 0-100
	CliffLength     *int      `json:"cliff_length,omitempty"`
	CliffLengthUnit CliffUnit `json:"cliff_length_unit,omitempty"`

	// Immediate vesting.
	IncludeImmediateVesting    bool     `json:"include_immediate_vesting"`
	ImmediateVestingPercentage *float64 `json:"immediate_vesting_percentage,omitempty"` // 0-100

	// Custom schedule fields.
	TypeOfVesting VestingType `json:"type_of_vesting,omitempty"`

	Tranches  []Tranche `json:"tranches,omitempty"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// Tranche is a single vesting period within a schedule.
type Tranche struct {
	ID                     string  `json:"id"`
	ScheduleID             string  `json:"schedule_id"`
	PeriodNumber           int     `json:"period_number"`
	Milestone              string  `json:"milestone,omitempty"`
	Length                 string  `json:"length"`                  // e.g. "1", "2"
	Frequency              string  `json:"frequency"`               // e.g. "every_month"
	Percentage             float64 `json:"percentage"`              // 0-100
	VestingOccurs          string  `json:"vesting_occurs"`          // e.g. "same_day"
	PerformanceConditionID string  `json:"performance_condition_id,omitempty"`
	CreatedAt              time.Time `json:"created_at"`
	UpdatedAt              time.Time `json:"updated_at"`
}

// PerformanceCondition defines a condition that must be met for vesting.
type PerformanceCondition struct {
	ID                  string                   `json:"id"`
	Name                string                   `json:"name"`
	ConditionType       PerformanceConditionType `json:"condition_type"`
	Details             string                   `json:"details,omitempty"`
	MaxPayout           *float64                 `json:"max_payout,omitempty"`
	MinPayout           *float64                 `json:"min_payout,omitempty"`
	PostTerminationTerm string                   `json:"post_termination_term,omitempty"`
	CreatedAt           time.Time                `json:"created_at"`
	UpdatedAt           time.Time                `json:"updated_at"`
}

// Event records an actual vesting occurrence.
type Event struct {
	ID         string    `json:"id"`
	TrancheID  string    `json:"tranche_id"`
	VestingDate time.Time `json:"vesting_date"`
	Percentage float64   `json:"percentage"` // percentage vested in this event
	CreatedAt  time.Time `json:"created_at"`
}
