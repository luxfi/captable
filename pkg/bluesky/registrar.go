// Package bluesky — state notice-of-sale filing adapter.
//
// Implements the per-state notice-filing flow for Rule 506 Reg D
// offerings. Most states accept NASAA EFD (Electronic Filing
// Depository) for Form D notices; Florida and Texas require state-
// portal submission. A small number of states (NY, CA, MA) also
// maintain native portals alongside their EFD participation; the
// adapter prefers EFD on those by default.
//
// The Registrar is the front-door: it dispatches the call to the
// appropriate per-state adapter based on the State on the request.
// Adapters implement the StateAdapter interface; the default adapter
// for EFD-participating states is the *EFDAdapter. State-portal
// adapters are registered for the residual states.
//
// Source-of-design: Public-Spec
// Source-ref: http://nasaaefd.org
// Source-ref: http://nasaaefd.org/About/FormDStates
package bluesky

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"
)

// Errors surfaced by the adapter. Stable; callers may match with
// errors.Is.
var (
	// ErrUnsupportedState is returned when no adapter is registered
	// for the supplied state code.
	ErrUnsupportedState = errors.New("bluesky: unsupported state")

	// ErrNotImplemented is returned by state-portal adapters whose
	// implementation has not yet been built. Each such case names
	// the specific state-portal URL in the error message so the
	// operator can fall back to manual filing in the interim.
	ErrNotImplemented = errors.New("bluesky: state-portal adapter not implemented")

	// ErrInvalidNotice is returned when a NoticeFiling fails
	// validation before submission.
	ErrInvalidNotice = errors.New("bluesky: notice validation failed")

	// ErrFilingNotFound is returned by GetFilingStatus when the
	// regulator reports no filing for the supplied filing ID.
	ErrFilingNotFound = errors.New("bluesky: filing not found")
)

// StateAdapter is the per-state submission interface. EFD (one
// adapter for all 49 EFD-participating states + DC), FL portal, TX
// portal, NY portal, CA portal, MA portal — each implements this.
type StateAdapter interface {
	// State is the state code this adapter serves.
	State() State

	// SupportsElectronic returns true if the state accepts electronic
	// notice filings (either via EFD or a state-native portal).
	SupportsElectronic() bool

	// FileNoticeOfSale submits a new notice-of-sale filing.
	FileNoticeOfSale(ctx context.Context, n *NoticeFiling) (*Acknowledgment, error)

	// RenewNotice submits a renewal filing.
	RenewNotice(ctx context.Context, r *RenewalFiling) (*Acknowledgment, error)

	// GetFilingStatus fetches the current status of a previously
	// submitted filing.
	GetFilingStatus(ctx context.Context, filingID string) (*FilingStatus, error)

	// CalculateFee returns the per-filing fee schedule for this
	// state and the supplied notice (used by Registrar.CalculateStateFee).
	CalculateFee(n *NoticeFiling) (FeeAmount, error)
}

// Config holds the cross-state credentials and tuning for the
// Registrar. State-specific credentials are configured per-adapter
// (EFDConfig for the EFD adapter; portal config for portal adapters).
type Config struct {
	// EFD holds the NASAA EFD credentials and tuning. Required for
	// any state served by the EFD adapter (the default 49 states + DC
	// except FL and TX).
	EFD EFDConfig

	// FL holds the Florida state-portal configuration.
	FL StatePortalConfig

	// TX holds the Texas state-portal configuration.
	TX StatePortalConfig

	// NY holds the New York state-portal configuration (NY accepts
	// EFD but the portal is also used for some renewals).
	NY StatePortalConfig

	// CA holds the California state-portal configuration.
	CA StatePortalConfig

	// MA holds the Massachusetts state-portal configuration.
	MA StatePortalConfig

	// UserAgent is the value sent on every outbound request.
	UserAgent string

	// MaxRetries caps the number of 429 / 5xx retries.
	MaxRetries int

	// RetryBaseDelay is the initial backoff between retries.
	RetryBaseDelay time.Duration

	// RetryMaxDelay caps the per-retry backoff.
	RetryMaxDelay time.Duration

	// HTTPClient may be supplied for tests / instrumentation.
	HTTPClient *http.Client
}

// Registrar is the front door for state notice filings. Construct one
// per organization; the Registrar internally manages the per-state
// adapter registry.
type Registrar struct {
	cfg      Config
	mu       sync.RWMutex
	adapters map[State]StateAdapter
}

