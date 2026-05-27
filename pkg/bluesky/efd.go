// EFD adapter — NASAA Electronic Filing Depository.
//
// Implements notice filings (initial, renewal, amendment) and status
// queries against the NASAA EFD API. EFD accepts Form D notice
// filings for 49 states + DC (Florida and Texas are excluded — those
// states require state-portal submission, handled by separate
// adapters in this package).
//
// EFD payment integration uses ACH for filing fees + system-use fees.
// The system-use fee is currently $160 per filing; state fees vary
// per state-statute and are returned by CalculateFee.
//
// Source-of-design: Public-Spec
// Source-ref: http://nasaaefd.org
// Source-ref: http://nasaaefd.org/About/FormDStates

package bluesky

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math/rand"
	"net/http"
	"strconv"
	"strings"
	"time"
)

// EFD environment URLs.
const (
	// EFDProdURL is the production EFD API endpoint.
	EFDProdURL = "https://www.efdnasaa.org"

	// EFDTestURL is the EFD test/sandbox endpoint.
	EFDTestURL = "https://test.efdnasaa.org"
)

// EFDSystemFeeUSD is the per-filing NASAA system-use fee charged on
// every EFD submission, layered on top of the state fee.
const EFDSystemFeeUSD = 160.00

// EFDConfig holds the EFD-specific configuration.
type EFDConfig struct {
	// BaseURL is one of EFDProdURL or EFDTestURL. Required.
	BaseURL string

	// Username is the EFD account username.
	Username string

	// Password is the EFD account password. Loaded from KMS; never
	// logged.
	Password string

	// FirmCRD is the SEC/FINRA CRD number for the filing firm (broker-
	// dealer of record). Required for filings made by a registered
	// broker-dealer; optional for issuer-direct filings.
	FirmCRD string

	// PayerName / PayerAccount are the ACH-payer details for system-
	// use + state fees, registered in EFD account settings.
	PayerName    string
	PayerAccount string
}

// EFDAdapter is the NASAA EFD submission client. Wrapped per-state by
// efdStateWrapper so each state's adapter reports its own State()
// while sharing the underlying transport.
type EFDAdapter struct {
	cfg            EFDConfig
	client         *http.Client
	userAgent      string
	maxRetries     int
	retryBaseDelay time.Duration
	retryMaxDelay  time.Duration
	rand           *rand.Rand
	token          string
	tokenExpires   time.Time
}

// NewEFDAdapter constructs an EFD adapter.
func NewEFDAdapter(cfg EFDConfig, client *http.Client, ua string, maxRetries int, baseDelay, maxDelay time.Duration) *EFDAdapter {
	if cfg.BaseURL == "" {
		cfg.BaseURL = EFDProdURL
	}
	return &EFDAdapter{
		cfg:            cfg,
		client:         client,
		userAgent:      ua,
		maxRetries:     maxRetries,
		retryBaseDelay: baseDelay,
		retryMaxDelay:  maxDelay,
		rand:           rand.New(rand.NewSource(time.Now().UnixNano())),
	}
}

// efdStateWrapper makes the shared EFDAdapter implement StateAdapter
// for a specific state.
type efdStateWrapper struct {
	efd   *EFDAdapter
	state State
}

func (w *efdStateWrapper) State() State              { return w.state }
func (w *efdStateWrapper) SupportsElectronic() bool { return true }

func (w *efdStateWrapper) FileNoticeOfSale(ctx context.Context, n *NoticeFiling) (*Acknowledgment, error) {
	return w.efd.fileNoticeOfSale(ctx, w.state, n)
}
func (w *efdStateWrapper) RenewNotice(ctx context.Context, r *RenewalFiling) (*Acknowledgment, error) {
	return w.efd.renewNotice(ctx, w.state, r)
}
func (w *efdStateWrapper) GetFilingStatus(ctx context.Context, filingID string) (*FilingStatus, error) {
	return w.efd.getFilingStatus(ctx, w.state, filingID)
}
func (w *efdStateWrapper) CalculateFee(n *NoticeFiling) (FeeAmount, error) {
	return w.efd.calculateFee(w.state, n)
}

// --- EFD HTTP transport ---

// efdLoginRequest is the body for POST /Account/Login.
type efdLoginRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
	FirmCRD  string `json:"firmCRD,omitempty"`
}

