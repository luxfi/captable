// Package iris — local type definitions for the IRS IRIS (Information
// Returns Intake System) e-file adapter. Modeled on the public IRIS A2A
// (Application-to-Application) schema (per Publication 5717 and the
// IRIS schema package published at https://www.irs.gov/e-file-providers/
// iris-online-portal). IRIS is mandatory for filers issuing >10 forms in
// aggregate per year effective Tax Year 2024.
//
// Source-of-design: Public-Spec
// Source-ref: https://www.irs.gov/e-file-providers/iris-online-portal
// Source-ref: IRS Publication 5717 — IRIS A2A Specifications
// Source-ref: IRS Publication 5718 — IRIS Electronic Filing Application Guide
package iris

import (
	"encoding/xml"
	"time"
)

// IRISEnv identifies the IRIS environment a Client targets. Production
// receives live filings; AATS (Assurance Testing System) is the
// sandbox the IRS provides for transmitter conformance.
type IRISEnv string

const (
	// EnvProduction is the live IRIS submission endpoint.
	EnvProduction IRISEnv = "production"

	// EnvAATS is the IRIS Assurance Testing System (sandbox).
	EnvAATS IRISEnv = "aats"
)

// FormType is the IRS form designation IRIS accepts. The string values
// are exactly what IRIS expects in the <FormTypeCd> element; do not
// localize. The TY-2024 IRIS schema covers the 1099 series enumerated
// below.
type FormType string

const (
	// Form1099DIV — dividends and distributions.
	Form1099DIV FormType = "1099-DIV"

	// Form1099B — proceeds from broker / barter exchange transactions.
	Form1099B FormType = "1099-B"

	// Form1099INT — interest income.
	Form1099INT FormType = "1099-INT"

	// Form1099MISC — miscellaneous income (rents, royalties, prizes,
	// medical/health payments, attorney gross proceeds).
	Form1099MISC FormType = "1099-MISC"

	// Form1099NEC — non-employee compensation (post-TY-2020 split-out
	// from 1099-MISC).
	Form1099NEC FormType = "1099-NEC"

	// Form1099OID — original issue discount.
	Form1099OID FormType = "1099-OID"

	// Form1099K — payment-card and third-party network transactions.
	Form1099K FormType = "1099-K"

	// Form1099R — distributions from pensions, annuities, retirement /
	// profit-sharing plans, IRAs, insurance contracts.
	Form1099R FormType = "1099-R"
)

// PaymentYearTypeCd is the IRIS <PaymentYr> + <CorrectedReturnInd>
// composite. Original/corrected/replaced semantics per Publication 5717
// §3.2.
type PaymentYearTypeCd string

const (
	// OriginalReturn marks a first-time submission for the tax year.
	OriginalReturn PaymentYearTypeCd = "O"

	// CorrectedReturn marks a correction to a previously-accepted
	// return. The OriginalReceiptID must reference the prior submission.
	CorrectedReturn PaymentYearTypeCd = "G"

	// ReplacementReturn marks a re-submission for a previously-rejected
	// return.
	ReplacementReturn PaymentYearTypeCd = "R"
)

// Address is the postal address shape used by every IRIS address
// element (payer, payee, transmitter). State must be a two-character
// US state code; for foreign addresses leave State empty and populate
// ForeignCountryCd with the ISO 3166-1 alpha-2 country code.
type Address struct {
	AddressLine1     string `xml:"AddressLine1Txt" json:"address_line_1"`
	AddressLine2     string `xml:"AddressLine2Txt,omitempty" json:"address_line_2,omitempty"`
	City             string `xml:"CityNm" json:"city"`
	State            string `xml:"USStateCd,omitempty" json:"state,omitempty"`
	ZipCode          string `xml:"USZIPCd,omitempty" json:"zip_code,omitempty"`
	ForeignCountryCd string `xml:"ForeignCountryCd,omitempty" json:"foreign_country_cd,omitempty"`
	ForeignPostalCd  string `xml:"ForeignPostalCd,omitempty" json:"foreign_postal_cd,omitempty"`
}

// Transmitter is the IRIS <Transmitter> element. The TCC (Transmitter
// Control Code) is issued by IRS through the IRIS Application portal
// and authenticates the transmitter on every submission. The
// transmitter is the entity that physically files; the payer (issuer)
// may or may not be the same legal entity.
type Transmitter struct {
	// TCC is the five-character IRS-assigned Transmitter Control Code.
	TCC string `xml:"TransmitterControlCd" json:"tcc"`

	// Name is the legal name registered on the IRIS Application.
	Name string `xml:"NameTransmitter" json:"name"`

	// EIN is the transmitter entity EIN (9 digits, no hyphen).
	EIN string `xml:"TransmitterEIN" json:"ein"`

	// Address is the transmitter's mailing address.
	Address Address `xml:"Address" json:"address"`

	// ContactName is the human contact at the transmitter for IRS
	// follow-up on the submission.
	ContactName string `xml:"ContactName" json:"contact_name"`

	// ContactPhone is the contact phone, 10 digits, no formatting.
	ContactPhone string `xml:"ContactPhone" json:"contact_phone"`

	// ContactEmail is the contact email.
	ContactEmail string `xml:"ContactEmail" json:"contact_email"`
}

