package tax

import "time"

// Form1099DIV represents the data needed for a 1099-DIV tax form.
type Form1099DIV struct {
	ID                  string  `json:"id"`
	TaxYear             int     `json:"tax_year"`
	PayerName           string  `json:"payer_name"`
	PayerEIN            string  `json:"payer_ein"`
	RecipientName       string  `json:"recipient_name"`
	RecipientTIN        string  `json:"recipient_tin"` // SSN or EIN
	RecipientAddress    string  `json:"recipient_address,omitempty"`
	OrdinaryDividends   float64 `json:"ordinary_dividends"`       // Box 1a
	QualifiedDividends  float64 `json:"qualified_dividends"`      // Box 1b
	CapitalGainDist     float64 `json:"capital_gain_dist"`        // Box 2a
	Section199ADividends float64 `json:"section_199a_dividends"`  // Box 5
	FederalTaxWithheld  float64 `json:"federal_tax_withheld"`     // Box 4
	StateTaxWithheld    float64 `json:"state_tax_withheld"`       // Box 16
	State               string  `json:"state,omitempty"`          // Box 15
	Status              string  `json:"status"`                   // draft, generated, filed, corrected
	GeneratedAt         *time.Time `json:"generated_at,omitempty"`
}

// Form1099B represents the data needed for a 1099-B (proceeds from broker/barter exchange).
type Form1099B struct {
	ID                 string  `json:"id"`
	TaxYear            int     `json:"tax_year"`
	PayerName          string  `json:"payer_name"`
	PayerEIN           string  `json:"payer_ein"`
	RecipientName      string  `json:"recipient_name"`
	RecipientTIN       string  `json:"recipient_tin"`
	Description        string  `json:"description"`            // security description
	DateAcquired       string  `json:"date_acquired"`
	DateSold           string  `json:"date_sold"`
	Proceeds           float64 `json:"proceeds"`               // Box 1d
	CostBasis          float64 `json:"cost_basis"`             // Box 1e
	GainLoss           float64 `json:"gain_loss"`              // computed
	ShortTermLongTerm  string  `json:"short_term_long_term"`   // short, long
	FederalTaxWithheld float64 `json:"federal_tax_withheld"`
	Status             string  `json:"status"`
	GeneratedAt        *time.Time `json:"generated_at,omitempty"`
}

// ScheduleK1 represents a Schedule K-1 (partner's share of income/deductions).
type ScheduleK1 struct {
	ID                  string  `json:"id"`
	TaxYear             int     `json:"tax_year"`
	PartnershipName     string  `json:"partnership_name"`
	PartnershipEIN      string  `json:"partnership_ein"`
	PartnerName         string  `json:"partner_name"`
	PartnerTIN          string  `json:"partner_tin"`
	OwnershipPercent    float64 `json:"ownership_percent"`
	OrdinaryIncome      float64 `json:"ordinary_income"`         // Box 1
	NetRentalIncome     float64 `json:"net_rental_income"`       // Box 2
	OtherNetIncome      float64 `json:"other_net_income"`        // Box 3
	GuaranteedPayments  float64 `json:"guaranteed_payments"`     // Box 4
	CapitalGainLoss     float64 `json:"capital_gain_loss"`       // Box 9a
	Distributions       float64 `json:"distributions"`           // Box 19
	Status              string  `json:"status"`
	GeneratedAt         *time.Time `json:"generated_at,omitempty"`
}

// GenerationRequest specifies what tax forms to generate.
type GenerationRequest struct {
	CompanyID string `json:"company_id"`
	TaxYear   int    `json:"tax_year"`
	FormType  string `json:"form_type"` // 1099-DIV, 1099-B, K-1
}

// GenerationResult summarizes the output of a tax form generation run.
type GenerationResult struct {
	FormType     string `json:"form_type"`
	TaxYear      int    `json:"tax_year"`
	FormsCreated int    `json:"forms_created"`
	Errors       int    `json:"errors"`
}
