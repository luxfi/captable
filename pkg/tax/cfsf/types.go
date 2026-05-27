// Package cfsf — Combined Federal/State Filing (CFSF) routing
// helpers for the 1099 series. CFSF lets a federal IRS filing
// auto-route to participating states without a separate state-by-state
// filing. The state's IRS-published participation list (Publication
// 1220 Part A §10) determines whether the federal filing satisfies
// the state's recordkeeping obligation.
//
// This package does NOT submit to states directly — for participating
// states, the federal IRIS / FIRE filing carries the CFSF indicator
// and the IRS forwards the per-state slice. For non-participating
// states, the package emits a StateFiling stub naming the
// per-state-DOR portal that the caller must file against directly.
//
// Source-of-design: Public-Spec
// Source-ref: IRS Publication 1220 Part A §10 — Combined Federal/State Filing Program
// Source-ref: https://www.irs.gov/pub/irs-pdf/p1220.pdf (Table 1 of §10 lists participating states + state codes)
package cfsf

import (
	"github.com/luxfi/captable/pkg/tax/iris"
)

// State is a USPS two-letter state code (plus "DC" and US territory
// codes) used to scope the CFSF routing decision.
type State string

// CFSFStateCode is the two-digit IRS-assigned numeric code for a
// CFSF-participating state. Used in the Combined Federal/State Code
// field on FIRE K records and as the StateCd on IRIS state info
// blocks.
type CFSFStateCode string

// PortalKind identifies the per-state-DOR filing surface for non-CFSF
// states.
type PortalKind string

const (
	// PortalDORWebsite is the DOR's web portal upload (typical for
	// non-CFSF states).
	PortalDORWebsite PortalKind = "dor_website"

	// PortalFSET is the FSET (Federation of Tax Administrators)
	// XML-over-MIME schema (used by several states including IL and
	// PA when their portals accept FSET payloads).
	PortalFSET PortalKind = "fset"

	// PortalNone marks a state with no e-file requirement at the
	// threshold the filer is at (low-volume safe harbor).
	PortalNone PortalKind = "none"
)

// StateProfile captures the state's CFSF participation status and the
// non-CFSF fallback portal information.
type StateProfile struct {
	// State is the USPS state code.
	State State

	// ParticipatesInCFSF indicates whether the IRS forwards the
	// federal 1099 filing to this state under the CFSF program.
	ParticipatesInCFSF bool

	// CFSFCode is the IRS-assigned two-digit numeric state code,
	// populated only for CFSF participants.
	CFSFCode CFSFStateCode

	// FallbackPortal identifies the non-CFSF filing surface for
	// non-participants.
	FallbackPortal PortalKind

	// FallbackURL is the URL or human-readable identifier of the
	// fallback portal for operator handoff (empty for CFSF
	// participants and for PortalNone states).
	FallbackURL string

	// FormThreshold (per state) — number of forms above which the
	// state requires e-filing. Zero means the state has no separate
	// threshold (defers to federal).
	FormThreshold int
}

// StateFiling is the per-state filing output of RouteToStates.
// CFSF-participant routings are pure metadata (the federal filing
// already satisfies the state's recordkeeping); non-participant
// routings carry the per-state filing instructions for the operator
// to action.
type StateFiling struct {
	// State is the USPS state code.
	State State

	// RoutingType is one of: "cfsf" (federal filing satisfies),
	// "direct" (operator must file at the state portal), "none"
	// (state has no separate filing requirement at this volume).
	RoutingType string

	// CFSFCode is the IRS-assigned two-digit state code; populated
	// for "cfsf" routings.
	CFSFCode CFSFStateCode

	// Portal identifies the non-CFSF portal for "direct" routings.
	Portal PortalKind

	// PortalURL identifies the per-state-DOR portal URL for "direct"
	// routings — the operator hand-off target.
	PortalURL string

	// PayeeCount is the count of payee records routed to this state.
	PayeeCount int

	// TotalStateTaxWithheld is the sum of state-income-tax-withheld
	// across all routed payees for this state (in cents).
	TotalStateTaxWithheld int64

	// TotalStateIncome is the sum of state-income across all routed
	// payees for this state (in cents).
	TotalStateIncome int64

	// Notes carries human-readable operator notes — e.g., the per-
	// state e-file threshold, the per-state due date, the per-state
	// account-registration prerequisite.
	Notes string
}