// Payer is the IRIS <Payer> element. The payer is the issuer of the
// information return — the entity that made the reportable payment to
// the payee.
type Payer struct {
	// Name is the legal name of the payer.
	Name string `xml:"PayerNm" json:"name"`

	// EIN is the payer EIN (9 digits, no hyphen).
	EIN string `xml:"PayerEIN" json:"ein"`

	// NameControl is a four-character payer-name-control derived from
	// the payer's legal name per IRS Publication 4164 Part B. The
	// adapter computes a default if empty.
	NameControl string `xml:"PayerNameControl,omitempty" json:"name_control,omitempty"`

	// Address is the payer's mailing address.
	Address Address `xml:"Address" json:"address"`

	// PhoneNum is the payer's phone, 10 digits.
	PhoneNum string `xml:"PhoneNum,omitempty" json:"phone_num,omitempty"`
}

// Payee is the IRIS <Payee> element. The payee is the recipient of the
// reportable payment. TIN is the SSN (for natural persons) or EIN (for
// entities) without hyphens.
type Payee struct {
	// TIN is the payee tax identification number (9 digits, no hyphen).
	TIN string `xml:"TIN" json:"tin"`

	// TINType discriminates SSN ("S"), EIN ("E"), ITIN ("I"), or ATIN
	// ("A"). Required by the IRIS schema.
	TINType string `xml:"TINTypeCd" json:"tin_type"`

	// Name is the payee's legal name. Natural persons are formatted
	// "LAST FIRST MIDDLE" per IRS convention; entities use the
	// registered legal name.
	Name string `xml:"PayeeNm" json:"name"`

	// NameLine2 is the optional second name line (e.g., trust trustee
	// designation, "DBA" name, "C/O" line).
	NameLine2 string `xml:"PayeeNm2,omitempty" json:"name_line_2,omitempty"`

	// Address is the payee's mailing address.
	Address Address `xml:"Address" json:"address"`

	// AccountNum is the payer's account number for this payee. Optional
	// in most schemas but required if the payer files multiple returns
	// for the same payee (so the IRS can match corrections).
	AccountNum string `xml:"AccountNum,omitempty" json:"account_num,omitempty"`
}

// Form1099DIVData carries the box values for a 1099-DIV. Values map
// directly to the IRIS <Form1099Div> element children.
type Form1099DIVData struct {
	OrdinaryDividends            float64 `xml:"OrdinaryDividendAmt" json:"ordinary_dividends"`              // Box 1a
	QualifiedDividends           float64 `xml:"QualifiedDividendAmt,omitempty" json:"qualified_dividends"`  // Box 1b
	TotalCapGainDist             float64 `xml:"TotalCapitalGainDistribution,omitempty" json:"total_capital_gain_dist"` // Box 2a
	UnrecaptureSec1250Gain       float64 `xml:"UnrecapturedSec1250GainAmt,omitempty" json:"unrecapture_sec_1250_gain"` // Box 2b
	Section1202Gain              float64 `xml:"Section1202GainAmt,omitempty" json:"section_1202_gain"`      // Box 2c
	CollectiblesGain             float64 `xml:"CollectibleGainAmt,omitempty" json:"collectibles_gain"`      // Box 2d
	Section897OrdinaryDividends  float64 `xml:"Section897OrdinaryDividendAmt,omitempty" json:"section_897_ordinary_dividends"` // Box 2e
	Section897CapGain            float64 `xml:"Section897CapitalGainAmt,omitempty" json:"section_897_cap_gain"` // Box 2f
	NondividendDistributions     float64 `xml:"NondividendDistributionAmt,omitempty" json:"nondividend_distributions"` // Box 3
	FederalTaxWithheld           float64 `xml:"FederalTaxWithheldAmt,omitempty" json:"federal_tax_withheld"`   // Box 4
	Section199ADividends         float64 `xml:"Sec199ADividendAmt,omitempty" json:"section_199a_dividends"`    // Box 5
	InvestmentExpenses           float64 `xml:"InvestmentExpenseAmt,omitempty" json:"investment_expenses"`     // Box 6
	ForeignTaxPaid               float64 `xml:"ForeignTaxPaidAmt,omitempty" json:"foreign_tax_paid"`           // Box 7
	ForeignCountry               string  `xml:"ForeignCountryNm,omitempty" json:"foreign_country,omitempty"`   // Box 8
	CashLiquidationDistributions float64 `xml:"CashLiquidationDistribAmt,omitempty" json:"cash_liquidation_distributions"`     // Box 9
	NoncashLiqDistributions      float64 `xml:"NoncashLiquidationDistribAmt,omitempty" json:"noncash_liq_distributions"`       // Box 10
	ExemptInterestDividends      float64 `xml:"ExemptInterestDividendAmt,omitempty" json:"exempt_interest_dividends"`          // Box 12
	SpecifiedPrivActBondIntDiv   float64 `xml:"SpecPrivateActivityBondInterestDividendAmt,omitempty" json:"specified_priv_act_bond_int_div"` // Box 13
	StateTaxWithheld             float64 `xml:"StateTaxWithheldAmt,omitempty" json:"state_tax_withheld"`       // Box 14
	StateCd                      string  `xml:"StateCd,omitempty" json:"state_cd,omitempty"`                   // Box 15
	StatePayerStateNum           string  `xml:"StatePayerStateNum,omitempty" json:"state_payer_state_num,omitempty"`           // Box 16
	StateIncome                  float64 `xml:"StateIncomeAmt,omitempty" json:"state_income"`                  // Box 16 (alt)
	FATCAFilingRequirement       bool    `xml:"FATCAFilingRequirementInd,omitempty" json:"fatca_filing_requirement"`
	SecondTINNotice              bool    `xml:"SecondTINNoticeInd,omitempty" json:"second_tin_notice"`
}

