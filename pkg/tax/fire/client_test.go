package fire

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

// --- fixtures ---

func testTransmitter() TransmitterRecord {
	return TransmitterRecord{
		PaymentYear:           2025,
		TIN:                   "123456789",
		TCC:                   "ABC12",
		TestFileInd:           true,
		TransmitterName:       "LUX INDUSTRIES INC",
		CompanyName:           "LUX INDUSTRIES INC",
		CompanyMailingAddress: "1 LUX WAY",
		CompanyCity:           "WILMINGTON",
		CompanyState:          "DE",
		CompanyZip:            "198010000",
		ContactName:           "TAX OPERATOR",
		ContactPhone:          "5555551234",
		ContactEmail:          "tax@luxfi.io",
		VendorIndicator:       "I",
	}
}

func testPayerGroup(formCode FormCode, amountCodes string) PayerGroup {
	return PayerGroup{
		Payer: PayerRecord{
			PaymentYear:          2025,
			PayerTIN:             "987654321",
			PayerNameControl:     "ACME",
			TypeOfReturn:         formCode,
			AmountCodes:          amountCodes,
			PayerName:            "ACME CORPORATION",
			PayerShippingAddress: "100 ACME PLAZA",
			PayerCity:            "TULSA",
			PayerState:           "OK",
			PayerZip:             "741040000",
			PayerPhone:           "9185551111",
		},
		Payees: []PayeeRecord{
			{
				PaymentYear:         2025,
				NameControl:         "DOE ",
				TypeOfTIN:           "2",
				PayeeTIN:            "111223333",
				PayerAccountNum:     "ACCT-001",
				PaymentAmounts:      map[byte]int64{'1': 500000, '2': 450000, '4': 75000},
				PayeeFirstNameLine:  "DOE JANE",
				PayeeMailingAddress: "200 MAIN ST",
				PayeeCity:           "TULSA",
				PayeeState:          "OK",
				PayeeZip:            "741030000",
			},
		},
	}
}

// --- marshal ---

func TestMarshal_FileLayout(t *testing.T) {
	f := &FIREFile{
		Transmitter: testTransmitter(),
		PayerGroups: []PayerGroup{
			testPayerGroup(FormCode1099DIV, "124"),
		},
	}
	out, err := Marshal(f)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}
	// Each record is 750 + 2 bytes (CR-LF). T + A + B + C + F = 5 records.
	expected := 5 * (RecordLen + 2)
	if len(out) != expected {
		t.Fatalf("len(out) = %d, want %d (5 records × 752)", len(out), expected)
	}
	// First byte of each record must be the record type.
	for i, ch := range []byte{'T', 'A', 'B', 'C', 'F'} {
		off := i * (RecordLen + 2)
		if out[off] != ch {
			t.Fatalf("record %d (offset %d) = %c, want %c", i, off, out[off], ch)
		}
	}
	// CR-LF at the end of every record.
	for i := 0; i < 5; i++ {
		off := i*(RecordLen+2) + RecordLen
		if out[off] != '\r' || out[off+1] != '\n' {
			t.Fatalf("record %d missing CR-LF terminator", i)
		}
	}
}

func TestMarshal_TransmitterFields(t *testing.T) {
	f := &FIREFile{
		Transmitter: testTransmitter(),
		PayerGroups: []PayerGroup{
			testPayerGroup(FormCode1099DIV, "1"),
		},
	}
	out, err := Marshal(f)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}
	tRec := out[:RecordLen]
	// Position 1: 'T'
	if tRec[0] != 'T' {
		t.Fatalf("T[0] = %c", tRec[0])
	}
	// Position 2-5: PaymentYear "2025"
	if string(tRec[1:5]) != "2025" {
		t.Fatalf("T[1:5] = %q, want 2025", string(tRec[1:5]))
	}
	// Position 7-15: TIN
	if string(tRec[6:15]) != "123456789" {
		t.Fatalf("T[6:15] = %q, want 123456789", string(tRec[6:15]))
	}
	// Position 16-20: TCC
	if string(tRec[15:20]) != "ABC12" {
		t.Fatalf("T[15:20] = %q, want ABC12", string(tRec[15:20]))
	}
	// Position 28: TestFileInd = T
	if tRec[27] != 'T' {
		t.Fatalf("T[27] = %c, want T", tRec[27])
	}
	// Position 30-69: TransmitterName left-justified.
	got := strings.TrimRight(string(tRec[29:69]), " ")
	if got != "LUX INDUSTRIES INC" {
		t.Fatalf("transmitter name = %q", got)
	}
	// Position 296-303: TotalPayees zero-padded right-justified.
	if string(tRec[295:303]) != "00000001" {
		t.Fatalf("total payees = %q, want 00000001", string(tRec[295:303]))
	}
}

