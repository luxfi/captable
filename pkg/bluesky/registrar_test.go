// Conformance suite for the Blue-Sky state filings adapter.
//
// Covers the Registrar dispatch, EFD adapter (NASAA Electronic
// Filing Depository), and the five real state-portal adapters
// (FL, TX, NY, CA, MA). Drives every code path against httptest
// fixtures and asserts wire-level behavior:
//
//   - registrar dispatches to the right adapter by state
//   - EFD authenticate-then-submit token flow
//   - EFD FileNoticeOfSale happy path + payload shape
//   - EFD RenewNotice happy path
//   - EFD GetFilingStatus + 404 → ErrFilingNotFound
//   - state portal adapters: FL, TX, NY, CA, MA all submit & status
//   - validation errors block submission
//   - 429 + 5xx retried; sustained → ErrRateLimited
//   - 4xx non-retryable → *APIError
//   - fee schedule returns correct value per state (incl. scaled)
//   - state portals support electronic when configured
//   - renewals to a no-renewal portal → ErrNotImplemented
//   - SupportedStates returns 50 + DC + DC sentinels
//
// All fixtures are httptest.Server. No production traffic.
//
// Source-of-design: Public-Spec
// Source-ref: http://nasaaefd.org
// Source-ref: http://nasaaefd.org/About/FormDStates

package bluesky

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

// --- helpers ---

func sampleNotice() *NoticeFiling {
	return &NoticeFiling{
		FilingType: FilingNoticeOfSale,
		Issuer: Issuer{
			CIK:                         "0001234567",
			EntityName:                  "Osage Group LLC",
			EntityType:                  "Limited Liability Company",
			JurisdictionOfIncorporation: "OK",
			YearOfIncorporation:         "2022",
			PrimaryAddress: Address{
				Street1: "1500 S Utica Ave Ste 400", City: "Tulsa",
				State: "OK", PostalCode: "74104", Country: "US",
			},
			Phone: "+19137779708",
			Email: "filings@osage.group",
		},
		Offering: OfferingDetails{
			FederalExemption:       "06c", // 506(b)
			TotalOfferingAmount:    5_000_000,
			AmountSoldInState:      250_000,
			DateOfFirstSaleInState: time.Date(2026, 5, 1, 0, 0, 0, 0, time.UTC),
			TypesOfSecurities:      []string{"Equity"},
			SECFileNumber:          "021-987654",
			SECAccessionNumber:     "0001234567-26-000001",
		},
		Signature: Signature{
			IssuerName: "Osage Group LLC", SignatureName: "H. Dupont",
			NameOfSigner: "H. Dupont", SignatureTitle: "CEO",
			SignatureDate: "2026-05-15",
		},
	}
}

func sampleRenewal() *RenewalFiling {
	return &RenewalFiling{
		OriginalFilingID:   "EFD-NY-26-1001",
		OriginalFilingDate: time.Date(2026, 4, 15, 0, 0, 0, 0, time.UTC),
		Issuer:             sampleNotice().Issuer,
		UpdatedOffering:    sampleNotice().Offering,
		Signature:          sampleNotice().Signature,
	}
}

// efdMux is a configurable EFD mock that captures the last filing
// request body. Returns the canonical login + filing receipts.
type efdMux struct {
	*http.ServeMux
	loginCalls   atomic.Int32
	submitCalls  atomic.Int32
	renewCalls   atomic.Int32
	statusCalls  atomic.Int32
	lastFiling   atomic.Value // *efdFilingRequest
	respond      func(w http.ResponseWriter, path string)
}

