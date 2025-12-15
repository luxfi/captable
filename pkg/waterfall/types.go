package waterfall

// TransactionType describes the nature of the exit event.
type TransactionType string

const (
	Acquisition TransactionType = "acquisition"
	IPO         TransactionType = "ipo"
	Dissolution TransactionType = "dissolution"
)

// ExitScenario defines the exit event parameters.
type ExitScenario struct {
	TotalProceeds   float64         `json:"total_proceeds"`
	TransactionType TransactionType `json:"transaction_type"`
}

// ShareClassInput describes one class of equity entering the waterfall.
type ShareClassInput struct {
	Name               string  `json:"name"`
	SharesOutstanding  int64   `json:"shares_outstanding"`
	InvestmentAmount   float64 `json:"investment_amount"`    // total dollars invested
	LiquidationMultiple float64 `json:"liquidation_multiple"` // e.g. 1.0, 2.0
	Participating      bool    `json:"participating"`
	ParticipationCap   float64 `json:"participation_cap"` // 0 means uncapped; expressed as multiple of investment
	Seniority          int     `json:"seniority"`         // higher = more senior; 0 = common
	ConversionRatio    float64 `json:"conversion_ratio"`  // preferred shares to common shares; 0 or 1 = 1:1
}

// WaterfallResult is the complete output of a waterfall calculation.
type WaterfallResult struct {
	TotalDistributed float64      `json:"total_distributed"`
	Remainder        float64      `json:"remainder"`
	Tiers            []TierResult `json:"tiers"`
}

// TierResult holds the distributions for a single waterfall tier.
type TierResult struct {
	Name          string                       `json:"name"`
	Distributions map[string]ClassDistribution `json:"distributions"` // keyed by class name
}

// ClassDistribution is the breakdown of what a single share class receives in a tier.
type ClassDistribution struct {
	LiquidationPayout  float64 `json:"liquidation_payout"`
	ParticipationPayout float64 `json:"participation_payout"`
	ConversionPayout   float64 `json:"conversion_payout"`
	TotalPayout        float64 `json:"total_payout"`
	PerSharePayout     float64 `json:"per_share_payout"`
	ReturnMultiple     float64 `json:"return_multiple"`
}
