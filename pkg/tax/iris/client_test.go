package iris

import (
	"context"
	"encoding/json"
	"encoding/xml"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

// --- test fixtures ---

func testTransmitter() Transmitter {
	return Transmitter{
		TCC:          "ABC12",
		Name:         "Lux Industries Inc.",
		EIN:          "123456789",
		ContactName:  "Filing Operator",
		ContactPhone: "5555551234",
		ContactEmail: "tax@luxfi.io",
		Address: Address{
			AddressLine1: "1 Lux Way",
			City:         "Wilmington",
			State:        "DE",
			ZipCode:      "19801",
		},
	}
}

func testPayer() Payer {
	return Payer{
		Name: "Acme Corporation",
		EIN:  "987654321",
		Address: Address{
			AddressLine1: "100 Acme Plaza",
			City:         "Tulsa",
			State:        "OK",
			ZipCode:      "74104",
		},
		PhoneNum: "9185551111",
	}
}

func testPayee(seq int) Payee {
	return Payee{
		TIN:     fmt.Sprintf("11111111%d", seq%10),
		TINType: "S",
		Name:    fmt.Sprintf("PAYEE%d JANE", seq),
		Address: Address{
			AddressLine1: fmt.Sprintf("%d Main St", seq),
			City:         "Tulsa",
			State:        "OK",
			ZipCode:      "74103",
		},
	}
}

func newAcceptingServer(t *testing.T) (*httptest.Server, *capturedRequests) {
	t.Helper()
	captured := &capturedRequests{}
	mux := http.NewServeMux()

	mux.HandleFunc("/oauth2/token", func(w http.ResponseWriter, r *http.Request) {
		captured.authCount++
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"access_token": "jwt-test-token",
			"token_type":   "Bearer",
			"expires_in":   3600,
		})
	})

	mux.HandleFunc("/submissions", func(w http.ResponseWriter, r *http.Request) {
		captured.submitCount++
		captured.lastMethod = r.Method
		body, _ := io.ReadAll(r.Body)
		captured.lastBody = string(body)
		captured.lastAuth = r.Header.Get("Authorization")
		captured.lastTCC = r.Header.Get("X-IRS-TCC")
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		_ = json.NewEncoder(w).Encode(map[string]any{
			"receipt_id":   fmt.Sprintf("ABC12-%05d-20260525120000", captured.submitCount),
			"status":       "Received",
			"submitted_at": time.Now().UTC().Format(time.RFC3339),
		})
	})

	mux.HandleFunc("/submissions/", func(w http.ResponseWriter, r *http.Request) {
		// path: /submissions/{receipt_id}/status
		captured.statusCount++
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"receipt_id":   "ABC12-00001-20260525120000",
			"status":       "Accepted",
			"accepted_at":  time.Now().UTC().Format(time.RFC3339),
			"accepted_cnt": 1,
			"rejected_cnt": 0,
		})
	})

	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	return srv, captured
}

type capturedRequests struct {
	authCount   int
	submitCount int
	statusCount int
	lastMethod  string
	lastBody    string
	lastAuth    string
	lastTCC     string
}

func newClient(t *testing.T, baseURL string) *Client {
	t.Helper()
	return NewClient("ABC12", EnvAATS,
		WithBaseURL(baseURL),
		WithHTTPClient(&http.Client{Timeout: 5 * time.Second}),
		WithCredentials(Credentials{
			ClientID:     "iris-client-id",
			ClientSecret: "iris-client-secret",
			Username:     "iris-user",
			Password:     "iris-pass",
		}),
		WithMaxRetries(1),
		WithClock(func() time.Time {
			return time.Date(2026, 5, 25, 12, 0, 0, 0, time.UTC)
		}),
	)
}

// --- happy paths, one per form type ---

