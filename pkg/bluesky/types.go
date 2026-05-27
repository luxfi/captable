// Package bluesky — local type definitions for state blue-sky notice
// filings. Models notice-of-sale filings under §18 of the Securities
// Act (NSMIA preemption — covered securities under §18(b)(4) are
// preempted from state registration but most states still require a
// notice filing for Reg D Rule 506 offerings).
//
// The NASAA EFD (Electronic Filing Depository) at https://www.efdnasaa.org
// is the primary submission path for the 49 states + DC that accept
// electronic notice filings. Florida and Texas are not on EFD and
// require state-portal submission.
//
// Source-of-design: Public-Spec
// Source-ref: http://nasaaefd.org
package bluesky

import "time"

// FilingType discriminates the kind of state notice filing.
type FilingType string

const (
	// FilingNoticeOfSale is the initial Form D notice filing for a
	// Rule 506 offering. Most states require this within 15 days of
	// the first sale into the state.
	FilingNoticeOfSale FilingType = "notice_of_sale"

	// FilingRenewal is a renewal of a previously filed notice. Some
	// states (e.g., NY) require annual renewals as long as the
	// offering remains open.
	FilingRenewal FilingType = "renewal"

	// FilingAmendment is an amendment to a previously filed notice
	// (typically used when a material change occurs in the underlying
	// SEC Form D).
	FilingAmendment FilingType = "amendment"
)

// State is the canonical two-letter US state code used by all
// adapters. Includes DC.
type State string

// Address is the postal address shape used by every state filing
// element.
type Address struct {
	Street1    string `json:"street1"`
	Street2    string `json:"street2,omitempty"`
	City       string `json:"city"`
	State      string `json:"state"`
	PostalCode string `json:"postal_code"`
	Country    string `json:"country"`
}

// Issuer is the issuer block on a state notice filing. Mirrors the
// Form D primaryIssuer shape but with the simpler set of fields most
// state forms require.
type Issuer struct {
	CIK                         string  `json:"cik,omitempty"`
	EntityName                  string  `json:"entity_name"`
	EntityType                  string  `json:"entity_type"`
	JurisdictionOfIncorporation string  `json:"jurisdiction_of_incorporation"`
	YearOfIncorporation         string  `json:"year_of_incorporation,omitempty"`
	PrimaryAddress              Address `json:"primary_address"`
	Phone                       string  `json:"phone"`
	Email                       string  `json:"email,omitempty"`
}

// OfferingDetails captures the offering-specific fields most states
// require on a notice filing.
type OfferingDetails struct {
	// FederalExemption is the claimed federal exemption (e.g.,
	// "506b", "506c", "504", "Reg A+ Tier 2"). For the most common
	// case, this is one of the EDGAR FederalExemption values.
	FederalExemption string `json:"federal_exemption"`

	// TotalOfferingAmount is the total US offering size in USD; some
	// states base their fee on this number.
	TotalOfferingAmount float64 `json:"total_offering_amount"`

	// AmountSoldInState is the dollar amount sold to investors in
	// this state at the time of filing. Some state fees are based
	// on this rather than total offering.
	AmountSoldInState float64 `json:"amount_sold_in_state"`

	// DateOfFirstSaleInState is the date of the first sale in this
	// state (the 15-day clock typically starts here).
	DateOfFirstSaleInState time.Time `json:"date_of_first_sale_in_state"`

	// TypesOfSecurities is the list of security types in the offering
	// (same value set as the EDGAR Form D types).
	TypesOfSecurities []string `json:"types_of_securities"`

	// SECFileNumber is the SEC-assigned file number from the EDGAR
	// Form D filing, if already filed. Several states cross-reference
	// this on the notice form.
	SECFileNumber string `json:"sec_file_number,omitempty"`

	// SECAccessionNumber is the EDGAR-assigned accession number for
	// the underlying Form D.
	SECAccessionNumber string `json:"sec_accession_number,omitempty"`
}

