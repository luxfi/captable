// Conformance suite for the SEC EDGAR Form D filing adapter.
//
// Drives every method of the Filer against httptest.Server fixtures
// and asserts the wire-level behavior promised by the Form D XML
// schema and the EDGAR Filer Manual:
//
//   - validateFormD enforces every must-have field
//   - marshalFormD emits a well-formed Form D primaryDocument
//   - buildSGMLSubmission wraps the XML in the EDGAR SGML envelope
//     with TYPE / CIK / CCC / CONTACT / DOCUMENT segments
//   - FileFormD posts multipart/form-data with the SGML submission and
//     primary_doc.xml part; happy path returns a parsed Acknowledgment
//   - 429 + Retry-After honored; exponential backoff; sustained 429 →
//     ErrRateLimited
//   - Non-retryable 4xx → *APIError
//   - REJECTED status → ErrSubmissionRejected
//   - AmendFormD requires FileNumber and emits the amendment marker
//   - GetFilingStatus returns ErrAccessionNotFound on 404
//   - normalizeCIK left-pads numeric input to ten digits
//
// No production HTTP traffic. All fixtures are httptest.Server.
//
// Source-of-design: Public-Spec
// Source-ref: https://www.sec.gov/info/edgar/specifications/formdxml.pdf
// Source-ref: https://www.sec.gov/edgar/filer-information/current-edgar-technical-specifications

package edgar