func TestSubmitForm_1099DIV_HappyPath(t *testing.T) {
	srv, captured := newAcceptingServer(t)
	c := newClient(t, srv.URL)

	fs := &FormSubmission{
		FormType:    Form1099DIV,
		TaxYear:     2025,
		Transmitter: testTransmitter(),
		Payer:       testPayer(),
		Payees: []PayeeBlock{
			{
				Payee: testPayee(1),
				Data: &Form1099DIVData{
					OrdinaryDividends:  5000.00,
					QualifiedDividends: 4500.00,
					FederalTaxWithheld: 1500.00,
				},
			},
		},
	}

	ack, err := c.SubmitForm(context.Background(), fs)
	if err != nil {
		t.Fatalf("SubmitForm: %v", err)
	}
	if !strings.HasPrefix(ack.ReceiptID, "ABC12-") {
		t.Fatalf("ReceiptID = %q, want ABC12-* prefix", ack.ReceiptID)
	}
	if ack.Status != "Received" {
		t.Fatalf("Status = %q, want Received", ack.Status)
	}
	if captured.authCount != 1 {
		t.Fatalf("authCount = %d, want 1", captured.authCount)
	}
	if captured.lastTCC != "ABC12" {
		t.Fatalf("X-IRS-TCC = %q, want ABC12", captured.lastTCC)
	}
	if captured.lastAuth != "Bearer jwt-test-token" {
		t.Fatalf("Authorization = %q, want bearer token", captured.lastAuth)
	}
	if !strings.Contains(captured.lastBody, "<FormTypeCd>1099-DIV</FormTypeCd>") {
		t.Fatalf("body missing form type cd: %s", captured.lastBody)
	}
	if !strings.Contains(captured.lastBody, "<DividendPayments>") {
		t.Fatalf("body missing DividendPayments element: %s", captured.lastBody)
	}
	if !strings.Contains(captured.lastBody, "<TransmitterControlCd>ABC12</TransmitterControlCd>") {
		t.Fatalf("body missing TCC: %s", captured.lastBody)
	}
}

func TestSubmitForm_1099B_HappyPath(t *testing.T) {
	srv, captured := newAcceptingServer(t)
	c := newClient(t, srv.URL)

	fs := &FormSubmission{
		FormType:    Form1099B,
		TaxYear:     2025,
		Transmitter: testTransmitter(),
		Payer:       testPayer(),
		Payees: []PayeeBlock{
			{
				Payee: testPayee(2),
				Data: &Form1099BData{
					Description:       "100 SHS ACME CORP",
					DateAcquired:      "2024-01-15",
					DateSold:          "2025-06-15",
					Proceeds:          15000.00,
					CostBasis:         10000.00,
					ShortTermLongTerm: "L",
				},
			},
		},
	}

	ack, err := c.SubmitForm(context.Background(), fs)
	if err != nil {
		t.Fatalf("SubmitForm: %v", err)
	}
	if ack.Status != "Received" {
		t.Fatalf("Status = %q", ack.Status)
	}
	if !strings.Contains(captured.lastBody, "<BrokerProceeds>") {
		t.Fatalf("body missing BrokerProceeds element: %s", captured.lastBody)
	}
}

func TestSubmitForm_1099INT_HappyPath(t *testing.T) {
	srv, captured := newAcceptingServer(t)
	c := newClient(t, srv.URL)

	fs := &FormSubmission{
		FormType:    Form1099INT,
		TaxYear:     2025,
		Transmitter: testTransmitter(),
		Payer:       testPayer(),
		Payees: []PayeeBlock{
			{
				Payee: testPayee(3),
				Data: &Form1099INTData{InterestIncome: 750.00},
			},
		},
	}

	ack, err := c.SubmitForm(context.Background(), fs)
	if err != nil {
		t.Fatalf("SubmitForm: %v", err)
	}
	if ack.Status != "Received" {
		t.Fatalf("Status = %q", ack.Status)
	}
	if !strings.Contains(captured.lastBody, "<InterestIncome>") {
		t.Fatalf("body missing InterestIncome: %s", captured.lastBody)
	}
}

