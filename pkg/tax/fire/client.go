// Package fire — IRS FIRE (Filing Information Returns Electronically)
// e-file adapter for the 1099 series. FIRE is the legacy IRS e-file
// surface; IRIS is the modern replacement and is mandatory going
// forward (TY 2024+). The FIRE adapter is preserved here to file
// corrections against older years that were originally submitted to
// FIRE and remain in IRS's FIRE-side records.
//
// FIRE file format is fixed-width records, 750 bytes per record (left-
// padded with zeros, right-padded with blanks), record types T A B C K
// F. The adapter serializes a FIREFile into the wire bytes and posts
// to the FIRE upload endpoint.
//
// Source-of-design: Public-Spec
// Source-ref: IRS Publication 1220 — Specifications for Electronic Filing of Forms 1097, 1098, 1099, 3921, 3922, 5498, and W-2G
// Source-ref: https://www.irs.gov/pub/irs-pdf/p1220.pdf
package fire

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"math/rand"
	"mime/multipart"
	"net/http"
	"strconv"
	"strings"
	"time"
)

// Environment endpoints for the FIRE submission surface. Per
// Publication 1220 Part B §3.
const (
	// ProdURL is the FIRE production system endpoint.
	ProdURL = "https://fire.irs.gov"

	// TestURL is the FIRE test system endpoint.
	TestURL = "https://fire.test.irs.gov"
)

// RecordLen is the fixed FIRE record length per Publication 1220 Part
// C §1. Every record (T, A, B, C, K, F) is exactly 750 bytes.
const RecordLen = 750

// Errors surfaced by the adapter.
var (
	ErrMissingTCC      = errors.New("fire: TCC is required")
	ErrInvalidFile     = errors.New("fire: file validation failed")
	ErrSubmissionRejected = errors.New("fire: submission rejected")
	ErrRateLimited     = errors.New("fire: rate limited after retry budget exhausted")
	ErrUnsupportedFormCode = errors.New("fire: unsupported form code")
)

// APIError is the structured error returned when FIRE replies with a
// non-2xx status.
type APIError struct {
	StatusCode int
	Endpoint   string
	Body       string
	RetryAfter time.Duration
}

// Error implements the error interface.
func (e *APIError) Error() string {
	return fmt.Sprintf("fire API %d on %s: %s", e.StatusCode, e.Endpoint, e.Body)
}

// Config holds the per-issuer authentication and tuning for the FIRE
// adapter. TCC + LoginName + PasswordHash are loaded from KMS, never
// logged.
type Config struct {
	// TCC is the IRS-assigned Transmitter Control Code (5 chars).
	TCC string

	// Env is one of EnvProduction or EnvTest.
	Env FIREEnv

	// LoginName is the FIRE-system user name registered against the
	// transmitter.
	LoginName string

	// PasswordHash is the FIRE-system password (loaded from KMS, never
	// logged).
	PasswordHash string

	// UserAgent is the value sent on every outbound request.
	UserAgent string

	// MaxRetries caps the number of 429 / 503 retries.
	MaxRetries int

	// RetryBaseDelay is the initial backoff between retries.
	RetryBaseDelay time.Duration

	// RetryMaxDelay caps the per-retry backoff.
	RetryMaxDelay time.Duration

	// HTTPClient may be supplied for tests / instrumentation.
	HTTPClient *http.Client

	// BaseURL overrides the env-resolved endpoint. Test-only.
	BaseURL string

	// Clock returns the current time; defaults to time.Now.
	Clock func() time.Time
}

// Client is the FIRE submission client.
type Client struct {
	cfg     Config
	client  *http.Client
	rand    *rand.Rand
	baseURL string
}

