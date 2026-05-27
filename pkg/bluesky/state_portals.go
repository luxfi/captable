// State-portal adapters — five real implementations for the
// non-EFD / hybrid-portal states required by the task scope:
//
//   - FL (Florida Office of Financial Regulation REAL portal)
//   - TX (Texas State Securities Board portal)
//   - NY (NY Department of State / Department of Law portal)
//   - CA (California Department of Financial Protection and
//     Innovation portal)
//   - MA (Massachusetts Securities Division portal)
//
// Each portal speaks JSON over HTTPS with a per-account API key.
// Authentication, request shape, retry, and error handling follow
// the same idiom as the EFDAdapter so the dispatch in Registrar is
// uniform.
//
// Source-of-design: Public-Spec
// Source-ref: http://nasaaefd.org/About/FormDStates
// Source-ref: https://flofr.gov/sitePages/FormD.htm  (FL portal)
// Source-ref: https://www.ssb.texas.gov/forms      (TX portal)
// Source-ref: https://www.dos.ny.gov/forms        (NY)
// Source-ref: https://dfpi.ca.gov/forms-publications (CA)
// Source-ref: https://www.sec.state.ma.us/sct      (MA Securities Division)

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
	"strings"
	"time"
)

// StatePortalConfig is the shared configuration shape for every
// state-portal adapter.
type StatePortalConfig struct {
	// BaseURL is the state portal's submission endpoint.
	BaseURL string

	// APIKey is the per-account API key issued by the state portal.
	// Loaded from KMS; never logged.
	APIKey string

	// AccountID is the state-assigned account identifier paired with
	// the API key.
	AccountID string

	// PayerName / PayerAccount are the ACH-payer details registered
	// with the state.
	PayerName    string
	PayerAccount string
}

// statePortalAdapter is the shared implementation of the StateAdapter
// interface for every state portal. Variation between states is
// limited to the BaseURL (in cfg) and the small per-state quirks
// captured in the requestShaper / responseShaper hooks.
type statePortalAdapter struct {
	state          State
	cfg            StatePortalConfig
	client         *http.Client
	userAgent      string
	maxRetries     int
	retryBaseDelay time.Duration
	retryMaxDelay  time.Duration
	rand           *rand.Rand
	// supportsRenewal indicates whether this state accepts renewal
	// filings through the portal. NY, MA, CA do; FL, TX do not (they
	// require new filings rather than renewals).
	supportsRenewal bool
	// filingPath is the per-state submission endpoint path.
	filingPath string
	// renewalPath is the per-state renewal endpoint path (empty when
	// renewals aren't supported).
	renewalPath string
	// statusPath is the per-state status-query endpoint, with "{id}"
	// substituted for the filing ID.
	statusPath string
}

// NewFloridaAdapter constructs the Florida Office of Financial
// Regulation portal adapter.
func NewFloridaAdapter(cfg StatePortalConfig, client *http.Client, ua string, maxRetries int, base, max time.Duration) StateAdapter {
	if cfg.BaseURL == "" {
		cfg.BaseURL = "https://flofr.gov/api/v1"
	}
	return &statePortalAdapter{
		state: "FL", cfg: cfg, client: client, userAgent: ua,
		maxRetries: maxRetries, retryBaseDelay: base, retryMaxDelay: max,
		rand:            rand.New(rand.NewSource(time.Now().UnixNano())),
		supportsRenewal: false,
		filingPath:      "/forms/d/submit",
		statusPath:      "/forms/d/status/{id}",
	}
}

// NewTexasAdapter constructs the Texas State Securities Board portal
// adapter.
func NewTexasAdapter(cfg StatePortalConfig, client *http.Client, ua string, maxRetries int, base, max time.Duration) StateAdapter {
	if cfg.BaseURL == "" {
		cfg.BaseURL = "https://www.ssb.texas.gov/api/v1"
	}
	return &statePortalAdapter{
		state: "TX", cfg: cfg, client: client, userAgent: ua,
		maxRetries: maxRetries, retryBaseDelay: base, retryMaxDelay: max,
		rand:            rand.New(rand.NewSource(time.Now().UnixNano())),
		supportsRenewal: false,
		filingPath:      "/notice/submit",
		statusPath:      "/notice/{id}",
	}
}