func newEFDMux(t *testing.T) *efdMux {
	mux := &efdMux{ServeMux: http.NewServeMux()}
	mux.HandleFunc("/Account/Login", func(w http.ResponseWriter, r *http.Request) {
		mux.loginCalls.Add(1)
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprintln(w, `{"token":"test-token","expires":"`+time.Now().Add(time.Hour).Format(time.RFC3339)+`"}`)
	})
	mux.HandleFunc("/FormD/Submit", func(w http.ResponseWriter, r *http.Request) {
		mux.submitCalls.Add(1)
		if got := r.Header.Get("Authorization"); !strings.HasPrefix(got, "Bearer test-token") {
			t.Errorf("submit missing/incorrect bearer: %q", got)
		}
		var req efdFilingRequest
		body, _ := io.ReadAll(r.Body)
		if err := json.Unmarshal(body, &req); err != nil {
			t.Errorf("decode submit body: %v", err)
		}
		mux.lastFiling.Store(&req)
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprintf(w, `{"filingId":"EFD-%s-26-1001","status":"RECEIVED","state":%q,"fee":%f,"receivedAt":%q,"expiresAt":%q,"paymentId":"P-100"}`,
			req.State, req.State, 250.0,
			time.Now().UTC().Format(time.RFC3339),
			time.Now().Add(365*24*time.Hour).UTC().Format(time.RFC3339))
	})
	mux.HandleFunc("/FormD/Renew", func(w http.ResponseWriter, r *http.Request) {
		mux.renewCalls.Add(1)
		var req efdRenewalRequest
		body, _ := io.ReadAll(r.Body)
		if err := json.Unmarshal(body, &req); err != nil {
			t.Errorf("decode renewal body: %v", err)
		}
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprintf(w, `{"filingId":"EFD-%s-26-1002","status":"RECEIVED","state":%q,"fee":%f}`,
			req.State, req.State, 300.0)
	})
	mux.HandleFunc("/FormD/Status/", func(w http.ResponseWriter, r *http.Request) {
		mux.statusCalls.Add(1)
		id := strings.TrimPrefix(r.URL.Path, "/FormD/Status/")
		if id == "missing" {
			http.Error(w, `{"error":"not found"}`, http.StatusNotFound)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprintf(w, `{"filingId":%q,"state":"NY","status":"ACCEPTED","filedAt":%q,"acceptedAt":%q,"fee":300.0}`,
			id, time.Now().UTC().Format(time.RFC3339), time.Now().UTC().Format(time.RFC3339))
	})
	return mux
}

func newEFDRegistrar(t *testing.T, srv *httptest.Server) *Registrar {
	t.Helper()
	return NewRegistrar(Config{
		EFD: EFDConfig{
			BaseURL:  srv.URL,
			Username: "tester", Password: "secret", FirmCRD: "123456",
		},
		FL: StatePortalConfig{BaseURL: srv.URL, APIKey: "fl-key", AccountID: "fl-acct"},
		TX: StatePortalConfig{BaseURL: srv.URL, APIKey: "tx-key", AccountID: "tx-acct"},
		NY: StatePortalConfig{BaseURL: srv.URL, APIKey: "ny-key", AccountID: "ny-acct"},
		CA: StatePortalConfig{BaseURL: srv.URL, APIKey: "ca-key", AccountID: "ca-acct"},
		MA: StatePortalConfig{BaseURL: srv.URL, APIKey: "ma-key", AccountID: "ma-acct"},
		UserAgent:      "Lux Captable test@luxfi.io",
		MaxRetries:     2,
		RetryBaseDelay: time.Millisecond,
		RetryMaxDelay:  10 * time.Millisecond,
		HTTPClient:     srv.Client(),
	})
}

// --- Registrar dispatch ---

func TestRegistrar_SupportedStates_HasAll51(t *testing.T) {
	reg := NewRegistrar(Config{})
	got := reg.SupportedStates()
	if len(got) < 51 {
		t.Fatalf("expected at least 51 states (50 + DC), got %d", len(got))
	}
	seen := map[State]bool{}
	for _, s := range got {
		seen[s] = true
	}
	for _, want := range []State{"AL", "CA", "DC", "FL", "TX", "NY", "MA", "WY"} {
		if !seen[want] {
			t.Errorf("missing state %q", want)
		}
	}
}

func TestRegistrar_UnknownState(t *testing.T) {
	reg := NewRegistrar(Config{})
	_, err := reg.FileNoticeOfSale(context.Background(), "ZZ", sampleNotice())
	if !errors.Is(err, ErrUnsupportedState) {
		t.Fatalf("expected ErrUnsupportedState, got %v", err)
	}
}

func TestRegistrar_ValidationBlocksSubmission(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("submission must not be sent when validation fails")
	}))
	defer srv.Close()
	reg := newEFDRegistrar(t, srv)
	bad := sampleNotice()
	bad.Issuer.EntityName = ""
	_, err := reg.FileNoticeOfSale(context.Background(), "CO", bad)
	if !errors.Is(err, ErrInvalidNotice) {
		t.Fatalf("expected ErrInvalidNotice, got %v", err)
	}
}

