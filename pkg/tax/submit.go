// Tax e-file submission entrypoint. SubmitForm routes a generated
// 1099 form to IRIS by default (mandatory for TY 2024+) with FIRE as
// the explicit fallback for legacy-year submissions.
//
// Source-of-design: Public-Spec
// Source-ref: IRS Publication 5717 — IRIS A2A Specifications
// Source-ref: IRS Publication 1220 — FIRE Specifications
package tax

import (
	"context"
	"errors"
	"fmt"

	"github.com/luxfi/captable/pkg/tax/fire"
	"github.com/luxfi/captable/pkg/tax/iris"
	"github.com/luxfi/captable/pkg/tax/tcc"
)

// IRISClient is the minimal interface SubmitForm needs from the IRIS
// adapter. The package-level *iris.Client satisfies this; the
// interface lets callers stub for tests.
type IRISClient interface {
	SubmitForm(ctx context.Context, fs *iris.FormSubmission) (*iris.Acknowledgment, error)
}

// FIREClient is the minimal interface SubmitForm needs from the FIRE
// adapter. The package-level *fire.Client satisfies this.
type FIREClient interface {
	SubmitFile(ctx context.Context, f *fire.FIREFile) (*fire.Acknowledgment, error)
}

// SubmissionResult unifies the IRIS and FIRE acknowledgment surfaces
// so callers see one return type regardless of which transport was
// used.
type SubmissionResult struct {
	// Transport is one of "iris" or "fire".
	Transport string

	// ReceiptID is the IRIS Receipt ID (for IRIS submissions) or the
	// FIRE filename (for FIRE submissions).
	ReceiptID string

	// Status is the e-file system's acceptance status.
	Status string
}

// SubmissionOptions modifies the SubmitForm routing decision.
type SubmissionOptions struct {
	// ForceLegacyFIRE forces routing to FIRE regardless of tax year.
	// Use only for corrections of older years that were originally
	// filed to FIRE and remain in FIRE-side records.
	ForceLegacyFIRE bool

	// IRIS is the IRIS client; required unless ForceLegacyFIRE is
	// true.
	IRIS IRISClient

	// FIRE is the FIRE client; required if ForceLegacyFIRE is true or
	// the form's tax year is below IRISMandatoryTaxYear.
	FIRE FIREClient

	// TCCRegistry resolves the issuer's TCC for the chosen transport
	// and env. Optional; callers may pre-populate the IRIS / FIRE
	// transmitter on the wire instead.
	TCCRegistry *tcc.Registry

	// IssuerID identifies the issuer in the TCC registry.
	IssuerID string

	// IRISEnv selects the IRIS env to route to.
	IRISEnv iris.IRISEnv

	// FIREEnv selects the FIRE env to route to.
	FIREEnv fire.FIREEnv
}

// IRISMandatoryTaxYear is the first tax year for which IRIS is the
// mandatory transport. Forms with TaxYear >= this value MUST file to
// IRIS unless ForceLegacyFIRE is set. (IRIS became mandatory for TY
// 2024 per IRS final regs published Feb 2023.)
const IRISMandatoryTaxYear = 2024

// ErrNoTransportConfigured is returned when SubmitForm cannot pick a
// route because neither IRIS nor FIRE was configured.
var ErrNoTransportConfigured = errors.New("tax: no e-file transport configured")