// Form1099BData carries the box values for a 1099-B (broker/barter
// proceeds). Values map directly to the IRIS <Form1099B> element
// children.
type Form1099BData struct {
	CUSIP                  string  `xml:"CUSIPNum,omitempty" json:"cusip,omitempty"`              // Box (1a)
	Description            string  `xml:"PropertyDescription" json:"description"`                 // Box 1a
	DateAcquired           string  `xml:"AcquiredDt,omitempty" json:"date_acquired,omitempty"`    // Box 1b YYYY-MM-DD
	DateSold               string  `xml:"SoldDt" json:"date_sold"`                                // Box 1c YYYY-MM-DD
	Proceeds               float64 `xml:"ProceedsAmt" json:"proceeds"`                            // Box 1d
	CostBasis              float64 `xml:"CostOrBasisAmt,omitempty" json:"cost_basis"`             // Box 1e
	AccruedMarketDiscount  float64 `xml:"AccruedMarketDiscountAmt,omitempty" json:"accrued_market_discount"` // Box 1f
	WashSaleLossDisallowed float64 `xml:"WashSaleLossDisallowedAmt,omitempty" json:"wash_sale_loss_disallowed"` // Box 1g
	ShortTermLongTerm      string  `xml:"ShortLongTermInd" json:"short_term_long_term"`           // Box 2 — S/L/X
	OrdinaryInd            bool    `xml:"OrdinaryInd,omitempty" json:"ordinary_ind,omitempty"`    // Box 2 alt
	FederalTaxWithheld     float64 `xml:"FederalTaxWithheldAmt,omitempty" json:"federal_tax_withheld"` // Box 4
	NoncoveredSecurity     bool    `xml:"NoncoveredSecurityInd,omitempty" json:"noncovered_security"`  // Box 5
	GrossProceedsLossNet   string  `xml:"GrossProceedsLossNetInd,omitempty" json:"gross_proceeds_loss_net"` // Box 6 — gross/net
	Section897OrdLossNet   bool    `xml:"Section897LossNetReportingInd,omitempty" json:"section_897_ord_loss_net"`
	BartsAndExchangesAmt   float64 `xml:"BartsAndExchangesAmt,omitempty" json:"bartering_amt"`    // Box 13
	StateTaxWithheld       float64 `xml:"StateTaxWithheldAmt,omitempty" json:"state_tax_withheld"`
	StateCd                string  `xml:"StateCd,omitempty" json:"state_cd,omitempty"`
	StatePayerStateNum     string  `xml:"StatePayerStateNum,omitempty" json:"state_payer_state_num,omitempty"`
}

