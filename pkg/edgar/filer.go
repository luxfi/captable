// Package edgar — SEC EDGAR Form D filing adapter.
//
// Implements the EDGAR Form D submission flow per the SEC's EDGAR
// Filer Manual Volume II §3 (Submission Header) and the Form D XML
// primaryDocument schema. Submissions are wrapped in a SUBMISSION
// SGML header that names the FORM-TYPE, CIK, ACCESSION-NUMBER stub,
// SROS, CONFIRMING-COPY policy, and CCC authentication, then carry
// the Form D XML payload as a DOCUMENT body of TYPE D and primary
// document name "primary_doc.xml".
//
// The package speaks the EDGARLink Online HTTP submission protocol —
// multipart/form-data POST against the filer endpoint, response is a
// SUBMISSION-ID acknowledgment, follow-up status via the EDGAR filing
// status endpoint.
//
// Source-of-design: Public-Spec
// Source-ref: https://www.sec.gov/info/edgar/specifications/formdxml.pdf
// Source-ref: https://www.sec.gov/edgar/filer-information/current-edgar-technical-specifications
package edgar

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"math/rand"
	"mime/multipart"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"time"
)

// Environment base URLs for the EDGAR filer surface. The production
// surface is the EDGARLink Online filer; the test surface mirrors it
// at filermanagement.edgarfiling.sec.gov for filer-management actions
// and at edgarfiling.sec.gov for filing submissions. Per the EDGAR
// Filer Manual Volume I §3.1.1 and §6.
const (
	// ProdURL is the EDGARLink Online production submission endpoint.
	ProdURL = "https://www.edgarfiling.sec.gov"

	// TestURL is the EDGARLink Online test/sandbox submission endpoint.
	TestURL = "https://filermanagement.edgarfiling.sec.gov"
)

// Errors surfaced by the adapter. Stable; callers may match with
// errors.Is.
var (
	// ErrMissingCIK is returned when a method requires a CIK and none
	// was provided.
	ErrMissingCIK = errors.New("edgar: CIK is required")

	// ErrMissingCCC is returned when a method requires a CCC and none
	// was provided.
	ErrMissingCCC = errors.New("edgar: CCC is required")

	// ErrInvalidFormD is returned when a FormDFiling struct fails
	// validation before submission.
	ErrInvalidFormD = errors.New("edgar: form D validation failed")

	// ErrSubmissionRejected is returned when EDGAR rejects a submission
	// outright (non-2xx response with a parsable rejection body). The
	// wrapped *APIError carries the SEC-reported reason.
	ErrSubmissionRejected = errors.New("edgar: submission rejected")

	// ErrAccessionNotFound is returned by GetFilingStatus when EDGAR
	// reports no filing for the supplied accession number.
	ErrAccessionNotFound = errors.New("edgar: accession not found")

	// ErrRateLimited is returned when EDGAR sustains 429 / 503 across
	// the configured retry budget.
	ErrRateLimited = errors.New("edgar: rate limited after retry budget exhausted")
)

// APIError is the structured error returned when EDGAR replies with a
// non-2xx status. Callers may use errors.As to recover it.
type APIError struct {
	StatusCode int
	Endpoint   string
	Body       string
	RetryAfter time.Duration
}

// Error implements the error interface.
func (e *APIError) Error() string {
	return fmt.Sprintf("edgar API %d on %s: %s", e.StatusCode, e.Endpoint, e.Body)
}

