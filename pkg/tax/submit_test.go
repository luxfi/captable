package tax

import (
	"context"
	"errors"
	"testing"

	"github.com/luxfi/captable/pkg/tax/fire"
	"github.com/luxfi/captable/pkg/tax/iris"
)

// --- stubs ---

type stubIRIS struct {
	lastFS *iris.FormSubmission
	ack    *iris.Acknowledgment
	err    error
}

func (s *stubIRIS) SubmitForm(_ context.Context, fs *iris.FormSubmission) (*iris.Acknowledgment, error) {
	s.lastFS = fs
	if s.err != nil {
		return nil, s.err
	}
	if s.ack != nil {
		return s.ack, nil
	}
	return &iris.Acknowledgment{ReceiptID: "ABC12-TEST-1", Status: "Received"}, nil
}

type stubFIRE struct {
	lastFile *fire.FIREFile
	ack      *fire.Acknowledgment
	err      error
}

func (s *stubFIRE) SubmitFile(_ context.Context, f *fire.FIREFile) (*fire.Acknowledgment, error) {
	s.lastFile = f
	if s.err != nil {
		return nil, s.err
	}
	if s.ack != nil {
		return s.ack, nil
	}
	return &fire.Acknowledgment{Filename: "fire-test.txt", Status: "Good"}, nil
}

// --- SubmitForm (1099-DIV) ---

func TestSubmitForm_RoutesToIRIS_TY2024(t *testing.T) {
	si := &stubIRIS{}
	res, err := SubmitForm(context.Background(), &Form1099DIV{
		TaxYear:           2025,
		PayerName:         "ACME",
		PayerEIN:          "987654321",
		RecipientName:     "DOE JANE",
		RecipientTIN:      "111-22-3333",
		OrdinaryDividends: 5000.00,
		FederalTaxWithheld: 750.00,
	}, SubmissionOptions{IRIS: si})
	if err != nil {
		t.Fatalf("SubmitForm: %v", err)
	}
	if res.Transport != "iris" {
		t.Fatalf("Transport = %q, want iris", res.Transport)
	}
	if res.ReceiptID != "ABC12-TEST-1" {
		t.Fatalf("ReceiptID = %q", res.ReceiptID)
	}
	if si.lastFS.FormType != iris.Form1099DIV {
		t.Fatalf("FormType = %q", si.lastFS.FormType)
	}
	if si.lastFS.Payees[0].Payee.TIN != "111223333" {
		t.Fatalf("TIN = %q (should be hyphen-stripped)", si.lastFS.Payees[0].Payee.TIN)
	}
	if si.lastFS.Transmitter.EIN != "987654321" {
		t.Fatalf("Transmitter EIN = %q", si.lastFS.Transmitter.EIN)
	}
}

func TestSubmitForm_RoutesToFIRE_LegacyYear(t *testing.T) {
	sf := &stubFIRE{}
	res, err := SubmitForm(context.Background(), &Form1099DIV{
		TaxYear:           2023, // pre-IRIS-mandatory
		PayerName:         "ACME",
		PayerEIN:          "987654321",
		RecipientName:     "DOE JANE",
		RecipientTIN:      "111-22-3333",
		OrdinaryDividends: 5000.00,
	}, SubmissionOptions{FIRE: sf, FIREEnv: fire.EnvProduction})
	if err != nil {
		t.Fatalf("SubmitForm: %v", err)
	}
	if res.Transport != "fire" {
		t.Fatalf("Transport = %q, want fire", res.Transport)
	}
	if sf.lastFile.PayerGroups[0].Payer.TypeOfReturn != fire.FormCode1099DIV {
		t.Fatalf("Form code = %q", sf.lastFile.PayerGroups[0].Payer.TypeOfReturn)
	}
}

func TestSubmitForm_ForceLegacyFIRE(t *testing.T) {
	sf := &stubFIRE{}
	res, err := SubmitForm(context.Background(), &Form1099DIV{
		TaxYear:   2025, // post-IRIS-mandatory but forced to FIRE
		PayerName: "ACME",
		PayerEIN:  "987654321",
		RecipientName: "X",
		RecipientTIN:  "111223333",
	}, SubmissionOptions{
		FIRE:            sf,
		ForceLegacyFIRE: true,
		FIREEnv:         fire.EnvProduction,
	})
	if err != nil {
		t.Fatalf("SubmitForm: %v", err)
	}
	if res.Transport != "fire" {
		t.Fatalf("Transport = %q, want fire", res.Transport)
	}
}

