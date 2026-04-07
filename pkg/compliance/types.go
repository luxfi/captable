package compliance

import "time"

// FormD represents a Regulation D Form D filing with the SEC.
type FormD struct {
	ID                string     `json:"id"`
	CompanyID         string     `json:"company_id"`
	SecurityID        string     `json:"security_id"`
	Exemption         string     `json:"exemption"` // 506b, 506c, 504
	TotalOffering     float64    `json:"total_offering"`
	TotalSold         float64    `json:"total_sold"`
	TotalRemaining    float64    `json:"total_remaining"`
	NumInvestors      int        `json:"num_investors"`
	NumAccredited     int        `json:"num_accredited"`
	NumNonAccredited  int        `json:"num_non_accredited"`
	FirstSaleDate     *time.Time `json:"first_sale_date,omitempty"`
	FilingDate        *time.Time `json:"filing_date,omitempty"`
	AmendmentDate     *time.Time `json:"amendment_date,omitempty"`
	Status            string     `json:"status"` // draft, filed, amended
	SECFileNumber     string     `json:"sec_file_number,omitempty"`
	CreatedAt         time.Time  `json:"created_at"`
	UpdatedAt         time.Time  `json:"updated_at"`
}

// BlueSkyFiling tracks state-level securities filing (blue sky law compliance).
type BlueSkyFiling struct {
	ID          string     `json:"id"`
	CompanyID   string     `json:"company_id"`
	SecurityID  string     `json:"security_id"`
	State       string     `json:"state"`      // US state code (CA, NY, etc.)
	FilingType  string     `json:"filing_type"` // notice, registration, exemption
	Status      string     `json:"status"`      // pending, filed, approved, rejected
	FilingDate  *time.Time `json:"filing_date,omitempty"`
	ApprovedDate *time.Time `json:"approved_date,omitempty"`
	ExpiresDate *time.Time `json:"expires_date,omitempty"`
	Fee         float64    `json:"fee,omitempty"`
	Notes       string     `json:"notes,omitempty"`
}

// AccreditationCheck is the result of verifying an investor's accreditation status.
type AccreditationCheck struct {
	StakeholderID  string    `json:"stakeholder_id"`
	IsAccredited   bool      `json:"is_accredited"`
	Method         string    `json:"method"` // self_cert, income_verification, net_worth, professional, entity
	VerifiedAt     time.Time `json:"verified_at"`
	ExpiresAt      time.Time `json:"expires_at"`
	DocumentID     string    `json:"document_id,omitempty"` // reference to verification document
}

// ComplianceCheck evaluates whether an offering is compliant with its exemption.
type ComplianceCheckResult struct {
	Compliant    bool     `json:"compliant"`
	Violations   []string `json:"violations,omitempty"`
	Warnings     []string `json:"warnings,omitempty"`
}

// RegDCheck distinguishes between 506(b) and 506(c) compliance requirements.
type RegDCheck struct {
	Exemption              string `json:"exemption"` // "506b" or "506c"
	TotalInvestors         int    `json:"total_investors"`
	AccreditedInvestors    int    `json:"accredited_investors"`
	NonAccreditedInvestors int    `json:"non_accredited_investors"`
	// 506(b) specific.
	HasGeneralSolicitation bool `json:"has_general_solicitation"`
	HasPreexistingRelationship bool `json:"has_preexisting_relationship"`
	// 506(c) specific.
	AllVerifiedAccredited bool `json:"all_verified_accredited"` // third-party verification required
}

// RegSCheck evaluates Regulation S (offshore) exemption compliance.
type RegSCheck struct {
	IsOffshoreSale       bool   `json:"is_offshore_sale"`
	SellerIsUS           bool   `json:"seller_is_us"`
	BuyerJurisdiction    string `json:"buyer_jurisdiction"`
	HasDirectedSelling   bool   `json:"has_directed_selling"`     // no directed selling efforts in US
	DistributionComplete bool   `json:"distribution_complete"`    // distribution compliance period ended
	CompliancePeriodDays int    `json:"compliance_period_days"`   // 40 or 365 depending on category
}

// Rule144Check evaluates Rule 144 resale exemption requirements.
type Rule144Check struct {
	IsAffiliate        bool       `json:"is_affiliate"`
	HoldingPeriodMet   bool       `json:"holding_period_met"`   // 6 months (reporting) or 12 months (non-reporting)
	HoldingStartDate   *time.Time `json:"holding_start_date,omitempty"`
	// Affiliate-only requirements.
	VolumeLimit        int64   `json:"volume_limit,omitempty"`        // max shares sellable
	CurrentVolume      int64   `json:"current_volume,omitempty"`      // shares sold in trailing 3 months
	MannerOfSale       string  `json:"manner_of_sale,omitempty"`      // broker, market_maker
	Form144Filed       bool    `json:"form_144_filed"`
	Form144FilingDate  *time.Time `json:"form_144_filing_date,omitempty"`
	IssuerIsReporting  bool    `json:"issuer_is_reporting"`           // current SEC reporting
	PublicInfoAvailable bool   `json:"public_info_available"`         // adequate public info
}