func TestMarshal_PayeeAmountCodes(t *testing.T) {
	f := &FIREFile{
		Transmitter: testTransmitter(),
		PayerGroups: []PayerGroup{
			testPayerGroup(FormCode1099DIV, "124"),
		},
	}
	out, err := Marshal(f)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}
	bRec := out[2*(RecordLen+2) : 2*(RecordLen+2)+RecordLen]
	if bRec[0] != 'B' {
		t.Fatalf("not a B record")
	}
	// Amount code '1' at positions 55-66 (12 positions).
	// 5000.00 dollars = 500000 cents -> "000000500000"
	if string(bRec[54:66]) != "000000500000" {
		t.Fatalf("amount[1] = %q, want 000000500000", string(bRec[54:66]))
	}
	// Amount code '2' at positions 67-78.
	if string(bRec[66:78]) != "000000450000" {
		t.Fatalf("amount[2] = %q, want 000000450000", string(bRec[66:78]))
	}
	// Amount code '4' at positions 91-102.
	if string(bRec[90:102]) != "000000075000" {
		t.Fatalf("amount[4] = %q, want 000000075000", string(bRec[90:102]))
	}
}

func TestMarshal_EndOfPayerTotals(t *testing.T) {
	f := &FIREFile{
		Transmitter: testTransmitter(),
		PayerGroups: []PayerGroup{
			testPayerGroup(FormCode1099DIV, "12"),
		},
	}
	// Add a second payee with different amounts.
	f.PayerGroups[0].Payees = append(f.PayerGroups[0].Payees, PayeeRecord{
		PaymentYear:         2025,
		NameControl:         "SMIT",
		TypeOfTIN:           "2",
		PayeeTIN:            "444556666",
		PaymentAmounts:      map[byte]int64{'1': 250000, '2': 200000},
		PayeeFirstNameLine:  "SMITH JOHN",
		PayeeMailingAddress: "300 OAK AVE",
		PayeeCity:           "TULSA",
		PayeeState:          "OK",
		PayeeZip:            "741030000",
	})
	out, err := Marshal(f)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}
	// Layout: T + A + B + B + C + F = 6 records.
	cRec := out[4*(RecordLen+2) : 4*(RecordLen+2)+RecordLen]
	if cRec[0] != 'C' {
		t.Fatalf("not a C record")
	}
	// Num payees at 2-9.
	if string(cRec[1:9]) != "00000002" {
		t.Fatalf("num payees = %q, want 00000002", string(cRec[1:9]))
	}
	// Amount code '1' total at positions 16-33 (18 positions).
	// 500000 + 250000 = 750000 cents
	if string(cRec[15:33]) != "000000000000750000" {
		t.Fatalf("amount[1] total = %q, want 000000000000750000", string(cRec[15:33]))
	}
}

func TestMarshal_EndOfFile(t *testing.T) {
	f := &FIREFile{
		Transmitter: testTransmitter(),
		PayerGroups: []PayerGroup{
			testPayerGroup(FormCode1099DIV, "1"),
			testPayerGroup(FormCode1099INT, "1"),
		},
	}
	out, err := Marshal(f)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}
	// Layout: T + A + B + C + A + B + C + F = 8 records.
	// F is the last record.
	off := 7 * (RecordLen + 2)
	fRec := out[off : off+RecordLen]
	if fRec[0] != 'F' {
		t.Fatalf("not an F record")
	}
	if string(fRec[1:9]) != "00000002" {
		t.Fatalf("num payers = %q, want 00000002", string(fRec[1:9]))
	}
	if string(fRec[21:29]) != "00000002" {
		t.Fatalf("num payees = %q, want 00000002", string(fRec[21:29]))
	}
}

