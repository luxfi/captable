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