// Form1099INTData carries the box values for a 1099-INT (interest
// income).
type Form1099INTData struct {
	PayerRTN                string  `xml:"PayerRTN,omitempty" json:"payer_rtn,omitempty"`          // Box 0
	InterestIncome          float64 `xml:"InterestIncomeAmt" json:"interest_income"`               // Box 1
	EarlyWithdrawalPenalty  float64 `xml:"EarlyWithdrawalPenaltyAmt,omitempty" json:"early_withdrawal_penalty"` // Box 2
	USSavingsBondTreasInt   float64 `xml:"USSavingsBondTreasIntAmt,omitempty" json:"us_savings_bond_treas_int"` // Box 3
	FederalTaxWithheld      float64 `xml:"FederalTaxWithheldAmt,omitempty" json:"federal_tax_withheld"` // Box 4
	InvestmentExpenses      float64 `xml:"InvestmentExpenseAmt,omitempty" json:"investment_expenses"`   // Box 5
	ForeignTaxPaid          float64 `xml:"ForeignTaxPaidAmt,omitempty" json:"foreign_tax_paid"`         // Box 6
	ForeignCountry          string  `xml:"ForeignCountryNm,omitempty" json:"foreign_country,omitempty"` // Box 7
	TaxExemptInterest       float64 `xml:"TaxExemptInterestAmt,omitempty" json:"tax_exempt_interest"`   // Box 8
	SpecPvtActBondInterest  float64 `xml:"SpecPrivateActivityBondInterestAmt,omitempty" json:"spec_pvt_act_bond_interest"` // Box 9
	MarketDiscount          float64 `xml:"MarketDiscountAmt,omitempty" json:"market_discount"`          // Box 10
	BondPremium             float64 `xml:"BondPremiumAmt,omitempty" json:"bond_premium"`                // Box 11
	BondPremiumTreasury     float64 `xml:"BondPremiumOnTreasuryAmt,omitempty" json:"bond_premium_treasury"` // Box 12
	BondPremiumTaxExempt    float64 `xml:"BondPremiumOnTaxExemptBondAmt,omitempty" json:"bond_premium_tax_exempt"` // Box 13
	TaxExemptCUSIP          string  `xml:"TaxExemptCUSIPNum,omitempty" json:"tax_exempt_cusip,omitempty"` // Box 14
	StateCd                 string  `xml:"StateCd,omitempty" json:"state_cd,omitempty"`                 // Box 15
	StatePayerStateNum      string  `xml:"StatePayerStateNum,omitempty" json:"state_payer_state_num,omitempty"` // Box 16
	StateTaxWithheld        float64 `xml:"StateTaxWithheldAmt,omitempty" json:"state_tax_withheld"`     // Box 17
	FATCAFilingRequirement  bool    `xml:"FATCAFilingRequirementInd,omitempty" json:"fatca_filing_requirement"`
	SecondTINNotice         bool    `xml:"SecondTINNoticeInd,omitempty" json:"second_tin_notice"`
}

// Form1099MISCData carries the box values for a 1099-MISC. Boxes
// renumbered per TY-2024 schema.
type Form1099MISCData struct {
	Rents                       float64 `xml:"RentAmt,omitempty" json:"rents"`                              // Box 1
	Royalties                   float64 `xml:"RoyaltyAmt,omitempty" json:"royalties"`                       // Box 2
	OtherIncome                 float64 `xml:"OtherIncomeAmt,omitempty" json:"other_income"`                // Box 3
	FederalTaxWithheld          float64 `xml:"FederalTaxWithheldAmt,omitempty" json:"federal_tax_withheld"` // Box 4
	FishingBoatProceeds         float64 `xml:"FishingBoatProceedsAmt,omitempty" json:"fishing_boat_proceeds"` // Box 5
	MedicalHealthPayments       float64 `xml:"MedicalHealthPaymentAmt,omitempty" json:"medical_health_payments"` // Box 6
	PayerDirectSalesInd         bool    `xml:"PayerDirectSalesInd,omitempty" json:"payer_direct_sales"`     // Box 7
	SubstitutePayments          float64 `xml:"SubstitutePaymentsAmt,omitempty" json:"substitute_payments"`  // Box 8
	CropInsuranceProceeds       float64 `xml:"CropInsuranceProceedsAmt,omitempty" json:"crop_insurance_proceeds"` // Box 9
	GrossAttorneyProceeds       float64 `xml:"GrossAttorneyProceedsAmt,omitempty" json:"gross_attorney_proceeds"` // Box 10
	FishPurchasedForResale      float64 `xml:"FishPurchasedForResaleAmt,omitempty" json:"fish_purchased_for_resale"` // Box 11
	Section409ADeferrals        float64 `xml:"Section409ADeferralsAmt,omitempty" json:"section_409a_deferrals"` // Box 12
	FATCAFilingRequirement      bool    `xml:"FATCAFilingRequirementInd,omitempty" json:"fatca_filing_requirement"` // Box 13
	ExcessGoldenParachutePayts  float64 `xml:"ExcessGoldenParachutePaymentsAmt,omitempty" json:"excess_golden_parachute_payments"` // Box 14
	NonqualifiedDeferredComp    float64 `xml:"NonqualifiedDeferredCompensationAmt,omitempty" json:"nonqualified_deferred_compensation"` // Box 15
	StateTaxWithheld            float64 `xml:"StateTaxWithheldAmt,omitempty" json:"state_tax_withheld"`     // Box 16
	StatePayerStateNum          string  `xml:"StatePayerStateNum,omitempty" json:"state_payer_state_num,omitempty"` // Box 17
	StateIncome                 float64 `xml:"StateIncomeAmt,omitempty" json:"state_income"`                // Box 18
	StateCd                     string  `xml:"StateCd,omitempty" json:"state_cd,omitempty"`
	SecondTINNotice             bool    `xml:"SecondTINNoticeInd,omitempty" json:"second_tin_notice"`
}