// Config holds the per-issuer authentication and tuning for the EDGAR
// filer. CIK and CCC are SEC-assigned identifiers issued through the
// EDGAR Filer Management System; one CIK per filer entity, with the
// CCC ("CIK Confirmation Code") rotating periodically. Both are
// loaded from KMS, never logged.
type Config struct {
	// BaseURL is one of ProdURL or TestURL. Required.
	BaseURL string

	// CIK is the SEC Central Index Key for the filer. Ten-digit,
	// zero-padded (e.g., "0001234567"). The adapter normalizes shorter
	// numeric inputs by left-padding with zeros.
	CIK string

	// CCC is the eight-character CIK Confirmation Code paired with
	// the CIK for filer authentication. Required for every
	// submission.
	CCC string

	// PasswordHashOrAPIKey is the FilerID password hash or EDGARLink
	// API key used as the secondary auth factor on the submission
	// endpoint. Optional for non-authenticated test submissions but
	// required for production.
	PasswordHashOrAPIKey string

	// ContactEmail is included in the EDGAR submission header so the
	// SEC can reach the filer for resubmission requests. Required.
	ContactEmail string

	// UserAgent is the value sent on every outbound request. The SEC
	// requires a descriptive User-Agent on automated EDGAR access per
	// the "EDGAR Sample Code" guidance — typically
	// "<Company Name> <Contact Email>". Required.
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
}

// Filer is the EDGAR Form D submission client. Construct one per
// issuer-CIK; the CIK + CCC pair authenticates every submission.
type Filer struct {
	cfg    Config
	client *http.Client
	rand   *rand.Rand
}

// NewFiler constructs a Filer. CIK and CCC are mandatory; if either
// is empty, methods will return ErrMissingCIK / ErrMissingCCC at the
// call site rather than at construction time so tests can override
// later if needed.
func NewFiler(cfg Config) *Filer {
	if cfg.BaseURL == "" {
		cfg.BaseURL = ProdURL
	}
	if cfg.HTTPClient == nil {
		cfg.HTTPClient = &http.Client{Timeout: 30 * time.Second}
	}
	if cfg.MaxRetries == 0 {
		cfg.MaxRetries = 4
	}
	if cfg.RetryBaseDelay == 0 {
		cfg.RetryBaseDelay = 500 * time.Millisecond
	}
	if cfg.RetryMaxDelay == 0 {
		cfg.RetryMaxDelay = 30 * time.Second
	}
	cfg.CIK = normalizeCIK(cfg.CIK)
	return &Filer{
		cfg:    cfg,
		client: cfg.HTTPClient,
		rand:   rand.New(rand.NewSource(time.Now().UnixNano())),
	}
}

// CIK returns the configured CIK (zero-padded to ten digits).
func (f *Filer) CIK() string { return f.cfg.CIK }

// FileFormD submits a new Form D filing to EDGAR. Returns an
// Acknowledgment carrying the EDGAR-assigned submission identifier
// (and accession number, once assigned).
//
// The submission is built by:
//
//   1. Validating the FormDFiling struct against the public Form D
//      schema constraints.
//   2. Marshaling the FormDFiling to the SEC's Form D XML
//      primaryDocument shape.
//   3. Wrapping the XML in a SUBMISSION SGML header per Filer Manual
//      Volume II §3.
//   4. POSTing the multipart submission to the EDGARLink Online
//      submission endpoint.
//   5. Parsing the EDGAR acknowledgment payload.
func (f *Filer) FileFormD(ctx context.Context, fd *FormDFiling) (*Acknowledgment, error) {
	if f.cfg.CIK == "" {
		return nil, ErrMissingCIK
	}
	if f.cfg.CCC == "" {
		return nil, ErrMissingCCC
	}
	if fd.SubmissionType == "" {
		fd.SubmissionType = SubmissionFormD
	}
	fd.IsAmendment = (fd.SubmissionType == SubmissionFormDA)
	if err := validateFormD(fd, false); err != nil {
		return nil, fmt.Errorf("%w: %v", ErrInvalidFormD, err)
	}
	return f.submit(ctx, fd)
}

// AmendFormD submits an amendment to a previously filed Form D. The
// FileNumber field on the FormDFiling must be populated with the SEC
// file number from the original filing.
func (f *Filer) AmendFormD(ctx context.Context, fd *FormDFiling) (*Acknowledgment, error) {
	if f.cfg.CIK == "" {
		return nil, ErrMissingCIK
	}
	if f.cfg.CCC == "" {
		return nil, ErrMissingCCC
	}
	fd.SubmissionType = SubmissionFormDA
	fd.IsAmendment = true
	if err := validateFormD(fd, true); err != nil {
		return nil, fmt.Errorf("%w: %v", ErrInvalidFormD, err)
	}
	return f.submit(ctx, fd)
}

