package cfsf

import (
	"testing"

	"github.com/luxfi/captable/pkg/tax/iris"
)

func TestRouteToStates_CFSFParticipants(t *testing.T) {
	fs := &iris.FormSubmission{
		FormType: iris.Form1099DIV,
		TaxYear:  2025,
		Payees: []iris.PayeeBlock{
			{
				Payee: iris.Payee{TIN: "111223333", TINType: "S", Name: "DOE JANE"},
				Data: &iris.Form1099DIVData{
					OrdinaryDividends: 5000.00,
					StateCd:           "CA",
					StateTaxWithheld:  250.00,
					StateIncome:       5000.00,
				},
			},
			{
				Payee: iris.Payee{TIN: "222334444", TINType: "S", Name: "ROE BOB"},
				Data: &iris.Form1099DIVData{
					OrdinaryDividends: 2000.00,
					StateCd:           "NY",
					StateTaxWithheld:  120.00,
					StateIncome:       2000.00,
				},
			},
		},
	}

	cases := []struct {
		state          State
		wantRoutingTyp string
		wantCFSFCode   CFSFStateCode
	}{
		{"CA", "cfsf", "06"},
		{"NY", "direct", ""}, // NY does not participate in CFSF
		{"TX", "none", ""},   // no state income tax
		{"MA", "cfsf", "25"},
		{"FL", "none", ""},
		{"DE", "cfsf", "10"},
		{"ID", "cfsf", "16"},
	}
	for _, tc := range cases {
		t.Run(string(tc.state), func(t *testing.T) {
			filings, err := RouteToStates(IRISWrapper{FS: fs}, []State{tc.state})
			if err != nil {
				t.Fatalf("RouteToStates: %v", err)
			}
			if len(filings) != 1 {
				t.Fatalf("len(filings) = %d, want 1", len(filings))
			}
			f := filings[0]
			if f.RoutingType != tc.wantRoutingTyp {
				t.Fatalf("routing_type = %q, want %q", f.RoutingType, tc.wantRoutingTyp)
			}
			if tc.wantCFSFCode != "" && f.CFSFCode != tc.wantCFSFCode {
				t.Fatalf("CFSF code = %q, want %q", f.CFSFCode, tc.wantCFSFCode)
			}
		})
	}
}

func TestRouteToStates_AutoInfer(t *testing.T) {
	fs := &iris.FormSubmission{
		FormType: iris.Form1099DIV,
		TaxYear:  2025,
		Payees: []iris.PayeeBlock{
			{Payee: iris.Payee{TIN: "111111111", TINType: "S", Name: "X"},
				Data: &iris.Form1099DIVData{OrdinaryDividends: 100, StateCd: "CA", StateTaxWithheld: 5, StateIncome: 100}},
			{Payee: iris.Payee{TIN: "222222222", TINType: "S", Name: "Y"},
				Data: &iris.Form1099DIVData{OrdinaryDividends: 100, StateCd: "MA", StateTaxWithheld: 5, StateIncome: 100}},
			{Payee: iris.Payee{TIN: "333333333", TINType: "S", Name: "Z"},
				Data: &iris.Form1099DIVData{OrdinaryDividends: 100, StateCd: "NY", StateTaxWithheld: 5, StateIncome: 100}},
		},
	}

	filings, err := RouteToStates(IRISWrapper{FS: fs}, nil)
	if err != nil {
		t.Fatalf("RouteToStates: %v", err)
	}
	if len(filings) != 3 {
		t.Fatalf("len(filings) = %d, want 3 (CA + MA + NY)", len(filings))
	}
}

func TestRouteToStates_AggregatesAmounts(t *testing.T) {
	fs := &iris.FormSubmission{
		FormType: iris.Form1099DIV,
		TaxYear:  2025,
		Payees: []iris.PayeeBlock{
			{Payee: iris.Payee{TIN: "111111111", TINType: "S", Name: "A"},
				Data: &iris.Form1099DIVData{StateCd: "CA", StateTaxWithheld: 100.00, StateIncome: 2000.00}},
			{Payee: iris.Payee{TIN: "222222222", TINType: "S", Name: "B"},
				Data: &iris.Form1099DIVData{StateCd: "CA", StateTaxWithheld: 50.00, StateIncome: 1000.00}},
		},
	}
	filings, err := RouteToStates(IRISWrapper{FS: fs}, []State{"CA"})
	if err != nil {
		t.Fatalf("RouteToStates: %v", err)
	}
	if len(filings) != 1 {
		t.Fatalf("len(filings) = %d", len(filings))
	}
	f := filings[0]
	if f.PayeeCount != 2 {
		t.Fatalf("PayeeCount = %d, want 2", f.PayeeCount)
	}
	if f.TotalStateTaxWithheld != 15000 { // (100 + 50) * 100 cents
		t.Fatalf("TotalStateTaxWithheld = %d, want 15000", f.TotalStateTaxWithheld)
	}
	if f.TotalStateIncome != 300000 { // (2000 + 1000) * 100 cents
		t.Fatalf("TotalStateIncome = %d, want 300000", f.TotalStateIncome)
	}
}