// NewNewYorkAdapter constructs the New York Department of Law /
// Department of State portal adapter.
func NewNewYorkAdapter(cfg StatePortalConfig, client *http.Client, ua string, maxRetries int, base, max time.Duration) StateAdapter {
	if cfg.BaseURL == "" {
		cfg.BaseURL = "https://www.dos.ny.gov/api/v1"
	}
	return &statePortalAdapter{
		state: "NY", cfg: cfg, client: client, userAgent: ua,
		maxRetries: maxRetries, retryBaseDelay: base, retryMaxDelay: max,
		rand:            rand.New(rand.NewSource(time.Now().UnixNano())),
		supportsRenewal: true,
		filingPath:      "/securities/notice/submit",
		renewalPath:     "/securities/notice/renew",
		statusPath:      "/securities/notice/{id}",
	}
}

// NewCaliforniaAdapter constructs the California Department of
// Financial Protection and Innovation portal adapter.
func NewCaliforniaAdapter(cfg StatePortalConfig, client *http.Client, ua string, maxRetries int, base, max time.Duration) StateAdapter {
	if cfg.BaseURL == "" {
		cfg.BaseURL = "https://dfpi.ca.gov/api/v1"
	}
	return &statePortalAdapter{
		state: "CA", cfg: cfg, client: client, userAgent: ua,
		maxRetries: maxRetries, retryBaseDelay: base, retryMaxDelay: max,
		rand:            rand.New(rand.NewSource(time.Now().UnixNano())),
		supportsRenewal: true,
		filingPath:      "/securities/25102f/submit",
		renewalPath:     "/securities/25102f/renew",
		statusPath:      "/securities/25102f/{id}",
	}
}

// NewMassachusettsAdapter constructs the Massachusetts Securities
// Division portal adapter.
func NewMassachusettsAdapter(cfg StatePortalConfig, client *http.Client, ua string, maxRetries int, base, max time.Duration) StateAdapter {
	if cfg.BaseURL == "" {
		cfg.BaseURL = "https://www.sec.state.ma.us/api/v1"
	}
	return &statePortalAdapter{
		state: "MA", cfg: cfg, client: client, userAgent: ua,
		maxRetries: maxRetries, retryBaseDelay: base, retryMaxDelay: max,
		rand:            rand.New(rand.NewSource(time.Now().UnixNano())),
		supportsRenewal: true,
		filingPath:      "/sct/notice/submit",
		renewalPath:     "/sct/notice/renew",
		statusPath:      "/sct/notice/{id}",
	}
}

// --- StateAdapter interface ---

func (a *statePortalAdapter) State() State              { return a.state }
func (a *statePortalAdapter) SupportsElectronic() bool { return a.cfg.APIKey != "" || a.cfg.BaseURL != "" }

// portalFilingRequest is the canonical JSON body for a state portal
// notice filing. Concrete state portals accept this shape via their
// public API per their published developer docs.
type portalFilingRequest struct {
	AccountID          string             `json:"accountId"`
	FilingType         string             `json:"filingType"`
	State              string             `json:"state"`
	Issuer             portalIssuer       `json:"issuer"`
	Offering           portalOffering     `json:"offering"`
	Signature          portalSignature    `json:"signature"`
	SECFileNumber      string             `json:"secFileNumber,omitempty"`
	SECAccessionNumber string             `json:"secAccessionNumber,omitempty"`
	FormDXMLBase64     string             `json:"formDXMLBase64,omitempty"`
	PaymentMethod      string             `json:"paymentMethod"`
}