import (
	"bytes"
	"context"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"mime"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

// --- helpers ---

func newTestFiler(t *testing.T, srv *httptest.Server) *Filer {
	t.Helper()
	return NewFiler(Config{
		BaseURL:        srv.URL,
		CIK:            "1234567",
		CCC:            "Aa1!Bb2@",
		ContactEmail:   "filings@osage.group",
		UserAgent:      "Lux Captable test@luxfi.io",
		MaxRetries:     2,
		RetryBaseDelay: time.Millisecond,
		RetryMaxDelay:  10 * time.Millisecond,
		HTTPClient:     srv.Client(),
	})
}

func sampleFormD() *FormDFiling {
	when := time.Date(2026, 5, 1, 0, 0, 0, 0, time.UTC)
	return &FormDFiling{
		SubmissionType: SubmissionFormD,
		IsNewFiling:    true,
		PrimaryIssuer: Issuer{
			CIK:                         "1234567",
			EntityName:                  "Osage Group LLC",
			EntityType:                  EntityLLC,
			YearOfIncorporation:         "2022",
			JurisdictionOfIncorporation: "OK",
			PrimaryAddress: Address{
				Street1:        "1500 S Utica Ave Ste 400",
				City:           "Tulsa",
				StateOrCountry: "OK",
				ZipCode:        "74104",
			},
			Phone: "+19137779708",
		},
		RelatedPersons: []RelatedPerson{{
			FirstName: "H.", LastName: "Dupont",
			Address: Address{
				Street1: "1500 S Utica Ave Ste 400", City: "Tulsa",
				StateOrCountry: "OK", ZipCode: "74104",
			},
			Relationships: []string{"Executive Officer", "Director"},
			Clarification: "Chief Executive Officer & Chairman of the Board",
		}},
		IndustryGroup:     IndustryTechnology,
		IssuerRevenueRange: "Decline to Disclose",
		FederalExemptions: []FederalExemption{ExemptionRule506b},
		TypesOfSecurities: []string{"Equity"},
		DateOfFirstSale:   &when,
		OfferingSalesAmount: OfferingSalesAmount{
			TotalOfferingAmount: 5_000_000,
			TotalAmountSold:     1_000_000,
		},
		InvestorCount: InvestorCount{
			TotalAlreadyInvested:  10,
			NonAccreditedInvested: 0,
		},
		MinimumInvestmentAccepted: 25_000,
		Signatures: []Signature{{
			IssuerName:     "Osage Group LLC",
			SignatureName:  "H. Dupont",
			NameOfSigner:   "H. Dupont",
			SignatureTitle: "Chief Executive Officer",
			SignatureDate:  "2026-05-15",
		}},
	}
}

// writeSubmissionReceipt writes a canonical SubmissionReceipt body.
func writeSubmissionReceipt(t *testing.T, w http.ResponseWriter, accession, status string, messages ...string) {
	t.Helper()
	w.Header().Set("Content-Type", "application/xml")
	type msgs struct {
		Message []string `xml:"message"`
	}
	type receipt struct {
		XMLName         xml.Name `xml:"SubmissionReceipt"`
		SubmissionID    string   `xml:"submissionId"`
		AccessionNumber string   `xml:"accessionNumber"`
		Status          string   `xml:"status"`
		FileNumber      string   `xml:"fileNumber"`
		ReceivedAt      string   `xml:"receivedAt"`
		Messages        msgs     `xml:"messages"`
	}
	r := receipt{
		SubmissionID:    "S-abc-123",
		AccessionNumber: accession,
		Status:          status,
		FileNumber:      "021-987654",
		ReceivedAt:      "2026-05-15T19:00:00Z",
		Messages:        msgs{Message: messages},
	}
	if err := xml.NewEncoder(w).Encode(r); err != nil {
		t.Fatalf("encode receipt: %v", err)
	}
}

// parseMultipartSubmission decodes the multipart body sent by the
// adapter and returns the parsed form fields keyed by name.
func parseMultipartSubmission(t *testing.T, r *http.Request) (fields map[string]string, primaryDoc []byte) {
	t.Helper()
	mediaType, params, err := mime.ParseMediaType(r.Header.Get("Content-Type"))
	if err != nil {
		t.Fatalf("parseMediaType: %v", err)
	}
	if !strings.HasPrefix(mediaType, "multipart/") {
		t.Fatalf("content-type not multipart: %q", mediaType)
	}
	fields = make(map[string]string)
	reader := multipart.NewReader(r.Body, params["boundary"])
	for {
		part, err := reader.NextPart()
		if err == io.EOF {
			break
		}
		if err != nil {
			t.Fatalf("nextPart: %v", err)
		}
		data, err := io.ReadAll(part)
		if err != nil {
			t.Fatalf("read part: %v", err)
		}
		if part.FileName() == "primary_doc.xml" {
			primaryDoc = data
			continue
		}
		fields[part.FormName()] = string(data)
	}
	return fields, primaryDoc
}

// --- validateFormD ---

func TestValidateFormD_OK(t *testing.T) {
	if err := validateFormD(sampleFormD(), false); err != nil {
		t.Fatalf("expected valid, got %v", err)
	}
}

func TestValidateFormD_MissingCIK(t *testing.T) {
	fd := sampleFormD()
	fd.PrimaryIssuer.CIK = ""
	if err := validateFormD(fd, false); err == nil || !strings.Contains(err.Error(), "CIK") {
		t.Fatalf("expected CIK error, got %v", err)
	}
}

func TestValidateFormD_MissingRelatedPersons(t *testing.T) {
	fd := sampleFormD()
	fd.RelatedPersons = nil
	if err := validateFormD(fd, false); err == nil || !strings.Contains(err.Error(), "related person") {
		t.Fatalf("expected related-persons error, got %v", err)
	}
}

func TestValidateFormD_MissingFederalExemption(t *testing.T) {
	fd := sampleFormD()
	fd.FederalExemptions = nil
	if err := validateFormD(fd, false); err == nil || !strings.Contains(err.Error(), "federal exemption") {
		t.Fatalf("expected federal-exemption error, got %v", err)
	}
}

func TestValidateFormD_Rule506cBlocksNonAccredited(t *testing.T) {
	fd := sampleFormD()
	fd.FederalExemptions = []FederalExemption{ExemptionRule506c}
	fd.InvestorCount.NonAccreditedInvested = 1
	if err := validateFormD(fd, false); err == nil || !strings.Contains(err.Error(), "506(c)") {
		t.Fatalf("expected 506(c) violation, got %v", err)
	}
}

func TestValidateFormD_Rule504Caps10M(t *testing.T) {
	fd := sampleFormD()
	fd.FederalExemptions = []FederalExemption{ExemptionRule504}
	fd.OfferingSalesAmount.TotalOfferingAmount = 15_000_000
	if err := validateFormD(fd, false); err == nil || !strings.Contains(err.Error(), "504") {
		t.Fatalf("expected 504 cap violation, got %v", err)
	}
}

func TestValidateFormD_AmendmentRequiresFileNumber(t *testing.T) {
	fd := sampleFormD()
	fd.SubmissionType = SubmissionFormDA
	if err := validateFormD(fd, true); err == nil || !strings.Contains(err.Error(), "FileNumber") {
		t.Fatalf("expected FileNumber error, got %v", err)
	}
}

func TestValidateFormD_BadSignatureDate(t *testing.T) {
	fd := sampleFormD()
	fd.Signatures[0].SignatureDate = "May 15, 2026"
	if err := validateFormD(fd, false); err == nil || !strings.Contains(err.Error(), "date") {
		t.Fatalf("expected date format error, got %v", err)
	}
}

// --- marshalFormD ---

func TestMarshalFormD_WellFormed(t *testing.T) {
	out, err := marshalFormD(sampleFormD())
	if err != nil {
		t.Fatalf("marshalFormD: %v", err)
	}
	if !bytes.HasPrefix(out, []byte(`<?xml version="1.0" encoding="UTF-8"?>`)) {
		t.Fatalf("missing XML prolog: %s", string(out[:60]))
	}
	if !bytes.Contains(out, []byte("<edgarSubmission")) {
		t.Fatalf("missing root element")
	}
	if !bytes.Contains(out, []byte("<submissionType>D</submissionType>")) {
		t.Fatalf("missing submissionType=D")
	}
	if !bytes.Contains(out, []byte("<entityName>Osage Group LLC</entityName>")) {
		t.Fatalf("missing entity name")
	}
	if !bytes.Contains(out, []byte("<isEquityType>true</isEquityType>")) {
		t.Fatalf("missing equity-type marker")
	}
	if !bytes.Contains(out, []byte("<totalOfferingAmount>5000000.00</totalOfferingAmount>")) {
		t.Fatalf("missing totalOfferingAmount")
	}
	if !bytes.Contains(out, []byte("<totalRemaining>4000000.00</totalRemaining>")) {
		t.Fatalf("missing totalRemaining; got %s", out)
	}
	// Round-trip parse.
	var sanity edgarSubmission
	if err := xml.Unmarshal(out, &sanity); err != nil {
		t.Fatalf("round-trip parse: %v", err)
	}
	if sanity.PrimaryIssuer.EntityName != "Osage Group LLC" {
		t.Fatalf("round-trip entity name = %q", sanity.PrimaryIssuer.EntityName)
	}
}

func TestMarshalFormD_Indefinite(t *testing.T) {
	fd := sampleFormD()
	fd.OfferingSalesAmount = OfferingSalesAmount{IsIndefinite: true, TotalAmountSold: 250_000}
	out, err := marshalFormD(fd)
	if err != nil {
		t.Fatalf("marshalFormD: %v", err)
	}
	if !bytes.Contains(out, []byte("<totalOfferingAmount>Indefinite</totalOfferingAmount>")) {
		t.Fatalf("expected Indefinite sentinel; got %s", out)
	}
}

func TestMarshalFormD_FirstSaleYetToOccur(t *testing.T) {
	fd := sampleFormD()
	fd.DateOfFirstSale = nil
	fd.FirstSaleYetToOccur = true
	out, err := marshalFormD(fd)
	if err != nil {
		t.Fatalf("marshalFormD: %v", err)
	}
	if !bytes.Contains(out, []byte("<yetToOccur>true</yetToOccur>")) {
		t.Fatalf("expected yetToOccur=true; got %s", out)
	}
}

// --- buildSGMLSubmission ---

func TestBuildSGML_HeaderAndDocument(t *testing.T) {
	fd := sampleFormD()
	xmlPayload, err := marshalFormD(fd)
	if err != nil {
		t.Fatalf("marshalFormD: %v", err)
	}
	sgml := buildSGMLSubmission(Config{
		CIK: "0001234567", CCC: "Aa1!Bb2@", ContactEmail: "filings@osage.group",
	}, fd, xmlPayload)
	s := string(sgml)
	for _, want := range []string{
		"<SUBMISSION>", "</SUBMISSION>",
		"<TYPE>D", "<CIK>0001234567", "<CCC>Aa1!Bb2@",
		"<NOTIFY-INTERNET>filings@osage.group",
		"<DOCUMENT>", "</DOCUMENT>",
		"<FILENAME>primary_doc.xml", "<TEXT>",
		"<edgarSubmission",
	} {
		if !strings.Contains(s, want) {
			t.Fatalf("SGML missing %q in:\n%s", want, s)
		}
	}
}

func TestBuildSGML_AmendmentCarriesFileNumber(t *testing.T) {
	fd := sampleFormD()
	fd.SubmissionType = SubmissionFormDA
	fd.FileNumber = "021-987654"
	xmlPayload, err := marshalFormD(fd)
	if err != nil {
		t.Fatalf("marshalFormD: %v", err)
	}
	sgml := buildSGMLSubmission(Config{CIK: "0001234567", CCC: "X"}, fd, xmlPayload)
	if !strings.Contains(string(sgml), "<FILE-NUMBER>021-987654") {
		t.Fatalf("missing FILE-NUMBER on amendment: %s", sgml)
	}
}

// --- FileFormD HTTP transport ---

func TestFileFormD_HappyPath(t *testing.T) {
	mux := http.NewServeMux()
	var captured atomic.Value
	mux.HandleFunc("/cgi-bin/edgarsubmit", func(w http.ResponseWriter, r *http.Request) {
		fields, primaryDoc := parseMultipartSubmission(t, r)
		captured.Store(fields)
		if fields["CIK"] != "0001234567" {
			t.Errorf("CIK field = %q", fields["CIK"])
		}
		if fields["CCC"] != "Aa1!Bb2@" {
			t.Errorf("CCC field = %q", fields["CCC"])
		}
		if !strings.Contains(fields["submission"], "<SUBMISSION>") {
			t.Errorf("submission missing SGML envelope")
		}
		if !bytes.Contains(primaryDoc, []byte("<edgarSubmission")) {
			t.Errorf("primary_doc.xml missing edgarSubmission")
		}
		if r.Header.Get("User-Agent") == "" {
			t.Errorf("missing User-Agent")
		}
		writeSubmissionReceipt(t, w, "0001234567-26-000001", "RECEIVED")
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	f := newTestFiler(t, srv)
	ack, err := f.FileFormD(context.Background(), sampleFormD())
	if err != nil {
		t.Fatalf("FileFormD: %v", err)
	}
	if ack.AccessionNumber != "0001234567-26-000001" {
		t.Fatalf("accession = %q", ack.AccessionNumber)
	}
	if ack.Status != "RECEIVED" {
		t.Fatalf("status = %q", ack.Status)
	}
	if ack.SubmissionID != "S-abc-123" {
		t.Fatalf("submissionId = %q", ack.SubmissionID)
	}
}

func TestFileFormD_MissingCIK(t *testing.T) {
	f := NewFiler(Config{BaseURL: "https://example", CCC: "x"})
	_, err := f.FileFormD(context.Background(), sampleFormD())
	if !errors.Is(err, ErrMissingCIK) {
		t.Fatalf("expected ErrMissingCIK, got %v", err)
	}
}

func TestFileFormD_MissingCCC(t *testing.T) {
	f := NewFiler(Config{BaseURL: "https://example", CIK: "1234567"})
	_, err := f.FileFormD(context.Background(), sampleFormD())
	if !errors.Is(err, ErrMissingCCC) {
		t.Fatalf("expected ErrMissingCCC, got %v", err)
	}
}

func TestFileFormD_ValidationError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("should not POST when validation fails")
	}))
	defer srv.Close()
	f := newTestFiler(t, srv)
	fd := sampleFormD()
	fd.PrimaryIssuer.EntityName = ""
	_, err := f.FileFormD(context.Background(), fd)
	if !errors.Is(err, ErrInvalidFormD) {
		t.Fatalf("expected ErrInvalidFormD, got %v", err)
	}
}