func TestRouteToStates_PortalsForNonCFSF(t *testing.T) {
	cases := []struct {
		state      State
		wantPortal PortalKind
	}{
		{"NY", PortalDORWebsite},
		{"IL", PortalFSET},
		{"PA", PortalFSET},
		{"OR", PortalDORWebsite},
		{"VA", PortalDORWebsite},
	}
	fs := &iris.FormSubmission{FormType: iris.Form1099DIV, TaxYear: 2025}
	for _, tc := range cases {
		t.Run(string(tc.state), func(t *testing.T) {
			filings, err := RouteToStates(IRISWrapper{FS: fs}, []State{tc.state})
			if err != nil {
				t.Fatalf("RouteToStates: %v", err)
			}
			if filings[0].Portal != tc.wantPortal {
				t.Fatalf("portal = %q, want %q", filings[0].Portal, tc.wantPortal)
			}
			if filings[0].PortalURL == "" {
				t.Fatalf("portal URL empty for %s", tc.state)
			}
		})
	}
}

func TestRouteToStates_UnknownState(t *testing.T) {
	fs := &iris.FormSubmission{FormType: iris.Form1099DIV, TaxYear: 2025}
	filings, err := RouteToStates(IRISWrapper{FS: fs}, []State{"ZZ"}) // unknown
	if err != nil {
		t.Fatalf("RouteToStates: %v", err)
	}
	if filings[0].RoutingType != "direct" {
		t.Fatalf("routing_type = %q, want direct", filings[0].RoutingType)
	}
	if filings[0].Notes == "" {
		t.Fatalf("notes empty for unknown state")
	}
}

func TestRegisteredStateCount(t *testing.T) {
	cfsf, non, total := RegisteredStateCount()
	if cfsf < 30 {
		t.Fatalf("CFSF count = %d, want >= 30 (Pub 1220 §10 lists ~32 participants)", cfsf)
	}
	if non < 10 {
		t.Fatalf("non-CFSF count = %d, want >= 10", non)
	}
	if total != cfsf+non {
		t.Fatalf("total = %d, want %d", total, cfsf+non)
	}
}

func TestAllParticipants(t *testing.T) {
	parts := AllParticipants()
	if len(parts) < 30 {
		t.Fatalf("participants = %d, want >= 30", len(parts))
	}
	for _, p := range parts {
		if !p.ParticipatesInCFSF {
			t.Fatalf("non-participant %s in participants list", p.State)
		}
		if p.CFSFCode == "" {
			t.Fatalf("CFSF participant %s missing CFSF code", p.State)
		}
	}
}

func TestRouteToStates_NilSubmission(t *testing.T) {
	_, err := RouteToStates(nil, []State{"CA"})
	if err == nil {
		t.Fatal("expected error for nil submission")
	}
}

// Conformance test required by task spec:
// "CFSF state routing for 5 representative CFSF participants (CA, NY, TX, FL, MA)
//  + 2 non-CFSF (e.g., DE, ID)"
//
// Wait — task said "CA, NY, TX, FL, MA" but NY/TX/FL are not CFSF
// participants. The 5 representative CFSF participants we test are
// CA, MA, OH, OK, IN; the 2 non-CFSF for the second category are NY
// and DE. We retain the test name from the task spec for traceability
// and assert correct routing across all 7 states.
func TestRouteToStates_TaskSpecConformance(t *testing.T) {
	fs := &iris.FormSubmission{FormType: iris.Form1099DIV, TaxYear: 2025}

	cases := []struct {
		state          State
		wantRoutingTyp string
		wantCFSFCode   CFSFStateCode
	}{
		// CFSF participants:
		{"CA", "cfsf", "06"},
		{"MA", "cfsf", "25"},
		{"OH", "cfsf", "39"},
		{"OK", "cfsf", "40"},
		{"IN", "cfsf", "18"},
		// Non-CFSF (NY = direct portal, TX/FL = none, DE participates so
		// excluded). Adding two non-CFSF: NY direct + IL FSET.
		{"NY", "direct", ""},
		{"IL", "direct", ""},
		// No-income-tax (also non-CFSF):
		{"TX", "none", ""},
		{"FL", "none", ""},
	}

	for _, tc := range cases {
		t.Run(string(tc.state), func(t *testing.T) {
			filings, err := RouteToStates(IRISWrapper{FS: fs}, []State{tc.state})
			if err != nil {
				t.Fatalf("RouteToStates: %v", err)
			}
			if filings[0].RoutingType != tc.wantRoutingTyp {
				t.Fatalf("routing_type = %q, want %q", filings[0].RoutingType, tc.wantRoutingTyp)
			}
			if tc.wantCFSFCode != "" && filings[0].CFSFCode != tc.wantCFSFCode {
				t.Fatalf("CFSF code = %q, want %q", filings[0].CFSFCode, tc.wantCFSFCode)
			}
		})
	}
}
