// Package edgar — local type definitions for the SEC EDGAR Form D
// filing adapter. Modeled directly on the public Form D XML schema
// (Form D XSD published by the SEC, primaryDocument shape) and the
// EDGAR Filer Manual Volume II submission envelope (SGML header
// wrapping the XML primary document plus EX-99 exhibits).
//
// Source-of-design: Public-Spec
// Source-ref: https://www.sec.gov/info/edgar/specifications/formdxml.pdf
// Source-ref: https://www.sec.gov/edgar/filer-information/current-edgar-technical-specifications
package edgar

import "time"

// SubmissionType is the EDGAR <TYPE> tag for a Form D filing. The SEC
// distinguishes original filings, amendments, and notice-of-sale
// amendments. The string values are exactly what EDGAR expects on the
// wire — do not localize.
type SubmissionType string

const (
	// SubmissionFormD is the original Form D notice of exempt offering
	// of securities.
	SubmissionFormD SubmissionType = "D"

	// SubmissionFormDA is an amendment to a previously filed Form D.
	// Amendments are required when material information changes or
	// annually for offerings that remain open more than 12 months.
	SubmissionFormDA SubmissionType = "D/A"
)

// FederalExemption identifies a claimed federal securities-law
// exemption on a Form D filing. Values are the canonical strings
// EDGAR's primaryDocument expects for the federalExemptionsExclusions
// repeating element.
type FederalExemption string

const (
	// ExemptionRule504 is Rule 504 of Regulation D — max $10M aggregate
	// in 12 months, limited general solicitation per Rule 504(b)(1).
	ExemptionRule504 FederalExemption = "06b"

	// ExemptionRule506b is Rule 506(b) — unlimited offering size, max
	// 35 non-accredited sophisticated investors, no general
	// solicitation.
	ExemptionRule506b FederalExemption = "06c"

	// ExemptionRule506c is Rule 506(c) — unlimited offering size,
	// general solicitation permitted, all purchasers must be
	// accredited and accreditation must be verified.
	ExemptionRule506c FederalExemption = "06c-506c"

	// ExemptionSection4a5 is Section 4(a)(5) — accredited-investor-only
	// offerings up to $5M without general solicitation.
	ExemptionSection4a5 FederalExemption = "3C.1"

	// ExemptionInvCo3c1 is Investment Company Act § 3(c)(1) — fund
	// exemption (under 100 beneficial owners).
	ExemptionInvCo3c1 FederalExemption = "3C.1"

	// ExemptionInvCo3c7 is Investment Company Act § 3(c)(7) — fund
	// exemption (all qualified purchasers).
	ExemptionInvCo3c7 FederalExemption = "3C.7"
)

// IndustryGroup is the EDGAR <industryGroupType> for the issuer's
// industry classification on Form D. Values are the canonical strings
// the SEC publishes in the Form D primaryDocument schema.
type IndustryGroup string

const (
	IndustryAgriculture           IndustryGroup = "Agriculture"
	IndustryBanking               IndustryGroup = "Commercial Banking"
	IndustryInsurance             IndustryGroup = "Insurance"
	IndustryInvesting             IndustryGroup = "Investing"
	IndustryInvestmentBanking     IndustryGroup = "Investment Banking"
	IndustryPooledInvestmentFund  IndustryGroup = "Pooled Investment Fund"
	IndustryHedgeFund             IndustryGroup = "Hedge Fund"
	IndustryPrivateEquityFund     IndustryGroup = "Private Equity Fund"
	IndustryVentureCapitalFund    IndustryGroup = "Venture Capital Fund"
	IndustryOtherInvestmentFund   IndustryGroup = "Other Investment Fund"
	IndustryRealEstate            IndustryGroup = "Real Estate"
	IndustryTechnology            IndustryGroup = "Technology"
	IndustryTravel                IndustryGroup = "Travel"
	IndustryOther                 IndustryGroup = "Other"
)

// EntityType is the EDGAR <entityType> for the issuer on Form D.
type EntityType string

const (
	EntityCorporation          EntityType = "Corporation"
	EntityLimitedPartnership   EntityType = "Limited Partnership"
	EntityLLC                  EntityType = "Limited Liability Company"
	EntityGeneralPartnership   EntityType = "General Partnership"
	EntityBusinessTrust        EntityType = "Business Trust"
	EntityOther                EntityType = "Other"
)

// Address is the postal address shape used by every Form D address
// element (issuer primary office, related-person address, etc).
type Address struct {
	Street1    string `xml:"street1" json:"street1"`
	Street2    string `xml:"street2,omitempty" json:"street2,omitempty"`
	City       string `xml:"city" json:"city"`
	StateOrCountry string `xml:"stateOrCountry" json:"state_or_country"`
	ZipCode    string `xml:"zipCode" json:"zip_code"`
}

