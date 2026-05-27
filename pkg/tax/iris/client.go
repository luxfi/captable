// Package iris — IRS IRIS (Information Returns Intake System) A2A
// e-file adapter for the 1099 series. IRIS is the IRS's modernized
// e-file system, mandatory for filers issuing >10 information returns
// in aggregate per year effective Tax Year 2024.
//
// The adapter speaks the IRIS A2A REST surface (per Publication 5717)
// with JWT bearer authentication, marshals submissions to the IRIS
// schema XML, and parses the per-submission acknowledgment.
//
// Source-of-design: Public-Spec
// Source-ref: https://www.irs.gov/e-file-providers/iris-online-portal
// Source-ref: IRS Publication 5717 — IRIS A2A Specifications
// Source-ref: IRS Publication 5718 — IRIS Electronic Filing Application Guide
package iris

import (
	"bytes"
	"context"
	"encoding/json"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"math/rand"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

// Environment endpoints. IRIS A2A is published on two surfaces — the
// AATS sandbox for transmitter conformance and the live production
// surface. Per Publication 5717 §2.
const (
	// ProdURL is the IRIS A2A production submission endpoint.
	ProdURL = "https://la.www4.irs.gov/iris/a2a"

	// AATSURL is the IRIS A2A Assurance Testing System (sandbox)
	// endpoint.
	AATSURL = "https://la.alt.www4.irs.gov/iris/a2a"
)

// MaxPayeesPerSubmission is the IRIS hard cap on payee records in a
// single submission. Per Publication 5717 §3.1.
const MaxPayeesPerSubmission = 100_000

// Errors surfaced by the adapter. Stable; callers may match with
// errors.Is.
var (
	// ErrMissingTCC is returned when a method requires a TCC and the
	// client was constructed without one.
	ErrMissingTCC = errors.New("iris: TCC is required")

	// ErrMissingJWT is returned when a method requires a bearer token
	// and the client has not been authenticated.
	ErrMissingJWT = errors.New("iris: JWT bearer token is required (call Authenticate first)")

	// ErrInvalidSubmission is returned when a FormSubmission fails
	// validation before transmission.
	ErrInvalidSubmission = errors.New("iris: submission validation failed")

	// ErrUnsupportedFormType is returned when the FormSubmission.FormType
	// is not one of the supported 1099 variants.
	ErrUnsupportedFormType = errors.New("iris: unsupported form type")

	// ErrTooManyPayees is returned when a single FormSubmission carries
	// more than MaxPayeesPerSubmission payee records.
	ErrTooManyPayees = errors.New("iris: too many payees per submission")

	// ErrSubmissionRejected is returned when IRIS rejects the
	// submission. The wrapped *APIError carries the IRS-reported reason.
	ErrSubmissionRejected = errors.New("iris: submission rejected")

	// ErrReceiptNotFound is returned by GetSubmissionStatus when IRIS
	// reports no submission for the supplied Receipt ID.
	ErrReceiptNotFound = errors.New("iris: receipt id not found")

	// ErrRateLimited is returned when IRIS sustains 429 / 503 across
	// the configured retry budget.
	ErrRateLimited = errors.New("iris: rate limited after retry budget exhausted")

	// ErrAuthFailed is returned when JWT authentication returns 401.
	ErrAuthFailed = errors.New("iris: authentication failed")
)

// APIError is the structured error returned when IRIS replies with a
// non-2xx status. Callers may use errors.As to recover it.
type APIError struct {
	StatusCode int
	Endpoint   string
	Body       string
	RetryAfter time.Duration
}

// Error implements the error interface.
func (e *APIError) Error() string {
	return fmt.Sprintf("iris API %d on %s: %s", e.StatusCode, e.Endpoint, e.Body)
}

// Credentials carries the JWT-authentication material per Publication
// 5717 §4. IRIS uses a client-credentials JWT issued by the IRS
// Identity service against the transmitter's IRIS Application
// registration.
type Credentials struct {
	// ClientID is the IRIS-Application-assigned client identifier.
	ClientID string

	// ClientSecret is the IRIS-Application-assigned client secret.
	ClientSecret string

	// Username is the IRIS user account associated with the
	// transmitter (the "Responsible Official" or "Authorized User"
	// per the IRIS Application). Required for the IRS Identity
	// password-grant exchange.
	Username string

	// Password is the IRIS user password. Loaded from KMS; never
	// logged.
	Password string
}

// Config holds the per-client configuration for the IRIS adapter. TCC
// authenticates the transmitter on every submission and is loaded from
// KMS, never logged.
type Config struct {
	// TCC is the five-character IRS-assigned Transmitter Control Code.
	// Required.
	TCC string

	// Env is one of EnvProduction or EnvAATS. Required.
	Env IRISEnv

	// Credentials are the JWT exchange credentials. Optional at
	// construction — callers may construct the client and then call
	// Authenticate with credentials at runtime.
	Credentials *Credentials

	// UserAgent is the value sent on every outbound request. Defaults
	// to "luxfi-captable-iris/1.0".
	UserAgent string

	// MaxRetries caps the number of 429 / 503 retries. Default 4
	// (5 total attempts).
	MaxRetries int

	// RetryBaseDelay is the initial backoff between retries. Default
	// 500ms; jittered exponentially up to RetryMaxDelay.
	RetryBaseDelay time.Duration

	// RetryMaxDelay caps the per-retry backoff. Default 30s.
	RetryMaxDelay time.Duration

	// HTTPClient may be supplied for tests / instrumentation. If nil,
	// a 30-second-timeout http.Client is constructed.
	HTTPClient *http.Client

	// BaseURL overrides the environment endpoint. Empty -> resolve
	// from Env. Set in tests against an httptest.Server.
	BaseURL string

	// Clock returns the current time; defaults to time.Now. Pluggable
	// for deterministic tests.
	Clock func() time.Time
}

// Client is the IRIS A2A e-file submission client.
type Client struct {
	cfg      Config
	client   *http.Client
	rand     *rand.Rand
	baseURL  string

	// jwt is the bearer token returned by Authenticate; set on
	// successful exchange and refreshed on 401.
	jwt       string
	jwtExpiry time.Time
}

// NewClient constructs an IRIS Client. Env must be one of
// EnvProduction or EnvAATS; the base URL is resolved from Env unless
// cfg.BaseURL overrides it (test-only).
func NewClient(tcc string, env IRISEnv, opts ...Option) *Client {
	cfg := Config{
		TCC:            tcc,
		Env:            env,
		UserAgent:      "luxfi-captable-iris/1.0",
		MaxRetries:     4,
		RetryBaseDelay: 500 * time.Millisecond,
		RetryMaxDelay:  30 * time.Second,
		HTTPClient:     &http.Client{Timeout: 30 * time.Second},
		Clock:          time.Now,
	}
	for _, opt := range opts {
		opt(&cfg)
	}

	baseURL := cfg.BaseURL
	if baseURL == "" {
		switch env {
		case EnvProduction:
			baseURL = ProdURL
		default:
			baseURL = AATSURL
		}
	}

	return &Client{
		cfg:     cfg,
		client:  cfg.HTTPClient,
		rand:    rand.New(rand.NewSource(cfg.Clock().UnixNano())),
		baseURL: baseURL,
	}
}

// Option is a functional option to NewClient.
type Option func(*Config)

// WithCredentials sets the JWT-exchange credentials. The client may
// also be constructed without credentials and Authenticate called
// later.
func WithCredentials(c Credentials) Option { return func(cfg *Config) { cfg.Credentials = &c } }

// WithHTTPClient overrides the default http.Client (e.g., for tests
// against httptest.Server).
func WithHTTPClient(c *http.Client) Option { return func(cfg *Config) { cfg.HTTPClient = c } }

// WithBaseURL overrides the env-resolved IRIS endpoint. Test-only;
// production code should leave this unset and rely on the Env switch.
func WithBaseURL(u string) Option { return func(cfg *Config) { cfg.BaseURL = u } }

// WithUserAgent sets a custom User-Agent header.
func WithUserAgent(ua string) Option { return func(cfg *Config) { cfg.UserAgent = ua } }

// WithMaxRetries sets the retry cap on 429 / 503 responses.
func WithMaxRetries(n int) Option { return func(cfg *Config) { cfg.MaxRetries = n } }

// WithClock sets a custom clock (test-only).
func WithClock(fn func() time.Time) Option { return func(cfg *Config) { cfg.Clock = fn } }

// TCC returns the configured Transmitter Control Code.
func (c *Client) TCC() string { return c.cfg.TCC }

// Env returns the configured IRIS environment.
func (c *Client) Env() IRISEnv { return c.cfg.Env }

// BaseURL returns the resolved base URL the client targets.
func (c *Client) BaseURL() string { return c.baseURL }

// Authenticate exchanges the configured credentials for a JWT bearer
// token. IRIS issues short-lived tokens (1 hour); the client tracks
// the expiry and re-authenticates on demand.
func (c *Client) Authenticate(ctx context.Context) error {
	if c.cfg.Credentials == nil {
		return fmt.Errorf("iris: credentials required for authentication")
	}
	if c.cfg.TCC == "" {
		return ErrMissingTCC
	}

	body, err := json.Marshal(map[string]string{
		"client_id":     c.cfg.Credentials.ClientID,
		"client_secret": c.cfg.Credentials.ClientSecret,
		"username":      c.cfg.Credentials.Username,
		"password":      c.cfg.Credentials.Password,
		"grant_type":    "password",
		"tcc":           c.cfg.TCC,
	})
	if err != nil {
		return fmt.Errorf("iris: marshal auth body: %w", err)
	}

	endpoint := "/oauth2/token"
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+endpoint, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("iris: build auth request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", c.cfg.UserAgent)

	resp, err := c.client.Do(req)
	if err != nil {
		return fmt.Errorf("iris: auth request: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("iris: read auth response: %w", err)
	}

	if resp.StatusCode == http.StatusUnauthorized {
		return fmt.Errorf("%w: %s", ErrAuthFailed, string(respBody))
	}
	if resp.StatusCode >= 400 {
		return &APIError{StatusCode: resp.StatusCode, Endpoint: endpoint, Body: string(respBody)}
	}

	var ar struct {
		AccessToken string `json:"access_token"`
		ExpiresIn   int    `json:"expires_in"`
		TokenType   string `json:"token_type"`
	}
	if err := json.Unmarshal(respBody, &ar); err != nil {
		return fmt.Errorf("iris: parse auth response: %w", err)
	}
	if ar.AccessToken == "" {
		return fmt.Errorf("iris: auth response missing access_token")
	}

	c.jwt = ar.AccessToken
	if ar.ExpiresIn <= 0 {
		ar.ExpiresIn = 3600
	}
	c.jwtExpiry = c.cfg.Clock().Add(time.Duration(ar.ExpiresIn) * time.Second)
	return nil
}

// SetJWT sets a pre-issued bearer token (for callers that hold their
// own token cache). The expiry is taken at face value; the client
// re-authenticates on 401 if credentials are configured.
func (c *Client) SetJWT(token string, expiry time.Time) {
	c.jwt = token
	c.jwtExpiry = expiry
}

// SubmitForm submits one FormSubmission to IRIS. Returns an
// Acknowledgment carrying the IRIS Receipt ID and the initial
// acceptance status.
func (c *Client) SubmitForm(ctx context.Context, fs *FormSubmission) (*Acknowledgment, error) {
	if c.cfg.TCC == "" {
		return nil, ErrMissingTCC
	}
	if err := c.ensureAuthenticated(ctx); err != nil {
		return nil, err
	}
	if err := validateSubmission(fs); err != nil {
		return nil, fmt.Errorf("%w: %v", ErrInvalidSubmission, err)
	}

	// Default PaymentYearTypeCd from missing -> Original.
	if fs.PaymentYearTypeCd == "" {
		fs.PaymentYearTypeCd = OriginalReturn
	}
	// Default Transmitter.TCC from client if empty.
	if fs.Transmitter.TCC == "" {
		fs.Transmitter.TCC = c.cfg.TCC
	}
	// Default TestFileInd from environment.
	if c.cfg.Env == EnvAATS {
		fs.TestFileInd = true
	}

	wire, err := marshalSubmission(fs)
	if err != nil {
		return nil, fmt.Errorf("iris: marshal submission: %w", err)
	}

	endpoint := "/submissions"
	body, status, err := c.doRequest(ctx, http.MethodPost, endpoint, wire, "application/xml")
	// If we got an API error AND a parseable acknowledgment in the body
	// (the IRIS-rejected case), prefer the typed ErrSubmissionRejected.
	if err != nil {
		if len(body) > 0 {
			if ack, perr := parseAcknowledgment(body); perr == nil && (strings.EqualFold(ack.Status, "Rejected") || len(ack.Errors) > 0) {
				if ack.SubmittedAt.IsZero() {
					ack.SubmittedAt = c.cfg.Clock().UTC()
				}
				return ack, fmt.Errorf("%w: %s", ErrSubmissionRejected, formatErrors(ack.Errors))
			}
		}
		return nil, err
	}
	ack, err := parseAcknowledgment(body)
	if err != nil {
		return nil, fmt.Errorf("iris: parse acknowledgment: %w", err)
	}
	if ack.SubmittedAt.IsZero() {
		ack.SubmittedAt = c.cfg.Clock().UTC()
	}
	if status >= 400 || strings.EqualFold(ack.Status, "Rejected") {
		return ack, fmt.Errorf("%w: %s", ErrSubmissionRejected, formatErrors(ack.Errors))
	}
	return ack, nil
}

// GetSubmissionStatus retrieves the current status of a previously-
// submitted FormSubmission by Receipt ID.
func (c *Client) GetSubmissionStatus(ctx context.Context, receiptID string) (*Status, error) {
	if c.cfg.TCC == "" {
		return nil, ErrMissingTCC
	}
	if receiptID == "" {
		return nil, fmt.Errorf("iris: receipt_id is required")
	}
	if err := c.ensureAuthenticated(ctx); err != nil {
		return nil, err
	}

	endpoint := fmt.Sprintf("/submissions/%s/status", url.PathEscape(receiptID))
	body, status, err := c.doRequest(ctx, http.MethodGet, endpoint, nil, "")
	if err != nil {
		var apiErr *APIError
		if errors.As(err, &apiErr) && apiErr.StatusCode == http.StatusNotFound {
			return nil, ErrReceiptNotFound
		}
		return nil, err
	}
	_ = status
	return parseStatus(body)
}

// ListSubmissions returns a page of submissions matching the supplied
// options. Pagination via opts.PageToken; the returned slice's last
// entry has PageToken-equivalent metadata, but the caller passes the
// opaque token through unchanged for the next call.
func (c *Client) ListSubmissions(ctx context.Context, opts *ListOptions) ([]*Submission, error) {
	if c.cfg.TCC == "" {
		return nil, ErrMissingTCC
	}
	if opts == nil {
		opts = &ListOptions{TCC: c.cfg.TCC}
	}
	if opts.TCC == "" {
		opts.TCC = c.cfg.TCC
	}
	if err := c.ensureAuthenticated(ctx); err != nil {
		return nil, err
	}

	q := url.Values{}
	q.Set("tcc", opts.TCC)
	if opts.FormType != "" {
		q.Set("form_type", string(opts.FormType))
	}
	if opts.TaxYear > 0 {
		q.Set("tax_year", strconv.Itoa(opts.TaxYear))
	}
	if opts.Status != "" {
		q.Set("status", opts.Status)
	}
	if !opts.SubmittedAfter.IsZero() {
		q.Set("submitted_after", opts.SubmittedAfter.UTC().Format(time.RFC3339))
	}
	if !opts.SubmittedBefore.IsZero() {
		q.Set("submitted_before", opts.SubmittedBefore.UTC().Format(time.RFC3339))
	}
	if opts.PageSize > 0 {
		q.Set("page_size", strconv.Itoa(opts.PageSize))
	}
	if opts.PageToken != "" {
		q.Set("page_token", opts.PageToken)
	}

	endpoint := "/submissions?" + q.Encode()
	body, _, err := c.doRequest(ctx, http.MethodGet, endpoint, nil, "")
	if err != nil {
		return nil, err
	}
	return parseSubmissions(body)
}

// CorrectSubmission files a correction against a previously-accepted
// submission. The original submission is identified by originalID
// (IRIS Receipt ID). The Correction carries the corrected payees and
// the (Transmitter, Payer, FormType, TaxYear) tuple that must match
// the original submission — IRIS rejects corrections that don't match
// on payer EIN.
func (c *Client) CorrectSubmission(ctx context.Context, originalID string, corr *Correction) (*Acknowledgment, error) {
	if c.cfg.TCC == "" {
		return nil, ErrMissingTCC
	}
	if originalID == "" {
		return nil, fmt.Errorf("iris: original_receipt_id is required")
	}
	if corr == nil {
		return nil, fmt.Errorf("iris: correction is required")
	}
	if len(corr.Payees) == 0 {
		return nil, fmt.Errorf("iris: correction requires at least one payee")
	}
	if corr.FormType == "" {
		return nil, fmt.Errorf("iris: correction.form_type is required")
	}
	if corr.TaxYear == 0 {
		return nil, fmt.Errorf("iris: correction.tax_year is required")
	}
	if corr.Transmitter.TCC == "" {
		corr.Transmitter.TCC = c.cfg.TCC
	}

	// Build a correction submission. The header carries
	// PaymentYearTypeCd = Corrected and OriginalReceiptID = originalID;
	// each payee carries its own CorrectionType per Pub 5717 §3.2.
	corrSubmission := &FormSubmission{
		FormType:          corr.FormType,
		TaxYear:           corr.TaxYear,
		PaymentYearTypeCd: CorrectedReturn,
		OriginalReceiptID: originalID,
		Transmitter:       corr.Transmitter,
		Payer:             corr.Payer,
		Payees:            corr.Payees,
	}

	return c.SubmitForm(ctx, corrSubmission)
}

// --- internals ---

// ensureAuthenticated re-authenticates if no token is held or the
// current token is within 60s of expiry.
func (c *Client) ensureAuthenticated(ctx context.Context) error {
	if c.jwt == "" {
		if c.cfg.Credentials == nil {
			return ErrMissingJWT
		}
		return c.Authenticate(ctx)
	}
	if !c.jwtExpiry.IsZero() && c.cfg.Clock().Add(60*time.Second).After(c.jwtExpiry) {
		if c.cfg.Credentials != nil {
			return c.Authenticate(ctx)
		}
		// Token expired and no credentials to renew — clear and fail.
		c.jwt = ""
		return ErrMissingJWT
	}
	return nil
}

// doRequest performs a single HTTP request against the IRIS surface
// with the standard retry loop. The Authorization header is set from
// the held JWT; on 401, one re-authentication is attempted before the
// retry budget continues.
func (c *Client) doRequest(ctx context.Context, method, path string, body []byte, contentType string) ([]byte, int, error) {
	var (
		lastStatus     int
		lastBody       []byte
		lastRetryAfter time.Duration
	)

	doOnce := func() (*http.Response, []byte, error) {
		var bodyReader io.Reader
		if len(body) > 0 {
			bodyReader = bytes.NewReader(body)
		}
		req, err := http.NewRequestWithContext(ctx, method, c.baseURL+path, bodyReader)
		if err != nil {
			return nil, nil, err
		}
		if contentType != "" {
			req.Header.Set("Content-Type", contentType)
		}
		req.Header.Set("Accept", "application/xml, application/json")
		req.Header.Set("Authorization", "Bearer "+c.jwt)
		req.Header.Set("User-Agent", c.cfg.UserAgent)
		req.Header.Set("X-IRS-TCC", c.cfg.TCC)
		resp, err := c.client.Do(req)
		if err != nil {
			return nil, nil, err
		}
		defer resp.Body.Close()
		respBody, err := io.ReadAll(resp.Body)
		return resp, respBody, err
	}

	for attempt := 0; attempt <= c.cfg.MaxRetries; attempt++ {
		resp, respBody, err := doOnce()
		if err != nil {
			if attempt == c.cfg.MaxRetries {
				return nil, 0, err
			}
			if waitErr := c.sleepBackoff(ctx, attempt, 0); waitErr != nil {
				return nil, 0, waitErr
			}
			continue
		}
		lastStatus = resp.StatusCode
		lastBody = respBody

		if resp.StatusCode == http.StatusUnauthorized && c.cfg.Credentials != nil {
			// One-shot re-auth attempt then retry.
			if err := c.Authenticate(ctx); err != nil {
				return nil, 0, fmt.Errorf("iris: re-auth on 401 failed: %w", err)
			}
			if attempt == c.cfg.MaxRetries {
				break
			}
			continue
		}
		if resp.StatusCode == http.StatusTooManyRequests ||
			(resp.StatusCode >= 500 && resp.StatusCode != http.StatusNotImplemented) {
			lastRetryAfter = parseRetryAfter(resp.Header.Get("Retry-After"))
			if attempt == c.cfg.MaxRetries {
				break
			}
			if waitErr := c.sleepBackoff(ctx, attempt, lastRetryAfter); waitErr != nil {
				return nil, 0, waitErr
			}
			continue
		}
		if resp.StatusCode >= 400 {
			return respBody, resp.StatusCode, &APIError{
				StatusCode: resp.StatusCode,
				Endpoint:   path,
				Body:       string(respBody),
			}
		}
		return respBody, resp.StatusCode, nil
	}

	apiErr := &APIError{
		StatusCode: lastStatus,
		Endpoint:   path,
		Body:       string(lastBody),
		RetryAfter: lastRetryAfter,
	}
	if lastStatus == http.StatusTooManyRequests {
		return nil, lastStatus, fmt.Errorf("%w: %s", ErrRateLimited, apiErr.Error())
	}
	return nil, lastStatus, apiErr
}

// sleepBackoff waits between retries. If retryAfter is non-zero (from
// the server's Retry-After header), it is honored exactly; otherwise
// exponential backoff with full jitter is applied.
func (c *Client) sleepBackoff(ctx context.Context, attempt int, retryAfter time.Duration) error {
	var wait time.Duration
	if retryAfter > 0 {
		wait = retryAfter
	} else {
		exp := c.cfg.RetryBaseDelay << attempt
		if exp <= 0 || exp > c.cfg.RetryMaxDelay {
			exp = c.cfg.RetryMaxDelay
		}
		wait = time.Duration(c.rand.Int63n(int64(exp) + 1))
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

// parseRetryAfter decodes the Retry-After header in either delta-
// seconds or HTTP-date form. Returns 0 on parse failure.
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

// formatErrors collapses a slice of SubmissionError into a single
// joined string for use in the ErrSubmissionRejected wrapping message.
func formatErrors(errs []SubmissionError) string {
	if len(errs) == 0 {
		return "no error detail"
	}
	parts := make([]string, 0, len(errs))
	for _, e := range errs {
		parts = append(parts, fmt.Sprintf("[%s] %s", e.Code, e.Message))
	}
	return strings.Join(parts, "; ")
}

// --- marshal / unmarshal ---

// marshalSubmission renders a FormSubmission into the IRIS schema XML.
func marshalSubmission(fs *FormSubmission) ([]byte, error) {
	// Pick the per-form element name.
	elemName := formElementName(fs.FormType)
	if elemName == "" {
		return nil, ErrUnsupportedFormType
	}

	root := xmlSubmissionRoot{
		Xmlns:    "http://www.irs.gov/efile/iris",
		XmlnsXSI: "http://www.w3.org/2001/XMLSchema-instance",
		SubmissionHeader: xmlSubmissionHdr{
			TaxYear:           fs.TaxYear,
			FormTypeCd:        fs.FormType,
			PaymentYearTypeCd: fs.PaymentYearTypeCd,
			OriginalReceiptID: fs.OriginalReceiptID,
			Transmitter:       fs.Transmitter,
			Payer:             fs.Payer,
			TotalPayeeCnt:     len(fs.Payees),
		},
	}
	if fs.TestFileInd {
		root.SubmissionHeader.TestFileInd = "X"
	}

	// Marshal the root with the SubmissionHeader, then append each
	// payee's per-form element built from the typed Data. We
	// hand-render the inner records to keep the IRIS schema element
	// names exact.
	var buf bytes.Buffer
	buf.WriteString(xml.Header)

	// Marshal root (header + open tag) without records first.
	root.Form1099Records = nil
	headerOnly, err := xml.MarshalIndent(root, "", "  ")
	if err != nil {
		return nil, err
	}
	// Strip the self-closing/end of root so we can append per-payee
	// records.
	rendered := string(headerOnly)
	closeIdx := strings.LastIndex(rendered, "</Form1099Submission>")
	if closeIdx < 0 {
		return nil, fmt.Errorf("iris: unexpected marshal output")
	}
	buf.WriteString(rendered[:closeIdx])

	for i, p := range fs.Payees {
		recordXML, err := marshalPayeeRecord(fs.FormType, elemName, p, i)
		if err != nil {
			return nil, err
		}
		buf.WriteString("\n  ")
		buf.Write(recordXML)
	}

	buf.WriteString("\n</Form1099Submission>\n")
	return buf.Bytes(), nil
}

// formElementName returns the IRIS per-form wire element name for a
// FormType. Returns empty string on an unsupported type.
func formElementName(ft FormType) string {
	switch ft {
	case Form1099DIV:
		return "Form1099DIV"
	case Form1099B:
		return "Form1099B"
	case Form1099INT:
		return "Form1099INT"
	case Form1099MISC:
		return "Form1099MISC"
	case Form1099NEC:
		return "Form1099NEC"
	case Form1099OID:
		return "Form1099OID"
	case Form1099K:
		return "Form1099K"
	case Form1099R:
		return "Form1099R"
	default:
		return ""
	}
}

// marshalPayeeRecord renders one payee block (Payee + per-form data)
// as an XML element.
func marshalPayeeRecord(ft FormType, elemName string, p PayeeBlock, _ int) ([]byte, error) {
	var buf bytes.Buffer
	if p.CorrectionType != "" {
		fmt.Fprintf(&buf, "<%s correctionTypeCd=%q>", elemName, p.CorrectionType)
	} else {
		fmt.Fprintf(&buf, "<%s>", elemName)
	}
	payeeXML, err := xml.MarshalIndent(p.Payee, "    ", "  ")
	if err != nil {
		return nil, err
	}
	// Rename root element from default to "Payee".
	payeeStr := string(payeeXML)
	payeeStr = strings.Replace(payeeStr, "<Payee>", "<Payee>", 1)
	buf.WriteString("\n    ")
	buf.WriteString(payeeStr)

	// Marshal the per-form data with the wire-element wrapper.
	dataXML, err := marshalFormData(ft, p.Data)
	if err != nil {
		return nil, err
	}
	buf.WriteString("\n    ")
	buf.Write(dataXML)

	fmt.Fprintf(&buf, "\n  </%s>", elemName)
	return buf.Bytes(), nil
}

// marshalFormData wraps a typed Form1099*Data into its IRIS wire
// element. The element names match the IRIS schema (PaymentChoice in
// the schema; here flattened to the per-form payload element).
func marshalFormData(ft FormType, data any) ([]byte, error) {
	if data == nil {
		return nil, fmt.Errorf("iris: payee data is required for form %s", ft)
	}
	type wrapper struct {
		XMLName xml.Name
		Inner   any `xml:",innerxml"`
	}
	wireName, structPtr := wireNameAndStruct(ft, data)
	if wireName == "" {
		return nil, fmt.Errorf("iris: data type %T not compatible with form %s", data, ft)
	}
	inner, err := xml.MarshalIndent(structPtr, "    ", "  ")
	if err != nil {
		return nil, err
	}
	// Replace the struct's default root element with the wire name.
	innerStr := string(inner)
	// Find the first '>' and the last '<' to extract the inner content.
	firstClose := strings.Index(innerStr, ">")
	lastOpen := strings.LastIndex(innerStr, "<")
	if firstClose < 0 || lastOpen < 0 || firstClose >= lastOpen {
		return nil, fmt.Errorf("iris: marshal data: unexpected structure")
	}
	innerContent := innerStr[firstClose+1 : lastOpen]
	return []byte(fmt.Sprintf("<%s>%s</%s>", wireName, innerContent, wireName)), nil
}

// wireNameAndStruct returns the IRIS wire element name and a typed
// pointer for the given FormType + data value. The data may arrive as
// a value or a pointer; the function normalizes to a pointer for the
// xml encoder.
func wireNameAndStruct(ft FormType, data any) (string, any) {
	switch ft {
	case Form1099DIV:
		switch v := data.(type) {
		case *Form1099DIVData:
			return "DividendPayments", v
		case Form1099DIVData:
			return "DividendPayments", &v
		}
	case Form1099B:
		switch v := data.(type) {
		case *Form1099BData:
			return "BrokerProceeds", v
		case Form1099BData:
			return "BrokerProceeds", &v
		}
	case Form1099INT:
		switch v := data.(type) {
		case *Form1099INTData:
			return "InterestIncome", v
		case Form1099INTData:
			return "InterestIncome", &v
		}
	case Form1099MISC:
		switch v := data.(type) {
		case *Form1099MISCData:
			return "MiscellaneousIncome", v
		case Form1099MISCData:
			return "MiscellaneousIncome", &v
		}
	case Form1099NEC:
		switch v := data.(type) {
		case *Form1099NECData:
			return "NonemployeeCompensation", v
		case Form1099NECData:
			return "NonemployeeCompensation", &v
		}
	case Form1099OID:
		switch v := data.(type) {
		case *Form1099OIDData:
			return "OriginalIssueDiscount", v
		case Form1099OIDData:
			return "OriginalIssueDiscount", &v
		}
	case Form1099K:
		switch v := data.(type) {
		case *Form1099KData:
			return "PaymentCardOrThirdParty", v
		case Form1099KData:
			return "PaymentCardOrThirdParty", &v
		}
	case Form1099R:
		switch v := data.(type) {
		case *Form1099RData:
			return "Distributions", v
		case Form1099RData:
			return "Distributions", &v
		}
	}
	return "", nil
}

// validateSubmission runs client-side validation against a
// FormSubmission. Validation mirrors the IRIS schema's required
// elements and the Pub 5717 §3.1 constraints.
func validateSubmission(fs *FormSubmission) error {
	if fs == nil {
		return fmt.Errorf("submission is nil")
	}
	if fs.FormType == "" {
		return fmt.Errorf("form_type is required")
	}
	if formElementName(fs.FormType) == "" {
		return fmt.Errorf("unsupported form_type %q", fs.FormType)
	}
	if fs.TaxYear < 2020 || fs.TaxYear > time.Now().Year()+1 {
		return fmt.Errorf("tax_year %d out of range", fs.TaxYear)
	}
	if fs.Transmitter.TCC == "" {
		return fmt.Errorf("transmitter.tcc is required")
	}
	if fs.Transmitter.EIN == "" {
		return fmt.Errorf("transmitter.ein is required")
	}
	if len(fs.Transmitter.EIN) != 9 {
		return fmt.Errorf("transmitter.ein must be 9 digits, got %d", len(fs.Transmitter.EIN))
	}
	if fs.Payer.Name == "" {
		return fmt.Errorf("payer.name is required")
	}
	if fs.Payer.EIN == "" {
		return fmt.Errorf("payer.ein is required")
	}
	if len(fs.Payer.EIN) != 9 {
		return fmt.Errorf("payer.ein must be 9 digits, got %d", len(fs.Payer.EIN))
	}
	if len(fs.Payees) == 0 {
		return fmt.Errorf("at least one payee is required")
	}
	if len(fs.Payees) > MaxPayeesPerSubmission {
		return ErrTooManyPayees
	}
	for i, p := range fs.Payees {
		if p.Payee.TIN == "" {
			return fmt.Errorf("payees[%d].tin is required", i)
		}
		if len(p.Payee.TIN) != 9 {
			return fmt.Errorf("payees[%d].tin must be 9 digits", i)
		}
		if p.Payee.TINType == "" {
			return fmt.Errorf("payees[%d].tin_type is required (one of S/E/I/A)", i)
		}
		if p.Payee.Name == "" {
			return fmt.Errorf("payees[%d].name is required", i)
		}
		if p.Data == nil {
			return fmt.Errorf("payees[%d].data is required", i)
		}
		if name, _ := wireNameAndStruct(fs.FormType, p.Data); name == "" {
			return fmt.Errorf("payees[%d].data type %T incompatible with form_type %s",
				i, p.Data, fs.FormType)
		}
	}
	if fs.PaymentYearTypeCd == CorrectedReturn || fs.PaymentYearTypeCd == ReplacementReturn {
		if fs.OriginalReceiptID == "" {
			return fmt.Errorf("original_receipt_id is required for %s submissions", fs.PaymentYearTypeCd)
		}
	}
	return nil
}

// parseAcknowledgment decodes the IRIS XML / JSON acknowledgment
// payload returned by SubmitForm. IRIS may reply with either
// content-type; the adapter sniffs the first byte.
func parseAcknowledgment(body []byte) (*Acknowledgment, error) {
	trimmed := bytes.TrimSpace(body)
	if len(trimmed) == 0 {
		return nil, fmt.Errorf("empty acknowledgment body")
	}
	if trimmed[0] == '<' {
		return parseXMLAck(trimmed)
	}
	if trimmed[0] == '{' {
		return parseJSONAck(trimmed)
	}
	return nil, fmt.Errorf("unrecognized acknowledgment format")
}

type xmlAck struct {
	XMLName     xml.Name `xml:"Acknowledgment"`
	ReceiptID   string   `xml:"ReceiptID"`
	Status      string   `xml:"Status"`
	SubmittedAt string   `xml:"SubmittedAt"`
	Errors      []xmlAckError `xml:"Errors>Error"`
	Messages    []string `xml:"Messages>Message"`
}

type xmlAckError struct {
	PayeeIndex int    `xml:"PayeeIndex"`
	Code       string `xml:"Code"`
	Message    string `xml:"Message"`
	Severity   string `xml:"Severity"`
}

func parseXMLAck(body []byte) (*Acknowledgment, error) {
	var x xmlAck
	if err := xml.Unmarshal(body, &x); err != nil {
		return nil, fmt.Errorf("parse xml ack: %w", err)
	}
	ack := &Acknowledgment{
		ReceiptID: x.ReceiptID,
		Status:    x.Status,
		Messages:  x.Messages,
	}
	if x.SubmittedAt != "" {
		if t, err := time.Parse(time.RFC3339, x.SubmittedAt); err == nil {
			ack.SubmittedAt = t
		}
	}
	for _, e := range x.Errors {
		ack.Errors = append(ack.Errors, SubmissionError{
			PayeeIndex: e.PayeeIndex,
			Code:       e.Code,
			Message:    e.Message,
			Severity:   e.Severity,
		})
	}
	return ack, nil
}

type jsonAck struct {
	ReceiptID   string `json:"receipt_id"`
	Status      string `json:"status"`
	SubmittedAt string `json:"submitted_at"`
	Errors      []jsonAckError `json:"errors"`
	Messages    []string `json:"messages"`
}

type jsonAckError struct {
	PayeeIndex int    `json:"payee_index"`
	Code       string `json:"code"`
	Message    string `json:"message"`
	Severity   string `json:"severity"`
}

func parseJSONAck(body []byte) (*Acknowledgment, error) {
	var j jsonAck
	if err := json.Unmarshal(body, &j); err != nil {
		return nil, fmt.Errorf("parse json ack: %w", err)
	}
	ack := &Acknowledgment{
		ReceiptID: j.ReceiptID,
		Status:    j.Status,
		Messages:  j.Messages,
	}
	if j.SubmittedAt != "" {
		if t, err := time.Parse(time.RFC3339, j.SubmittedAt); err == nil {
			ack.SubmittedAt = t
		}
	}
	for _, e := range j.Errors {
		ack.Errors = append(ack.Errors, SubmissionError{
			PayeeIndex: e.PayeeIndex,
			Code:       e.Code,
			Message:    e.Message,
			Severity:   e.Severity,
		})
	}
	return ack, nil
}

// parseStatus decodes the IRIS XML/JSON status reply.
func parseStatus(body []byte) (*Status, error) {
	trimmed := bytes.TrimSpace(body)
	if len(trimmed) == 0 {
		return nil, fmt.Errorf("empty status body")
	}
	if trimmed[0] == '<' {
		return parseXMLStatus(trimmed)
	}
	return parseJSONStatus(trimmed)
}

type xmlStatus struct {
	XMLName     xml.Name `xml:"Status"`
	ReceiptID   string   `xml:"ReceiptID"`
	State       string   `xml:"Status"` // re-named to State to avoid wire name collision when embedded
	AcceptedAt  string   `xml:"AcceptedAt"`
	RejectedAt  string   `xml:"RejectedAt"`
	AcceptedCnt int      `xml:"AcceptedCnt"`
	RejectedCnt int      `xml:"RejectedCnt"`
	Errors      []xmlAckError `xml:"Errors>Error"`
}

func parseXMLStatus(body []byte) (*Status, error) {
	var x xmlStatus
	if err := xml.Unmarshal(body, &x); err != nil {
		return nil, fmt.Errorf("parse xml status: %w", err)
	}
	s := &Status{
		ReceiptID:   x.ReceiptID,
		Status:      x.State,
		AcceptedCnt: x.AcceptedCnt,
		RejectedCnt: x.RejectedCnt,
	}
	if x.AcceptedAt != "" {
		if t, err := time.Parse(time.RFC3339, x.AcceptedAt); err == nil {
			s.AcceptedAt = t
		}
	}
	if x.RejectedAt != "" {
		if t, err := time.Parse(time.RFC3339, x.RejectedAt); err == nil {
			s.RejectedAt = t
		}
	}
	for _, e := range x.Errors {
		s.Errors = append(s.Errors, SubmissionError{
			PayeeIndex: e.PayeeIndex,
			Code:       e.Code,
			Message:    e.Message,
			Severity:   e.Severity,
		})
	}
	return s, nil
}

type jsonStatus struct {
	ReceiptID   string         `json:"receipt_id"`
	Status      string         `json:"status"`
	AcceptedAt  string         `json:"accepted_at"`
	RejectedAt  string         `json:"rejected_at"`
	AcceptedCnt int            `json:"accepted_cnt"`
	RejectedCnt int            `json:"rejected_cnt"`
	Errors      []jsonAckError `json:"errors"`
}

func parseJSONStatus(body []byte) (*Status, error) {
	var j jsonStatus
	if err := json.Unmarshal(body, &j); err != nil {
		return nil, fmt.Errorf("parse json status: %w", err)
	}
	s := &Status{
		ReceiptID:   j.ReceiptID,
		Status:      j.Status,
		AcceptedCnt: j.AcceptedCnt,
		RejectedCnt: j.RejectedCnt,
	}
	if j.AcceptedAt != "" {
		if t, err := time.Parse(time.RFC3339, j.AcceptedAt); err == nil {
			s.AcceptedAt = t
		}
	}
	if j.RejectedAt != "" {
		if t, err := time.Parse(time.RFC3339, j.RejectedAt); err == nil {
			s.RejectedAt = t
		}
	}
	for _, e := range j.Errors {
		s.Errors = append(s.Errors, SubmissionError{
			PayeeIndex: e.PayeeIndex,
			Code:       e.Code,
			Message:    e.Message,
			Severity:   e.Severity,
		})
	}
	return s, nil
}

// parseSubmissions decodes the list-submissions reply.
func parseSubmissions(body []byte) ([]*Submission, error) {
	trimmed := bytes.TrimSpace(body)
	if len(trimmed) == 0 {
		return nil, nil
	}
	if trimmed[0] == '{' {
		var resp struct {
			Submissions []*Submission `json:"submissions"`
		}
		if err := json.Unmarshal(trimmed, &resp); err != nil {
			return nil, fmt.Errorf("parse submissions: %w", err)
		}
		return resp.Submissions, nil
	}
	// XML list — unmarshal into a wrapper.
	type wirelist struct {
		XMLName     xml.Name       `xml:"Submissions"`
		Submissions []wireSubmission `xml:"Submission"`
	}
	var wl wirelist
	if err := xml.Unmarshal(trimmed, &wl); err != nil {
		return nil, fmt.Errorf("parse xml submissions: %w", err)
	}
	out := make([]*Submission, 0, len(wl.Submissions))
	for _, w := range wl.Submissions {
		s := &Submission{
			ReceiptID:  w.ReceiptID,
			FormType:   FormType(w.FormType),
			TaxYear:    w.TaxYear,
			PayeeCount: w.PayeeCount,
			Status:     w.Status,
		}
		if w.SubmittedAt != "" {
			if t, err := time.Parse(time.RFC3339, w.SubmittedAt); err == nil {
				s.SubmittedAt = t
			}
		}
		out = append(out, s)
	}
	return out, nil
}

type wireSubmission struct {
	ReceiptID   string `xml:"ReceiptID"`
	FormType    string `xml:"FormType"`
	TaxYear     int    `xml:"TaxYear"`
	PayeeCount  int    `xml:"PayeeCount"`
	Status      string `xml:"Status"`
	SubmittedAt string `xml:"SubmittedAt"`
}