// FormSubmissionForRouting is a thin wrapper accepted by
// RouteToStates. It is satisfied by *iris.FormSubmission and is kept
// distinct so callers may also route FIRE-side submissions through
// the same engine.
type FormSubmissionForRouting interface {
	// StateBreakdown returns a map of USPS state code to the slice of
	// per-payee (TaxWithheld cents, Income cents) pairs that the
	// federal filing carries for that state. Used to compute the per-
	// state aggregates the StateFiling exposes.
	StateBreakdown() map[State][]StateAmount
}

// StateAmount is a single payee's per-state amounts (in cents).
type StateAmount struct {
	TaxWithheld int64
	Income      int64
}

// IRISWrapper adapts an *iris.FormSubmission to
// FormSubmissionForRouting by reading the StateCd field from each
// payee's typed Form1099*Data.
type IRISWrapper struct {
	FS *iris.FormSubmission
}

// StateBreakdown implements FormSubmissionForRouting on an
// *iris.FormSubmission by reading the StateCd / StateTaxWithheld /
// StateIncome fields from each payee's typed data.
func (w IRISWrapper) StateBreakdown() map[State][]StateAmount {
	out := make(map[State][]StateAmount)
	if w.FS == nil {
		return out
	}
	for _, p := range w.FS.Payees {
		st, tax, inc := stateAmountsFromIRIS(p.Data)
		if st == "" {
			continue
		}
		out[State(st)] = append(out[State(st)], StateAmount{
			TaxWithheld: dollarsToCents(tax),
			Income:      dollarsToCents(inc),
		})
	}
	return out
}

// stateAmountsFromIRIS pulls (StateCd, StateTaxWithheld, StateIncome)
// from a Form1099*Data value. Returns ("", 0, 0) if the data type is
// nil or unknown.
func stateAmountsFromIRIS(data any) (state string, tax, income float64) {
	switch d := data.(type) {
	case *iris.Form1099DIVData:
		return d.StateCd, d.StateTaxWithheld, d.StateIncome
	case iris.Form1099DIVData:
		return d.StateCd, d.StateTaxWithheld, d.StateIncome
	case *iris.Form1099BData:
		return d.StateCd, d.StateTaxWithheld, 0
	case iris.Form1099BData:
		return d.StateCd, d.StateTaxWithheld, 0
	case *iris.Form1099INTData:
		return d.StateCd, d.StateTaxWithheld, 0
	case iris.Form1099INTData:
		return d.StateCd, d.StateTaxWithheld, 0
	case *iris.Form1099MISCData:
		return d.StateCd, d.StateTaxWithheld, d.StateIncome
	case iris.Form1099MISCData:
		return d.StateCd, d.StateTaxWithheld, d.StateIncome
	case *iris.Form1099NECData:
		return d.StateCd, d.StateTaxWithheld, d.StateIncome
	case iris.Form1099NECData:
		return d.StateCd, d.StateTaxWithheld, d.StateIncome
	case *iris.Form1099OIDData:
		return d.StateCd, d.StateTaxWithheld, 0
	case iris.Form1099OIDData:
		return d.StateCd, d.StateTaxWithheld, 0
	case *iris.Form1099KData:
		return d.StateCd, d.StateTaxWithheld, d.StateIncome
	case iris.Form1099KData:
		return d.StateCd, d.StateTaxWithheld, d.StateIncome
	case *iris.Form1099RData:
		return d.StateCd, d.StateTaxWithheld, d.StateDistribution
	case iris.Form1099RData:
		return d.StateCd, d.StateTaxWithheld, d.StateDistribution
	}
	return "", 0, 0
}

func dollarsToCents(v float64) int64 {
	return int64(v*100 + 0.5)
}