// GetFilingStatus retrieves the current status of a previously
// submitted filing. Status is resolved via the EDGAR filing-status
// endpoint, keyed by the accession number EDGAR assigned at receipt.
func (f *Filer) GetFilingStatus(ctx context.Context, accessionNumber string) (*FilingStatus, error) {
	if f.cfg.CIK == "" {
		return nil, ErrMissingCIK
	}
	if accessionNumber == "" {
		return nil, errors.New("edgar: accession number is required")
	}
	endpoint := fmt.Sprintf("/cgi-bin/edgarstatus?accession=%s&CIK=%s", accessionNumber, f.cfg.CIK)
	body, err := f.doGet(ctx, endpoint)
	if err != nil {
		var apiErr *APIError
		if errors.As(err, &apiErr) && apiErr.StatusCode == http.StatusNotFound {
			return nil, ErrAccessionNotFound
		}
		return nil, err
	}
	return parseStatusReply(body)
}

// --- submission pipeline ---

// submit serializes, wraps, and POSTs a FormDFiling.
func (f *Filer) submit(ctx context.Context, fd *FormDFiling) (*Acknowledgment, error) {
	primaryXML, err := marshalFormD(fd)
	if err != nil {
		return nil, fmt.Errorf("edgar: marshal form D: %w", err)
	}
	submission := buildSGMLSubmission(f.cfg, fd, primaryXML)
	return f.postSubmission(ctx, submission, primaryXML)
}

// postSubmission performs a multipart/form-data POST to the EDGAR
// submission endpoint. The SGML envelope rides as form field
// "submission" alongside a single "primary_doc.xml" file part.
func (f *Filer) postSubmission(ctx context.Context, submission []byte, primaryXML []byte) (*Acknowledgment, error) {
	endpoint := "/cgi-bin/edgarsubmit"
	var buf bytes.Buffer
	w := multipart.NewWriter(&buf)

	if err := w.WriteField("submission", string(submission)); err != nil {
		return nil, fmt.Errorf("edgar: write submission field: %w", err)
	}
	if err := w.WriteField("CIK", f.cfg.CIK); err != nil {
		return nil, fmt.Errorf("edgar: write cik field: %w", err)
	}
	if err := w.WriteField("CCC", f.cfg.CCC); err != nil {
		return nil, fmt.Errorf("edgar: write ccc field: %w", err)
	}
	docPart, err := w.CreateFormFile("primary_doc.xml", "primary_doc.xml")
	if err != nil {
		return nil, fmt.Errorf("edgar: create primary doc part: %w", err)
	}
	if _, err := docPart.Write(primaryXML); err != nil {
		return nil, fmt.Errorf("edgar: write primary doc: %w", err)
	}
	if err := w.Close(); err != nil {
		return nil, fmt.Errorf("edgar: close multipart writer: %w", err)
	}

	doOnce := func() (*http.Response, []byte, error) {
		req, err := http.NewRequestWithContext(ctx, http.MethodPost, f.cfg.BaseURL+endpoint, bytes.NewReader(buf.Bytes()))
		if err != nil {
			return nil, nil, err
		}
		req.Header.Set("Content-Type", w.FormDataContentType())
		req.Header.Set("Accept", "application/xml")
		req.Header.Set("User-Agent", f.cfg.UserAgent)
		resp, err := f.client.Do(req)
		if err != nil {
			return nil, nil, err
		}
		defer resp.Body.Close()
		body, err := io.ReadAll(resp.Body)
		return resp, body, err
	}

	body, err := f.doWithRetry(ctx, endpoint, doOnce)
	if err != nil {
		return nil, err
	}
	ack, err := parseAcknowledgment(body)
	if err != nil {
		return nil, fmt.Errorf("edgar: parse acknowledgment: %w", err)
	}
	if ack.Status == "REJECTED" {
		return ack, fmt.Errorf("%w: %s", ErrSubmissionRejected, strings.Join(ack.Messages, "; "))
	}
	return ack, nil
}