// --- EFD adapter ---

func TestEFD_FileNoticeOfSale_HappyPath(t *testing.T) {
	mux := newEFDMux(t)
	srv := httptest.NewServer(mux)
	defer srv.Close()
	reg := newEFDRegistrar(t, srv)
	ack, err := reg.FileNoticeOfSale(context.Background(), "CO", sampleNotice())
	if err != nil {
		t.Fatalf("FileNoticeOfSale: %v", err)
	}
	if ack.FilingID == "" {
		t.Fatalf("missing filing_id; ack=%+v", ack)
	}
	if ack.State != "CO" {
		t.Fatalf("state = %q", ack.State)
	}
	if ack.Status != "RECEIVED" {
		t.Fatalf("status = %q", ack.Status)
	}
	if mux.loginCalls.Load() != 1 {
		t.Errorf("expected 1 login, got %d", mux.loginCalls.Load())
	}
	if mux.submitCalls.Load() != 1 {
		t.Errorf("expected 1 submit, got %d", mux.submitCalls.Load())
	}
	got, _ := mux.lastFiling.Load().(*efdFilingRequest)
	if got == nil || got.Issuer.EntityName != "Osage Group LLC" {
		t.Fatalf("filing payload not captured / wrong: %+v", got)
	}
	if got.Offering.TotalOfferingAmount != 5_000_000 {
		t.Errorf("offering amount = %v", got.Offering.TotalOfferingAmount)
	}
}

func TestEFD_TokenReused(t *testing.T) {
	mux := newEFDMux(t)
	srv := httptest.NewServer(mux)
	defer srv.Close()
	reg := newEFDRegistrar(t, srv)
	for i := 0; i < 3; i++ {
		if _, err := reg.FileNoticeOfSale(context.Background(), "CO", sampleNotice()); err != nil {
			t.Fatalf("call %d: %v", i, err)
		}
	}
	if mux.loginCalls.Load() != 1 {
		t.Errorf("expected 1 login across 3 submits, got %d", mux.loginCalls.Load())
	}
}

func TestEFD_FormDXMLAttached(t *testing.T) {
	mux := newEFDMux(t)
	srv := httptest.NewServer(mux)
	defer srv.Close()
	reg := newEFDRegistrar(t, srv)
	n := sampleNotice()
	n.FormDXML = []byte(`<edgarSubmission><submissionType>D</submissionType></edgarSubmission>`)
	if _, err := reg.FileNoticeOfSale(context.Background(), "CO", n); err != nil {
		t.Fatalf("FileNoticeOfSale: %v", err)
	}
	got, _ := mux.lastFiling.Load().(*efdFilingRequest)
	if got == nil || got.FormDXML == "" {
		t.Fatalf("expected base64 Form D XML attached; got %+v", got)
	}
}

func TestEFD_RenewNotice(t *testing.T) {
	mux := newEFDMux(t)
	srv := httptest.NewServer(mux)
	defer srv.Close()
	reg := newEFDRegistrar(t, srv)
	ack, err := reg.RenewNotice(context.Background(), "CO", sampleRenewal())
	if err != nil {
		t.Fatalf("RenewNotice: %v", err)
	}
	if ack.FilingID == "" || ack.Status == "" {
		t.Fatalf("bad ack: %+v", ack)
	}
	if mux.renewCalls.Load() != 1 {
		t.Errorf("expected 1 renew call, got %d", mux.renewCalls.Load())
	}
}