// Issuer is one issuer block on a Form D. Form D supports multiple
// issuers (primary + additional) for offerings by related entities; the
// first issuer is the primary issuer and carries the CIK that signs
// the filing.
type Issuer struct {
	// CIK is the SEC Central Index Key for this issuer. Ten-digit,
	// zero-padded. Required for the primary issuer.
	CIK string `xml:"cik" json:"cik"`

	// EntityName is the legal name of the issuer.
	EntityName string `xml:"entityName" json:"entity_name"`

	// EntityType discriminates the legal form of the issuer.
	EntityType EntityType `xml:"entityType" json:"entity_type"`

	// YearOfIncorporation is a four-digit year. Use "OverFiveYears" /
	// "WithinLastFiveYears" / "YetToBeFormed" string sentinels only
	// when the exact year is not known; otherwise the literal year.
	YearOfIncorporation string `xml:"yearOfIncorporation" json:"year_of_incorporation"`

	// JurisdictionOfIncorporation is the two-character US state code or
	// three-character ISO 3166 country code where the issuer is
	// organized.
	JurisdictionOfIncorporation string `xml:"jurisdictionOfIncorporation" json:"jurisdiction_of_incorporation"`

	// PrimaryAddress is the issuer's principal executive office.
	PrimaryAddress Address `xml:"primaryAddress" json:"primary_address"`

	// Phone is the issuer's main phone number, digits only with a
	// leading "+" country code permitted.
	Phone string `xml:"phoneNumber" json:"phone_number"`
}

// RelatedPerson is one entry in the related-persons block — executive
// officers, directors, promoters. Form D requires the full list at the
// time of filing.
type RelatedPerson struct {
	// FirstName, MiddleName, LastName form the natural-person name.
	FirstName  string `xml:"firstName" json:"first_name"`
	MiddleName string `xml:"middleName,omitempty" json:"middle_name,omitempty"`
	LastName   string `xml:"lastName" json:"last_name"`

	// Address is the related person's mailing address, typically c/o
	// the issuer's principal office.
	Address Address `xml:"address" json:"address"`

	// Relationships is the list of relationships this person bears to
	// the issuer. Allowed values per the Form D schema:
	// "Executive Officer", "Director", "Promoter".
	Relationships []string `xml:"relationships>relationship" json:"relationships"`

	// Clarification is an optional plain-language qualifier (e.g.,
	// "CFO", "Independent Director").
	Clarification string `xml:"clarificationOfResponse,omitempty" json:"clarification,omitempty"`
}

// OfferingSalesAmount is the dollar amounts block on Form D — total
// offering size, total amount sold to date, total remaining to be
// sold, and the (optional) clarification of indefinite offering.
type OfferingSalesAmount struct {
	// TotalOfferingAmount is the total offering size in USD. Set
	// IsIndefinite=true and leave this field 0 if the amount cannot
	// be determined.
	TotalOfferingAmount float64 `xml:"totalOfferingAmount" json:"total_offering_amount"`

	// TotalAmountSold is the aggregate amount of securities already
	// sold as of the filing date.
	TotalAmountSold float64 `xml:"totalAmountSold" json:"total_amount_sold"`

	// TotalRemaining is the difference between TotalOfferingAmount and
	// TotalAmountSold. The adapter computes this; callers may leave it
	// zero.
	TotalRemaining float64 `xml:"totalRemaining" json:"total_remaining"`

	// IsIndefinite signals an offering of indefinite size. When true,
	// TotalOfferingAmount is reported as the sentinel "Indefinite" in
	// the wire payload rather than a numeric value.
	IsIndefinite bool `xml:"-" json:"is_indefinite,omitempty"`
}

// InvestorCount is the investor-count block on Form D.
type InvestorCount struct {
	// TotalAlreadyInvested is the count of investors who have already
	// purchased in the offering.
	TotalAlreadyInvested int `xml:"totalNumberAlreadyInvested" json:"total_already_invested"`

	// NonAccreditedInvested is the subset of TotalAlreadyInvested who
	// are non-accredited (relevant only under Rule 506(b) — must be
	// zero for Rule 506(c)).
	NonAccreditedInvested int `xml:"nonAccreditedInvestorsAlreadyInvested" json:"non_accredited_invested"`
}

