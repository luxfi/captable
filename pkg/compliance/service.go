package compliance

import (
	"context"
	"fmt"
	"time"
)

// Service handles securities compliance — Form D, blue sky filings, and accreditation checks.
type Service struct {
	repo         Repository
	formDFiler   FormDFiler
	blueSkyFiler BlueSkyFiler
}

// NewService creates a compliance service.
func NewService(repo Repository) *Service {
	return &Service{repo: repo}
}

// CreateFormD creates a new Form D filing record.
func (s *Service) CreateFormD(ctx context.Context, f *FormD) error {
	if f.CompanyID == "" {
		return fmt.Errorf("company_id is required")
	}
	if f.Exemption == "" {
		return fmt.Errorf("exemption is required")
	}
	if f.TotalOffering <= 0 {
		return fmt.Errorf("total_offering must be positive")
	}
	now := time.Now().UTC()
	f.CreatedAt = now
	f.UpdatedAt = now
	f.TotalRemaining = f.TotalOffering - f.TotalSold
	if f.Status == "" {
		f.Status = "draft"
	}
	return s.repo.CreateFormD(ctx, f)
}

// CheckRegDCompliance validates a Reg D offering against SEC rules.
func (s *Service) CheckRegDCompliance(f *FormD) *ComplianceCheckResult {
	result := &ComplianceCheckResult{Compliant: true}

	switch f.Exemption {
	case "506b":
		// Rule 506(b): max 35 non-accredited investors.
		if f.NumNonAccredited > 35 {
			result.Compliant = false
			result.Violations = append(result.Violations,
				fmt.Sprintf("506(b) allows max 35 non-accredited investors, have %d", f.NumNonAccredited))
		}
		// Warning at 30+.
		if f.NumNonAccredited >= 30 && f.NumNonAccredited <= 35 {
			result.Warnings = append(result.Warnings,
				fmt.Sprintf("approaching 506(b) non-accredited limit: %d/35", f.NumNonAccredited))
		}

	case "506c":
		// Rule 506(c): ALL investors must be accredited.
		if f.NumNonAccredited > 0 {
			result.Compliant = false
			result.Violations = append(result.Violations,
				fmt.Sprintf("506(c) requires all investors to be accredited, have %d non-accredited", f.NumNonAccredited))
		}

	case "504":
		// Rule 504: max $10M in 12 months.
		if f.TotalOffering > 10_000_000 {
			result.Compliant = false
			result.Violations = append(result.Violations,
				fmt.Sprintf("Rule 504 max offering is $10M, offering is $%.2f", f.TotalOffering))
		}
	}

	// Form D must be filed within 15 days of first sale.
	if f.FirstSaleDate != nil && f.FilingDate == nil {
		deadline := f.FirstSaleDate.Add(15 * 24 * time.Hour)
		if time.Now().After(deadline) {
			result.Compliant = false
			result.Violations = append(result.Violations,
				"Form D not filed within 15 days of first sale")
		} else {
			daysLeft := int(time.Until(deadline).Hours() / 24)
			result.Warnings = append(result.Warnings,
				fmt.Sprintf("Form D filing deadline in %d days", daysLeft))
		}
	}

	return result
}

// CreateBlueSkyFiling creates a new state filing record.
func (s *Service) CreateBlueSkyFiling(ctx context.Context, b *BlueSkyFiling) error {
	if b.CompanyID == "" {
		return fmt.Errorf("company_id is required")
	}
	if b.State == "" {
		return fmt.Errorf("state is required")
	}
	if b.FilingType == "" {
		return fmt.Errorf("filing_type is required")
	}
	if b.Status == "" {
		b.Status = "pending"
	}
	return s.repo.CreateBlueSkyFiling(ctx, b)
}

// GetRequiredStates returns the list of US states that typically require blue sky notice filings.
// This is a simplified reference — actual requirements vary by exemption and offering type.
func (s *Service) GetRequiredStates() []string {
	return []string{
		"AL", "AK", "AZ", "AR", "CA", "CO", "CT", "DE", "FL", "GA",
		"HI", "ID", "IL", "IN", "IA", "KS", "KY", "LA", "ME", "MD",
		"MA", "MI", "MN", "MS", "MO", "MT", "NE", "NV", "NH", "NJ",
		"NM", "NY", "NC", "ND", "OH", "OK", "OR", "PA", "RI", "SC",
		"SD", "TN", "TX", "UT", "VT", "VA", "WA", "WV", "WI", "WY",
		"DC",
	}
}