// Form1099NECData carries the box values for a 1099-NEC (non-employee
// compensation). The form is small: only 4 reportable boxes.
type Form1099NECData struct {
	NonemployeeCompensation float64 `xml:"NonemployeeCompensationAmt" json:"nonemployee_compensation"`     // Box 1
	PayerDirectSalesInd     bool    `xml:"PayerDirectSalesInd,omitempty" json:"payer_direct_sales"`        // Box 2
	FederalTaxWithheld      float64 `xml:"FederalTaxWithheldAmt,omitempty" json:"federal_tax_withheld"`    // Box 4
	StateTaxWithheld        float64 `xml:"StateTaxWithheldAmt,omitempty" json:"state_tax_withheld"`        // Box 5
	StatePayerStateNum      string  `xml:"StatePayerStateNum,omitempty" json:"state_payer_state_num,omitempty"` // Box 6
	StateIncome             float64 `xml:"StateIncomeAmt,omitempty" json:"state_income"`                   // Box 7
	StateCd                 string  `xml:"StateCd,omitempty" json:"state_cd,omitempty"`
	SecondTINNotice         bool    `xml:"SecondTINNoticeInd,omitempty" json:"second_tin_notice"`
}

// Form1099OIDData carries the box values for a 1099-OID (original
// issue discount).
type Form1099OIDData struct {
	OriginalIssueDiscount        float64 `xml:"OrigIssueDiscountAmt" json:"original_issue_discount"`              // Box 1
	OtherPeriodicInterest        float64 `xml:"OtherPeriodicInterestAmt,omitempty" json:"other_periodic_interest"` // Box 2
	EarlyWithdrawalPenalty       float64 `xml:"EarlyWithdrawalPenaltyAmt,omitempty" json:"early_withdrawal_penalty"` // Box 3
	FederalTaxWithheld           float64 `xml:"FederalTaxWithheldAmt,omitempty" json:"federal_tax_withheld"`      // Box 4
	MarketDiscount               float64 `xml:"MarketDiscountAmt,omitempty" json:"market_discount"`               // Box 5
	AcquisitionPremium           float64 `xml:"AcquisitionPremiumAmt,omitempty" json:"acquisition_premium"`       // Box 6
	Description                  string  `xml:"PropertyDescription,omitempty" json:"description,omitempty"`       // Box 7
	OriginalIssueDiscOnUSTreas   float64 `xml:"OrigIssueDiscOnUSTreasAmt,omitempty" json:"orig_issue_disc_on_us_treas"` // Box 8
	InvestmentExpenses           float64 `xml:"InvestmentExpenseAmt,omitempty" json:"investment_expenses"`        // Box 9
	BondPremium                  float64 `xml:"BondPremiumAmt,omitempty" json:"bond_premium"`                     // Box 10
	TaxExemptOIDAmt              float64 `xml:"TaxExemptOIDAmt,omitempty" json:"tax_exempt_oid"`                  // Box 11
	StateCd                      string  `xml:"StateCd,omitempty" json:"state_cd,omitempty"`
	StatePayerStateNum           string  `xml:"StatePayerStateNum,omitempty" json:"state_payer_state_num,omitempty"`
	StateTaxWithheld             float64 `xml:"StateTaxWithheldAmt,omitempty" json:"state_tax_withheld"`
	FATCAFilingRequirement       bool    `xml:"FATCAFilingRequirementInd,omitempty" json:"fatca_filing_requirement"`
	SecondTINNotice              bool    `xml:"SecondTINNoticeInd,omitempty" json:"second_tin_notice"`
}