func TestFileFormD_RejectedByEDGAR(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/cgi-bin/edgarsubmit", func(w http.ResponseWriter, r *http.Request) {
		writeSubmissionReceipt(t, w, "0001234567-26-000002", "REJECTED", "Issuer name conflicts with EDGAR record")
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()
	f := newTestFiler(t, srv)
	ack, err := f.FileFormD(context.Background(), sampleFormD())
	if !errors.Is(err, ErrSubmissionRejected) {
		t.Fatalf("expected ErrSubmissionRejected, got %v", err)
	}
	if ack == nil || ack.Status != "REJECTED" {
		t.Fatalf("expected REJECTED ack, got %+v", ack)
	}
}

func TestFileFormD_4xxNonRetryable(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/cgi-bin/edgarsubmit", func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, `<error>bad CCC</error>`, http.StatusUnauthorized)
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()
	f := newTestFiler(t, srv)
	_, err := f.FileFormD(context.Background(), sampleFormD())
	var apiErr *APIError
	if !errors.As(err, &apiErr) || apiErr.StatusCode != http.StatusUnauthorized {
		t.Fatalf("expected 401 APIError, got %v", err)
	}
}

func TestFileFormD_429RetriedThenSuccess(t *testing.T) {
	var calls atomic.Int32
	mux := http.NewServeMux()
	mux.HandleFunc("/cgi-bin/edgarsubmit", func(w http.ResponseWriter, r *http.Request) {
		n := calls.Add(1)
		if n == 1 {
			w.Header().Set("Retry-After", "0")
			http.Error(w, "throttled", http.StatusTooManyRequests)
			return
		}
		writeSubmissionReceipt(t, w, "0001234567-26-000003", "RECEIVED")
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()
	f := newTestFiler(t, srv)
	ack, err := f.FileFormD(context.Background(), sampleFormD())
	if err != nil {
		t.Fatalf("FileFormD: %v", err)
	}
	if ack.Status != "RECEIVED" {
		t.Fatalf("status = %q", ack.Status)
	}
	if calls.Load() < 2 {
		t.Fatalf("expected at least 2 calls, got %d", calls.Load())
	}
}

func TestFileFormD_SustainedRateLimit(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/cgi-bin/edgarsubmit", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Retry-After", "0")
		http.Error(w, "throttled", http.StatusTooManyRequests)
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()
	f := newTestFiler(t, srv)
	_, err := f.FileFormD(context.Background(), sampleFormD())
	if !errors.Is(err, ErrRateLimited) {
		t.Fatalf("expected ErrRateLimited, got %v", err)
	}
}

func TestFileFormD_Retries5xx(t *testing.T) {
	var calls atomic.Int32
	mux := http.NewServeMux()
	mux.HandleFunc("/cgi-bin/edgarsubmit", func(w http.ResponseWriter, r *http.Request) {
		if calls.Add(1) == 1 {
			http.Error(w, "edgar down", http.StatusServiceUnavailable)
			return
		}
		writeSubmissionReceipt(t, w, "0001234567-26-000004", "RECEIVED")
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()
	f := newTestFiler(t, srv)
	if _, err := f.FileFormD(context.Background(), sampleFormD()); err != nil {
		t.Fatalf("FileFormD: %v", err)
	}
}

func TestFileFormD_RespectsContextCancellation(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/cgi-bin/edgarsubmit", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Retry-After", "60")
		http.Error(w, "throttled", http.StatusTooManyRequests)
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()
	f := newTestFiler(t, srv)
	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		time.Sleep(5 * time.Millisecond)
		cancel()
	}()
	_, err := f.FileFormD(ctx, sampleFormD())
	if err == nil {
		t.Fatal("expected error")
	}
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("expected ctx Canceled, got %v", err)
	}
}