// NoticeFiling is the in-memory representation of a state notice-of-
// sale filing. Submitted via Registrar.FileNoticeOfSale.
type NoticeFiling struct {
	// FilingType is one of FilingNoticeOfSale, FilingRenewal,
	// FilingAmendment.
	FilingType FilingType `json:"filing_type"`

	// State is the two-letter US state code (or "DC").
	State State `json:"state"`

	// Issuer carries the issuer-level information.
	Issuer Issuer `json:"issuer"`

	// Offering carries the offering-level information.
	Offering OfferingDetails `json:"offering"`

	// Signature is the issuer-officer signature block on the notice
	// form.
	Signature Signature `json:"signature"`

	// FormDXML is the optional attached Form D XML body. Many states
	// require the underlying Form D as an attachment to the state
	// notice filing.
	FormDXML []byte `json:"form_d_xml,omitempty"`

	// AdditionalDocuments are any state-specific attachments
	// (e.g., consent-to-service-of-process forms).
	AdditionalDocuments []AttachedDocument `json:"additional_documents,omitempty"`
}

// RenewalFiling is the in-memory representation of a state renewal
// filing. Several states (NY, MA, CA, NJ, PA, OH, IL) require annual
// renewals for offerings that remain open beyond the initial 12-month
// period.
type RenewalFiling struct {
	State                State           `json:"state"`
	Issuer               Issuer          `json:"issuer"`
	OriginalFilingID     string          `json:"original_filing_id"`
	OriginalFilingDate   time.Time       `json:"original_filing_date"`
	UpdatedOffering      OfferingDetails `json:"updated_offering"`
	Signature            Signature       `json:"signature"`
}

// Signature is the signature block at the bottom of the state notice
// form.
type Signature struct {
	IssuerName     string `json:"issuer_name"`
	SignatureName  string `json:"signature_name"`
	NameOfSigner   string `json:"name_of_signer"`
	SignatureTitle string `json:"signature_title"`
	SignatureDate  string `json:"signature_date"` // YYYY-MM-DD
}

// AttachedDocument is one attachment to a state notice filing.
type AttachedDocument struct {
	Filename    string `json:"filename"`
	ContentType string `json:"content_type"`
	Body        []byte `json:"body"`
}

// Acknowledgment is the parsed reply the state regulator returns on
// receipt of a filing. State-specific fields are surfaced in the
// stable shape below.
type Acknowledgment struct {
	// FilingID is the state-assigned identifier for the filing.
	// Format varies by state.
	FilingID string `json:"filing_id"`

	// Status is one of "RECEIVED", "ACCEPTED", "FILED", "APPROVED",
	// "SUSPENDED", "REJECTED".
	Status string `json:"status"`

	// State is the state code this acknowledgment is for.
	State State `json:"state"`

	// Fee is the actual fee charged for this filing in USD.
	Fee float64 `json:"fee"`

	// ReceivedAt is the timestamp the regulator reports for receiving
	// the filing.
	ReceivedAt time.Time `json:"received_at"`

	// ExpiresAt is the date the filing expires (typically one year
	// from FilingDate for offerings requiring annual renewal).
	ExpiresAt time.Time `json:"expires_at,omitempty"`

	// PaymentID is the state-assigned ACH payment identifier from EFD.
	PaymentID string `json:"payment_id,omitempty"`

	// Messages are any informational, warning, or error messages
	// returned with the acknowledgment.
	Messages []string `json:"messages,omitempty"`
}

// FilingStatus is the parsed reply from GetFilingStatus.
type FilingStatus struct {
	FilingID    string    `json:"filing_id"`
	State       State     `json:"state"`
	Status      string    `json:"status"`
	FiledAt     time.Time `json:"filed_at,omitempty"`
	AcceptedAt  time.Time `json:"accepted_at,omitempty"`
	ExpiresAt   time.Time `json:"expires_at,omitempty"`
	Fee         float64   `json:"fee"`
	Messages    []string  `json:"messages,omitempty"`
}

// FeeAmount is the per-state fee with the breakdown the calculator
// returns. Returned in USD cents-as-float (e.g., 200.00 = $200.00) to
// match the rest of the captable surface.
type FeeAmount struct {
	State         State   `json:"state"`
	StateFee      float64 `json:"state_fee"`       // statutory state fee
	SystemFee     float64 `json:"system_fee"`      // NASAA EFD system-use fee (~$160 per filing)
	TotalDue      float64 `json:"total_due"`       // sum of state + system
	Currency      string  `json:"currency"`        // always "USD"
	Method        string  `json:"method"`          // "ACH" via EFD; "Check" / "Wire" for paper states
	Notes         string  `json:"notes,omitempty"` // e.g., "lifetime cap $750"
}