// Form1099KData carries the box values for a 1099-K (payment-card /
// third-party network transactions). The TY-2024 threshold is $5,000
// aggregate (will fall further in subsequent TYs).
type Form1099KData struct {
	GrossAmount             float64 `xml:"GrossAmt" json:"gross_amount"`                              // Box 1a
	CardNotPresentTrans     float64 `xml:"CardNotPresentTransactionsAmt,omitempty" json:"card_not_present_trans"` // Box 1b
	MerchantCategoryCd      string  `xml:"MerchantCategoryCd,omitempty" json:"merchant_category_cd,omitempty"`     // Box 2
	PaymentTransNum         int     `xml:"PaymentTransactionNum,omitempty" json:"payment_transaction_num"`         // Box 3
	FederalTaxWithheld      float64 `xml:"FederalTaxWithheldAmt,omitempty" json:"federal_tax_withheld"`            // Box 4
	JanGrossAmount          float64 `xml:"JanuaryGrossAmt,omitempty" json:"jan_gross_amount"`                      // Box 5a
	FebGrossAmount          float64 `xml:"FebruaryGrossAmt,omitempty" json:"feb_gross_amount"`                     // Box 5b
	MarGrossAmount          float64 `xml:"MarchGrossAmt,omitempty" json:"mar_gross_amount"`                        // Box 5c
	AprGrossAmount          float64 `xml:"AprilGrossAmt,omitempty" json:"apr_gross_amount"`                        // Box 5d
	MayGrossAmount          float64 `xml:"MayGrossAmt,omitempty" json:"may_gross_amount"`                          // Box 5e
	JunGrossAmount          float64 `xml:"JuneGrossAmt,omitempty" json:"jun_gross_amount"`                         // Box 5f
	JulGrossAmount          float64 `xml:"JulyGrossAmt,omitempty" json:"jul_gross_amount"`                         // Box 5g
	AugGrossAmount          float64 `xml:"AugustGrossAmt,omitempty" json:"aug_gross_amount"`                       // Box 5h
	SepGrossAmount          float64 `xml:"SeptemberGrossAmt,omitempty" json:"sep_gross_amount"`                    // Box 5i
	OctGrossAmount          float64 `xml:"OctoberGrossAmt,omitempty" json:"oct_gross_amount"`                      // Box 5j
	NovGrossAmount          float64 `xml:"NovemberGrossAmt,omitempty" json:"nov_gross_amount"`                     // Box 5k
	DecGrossAmount          float64 `xml:"DecemberGrossAmt,omitempty" json:"dec_gross_amount"`                     // Box 5l
	StateCd                 string  `xml:"StateCd,omitempty" json:"state_cd,omitempty"`                            // Box 6
	StatePayerStateNum      string  `xml:"StatePayerStateNum,omitempty" json:"state_payer_state_num,omitempty"`    // Box 7
	StateIncome             float64 `xml:"StateIncomeAmt,omitempty" json:"state_income"`                           // Box 8
	StateTaxWithheld        float64 `xml:"StateTaxWithheldAmt,omitempty" json:"state_tax_withheld"`                // Box 9
	PSEIndicator            string  `xml:"PaymentSettlementEntityInd,omitempty" json:"pse_indicator,omitempty"`    // PSE/EPF
	TransactionsIndicator   string  `xml:"TransactionsIndicatorCd,omitempty" json:"transactions_indicator"`        // payment_card or third_party_network
}

// Form1099RData carries the box values for a 1099-R (pension /
// retirement distributions).
type Form1099RData struct {
	GrossDistribution         float64 `xml:"GrossDistributionAmt" json:"gross_distribution"`             // Box 1
	TaxableAmount             float64 `xml:"TaxableAmt,omitempty" json:"taxable_amount"`                 // Box 2a
	TaxableAmountNotDetermined bool   `xml:"TaxableAmtNotDeterminedInd,omitempty" json:"taxable_amount_not_determined"` // Box 2b
	TotalDistributionInd      bool    `xml:"TotalDistributionInd,omitempty" json:"total_distribution"`   // Box 2b
	CapitalGain               float64 `xml:"CapitalGainAmt,omitempty" json:"capital_gain"`               // Box 3
	FederalTaxWithheld        float64 `xml:"FederalTaxWithheldAmt,omitempty" json:"federal_tax_withheld"` // Box 4
	EmployeeContribOrInsPrem  float64 `xml:"EmployeeContribOrInsPremAmt,omitempty" json:"employee_contrib_or_ins_prem"` // Box 5
	NetUnrealizedAppreciation float64 `xml:"NetUnrealizedApprecAmt,omitempty" json:"net_unrealized_appreciation"`       // Box 6
	DistributionCodeCd        string  `xml:"DistributionCodeCd,omitempty" json:"distribution_code_cd"`   // Box 7
	IRASEPSIMPLEInd           bool    `xml:"IRASEPSIMPLEInd,omitempty" json:"ira_sep_simple"`            // Box 7 (IRA/SEP/SIMPLE)
	OtherAmount               float64 `xml:"OtherAmt,omitempty" json:"other_amount"`                     // Box 8
	OtherPct                  float64 `xml:"OtherPct,omitempty" json:"other_pct"`                        // Box 8 (alt)
	TotalEmployeeContribAmt   float64 `xml:"YourPercentTotalDistributionPct,omitempty" json:"total_employee_contrib_pct"` // Box 9a
	EmployeeContributionsAmt  float64 `xml:"EmployeeContributionsAmt,omitempty" json:"employee_contributions_amt"`         // Box 9b
	AmountAllocatedToIRR      float64 `xml:"AmountAllocatedToIRRAmt,omitempty" json:"amount_allocated_to_irr"`             // Box 10
	FirstYearRothContribYr    int     `xml:"FirstYearOfDsgnRothContribYr,omitempty" json:"first_year_roth_contrib_yr"`     // Box 11
	StateTaxWithheld          float64 `xml:"StateTaxWithheldAmt,omitempty" json:"state_tax_withheld"`    // Box 14
	StateCd                   string  `xml:"StateCd,omitempty" json:"state_cd,omitempty"`                // Box 15
	StatePayerStateNum        string  `xml:"StatePayerStateNum,omitempty" json:"state_payer_state_num,omitempty"` // Box 15
	StateDistribution         float64 `xml:"StateDistributionAmt,omitempty" json:"state_distribution"`   // Box 16
}

