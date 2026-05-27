// EDGAR Form D marshaling, validation, SGML envelope construction,
// and acknowledgment parsing.
//
// Marshal builds the SEC Form D XML primaryDocument shape; the
// SGML wrapper provides the EDGAR submission header that names the
// FORM-TYPE, CIK, CCC, contact, and the embedded DOCUMENT segment.
// Acknowledgment parsing handles the EDGAR reply envelope (XML
// submission-receipt) and the older SGML-style reply for older
// submission endpoints.
//
// Source-of-design: Public-Spec
// Source-ref: https://www.sec.gov/info/edgar/specifications/formdxml.pdf
// Source-ref: https://www.sec.gov/edgar/filer-information/current-edgar-technical-specifications

package edgar

import (
	"encoding/xml"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"
)

// validateFormD enforces the public Form D schema constraints. The
// validation is intentionally permissive of optional sub-blocks but
// strict on the always-required fields: PrimaryIssuer (CIK, name,
// type, jurisdiction, address, phone), at least one related person,
// at least one federal exemption, and at least one signature. The
// amend parameter, when true, additionally requires FileNumber.
func validateFormD(fd *FormDFiling, amend bool) error {
	if fd == nil {
		return errors.New("nil filing")
	}
	if amend && strings.TrimSpace(fd.FileNumber) == "" {
		return errors.New("amendments require FileNumber")
	}
	if fd.PrimaryIssuer.CIK == "" {
		return errors.New("primary issuer CIK is required")
	}
	if fd.PrimaryIssuer.EntityName == "" {
		return errors.New("primary issuer entity name is required")
	}
	if fd.PrimaryIssuer.EntityType == "" {
		return errors.New("primary issuer entity type is required")
	}
	if fd.PrimaryIssuer.JurisdictionOfIncorporation == "" {
		return errors.New("primary issuer jurisdiction is required")
	}
	if fd.PrimaryIssuer.PrimaryAddress.Street1 == "" ||
		fd.PrimaryIssuer.PrimaryAddress.City == "" ||
		fd.PrimaryIssuer.PrimaryAddress.StateOrCountry == "" ||
		fd.PrimaryIssuer.PrimaryAddress.ZipCode == "" {
		return errors.New("primary issuer address is incomplete")
	}
	if fd.PrimaryIssuer.Phone == "" {
		return errors.New("primary issuer phone number is required")
	}
	if len(fd.RelatedPersons) == 0 {
		return errors.New("at least one related person is required")
	}
	for i, rp := range fd.RelatedPersons {
		if rp.FirstName == "" || rp.LastName == "" {
			return fmt.Errorf("related person %d: name is required", i)
		}
		if len(rp.Relationships) == 0 {
			return fmt.Errorf("related person %d: at least one relationship is required", i)
		}
	}
	if fd.IndustryGroup == "" {
		return errors.New("industry group is required")
	}
	if len(fd.FederalExemptions) == 0 {
		return errors.New("at least one federal exemption is required")
	}
	if len(fd.TypesOfSecurities) == 0 {
		return errors.New("at least one type of security is required")
	}
	if !fd.FirstSaleYetToOccur && fd.DateOfFirstSale == nil && !amend {
		return errors.New("date of first sale or FirstSaleYetToOccur is required")
	}
	if len(fd.Signatures) == 0 {
		return errors.New("at least one signature is required")
	}
	for i, sig := range fd.Signatures {
		if sig.IssuerName == "" || sig.SignatureName == "" || sig.NameOfSigner == "" ||
			sig.SignatureTitle == "" || sig.SignatureDate == "" {
			return fmt.Errorf("signature %d is incomplete", i)
		}
		if _, err := time.Parse("2006-01-02", sig.SignatureDate); err != nil {
			return fmt.Errorf("signature %d: invalid date %q (want YYYY-MM-DD): %v", i, sig.SignatureDate, err)
		}
	}
	// 506(c) hard constraint — no non-accredited investors permitted.
	for _, ex := range fd.FederalExemptions {
		if ex == ExemptionRule506c && fd.InvestorCount.NonAccreditedInvested > 0 {
			return errors.New("Rule 506(c) prohibits non-accredited investors")
		}
	}
	// Rule 504 — max $10M aggregate.
	for _, ex := range fd.FederalExemptions {
		if ex == ExemptionRule504 && fd.OfferingSalesAmount.TotalOfferingAmount > 10_000_000 {
			return errors.New("Rule 504 caps total offering at $10,000,000")
		}
	}
	return nil
}