// efdLoginResponse is the reply from POST /Account/Login.
type efdLoginResponse struct {
	Token   string `json:"token"`
	Expires string `json:"expires"`
}

// efdFilingRequest is the body for POST /FormD/Submit.
type efdFilingRequest struct {
	State            string             `json:"state"`
	FilingType       string             `json:"filingType"`
	Issuer           efdIssuerPayload   `json:"issuer"`
	Offering         efdOfferingPayload `json:"offering"`
	Signature        efdSignaturePayload `json:"signature"`
	SECFileNumber    string             `json:"secFileNumber,omitempty"`
	SECAccessionNumber string           `json:"secAccessionNumber,omitempty"`
	FormDXML         string             `json:"formDXMLBase64,omitempty"` // base64 attached if provided
}

type efdIssuerPayload struct {
	CIK            string  `json:"cik,omitempty"`
	EntityName     string  `json:"entityName"`
	EntityType     string  `json:"entityType"`
	Jurisdiction   string  `json:"jurisdiction"`
	YearOfInc      string  `json:"yearOfIncorporation,omitempty"`
	Street1        string  `json:"street1"`
	Street2        string  `json:"street2,omitempty"`
	City           string  `json:"city"`
	State          string  `json:"state"`
	PostalCode     string  `json:"postalCode"`
	Country        string  `json:"country"`
	Phone          string  `json:"phone"`
	Email          string  `json:"email,omitempty"`
}

type efdOfferingPayload struct {
	FederalExemption       string   `json:"federalExemption"`
	TotalOfferingAmount    float64  `json:"totalOfferingAmount"`
	AmountSoldInState      float64  `json:"amountSoldInState"`
	DateOfFirstSaleInState string   `json:"dateOfFirstSaleInState"` // YYYY-MM-DD
	TypesOfSecurities      []string `json:"typesOfSecurities"`
}

type efdSignaturePayload struct {
	IssuerName     string `json:"issuerName"`
	SignatureName  string `json:"signatureName"`
	NameOfSigner   string `json:"nameOfSigner"`
	SignatureTitle string `json:"signatureTitle"`
	SignatureDate  string `json:"signatureDate"`
}

// efdFilingResponse is the reply from POST /FormD/Submit.
type efdFilingResponse struct {
	FilingID   string   `json:"filingId"`
	Status     string   `json:"status"`
	State      string   `json:"state"`
	Fee        float64  `json:"fee"`
	ReceivedAt string   `json:"receivedAt"`
	ExpiresAt  string   `json:"expiresAt"`
	PaymentID  string   `json:"paymentId"`
	Messages   []string `json:"messages"`
}

// efdRenewalRequest is the body for POST /FormD/Renew.
type efdRenewalRequest struct {
	State            string             `json:"state"`
	OriginalFilingID string             `json:"originalFilingId"`
	Issuer           efdIssuerPayload   `json:"issuer"`
	Offering         efdOfferingPayload `json:"offering"`
	Signature        efdSignaturePayload `json:"signature"`
}

// efdStatusResponse is the reply from GET /FormD/Status/{filingId}.
type efdStatusResponse struct {
	FilingID   string   `json:"filingId"`
	State      string   `json:"state"`
	Status     string   `json:"status"`
	FiledAt    string   `json:"filedAt"`
	AcceptedAt string   `json:"acceptedAt"`
	ExpiresAt  string   `json:"expiresAt"`
	Fee        float64  `json:"fee"`
	Messages   []string `json:"messages"`
}

// authenticate logs the adapter in to EFD if no fresh token is held.
func (a *EFDAdapter) authenticate(ctx context.Context) error {
	if a.token != "" && time.Now().Before(a.tokenExpires.Add(-1*time.Minute)) {
		return nil
	}
	if a.cfg.Username == "" || a.cfg.Password == "" {
		return errors.New("bluesky: EFD credentials missing")
	}
	body, err := a.doJSON(ctx, http.MethodPost, "/Account/Login", efdLoginRequest{
		Username: a.cfg.Username, Password: a.cfg.Password, FirmCRD: a.cfg.FirmCRD,
	})
	if err != nil {
		return fmt.Errorf("bluesky: EFD login: %w", err)
	}
	var lr efdLoginResponse
	if err := json.Unmarshal(body, &lr); err != nil {
		return fmt.Errorf("bluesky: decode EFD login: %w", err)
	}
	a.token = lr.Token
	if lr.Expires != "" {
		if t, err := time.Parse(time.RFC3339, lr.Expires); err == nil {
			a.tokenExpires = t
		} else {
			a.tokenExpires = time.Now().Add(15 * time.Minute)
		}
	} else {
		a.tokenExpires = time.Now().Add(15 * time.Minute)
	}
	return nil
}

