// Package tcc — Transmitter Control Code registry. Each issuer that
// files its own information returns has its own IRS-issued TCC; Lux
// may also act as agent under its own TCC. The registry resolves
// (issuer, env) -> TCC and tracks per-TCC scoping rules.
//
// Source-of-design: Public-Spec
// Source-ref: IRS Publication 5717 §4 — Transmitter Control Code
// Source-ref: IRS Publication 1220 Part B §2 — TCC Application Process
package tcc

import (
	"errors"
	"fmt"
	"sync"
)

// Env identifies the IRS environment a TCC is registered against.
// Sandboxes (IRIS AATS, FIRE Test) require their own TCC distinct
// from the production TCC.
type Env string

const (
	// EnvProduction is the production TCC scope.
	EnvProduction Env = "production"

	// EnvSandbox is the sandbox TCC scope (IRIS AATS, FIRE Test).
	EnvSandbox Env = "sandbox"
)

// System identifies the IRS e-file system the TCC is registered
// against — IRIS or FIRE. The same legal entity may hold separate
// IRIS and FIRE TCCs.
type System string

const (
	// SystemIRIS is the IRIS A2A scope.
	SystemIRIS System = "iris"

	// SystemFIRE is the FIRE scope.
	SystemFIRE System = "fire"
)

// Agency identifies whether the TCC is used by the issuer directly or
// by a third-party transmitter acting on the issuer's behalf.
type Agency string

const (
	// AgencyIssuer = the TCC belongs to the issuer; the issuer
	// transmits its own returns.
	AgencyIssuer Agency = "issuer"

	// AgencyTransmitter = the TCC belongs to a third-party
	// transmitter (e.g., Lux Industries Inc. transmitting on behalf
	// of a captable customer).
	AgencyTransmitter Agency = "transmitter"
)

// Entry is one registry row — a single (issuer, system, env) -> TCC
// mapping.
type Entry struct {
	// IssuerID is the captable-level issuer identifier (the company
	// id under which returns are filed).
	IssuerID string

	// System is the e-file system the TCC is registered against.
	System System

	// Env is the IRS environment the TCC is registered for.
	Env Env

	// TCC is the IRS-assigned 5-character Transmitter Control Code.
	// Loaded from KMS, never logged.
	TCC string

	// EIN is the EIN bound to the TCC on the IRIS / FIRE
	// Application. The IRS rejects submissions where the
	// transmitter EIN on the wire does not match the EIN of record
	// for the TCC.
	EIN string

	// Agency identifies whether this TCC is the issuer's own or a
	// transmitter's.
	Agency Agency

	// TransmitterEntityID identifies the transmitter entity when
	// Agency == AgencyTransmitter (e.g., "lux-industries-inc").
	// Empty for AgencyIssuer.
	TransmitterEntityID string
}

// Registry is the in-memory TCC registry. The production registry is
// backed by KMS-encrypted storage outside the captable library; this
// in-memory implementation is the canonical contract that consumers
// (broker/treasury) instantiate.
type Registry struct {
	mu      sync.RWMutex
	entries map[registryKey]*Entry
}

// registryKey scopes a registry lookup to (issuer, system, env).
type registryKey struct {
	IssuerID string
	System   System
	Env      Env
}

// NewRegistry constructs an empty Registry. Callers Register
// entries from KMS at process boot.
func NewRegistry() *Registry {
	return &Registry{entries: make(map[registryKey]*Entry)}
}

// Errors surfaced by the registry.
var (
	// ErrNotRegistered is returned by Resolve when no TCC is registered
	// for the (issuer, system, env) tuple.
	ErrNotRegistered = errors.New("tcc: no TCC registered for the (issuer, system, env) tuple")

	// ErrInvalidEntry is returned by Register on validation failure.
	ErrInvalidEntry = errors.New("tcc: invalid entry")
)

// Register stores a new (issuer, system, env) -> TCC entry. Returns
// ErrInvalidEntry on validation failure.
func (r *Registry) Register(e *Entry) error {
	if err := validateEntry(e); err != nil {
		return fmt.Errorf("%w: %v", ErrInvalidEntry, err)
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	r.entries[registryKey{e.IssuerID, e.System, e.Env}] = e
	return nil
}

// Resolve returns the TCC entry for the (issuer, system, env) tuple.
// Returns ErrNotRegistered if no entry is present.
func (r *Registry) Resolve(issuerID string, system System, env Env) (*Entry, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	e, ok := r.entries[registryKey{issuerID, system, env}]
	if !ok {
		return nil, ErrNotRegistered
	}
	cp := *e
	return &cp, nil
}

// List returns a copy of all registered entries (no particular order).
func (r *Registry) List() []*Entry {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := make([]*Entry, 0, len(r.entries))
	for _, e := range r.entries {
		cp := *e
		out = append(out, &cp)
	}
	return out
}

// Unregister removes the (issuer, system, env) entry. Returns
// ErrNotRegistered if no entry was present.
func (r *Registry) Unregister(issuerID string, system System, env Env) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	key := registryKey{issuerID, system, env}
	if _, ok := r.entries[key]; !ok {
		return ErrNotRegistered
	}
	delete(r.entries, key)
	return nil
}

// validateEntry runs field-level validation on a registry entry.
func validateEntry(e *Entry) error {
	if e == nil {
		return fmt.Errorf("entry is nil")
	}
	if e.IssuerID == "" {
		return fmt.Errorf("issuer_id is required")
	}
	if e.System != SystemIRIS && e.System != SystemFIRE {
		return fmt.Errorf("system must be one of iris/fire, got %q", e.System)
	}
	if e.Env != EnvProduction && e.Env != EnvSandbox {
		return fmt.Errorf("env must be one of production/sandbox, got %q", e.Env)
	}
	if len(e.TCC) != 5 {
		return fmt.Errorf("tcc must be 5 chars, got %d", len(e.TCC))
	}
	if len(e.EIN) != 9 {
		return fmt.Errorf("ein must be 9 digits, got %d", len(e.EIN))
	}
	if e.Agency != AgencyIssuer && e.Agency != AgencyTransmitter {
		return fmt.Errorf("agency must be one of issuer/transmitter, got %q", e.Agency)
	}
	if e.Agency == AgencyTransmitter && e.TransmitterEntityID == "" {
		return fmt.Errorf("transmitter_entity_id is required when agency=transmitter")
	}
	return nil
}