// SubmitForm routes a Form1099 to the IRS e-file system. The default
// transport is IRIS (the modernized, mandatory-for-TY-2024+ system);
// FIRE is selected only when SubmissionOptions.ForceLegacyFIRE is
// true or the form's TaxYear is below IRISMandatoryTaxYear.
//
// The function builds a typed iris.FormSubmission or fire.FIREFile
// from the supplied tax form and delegates to the transport's
// adapter. Callers retain control over advanced fields (multi-payee
// submissions, corrections, custom transmitter info) by constructing
// iris.FormSubmission / fire.FIREFile directly and calling
// SubmitForm on the chosen client.
func SubmitForm(ctx context.Context, form *Form1099DIV, opts SubmissionOptions) (*SubmissionResult, error) {
	if form == nil {
		return nil, fmt.Errorf("tax: form is nil")
	}
	useFIRE := opts.ForceLegacyFIRE || form.TaxYear < IRISMandatoryTaxYear
	if useFIRE {
		if opts.FIRE == nil {
			return nil, fmt.Errorf("%w: FIRE client required for TY %d submission", ErrNoTransportConfigured, form.TaxYear)
		}
		f, err := buildFIREFileFromDIV(form, opts)
		if err != nil {
			return nil, err
		}
		ack, err := opts.FIRE.SubmitFile(ctx, f)
		if err != nil {
			return nil, err
		}
		return &SubmissionResult{
			Transport: "fire",
			ReceiptID: ack.Filename,
			Status:    ack.Status,
		}, nil
	}

	if opts.IRIS == nil {
		return nil, fmt.Errorf("%w: IRIS client required for TY %d submission", ErrNoTransportConfigured, form.TaxYear)
	}
	fs, err := buildIRISSubmissionFromDIV(form, opts)
	if err != nil {
		return nil, err
	}
	ack, err := opts.IRIS.SubmitForm(ctx, fs)
	if err != nil {
		return nil, err
	}
	return &SubmissionResult{
		Transport: "iris",
		ReceiptID: ack.ReceiptID,
		Status:    ack.Status,
	}, nil
}

// SubmitForm1099B is the 1099-B variant of SubmitForm.
func SubmitForm1099B(ctx context.Context, form *Form1099B, opts SubmissionOptions) (*SubmissionResult, error) {
	if form == nil {
		return nil, fmt.Errorf("tax: form is nil")
	}
	useFIRE := opts.ForceLegacyFIRE || form.TaxYear < IRISMandatoryTaxYear
	if useFIRE {
		if opts.FIRE == nil {
			return nil, fmt.Errorf("%w: FIRE client required for TY %d submission", ErrNoTransportConfigured, form.TaxYear)
		}
		f, err := buildFIREFileFromB(form, opts)
		if err != nil {
			return nil, err
		}
		ack, err := opts.FIRE.SubmitFile(ctx, f)
		if err != nil {
			return nil, err
		}
		return &SubmissionResult{Transport: "fire", ReceiptID: ack.Filename, Status: ack.Status}, nil
	}

	if opts.IRIS == nil {
		return nil, fmt.Errorf("%w: IRIS client required for TY %d submission", ErrNoTransportConfigured, form.TaxYear)
	}
	fs, err := buildIRISSubmissionFromB(form, opts)
	if err != nil {
		return nil, err
	}
	ack, err := opts.IRIS.SubmitForm(ctx, fs)
	if err != nil {
		return nil, err
	}
	return &SubmissionResult{Transport: "iris", ReceiptID: ack.ReceiptID, Status: ack.Status}, nil
}

// --- builders ---

// buildIRISSubmissionFromDIV constructs an iris.FormSubmission from a
// captable Form1099DIV.
func buildIRISSubmissionFromDIV(form *Form1099DIV, opts SubmissionOptions) (*iris.FormSubmission, error) {
	transmitterTCC := ""
	transmitterEIN := ""
	if opts.TCCRegistry != nil && opts.IssuerID != "" {
		entry, err := opts.TCCRegistry.Resolve(opts.IssuerID, tcc.SystemIRIS, mapIRISEnv(opts.IRISEnv))
		if err != nil {
			return nil, fmt.Errorf("tax: resolve TCC for issuer %q: %w", opts.IssuerID, err)
		}
		transmitterTCC = entry.TCC
		transmitterEIN = entry.EIN
	}
	return &iris.FormSubmission{
		FormType: iris.Form1099DIV,
		TaxYear:  form.TaxYear,
		Transmitter: iris.Transmitter{
			TCC:  transmitterTCC,
			Name: form.PayerName,
			EIN:  pickEIN(transmitterEIN, form.PayerEIN),
		},
		Payer: iris.Payer{
			Name: form.PayerName,
			EIN:  form.PayerEIN,
		},
		Payees: []iris.PayeeBlock{
			{
				Payee: iris.Payee{
					TIN:     stripTIN(form.RecipientTIN),
					TINType: "S",
					Name:    form.RecipientName,
				},
				Data: &iris.Form1099DIVData{
					OrdinaryDividends:    form.OrdinaryDividends,
					QualifiedDividends:   form.QualifiedDividends,
					TotalCapGainDist:     form.CapitalGainDist,
					Section199ADividends: form.Section199ADividends,
					FederalTaxWithheld:   form.FederalTaxWithheld,
					StateTaxWithheld:     form.StateTaxWithheld,
					StateCd:              form.State,
				},
			},
		},
	}, nil
}