func TestSubmitForm_1099MISC_HappyPath(t *testing.T) {
	srv, captured := newAcceptingServer(t)
	c := newClient(t, srv.URL)

	fs := &FormSubmission{
		FormType:    Form1099MISC,
		TaxYear:     2025,
		Transmitter: testTransmitter(),
		Payer:       testPayer(),
		Payees: []PayeeBlock{
			{
				Payee: testPayee(4),
				Data: &Form1099MISCData{Rents: 12000.00, Royalties: 500.00},
			},
		},
	}

	ack, err := c.SubmitForm(context.Background(), fs)
	if err != nil {
		t.Fatalf("SubmitForm: %v", err)
	}
	if ack.Status != "Received" {
		t.Fatalf("Status = %q", ack.Status)
	}
	if !strings.Contains(captured.lastBody, "<MiscellaneousIncome>") {
		t.Fatalf("body missing MiscellaneousIncome: %s", captured.lastBody)
	}
}

func TestSubmitForm_1099NEC_HappyPath(t *testing.T) {
	srv, captured := newAcceptingServer(t)
	c := newClient(t, srv.URL)

	fs := &FormSubmission{
		FormType:    Form1099NEC,
		TaxYear:     2025,
		Transmitter: testTransmitter(),
		Payer:       testPayer(),
		Payees: []PayeeBlock{
			{
				Payee: testPayee(5),
				Data: &Form1099NECData{NonemployeeCompensation: 50000.00},
			},
		},
	}

	ack, err := c.SubmitForm(context.Background(), fs)
	if err != nil {
		t.Fatalf("SubmitForm: %v", err)
	}
	if ack.Status != "Received" {
		t.Fatalf("Status = %q", ack.Status)
	}
	if !strings.Contains(captured.lastBody, "<NonemployeeCompensation>") {
		t.Fatalf("body missing NonemployeeCompensation: %s", captured.lastBody)
	}
}

func TestSubmitForm_1099OID_HappyPath(t *testing.T) {
	srv, captured := newAcceptingServer(t)
	c := newClient(t, srv.URL)

	fs := &FormSubmission{
		FormType:    Form1099OID,
		TaxYear:     2025,
		Transmitter: testTransmitter(),
		Payer:       testPayer(),
		Payees: []PayeeBlock{
			{
				Payee: testPayee(6),
				Data: &Form1099OIDData{OriginalIssueDiscount: 1200.00},
			},
		},
	}

	ack, err := c.SubmitForm(context.Background(), fs)
	if err != nil {
		t.Fatalf("SubmitForm: %v", err)
	}
	if ack.Status != "Received" {
		t.Fatalf("Status = %q", ack.Status)
	}
	if !strings.Contains(captured.lastBody, "<OriginalIssueDiscount>") {
		t.Fatalf("body missing OriginalIssueDiscount: %s", captured.lastBody)
	}
}

func TestSubmitForm_1099K_HappyPath(t *testing.T) {
	srv, captured := newAcceptingServer(t)
	c := newClient(t, srv.URL)

	fs := &FormSubmission{
		FormType:    Form1099K,
		TaxYear:     2025,
		Transmitter: testTransmitter(),
		Payer:       testPayer(),
		Payees: []PayeeBlock{
			{
				Payee: testPayee(7),
				Data: &Form1099KData{
					GrossAmount:           120000.00,
					PaymentTransNum:       400,
					PSEIndicator:          "PSE",
					TransactionsIndicator: "payment_card",
				},
			},
		},
	}

	ack, err := c.SubmitForm(context.Background(), fs)
	if err != nil {
		t.Fatalf("SubmitForm: %v", err)
	}
	if ack.Status != "Received" {
		t.Fatalf("Status = %q", ack.Status)
	}
	if !strings.Contains(captured.lastBody, "<PaymentCardOrThirdParty>") {
		t.Fatalf("body missing PaymentCardOrThirdParty: %s", captured.lastBody)
	}
}