func (a *EFDAdapter) fileNoticeOfSale(ctx context.Context, state State, n *NoticeFiling) (*Acknowledgment, error) {
	if err := a.authenticate(ctx); err != nil {
		return nil, err
	}
	req := buildEFDFilingRequest(state, n)
	body, err := a.doJSON(ctx, http.MethodPost, "/FormD/Submit", req)
	if err != nil {
		return nil, err
	}
	var resp efdFilingResponse
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, fmt.Errorf("bluesky: decode EFD submit: %w", err)
	}
	return efdAck(state, &resp), nil
}

func (a *EFDAdapter) renewNotice(ctx context.Context, state State, r *RenewalFiling) (*Acknowledgment, error) {
	if err := a.authenticate(ctx); err != nil {
		return nil, err
	}
	req := efdRenewalRequest{
		State:            string(state),
		OriginalFilingID: r.OriginalFilingID,
		Issuer:           buildEFDIssuer(r.Issuer),
		Offering:         buildEFDOffering(r.UpdatedOffering),
		Signature:        buildEFDSignature(r.Signature),
	}
	body, err := a.doJSON(ctx, http.MethodPost, "/FormD/Renew", req)
	if err != nil {
		return nil, err
	}
	var resp efdFilingResponse
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, fmt.Errorf("bluesky: decode EFD renew: %w", err)
	}
	return efdAck(state, &resp), nil
}

func (a *EFDAdapter) getFilingStatus(ctx context.Context, state State, filingID string) (*FilingStatus, error) {
	if err := a.authenticate(ctx); err != nil {
		return nil, err
	}
	body, err := a.doGet(ctx, "/FormD/Status/"+filingID)
	if err != nil {
		var apiErr *APIError
		if errors.As(err, &apiErr) && apiErr.StatusCode == http.StatusNotFound {
			return nil, ErrFilingNotFound
		}
		return nil, err
	}
	var resp efdStatusResponse
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, fmt.Errorf("bluesky: decode EFD status: %w", err)
	}
	out := &FilingStatus{
		FilingID: resp.FilingID,
		State:    state,
		Status:   strings.ToUpper(resp.Status),
		Fee:      resp.Fee,
		Messages: resp.Messages,
	}
	if t, err := time.Parse(time.RFC3339, resp.FiledAt); err == nil {
		out.FiledAt = t
	}
	if t, err := time.Parse(time.RFC3339, resp.AcceptedAt); err == nil {
		out.AcceptedAt = t
	}
	if t, err := time.Parse(time.RFC3339, resp.ExpiresAt); err == nil {
		out.ExpiresAt = t
	}
	return out, nil
}

func (a *EFDAdapter) calculateFee(state State, n *NoticeFiling) (FeeAmount, error) {
	fee := stateFeeSchedule(state, n)
	return FeeAmount{
		State:     state,
		StateFee:  fee.state,
		SystemFee: EFDSystemFeeUSD,
		TotalDue:  fee.state + EFDSystemFeeUSD,
		Currency:  "USD",
		Method:    "ACH",
		Notes:     fee.notes,
	}, nil
}

// --- HTTP helpers ---

// APIError is the structured error returned when EFD or a state
// portal replies with a non-2xx status.
type APIError struct {
	StatusCode int
	Endpoint   string
	Body       string
	RetryAfter time.Duration
}

func (e *APIError) Error() string {
	return fmt.Sprintf("bluesky API %d on %s: %s", e.StatusCode, e.Endpoint, e.Body)
}

// ErrRateLimited is returned when EFD or a state portal sustains
// 429 across the retry budget.
var ErrRateLimited = errors.New("bluesky: rate limited after retry budget exhausted")

