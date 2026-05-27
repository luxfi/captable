// Filing delegation — bridges the in-domain Form D / blue-sky models
// to the wire-level filer adapters in `pkg/edgar` and `pkg/bluesky`.
//
// The compliance package does not import the edgar / bluesky packages
// directly (those packages own the wire format and the HTTP transport
// against the SEC / NASAA endpoints, which is a separate concern from
// the in-memory compliance state managed here). Instead, callers wire
// a FormDFiler / BlueSkyFiler implementation in at startup via
// SetFormDFiler / SetBlueSkyFiler and then invoke FileFormD /
// FileBlueSkyNotice on the Service.
//
// Source-of-design: Public-Spec
// Source-ref: https://www.sec.gov/info/edgar/specifications/formdxml.pdf
// Source-ref: http://nasaaefd.org

package compliance

import (
	"context"
	"errors"
	"fmt"
	"time"
)

// ErrNoFiler is returned by FileFormD / FileBlueSkyNotice when no
// adapter has been wired via SetFormDFiler / SetBlueSkyFiler.
var ErrNoFiler = errors.New("compliance: no filer configured")

// FormDFiler is the minimal interface the compliance service requires
// from a Form D adapter. Implemented by *edgar.Filer.
type FormDFiler interface {
	// FileFormD submits a new Form D filing and returns the
	// EDGAR-assigned acknowledgment.
	FileFormD(ctx context.Context, payload any) (accessionNumber string, status string, err error)

	// AmendFormD submits an amendment to a previously filed Form D.
	AmendFormD(ctx context.Context, payload any) (accessionNumber string, status string, err error)
}

// BlueSkyFiler is the minimal interface the compliance service
// requires from a state notice-of-sale adapter. Implemented by
// *bluesky.Registrar.
type BlueSkyFiler interface {
	// FileNoticeOfSale submits a state notice-of-sale filing and
	// returns the state-assigned acknowledgment.
	FileNoticeOfSale(ctx context.Context, state string, payload any) (filingID string, status string, fee float64, err error)

	// RenewNotice submits a renewal for a state notice filing.
	RenewNotice(ctx context.Context, state string, payload any) (filingID string, status string, fee float64, err error)

	// CalculateStateFee returns the per-state filing fee for a notice
	// of sale, before the filing is submitted.
	CalculateStateFee(state string, payload any) (float64, error)
}

// SetFormDFiler wires the Form D filer adapter. Idempotent.
func (s *Service) SetFormDFiler(f FormDFiler) { s.formDFiler = f }

// SetBlueSkyFiler wires the blue-sky filer adapter. Idempotent.
func (s *Service) SetBlueSkyFiler(f BlueSkyFiler) { s.blueSkyFiler = f }

// FileFormD submits the FormD record's underlying payload through the
// wired EDGAR adapter, updates the FormD record with the resulting
// accession + status + filing date, and persists.
//
// The payload argument is the wire-level FormDFiling struct from
// `pkg/edgar` (declared as any here to keep this package free of an
// edgar import — the caller passes *edgar.FormDFiling).
func (s *Service) FileFormD(ctx context.Context, formDID string, payload any) (*FormD, error) {
	if s.formDFiler == nil {
		return nil, ErrNoFiler
	}
	rec, err := s.repo.GetFormD(ctx, formDID)
	if err != nil {
		return nil, fmt.Errorf("compliance: load form D %s: %w", formDID, err)
	}
	accession, status, err := s.formDFiler.FileFormD(ctx, payload)
	if err != nil {
		return rec, err
	}
	now := time.Now().UTC()
	rec.SECFileNumber = accession
	rec.FilingDate = &now
	if status == "ACCEPTED" || status == "RECEIVED" {
		rec.Status = "filed"
	} else if status == "REJECTED" || status == "SUSPENDED" {
		rec.Status = "rejected"
	}
	rec.UpdatedAt = now
	if err := s.repo.UpdateFormD(ctx, rec); err != nil {
		return rec, fmt.Errorf("compliance: persist form D %s after filing: %w", formDID, err)
	}
	return rec, nil
}

// AmendFormD submits an amendment to a previously filed Form D via
// the wired adapter, updates the FormD record, and persists.
func (s *Service) AmendFormD(ctx context.Context, formDID string, payload any) (*FormD, error) {
	if s.formDFiler == nil {
		return nil, ErrNoFiler
	}
	rec, err := s.repo.GetFormD(ctx, formDID)
	if err != nil {
		return nil, fmt.Errorf("compliance: load form D %s: %w", formDID, err)
	}
	accession, status, err := s.formDFiler.AmendFormD(ctx, payload)
	if err != nil {
		return rec, err
	}
	now := time.Now().UTC()
	rec.AmendmentDate = &now
	rec.SECFileNumber = accession
	if status == "ACCEPTED" || status == "RECEIVED" {
		rec.Status = "amended"
	} else if status == "REJECTED" || status == "SUSPENDED" {
		rec.Status = "rejected"
	}
	rec.UpdatedAt = now
	if err := s.repo.UpdateFormD(ctx, rec); err != nil {
		return rec, fmt.Errorf("compliance: persist form D %s after amendment: %w", formDID, err)
	}
	return rec, nil
}

// FileBlueSkyNotice submits a state notice filing via the wired
// adapter, updates the BlueSkyFiling record with state acknowledgment,
// fee, and status, and persists.
func (s *Service) FileBlueSkyNotice(ctx context.Context, filingID string, payload any) (*BlueSkyFiling, error) {
	if s.blueSkyFiler == nil {
		return nil, ErrNoFiler
	}
	rec, err := s.repo.GetBlueSkyFiling(ctx, filingID)
	if err != nil {
		return nil, fmt.Errorf("compliance: load blue sky filing %s: %w", filingID, err)
	}
	stateID, status, fee, err := s.blueSkyFiler.FileNoticeOfSale(ctx, rec.State, payload)
	if err != nil {
		return rec, err
	}
	now := time.Now().UTC()
	rec.FilingDate = &now
	rec.Fee = fee
	if status == "ACCEPTED" || status == "FILED" || status == "RECEIVED" {
		rec.Status = "filed"
	} else if status == "REJECTED" {
		rec.Status = "rejected"
	} else if status == "APPROVED" {
		rec.Status = "approved"
		rec.ApprovedDate = &now
	}
	if stateID != "" {
		// Stash the state-assigned ID in Notes for traceability.
		if rec.Notes == "" {
			rec.Notes = "state_id=" + stateID
		} else {
			rec.Notes = rec.Notes + "; state_id=" + stateID
		}
	}
	if err := s.repo.UpdateBlueSkyFiling(ctx, rec); err != nil {
		return rec, fmt.Errorf("compliance: persist blue sky filing %s after submit: %w", filingID, err)
	}
	return rec, nil
}