// PayeeBlock is the per-payee submission record carried inside a
// FormSubmission. The Data field is one of Form1099*Data per the
// FormType on the enclosing submission; the adapter chooses the wire
// element name from FormType at marshal time.
type PayeeBlock struct {
	// Payee identifies the recipient of the reportable payment.
	Payee Payee `json:"payee"`

	// Data is one of the Form1099*Data structs; the adapter dispatches
	// based on the submission's FormType.
	Data any `json:"data"`

	// CorrectionType is the per-payee correction indicator. Empty for
	// original returns; "G" for a correction; "C" for a corrected-
	// follow-up correction (the rare two-stage correction). Per
	// Publication 5717 §3.2.
	CorrectionType string `json:"correction_type,omitempty"`
}

// FormSubmission is one IRIS submission. A submission carries one
// (transmitter, payer, formType, taxYear) tuple plus a slice of payee
// records. The IRIS schema bounds a single submission at 100,000 payee
// records; the adapter validates this client-side.
type FormSubmission struct {
	// FormType selects the 1099 variant for the submission. All payee
	// records in a single submission must be the same form type.
	FormType FormType `json:"form_type"`

	// TaxYear is the four-digit tax year the forms cover.
	TaxYear int `json:"tax_year"`

	// PaymentYearTypeCd identifies original vs. corrected vs.
	// replacement submission. Defaults to OriginalReturn if empty.
	PaymentYearTypeCd PaymentYearTypeCd `json:"payment_year_type_cd,omitempty"`

	// OriginalReceiptID is the IRIS Receipt ID of the prior submission
	// being corrected or replaced. Required when PaymentYearTypeCd is
	// CorrectedReturn or ReplacementReturn.
	OriginalReceiptID string `json:"original_receipt_id,omitempty"`

	// Transmitter is the entity that physically files. Required.
	Transmitter Transmitter `json:"transmitter"`

	// Payer is the issuer of the information return. Required.
	Payer Payer `json:"payer"`

	// Payees is the slice of payee records (one per payee). At least
	// one is required; the IRIS hard limit is 100,000 per submission.
	Payees []PayeeBlock `json:"payees"`

	// TestFileInd flags the submission as a test file rather than a
	// production file. Always true in AATS; always false in production.
	// The adapter sets this automatically from IRISEnv if zero.
	TestFileInd bool `json:"test_file_ind,omitempty"`
}

// Acknowledgment is the parsed IRIS response after a SubmitForm or
// CorrectSubmission call. IRIS returns a ReceiptID immediately on
// receipt and the validation result (Accepted / Rejected / Accepted
// with errors) once schema/validation completes.
type Acknowledgment struct {
	// ReceiptID is the IRIS-assigned submission identifier. Format:
	// "{TCC}-{seq}-{YYYYMMDDHHMMSS}".
	ReceiptID string `json:"receipt_id"`

	// Status is one of: "Received", "Processing", "Accepted",
	// "AcceptedWithErrors", "Rejected".
	Status string `json:"status"`

	// SubmittedAt is the timestamp IRIS reports for receipt.
	SubmittedAt time.Time `json:"submitted_at"`

	// Errors holds per-payee schema or validation errors, if any.
	Errors []SubmissionError `json:"errors,omitempty"`

	// Messages are informational messages from IRIS (e.g., resource
	// constraints, retry suggestions).
	Messages []string `json:"messages,omitempty"`
}

// SubmissionError is one per-payee or per-form error returned by IRIS.
type SubmissionError struct {
	// PayeeIndex is the zero-based index of the payee record in the
	// submission's Payees slice that caused the error. -1 if the error
	// is at the submission level (transmitter / payer / schema).
	PayeeIndex int `json:"payee_index"`

	// Code is the IRIS error code (e.g., "VAL-0001"). Stable for
	// machine matching.
	Code string `json:"code"`

	// Message is the human-readable error message.
	Message string `json:"message"`

	// Severity is one of "Error" (rejection-causing) or "Warning"
	// (informational; accepted-with-errors).
	Severity string `json:"severity"`
}

// Status is the parsed reply from GetSubmissionStatus.
type Status struct {
	ReceiptID    string            `json:"receipt_id"`
	Status       string            `json:"status"`
	AcceptedAt   time.Time         `json:"accepted_at,omitempty"`
	RejectedAt   time.Time         `json:"rejected_at,omitempty"`
	AcceptedCnt  int               `json:"accepted_cnt,omitempty"`
	RejectedCnt  int               `json:"rejected_cnt,omitempty"`
	Errors       []SubmissionError `json:"errors,omitempty"`
}

// Submission is the summary record returned by ListSubmissions.
type Submission struct {
	ReceiptID   string    `json:"receipt_id"`
	FormType    FormType  `json:"form_type"`
	TaxYear     int       `json:"tax_year"`
	PayeeCount  int       `json:"payee_count"`
	Status      string    `json:"status"`
	SubmittedAt time.Time `json:"submitted_at"`
}