// buildIRISSubmissionFromB constructs an iris.FormSubmission from a
// captable Form1099B.
func buildIRISSubmissionFromB(form *Form1099B, opts SubmissionOptions) (*iris.FormSubmission, error) {
	transmitterTCC := ""
	transmitterEIN := ""
	if opts.TCCRegistry != nil && opts.IssuerID != "" {
		entry, err := opts.TCCRegistry.Resolve(opts.IssuerID, tcc.SystemIRIS, mapIRISEnv(opts.IRISEnv))
		if err != nil {
			return nil, fmt.Errorf("tax: resolve TCC for issuer %q: %w", opts.IssuerID, err)
		}
		transmitterTCC = entry.TCC
		transmitterEIN = entry.EIN
	}
	st := "S"
	switch form.ShortTermLongTerm {
	case "short":
		st = "S"
	case "long":
		st = "L"
	}
	return &iris.FormSubmission{
		FormType: iris.Form1099B,
		TaxYear:  form.TaxYear,
		Transmitter: iris.Transmitter{
			TCC:  transmitterTCC,
			Name: form.PayerName,
			EIN:  pickEIN(transmitterEIN, form.PayerEIN),
		},
		Payer: iris.Payer{
			Name: form.PayerName,
			EIN:  form.PayerEIN,
		},
		Payees: []iris.PayeeBlock{
			{
				Payee: iris.Payee{
					TIN:     stripTIN(form.RecipientTIN),
					TINType: "S",
					Name:    form.RecipientName,
				},
				Data: &iris.Form1099BData{
					Description:        form.Description,
					DateAcquired:       form.DateAcquired,
					DateSold:           form.DateSold,
					Proceeds:           form.Proceeds,
					CostBasis:          form.CostBasis,
					ShortTermLongTerm:  st,
					FederalTaxWithheld: form.FederalTaxWithheld,
				},
			},
		},
	}, nil
}

// buildFIREFileFromDIV constructs a fire.FIREFile carrying a single
// 1099-DIV payee record from a captable Form1099DIV.
func buildFIREFileFromDIV(form *Form1099DIV, opts SubmissionOptions) (*fire.FIREFile, error) {
	transmitterTCC := ""
	transmitterEIN := ""
	if opts.TCCRegistry != nil && opts.IssuerID != "" {
		entry, err := opts.TCCRegistry.Resolve(opts.IssuerID, tcc.SystemFIRE, mapFIREEnv(opts.FIREEnv))
		if err != nil {
			return nil, fmt.Errorf("tax: resolve TCC for issuer %q: %w", opts.IssuerID, err)
		}
		transmitterTCC = entry.TCC
		transmitterEIN = entry.EIN
	}
	return &fire.FIREFile{
		Transmitter: fire.TransmitterRecord{
			PaymentYear:           form.TaxYear,
			TIN:                   pickEIN(transmitterEIN, form.PayerEIN),
			TCC:                   transmitterTCC,
			TestFileInd:           opts.FIREEnv == fire.EnvTest,
			TransmitterName:       form.PayerName,
			CompanyName:           form.PayerName,
			CompanyMailingAddress: form.RecipientAddress,
			ContactName:           form.PayerName,
		},
		PayerGroups: []fire.PayerGroup{
			{
				Payer: fire.PayerRecord{
					PaymentYear:  form.TaxYear,
					PayerTIN:     form.PayerEIN,
					TypeOfReturn: fire.FormCode1099DIV,
					AmountCodes:  "124",
					PayerName:    form.PayerName,
				},
				Payees: []fire.PayeeRecord{
					{
						PaymentYear: form.TaxYear,
						TypeOfTIN:   "2",
						PayeeTIN:    stripTIN(form.RecipientTIN),
						PaymentAmounts: map[byte]int64{
							'1': dollarsToCents(form.OrdinaryDividends),
							'2': dollarsToCents(form.QualifiedDividends),
							'4': dollarsToCents(form.FederalTaxWithheld),
						},
						PayeeFirstNameLine: form.RecipientName,
					},
				},
			},
		},
	}, nil
}