// SalesCommissions is the optional sales-compensation block on Form D.
type SalesCommissions struct {
	// SalesCommissions is the total sales-commission amount in USD.
	SalesCommissions float64 `xml:"salesCommissions" json:"sales_commissions"`

	// SalesCommissionsEstimate marks the amount as an estimate when
	// the actual value is not yet final.
	SalesCommissionsEstimate bool `xml:"salesCommissionsEstimate" json:"sales_commissions_estimate"`

	// FindersFees is the total finder's fee amount in USD.
	FindersFees float64 `xml:"findersFees" json:"finders_fees"`

	// FindersFeesEstimate marks the finder's fee as an estimate.
	FindersFeesEstimate bool `xml:"findersFeesEstimate" json:"finders_fees_estimate"`
}

// UseOfProceeds is the optional use-of-proceeds block on Form D.
type UseOfProceeds struct {
	// GrossProceedsToIssuer is the gross proceeds the issuer expects
	// to retain in USD.
	GrossProceedsToIssuer float64 `xml:"grossProceedsUsed" json:"gross_proceeds_to_issuer"`

	// GrossProceedsEstimate marks the gross-proceeds value as an
	// estimate.
	GrossProceedsEstimate bool `xml:"grossProceedsUsedEstimate" json:"gross_proceeds_estimate"`

	// PaymentToExecutivesEstimate marks the executive-payment portion
	// as an estimate.
	PaymentToExecutivesEstimate bool `xml:"paymentToExecutivesEstimate" json:"payment_to_executives_estimate"`

	// ClarificationOfResponse is an optional plain-language explanation
	// of how the proceeds will be deployed.
	ClarificationOfResponse string `xml:"clarificationOfResponse,omitempty" json:"clarification,omitempty"`
}

// Signature is one signature block at the bottom of Form D. The SEC
// requires at least one issuer signature.
type Signature struct {
	IssuerName        string `xml:"issuerName" json:"issuer_name"`
	SignatureName     string `xml:"signatureName" json:"signature_name"`
	NameOfSigner      string `xml:"nameOfSigner" json:"name_of_signer"`
	SignatureTitle    string `xml:"signatureTitle" json:"signature_title"`
	SignatureDate     string `xml:"signatureDate" json:"signature_date"` // YYYY-MM-DD
}