// --- AmendFormD ---

func TestAmendFormD_HappyPath(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/cgi-bin/edgarsubmit", func(w http.ResponseWriter, r *http.Request) {
		fields, primaryDoc := parseMultipartSubmission(t, r)
		if !strings.Contains(fields["submission"], "<FILE-NUMBER>021-987654") {
			t.Errorf("missing FILE-NUMBER on amendment SGML")
		}
		if !bytes.Contains(primaryDoc, []byte("<submissionType>D/A</submissionType>")) {
			t.Errorf("primaryDoc missing D/A submission type: %s", primaryDoc)
		}
		writeSubmissionReceipt(t, w, "0001234567-26-000005", "RECEIVED")
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()
	f := newTestFiler(t, srv)
	fd := sampleFormD()
	fd.FileNumber = "021-987654"
	ack, err := f.AmendFormD(context.Background(), fd)
	if err != nil {
		t.Fatalf("AmendFormD: %v", err)
	}
	if ack.AccessionNumber != "0001234567-26-000005" {
		t.Fatalf("accession = %q", ack.AccessionNumber)
	}
}

func TestAmendFormD_MissingFileNumber(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("should not POST when validation fails")
	}))
	defer srv.Close()
	f := newTestFiler(t, srv)
	_, err := f.AmendFormD(context.Background(), sampleFormD())
	if !errors.Is(err, ErrInvalidFormD) {
		t.Fatalf("expected ErrInvalidFormD, got %v", err)
	}
}