func TestMarshal_AllFormCodes(t *testing.T) {
	cases := []FormCode{
		FormCode1099DIV,
		FormCode1099INT,
		FormCode1099B,
		FormCode1099MISC,
		FormCode1099NEC,
		FormCode1099OID,
		FormCode1099K,
		FormCode1099R,
	}
	for _, fc := range cases {
		t.Run(string(fc), func(t *testing.T) {
			f := &FIREFile{
				Transmitter: testTransmitter(),
				PayerGroups: []PayerGroup{testPayerGroup(fc, "1")},
			}
			out, err := Marshal(f)
			if err != nil {
				t.Fatalf("Marshal: %v", err)
			}
			// A record carries the form code at positions 26-27.
			aRec := out[RecordLen+2 : 2*(RecordLen+2)-2]
			if got := string(aRec[25:27]); got != string(fc) {
				t.Fatalf("form code = %q, want %q", got, string(fc))
			}
		})
	}
}

func TestMarshal_CFSF(t *testing.T) {
	f := &FIREFile{
		Transmitter: testTransmitter(),
		PayerGroups: []PayerGroup{
			testPayerGroup(FormCode1099DIV, "1"),
		},
		StateTotals: []StateTotalsRecord{
			{
				NumPayees:              1,
				AmountTotals:           map[byte]int64{'1': 500000},
				StateIncomeTaxWithheld: 25000,
				CombinedFedStateCode:   "06", // California
			},
		},
	}
	f.PayerGroups[0].Payer.CFSFInd = true
	f.PayerGroups[0].Payees[0].StateInfo = PayeeStateInfo{
		StateIncomeTaxWithheld: 25000,
		CombinedFedStateCode:   "06",
	}
	out, err := Marshal(f)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}
	// Find the K record.
	// Layout: T + A + B + C + K + F.
	kOff := 4 * (RecordLen + 2)
	kRec := out[kOff : kOff+RecordLen]
	if kRec[0] != 'K' {
		t.Fatalf("not a K record")
	}
	// Combined federal/state code at positions 747-748.
	if string(kRec[746:748]) != "06" {
		t.Fatalf("CFSF state code = %q, want 06", string(kRec[746:748]))
	}
}

// --- client / http ---

func newAcceptingServer(t *testing.T) (*httptest.Server, *captured) {
	t.Helper()
	cap := &captured{}
	mux := http.NewServeMux()
	mux.HandleFunc("/system/sendFile.aspx", func(w http.ResponseWriter, r *http.Request) {
		cap.submitCount++
		body, _ := io.ReadAll(r.Body)
		cap.lastBody = body
		cap.lastTCC = r.Header.Get("X-IRS-TCC")
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"filename":     "1099_ABC12_2025_test.txt",
			"status":       "Good",
			"submitted_at": time.Now().UTC().Format(time.RFC3339),
		})
	})
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	return srv, cap
}

type captured struct {
	submitCount int
	lastTCC     string
	lastBody    []byte
}

func TestSubmitFile_HappyPath(t *testing.T) {
	srv, cap := newAcceptingServer(t)
	c := NewClient("ABC12", EnvTest,
		WithBaseURL(srv.URL),
		WithCredentials("fire-user", "fire-pass"),
		WithMaxRetries(1),
		WithClock(func() time.Time { return time.Date(2026, 5, 25, 12, 0, 0, 0, time.UTC) }),
	)

	f := &FIREFile{
		Transmitter: testTransmitter(),
		PayerGroups: []PayerGroup{testPayerGroup(FormCode1099DIV, "1")},
	}
	ack, err := c.SubmitFile(context.Background(), f)
	if err != nil {
		t.Fatalf("SubmitFile: %v", err)
	}
	if ack.Status != "Good" {
		t.Fatalf("Status = %q, want Good", ack.Status)
	}
	if ack.Filename == "" {
		t.Fatalf("Filename empty")
	}
	if cap.lastTCC != "ABC12" {
		t.Fatalf("X-IRS-TCC = %q", cap.lastTCC)
	}
	// Multipart body contains the wire-format file content.
	if !bytesContains(cap.lastBody, []byte("LUX INDUSTRIES INC")) {
		t.Fatalf("body missing transmitter name")
	}
}