// --- XML marshaling ---

// edgarSubmission is the root of the Form D XML primaryDocument. The
// element name "edgarSubmission" matches the SEC's published Form D
// schema.
type edgarSubmission struct {
	XMLName             xml.Name              `xml:"edgarSubmission"`
	SchemaVersion       string                `xml:"schemaVersion,attr"`
	SubmissionType      SubmissionType        `xml:"submissionType"`
	TestOrLive          string                `xml:"testOrLive"`
	PrimaryIssuer       xmlIssuer             `xml:"primaryIssuer"`
	RelatedIssuersList  *xmlRelatedIssuers    `xml:"relatedIssuersList,omitempty"`
	RelatedPersonsList  xmlRelatedPersonsList `xml:"relatedPersonsList"`
	OfferingData        xmlOfferingData       `xml:"offeringData"`
	IsAmendment         bool                  `xml:"isAmendment"`
	PreviousAccession   string                `xml:"previousAccessionNumber,omitempty"`
}

type xmlIssuer struct {
	CIK                         string  `xml:"cik"`
	EntityName                  string  `xml:"entityName"`
	IssuerAddress               Address `xml:"issuerAddress"`
	IssuerPhoneNumber           string  `xml:"issuerPhoneNumber"`
	JurisdictionOfInc           string  `xml:"jurisdictionOfInc"`
	IssuerPreviousNameList      *xmlPrevNames `xml:"issuerPreviousNameList,omitempty"`
	EdgarPreviousNameList       *xmlPrevNames `xml:"edgarPreviousNameList,omitempty"`
	EntityType                  EntityType `xml:"entityType"`
	YearOfInc                   xmlYearOfInc `xml:"yearOfInc"`
}

type xmlPrevNames struct {
	PreviousName []string `xml:"previousName"`
}

type xmlYearOfInc struct {
	WithinFiveYears bool   `xml:"withinFiveYears,omitempty"`
	OverFiveYears   bool   `xml:"overFiveYears,omitempty"`
	YetToBeFormed   bool   `xml:"yetToBeFormed,omitempty"`
	Value           string `xml:"value,omitempty"`
}

type xmlRelatedIssuers struct {
	RelatedIssuer []xmlIssuer `xml:"relatedIssuer"`
}

type xmlRelatedPersonsList struct {
	RelatedPersonInfo []xmlRelatedPerson `xml:"relatedPersonInfo"`
}

type xmlRelatedPerson struct {
	RelatedPersonName    xmlPersonName    `xml:"relatedPersonName"`
	RelatedPersonAddress Address          `xml:"relatedPersonAddress"`
	RelatedPersonRelationshipList xmlRelationshipList `xml:"relatedPersonRelationshipList"`
	RelationshipClarification string `xml:"relationshipClarification,omitempty"`
}

type xmlPersonName struct {
	FirstName  string `xml:"firstName"`
	MiddleName string `xml:"middleName,omitempty"`
	LastName   string `xml:"lastName"`
}

type xmlRelationshipList struct {
	Relationship []string `xml:"relationship"`
}

type xmlOfferingData struct {
	IndustryGroup                 xmlIndustryGroup            `xml:"industryGroup"`
	IssuerSize                    xmlIssuerSize               `xml:"issuerSize"`
	FederalExemptionsExclusions   xmlExemptions               `xml:"federalExemptionsExclusions"`
	TypeOfFiling                  xmlTypeOfFiling             `xml:"typeOfFiling"`
	DateOfFirstSale               xmlDateOfFirstSale          `xml:"dateOfFirstSale"`
	MoreThanOneYear               bool                        `xml:"moreThanOneYear"`
	TypesOfSecuritiesOffered      xmlTypesOfSecurities        `xml:"typesOfSecuritiesOffered"`
	BusinessCombinationTransaction xmlBusinessCombination     `xml:"businessCombinationTransaction"`
	MinimumInvestmentAccepted     float64                     `xml:"minimumInvestmentAccepted"`
	SalesCompensationList         *xmlSalesCompensationList   `xml:"salesCompensationList,omitempty"`
	OfferingSalesAmounts          xmlOfferingSalesAmounts     `xml:"offeringSalesAmounts"`
	InvestorsInfo                 xmlInvestorsInfo            `xml:"investorsInfo"`
	SalesCommissionsFindersFees   xmlSalesCommissionsFindersFees `xml:"salesCommissionsFindersFeesExpenses"`
	UseOfProceeds                 xmlUseOfProceeds            `xml:"useOfProceeds"`
	SignatureBlock                xmlSignatureBlock           `xml:"signatureBlock"`
}

