package tax

import (
	"context"
	"fmt"
	"time"
)

// DividendProvider supplies dividend distribution data for tax form generation.
type DividendProvider interface {
	// GetDividendsByYear returns stakeholder ID -> total ordinary dividends for a tax year.
	GetDividendsByYear(ctx context.Context, companyID string, taxYear int) (map[string]float64, error)
}

// Service handles tax form generation (1099-DIV, 1099-B, Schedule K-1).
type Service struct {
	repo      Repository
	dividends DividendProvider
}

// NewService creates a tax service.
func NewService(repo Repository, dividends DividendProvider) *Service {
	return &Service{repo: repo, dividends: dividends}
}

// Generate1099DIV creates 1099-DIV forms for all dividend recipients in a tax year.
func (s *Service) Generate1099DIV(ctx context.Context, req *GenerationRequest, payerName, payerEIN string, recipients map[string]RecipientInfo) (*GenerationResult, error) {
	if req.CompanyID == "" {
		return nil, fmt.Errorf("company_id is required")
	}
	if req.TaxYear <= 0 {
		return nil, fmt.Errorf("tax_year must be positive")
	}

	divs, err := s.dividends.GetDividendsByYear(ctx, req.CompanyID, req.TaxYear)
	if err != nil {
		return nil, fmt.Errorf("get dividends: %w", err)
	}

	result := &GenerationResult{
		FormType: "1099-DIV",
		TaxYear:  req.TaxYear,
	}

	now := time.Now().UTC()
	for stakeholderID, amount := range divs {
		// IRS minimum reporting threshold for 1099-DIV is $10.
		if amount < 10.0 {
			continue
		}

		recip, ok := recipients[stakeholderID]
		if !ok {
			result.Errors++
			continue
		}

		form := &Form1099DIV{
			ID:                 fmt.Sprintf("1099div_%s_%d_%s", req.CompanyID, req.TaxYear, stakeholderID),
			TaxYear:            req.TaxYear,
			PayerName:          payerName,
			PayerEIN:           payerEIN,
			RecipientName:      recip.Name,
			RecipientTIN:       recip.TIN,
			RecipientAddress:   recip.Address,
			OrdinaryDividends:  amount,
			QualifiedDividends: amount, // simplified — in production, track qualified vs. non-qualified
			Status:             "generated",
			GeneratedAt:        &now,
		}

		if err := s.repo.CreateForm1099DIV(ctx, form); err != nil {
			result.Errors++
			continue
		}
		result.FormsCreated++
	}

	return result, nil
}

// Compute1099B calculates gain/loss for a sale transaction.
func (s *Service) Compute1099B(proceeds, costBasis float64, dateAcquired, dateSold time.Time) *Form1099B {
	gainLoss := proceeds - costBasis
	holdingDays := dateSold.Sub(dateAcquired).Hours() / 24
	term := "short"
	if holdingDays > 365 {
		term = "long"
	}

	return &Form1099B{
		DateAcquired:      dateAcquired.Format("2006-01-02"),
		DateSold:          dateSold.Format("2006-01-02"),
		Proceeds:          proceeds,
		CostBasis:         costBasis,
		GainLoss:          gainLoss,
		ShortTermLongTerm: term,
		Status:            "generated",
	}
}

// RecipientInfo contains the identifying information needed for tax forms.
type RecipientInfo struct {
	Name    string `json:"name"`
	TIN     string `json:"tin"`     // SSN or EIN
	Address string `json:"address"`
}
