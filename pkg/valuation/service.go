package valuation

import (
	"context"
	"fmt"
	"sort"
	"strconv"
	"time"
)

// Service provides 409A valuation tracking operations.
type Service struct {
	repo Repository
}

// NewService creates a valuation service backed by the given repository.
func NewService(repo Repository) *Service {
	return &Service{repo: repo}
}

// validMethods are the accepted 409A valuation methodologies.
var validMethods = map[string]bool{
	"dcf":               true,
	"market_comparable": true,
	"asset_based":       true,
	"backsolve":         true,
	"opm":               true,
}

// CreateValuation validates and persists a new 409A valuation.
func (s *Service) CreateValuation(ctx context.Context, v *Valuation409A) error {
	if v.CompanyID == "" {
		return fmt.Errorf("company_id is required")
	}
	if v.ShareClassID == "" {
		return fmt.Errorf("share_class_id is required")
	}
	if v.EffectiveDate.IsZero() {
		return fmt.Errorf("effective_date is required")
	}
	if v.FairMarketValue == "" {
		return fmt.Errorf("fair_market_value is required")
	}
	fmv, err := strconv.ParseFloat(v.FairMarketValue, 64)
	if err != nil || fmv <= 0 {
		return fmt.Errorf("fair_market_value must be a positive number")
	}
	if v.Method == "" {
		return fmt.Errorf("method is required")
	}
	if !validMethods[v.Method] {
		return fmt.Errorf("method must be one of: dcf, market_comparable, asset_based, backsolve, opm")
	}

	now := time.Now().UTC()
	v.CreatedAt = now
	v.UpdatedAt = now

	// Default expiration to 12 months from effective date per IRC 409A safe harbor.
	if v.ExpirationDate.IsZero() {
		v.ExpirationDate = v.EffectiveDate.AddDate(0, 12, 0)
	}
	if v.Status == "" {
		v.Status = "draft"
	}

	return s.repo.Create(ctx, v)
}

// GetCurrentFMV returns the fair market value from the latest non-expired 409A valuation
// for a given company and share class.
func (s *Service) GetCurrentFMV(ctx context.Context, companyID, shareClassID string) (string, error) {
	if companyID == "" {
		return "", fmt.Errorf("company_id is required")
	}
	if shareClassID == "" {
		return "", fmt.Errorf("share_class_id is required")
	}

	v, err := s.repo.GetLatest(ctx, companyID, shareClassID)
	if err != nil {
		return "", fmt.Errorf("get latest valuation: %w", err)
	}

	if s.IsExpired(v) {
		return "", fmt.Errorf("latest 409A valuation %s is expired (effective %s)",
			v.ID, v.EffectiveDate.Format("2006-01-02"))
	}

	return v.FairMarketValue, nil
}

// IsExpired checks whether a 409A valuation has expired.
// A valuation expires 12 months after its effective date, or at its explicit expiration date.
func (s *Service) IsExpired(v *Valuation409A) bool {
	now := time.Now().UTC()
	if !v.ExpirationDate.IsZero() {
		return now.After(v.ExpirationDate)
	}
	return now.After(v.EffectiveDate.AddDate(0, 12, 0))
}

// IsExpiredAt checks whether a 409A valuation is expired at a specific point in time.
func (s *Service) IsExpiredAt(v *Valuation409A, at time.Time) bool {
	if !v.ExpirationDate.IsZero() {
		return at.After(v.ExpirationDate)
	}
	return at.After(v.EffectiveDate.AddDate(0, 12, 0))
}

// ListHistory returns all 409A valuations for a company, sorted by effective date descending.
func (s *Service) ListHistory(ctx context.Context, companyID string) (*ValuationHistory, error) {
	if companyID == "" {
		return nil, fmt.Errorf("company_id is required")
	}

	vals, err := s.repo.List(ctx, companyID)
	if err != nil {
		return nil, fmt.Errorf("list valuations: %w", err)
	}

	// Sort by effective date descending (most recent first).
	sort.Slice(vals, func(i, j int) bool {
		return vals[i].EffectiveDate.After(vals[j].EffectiveDate)
	})

	history := &ValuationHistory{
		CompanyID:  companyID,
		Valuations: make([]Valuation409A, len(vals)),
	}
	for i, v := range vals {
		history.Valuations[i] = *v
	}

	return history, nil
}
