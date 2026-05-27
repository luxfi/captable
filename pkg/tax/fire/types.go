// Package fire — local type definitions for the IRS FIRE (Filing
// Information Returns Electronically) adapter. FIRE is the legacy IRS
// e-file surface for the 1099 series; IRIS is the modern replacement
// and is mandatory going forward (TY 2024+). FIRE is preserved here to
// file corrections against older years that were originally submitted
// to FIRE and remain in IRS's FIRE-side records.
//
// FIRE file format is fixed-width records per Publication 1220 — T,
// A, B, C, F record types in a single file, 750 bytes per record (left-
// padded with zeros, right-padded with blanks).
//
// Source-of-design: Public-Spec
// Source-ref: IRS Publication 1220 — Specifications for Electronic Filing of Forms 1097, 1098, 1099, 3921, 3922, 5498, and W-2G
// Source-ref: https://www.irs.gov/pub/irs-pdf/p1220.pdf
package fire

import "time"

// FIREEnv identifies the FIRE environment a Client targets. Production
// is the live FIRE filing surface; Test is the IRS-provided test
// environment used for transmitter conformance.
type FIREEnv string

const (
	// EnvProduction is the live FIRE production endpoint.
	EnvProduction FIREEnv = "production"

	// EnvTest is the FIRE test/sandbox endpoint.
	EnvTest FIREEnv = "test"
)

// RecordType is the FIRE record-type code in column 1 of every fixed-
// width record. Per Publication 1220 Part C.
type RecordType byte

const (
	// RecordT is the Transmitter record. One per file.
	RecordT RecordType = 'T'

	// RecordA is the Payer record. One per payer-form-type group.
	RecordA RecordType = 'A'

	// RecordB is the Payee/Recipient record. One per recipient.
	RecordB RecordType = 'B'

	// RecordC is the End-of-Payer record. Closes an A-block.
	RecordC RecordType = 'C'

	// RecordF is the End-of-File record. One per file.
	RecordF RecordType = 'F'

	// RecordK is the State Totals record (CFSF). One per state in
	// the file when participating in CFSF.
	RecordK RecordType = 'K'
)

// FormCode is the FIRE Type-of-Return code on the A record. Per
// Publication 1220 Part C §2 the codes are single-character or two-
// character identifiers; values are exactly what FIRE expects on the
// wire.
type FormCode string

const (
	FormCode1099DIV  FormCode = "1 "
	FormCode1099INT  FormCode = "6 "
	FormCode1099B    FormCode = "B "
	FormCode1099MISC FormCode = "A "
	FormCode1099NEC  FormCode = "NE"
	FormCode1099OID  FormCode = "D "
	FormCode1099K    FormCode = "MC"
	FormCode1099R    FormCode = "9 "
)

// TransmitterRecord is the FIRE T record. Position-1 == 'T'. The
// record carries the transmitter's TCC, EIN, name, and file
// identification. One T record per file.
type TransmitterRecord struct {
	// PaymentYear is the four-digit tax year. Position 2-5.
	PaymentYear int

	// PriorYear flags this as a prior-year filing. 'P' if yes,
	// blank otherwise. Position 6.
	PriorYear bool

	// TIN is the transmitter EIN (9 digits, no hyphen). Position 7-15.
	TIN string

	// TCC is the IRS-assigned Transmitter Control Code (5 chars).
	// Position 16-20.
	TCC string

	// TestFileInd is 'T' for test files, blank for production.
	// Position 28.
	TestFileInd bool

	// ForeignEntity is '1' if transmitter is a foreign entity, blank
	// otherwise. Position 29.
	ForeignEntity bool

	// TransmitterName is the entity name (40 chars). Position 30-69.
	TransmitterName string

	// TransmitterNameCont is the continuation name (40 chars).
	// Position 70-109.
	TransmitterNameCont string

	// CompanyName is the company name for the transmitter (40 chars).
	// Position 110-149.
	CompanyName string

	// CompanyNameCont is the continuation name (40 chars). Position
	// 150-189.
	CompanyNameCont string

	// CompanyMailingAddress is the address line (40 chars). Position
	// 190-229.
	CompanyMailingAddress string

	// CompanyCity (40 chars). Position 230-269.
	CompanyCity string

	// CompanyState (2 chars). Position 270-271.
	CompanyState string

	// CompanyZip (9 chars, no hyphen). Position 272-280.
	CompanyZip string

	// TotalPayees is the total count of B records in the entire file.
	// Position 296-303.
	TotalPayees int

	// ContactName (40 chars). Position 304-343.
	ContactName string

	// ContactPhone (15 chars, digits + optional ext "Ext nnnnn").
	// Position 344-358.
	ContactPhone string

	// ContactEmail (50 chars). Position 359-408.
	ContactEmail string

	// VendorIndicator is 'I' (in-house) or 'V' (vendor). Position
	// 519.
	VendorIndicator string

	// VendorName (40 chars). Position 520-559.
	VendorName string

	// VendorMailingAddress (40 chars). Position 560-599.
	VendorMailingAddress string

	// VendorCity (40 chars). Position 600-639.
	VendorCity string

	// VendorState (2 chars). Position 640-641.
	VendorState string

	// VendorZip (9 chars). Position 642-650.
	VendorZip string

	// VendorContactName (40 chars). Position 651-690.
	VendorContactName string

	// VendorContactPhone (15 chars). Position 691-705.
	VendorContactPhone string

	// VendorForeignEntityInd is '1' if the vendor is foreign.
	// Position 740.
	VendorForeignEntityInd bool

	// SequenceNum is the record sequence number; T is always 1.
	// Position 500-507.
	SequenceNum int
}