func TestSubmitForm_1099R_HappyPath(t *testing.T) {
	srv, captured := newAcceptingServer(t)
	c := newClient(t, srv.URL)

	fs := &FormSubmission{
		FormType:    Form1099R,
		TaxYear:     2025,
		Transmitter: testTransmitter(),
		Payer:       testPayer(),
		Payees: []PayeeBlock{
			{
				Payee: testPayee(8),
				Data: &Form1099RData{
					GrossDistribution:  25000.00,
					TaxableAmount:      25000.00,
					DistributionCodeCd: "1",
				},
			},
		},
	}

	ack, err := c.SubmitForm(context.Background(), fs)
	if err != nil {
		t.Fatalf("SubmitForm: %v", err)
	}
	if ack.Status != "Received" {
		t.Fatalf("Status = %q", ack.Status)
	}
	if !strings.Contains(captured.lastBody, "<Distributions>") {
		t.Fatalf("body missing Distributions: %s", captured.lastBody)
	}
}

// --- validation ---

func TestSubmitForm_ValidationErrors(t *testing.T) {
	srv, _ := newAcceptingServer(t)
	c := newClient(t, srv.URL)
	ctx := context.Background()

	cases := []struct {
		name string
		fs   *FormSubmission
		want string
	}{
		{
			name: "missing form type",
			fs:   &FormSubmission{TaxYear: 2025, Transmitter: testTransmitter(), Payer: testPayer()},
			want: "form_type is required",
		},
		{
			name: "unsupported form type",
			fs:   &FormSubmission{FormType: FormType("1099-XX"), TaxYear: 2025, Transmitter: testTransmitter(), Payer: testPayer()},
			want: "unsupported form_type",
		},
		{
			name: "tax year out of range",
			fs:   &FormSubmission{FormType: Form1099DIV, TaxYear: 1999, Transmitter: testTransmitter(), Payer: testPayer()},
			want: "tax_year",
		},
		{
			name: "missing transmitter ein",
			fs: &FormSubmission{
				FormType: Form1099DIV, TaxYear: 2025,
				Transmitter: Transmitter{TCC: "ABC12"},
				Payer:       testPayer(),
			},
			want: "transmitter.ein is required",
		},
		{
			name: "payer ein wrong length",
			fs: &FormSubmission{
				FormType: Form1099DIV, TaxYear: 2025,
				Transmitter: testTransmitter(),
				Payer:       Payer{Name: "x", EIN: "123"},
			},
			want: "payer.ein must be 9 digits",
		},
		{
			name: "no payees",
			fs: &FormSubmission{
				FormType: Form1099DIV, TaxYear: 2025,
				Transmitter: testTransmitter(),
				Payer:       testPayer(),
			},
			want: "at least one payee is required",
		},
		{
			name: "payee missing tin",
			fs: &FormSubmission{
				FormType: Form1099DIV, TaxYear: 2025,
				Transmitter: testTransmitter(),
				Payer:       testPayer(),
				Payees: []PayeeBlock{
					{Payee: Payee{Name: "x", TINType: "S"}, Data: &Form1099DIVData{OrdinaryDividends: 100}},
				},
			},
			want: "payees[0].tin is required",
		},
		{
			name: "payee data mismatched with form type",
			fs: &FormSubmission{
				FormType: Form1099DIV, TaxYear: 2025,
				Transmitter: testTransmitter(),
				Payer:       testPayer(),
				Payees: []PayeeBlock{
					{Payee: testPayee(1), Data: &Form1099BData{Description: "x"}},
				},
			},
			want: "incompatible with form_type",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := c.SubmitForm(ctx, tc.fs)
			if err == nil {
				t.Fatalf("expected error containing %q, got nil", tc.want)
			}
			if !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("error = %v, want substring %q", err, tc.want)
			}
		})
	}
}

