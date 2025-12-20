package corporate

import (
	"context"
	"fmt"
	"math"
	"time"
)

// Service handles corporate actions (splits, mergers, reclassifications).
type Service struct {
	repo Repository
}

// NewService creates a corporate actions service.
func NewService(repo Repository) *Service {
	return &Service{repo: repo}
}

// ProposeAction creates a new corporate action in proposed state.
func (s *Service) ProposeAction(ctx context.Context, a *Action) error {
	if a.CompanyID == "" {
		return fmt.Errorf("company_id is required")
	}
	if a.Type == "" {
		return fmt.Errorf("type is required")
	}
	if a.EffectiveDate.IsZero() {
		return fmt.Errorf("effective_date is required")
	}
	a.Status = "proposed"
	a.CreatedAt = time.Now().UTC()
	return s.repo.CreateAction(ctx, a)
}

// ApproveAction moves an action from proposed to approved.
func (s *Service) ApproveAction(ctx context.Context, actionID string) error {
	a, err := s.repo.GetAction(ctx, actionID)
	if err != nil {
		return fmt.Errorf("get action: %w", err)
	}
	if a.Status != "proposed" {
		return fmt.Errorf("action %s is not in proposed state (status=%s)", a.ID, a.Status)
	}
	now := time.Now().UTC()
	a.Status = "approved"
	a.ApprovedDate = &now
	return s.repo.UpdateAction(ctx, a)
}

// CalculateSplit computes the results of a stock split for a set of holdings.
// It does NOT persist anything — the caller uses the results to update the cap table.
func (s *Service) CalculateSplit(split *StockSplit, holdings map[string]int64) ([]SplitResult, error) {
	if split.Ratio <= 0 {
		return nil, fmt.Errorf("ratio must be positive")
	}
	if split.ShareClassID == "" {
		return nil, fmt.Errorf("share_class_id is required")
	}

	results := make([]SplitResult, 0, len(holdings))
	for stakeholderID, sharesBefore := range holdings {
		exact := float64(sharesBefore) * split.Ratio
		sharesAfter := int64(math.Floor(exact))
		fractional := exact - float64(sharesAfter)

		results = append(results, SplitResult{
			StakeholderID: stakeholderID,
			SharesBefore:  sharesBefore,
			SharesAfter:   sharesAfter,
			Fractional:    fractional,
		})
	}

	return results, nil
}

// ExecuteAction marks an action as executed.
func (s *Service) ExecuteAction(ctx context.Context, actionID string) error {
	a, err := s.repo.GetAction(ctx, actionID)
	if err != nil {
		return fmt.Errorf("get action: %w", err)
	}
	if a.Status != "approved" {
		return fmt.Errorf("action %s must be approved before execution (status=%s)", a.ID, a.Status)
	}
	now := time.Now().UTC()
	a.Status = "executed"
	a.ExecutedDate = &now
	return s.repo.UpdateAction(ctx, a)
}

// CancelAction cancels a proposed or approved action.
func (s *Service) CancelAction(ctx context.Context, actionID string) error {
	a, err := s.repo.GetAction(ctx, actionID)
	if err != nil {
		return fmt.Errorf("get action: %w", err)
	}
	if a.Status == "executed" {
		return fmt.Errorf("cannot cancel executed action %s", a.ID)
	}
	a.Status = "cancelled"
	return s.repo.UpdateAction(ctx, a)
}