// buildFIREFileFromB constructs a fire.FIREFile carrying a 1099-B
// payee record.
func buildFIREFileFromB(form *Form1099B, opts SubmissionOptions) (*fire.FIREFile, error) {
	transmitterTCC := ""
	transmitterEIN := ""
	if opts.TCCRegistry != nil && opts.IssuerID != "" {
		entry, err := opts.TCCRegistry.Resolve(opts.IssuerID, tcc.SystemFIRE, mapFIREEnv(opts.FIREEnv))
		if err != nil {
			return nil, fmt.Errorf("tax: resolve TCC for issuer %q: %w", opts.IssuerID, err)
		}
		transmitterTCC = entry.TCC
		transmitterEIN = entry.EIN
	}
	return &fire.FIREFile{
		Transmitter: fire.TransmitterRecord{
			PaymentYear:     form.TaxYear,
			TIN:             pickEIN(transmitterEIN, form.PayerEIN),
			TCC:             transmitterTCC,
			TestFileInd:     opts.FIREEnv == fire.EnvTest,
			TransmitterName: form.PayerName,
			CompanyName:     form.PayerName,
			ContactName:     form.PayerName,
		},
		PayerGroups: []fire.PayerGroup{
			{
				Payer: fire.PayerRecord{
					PaymentYear:  form.TaxYear,
					PayerTIN:     form.PayerEIN,
					TypeOfReturn: fire.FormCode1099B,
					AmountCodes:  "23A",
					PayerName:    form.PayerName,
				},
				Payees: []fire.PayeeRecord{
					{
						PaymentYear: form.TaxYear,
						TypeOfTIN:   "2",
						PayeeTIN:    stripTIN(form.RecipientTIN),
						PaymentAmounts: map[byte]int64{
							'2': dollarsToCents(form.Proceeds),
							'3': dollarsToCents(form.CostBasis),
							'A': dollarsToCents(form.FederalTaxWithheld),
						},
						PayeeFirstNameLine: form.RecipientName,
					},
				},
			},
		},
	}, nil
}

// stripTIN removes hyphens and spaces from a TIN.
func stripTIN(s string) string {
	out := make([]byte, 0, len(s))
	for i := 0; i < len(s); i++ {
		c := s[i]
		if c >= '0' && c <= '9' {
			out = append(out, c)
		}
	}
	return string(out)
}

// pickEIN returns the first non-empty EIN from the supplied
// alternatives.
func pickEIN(alts ...string) string {
	for _, e := range alts {
		s := stripTIN(e)
		if s != "" {
			return s
		}
	}
	return ""
}

// dollarsToCents converts USD dollars (as float) to integer cents
// with half-up rounding.
func dollarsToCents(v float64) int64 {
	if v < 0 {
		return -int64(-v*100 + 0.5)
	}
	return int64(v*100 + 0.5)
}

// mapIRISEnv converts an iris.IRISEnv to the tcc.Env used by the TCC
// registry.
func mapIRISEnv(e iris.IRISEnv) tcc.Env {
	if e == iris.EnvProduction {
		return tcc.EnvProduction
	}
	return tcc.EnvSandbox
}

// mapFIREEnv converts a fire.FIREEnv to the tcc.Env used by the TCC
// registry.
func mapFIREEnv(e fire.FIREEnv) tcc.Env {
	if e == fire.EnvProduction {
		return tcc.EnvProduction
	}
	return tcc.EnvSandbox
}