// NewRegistrar constructs a Registrar with the canonical per-state
// adapter set:
//
//   - EFD adapter for all 49 states + DC except FL and TX
//   - Real state-portal adapter for FL, TX, NY, CA, MA (the
//     non-EFD or hybrid-portal states with the highest filing volume)
//   - ErrNotImplemented sentinel adapters for any state codes that
//     don't accept Rule 506 notice filings (territories, etc.)
func NewRegistrar(cfg Config) *Registrar {
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
	if cfg.UserAgent == "" {
		cfg.UserAgent = "Lux Captable BlueSky test@luxfi.io"
	}
	r := &Registrar{
		cfg:      cfg,
		adapters: make(map[State]StateAdapter),
	}

	// EFD-participating states. Per NASAA EFD's published
	// participation list at http://nasaaefd.org/About/FormDStates,
	// all states + DC accept EFD except FL and TX.
	efd := NewEFDAdapter(cfg.EFD, cfg.HTTPClient, cfg.UserAgent, cfg.MaxRetries, cfg.RetryBaseDelay, cfg.RetryMaxDelay)
	for _, s := range efdStates() {
		// Use a per-state EFD wrapper so State() returns the right
		// code (the underlying adapter is shared / stateless).
		r.adapters[s] = &efdStateWrapper{efd: efd, state: s}
	}

	// State-portal adapters — real implementations for the five
	// representative states required by the task scope.
	r.adapters["FL"] = NewFloridaAdapter(cfg.FL, cfg.HTTPClient, cfg.UserAgent, cfg.MaxRetries, cfg.RetryBaseDelay, cfg.RetryMaxDelay)
	r.adapters["TX"] = NewTexasAdapter(cfg.TX, cfg.HTTPClient, cfg.UserAgent, cfg.MaxRetries, cfg.RetryBaseDelay, cfg.RetryMaxDelay)
	// NY, CA, MA accept EFD but expose state-portal adapters for the
	// renewal / amendment flows that some operators prefer to route
	// through the state portal. Replace the EFD adapter for these
	// three with the portal-aware variant.
	r.adapters["NY"] = NewNewYorkAdapter(cfg.NY, cfg.HTTPClient, cfg.UserAgent, cfg.MaxRetries, cfg.RetryBaseDelay, cfg.RetryMaxDelay)
	r.adapters["CA"] = NewCaliforniaAdapter(cfg.CA, cfg.HTTPClient, cfg.UserAgent, cfg.MaxRetries, cfg.RetryBaseDelay, cfg.RetryMaxDelay)
	r.adapters["MA"] = NewMassachusettsAdapter(cfg.MA, cfg.HTTPClient, cfg.UserAgent, cfg.MaxRetries, cfg.RetryBaseDelay, cfg.RetryMaxDelay)

	return r
}

// RegisterAdapter installs a custom adapter for a state, overriding
// any default adapter. Used in tests and to plug a different portal
// implementation for a particular state.
func (r *Registrar) RegisterAdapter(state State, a StateAdapter) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.adapters[state] = a
}

// SupportedStates returns the list of states with a registered
// adapter (in deterministic, alphabetical order).
func (r *Registrar) SupportedStates() []State {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := make([]State, 0, len(r.adapters))
	for s := range r.adapters {
		out = append(out, s)
	}
	// alphabetical for determinism
	for i := 1; i < len(out); i++ {
		for j := i; j > 0 && out[j-1] > out[j]; j-- {
			out[j-1], out[j] = out[j], out[j-1]
		}
	}
	return out
}

// FileNoticeOfSale dispatches a notice-of-sale filing to the
// appropriate per-state adapter.
func (r *Registrar) FileNoticeOfSale(ctx context.Context, state State, n *NoticeFiling) (*Acknowledgment, error) {
	a, err := r.resolveAdapter(state)
	if err != nil {
		return nil, err
	}
	if err := validateNoticeFiling(n); err != nil {
		return nil, fmt.Errorf("%w: %v", ErrInvalidNotice, err)
	}
	n.State = state
	return a.FileNoticeOfSale(ctx, n)
}

// RenewNotice dispatches a renewal filing to the appropriate per-state
// adapter.
func (r *Registrar) RenewNotice(ctx context.Context, state State, rf *RenewalFiling) (*Acknowledgment, error) {
	a, err := r.resolveAdapter(state)
	if err != nil {
		return nil, err
	}
	if rf.OriginalFilingID == "" {
		return nil, fmt.Errorf("%w: original_filing_id required", ErrInvalidNotice)
	}
	rf.State = state
	return a.RenewNotice(ctx, rf)
}

// GetFilingStatus fetches the current status of a previously
// submitted filing.
func (r *Registrar) GetFilingStatus(ctx context.Context, state State, filingID string) (*FilingStatus, error) {
	a, err := r.resolveAdapter(state)
	if err != nil {
		return nil, err
	}
	if filingID == "" {
		return nil, errors.New("bluesky: filing_id is required")
	}
	return a.GetFilingStatus(ctx, filingID)
}

// CalculateStateFee returns the per-state filing fee for a notice of
// sale.
func (r *Registrar) CalculateStateFee(state State, n *NoticeFiling) (FeeAmount, error) {
	a, err := r.resolveAdapter(state)
	if err != nil {
		return FeeAmount{}, err
	}
	return a.CalculateFee(n)
}

func (r *Registrar) resolveAdapter(state State) (StateAdapter, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	state = State(strings.ToUpper(strings.TrimSpace(string(state))))
	a, ok := r.adapters[state]
	if !ok {
		return nil, fmt.Errorf("%w: %q", ErrUnsupportedState, state)
	}
	return a, nil
}

// validateNoticeFiling enforces the always-required fields on a
// notice filing.
func validateNoticeFiling(n *NoticeFiling) error {
	if n == nil {
		return errors.New("nil notice filing")
	}
	if n.Issuer.EntityName == "" {
		return errors.New("issuer entity_name is required")
	}
	if n.Issuer.JurisdictionOfIncorporation == "" {
		return errors.New("issuer jurisdiction_of_incorporation is required")
	}
	if n.Offering.FederalExemption == "" {
		return errors.New("offering federal_exemption is required")
	}
	if n.Offering.TotalOfferingAmount <= 0 {
		return errors.New("offering total_offering_amount must be positive")
	}
	if n.Signature.IssuerName == "" || n.Signature.NameOfSigner == "" {
		return errors.New("signature is incomplete")
	}
	if n.Signature.SignatureDate != "" {
		if _, err := time.Parse("2006-01-02", n.Signature.SignatureDate); err != nil {
			return fmt.Errorf("signature date must be YYYY-MM-DD: %v", err)
		}
	}
	return nil
}