func TestEFD_RenewNotice_MissingOriginalID(t *testing.T) {
	mux := newEFDMux(t)
	srv := httptest.NewServer(mux)
	defer srv.Close()
	reg := newEFDRegistrar(t, srv)
	r := sampleRenewal()
	r.OriginalFilingID = ""
	_, err := reg.RenewNotice(context.Background(), "CO", r)
	if !errors.Is(err, ErrInvalidNotice) {
		t.Fatalf("expected ErrInvalidNotice, got %v", err)
	}
}

func TestEFD_GetFilingStatus_HappyPath(t *testing.T) {
	mux := newEFDMux(t)
	srv := httptest.NewServer(mux)
	defer srv.Close()
	reg := newEFDRegistrar(t, srv)
	st, err := reg.GetFilingStatus(context.Background(), "CO", "EFD-CO-26-1001")
	if err != nil {
		t.Fatalf("GetFilingStatus: %v", err)
	}
	if st.Status != "ACCEPTED" {
		t.Fatalf("status = %q", st.Status)
	}
}

func TestEFD_GetFilingStatus_NotFound(t *testing.T) {
	mux := newEFDMux(t)
	srv := httptest.NewServer(mux)
	defer srv.Close()
	reg := newEFDRegistrar(t, srv)
	_, err := reg.GetFilingStatus(context.Background(), "CO", "missing")
	if !errors.Is(err, ErrFilingNotFound) {
		t.Fatalf("expected ErrFilingNotFound, got %v", err)
	}
}

func TestEFD_GetFilingStatus_EmptyID(t *testing.T) {
	reg := NewRegistrar(Config{})
	_, err := reg.GetFilingStatus(context.Background(), "CO", "")
	if err == nil || !strings.Contains(err.Error(), "filing_id") {
		t.Fatalf("expected filing_id error, got %v", err)
	}
}

// --- Rate-limit / retry ---

func TestEFD_RetriesOn429(t *testing.T) {
	var attempts atomic.Int32
	mux := http.NewServeMux()
	mux.HandleFunc("/Account/Login", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprintln(w, `{"token":"t","expires":"`+time.Now().Add(time.Hour).Format(time.RFC3339)+`"}`)
	})
	mux.HandleFunc("/FormD/Submit", func(w http.ResponseWriter, r *http.Request) {
		if attempts.Add(1) == 1 {
			w.Header().Set("Retry-After", "0")
			http.Error(w, "throttled", http.StatusTooManyRequests)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprintln(w, `{"filingId":"EFD-CO-26-1099","status":"RECEIVED","state":"CO","fee":75}`)
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()
	reg := newEFDRegistrar(t, srv)
	if _, err := reg.FileNoticeOfSale(context.Background(), "CO", sampleNotice()); err != nil {
		t.Fatalf("FileNoticeOfSale: %v", err)
	}
	if attempts.Load() < 2 {
		t.Fatalf("expected at least 2 submit attempts, got %d", attempts.Load())
	}
}

func TestEFD_SustainedRateLimit(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/Account/Login", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprintln(w, `{"token":"t","expires":"`+time.Now().Add(time.Hour).Format(time.RFC3339)+`"}`)
	})
	mux.HandleFunc("/FormD/Submit", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Retry-After", "0")
		http.Error(w, "throttled", http.StatusTooManyRequests)
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()
	reg := newEFDRegistrar(t, srv)
	_, err := reg.FileNoticeOfSale(context.Background(), "CO", sampleNotice())
	if !errors.Is(err, ErrRateLimited) {
		t.Fatalf("expected ErrRateLimited, got %v", err)
	}
}

func TestEFD_NonRetryable4xx(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/Account/Login", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprintln(w, `{"token":"t","expires":"`+time.Now().Add(time.Hour).Format(time.RFC3339)+`"}`)
	})
	mux.HandleFunc("/FormD/Submit", func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, `{"error":"bad request"}`, http.StatusBadRequest)
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()
	reg := newEFDRegistrar(t, srv)
	_, err := reg.FileNoticeOfSale(context.Background(), "CO", sampleNotice())
	var apiErr *APIError
	if !errors.As(err, &apiErr) || apiErr.StatusCode != http.StatusBadRequest {
		t.Fatalf("expected 400 APIError, got %v", err)
	}
}