// NewClient constructs a FIRE Client.
func NewClient(tcc string, env FIREEnv, opts ...Option) *Client {
	cfg := Config{
		TCC:            tcc,
		Env:            env,
		UserAgent:      "luxfi-captable-fire/1.0",
		MaxRetries:     4,
		RetryBaseDelay: 500 * time.Millisecond,
		RetryMaxDelay:  30 * time.Second,
		HTTPClient:     &http.Client{Timeout: 60 * time.Second},
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
			baseURL = TestURL
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

// WithCredentials sets the FIRE login credentials.
func WithCredentials(login, passHash string) Option {
	return func(cfg *Config) {
		cfg.LoginName = login
		cfg.PasswordHash = passHash
	}
}

// WithHTTPClient overrides the default http.Client.
func WithHTTPClient(c *http.Client) Option { return func(cfg *Config) { cfg.HTTPClient = c } }

// WithBaseURL overrides the env-resolved FIRE endpoint. Test-only.
func WithBaseURL(u string) Option { return func(cfg *Config) { cfg.BaseURL = u } }

// WithUserAgent sets a custom User-Agent header.
func WithUserAgent(ua string) Option { return func(cfg *Config) { cfg.UserAgent = ua } }

// WithMaxRetries sets the retry cap on 429 / 503 responses.
func WithMaxRetries(n int) Option { return func(cfg *Config) { cfg.MaxRetries = n } }

// WithClock sets a custom clock (test-only).
func WithClock(fn func() time.Time) Option { return func(cfg *Config) { cfg.Clock = fn } }

// TCC returns the configured Transmitter Control Code.
func (c *Client) TCC() string { return c.cfg.TCC }

// Env returns the configured FIRE environment.
func (c *Client) Env() FIREEnv { return c.cfg.Env }

// BaseURL returns the resolved FIRE base URL.
func (c *Client) BaseURL() string { return c.baseURL }

// SubmitFile uploads a FIREFile to the FIRE system. The file is
// marshaled into fixed-width records, posted as a multipart upload,
// and the FIRE reply parsed into an Acknowledgment.
func (c *Client) SubmitFile(ctx context.Context, f *FIREFile) (*Acknowledgment, error) {
	if c.cfg.TCC == "" {
		return nil, ErrMissingTCC
	}
	if err := validateFile(f, c.cfg.Env == EnvTest); err != nil {
		return nil, fmt.Errorf("%w: %v", ErrInvalidFile, err)
	}

	wire, err := Marshal(f)
	if err != nil {
		return nil, fmt.Errorf("fire: marshal file: %w", err)
	}

	endpoint := "/system/sendFile.aspx"
	uploadName := fmt.Sprintf("1099_%s_%d_%s.txt",
		f.Transmitter.TCC, f.Transmitter.PaymentYear, c.fileHash(wire))

	var buf bytes.Buffer
	w := multipart.NewWriter(&buf)
	if err := w.WriteField("LoginName", c.cfg.LoginName); err != nil {
		return nil, fmt.Errorf("fire: write LoginName: %w", err)
	}
	if err := w.WriteField("Password", c.cfg.PasswordHash); err != nil {
		return nil, fmt.Errorf("fire: write Password: %w", err)
	}
	if err := w.WriteField("TCC", c.cfg.TCC); err != nil {
		return nil, fmt.Errorf("fire: write TCC: %w", err)
	}
	part, err := w.CreateFormFile("file", uploadName)
	if err != nil {
		return nil, fmt.Errorf("fire: create file part: %w", err)
	}
	if _, err := part.Write(wire); err != nil {
		return nil, fmt.Errorf("fire: write file payload: %w", err)
	}
	if err := w.Close(); err != nil {
		return nil, fmt.Errorf("fire: close multipart: %w", err)
	}

	respBody, statusCode, err := c.doMultipart(ctx, endpoint, w.FormDataContentType(), buf.Bytes())
	if err != nil {
		// If FIRE returned an HTTP error AND a parseable rejection body,
		// prefer the typed ErrSubmissionRejected.
		if len(respBody) > 0 {
			if ack, perr := parseAcknowledgment(respBody); perr == nil &&
				(strings.EqualFold(ack.Status, "Bad") || len(ack.Errors) > 0) {
				if ack.Filename == "" {
					ack.Filename = uploadName
				}
				if ack.SubmittedAt.IsZero() {
					ack.SubmittedAt = c.cfg.Clock().UTC()
				}
				return ack, fmt.Errorf("%w: %s", ErrSubmissionRejected, strings.Join(ack.Errors, "; "))
			}
		}
		return nil, err
	}
	ack, perr := parseAcknowledgment(respBody)
	if perr != nil {
		return nil, fmt.Errorf("fire: parse acknowledgment: %w", perr)
	}
	if ack.Filename == "" {
		ack.Filename = uploadName
	}
	if ack.SubmittedAt.IsZero() {
		ack.SubmittedAt = c.cfg.Clock().UTC()
	}
	if statusCode >= 400 || strings.EqualFold(ack.Status, "Bad") {
		return ack, fmt.Errorf("%w: %s", ErrSubmissionRejected, strings.Join(ack.Errors, "; "))
	}
	return ack, nil
}

// --- internals ---

// fileHash returns a short hex hash to disambiguate filenames in the
// FIRE filing UI. Not transmitted to FIRE in semantic-content positions.
func (c *Client) fileHash(b []byte) string {
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:8])
}

