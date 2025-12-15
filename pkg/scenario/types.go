package scenario

// FundingRound describes the terms of a priced equity round.
type FundingRound struct {
	Name              string  `json:"name"` // Seed, Series A, etc.
	PreMoneyValuation int64   `json:"pre_money_valuation"`
	InvestmentAmount  int64   `json:"investment_amount"`
	ShareClassType    string  `json:"share_class_type"` // preferred, common
	LiquidationMultiple float64 `json:"liquidation_multiple"`
	Participating     bool    `json:"participating"`
	ParticipationCap  float64 `json:"participation_cap"`
	AntiDilution      string  `json:"anti_dilution"` // none, broad_based, narrow_based, full_ratchet
}

// ProFormaResult is the output of modeling a funding round.
type ProFormaResult struct {
	PreMoneyValuation  int64          `json:"pre_money_valuation"`
	PostMoneyValuation int64          `json:"post_money_valuation"`
	PricePerShare      float64        `json:"price_per_share"`
	NewSharesIssued    int64          `json:"new_shares_issued"`
	OptionPoolIncrease int64          `json:"option_pool_increase"`
	Ownership          []OwnershipRow `json:"ownership"`
	FullyDilutedShares int64          `json:"fully_diluted_shares"`
	OptionPoolShares   int64          `json:"option_pool_shares"`
}

// OwnershipRow represents a single stakeholder's position before and after a round.
type OwnershipRow struct {
	Name          string  `json:"name"`
	SharesBefore  int64   `json:"shares_before"`
	PercentBefore float64 `json:"percent_before"`
	SharesAfter   int64   `json:"shares_after"`
	PercentAfter  float64 `json:"percent_after"`
	Dilution      float64 `json:"dilution"` // percentage points lost
}

// ExitScenario is the output of modeling a liquidity event.
type ExitScenario struct {
	ExitValuation int64              `json:"exit_valuation"`
	Ownership     []ExitOwnershipRow `json:"ownership"`
}

// ExitOwnershipRow represents a stakeholder's proceeds from an exit.
type ExitOwnershipRow struct {
	Name           string  `json:"name"`
	Proceeds       float64 `json:"proceeds"`
	ReturnMultiple float64 `json:"return_multiple"`
	PercentOfExit  float64 `json:"percent_of_exit"`
}