// PayerRecord is the FIRE A record. Position-1 == 'A'. One A record
// per (payer, form-type) group. The record carries the payer's EIN,
// the form code, and the amount-code map.
type PayerRecord struct {
	// PaymentYear (4 digits). Position 2-5.
	PaymentYear int

	// CFSFInd is '1' if this payer participates in the Combined
	// Federal/State Filing program for this submission. Position 6.
	CFSFInd bool

	// PayerTIN (9 digits, no hyphen). Position 12-20.
	PayerTIN string

	// PayerNameControl (4 chars). Position 21-24.
	PayerNameControl string

	// LastFilingInd is '1' to indicate the payer's last filing.
	// Position 25.
	LastFilingInd bool

	// TypeOfReturn is the FIRE form code. Position 26-27.
	TypeOfReturn FormCode

	// AmountCodes is the concatenated list of amount-codes the B
	// records will populate for this payer-form group (e.g., "1234A"
	// for 1099-DIV boxes 1a/1b/2a/3/4). Up to 18 codes; left-justified
	// in positions 28-45.
	AmountCodes string

	// PayerName (40 chars). Position 52-91.
	PayerName string

	// PayerNameCont (40 chars). Position 92-131.
	PayerNameCont string

	// PayerShippingAddress (40 chars). Position 132-171.
	PayerShippingAddress string

	// PayerCity (40 chars). Position 172-211.
	PayerCity string

	// PayerState (2 chars). Position 212-213.
	PayerState string

	// PayerZip (9 chars). Position 214-222.
	PayerZip string

	// PayerPhone (15 chars). Position 223-237.
	PayerPhone string

	// SequenceNum is the record sequence number within the file.
	// Position 500-507.
	SequenceNum int
}

// PayeeRecord is the FIRE B record. Position-1 == 'B'. One B record
// per recipient. Carries the recipient identifying info and up to 12
// dollar-amount fields indexed by AmountCodes.
type PayeeRecord struct {
	// PaymentYear (4 digits). Position 2-5.
	PaymentYear int

	// CorrectedReturnInd is 'G' (one-step correction), 'C' (two-step
	// second), or blank. Position 6.
	CorrectedReturnInd string

	// NameControl (4 chars). Position 7-10.
	NameControl string

	// TypeOfTIN is '1' (EIN), '2' (SSN), '3' (ITIN), '4' (ATIN),
	// blank otherwise. Position 11.
	TypeOfTIN string

	// PayeeTIN (9 digits, no hyphen). Position 12-20.
	PayeeTIN string

	// PayerAccountNum (20 chars). Position 21-40.
	PayerAccountNum string

	// PayerOfficeCode (4 chars). Position 41-44.
	PayerOfficeCode string

	// PaymentAmounts is a map from amount code character ('1'..'9',
	// 'A'..'I') to the dollar amount. Each amount is a 12-position
	// numeric field in cents (no decimal point). Amount-code-to-
	// position mapping per Publication 1220 Part C §1 — '1' is
	// position 55-66, '2' is 67-78, etc.
	PaymentAmounts map[byte]int64

	// ForeignCountryInd is '1' for foreign payee, blank otherwise.
	// Position 247.
	ForeignCountryInd bool

	// PayeeFirstNameLine (40 chars). Position 248-287.
	PayeeFirstNameLine string

	// PayeeSecondNameLine (40 chars). Position 288-327.
	PayeeSecondNameLine string

	// PayeeMailingAddress (40 chars). Position 367-406.
	PayeeMailingAddress string

	// PayeeCity (40 chars). Position 407-446.
	PayeeCity string

	// PayeeState (2 chars). Position 447-448.
	PayeeState string

	// PayeeZip (9 chars). Position 449-457.
	PayeeZip string

	// SequenceNum is the record sequence number. Position 500-507.
	SequenceNum int

	// SecondTINNotice is '2' if the payer received a second TIN
	// notice from the IRS for this payee, blank otherwise. Position
	// 545.
	SecondTINNotice bool

	// FormSpecificFields holds the per-form variable-position fields
	// (e.g., 1099-B's wash-sale loss disallowed, 1099-R's distribution
	// code). Empty for most forms. Positions vary per form per
	// Publication 1220 Part C.
	FormSpecificFields map[int]string

	// StateInfo holds the CFSF state filing block (positions 663-748).
	StateInfo PayeeStateInfo
}