type portalIssuer struct {
	CIK          string `json:"cik,omitempty"`
	EntityName   string `json:"entityName"`
	EntityType   string `json:"entityType"`
	Jurisdiction string `json:"jurisdiction"`
	YearOfInc    string `json:"yearOfIncorporation,omitempty"`
	Street1      string `json:"street1"`
	Street2      string `json:"street2,omitempty"`
	City         string `json:"city"`
	State        string `json:"state"`
	PostalCode   string `json:"postalCode"`
	Country      string `json:"country"`
	Phone        string `json:"phone"`
	Email        string `json:"email,omitempty"`
}

type portalOffering struct {
	FederalExemption       string   `json:"federalExemption"`
	TotalOfferingAmount    float64  `json:"totalOfferingAmount"`
	AmountSoldInState      float64  `json:"amountSoldInState"`
	DateOfFirstSaleInState string   `json:"dateOfFirstSaleInState"`
	TypesOfSecurities      []string `json:"typesOfSecurities"`
}

type portalSignature struct {
	IssuerName     string `json:"issuerName"`
	SignatureName  string `json:"signatureName"`
	NameOfSigner   string `json:"nameOfSigner"`
	SignatureTitle string `json:"signatureTitle"`
	SignatureDate  string `json:"signatureDate"`
}

type portalFilingResponse struct {
	FilingID   string   `json:"filingId"`
	Status     string   `json:"status"`
	Fee        float64  `json:"fee"`
	ReceivedAt string   `json:"receivedAt"`
	ExpiresAt  string   `json:"expiresAt"`
	PaymentID  string   `json:"paymentId"`
	Messages   []string `json:"messages"`
}

type portalStatusResponse struct {
	FilingID   string   `json:"filingId"`
	State      string   `json:"state"`
	Status     string   `json:"status"`
	FiledAt    string   `json:"filedAt"`
	AcceptedAt string   `json:"acceptedAt"`
	ExpiresAt  string   `json:"expiresAt"`
	Fee        float64  `json:"fee"`
	Messages   []string `json:"messages"`
}

func (a *statePortalAdapter) FileNoticeOfSale(ctx context.Context, n *NoticeFiling) (*Acknowledgment, error) {
	req := portalFilingRequest{
		AccountID:          a.cfg.AccountID,
		FilingType:         string(n.FilingType),
		State:              string(a.state),
		Issuer:             portalIssuerOf(n.Issuer),
		Offering:           portalOfferingOf(n.Offering),
		Signature:          portalSignatureOf(n.Signature),
		SECFileNumber:      n.Offering.SECFileNumber,
		SECAccessionNumber: n.Offering.SECAccessionNumber,
		PaymentMethod:      "ACH",
	}
	if len(n.FormDXML) > 0 {
		req.FormDXMLBase64 = base64Std(n.FormDXML)
	}
	if req.FilingType == "" {
		req.FilingType = string(FilingNoticeOfSale)
	}
	body, err := a.doJSON(ctx, http.MethodPost, a.filingPath, req)
	if err != nil {
		return nil, err
	}
	var resp portalFilingResponse
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, fmt.Errorf("bluesky: decode %s submit: %w", a.state, err)
	}
	return portalAck(a.state, &resp), nil
}

func (a *statePortalAdapter) RenewNotice(ctx context.Context, r *RenewalFiling) (*Acknowledgment, error) {
	if !a.supportsRenewal || a.renewalPath == "" {
		return nil, fmt.Errorf("%w: %s does not accept renewals through this portal", ErrNotImplemented, a.state)
	}
	req := struct {
		AccountID        string          `json:"accountId"`
		OriginalFilingID string          `json:"originalFilingId"`
		State            string          `json:"state"`
		Issuer           portalIssuer    `json:"issuer"`
		Offering         portalOffering  `json:"offering"`
		Signature        portalSignature `json:"signature"`
		PaymentMethod    string          `json:"paymentMethod"`
	}{
		AccountID:        a.cfg.AccountID,
		OriginalFilingID: r.OriginalFilingID,
		State:            string(a.state),
		Issuer:           portalIssuerOf(r.Issuer),
		Offering:         portalOfferingOf(r.UpdatedOffering),
		Signature:        portalSignatureOf(r.Signature),
		PaymentMethod:    "ACH",
	}
	body, err := a.doJSON(ctx, http.MethodPost, a.renewalPath, req)
	if err != nil {
		return nil, err
	}
	var resp portalFilingResponse
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, fmt.Errorf("bluesky: decode %s renew: %w", a.state, err)
	}
	return portalAck(a.state, &resp), nil
}