type xmlIndustryGroup struct {
	IndustryGroupType         IndustryGroup `xml:"industryGroupType"`
	InvestmentFundType        string        `xml:"investmentFundType,omitempty"`
	Is40ActInvestmentCompany  bool          `xml:"is40Act,omitempty"`
}

type xmlIssuerSize struct {
	RevenueRange     string `xml:"revenueRange,omitempty"`
	AggregateNAVRange string `xml:"aggregateNetAssetValueRange,omitempty"`
}

type xmlExemptions struct {
	Item06b []FederalExemption `xml:"item06b"`
}

type xmlTypeOfFiling struct {
	NewFiling   xmlNewFiling   `xml:"newFiling"`
	Amendment   xmlAmendment   `xml:"amendment"`
}

type xmlNewFiling struct {
	IsNewFiling bool `xml:"newFiling"`
}

type xmlAmendment struct {
	IsAmendment       bool   `xml:"isAmendment"`
	PreviousAccession string `xml:"previousAccessionNumber,omitempty"`
}

type xmlDateOfFirstSale struct {
	Value      string `xml:"value,omitempty"`
	YetToOccur bool   `xml:"yetToOccur,omitempty"`
}

type xmlTypesOfSecurities struct {
	IsEquityType       bool `xml:"isEquityType,omitempty"`
	IsDebtType         bool `xml:"isDebtType,omitempty"`
	IsOptionToAcquireType bool `xml:"isOptionToAcquireType,omitempty"`
	IsSecurityToBeAcquiredType bool `xml:"isSecurityToBeAcquiredType,omitempty"`
	IsPooledInvestmentFundType bool `xml:"isPooledInvestmentFundType,omitempty"`
	IsTenantInCommonType bool `xml:"isTenantInCommonType,omitempty"`
	IsMineralPropertyType bool `xml:"isMineralPropertyType,omitempty"`
	IsOtherType         bool   `xml:"isOtherType,omitempty"`
	DescriptionOfOther  string `xml:"descriptionOfOther,omitempty"`
}

type xmlBusinessCombination struct {
	IsBusinessCombination bool   `xml:"isBusinessCombination"`
	Clarification         string `xml:"clarificationOfResponse,omitempty"`
}

type xmlSalesCompensationList struct {
	RecipientList []xmlRecipient `xml:"recipientList"`
}

type xmlRecipient struct {
	RecipientName    string  `xml:"recipientName"`
	RecipientCRDNumber string `xml:"recipientCRDNumber,omitempty"`
	Address          Address `xml:"recipientAddress"`
	StatesOfSolicitation []string `xml:"statesOfSolicitationList>state,omitempty"`
}

type xmlOfferingSalesAmounts struct {
	TotalOfferingAmount      string  `xml:"totalOfferingAmount"`
	TotalAmountSold          float64 `xml:"totalAmountSold"`
	TotalRemaining           string  `xml:"totalRemaining"`
	Clarification            string  `xml:"clarificationOfResponse,omitempty"`
}

type xmlInvestorsInfo struct {
	HasNonAccreditedInvestors bool `xml:"hasNonAccreditedInvestors"`
	TotalNumberAlreadyInvested int `xml:"totalNumberAlreadyInvested"`
	NonAccreditedInvestorsAlreadyInvested int `xml:"nonAccreditedInvestorsAlreadyInvested,omitempty"`
}

type xmlSalesCommissionsFindersFees struct {
	SalesCommissions          float64 `xml:"salesCommissionsAmount"`
	SalesCommissionsEstimate  bool    `xml:"salesCommissionsEstimate"`
	FindersFees               float64 `xml:"findersFeesAmount"`
	FindersFeesEstimate       bool    `xml:"findersFeesEstimate"`
}

type xmlUseOfProceeds struct {
	GrossProceedsUsed         float64 `xml:"grossProceedsUsed"`
	GrossProceedsUsedEstimate bool    `xml:"grossProceedsUsedEstimate"`
	Clarification             string  `xml:"clarificationOfResponse,omitempty"`
}

type xmlSignatureBlock struct {
	AuthorizedRepresentative bool        `xml:"authorizedRepresentative"`
	Signature                []Signature `xml:"signature"`
}