// doMultipart performs a single multipart POST against the FIRE
// surface with the standard 429/5xx retry loop.
func (c *Client) doMultipart(ctx context.Context, path, contentType string, body []byte) ([]byte, int, error) {
	var (
		lastStatus     int
		lastBody       []byte
		lastRetryAfter time.Duration
	)
	doOnce := func() (*http.Response, []byte, error) {
		req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+path, bytes.NewReader(body))
		if err != nil {
			return nil, nil, err
		}
		req.Header.Set("Content-Type", contentType)
		req.Header.Set("Accept", "text/html, application/json, text/plain")
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

// sleepBackoff waits between retries.
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

// validateFile runs client-side validation against a FIREFile.
func validateFile(f *FIREFile, isTest bool) error {
	if f == nil {
		return fmt.Errorf("file is nil")
	}
	if f.Transmitter.TCC == "" {
		return fmt.Errorf("transmitter.tcc is required")
	}
	if len(f.Transmitter.TCC) != 5 {
		return fmt.Errorf("transmitter.tcc must be 5 chars, got %d", len(f.Transmitter.TCC))
	}
	if f.Transmitter.TIN == "" {
		return fmt.Errorf("transmitter.tin is required")
	}
	if len(f.Transmitter.TIN) != 9 {
		return fmt.Errorf("transmitter.tin must be 9 digits, got %d", len(f.Transmitter.TIN))
	}
	if f.Transmitter.PaymentYear < 2000 || f.Transmitter.PaymentYear > time.Now().Year()+1 {
		return fmt.Errorf("transmitter.payment_year %d out of range", f.Transmitter.PaymentYear)
	}
	if isTest && !f.Transmitter.TestFileInd {
		return fmt.Errorf("transmitter.test_file_ind must be true for FIRE test environment")
	}
	if len(f.PayerGroups) == 0 {
		return fmt.Errorf("at least one payer group is required")
	}
	for i, g := range f.PayerGroups {
		if g.Payer.PayerTIN == "" {
			return fmt.Errorf("payer_groups[%d].payer.tin is required", i)
		}
		if len(g.Payer.PayerTIN) != 9 {
			return fmt.Errorf("payer_groups[%d].payer.tin must be 9 digits", i)
		}
		if g.Payer.TypeOfReturn == "" {
			return fmt.Errorf("payer_groups[%d].payer.type_of_return is required", i)
		}
		if g.Payer.AmountCodes == "" {
			return fmt.Errorf("payer_groups[%d].payer.amount_codes is required", i)
		}
		if len(g.Payees) == 0 {
			return fmt.Errorf("payer_groups[%d] must have at least one payee", i)
		}
		for j, p := range g.Payees {
			if p.PayeeTIN == "" {
				return fmt.Errorf("payer_groups[%d].payees[%d].tin is required", i, j)
			}
			if len(p.PayeeTIN) != 9 {
				return fmt.Errorf("payer_groups[%d].payees[%d].tin must be 9 digits", i, j)
			}
		}
	}
	return nil
}

// parseAcknowledgment decodes the FIRE acknowledgment reply. FIRE
// returns HTML on the web upload and JSON on the API surface; the
// adapter sniffs the content.
func parseAcknowledgment(body []byte) (*Acknowledgment, error) {
	if len(body) == 0 {
		return &Acknowledgment{Status: "Good"}, nil
	}
	s := strings.TrimSpace(string(body))
	if strings.HasPrefix(s, "{") {
		return parseJSONAck(body)
	}
	// HTML path — extract the Filename and FileStatus tokens FIRE
	// renders in the success-page output.
	ack := &Acknowledgment{}
	if i := strings.Index(s, "Filename:"); i >= 0 {
		rest := strings.TrimSpace(s[i+len("Filename:"):])
		if nl := strings.IndexAny(rest, "\r\n<"); nl > 0 {
			ack.Filename = strings.TrimSpace(rest[:nl])
		}
	}
	if i := strings.Index(s, "Status:"); i >= 0 {
		rest := strings.TrimSpace(s[i+len("Status:"):])
		if nl := strings.IndexAny(rest, "\r\n<"); nl > 0 {
			ack.Status = strings.TrimSpace(rest[:nl])
		}
	}
	if ack.Status == "" {
		ack.Status = "Good"
	}
	return ack, nil
}

func parseJSONAck(body []byte) (*Acknowledgment, error) {
	type wire struct {
		Filename    string   `json:"filename"`
		Status      string   `json:"status"`
		SubmittedAt string   `json:"submitted_at"`
		Errors      []string `json:"errors"`
		Messages    []string `json:"messages"`
	}
	var w wire
	if err := jsonUnmarshal(body, &w); err != nil {
		return nil, err
	}
	ack := &Acknowledgment{
		Filename: w.Filename,
		Status:   w.Status,
		Errors:   w.Errors,
		Messages: w.Messages,
	}
	if w.SubmittedAt != "" {
		if t, err := time.Parse(time.RFC3339, w.SubmittedAt); err == nil {
			ack.SubmittedAt = t
		}
	}
	return ack, nil
}
