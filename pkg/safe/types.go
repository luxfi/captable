package safe

import "time"

// SafeType distinguishes pre-money vs post-money SAFEs.
type SafeType string

const (
	PreMoney  SafeType = "pre_money"
	PostMoney SafeType = "post_money"
)

// SafeStatus tracks the lifecycle of a SAFE.
type SafeStatus string

const (
	StatusDraft     SafeStatus = "draft"
	StatusActive    SafeStatus = "active"
	StatusPending   SafeStatus = "pending"
	StatusConverted SafeStatus = "converted"
	StatusExpired   SafeStatus = "expired"
	StatusCancelled SafeStatus = "cancelled"
)

// SafeTemplate identifies the standard SAFE template used.
type SafeTemplate string

const (
	TemplatePostMoneyCap               SafeTemplate = "post_money_cap"
	TemplatePostMoneyDiscount          SafeTemplate = "post_money_discount"
	TemplatePostMoneyMFN               SafeTemplate = "post_money_mfn"
	TemplatePostMoneyCapWithProRata    SafeTemplate = "post_money_cap_with_pro_rata"
	TemplatePostMoneyDiscountWithProRata SafeTemplate = "post_money_discount_with_pro_rata"
	TemplatePostMoneyMFNWithProRata    SafeTemplate = "post_money_mfn_with_pro_rata"
	TemplateCustom                     SafeTemplate = "custom"
)

// Safe represents a Simple Agreement for Future Equity.
type Safe struct {
	ID              string       `json:"id"`
	PublicID        string       `json:"public_id"` // e.g. SAFE-01
	CompanyID       string       `json:"company_id"`
	StakeholderID   string       `json:"stakeholder_id"`
	Type            SafeType     `json:"type"`
	Status          SafeStatus   `json:"status"`
	Template        SafeTemplate `json:"template,omitempty"`
	Capital         float64      `json:"capital"`                    // amount invested
	ValuationCap    *float64     `json:"valuation_cap,omitempty"`
	DiscountRate    *float64     `json:"discount_rate,omitempty"`    // e.g. 0.20 for 20%
	MFN             bool         `json:"mfn"`                       // most favored nation
	ProRata         bool         `json:"pro_rata"`                  // pro rata rights
	AdditionalTerms string       `json:"additional_terms,omitempty"`
	IssueDate         time.Time  `json:"issue_date"`
	BoardApprovalDate time.Time  `json:"board_approval_date"`
	CreatedAt       time.Time    `json:"created_at"`
	UpdatedAt       time.Time    `json:"updated_at"`
}

// ConversionResult holds the output of converting a SAFE to equity.
type ConversionResult struct {
	SafeID           string  `json:"safe_id"`
	ShareClassID     string  `json:"share_class_id"`
	Shares           int64   `json:"shares"`
	PricePerShare    float64 `json:"price_per_share"`
	OwnershipPercent float64 `json:"ownership_percent"`
}

// Converter calculates SAFE-to-equity conversion.
type Converter interface {
	// Convert computes the shares and price for a given SAFE at a priced round.
	Convert(s *Safe, preMoneyValuation float64, pricePerShare float64) (*ConversionResult, error)
}
