// Compliance ↔ Filer integration tests — verifies the bridge from
// the in-domain Form D / BlueSkyFiling records to the wire-level
// adapters in pkg/edgar and pkg/bluesky.
//
// Source-of-design: Public-Spec
// Source-ref: https://www.sec.gov/info/edgar/specifications/formdxml.pdf
// Source-ref: http://nasaaefd.org

package compliance

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"
)

// fakeFormDFiler implements FormDFiler in-memory for the bridge test.
type fakeFormDFiler struct {
	fileCalls  atomic.Int32
	amendCalls atomic.Int32
	status     string
	err        error
}

func (f *fakeFormDFiler) FileFormD(_ context.Context, _ any) (string, string, error) {
	f.fileCalls.Add(1)
	if f.err != nil {
		return "", "", f.err
	}
	return "0001234567-26-000001", f.status, nil
}

func (f *fakeFormDFiler) AmendFormD(_ context.Context, _ any) (string, string, error) {
	f.amendCalls.Add(1)
	if f.err != nil {
		return "", "", f.err
	}
	return "0001234567-26-000002", f.status, nil
}

// fakeBlueSkyFiler implements BlueSkyFiler in-memory.
type fakeBlueSkyFiler struct {
	fileCalls   atomic.Int32
	renewCalls  atomic.Int32
	feeCalls    atomic.Int32
	status      string
	fee         float64
	err         error
}

func (f *fakeBlueSkyFiler) FileNoticeOfSale(_ context.Context, _ string, _ any) (string, string, float64, error) {
	f.fileCalls.Add(1)
	if f.err != nil {
		return "", "", 0, f.err
	}
	return "EFD-CO-26-1001", f.status, f.fee, nil
}

func (f *fakeBlueSkyFiler) RenewNotice(_ context.Context, _ string, _ any) (string, string, float64, error) {
	f.renewCalls.Add(1)
	return "EFD-CO-26-1002", f.status, f.fee, f.err
}

func (f *fakeBlueSkyFiler) CalculateStateFee(_ string, _ any) (float64, error) {
	f.feeCalls.Add(1)
	return f.fee, f.err
}

// --- FileFormD ---

func TestService_FileFormD_RequiresFiler(t *testing.T) {
	svc := NewService(newMemRepo())
	_, err := svc.FileFormD(context.Background(), "f1", nil)
	if !errors.Is(err, ErrNoFiler) {
		t.Fatalf("expected ErrNoFiler, got %v", err)
	}
}

func TestService_FileFormD_HappyPath(t *testing.T) {
	repo := newMemRepo()
	svc := NewService(repo)
	filer := &fakeFormDFiler{status: "RECEIVED"}
	svc.SetFormDFiler(filer)

	ctx := context.Background()
	if err := svc.CreateFormD(ctx, &FormD{
		ID: "f1", CompanyID: "c1", Exemption: "506b", TotalOffering: 5_000_000,
	}); err != nil {
		t.Fatalf("CreateFormD: %v", err)
	}

	rec, err := svc.FileFormD(ctx, "f1", nil /* payload would be *edgar.FormDFiling */)
	if err != nil {
		t.Fatalf("FileFormD: %v", err)
	}
	if filer.fileCalls.Load() != 1 {
		t.Errorf("expected 1 filer call, got %d", filer.fileCalls.Load())
	}
	if rec.Status != "filed" {
		t.Errorf("status = %q, want filed", rec.Status)
	}
	if rec.SECFileNumber != "0001234567-26-000001" {
		t.Errorf("SECFileNumber = %q", rec.SECFileNumber)
	}
	if rec.FilingDate == nil {
		t.Errorf("FilingDate not set")
	}
}

func TestService_FileFormD_Rejected(t *testing.T) {
	repo := newMemRepo()
	svc := NewService(repo)
	filer := &fakeFormDFiler{status: "REJECTED"}
	svc.SetFormDFiler(filer)

	ctx := context.Background()
	if err := svc.CreateFormD(ctx, &FormD{
		ID: "f1", CompanyID: "c1", Exemption: "506b", TotalOffering: 5_000_000,
	}); err != nil {
		t.Fatalf("CreateFormD: %v", err)
	}
	rec, err := svc.FileFormD(ctx, "f1", nil)
	if err != nil {
		t.Fatalf("FileFormD: %v", err)
	}
	if rec.Status != "rejected" {
		t.Errorf("status = %q, want rejected", rec.Status)
	}
}

func TestService_AmendFormD(t *testing.T) {
	repo := newMemRepo()
	svc := NewService(repo)
	filer := &fakeFormDFiler{status: "RECEIVED"}
	svc.SetFormDFiler(filer)
	ctx := context.Background()
	if err := svc.CreateFormD(ctx, &FormD{
		ID: "f1", CompanyID: "c1", Exemption: "506b", TotalOffering: 5_000_000,
	}); err != nil {
		t.Fatalf("CreateFormD: %v", err)
	}
	rec, err := svc.AmendFormD(ctx, "f1", nil)
	if err != nil {
		t.Fatalf("AmendFormD: %v", err)
	}
	if filer.amendCalls.Load() != 1 {
		t.Errorf("expected 1 amend call, got %d", filer.amendCalls.Load())
	}
	if rec.Status != "amended" {
		t.Errorf("status = %q, want amended", rec.Status)
	}
	if rec.AmendmentDate == nil {
		t.Errorf("AmendmentDate not set")
	}
}

// --- FileBlueSkyNotice ---

func TestService_FileBlueSkyNotice_RequiresFiler(t *testing.T) {
	svc := NewService(newMemRepo())
	_, err := svc.FileBlueSkyNotice(context.Background(), "b1", nil)
	if !errors.Is(err, ErrNoFiler) {
		t.Fatalf("expected ErrNoFiler, got %v", err)
	}
}

func TestService_FileBlueSkyNotice_HappyPath(t *testing.T) {
	repo := newMemRepo()
	svc := NewService(repo)
	filer := &fakeBlueSkyFiler{status: "FILED", fee: 75.00}
	svc.SetBlueSkyFiler(filer)

	ctx := context.Background()
	if err := svc.CreateBlueSkyFiling(ctx, &BlueSkyFiling{
		ID: "b1", CompanyID: "c1", State: "CO", FilingType: "notice",
	}); err != nil {
		t.Fatalf("CreateBlueSkyFiling: %v", err)
	}
	rec, err := svc.FileBlueSkyNotice(ctx, "b1", nil)
	if err != nil {
		t.Fatalf("FileBlueSkyNotice: %v", err)
	}
	if filer.fileCalls.Load() != 1 {
		t.Errorf("expected 1 filer call, got %d", filer.fileCalls.Load())
	}
	if rec.Status != "filed" {
		t.Errorf("status = %q, want filed", rec.Status)
	}
	if rec.Fee != 75.00 {
		t.Errorf("fee = %v", rec.Fee)
	}
	if rec.FilingDate == nil {
		t.Errorf("FilingDate not set")
	}
	if rec.Notes == "" {
		t.Errorf("expected state_id stashed in Notes")
	}
}