func TestSubmitForm_RejectsTooManyPayees(t *testing.T) {
	srv, _ := newAcceptingServer(t)
	c := newClient(t, srv.URL)

	fs := &FormSubmission{
		FormType:    Form1099DIV,
		TaxYear:     2025,
		Transmitter: testTransmitter(),
		Payer:       testPayer(),
		Payees:      make([]PayeeBlock, MaxPayeesPerSubmission+1),
	}
	_, err := c.SubmitForm(context.Background(), fs)
	if err == nil {
		t.Fatal("expected error for too many payees")
	}
	if !strings.Contains(err.Error(), "too many payees") {
		t.Fatalf("err = %v, want too many payees", err)
	}
}

// --- status / list / correction ---

func TestGetSubmissionStatus(t *testing.T) {
	srv, captured := newAcceptingServer(t)
	c := newClient(t, srv.URL)

	st, err := c.GetSubmissionStatus(context.Background(), "ABC12-00001-20260525120000")
	if err != nil {
		t.Fatalf("GetSubmissionStatus: %v", err)
	}
	if st.Status != "Accepted" {
		t.Fatalf("Status = %q, want Accepted", st.Status)
	}
	if captured.statusCount != 1 {
		t.Fatalf("statusCount = %d, want 1", captured.statusCount)
	}
}

func TestGetSubmissionStatus_NotFound(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/oauth2/token", func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{
			"access_token": "jwt", "expires_in": 3600,
		})
	})
	mux.HandleFunc("/submissions/", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		_, _ = w.Write([]byte(`{"error":"not found"}`))
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	c := newClient(t, srv.URL)
	_, err := c.GetSubmissionStatus(context.Background(), "missing")
	if err != ErrReceiptNotFound {
		t.Fatalf("err = %v, want ErrReceiptNotFound", err)
	}
}

func TestListSubmissions(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/oauth2/token", func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{"access_token": "jwt", "expires_in": 3600})
	})
	mux.HandleFunc("/submissions", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"submissions": []map[string]any{
				{
					"receipt_id":   "ABC12-00001-20260525120000",
					"form_type":    "1099-DIV",
					"tax_year":     2025,
					"payee_count":  10,
					"status":       "Accepted",
					"submitted_at": time.Now().UTC().Format(time.RFC3339),
				},
			},
		})
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	c := newClient(t, srv.URL)
	subs, err := c.ListSubmissions(context.Background(), &ListOptions{
		TCC: "ABC12", FormType: Form1099DIV, TaxYear: 2025, PageSize: 100,
	})
	if err != nil {
		t.Fatalf("ListSubmissions: %v", err)
	}
	if len(subs) != 1 || subs[0].FormType != Form1099DIV {
		t.Fatalf("subs = %+v, want 1 entry of 1099-DIV", subs)
	}
}

func TestCorrectSubmission(t *testing.T) {
	srv, captured := newAcceptingServer(t)
	c := newClient(t, srv.URL)

	corr := &Correction{
		CorrectionType: "OneStep",
		Reason:         "amount correction",
		FormType:       Form1099DIV,
		TaxYear:        2025,
		Transmitter:    testTransmitter(),
		Payer:          testPayer(),
		Payees: []PayeeBlock{
			{
				Payee: testPayee(1),
				Data: &Form1099DIVData{
					OrdinaryDividends:  6000.00, // corrected from 5000
					QualifiedDividends: 5500.00,
				},
				CorrectionType: "G",
			},
		},
	}

	ack, err := c.CorrectSubmission(context.Background(), "ABC12-00001-20260525120000", corr)
	if err != nil {
		t.Fatalf("CorrectSubmission: %v", err)
	}
	if !strings.HasPrefix(ack.ReceiptID, "ABC12-") {
		t.Fatalf("ack.ReceiptID = %q", ack.ReceiptID)
	}
	if !strings.Contains(captured.lastBody, "<PaymentYearTypeCd>G</PaymentYearTypeCd>") {
		t.Fatalf("body missing corrected indicator: %s", captured.lastBody)
	}
	if !strings.Contains(captured.lastBody, "<OriginalReceiptID>ABC12-00001-20260525120000</OriginalReceiptID>") {
		t.Fatalf("body missing original receipt id: %s", captured.lastBody)
	}
	if !strings.Contains(captured.lastBody, `correctionTypeCd="G"`) {
		t.Fatalf("body missing per-payee correctionTypeCd: %s", captured.lastBody)
	}
}