func TestSubmitForm_NoTransport(t *testing.T) {
	_, err := SubmitForm(context.Background(), &Form1099DIV{TaxYear: 2025}, SubmissionOptions{})
	if !errors.Is(err, ErrNoTransportConfigured) {
		t.Fatalf("err = %v, want ErrNoTransportConfigured", err)
	}
}

func TestSubmitForm_NoTransport_Legacy(t *testing.T) {
	_, err := SubmitForm(context.Background(), &Form1099DIV{TaxYear: 2022}, SubmissionOptions{})
	if !errors.Is(err, ErrNoTransportConfigured) {
		t.Fatalf("err = %v, want ErrNoTransportConfigured", err)
	}
}

func TestSubmitForm_NilForm(t *testing.T) {
	_, err := SubmitForm(context.Background(), nil, SubmissionOptions{IRIS: &stubIRIS{}})
	if err == nil {
		t.Fatal("expected error for nil form")
	}
}

func TestSubmitForm_PropagatesError(t *testing.T) {
	si := &stubIRIS{err: errors.New("network down")}
	_, err := SubmitForm(context.Background(), &Form1099DIV{
		TaxYear:   2025,
		PayerName: "x", PayerEIN: "987654321",
		RecipientName: "y", RecipientTIN: "111223333",
	}, SubmissionOptions{IRIS: si})
	if err == nil || err.Error() != "network down" {
		t.Fatalf("err = %v, want network down", err)
	}
}

// --- SubmitForm1099B ---

func TestSubmitForm1099B_RoutesToIRIS(t *testing.T) {
	si := &stubIRIS{}
	res, err := SubmitForm1099B(context.Background(), &Form1099B{
		TaxYear:           2025,
		PayerName:         "ACME",
		PayerEIN:          "987654321",
		RecipientName:     "DOE JANE",
		RecipientTIN:      "111-22-3333",
		Description:       "100 SHS ACME CORP",
		DateAcquired:      "2024-01-15",
		DateSold:          "2025-06-15",
		Proceeds:          15000.00,
		CostBasis:         10000.00,
		ShortTermLongTerm: "long",
	}, SubmissionOptions{IRIS: si})
	if err != nil {
		t.Fatalf("SubmitForm1099B: %v", err)
	}
	if res.Transport != "iris" {
		t.Fatalf("Transport = %q", res.Transport)
	}
	d := si.lastFS.Payees[0].Data.(*iris.Form1099BData)
	if d.ShortTermLongTerm != "L" {
		t.Fatalf("ShortTermLongTerm = %q, want L", d.ShortTermLongTerm)
	}
}

func TestSubmitForm1099B_RoutesToFIRE_Legacy(t *testing.T) {
	sf := &stubFIRE{}
	res, err := SubmitForm1099B(context.Background(), &Form1099B{
		TaxYear: 2022, PayerName: "x", PayerEIN: "987654321",
		RecipientName: "y", RecipientTIN: "111223333",
		Proceeds: 1000, CostBasis: 800,
	}, SubmissionOptions{FIRE: sf})
	if err != nil {
		t.Fatalf("SubmitForm1099B: %v", err)
	}
	if res.Transport != "fire" {
		t.Fatalf("Transport = %q", res.Transport)
	}
	if sf.lastFile.PayerGroups[0].Payer.TypeOfReturn != fire.FormCode1099B {
		t.Fatalf("FormCode = %q", sf.lastFile.PayerGroups[0].Payer.TypeOfReturn)
	}
}

func TestStripTIN(t *testing.T) {
	cases := []struct{ in, want string }{
		{"111-22-3333", "111223333"},
		{"111 22 3333", "111223333"},
		{"111223333", "111223333"},
		{"", ""},
	}
	for _, tc := range cases {
		if got := stripTIN(tc.in); got != tc.want {
			t.Fatalf("stripTIN(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}

func TestDollarsToCents(t *testing.T) {
	cases := []struct {
		in   float64
		want int64
	}{
		{0, 0},
		{1.00, 100},
		{1.25, 125},
		{9999.99, 999999},
		{-50.50, -5050},
	}
	for _, tc := range cases {
		if got := dollarsToCents(tc.in); got != tc.want {
			t.Fatalf("dollarsToCents(%v) = %d, want %d", tc.in, got, tc.want)
		}
	}
}
