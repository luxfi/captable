// CFSF routing engine — given a federal 1099 submission, produce the
// per-state filing decisions (CFSF-satisfied / direct-file / no-tax
// state).
//
// Source-of-design: Public-Spec
// Source-ref: IRS Publication 1220 Part A §10
package cfsf

import (
	"fmt"
	"sort"
)

// RouteToStates produces the per-state filing decisions for a federal
// 1099 submission. Each requested state appears in the output:
//
//   - CFSF participant: routing_type="cfsf", CFSFCode populated; the
//     federal IRIS/FIRE filing carries the per-payee state slice and
//     the IRS auto-routes to the state. No direct-file action needed.
//
//   - Non-CFSF state with a DOR portal: routing_type="direct", Portal
//     and PortalURL populated; the operator must hand-file (CSV upload
//     or FSET) at the named portal.
//
//   - Non-CFSF state with no broad state income tax (FL/TX/NV/etc.):
//     routing_type="none"; no action.
//
// If the requested states slice is empty, the function infers states
// from the submission's StateBreakdown — any state that appears in the
// federal payee data is routed automatically.
func RouteToStates(fs FormSubmissionForRouting, states []State) ([]*StateFiling, error) {
	if fs == nil {
		return nil, fmt.Errorf("cfsf: submission is nil")
	}
	breakdown := fs.StateBreakdown()
	if len(states) == 0 {
		states = make([]State, 0, len(breakdown))
		for s := range breakdown {
			states = append(states, s)
		}
		sort.Slice(states, func(i, j int) bool { return states[i] < states[j] })
	}

	out := make([]*StateFiling, 0, len(states))
	for _, s := range states {
		filing := buildFiling(s, breakdown[s])
		out = append(out, filing)
	}
	return out, nil
}

// buildFiling produces a single StateFiling from a state code + the
// per-payee state breakdown for that state.
func buildFiling(s State, amounts []StateAmount) *StateFiling {
	prof, known := GetProfile(s)
	filing := &StateFiling{
		State:      s,
		PayeeCount: len(amounts),
	}
	for _, a := range amounts {
		filing.TotalStateTaxWithheld += a.TaxWithheld
		filing.TotalStateIncome += a.Income
	}
	if !known {
		filing.RoutingType = "direct"
		filing.Portal = PortalDORWebsite
		filing.Notes = fmt.Sprintf("state %s not in CFSF registry — verify per-state portal manually", s)
		return filing
	}
	switch {
	case prof.ParticipatesInCFSF:
		filing.RoutingType = "cfsf"
		filing.CFSFCode = prof.CFSFCode
		filing.Notes = fmt.Sprintf("federal filing auto-routes to %s under CFSF (state code %s)", s, prof.CFSFCode)
	case prof.FallbackPortal == PortalNone:
		filing.RoutingType = "none"
		filing.Notes = prof.FallbackURL
	default:
		filing.RoutingType = "direct"
		filing.Portal = prof.FallbackPortal
		filing.PortalURL = prof.FallbackURL
		if prof.FormThreshold > 0 && filing.PayeeCount < prof.FormThreshold {
			filing.Notes = fmt.Sprintf("below %s e-file threshold (%d forms); paper filing permitted",
				s, prof.FormThreshold)
		} else {
			filing.Notes = fmt.Sprintf("file directly at %s DOR portal (%s)", s, string(prof.FallbackPortal))
		}
	}
	return filing
}