// ListOptions controls the ListSubmissions paginated query.
type ListOptions struct {
	// TCC scopes the listing to a single transmitter. Required.
	TCC string `json:"tcc"`

	// FormType filters by form type if non-empty.
	FormType FormType `json:"form_type,omitempty"`

	// TaxYear filters by tax year if non-zero.
	TaxYear int `json:"tax_year,omitempty"`

	// Status filters by acknowledgment status if non-empty.
	Status string `json:"status,omitempty"`

	// SubmittedAfter / SubmittedBefore bound the submission time range.
	SubmittedAfter  time.Time `json:"submitted_after,omitempty"`
	SubmittedBefore time.Time `json:"submitted_before,omitempty"`

	// PageSize bounds the per-page record count (default 100, max
	// 1000).
	PageSize int `json:"page_size,omitempty"`

	// PageToken is the opaque continuation token returned by the prior
	// call.
	PageToken string `json:"page_token,omitempty"`
}

// Correction is the per-payee correction record passed to
// CorrectSubmission. The adapter constructs a corrected FormSubmission
// from the original (resolved via OriginalReceiptID) plus the
// Correction.Payees overlay. Transmitter, Payer, FormType and TaxYear
// must be supplied by the caller — IRIS keys corrections on the
// (TCC, OriginalReceiptID, FormType, TaxYear) tuple plus the per-payee
// CorrectionType.
type Correction struct {
	// CorrectionType selects the IRIS correction kind: "OneStep" for
	// the common single-step correction, "TwoStep" for the rare
	// two-step correction (where the original payee identification was
	// wrong — file a $0 correction first, then file the right amount
	// against the right payee).
	CorrectionType string `json:"correction_type"`

	// Reason is a human-readable explanation of the correction (logged
	// for audit; not transmitted to IRIS).
	Reason string `json:"reason"`

	// FormType selects the 1099 variant. Must match the original
	// submission's form type.
	FormType FormType `json:"form_type"`

	// TaxYear is the original submission's tax year.
	TaxYear int `json:"tax_year"`

	// Transmitter identifies the entity filing the correction.
	Transmitter Transmitter `json:"transmitter"`

	// Payer is the original payer. EIN and Name MUST match the
	// original submission; IRIS rejects corrections under a different
	// payer EIN.
	Payer Payer `json:"payer"`

	// Payees is the set of corrected payee records.
	Payees []PayeeBlock `json:"payees"`
}

// --- XML wire envelope ---
//
// The IRIS schema wraps a submission in a <Form1099Submission> root
// element with <SubmissionHeader> and per-form <Form1099*> repeating
// elements. The adapter marshals to the wire shape below; the inner
// element name is selected at marshal time from the submission's
// FormType.

// xmlSubmissionRoot is the wire root element.
type xmlSubmissionRoot struct {
	XMLName             xml.Name           `xml:"Form1099Submission"`
	Xmlns               string             `xml:"xmlns,attr"`
	XmlnsXSI            string             `xml:"xmlns:xsi,attr"`
	SubmissionHeader    xmlSubmissionHdr   `xml:"SubmissionHeader"`
	Form1099Records     []xmlRawForm       `xml:",any"`
}

// xmlSubmissionHdr is the SubmissionHeader element.
type xmlSubmissionHdr struct {
	TaxYear            int                  `xml:"TaxYr"`
	FormTypeCd         FormType             `xml:"FormTypeCd"`
	PaymentYearTypeCd  PaymentYearTypeCd    `xml:"PaymentYearTypeCd"`
	OriginalReceiptID  string               `xml:"OriginalReceiptID,omitempty"`
	TestFileInd        string               `xml:"TestFileInd,omitempty"` // "X" if test
	Transmitter        Transmitter          `xml:"Transmitter"`
	Payer              Payer                `xml:"Payer"`
	TotalPayeeCnt      int                  `xml:"TotalPayeeCnt"`
}

// xmlRawForm carries one per-payee record. The element name is set
// from FormType at marshal time.
type xmlRawForm struct {
	XMLName        xml.Name
	CorrectionType string  `xml:"CorrectionTypeCd,omitempty"`
	Payee          Payee   `xml:"Payee"`
	Inner          xmlInner
}

// xmlInner is the form-specific payload. Exactly one of the fields
// below is populated per submission, chosen by FormType.
type xmlInner struct {
	DIV  *Form1099DIVData  `xml:"DividendPayments,omitempty"`
	B    *Form1099BData    `xml:"BrokerProceeds,omitempty"`
	INT  *Form1099INTData  `xml:"InterestIncome,omitempty"`
	MISC *Form1099MISCData `xml:"MiscellaneousIncome,omitempty"`
	NEC  *Form1099NECData  `xml:"NonemployeeCompensation,omitempty"`
	OID  *Form1099OIDData  `xml:"OriginalIssueDiscount,omitempty"`
	K    *Form1099KData    `xml:"PaymentCardOrThirdParty,omitempty"`
	R    *Form1099RData    `xml:"Distributions,omitempty"`
}