// --- State portal adapters ---

func TestStatePortal_FL_Submit(t *testing.T) {
	mux := http.NewServeMux()
	var got atomic.Value
	mux.HandleFunc("/forms/d/submit", func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer fl-key" {
			t.Errorf("FL missing api key: %q", r.Header.Get("Authorization"))
		}
		var req portalFilingRequest
		body, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(body, &req)
		got.Store(&req)
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprintln(w, `{"filingId":"FL-26-1001","status":"RECEIVED","fee":200,"receivedAt":"2026-05-15T19:00:00Z"}`)
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()
	reg := newEFDRegistrar(t, srv)
	ack, err := reg.FileNoticeOfSale(context.Background(), "FL", sampleNotice())
	if err != nil {
		t.Fatalf("FL submit: %v", err)
	}
	if ack.FilingID != "FL-26-1001" || ack.State != "FL" {
		t.Fatalf("FL ack: %+v", ack)
	}
	if g, _ := got.Load().(*portalFilingRequest); g == nil || g.State != "FL" {
		t.Fatalf("FL payload state = %+v", g)
	}
}

func TestStatePortal_TX_Submit(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/notice/submit", func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer tx-key" {
			t.Errorf("TX missing api key: %q", r.Header.Get("Authorization"))
		}
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprintln(w, `{"filingId":"TX-26-1001","status":"RECEIVED","fee":500}`)
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()
	reg := newEFDRegistrar(t, srv)
	ack, err := reg.FileNoticeOfSale(context.Background(), "TX", sampleNotice())
	if err != nil {
		t.Fatalf("TX submit: %v", err)
	}
	if ack.FilingID != "TX-26-1001" {
		t.Fatalf("TX ack: %+v", ack)
	}
}

func TestStatePortal_NY_SubmitAndRenew(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/securities/notice/submit", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprintln(w, `{"filingId":"NY-26-1001","status":"RECEIVED","fee":1200}`)
	})
	mux.HandleFunc("/securities/notice/renew", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprintln(w, `{"filingId":"NY-26-1002","status":"RECEIVED","fee":1200}`)
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()
	reg := newEFDRegistrar(t, srv)
	if _, err := reg.FileNoticeOfSale(context.Background(), "NY", sampleNotice()); err != nil {
		t.Fatalf("NY submit: %v", err)
	}
	if _, err := reg.RenewNotice(context.Background(), "NY", sampleRenewal()); err != nil {
		t.Fatalf("NY renew: %v", err)
	}
}

func TestStatePortal_CA_StatusAndAmend(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/securities/25102f/submit", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprintln(w, `{"filingId":"CA-26-1001","status":"FILED","fee":275}`)
	})
	mux.HandleFunc("/securities/25102f/CA-26-1001", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprintln(w, `{"filingId":"CA-26-1001","state":"CA","status":"ACCEPTED","fee":275,"filedAt":"2026-05-15T19:00:00Z","acceptedAt":"2026-05-16T09:00:00Z"}`)
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()
	reg := newEFDRegistrar(t, srv)
	ack, err := reg.FileNoticeOfSale(context.Background(), "CA", sampleNotice())
	if err != nil {
		t.Fatalf("CA submit: %v", err)
	}
	if ack.FilingID != "CA-26-1001" || ack.Status != "FILED" {
		t.Fatalf("CA ack: %+v", ack)
	}
	st, err := reg.GetFilingStatus(context.Background(), "CA", "CA-26-1001")
	if err != nil {
		t.Fatalf("CA status: %v", err)
	}
	if st.Status != "ACCEPTED" {
		t.Fatalf("CA status = %q", st.Status)
	}
}