func (a *EFDAdapter) doJSON(ctx context.Context, method, path string, body any) ([]byte, error) {
	var reqBody []byte
	if body != nil {
		var err error
		reqBody, err = json.Marshal(body)
		if err != nil {
			return nil, fmt.Errorf("bluesky: marshal: %w", err)
		}
	}
	doOnce := func() (*http.Response, []byte, error) {
		var reader io.Reader
		if reqBody != nil {
			reader = bytes.NewReader(reqBody)
		}
		req, err := http.NewRequestWithContext(ctx, method, a.cfg.BaseURL+path, reader)
		if err != nil {
			return nil, nil, err
		}
		if reqBody != nil {
			req.Header.Set("Content-Type", "application/json")
		}
		req.Header.Set("Accept", "application/json")
		req.Header.Set("User-Agent", a.userAgent)
		if a.token != "" && path != "/Account/Login" {
			req.Header.Set("Authorization", "Bearer "+a.token)
		}
		resp, err := a.client.Do(req)
		if err != nil {
			return nil, nil, err
		}
		defer resp.Body.Close()
		rb, err := io.ReadAll(resp.Body)
		return resp, rb, err
	}
	return a.doWithRetry(ctx, path, doOnce)
}

func (a *EFDAdapter) doGet(ctx context.Context, path string) ([]byte, error) {
	doOnce := func() (*http.Response, []byte, error) {
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, a.cfg.BaseURL+path, nil)
		if err != nil {
			return nil, nil, err
		}
		req.Header.Set("Accept", "application/json")
		req.Header.Set("User-Agent", a.userAgent)
		if a.token != "" {
			req.Header.Set("Authorization", "Bearer "+a.token)
		}
		resp, err := a.client.Do(req)
		if err != nil {
			return nil, nil, err
		}
		defer resp.Body.Close()
		body, err := io.ReadAll(resp.Body)
		return resp, body, err
	}
	return a.doWithRetry(ctx, path, doOnce)
}

func (a *EFDAdapter) doWithRetry(ctx context.Context, path string, fn func() (*http.Response, []byte, error)) ([]byte, error) {
	var lastBody []byte
	var lastStatus int
	var lastRetryAfter time.Duration
	for attempt := 0; attempt <= a.maxRetries; attempt++ {
		resp, body, err := fn()
		if err != nil {
			if attempt == a.maxRetries {
				return nil, err
			}
			if waitErr := a.sleepBackoff(ctx, attempt, 0); waitErr != nil {
				return nil, waitErr
			}
			continue
		}
		lastBody = body
		lastStatus = resp.StatusCode
		if resp.StatusCode == http.StatusTooManyRequests ||
			(resp.StatusCode >= 500 && resp.StatusCode != http.StatusNotImplemented) {
			lastRetryAfter = parseRetryAfter(resp.Header.Get("Retry-After"))
			if attempt == a.maxRetries {
				break
			}
			if waitErr := a.sleepBackoff(ctx, attempt, lastRetryAfter); waitErr != nil {
				return nil, waitErr
			}
			continue
		}
		if resp.StatusCode >= 400 {
			return nil, &APIError{
				StatusCode: resp.StatusCode, Endpoint: path, Body: string(body),
			}
		}
		return body, nil
	}
	apiErr := &APIError{
		StatusCode: lastStatus, Endpoint: path, Body: string(lastBody), RetryAfter: lastRetryAfter,
	}
	if lastStatus == http.StatusTooManyRequests {
		return nil, fmt.Errorf("%w: %s", ErrRateLimited, apiErr.Error())
	}
	return nil, apiErr
}

func (a *EFDAdapter) sleepBackoff(ctx context.Context, attempt int, retryAfter time.Duration) error {
	var wait time.Duration
	if retryAfter > 0 {
		wait = retryAfter
	} else {
		exp := a.retryBaseDelay << attempt
		if exp <= 0 || exp > a.retryMaxDelay {
			exp = a.retryMaxDelay
		}
		wait = time.Duration(a.rand.Int63n(int64(exp) + 1))
	}
	t := time.NewTimer(wait)
	defer t.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-t.C:
		return nil
	}
}

func parseRetryAfter(s string) time.Duration {
	if s == "" {
		return 0
	}
	if secs, err := strconv.Atoi(strings.TrimSpace(s)); err == nil && secs >= 0 {
		return time.Duration(secs) * time.Second
	}
	if t, err := http.ParseTime(s); err == nil {
		d := time.Until(t)
		if d > 0 {
			return d
		}
	}
	return 0
}

// --- Builders ---

