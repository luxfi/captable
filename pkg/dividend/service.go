package dividend

import (
	"context"
	"fmt"
	"time"
)

// HoldingsProvider is the interface for looking up stakeholder positions at a record date.
// Consumers inject this — it is NOT provided by this package.
type HoldingsProvider interface {
	// GetHoldingsAtDate returns stakeholder ID -> shares held for a share class at a point in time.
	GetHoldingsAtDate(ctx context.Context, companyID, shareClassID string, date time.Time) (map[string]int64, error)
}

// Service handles dividend declaration, record date processing, and distribution.
type Service struct {
	repo     Repository
	holdings HoldingsProvider
}

// NewService creates a dividend service.
func NewService(repo Repository, holdings HoldingsProvider) *Service {
	return &Service{repo: repo, holdings: holdings}
}

// Declare creates a new dividend declaration.
func (s *Service) Declare(ctx context.Context, d *Declaration) error {
	if d.CompanyID == "" {
		return fmt.Errorf("company_id is required")
	}
	if d.ShareClassID == "" {
		return fmt.Errorf("share_class_id is required")
	}
	if d.Type == "" {
		return fmt.Errorf("type is required")
	}
	if d.Type == "cash" && d.AmountPerShare <= 0 {
		return fmt.Errorf("amount_per_share must be positive for cash dividend")
	}
	if d.Type == "stock" && d.StockRatio <= 0 {
		return fmt.Errorf("stock_ratio must be positive for stock dividend")
	}
	if d.RecordDate.IsZero() {
		return fmt.Errorf("record_date is required")
	}
	if d.PayableDate.IsZero() {
		return fmt.Errorf("payable_date is required")
	}
	if d.PayableDate.Before(d.RecordDate) {
		return fmt.Errorf("payable_date must be after record_date")
	}

	now := time.Now().UTC()
	d.CreatedAt = now
	if d.DeclarationDate.IsZero() {
		d.DeclarationDate = now
	}
	if d.ExDividendDate.IsZero() {
		// Ex-dividend is typically 1 business day before record date.
		d.ExDividendDate = d.RecordDate.Add(-24 * time.Hour)
	}
	if d.Status == "" {
		d.Status = "declared"
	}

	return s.repo.CreateDeclaration(ctx, d)
}

// ProcessRecordDate looks up all holders at the record date and creates distributions.
func (s *Service) ProcessRecordDate(ctx context.Context, declarationID string) (int, error) {
	decl, err := s.repo.GetDeclaration(ctx, declarationID)
	if err != nil {
		return 0, fmt.Errorf("get declaration: %w", err)
	}
	if decl.Status != "declared" {
		return 0, fmt.Errorf("declaration %s is not in declared state (status=%s)", decl.ID, decl.Status)
	}

	holdings, err := s.holdings.GetHoldingsAtDate(ctx, decl.CompanyID, decl.ShareClassID, decl.RecordDate)
	if err != nil {
		return 0, fmt.Errorf("get holdings: %w", err)
	}

	var totalAmount float64
	count := 0

	for stakeholderID, shares := range holdings {
		if shares <= 0 {
			continue
		}

		var amount float64
		var stockShares int64
		if decl.Type == "cash" {
			amount = float64(shares) * decl.AmountPerShare
		} else if decl.Type == "stock" {
			stockShares = int64(float64(shares) * decl.StockRatio)
			if stockShares <= 0 {
				continue
			}
		}

		dist := &Distribution{
			ID:            fmt.Sprintf("dist_%s_%s", declarationID, stakeholderID),
			DeclarationID: declarationID,
			StakeholderID: stakeholderID,
			Shares:        shares,
			GrossAmount:        amount,
			StockShares:   stockShares,
			NetAmount:     amount, // withholding computed separately
			Status:        "pending",
		}
		if err := s.repo.CreateDistribution(ctx, dist); err != nil {
			return count, fmt.Errorf("create distribution for %s: %w", stakeholderID, err)
		}
		totalAmount += amount
		count++
	}

	// Update declaration.
	decl.TotalAmount = totalAmount
	decl.Status = "record_set"
	if err := s.repo.UpdateDeclaration(ctx, decl); err != nil {
		return count, fmt.Errorf("update declaration: %w", err)
	}

	return count, nil
}

// GetSummary returns aggregate distribution stats for a declaration.
func (s *Service) GetSummary(ctx context.Context, declarationID string) (*DistributionSummary, error) {
	dists, err := s.repo.ListDistributions(ctx, declarationID)
	if err != nil {
		return nil, fmt.Errorf("list distributions: %w", err)
	}

	summary := &DistributionSummary{DeclarationID: declarationID}
	for _, d := range dists {
		summary.RecipientsCount++
		summary.TotalGross += d.GrossAmount
		summary.TotalWithholding += d.TaxWithholding
		summary.TotalNet += d.NetAmount
		switch d.Status {
		case "paid":
			summary.PaidCount++
		case "pending":
			summary.PendingCount++
		}
	}
	return summary, nil
}

// MarkPaid marks a distribution as paid.
func (s *Service) MarkPaid(ctx context.Context, distributionID string) error {
	d, err := s.repo.GetDistribution(ctx, distributionID)
	if err != nil {
		return fmt.Errorf("get distribution: %w", err)
	}
	now := time.Now().UTC()
	d.Status = "paid"
	d.PaidAt = &now
	return s.repo.UpdateDistribution(ctx, d)
}