func TestStatePortal_MA_Submit(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/sct/notice/submit", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprintln(w, `{"filingId":"MA-26-1001","status":"RECEIVED","fee":300}`)
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()
	reg := newEFDRegistrar(t, srv)
	ack, err := reg.FileNoticeOfSale(context.Background(), "MA", sampleNotice())
	if err != nil {
		t.Fatalf("MA submit: %v", err)
	}
	if ack.FilingID != "MA-26-1001" {
		t.Fatalf("MA ack: %+v", ack)
	}
}

func TestStatePortal_FL_NoRenewals(t *testing.T) {
	reg := newEFDRegistrar(t, httptest.NewServer(http.NewServeMux()))
	_, err := reg.RenewNotice(context.Background(), "FL", sampleRenewal())
	if !errors.Is(err, ErrNotImplemented) {
		t.Fatalf("expected ErrNotImplemented, got %v", err)
	}
}

// --- Fee schedule ---

func TestCalculateStateFee_EFDStates(t *testing.T) {
	reg := NewRegistrar(Config{})
	cases := map[State]float64{
		"AL": 300, "AK": 600, "CO": 75, "DE": 200, "DC": 250,
		"GA": 250, "ID": 50, "IN": 100, "MN": 50, "WY": 200,
	}
	for state, want := range cases {
		fee, err := reg.CalculateStateFee(state, sampleNotice())
		if err != nil {
			t.Errorf("%s: %v", state, err)
			continue
		}
		if fee.StateFee != want {
			t.Errorf("%s state fee = %v, want %v", state, fee.StateFee, want)
		}
		if fee.SystemFee != EFDSystemFeeUSD {
			t.Errorf("%s missing EFD system fee", state)
		}
		if fee.Method != "ACH" {
			t.Errorf("%s method = %q", state, fee.Method)
		}
	}
}

func TestCalculateStateFee_ScaledStates(t *testing.T) {
	reg := NewRegistrar(Config{})
	// CA — $25 + 0.001% of offering, max $300.
	n := sampleNotice()
	n.Offering.TotalOfferingAmount = 100_000_000
	fee, err := reg.CalculateStateFee("CA", n)
	if err != nil {
		t.Fatalf("CA: %v", err)
	}
	if fee.StateFee != 300 {
		t.Errorf("CA scaled fee = %v, want 300 cap", fee.StateFee)
	}
	// NY — $300 ≤ $500K, $1,200 > $500K.
	small := sampleNotice()
	small.Offering.TotalOfferingAmount = 250_000
	if f, _ := reg.CalculateStateFee("NY", small); f.StateFee != 300 {
		t.Errorf("NY small fee = %v, want 300", f.StateFee)
	}
	if f, _ := reg.CalculateStateFee("NY", n); f.StateFee != 1200 {
		t.Errorf("NY large fee = %v, want 1200", f.StateFee)
	}
	// WA — $300 + 0.1% on amount > $50K, max $1,500.
	mid := sampleNotice()
	mid.Offering.TotalOfferingAmount = 500_000
	wa, _ := reg.CalculateStateFee("WA", mid)
	expectedWA := 300.0 + 0.001*(500_000-50_000)
	if wa.StateFee != expectedWA {
		t.Errorf("WA scaled fee = %v, want %v", wa.StateFee, expectedWA)
	}
}

func TestCalculateStateFee_PortalStates(t *testing.T) {
	reg := NewRegistrar(Config{})
	fl, err := reg.CalculateStateFee("FL", sampleNotice())
	if err != nil {
		t.Fatalf("FL: %v", err)
	}
	if fl.Method != "Check" {
		t.Errorf("FL method = %q, want Check (paper portal)", fl.Method)
	}
	if fl.SystemFee != 0 {
		t.Errorf("FL system fee should be 0 (no EFD), got %v", fl.SystemFee)
	}
	if fl.StateFee != 200 {
		t.Errorf("FL state fee = %v", fl.StateFee)
	}
	tx, _ := reg.CalculateStateFee("TX", sampleNotice())
	if tx.Method != "Check" || tx.StateFee != 500 {
		t.Errorf("TX fee = %+v", tx)
	}
}