// marshalFormD renders a FormDFiling into the SEC Form D XML
// primaryDocument shape. Returns the bytes to be wrapped in the SGML
// envelope.
func marshalFormD(fd *FormDFiling) ([]byte, error) {
	es := edgarSubmission{
		SchemaVersion: "X0708",
		SubmissionType: fd.SubmissionType,
		TestOrLive:     "LIVE",
		IsAmendment:    fd.IsAmendment,
		PreviousAccession: fd.FileNumber,
	}
	if es.IsAmendment {
		es.TestOrLive = "LIVE"
	}
	es.PrimaryIssuer = toXMLIssuer(fd.PrimaryIssuer)
	if len(fd.RelatedIssuers) > 0 {
		var rel []xmlIssuer
		for _, ri := range fd.RelatedIssuers {
			rel = append(rel, toXMLIssuer(ri))
		}
		es.RelatedIssuersList = &xmlRelatedIssuers{RelatedIssuer: rel}
	}

	var rps []xmlRelatedPerson
	for _, rp := range fd.RelatedPersons {
		rps = append(rps, xmlRelatedPerson{
			RelatedPersonName: xmlPersonName{
				FirstName: rp.FirstName, MiddleName: rp.MiddleName, LastName: rp.LastName,
			},
			RelatedPersonAddress: rp.Address,
			RelatedPersonRelationshipList: xmlRelationshipList{Relationship: rp.Relationships},
			RelationshipClarification:     rp.Clarification,
		})
	}
	es.RelatedPersonsList = xmlRelatedPersonsList{RelatedPersonInfo: rps}

	es.OfferingData = xmlOfferingData{
		IndustryGroup: xmlIndustryGroup{IndustryGroupType: fd.IndustryGroup},
		IssuerSize: xmlIssuerSize{
			RevenueRange:      fd.IssuerRevenueRange,
			AggregateNAVRange: fd.AggregateNetAssetValueRange,
		},
		FederalExemptionsExclusions: xmlExemptions{Item06b: fd.FederalExemptions},
		TypeOfFiling: xmlTypeOfFiling{
			NewFiling: xmlNewFiling{IsNewFiling: fd.IsNewFiling},
			Amendment: xmlAmendment{IsAmendment: fd.IsAmendment, PreviousAccession: fd.FileNumber},
		},
		DateOfFirstSale: buildDateOfFirstSale(fd),
		MoreThanOneYear: fd.DurationMoreThanOneYear,
		TypesOfSecuritiesOffered: buildTypesOfSecurities(fd.TypesOfSecurities),
		BusinessCombinationTransaction: xmlBusinessCombination{
			IsBusinessCombination: fd.IsBusinessCombinationTransaction,
		},
		MinimumInvestmentAccepted: fd.MinimumInvestmentAccepted,
		OfferingSalesAmounts: buildOfferingSalesAmounts(fd.OfferingSalesAmount),
		InvestorsInfo: xmlInvestorsInfo{
			HasNonAccreditedInvestors:             fd.InvestorCount.NonAccreditedInvested > 0,
			TotalNumberAlreadyInvested:            fd.InvestorCount.TotalAlreadyInvested,
			NonAccreditedInvestorsAlreadyInvested: fd.InvestorCount.NonAccreditedInvested,
		},
		SalesCommissionsFindersFees: xmlSalesCommissionsFindersFees{
			SalesCommissions:         fd.SalesCommissions.SalesCommissions,
			SalesCommissionsEstimate: fd.SalesCommissions.SalesCommissionsEstimate,
			FindersFees:              fd.SalesCommissions.FindersFees,
			FindersFeesEstimate:      fd.SalesCommissions.FindersFeesEstimate,
		},
		UseOfProceeds: xmlUseOfProceeds{
			GrossProceedsUsed:         fd.UseOfProceeds.GrossProceedsToIssuer,
			GrossProceedsUsedEstimate: fd.UseOfProceeds.GrossProceedsEstimate,
			Clarification:             fd.UseOfProceeds.ClarificationOfResponse,
		},
		SignatureBlock: xmlSignatureBlock{
			AuthorizedRepresentative: true,
			Signature:                fd.Signatures,
		},
	}

	out, err := xml.MarshalIndent(es, "", "  ")
	if err != nil {
		return nil, err
	}
	prolog := []byte(`<?xml version="1.0" encoding="UTF-8"?>` + "\n")
	return append(prolog, out...), nil
}