// FormDFiling is the in-memory representation of an EDGAR Form D
// filing. Callers populate the struct and submit it via
// Filer.FileFormD. The package marshals to the SEC's Form D XML
// primaryDocument format, wraps in the SGML EDGAR header, and posts
// to the EDGAR submission endpoint.
type FormDFiling struct {
	// SubmissionType is the EDGAR submission type. Use SubmissionFormD
	// for an original filing or SubmissionFormDA for an amendment.
	SubmissionType SubmissionType `json:"submission_type"`

	// FileNumber is the SEC-assigned file number from the prior filing
	// being amended. Required when SubmissionType is SubmissionFormDA;
	// must be empty for original filings.
	FileNumber string `json:"file_number,omitempty"`

	// IsAmendment mirrors (SubmissionType == SubmissionFormDA) into the
	// XML primaryDocument's <isAmendment> tag. Set by the adapter on
	// submit; callers may leave it zero.
	IsAmendment bool `json:"is_amendment"`

	// PrimaryIssuer is the issuer that signs the filing.
	PrimaryIssuer Issuer `json:"primary_issuer"`

	// RelatedIssuers is the list of additional issuers, if the
	// offering is by a group of related entities. Empty slice for the
	// common single-issuer case.
	RelatedIssuers []Issuer `json:"related_issuers,omitempty"`

	// RelatedPersons is the list of executive officers, directors, and
	// promoters required by Form D Item 3.
	RelatedPersons []RelatedPerson `json:"related_persons"`

	// IndustryGroup is the issuer's industry classification.
	IndustryGroup IndustryGroup `json:"industry_group"`

	// IssuerRevenueRange (Item 5) is the revenue range of the issuer.
	// Allowed: "Decline to Disclose", "No Revenues", "$1 - $1,000,000",
	// "$1,000,001 - $5,000,000", "$5,000,001 - $25,000,000",
	// "$25,000,001 - $100,000,000", "Over $100,000,000". For pooled
	// investment funds, set AggregateNetAssetValueRange instead.
	IssuerRevenueRange string `json:"issuer_revenue_range,omitempty"`

	// AggregateNetAssetValueRange (Item 5) is the NAV range for pooled
	// investment funds. Same allowed values as IssuerRevenueRange
	// except with NAV labels.
	AggregateNetAssetValueRange string `json:"aggregate_nav_range,omitempty"`

	// FederalExemptions is the list of claimed federal exemptions.
	// Typical: a single exemption like ExemptionRule506b or
	// ExemptionRule506c; fund offerings combine a Rule 506 exemption
	// with one of the Investment Company Act exemptions.
	FederalExemptions []FederalExemption `json:"federal_exemptions"`

	// IsNewFiling distinguishes a brand-new offering (true) from a
	// notice that the offering is already in progress (false). For
	// amendments this field is meaningless — set IsAmendment.
	IsNewFiling bool `json:"is_new_filing"`

	// DateOfFirstSale is the date of the first sale of securities in
	// the offering. Required for original filings unless YetToOccur
	// is true.
	DateOfFirstSale *time.Time `json:"date_of_first_sale,omitempty"`

	// FirstSaleYetToOccur signals that no sales have occurred yet (the
	// filing is anticipatory).
	FirstSaleYetToOccur bool `json:"first_sale_yet_to_occur"`

	// DurationMoreThanOneYear marks an offering with a duration of more
	// than one year.
	DurationMoreThanOneYear bool `json:"duration_more_than_one_year"`

	// TypesOfSecurities is the list of security types in this offering.
	// Allowed (multi-select): "Equity", "Debt", "Option, Warrant or
	// Other Right to Acquire Another Security", "Security to be
	// Acquired Upon Exercise of Option, Warrant or Other Right to
	// Acquire Security", "Pooled Investment Fund Interests",
	// "Tenant-in-Common Securities", "Mineral Property Securities",
	// "Other".
	TypesOfSecurities []string `json:"types_of_securities"`

	// IsBusinessCombinationTransaction marks the offering as part of a
	// business-combination transaction (M&A).
	IsBusinessCombinationTransaction bool `json:"is_business_combination_transaction"`

	// MinimumInvestmentAccepted is the minimum investment per investor
	// in USD. Zero means no minimum.
	MinimumInvestmentAccepted float64 `json:"minimum_investment_accepted"`

	// OfferingSalesAmount holds the offering-size + amount-sold +
	// remaining figures (Items 13).
	OfferingSalesAmount OfferingSalesAmount `json:"offering_sales_amount"`

	// InvestorCount holds the investor-count block (Item 14).
	InvestorCount InvestorCount `json:"investor_count"`

	// SalesCommissions is the optional sales-compensation block (Item
	// 15). The zero value indicates no commissions to report.
	SalesCommissions SalesCommissions `json:"sales_commissions"`

	// UseOfProceeds is the optional use-of-proceeds block (Item 16).
	UseOfProceeds UseOfProceeds `json:"use_of_proceeds"`

	// Signatures is the list of signature blocks at the bottom of the
	// form. At least one signature is required; typically one per
	// issuer.
	Signatures []Signature `json:"signatures"`
}

// Acknowledgment is the parsed reply EDGAR returns after a successful
// (or rejected) submission. EDGAR returns a SUBMISSION-ID immediately
// on receipt and then later sends a filing-status notification.
type Acknowledgment struct {
	// AccessionNumber is the SEC-assigned accession number for the
	// filing, of the form "0000000000-00-000000". Populated only once
	// EDGAR has accepted the filing.
	AccessionNumber string `json:"accession_number"`

	// SubmissionID is the EDGAR-assigned ephemeral identifier used for
	// status polling immediately after submission.
	SubmissionID string `json:"submission_id"`

	// Status is one of: "RECEIVED", "ACCEPTED", "SUSPENDED", "REJECTED",
	// "DISSEMINATING", "DISSEMINATED".
	Status string `json:"status"`

	// FileNumber is the SEC file number assigned to the filing (e.g.,
	// "021-123456" for Form D).
	FileNumber string `json:"file_number,omitempty"`

	// ReceivedAt is the timestamp EDGAR reports for receiving the
	// submission.
	ReceivedAt time.Time `json:"received_at"`

	// Messages are any informational, warning, or error messages EDGAR
	// returned in the acknowledgment.
	Messages []string `json:"messages,omitempty"`
}

// FilingStatus is the parsed reply from GetFilingStatus.
type FilingStatus struct {
	// AccessionNumber is the SEC-assigned accession number (once
	// EDGAR has assigned one).
	AccessionNumber string `json:"accession_number"`

	// Status is the current EDGAR status; same value set as
	// Acknowledgment.Status.
	Status string `json:"status"`

	// FileNumber is the SEC-assigned file number for the filing.
	FileNumber string `json:"file_number,omitempty"`

	// AcceptedAt is the timestamp EDGAR reports for acceptance (zero
	// until accepted).
	AcceptedAt time.Time `json:"accepted_at,omitempty"`

	// Messages are informational, warning, or error messages from the
	// status reply.
	Messages []string `json:"messages,omitempty"`
}
