package tcc

import (
	"errors"
	"testing"
)

func TestRegister_HappyPath(t *testing.T) {
	r := NewRegistry()
	err := r.Register(&Entry{
		IssuerID: "acme",
		System:   SystemIRIS,
		Env:      EnvProduction,
		TCC:      "ABC12",
		EIN:      "987654321",
		Agency:   AgencyIssuer,
	})
	if err != nil {
		t.Fatalf("Register: %v", err)
	}
	got, err := r.Resolve("acme", SystemIRIS, EnvProduction)
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	if got.TCC != "ABC12" {
		t.Fatalf("TCC = %q, want ABC12", got.TCC)
	}
}

func TestResolve_NotRegistered(t *testing.T) {
	r := NewRegistry()
	_, err := r.Resolve("ghost", SystemIRIS, EnvProduction)
	if !errors.Is(err, ErrNotRegistered) {
		t.Fatalf("err = %v, want ErrNotRegistered", err)
	}
}

func TestRegister_Scoping(t *testing.T) {
	r := NewRegistry()

	// Same issuer with separate sandbox + prod TCCs (different IRS-
	// assigned codes).
	must := func(e *Entry) {
		t.Helper()
		if err := r.Register(e); err != nil {
			t.Fatalf("Register: %v", err)
		}
	}
	must(&Entry{IssuerID: "acme", System: SystemIRIS, Env: EnvSandbox, TCC: "SBX01", EIN: "987654321", Agency: AgencyIssuer})
	must(&Entry{IssuerID: "acme", System: SystemIRIS, Env: EnvProduction, TCC: "PRD01", EIN: "987654321", Agency: AgencyIssuer})
	must(&Entry{IssuerID: "acme", System: SystemFIRE, Env: EnvProduction, TCC: "FRE01", EIN: "987654321", Agency: AgencyIssuer})

	resolveExpect := func(system System, env Env, want string) {
		t.Helper()
		got, err := r.Resolve("acme", system, env)
		if err != nil {
			t.Fatalf("Resolve(%s,%s): %v", system, env, err)
		}
		if got.TCC != want {
			t.Fatalf("TCC = %q, want %q", got.TCC, want)
		}
	}
	resolveExpect(SystemIRIS, EnvSandbox, "SBX01")
	resolveExpect(SystemIRIS, EnvProduction, "PRD01")
	resolveExpect(SystemFIRE, EnvProduction, "FRE01")
}

func TestRegister_TransmitterAgency(t *testing.T) {
	r := NewRegistry()
	err := r.Register(&Entry{
		IssuerID:            "small-co",
		System:              SystemIRIS,
		Env:                 EnvProduction,
		TCC:                 "LUX01",
		EIN:                 "200000001",
		Agency:              AgencyTransmitter,
		TransmitterEntityID: "lux-industries-inc",
	})
	if err != nil {
		t.Fatalf("Register: %v", err)
	}
	got, _ := r.Resolve("small-co", SystemIRIS, EnvProduction)
	if got.Agency != AgencyTransmitter {
		t.Fatalf("Agency = %q, want transmitter", got.Agency)
	}
	if got.TransmitterEntityID != "lux-industries-inc" {
		t.Fatalf("TransmitterEntityID = %q", got.TransmitterEntityID)
	}
}

func TestRegister_ValidationFailures(t *testing.T) {
	r := NewRegistry()
	cases := []struct {
		name string
		e    *Entry
		want string
	}{
		{"missing issuer", &Entry{System: SystemIRIS, Env: EnvProduction, TCC: "ABC12", EIN: "123456789", Agency: AgencyIssuer}, "issuer_id"},
		{"bad system", &Entry{IssuerID: "x", System: System("xx"), Env: EnvProduction, TCC: "ABC12", EIN: "123456789", Agency: AgencyIssuer}, "system"},
		{"bad env", &Entry{IssuerID: "x", System: SystemIRIS, Env: Env("zz"), TCC: "ABC12", EIN: "123456789", Agency: AgencyIssuer}, "env"},
		{"short tcc", &Entry{IssuerID: "x", System: SystemIRIS, Env: EnvProduction, TCC: "AB", EIN: "123456789", Agency: AgencyIssuer}, "tcc"},
		{"short ein", &Entry{IssuerID: "x", System: SystemIRIS, Env: EnvProduction, TCC: "ABC12", EIN: "12", Agency: AgencyIssuer}, "ein"},
		{"bad agency", &Entry{IssuerID: "x", System: SystemIRIS, Env: EnvProduction, TCC: "ABC12", EIN: "123456789", Agency: Agency("partner")}, "agency"},
		{"transmitter without entity id", &Entry{IssuerID: "x", System: SystemIRIS, Env: EnvProduction, TCC: "ABC12", EIN: "123456789", Agency: AgencyTransmitter}, "transmitter_entity_id"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := r.Register(tc.e)
			if err == nil {
				t.Fatalf("expected error containing %q", tc.want)
			}
			if !errors.Is(err, ErrInvalidEntry) {
				t.Fatalf("err = %v, want ErrInvalidEntry", err)
			}
		})
	}
}

func TestUnregister(t *testing.T) {
	r := NewRegistry()
	_ = r.Register(&Entry{IssuerID: "x", System: SystemIRIS, Env: EnvProduction, TCC: "ABC12", EIN: "987654321", Agency: AgencyIssuer})
	if err := r.Unregister("x", SystemIRIS, EnvProduction); err != nil {
		t.Fatalf("Unregister: %v", err)
	}
	if err := r.Unregister("x", SystemIRIS, EnvProduction); !errors.Is(err, ErrNotRegistered) {
		t.Fatalf("err = %v, want ErrNotRegistered", err)
	}
}

func TestList(t *testing.T) {
	r := NewRegistry()
	_ = r.Register(&Entry{IssuerID: "a", System: SystemIRIS, Env: EnvProduction, TCC: "AAA01", EIN: "111111111", Agency: AgencyIssuer})
	_ = r.Register(&Entry{IssuerID: "b", System: SystemFIRE, Env: EnvProduction, TCC: "BBB01", EIN: "222222222", Agency: AgencyIssuer})
	got := r.List()
	if len(got) != 2 {
		t.Fatalf("len(List()) = %d, want 2", len(got))
	}
}
