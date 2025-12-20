package transfer

import (
	"context"
	"fmt"
	"math"
	"time"
)

// Service evaluates transfer restrictions including Rule 144, lockup periods,
// and custom restrictions. It does NOT execute transfers — that is done by
// the securities service. This service answers: "is this transfer allowed?"
type Service struct {
	repo Repository
}

// NewService creates a transfer restriction service.
func NewService(repo Repository) *Service {
	return &Service{repo: repo}
}

// CreateRestriction registers a new transfer restriction.
func (s *Service) CreateRestriction(ctx context.Context, r *Restriction) error {
	if r.CompanyID == "" {
		return fmt.Errorf("company_id is required")
	}
	if r.Type == "" {
		return fmt.Errorf("type is required")
	}
	if r.StartDate.IsZero() {
		r.StartDate = time.Now().UTC()
	}
	if r.Status == "" {
		r.Status = "active"
	}
	r.CreatedAt = time.Now().UTC()
	return s.repo.CreateRestriction(ctx, r)
}

// CheckTransfer evaluates all active restrictions for a proposed transfer.
func (s *Service) CheckTransfer(ctx context.Context, req *CheckRequest) (*CheckResult, error) {
	if req.CompanyID == "" {
		return nil, fmt.Errorf("company_id is required")
	}
	if req.FromStakeholder == "" {
		return nil, fmt.Errorf("from_stakeholder is required")
	}
	if req.Shares <= 0 {
		return nil, fmt.Errorf("shares must be positive")
	}

	restrictions, err := s.repo.ListRestrictions(ctx, req.CompanyID)
	if err != nil {
		return nil, fmt.Errorf("list restrictions: %w", err)
	}

	result := &CheckResult{Allowed: true}
	now := time.Now()

	for _, r := range restrictions {
		if r.Status != "active" {
			continue
		}
		// Check if expired.
		if r.EndDate != nil && r.EndDate.Before(now) {
			continue
		}
		// Check if restriction applies to this stakeholder.
		if r.StakeholderID != "" && r.StakeholderID != req.FromStakeholder {
			continue
		}
		// Check if restriction applies to this share class.
		if r.ShareClassID != "" && r.ShareClassID != req.ShareClassID {
			continue
		}

		// This restriction applies — block the transfer.
		result.Allowed = false
		desc := fmt.Sprintf("%s restriction (id=%s)", r.Type, r.ID)
		if r.Conditions != "" {
			desc += ": " + r.Conditions
		}
		if r.EndDate != nil {
			desc += fmt.Sprintf(" [expires %s]", r.EndDate.Format("2006-01-02"))
		}
		result.Restrictions = append(result.Restrictions, desc)
	}

	return result, nil
}

// CheckRule144 evaluates SEC Rule 144 eligibility for restricted securities.
//
// Rule 144 allows public resale of restricted/control securities if conditions are met:
//   - Holding period: 6 months (reporting issuer) or 12 months (non-reporting)
//   - Volume limit (affiliates): greater of 1% outstanding or avg weekly trading vol
//   - Current public information must be available
//   - Affiliates must file Form 144 if > 5,000 shares or > $50,000
func (s *Service) CheckRule144(check *Rule144Check) *Rule144Result {
	now := time.Now()
	monthsHeld := int(now.Sub(check.AcquiredDate).Hours() / (24 * 30))

	// Determine required holding period.
	requiredMonths := 12
	if check.IsReporting {
		requiredMonths = 6
	}

	result := &Rule144Result{
		HoldingPeriod: requiredMonths,
		MonthsHeld:    monthsHeld,
	}

	// Check holding period.
	if monthsHeld < requiredMonths {
		result.Eligible = false
		result.Reason = fmt.Sprintf("holding period not met: %d months held, %d required", monthsHeld, requiredMonths)
		return result
	}

	// Non-affiliates with 12+ months holding: no further restrictions.
	if !check.IsAffiliate && monthsHeld >= 12 {
		result.Eligible = true
		return result
	}

	// Affiliates: apply volume limitations.
	if check.IsAffiliate {
		// Volume limit: greater of 1% of outstanding shares or average weekly volume.
		onePercent := int64(math.Ceil(float64(check.SharesHeld) * 0.01))
		volumeLimit := onePercent
		if check.AvgWeeklyVol > 0 && check.AvgWeeklyVol > volumeLimit {
			volumeLimit = check.AvgWeeklyVol
		}
		result.VolumeLimit = volumeLimit

		if check.SharesToSell > volumeLimit {
			result.Eligible = false
			result.Reason = fmt.Sprintf("volume limit exceeded: selling %d, limit %d per 90-day period",
				check.SharesToSell, volumeLimit)
			return result
		}
	}

	result.Eligible = true
	return result
}

// WaiveRestriction marks a restriction as waived (e.g., board approval granted).
func (s *Service) WaiveRestriction(ctx context.Context, restrictionID string) error {
	r, err := s.repo.GetRestriction(ctx, restrictionID)
	if err != nil {
		return fmt.Errorf("get restriction: %w", err)
	}
	r.Status = "waived"
	return s.repo.UpdateRestriction(ctx, r)
}