// --- retry / rate limit ---

func TestSubmitForm_RetriesOn429(t *testing.T) {
	attempts := 0
	mux := http.NewServeMux()
	mux.HandleFunc("/oauth2/token", func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{"access_token": "jwt", "expires_in": 3600})
	})
	mux.HandleFunc("/submissions", func(w http.ResponseWriter, r *http.Request) {
		attempts++
		if attempts < 2 {
			w.Header().Set("Retry-After", "0")
			w.WriteHeader(http.StatusTooManyRequests)
			return
		}
		w.WriteHeader(http.StatusCreated)
		_ = json.NewEncoder(w).Encode(map[string]any{
			"receipt_id": "ABC12-RETRY-OK", "status": "Received",
		})
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	c := newClient(t, srv.URL)

	fs := &FormSubmission{
		FormType:    Form1099DIV,
		TaxYear:     2025,
		Transmitter: testTransmitter(),
		Payer:       testPayer(),
		Payees: []PayeeBlock{
			{Payee: testPayee(1), Data: &Form1099DIVData{OrdinaryDividends: 1000}},
		},
	}
	ack, err := c.SubmitForm(context.Background(), fs)
	if err != nil {
		t.Fatalf("SubmitForm: %v", err)
	}
	if ack.ReceiptID != "ABC12-RETRY-OK" {
		t.Fatalf("ReceiptID = %q", ack.ReceiptID)
	}
	if attempts != 2 {
		t.Fatalf("attempts = %d, want 2", attempts)
	}
}

func TestSubmitForm_Rejected(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/oauth2/token", func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{"access_token": "jwt", "expires_in": 3600})
	})
	mux.HandleFunc("/submissions", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		_ = json.NewEncoder(w).Encode(map[string]any{
			"receipt_id": "ABC12-REJ", "status": "Rejected",
			"errors": []map[string]any{
				{"payee_index": 0, "code": "VAL-0042", "message": "bad TIN", "severity": "Error"},
			},
		})
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	c := newClient(t, srv.URL)

	fs := &FormSubmission{
		FormType:    Form1099DIV,
		TaxYear:     2025,
		Transmitter: testTransmitter(),
		Payer:       testPayer(),
		Payees: []PayeeBlock{
			{Payee: testPayee(1), Data: &Form1099DIVData{OrdinaryDividends: 100}},
		},
	}
	_, err := c.SubmitForm(context.Background(), fs)
	if err == nil || !strings.Contains(err.Error(), "rejected") {
		t.Fatalf("err = %v, want rejection", err)
	}
}

// --- auth / re-auth ---

func TestAuthenticate_Failure(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/oauth2/token", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = w.Write([]byte(`{"error":"invalid credentials"}`))
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	c := newClient(t, srv.URL)
	err := c.Authenticate(context.Background())
	if err == nil || !strings.Contains(err.Error(), "authentication failed") {
		t.Fatalf("err = %v, want auth-failed", err)
	}
}