// --- GetFilingStatus ---

func TestGetFilingStatus_HappyPath(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/cgi-bin/edgarstatus", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("accession") != "0001234567-26-000001" {
			t.Errorf("missing accession query: %s", r.URL.RawQuery)
		}
		w.Header().Set("Content-Type", "application/xml")
		fmt.Fprintln(w, `<FilingStatus>
  <accessionNumber>0001234567-26-000001</accessionNumber>
  <status>ACCEPTED</status>
  <fileNumber>021-987654</fileNumber>
  <acceptedAt>2026-05-15T19:00:30Z</acceptedAt>
</FilingStatus>`)
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()
	f := newTestFiler(t, srv)
	st, err := f.GetFilingStatus(context.Background(), "0001234567-26-000001")
	if err != nil {
		t.Fatalf("GetFilingStatus: %v", err)
	}
	if st.Status != "ACCEPTED" {
		t.Fatalf("status = %q", st.Status)
	}
	if st.FileNumber != "021-987654" {
		t.Fatalf("fileNumber = %q", st.FileNumber)
	}
	if st.AcceptedAt.IsZero() {
		t.Fatalf("acceptedAt zero")
	}
}

func TestGetFilingStatus_NotFound(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/cgi-bin/edgarstatus", func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, `<error>unknown accession</error>`, http.StatusNotFound)
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()
	f := newTestFiler(t, srv)
	_, err := f.GetFilingStatus(context.Background(), "0001234567-26-999999")
	if !errors.Is(err, ErrAccessionNotFound) {
		t.Fatalf("expected ErrAccessionNotFound, got %v", err)
	}
}

