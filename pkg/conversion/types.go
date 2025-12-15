package conversion

import "time"

// SAFE represents a Simple Agreement for Future Equity.
type SAFE struct {
	ID               string    `json:"id"`
	InvestorID       string    `json:"investor_id"`
	InvestmentAmount float64   `json:"investment_amount"`
	ValuationCap     float64   `json:"valuation_cap"`     // 0 means no cap
	Discount         float64   `json:"discount"`           // percentage, e.g. 20 = 20%
	MFN              bool      `json:"mfn"`
	ProRata          bool      `json:"pro_rata"`
	Type             string    `json:"type"` // pre_money, post_money, mfn
	IssueDate        time.Time `json:"issue_date"`
}

// ConvertibleNote represents a convertible debt instrument.
type ConvertibleNote struct {
	ID                string    `json:"id"`
	InvestorID        string    `json:"investor_id"`
	PrincipalAmount   float64   `json:"principal_amount"`
	InterestRate      float64   `json:"interest_rate"` // annual %, e.g. 8 = 8%
	MaturityDate      time.Time `json:"maturity_date"`
	ValuationCap      float64   `json:"valuation_cap"`      // 0 means no cap
	Discount          float64   `json:"discount"`            // percentage, e.g. 20 = 20%
	ConversionTrigger string    `json:"conversion_trigger"`  // qualified_financing, maturity, change_of_control
	IssueDate         time.Time `json:"issue_date"`
}

// ConversionTrigger describes the event that causes conversion.
type ConversionTrigger struct {
	Type                      string  `json:"type"` // qualified_financing, maturity, change_of_control
	RoundPreMoney             int64   `json:"round_pre_money"`
	RoundPricePerShare        float64 `json:"round_price_per_share"`
	PreMoneyShares            int64   `json:"pre_money_shares"`  // total shares before conversion (for cap calculation)
	PostMoneyShares           int64   `json:"post_money_shares"` // total shares after conversion (for post-money SAFE)
	QualifiedFinancingMinimum float64 `json:"qualified_financing_minimum"`
	ConversionDate            time.Time `json:"conversion_date"` // used for interest accrual on notes
}

// ConversionResult is the output of converting a single instrument.
type ConversionResult struct {
	InstrumentID      string  `json:"instrument_id"`
	InstrumentType    string  `json:"instrument_type"` // safe, note
	InvestorID        string  `json:"investor_id"`
	OriginalInvestment float64 `json:"original_investment"`
	ConvertedAmount   float64 `json:"converted_amount"` // principal + accrued interest for notes; same as investment for SAFEs
	SharesIssued      int64   `json:"shares_issued"`
	PricePerShare     float64 `json:"price_per_share"`
	EffectiveDiscount float64 `json:"effective_discount"` // actual discount obtained vs round price
	ShareClassName    string  `json:"share_class_name"`
	Method            string  `json:"method"` // cap, discount, mfn
}
