package dividend

import "time"

// Disbursement types — covers both private equity and private credit
const (
	// Private Equity
	TypeDividend        = "dividend"
	TypeCapitalGains    = "capital_gains"
	TypeReturnOfCapital = "return_of_capital"
	TypePreferredReturn = "preferred_return"

	// Private Credit
	TypeInterestPayment    = "interest_payment"
	TypePrincipalRepayment = "principal_repayment"
	TypeFeeIncome          = "fee_income"
	TypeExitProceeds       = "exit_proceeds"

	// Additional
	TypeStockDividend    = "stock_dividend"
	TypeTokenDividend    = "token_dividend"     // any ERC-20 token
	TypePropertyDividend = "property_dividend"
	TypeSpecialDividend  = "special_dividend"
	TypeLiquidation      = "liquidation"
)

// Distribution structures
const (
	StructFixedAmount   = "fixed_amount"
	StructProRata       = "pro_rata"
	StructDailyFactor   = "daily_factor"
	StructTiered        = "tiered"
	StructCustomFormula = "custom_formula"
)

// PaymentToken defines what currency/token the dividend is paid in
type PaymentToken struct {
	Symbol          string `json:"symbol"`                     // LUSD, ETH, BTC, LQDTY, any ERC-20
	ContractAddress string `json:"contract_address,omitempty"` // ERC-20 contract address
	ChainID         int64  `json:"chain_id,omitempty"`
	Decimals        int    `json:"decimals"`
}

// Declaration is a board-declared dividend/disbursement.
type Declaration struct {
	ID              string       `json:"id"`
	CompanyID       string       `json:"company_id"`
	ShareClassID    string       `json:"share_class_id"`
	TenantID        string       `json:"tenant_id"`
	Type            string       `json:"type"`
	AssetClass      string       `json:"asset_class"`
	Structure       string       `json:"structure"`
	AmountPerShare  float64      `json:"amount_per_share"`
	TotalPool       float64      `json:"total_pool"`
	StockRatio      float64      `json:"stock_ratio,omitempty"`
	PaymentToken    PaymentToken `json:"payment_token"`
	DailyRate       float64      `json:"daily_rate,omitempty"`
	HurdleRate      float64      `json:"hurdle_rate,omitempty"`
	CarriedInterest float64      `json:"carried_interest,omitempty"`
	DeclarationDate time.Time    `json:"declaration_date"`
	RecordDate      time.Time    `json:"record_date"`
	ExDividendDate  time.Time    `json:"ex_dividend_date"`
	PayableDate     time.Time    `json:"payable_date"`
	TotalAmount     float64      `json:"total_amount"`
	Status          string       `json:"status"`
	TxHash          string       `json:"tx_hash,omitempty"`
	Description     string       `json:"description,omitempty"`
	ApprovedBy      string       `json:"approved_by,omitempty"`
	CreatedAt       time.Time    `json:"created_at"`
	UpdatedAt       time.Time    `json:"updated_at"`
}

// Distribution is a single payment to a stakeholder.
type Distribution struct {
	ID             string       `json:"id"`
	DeclarationID  string       `json:"declaration_id"`
	StakeholderID  string       `json:"stakeholder_id"`
	TenantID       string       `json:"tenant_id"`
	Shares         int64        `json:"shares"`
	OwnershipPct   float64      `json:"ownership_pct"`
	GrossAmount    float64      `json:"gross_amount"`
	TaxWithholding float64      `json:"tax_withholding"`
	NetAmount      float64      `json:"net_amount"`
	StockShares    int64        `json:"stock_shares,omitempty"`
	PaymentToken   PaymentToken `json:"payment_token"`
	AccruedAmount  float64      `json:"accrued_amount,omitempty"`
	DaysAccrued    int          `json:"days_accrued,omitempty"`
	Status         string       `json:"status"`
	PaymentMethod  string       `json:"payment_method"`
	WalletAddress  string       `json:"wallet_address,omitempty"`
	TxHash         string       `json:"tx_hash,omitempty"`
	PaidAt         *time.Time   `json:"paid_at,omitempty"`
	TaxFormType    string       `json:"tax_form_type,omitempty"`
	TaxYear        int          `json:"tax_year,omitempty"`
	CreatedAt      time.Time    `json:"created_at"`
}

// DistributionSummary is the aggregate view.
type DistributionSummary struct {
	DeclarationID      string  `json:"declaration_id"`
	Type               string  `json:"type"`
	PaymentTokenSymbol string  `json:"payment_token_symbol"`
	TotalGross         float64 `json:"total_gross"`
	TotalWithholding   float64 `json:"total_withholding"`
	TotalNet           float64 `json:"total_net"`
	RecipientsCount    int     `json:"recipients_count"`
	PaidCount          int     `json:"paid_count"`
	PendingCount       int     `json:"pending_count"`
}

// WaterfallTier for tiered/hurdle distributions (PE fund waterfalls)
type WaterfallTier struct {
	Name       string  `json:"name"`
	HurdleRate float64 `json:"hurdle_rate"`
	LPShare    float64 `json:"lp_share"`
	GPShare    float64 `json:"gp_share"`
}