func buildEFDFilingRequest(state State, n *NoticeFiling) efdFilingRequest {
	req := efdFilingRequest{
		State:              string(state),
		FilingType:         string(n.FilingType),
		Issuer:             buildEFDIssuer(n.Issuer),
		Offering:           buildEFDOffering(n.Offering),
		Signature:          buildEFDSignature(n.Signature),
		SECFileNumber:      n.Offering.SECFileNumber,
		SECAccessionNumber: n.Offering.SECAccessionNumber,
	}
	if len(n.FormDXML) > 0 {
		req.FormDXML = base64Std(n.FormDXML)
	}
	if req.FilingType == "" {
		req.FilingType = string(FilingNoticeOfSale)
	}
	return req
}

func buildEFDIssuer(i Issuer) efdIssuerPayload {
	return efdIssuerPayload{
		CIK:          i.CIK,
		EntityName:   i.EntityName,
		EntityType:   i.EntityType,
		Jurisdiction: i.JurisdictionOfIncorporation,
		YearOfInc:    i.YearOfIncorporation,
		Street1:      i.PrimaryAddress.Street1,
		Street2:      i.PrimaryAddress.Street2,
		City:         i.PrimaryAddress.City,
		State:        i.PrimaryAddress.State,
		PostalCode:   i.PrimaryAddress.PostalCode,
		Country:      i.PrimaryAddress.Country,
		Phone:        i.Phone,
		Email:        i.Email,
	}
}

func buildEFDOffering(o OfferingDetails) efdOfferingPayload {
	return efdOfferingPayload{
		FederalExemption:       o.FederalExemption,
		TotalOfferingAmount:    o.TotalOfferingAmount,
		AmountSoldInState:      o.AmountSoldInState,
		DateOfFirstSaleInState: o.DateOfFirstSaleInState.Format("2006-01-02"),
		TypesOfSecurities:      o.TypesOfSecurities,
	}
}

func buildEFDSignature(s Signature) efdSignaturePayload {
	return efdSignaturePayload{
		IssuerName:     s.IssuerName,
		SignatureName:  s.SignatureName,
		NameOfSigner:   s.NameOfSigner,
		SignatureTitle: s.SignatureTitle,
		SignatureDate:  s.SignatureDate,
	}
}

func efdAck(state State, resp *efdFilingResponse) *Acknowledgment {
	out := &Acknowledgment{
		FilingID:  resp.FilingID,
		Status:    strings.ToUpper(resp.Status),
		State:     state,
		Fee:       resp.Fee,
		PaymentID: resp.PaymentID,
		Messages:  resp.Messages,
	}
	if t, err := time.Parse(time.RFC3339, resp.ReceivedAt); err == nil {
		out.ReceivedAt = t
	}
	if t, err := time.Parse(time.RFC3339, resp.ExpiresAt); err == nil {
		out.ExpiresAt = t
	}
	if out.Status == "" {
		out.Status = "RECEIVED"
	}
	return out
}

// efdStates returns the canonical list of states that accept EFD
// notice filings per NASAA's published participation list. All 50
// states + DC except FL and TX.
func efdStates() []State {
	return []State{
		"AL", "AK", "AZ", "AR", "CA", "CO", "CT", "DE",
		"GA", "HI", "ID", "IL", "IN", "IA", "KS", "KY",
		"LA", "ME", "MD", "MA", "MI", "MN", "MS", "MO",
		"MT", "NE", "NV", "NH", "NJ", "NM", "NY", "NC",
		"ND", "OH", "OK", "OR", "PA", "RI", "SC", "SD",
		"TN", "UT", "VT", "VA", "WA", "WV", "WI", "WY",
		"DC",
	}
}

// base64Std encodes bytes as standard (padded) base64.
func base64Std(b []byte) string {
	const tbl = "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789+/"
	if len(b) == 0 {
		return ""
	}
	out := make([]byte, ((len(b)+2)/3)*4)
	j := 0
	for i := 0; i < len(b); i += 3 {
		var n uint32
		n |= uint32(b[i]) << 16
		if i+1 < len(b) {
			n |= uint32(b[i+1]) << 8
		}
		if i+2 < len(b) {
			n |= uint32(b[i+2])
		}
		out[j+0] = tbl[(n>>18)&0x3F]
		out[j+1] = tbl[(n>>12)&0x3F]
		if i+1 < len(b) {
			out[j+2] = tbl[(n>>6)&0x3F]
		} else {
			out[j+2] = '='
		}
		if i+2 < len(b) {
			out[j+3] = tbl[n&0x3F]
		} else {
			out[j+3] = '='
		}
		j += 4
	}
	return string(out)
}