func toXMLIssuer(i Issuer) xmlIssuer {
	out := xmlIssuer{
		CIK:                  normalizeCIK(i.CIK),
		EntityName:           i.EntityName,
		IssuerAddress:        i.PrimaryAddress,
		IssuerPhoneNumber:    i.Phone,
		JurisdictionOfInc:    i.JurisdictionOfIncorporation,
		EntityType:           i.EntityType,
	}
	switch i.YearOfIncorporation {
	case "WithinLastFiveYears":
		out.YearOfInc = xmlYearOfInc{WithinFiveYears: true}
	case "OverFiveYears":
		out.YearOfInc = xmlYearOfInc{OverFiveYears: true}
	case "YetToBeFormed":
		out.YearOfInc = xmlYearOfInc{YetToBeFormed: true}
	default:
		out.YearOfInc = xmlYearOfInc{Value: i.YearOfIncorporation}
	}
	return out
}

func buildDateOfFirstSale(fd *FormDFiling) xmlDateOfFirstSale {
	if fd.FirstSaleYetToOccur {
		return xmlDateOfFirstSale{YetToOccur: true}
	}
	if fd.DateOfFirstSale != nil {
		return xmlDateOfFirstSale{Value: fd.DateOfFirstSale.Format("2006-01-02")}
	}
	return xmlDateOfFirstSale{}
}

func buildTypesOfSecurities(types []string) xmlTypesOfSecurities {
	out := xmlTypesOfSecurities{}
	for _, t := range types {
		switch t {
		case "Equity":
			out.IsEquityType = true
		case "Debt":
			out.IsDebtType = true
		case "Option, Warrant or Other Right to Acquire Another Security":
			out.IsOptionToAcquireType = true
		case "Security to be Acquired Upon Exercise of Option, Warrant or Other Right to Acquire Security":
			out.IsSecurityToBeAcquiredType = true
		case "Pooled Investment Fund Interests":
			out.IsPooledInvestmentFundType = true
		case "Tenant-in-Common Securities":
			out.IsTenantInCommonType = true
		case "Mineral Property Securities":
			out.IsMineralPropertyType = true
		case "Other":
			out.IsOtherType = true
		default:
			out.IsOtherType = true
			out.DescriptionOfOther = t
		}
	}
	return out
}

func buildOfferingSalesAmounts(osa OfferingSalesAmount) xmlOfferingSalesAmounts {
	out := xmlOfferingSalesAmounts{
		TotalAmountSold: osa.TotalAmountSold,
	}
	if osa.IsIndefinite {
		out.TotalOfferingAmount = "Indefinite"
		out.TotalRemaining = "Indefinite"
		return out
	}
	out.TotalOfferingAmount = strconv.FormatFloat(osa.TotalOfferingAmount, 'f', 2, 64)
	remaining := osa.TotalRemaining
	if remaining == 0 {
		remaining = osa.TotalOfferingAmount - osa.TotalAmountSold
	}
	if remaining < 0 {
		remaining = 0
	}
	out.TotalRemaining = strconv.FormatFloat(remaining, 'f', 2, 64)
	return out
}

// --- SGML submission envelope ---

// buildSGMLSubmission wraps the Form D XML payload in the EDGAR
// SUBMISSION SGML header. The envelope structure follows EDGAR Filer
// Manual Volume II §3 — opening <SUBMISSION> tag, then header fields
// (TYPE, CIK, CCC, SROS, FILER, NOTIFY-INTERNET, CONTACT, PERIOD,
// RETURN-COPY), then one <DOCUMENT> block per attached file, then
// closing </SUBMISSION>.
func buildSGMLSubmission(cfg Config, fd *FormDFiling, primaryXML []byte) []byte {
	var b strings.Builder
	b.WriteString("<SUBMISSION>\n")
	fmt.Fprintf(&b, "<TYPE>%s\n", fd.SubmissionType)
	fmt.Fprintf(&b, "<CIK>%s\n", cfg.CIK)
	fmt.Fprintf(&b, "<CCC>%s\n", cfg.CCC)
	if fd.FileNumber != "" {
		fmt.Fprintf(&b, "<SROS>NONE\n")
		fmt.Fprintf(&b, "<FILE-NUMBER>%s\n", fd.FileNumber)
	}
	fmt.Fprintf(&b, "<CONFIRMING-COPY>NO\n")
	if cfg.ContactEmail != "" {
		fmt.Fprintf(&b, "<NOTIFY-INTERNET>%s\n", cfg.ContactEmail)
		fmt.Fprintf(&b, "<CONTACT>\n")
		fmt.Fprintf(&b, "<NAME>%s\n", fd.PrimaryIssuer.EntityName)
		fmt.Fprintf(&b, "<EMAIL>%s\n", cfg.ContactEmail)
		fmt.Fprintf(&b, "</CONTACT>\n")
	}
	fmt.Fprintf(&b, "<FILER>\n")
	fmt.Fprintf(&b, "<CIK>%s\n", cfg.CIK)
	fmt.Fprintf(&b, "<CCC>%s\n", cfg.CCC)
	fmt.Fprintf(&b, "</FILER>\n")

	// DOCUMENT body — TYPE D, primary_doc.xml.
	fmt.Fprintf(&b, "<DOCUMENT>\n")
	fmt.Fprintf(&b, "<TYPE>%s\n", fd.SubmissionType)
	fmt.Fprintf(&b, "<SEQUENCE>1\n")
	fmt.Fprintf(&b, "<FILENAME>primary_doc.xml\n")
	fmt.Fprintf(&b, "<TEXT>\n")
	b.Write(primaryXML)
	b.WriteString("\n</TEXT>\n")
	b.WriteString("</DOCUMENT>\n")

	b.WriteString("</SUBMISSION>\n")
	return []byte(b.String())
}