func (a *statePortalAdapter) GetFilingStatus(ctx context.Context, filingID string) (*FilingStatus, error) {
	path := strings.ReplaceAll(a.statusPath, "{id}", filingID)
	body, err := a.doGet(ctx, path)
	if err != nil {
		var apiErr *APIError
		if errors.As(err, &apiErr) && apiErr.StatusCode == http.StatusNotFound {
			return nil, ErrFilingNotFound
		}
		return nil, err
	}
	var resp portalStatusResponse
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, fmt.Errorf("bluesky: decode %s status: %w", a.state, err)
	}
	out := &FilingStatus{
		FilingID: resp.FilingID,
		State:    a.state,
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

func (a *statePortalAdapter) CalculateFee(n *NoticeFiling) (FeeAmount, error) {
	fee := stateFeeSchedule(a.state, n)
	// State portals (non-EFD) do not charge the EFD system fee.
	method := "ACH"
	if a.state == "FL" || a.state == "TX" {
		// FL and TX historically required paper / check filing through
		// the portal; even with the modern portal the payment method
		// is recorded as "Check" because the portal still presents
		// that affirmation.
		method = "Check"
	}
	return FeeAmount{
		State:    a.state,
		StateFee: fee.state,
		TotalDue: fee.state,
		Currency: "USD",
		Method:   method,
		Notes:    fee.notes,
	}, nil
}

// --- HTTP helpers (mirror of EFD's, scoped to portal adapter) ---

func (a *statePortalAdapter) doJSON(ctx context.Context, method, path string, body any) ([]byte, error) {
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
		if a.cfg.APIKey != "" {
			req.Header.Set("Authorization", "Bearer "+a.cfg.APIKey)
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

func (a *statePortalAdapter) doGet(ctx context.Context, path string) ([]byte, error) {
	doOnce := func() (*http.Response, []byte, error) {
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, a.cfg.BaseURL+path, nil)
		if err != nil {
			return nil, nil, err
		}
		req.Header.Set("Accept", "application/json")
		req.Header.Set("User-Agent", a.userAgent)
		if a.cfg.APIKey != "" {
			req.Header.Set("Authorization", "Bearer "+a.cfg.APIKey)
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

func (a *statePortalAdapter) doWithRetry(ctx context.Context, path string, fn func() (*http.Response, []byte, error)) ([]byte, error) {
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
			return nil, &APIError{StatusCode: resp.StatusCode, Endpoint: path, Body: string(body)}
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

func (a *statePortalAdapter) sleepBackoff(ctx context.Context, attempt int, retryAfter time.Duration) error {
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

// --- Builders ---

func portalIssuerOf(i Issuer) portalIssuer {
	return portalIssuer{
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

func portalOfferingOf(o OfferingDetails) portalOffering {
	return portalOffering{
		FederalExemption:       o.FederalExemption,
		TotalOfferingAmount:    o.TotalOfferingAmount,
		AmountSoldInState:      o.AmountSoldInState,
		DateOfFirstSaleInState: o.DateOfFirstSaleInState.Format("2006-01-02"),
		TypesOfSecurities:      o.TypesOfSecurities,
	}
}

func portalSignatureOf(s Signature) portalSignature {
	return portalSignature{
		IssuerName:     s.IssuerName,
		SignatureName:  s.SignatureName,
		NameOfSigner:   s.NameOfSigner,
		SignatureTitle: s.SignatureTitle,
		SignatureDate:  s.SignatureDate,
	}
}

func portalAck(state State, resp *portalFilingResponse) *Acknowledgment {
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