func TestSubmitForm_ReAuthOn401(t *testing.T) {
	authCount := 0
	submitCount := 0
	mux := http.NewServeMux()
	mux.HandleFunc("/oauth2/token", func(w http.ResponseWriter, r *http.Request) {
		authCount++
		_ = json.NewEncoder(w).Encode(map[string]any{
			"access_token": fmt.Sprintf("jwt-%d", authCount), "expires_in": 3600,
		})
	})
	mux.HandleFunc("/submissions", func(w http.ResponseWriter, r *http.Request) {
		submitCount++
		if submitCount < 2 {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		w.WriteHeader(http.StatusCreated)
		_ = json.NewEncoder(w).Encode(map[string]any{"receipt_id": "ABC12-OK", "status": "Received"})
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	c := newClient(t, srv.URL)

	fs := &FormSubmission{
		FormType:    Form1099DIV,
		TaxYear:     2025,
		Transmitter: testTransmitter(),
		Payer:       testPayer(),
		Payees: []PayeeBlock{
			{Payee: testPayee(1), Data: &Form1099DIVData{OrdinaryDividends: 100}},
		},
	}
	ack, err := c.SubmitForm(context.Background(), fs)
	if err != nil {
		t.Fatalf("SubmitForm: %v", err)
	}
	if ack.ReceiptID != "ABC12-OK" {
		t.Fatalf("ReceiptID = %q", ack.ReceiptID)
	}
	if authCount != 2 {
		t.Fatalf("authCount = %d, want 2 (initial + re-auth)", authCount)
	}
}

// --- XML acknowledgment parsing ---

func TestParseAck_XML(t *testing.T) {
	body := []byte(`<?xml version="1.0"?>
<Acknowledgment>
  <ReceiptID>ABC12-XML-1</ReceiptID>
  <Status>Accepted</Status>
  <SubmittedAt>2026-05-25T12:00:00Z</SubmittedAt>
</Acknowledgment>`)
	ack, err := parseAcknowledgment(body)
	if err != nil {
		t.Fatalf("parseAcknowledgment: %v", err)
	}
	if ack.ReceiptID != "ABC12-XML-1" {
		t.Fatalf("ReceiptID = %q", ack.ReceiptID)
	}
	if ack.Status != "Accepted" {
		t.Fatalf("Status = %q", ack.Status)
	}
}

// --- env / base URL resolution ---

func TestEnv_BaseURL(t *testing.T) {
	prod := NewClient("X", EnvProduction)
	if prod.BaseURL() != ProdURL {
		t.Fatalf("prod BaseURL = %q, want %q", prod.BaseURL(), ProdURL)
	}
	aats := NewClient("X", EnvAATS)
	if aats.BaseURL() != AATSURL {
		t.Fatalf("aats BaseURL = %q, want %q", aats.BaseURL(), AATSURL)
	}
}

func TestSubmitForm_MissingTCC(t *testing.T) {
	c := NewClient("", EnvAATS)
	c.SetJWT("token", time.Now().Add(time.Hour))
	_, err := c.SubmitForm(context.Background(), &FormSubmission{FormType: Form1099DIV, TaxYear: 2025})
	if err != ErrMissingTCC {
		t.Fatalf("err = %v, want ErrMissingTCC", err)
	}
}

// --- marshal sanity ---

func TestMarshalSubmission_AATSTestInd(t *testing.T) {
	fs := &FormSubmission{
		FormType:    Form1099NEC,
		TaxYear:     2025,
		Transmitter: testTransmitter(),
		Payer:       testPayer(),
		TestFileInd: true,
		Payees: []PayeeBlock{
			{Payee: testPayee(1), Data: &Form1099NECData{NonemployeeCompensation: 1500}},
		},
	}
	out, err := marshalSubmission(fs)
	if err != nil {
		t.Fatalf("marshalSubmission: %v", err)
	}
	if !strings.Contains(string(out), "<TestFileInd>X</TestFileInd>") {
		t.Fatalf("out missing TestFileInd: %s", out)
	}
	// Make sure the XML is parseable.
	dec := xml.NewDecoder(strings.NewReader(string(out)))
	for {
		if _, err := dec.Token(); err != nil {
			if err == io.EOF {
				break
			}
			t.Fatalf("XML decode error: %v", err)
		}
	}
}