// --- Acknowledgment / status parsing ---

// edgarAck is the XML acknowledgment EDGAR returns on a successful
// submission. The shape matches the canonical SubmissionReceipt
// returned by the EDGARLink Online endpoint.
type edgarAck struct {
	XMLName         xml.Name `xml:"SubmissionReceipt"`
	SubmissionID    string   `xml:"submissionId"`
	AccessionNumber string   `xml:"accessionNumber"`
	Status          string   `xml:"status"`
	FileNumber      string   `xml:"fileNumber"`
	ReceivedAt      string   `xml:"receivedAt"`
	Messages        []string `xml:"messages>message"`
}

// edgarStatus is the XML reply from the status endpoint.
type edgarStatus struct {
	XMLName         xml.Name `xml:"FilingStatus"`
	AccessionNumber string   `xml:"accessionNumber"`
	Status          string   `xml:"status"`
	FileNumber      string   `xml:"fileNumber"`
	AcceptedAt      string   `xml:"acceptedAt"`
	Messages        []string `xml:"messages>message"`
}

func parseAcknowledgment(body []byte) (*Acknowledgment, error) {
	var ack edgarAck
	if err := xml.Unmarshal(body, &ack); err != nil {
		return nil, fmt.Errorf("decode submission receipt: %w; body=%q", err, truncate(body, 256))
	}
	out := &Acknowledgment{
		AccessionNumber: ack.AccessionNumber,
		SubmissionID:    ack.SubmissionID,
		Status:          strings.ToUpper(ack.Status),
		FileNumber:      ack.FileNumber,
		Messages:        ack.Messages,
	}
	if ack.ReceivedAt != "" {
		if t, err := parseEDGARTime(ack.ReceivedAt); err == nil {
			out.ReceivedAt = t
		}
	}
	if out.Status == "" {
		out.Status = "RECEIVED"
	}
	return out, nil
}

func parseStatusReply(body []byte) (*FilingStatus, error) {
	var st edgarStatus
	if err := xml.Unmarshal(body, &st); err != nil {
		return nil, fmt.Errorf("decode filing status: %w; body=%q", err, truncate(body, 256))
	}
	out := &FilingStatus{
		AccessionNumber: st.AccessionNumber,
		Status:          strings.ToUpper(st.Status),
		FileNumber:      st.FileNumber,
		Messages:        st.Messages,
	}
	if st.AcceptedAt != "" {
		if t, err := parseEDGARTime(st.AcceptedAt); err == nil {
			out.AcceptedAt = t
		}
	}
	return out, nil
}

// parseEDGARTime accepts RFC3339, RFC1123, or the EDGAR-native
// "2006-01-02 15:04:05" form.
func parseEDGARTime(s string) (time.Time, error) {
	for _, layout := range []string{time.RFC3339, time.RFC1123, "2006-01-02 15:04:05", "2006-01-02T15:04:05"} {
		if t, err := time.Parse(layout, s); err == nil {
			return t, nil
		}
	}
	return time.Time{}, fmt.Errorf("unrecognized time format: %q", s)
}

func truncate(b []byte, n int) string {
	if len(b) <= n {
		return string(b)
	}
	return string(b[:n]) + "..."
}