// doGet executes a simple GET against an EDGAR endpoint with retry.
func (f *Filer) doGet(ctx context.Context, path string) ([]byte, error) {
	doOnce := func() (*http.Response, []byte, error) {
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, f.cfg.BaseURL+path, nil)
		if err != nil {
			return nil, nil, err
		}
		req.Header.Set("Accept", "application/xml")
		req.Header.Set("User-Agent", f.cfg.UserAgent)
		resp, err := f.client.Do(req)
		if err != nil {
			return nil, nil, err
		}
		defer resp.Body.Close()
		body, err := io.ReadAll(resp.Body)
		return resp, body, err
	}
	return f.doWithRetry(ctx, path, doOnce)
}

// doWithRetry implements the standard 429 / 5xx retry loop with
// exponential backoff + full jitter and Retry-After honoring.
func (f *Filer) doWithRetry(ctx context.Context, path string, fn func() (*http.Response, []byte, error)) ([]byte, error) {
	var lastBody []byte
	var lastStatus int
	var lastRetryAfter time.Duration

	for attempt := 0; attempt <= f.cfg.MaxRetries; attempt++ {
		resp, body, err := fn()
		if err != nil {
			if attempt == f.cfg.MaxRetries {
				return nil, err
			}
			if waitErr := f.sleepBackoff(ctx, attempt, 0); waitErr != nil {
				return nil, waitErr
			}
			continue
		}
		lastBody = body
		lastStatus = resp.StatusCode

		if resp.StatusCode == http.StatusTooManyRequests ||
			(resp.StatusCode >= 500 && resp.StatusCode != http.StatusNotImplemented) {
			lastRetryAfter = parseRetryAfter(resp.Header.Get("Retry-After"))
			if attempt == f.cfg.MaxRetries {
				break
			}
			if waitErr := f.sleepBackoff(ctx, attempt, lastRetryAfter); waitErr != nil {
				return nil, waitErr
			}
			continue
		}
		if resp.StatusCode >= 400 {
			return nil, &APIError{
				StatusCode: resp.StatusCode,
				Endpoint:   path,
				Body:       string(body),
			}
		}
		return body, nil
	}

	apiErr := &APIError{
		StatusCode: lastStatus,
		Endpoint:   path,
		Body:       string(lastBody),
		RetryAfter: lastRetryAfter,
	}
	if lastStatus == http.StatusTooManyRequests {
		return nil, fmt.Errorf("%w: %s", ErrRateLimited, apiErr.Error())
	}
	return nil, apiErr
}

// sleepBackoff waits between retries. If retryAfter is non-zero (from
// the server's Retry-After header), it is honored exactly; otherwise
// exponential backoff with full jitter is applied.
func (f *Filer) sleepBackoff(ctx context.Context, attempt int, retryAfter time.Duration) error {
	var wait time.Duration
	if retryAfter > 0 {
		wait = retryAfter
	} else {
		exp := f.cfg.RetryBaseDelay << attempt
		if exp <= 0 || exp > f.cfg.RetryMaxDelay {
			exp = f.cfg.RetryMaxDelay
		}
		wait = time.Duration(f.rand.Int63n(int64(exp) + 1))
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

// normalizeCIK left-pads a numeric CIK string with zeros up to ten
// digits. Non-numeric input is returned unchanged so the validator can
// surface a clear error.
func normalizeCIK(cik string) string {
	cik = strings.TrimSpace(cik)
	if cik == "" {
		return ""
	}
	if !cikDigits.MatchString(cik) {
		return cik
	}
	if len(cik) >= 10 {
		return cik
	}
	return strings.Repeat("0", 10-len(cik)) + cik
}

var cikDigits = regexp.MustCompile(`^[0-9]+$`)
