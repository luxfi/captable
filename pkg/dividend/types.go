package dividend

import "time"

// Declaration is a board-declared dividend.
type Declaration struct {
	ID             string    `json:"id"`
	CompanyID      string    `json:"company_id"`
	ShareClassID   string    `json:"share_class_id"`
	Type           string    `json:"type"`             // cash, stock, property
	AmountPerShare float64   `json:"amount_per_share"` // for cash dividends
	StockRatio     float64   `json:"stock_ratio,omitempty"` // for stock dividends (e.g., 0.05 = 5%)
	DeclarationDate time.Time `json:"declaration_date"`
	RecordDate     time.Time `json:"record_date"`
	ExDividendDate time.Time `json:"ex_dividend_date"`
	PayableDate    time.Time `json:"payable_date"`
	TotalAmount    float64   `json:"total_amount"`     // computed total distribution
	Status         string    `json:"status"`           // declared, record_set, distributed, cancelled
	CreatedAt      time.Time `json:"created_at"`
}

// Distribution is a single dividend payment to a stakeholder.
type Distribution struct {
	ID              string    `json:"id"`
	DeclarationID   string    `json:"declaration_id"`
	StakeholderID   string    `json:"stakeholder_id"`
	Shares          int64     `json:"shares"`           // shares held at record date
	Amount          float64   `json:"amount"`           // cash amount payable
	StockShares     int64     `json:"stock_shares,omitempty"` // shares issued (stock dividend)
	TaxWithholding  float64   `json:"tax_withholding,omitempty"`
	NetAmount       float64   `json:"net_amount"`
	Status          string    `json:"status"` // pending, paid, failed
	PaidAt          *time.Time `json:"paid_at,omitempty"`
}

// DistributionSummary is the aggregate view of a dividend declaration.
type DistributionSummary struct {
	DeclarationID     string  `json:"declaration_id"`
	TotalDistributed  float64 `json:"total_distributed"`
	TotalWithholding  float64 `json:"total_withholding"`
	TotalNet          float64 `json:"total_net"`
	RecipientsCount   int     `json:"recipients_count"`
	PaidCount         int     `json:"paid_count"`
	PendingCount      int     `json:"pending_count"`
}