func TestSubmitFile_ValidationErrors(t *testing.T) {
	c := NewClient("ABC12", EnvTest,
		WithCredentials("u", "p"),
	)
	ctx := context.Background()

	_, err := c.SubmitFile(ctx, &FIREFile{
		Transmitter: TransmitterRecord{PaymentYear: 2025, TCC: "ABC12"},
	})
	if err == nil || !strings.Contains(err.Error(), "tin is required") {
		t.Fatalf("err = %v, want missing tin", err)
	}

	_, err = c.SubmitFile(ctx, &FIREFile{
		Transmitter: testTransmitter(),
		PayerGroups: []PayerGroup{},
	})
	if err == nil || !strings.Contains(err.Error(), "at least one payer group") {
		t.Fatalf("err = %v, want missing payer group", err)
	}

	bad := testTransmitter()
	bad.TIN = "12"
	_, err = c.SubmitFile(ctx, &FIREFile{
		Transmitter: bad,
		PayerGroups: []PayerGroup{testPayerGroup(FormCode1099DIV, "1")},
	})
	if err == nil || !strings.Contains(err.Error(), "9 digits") {
		t.Fatalf("err = %v, want 9 digits", err)
	}
}

func TestSubmitFile_MissingTCC(t *testing.T) {
	c := NewClient("", EnvTest)
	_, err := c.SubmitFile(context.Background(), &FIREFile{})
	if err != ErrMissingTCC {
		t.Fatalf("err = %v, want ErrMissingTCC", err)
	}
}

func TestEnv_BaseURL(t *testing.T) {
	prod := NewClient("X", EnvProduction)
	if prod.BaseURL() != ProdURL {
		t.Fatalf("prod = %q, want %q", prod.BaseURL(), ProdURL)
	}
	test := NewClient("X", EnvTest)
	if test.BaseURL() != TestURL {
		t.Fatalf("test = %q, want %q", test.BaseURL(), TestURL)
	}
}

func TestSubmitFile_Rejected(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/system/sendFile.aspx", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		_ = json.NewEncoder(w).Encode(map[string]any{
			"filename": "1099_ABC12_2025.txt",
			"status":   "Bad",
			"errors":   []string{"transmitter EIN mismatch"},
		})
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	c := NewClient("ABC12", EnvTest,
		WithBaseURL(srv.URL),
		WithCredentials("u", "p"),
	)
	f := &FIREFile{
		Transmitter: testTransmitter(),
		PayerGroups: []PayerGroup{testPayerGroup(FormCode1099DIV, "1")},
	}
	_, err := c.SubmitFile(context.Background(), f)
	if err == nil || !strings.Contains(err.Error(), "rejected") {
		t.Fatalf("err = %v, want rejected", err)
	}
}

func bytesContains(haystack, needle []byte) bool {
	return strings.Contains(string(haystack), string(needle))
}

// --- happy path per form type (8 forms) ---

func TestSubmitFile_AllFormCodes_HappyPaths(t *testing.T) {
	srv, _ := newAcceptingServer(t)
	cases := []FormCode{
		FormCode1099DIV, FormCode1099INT, FormCode1099B,
		FormCode1099MISC, FormCode1099NEC, FormCode1099OID,
		FormCode1099K, FormCode1099R,
	}
	c := NewClient("ABC12", EnvTest,
		WithBaseURL(srv.URL),
		WithCredentials("u", "p"),
	)
	for _, fc := range cases {
		t.Run(string(fc), func(t *testing.T) {
			f := &FIREFile{
				Transmitter: testTransmitter(),
				PayerGroups: []PayerGroup{testPayerGroup(fc, "1")},
			}
			ack, err := c.SubmitFile(context.Background(), f)
			if err != nil {
				t.Fatalf("SubmitFile: %v", err)
			}
			if ack.Status != "Good" {
				t.Fatalf("Status = %q", ack.Status)
			}
		})
	}
}