func TestGetFilingStatus_MissingAccession(t *testing.T) {
	f := NewFiler(Config{BaseURL: "https://example", CIK: "1234567", CCC: "x"})
	if _, err := f.GetFilingStatus(context.Background(), ""); err == nil {
		t.Fatal("expected error for empty accession")
	}
}

// --- normalizeCIK ---

func TestNormalizeCIK(t *testing.T) {
	cases := map[string]string{
		"":           "",
		"1234567":    "0001234567",
		"0001234567": "0001234567",
		"99":         "0000000099",
		"abc":        "abc",
		" 12345 ":    "0000012345",
	}
	for in, want := range cases {
		if got := normalizeCIK(in); got != want {
			t.Errorf("normalizeCIK(%q) = %q, want %q", in, got, want)
		}
	}
}

// --- Acknowledgment parsing edge cases ---

func TestParseAcknowledgment_DefaultsReceived(t *testing.T) {
	body := []byte(`<SubmissionReceipt><submissionId>S-x</submissionId><accessionNumber>0001234567-26-000007</accessionNumber></SubmissionReceipt>`)
	ack, err := parseAcknowledgment(body)
	if err != nil {
		t.Fatal(err)
	}
	if ack.Status != "RECEIVED" {
		t.Fatalf("status = %q", ack.Status)
	}
}

func TestParseAcknowledgment_BadXML(t *testing.T) {
	if _, err := parseAcknowledgment([]byte("not xml")); err == nil {
		t.Fatal("expected error")
	}
}

// --- parseRetryAfter ---

func TestParseRetryAfter(t *testing.T) {
	if got := parseRetryAfter(""); got != 0 {
		t.Fatalf("empty got %v", got)
	}
	if got := parseRetryAfter("5"); got != 5*time.Second {
		t.Fatalf("5 got %v", got)
	}
	if got := parseRetryAfter("garbage"); got != 0 {
		t.Fatalf("garbage got %v", got)
	}
	future := time.Now().Add(2 * time.Second).UTC().Format(http.TimeFormat)
	if got := parseRetryAfter(future); got <= 0 {
		t.Fatalf("future date got %v", got)
	}
}