// PayeeStateInfo carries the CFSF state-totals positions on a B
// record. Empty if not participating in CFSF.
type PayeeStateInfo struct {
	// SpecialDataEntries (60 chars). Position 663-722.
	SpecialDataEntries string

	// StateIncomeTaxWithheld (cents, 12 positions). Position 723-734.
	StateIncomeTaxWithheld int64

	// LocalIncomeTaxWithheld (cents, 12 positions). Position 735-746.
	LocalIncomeTaxWithheld int64

	// CombinedFedStateCode (2 chars). Position 747-748.
	CombinedFedStateCode string
}

// EndOfPayerRecord is the FIRE C record. Position-1 == 'C'. Closes an
// A-block. Carries the total payee count for the A-block plus the
// sum of each amount-code field across the block's B records.
type EndOfPayerRecord struct {
	// NumPayees is the count of B records in the closing A-block.
	// Position 2-9.
	NumPayees int

	// AmountTotals is the per-amount-code total summed across all B
	// records in the block. Each total is an 18-position numeric in
	// cents. Position 10-189 (18 codes × 10? actually 18 × 18 = 324
	// per Pub 1220; the adapter renders the layout from the keys).
	AmountTotals map[byte]int64

	// SequenceNum is the record sequence number. Position 500-507.
	SequenceNum int
}

// EndOfFileRecord is the FIRE F record. Position-1 == 'F'. Closes the
// file. Carries the total A-record count and the total B-record count.
type EndOfFileRecord struct {
	// NumPayers is the total count of A records in the file. Position
	// 2-9.
	NumPayers int

	// NumPayees is the total count of B records in the file. Position
	// 22-29.
	NumPayees int

	// SequenceNum is the record sequence number. Position 500-507.
	SequenceNum int
}

// StateTotalsRecord is the FIRE K record. Position-1 == 'K'. One per
// state for files participating in the Combined Federal/State Filing
// program. Carries per-state amount totals across all B records that
// match the state.
type StateTotalsRecord struct {
	// NumPayees is the count of B records in this state's block.
	// Position 2-9.
	NumPayees int

	// AmountTotals is the per-amount-code state total summed across
	// the state's B records.
	AmountTotals map[byte]int64

	// StateIncomeTaxWithheld is the state income tax withheld total
	// (cents). Position 707-724.
	StateIncomeTaxWithheld int64

	// LocalIncomeTaxWithheld is the local income tax withheld total
	// (cents). Position 725-742.
	LocalIncomeTaxWithheld int64

	// CombinedFedStateCode (2 chars). Position 747-748.
	CombinedFedStateCode string

	// SequenceNum is the record sequence number. Position 500-507.
	SequenceNum int
}

// FIREFile is the full submission to FIRE — one T record, one or more
// (A record + B records + C record) groups, optional K records, one F
// record.
type FIREFile struct {
	// Transmitter is the T record at the head of the file.
	Transmitter TransmitterRecord

	// PayerGroups is the list of (A, B*, C) tuples.
	PayerGroups []PayerGroup

	// StateTotals is the optional list of K records (CFSF).
	StateTotals []StateTotalsRecord

	// EndOfFile is the F record. Populated by Marshal if zero on
	// input.
	EndOfFile EndOfFileRecord
}

// PayerGroup is one (A, B*, C) group within a FIRE file — the records
// for a single payer + form-type combination.
type PayerGroup struct {
	// Payer is the A record at the head of the group.
	Payer PayerRecord

	// Payees is the slice of B records for this payer-form.
	Payees []PayeeRecord

	// EndOfPayer is the C record. Populated by Marshal if zero on
	// input.
	EndOfPayer EndOfPayerRecord
}

// Acknowledgment is the parsed FIRE reply after a SubmitFile call.
// FIRE reports back a Filename / FileStatus pair.
type Acknowledgment struct {
	// Filename is the filename FIRE assigned (the upload key).
	Filename string `json:"filename"`

	// Status is one of: "Good", "Bad", "Replaced". Per Publication
	// 1220 Part B §7.
	Status string `json:"status"`

	// SubmittedAt is the timestamp FIRE reports for receipt.
	SubmittedAt time.Time `json:"submitted_at"`

	// Errors are per-record validation errors, if any.
	Errors []string `json:"errors,omitempty"`

	// Messages are informational messages from FIRE.
	Messages []string `json:"messages,omitempty"`
}