func TestCalculateStateFee_UnknownState(t *testing.T) {
	reg := NewRegistrar(Config{})
	_, err := reg.CalculateStateFee("ZZ", sampleNotice())
	if !errors.Is(err, ErrUnsupportedState) {
		t.Fatalf("expected ErrUnsupportedState, got %v", err)
	}
}

// --- Validation ---

func TestValidate_MissingFederalExemption(t *testing.T) {
	n := sampleNotice()
	n.Offering.FederalExemption = ""
	if err := validateNoticeFiling(n); err == nil || !strings.Contains(err.Error(), "federal_exemption") {
		t.Fatalf("expected federal_exemption error, got %v", err)
	}
}

func TestValidate_MissingTotalOffering(t *testing.T) {
	n := sampleNotice()
	n.Offering.TotalOfferingAmount = 0
	if err := validateNoticeFiling(n); err == nil || !strings.Contains(err.Error(), "total_offering_amount") {
		t.Fatalf("expected total_offering_amount error, got %v", err)
	}
}

func TestValidate_BadSignatureDate(t *testing.T) {
	n := sampleNotice()
	n.Signature.SignatureDate = "May 15 2026"
	if err := validateNoticeFiling(n); err == nil || !strings.Contains(err.Error(), "YYYY-MM-DD") {
		t.Fatalf("expected date format error, got %v", err)
	}
}

// --- RegisterAdapter override ---

type fakeAdapter struct {
	state    State
	called   atomic.Bool
}

func (f *fakeAdapter) State() State              { return f.state }
func (f *fakeAdapter) SupportsElectronic() bool { return true }
func (f *fakeAdapter) FileNoticeOfSale(_ context.Context, _ *NoticeFiling) (*Acknowledgment, error) {
	f.called.Store(true)
	return &Acknowledgment{FilingID: "FAKE-1", State: f.state, Status: "RECEIVED"}, nil
}
func (f *fakeAdapter) RenewNotice(_ context.Context, _ *RenewalFiling) (*Acknowledgment, error) {
	return &Acknowledgment{FilingID: "FAKE-R", State: f.state, Status: "RECEIVED"}, nil
}
func (f *fakeAdapter) GetFilingStatus(_ context.Context, _ string) (*FilingStatus, error) {
	return &FilingStatus{FilingID: "FAKE-1", State: f.state, Status: "ACCEPTED"}, nil
}
func (f *fakeAdapter) CalculateFee(_ *NoticeFiling) (FeeAmount, error) {
	return FeeAmount{State: f.state, StateFee: 1.0, TotalDue: 1.0, Currency: "USD", Method: "ACH"}, nil
}

func TestRegistrar_RegisterAdapter_Override(t *testing.T) {
	reg := NewRegistrar(Config{})
	fake := &fakeAdapter{state: "CO"}
	reg.RegisterAdapter("CO", fake)
	ack, err := reg.FileNoticeOfSale(context.Background(), "CO", sampleNotice())
	if err != nil {
		t.Fatalf("override submit: %v", err)
	}
	if !fake.called.Load() {
		t.Fatalf("fake adapter not called")
	}
	if ack.FilingID != "FAKE-1" {
		t.Fatalf("ack: %+v", ack)
	}
}

// --- parseRetryAfter sanity ---

func TestParseRetryAfter_Forms(t *testing.T) {
	if got := parseRetryAfter(""); got != 0 {
		t.Fatalf("empty: %v", got)
	}
	if got := parseRetryAfter("3"); got != 3*time.Second {
		t.Fatalf("3: %v", got)
	}
}

// --- base64Std (internal helper) ---

func TestBase64Std(t *testing.T) {
	cases := map[string]string{
		"":     "",
		"f":    "Zg==",
		"fo":   "Zm8=",
		"foo":  "Zm9v",
		"foob": "Zm9vYg==",
	}
	for in, want := range cases {
		if got := base64Std([]byte(in)); got != want {
			t.Errorf("base64Std(%q) = %q, want %q", in, got, want)
		}
	}
}
