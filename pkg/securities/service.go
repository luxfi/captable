package securities

import (
	"context"
	"fmt"
	"time"
)

// Service handles securities issuance, transfer, cancellation, and conversion.
type Service struct {
	repo Repository
}

// NewService creates a securities service backed by the given repository.
func NewService(repo Repository) *Service {
	return &Service{repo: repo}
}

// CreateSecurity registers a new security offering.
func (s *Service) CreateSecurity(ctx context.Context, sec *Security) error {
	if sec.CompanyID == "" {
		return fmt.Errorf("company_id is required")
	}
	if sec.Name == "" {
		return fmt.Errorf("name is required")
	}
	if sec.Type == "" {
		return fmt.Errorf("type is required")
	}
	now := time.Now().UTC()
	sec.CreatedAt = now
	sec.UpdatedAt = now
	if sec.Status == "" {
		sec.Status = "draft"
	}
	return s.repo.CreateSecurity(ctx, sec)
}

// GetSecurity retrieves a security by ID.
func (s *Service) GetSecurity(ctx context.Context, id string) (*Security, error) {
	if id == "" {
		return nil, fmt.Errorf("security id is required")
	}
	return s.repo.GetSecurity(ctx, id)
}

// ListSecurities returns all securities for a company.
func (s *Service) ListSecurities(ctx context.Context, companyID string) ([]*Security, error) {
	if companyID == "" {
		return nil, fmt.Errorf("company_id is required")
	}
	return s.repo.ListSecurities(ctx, companyID)
}

// Issue creates new shares for a stakeholder and records a ledger entry.
func (s *Service) Issue(ctx context.Context, req *IssuanceRequest) (*LedgerEntry, error) {
	if req.SecurityID == "" {
		return nil, fmt.Errorf("security_id is required")
	}
	if req.StakeholderID == "" {
		return nil, fmt.Errorf("stakeholder_id is required")
	}
	if req.Shares <= 0 {
		return nil, fmt.Errorf("shares must be positive")
	}
	if req.PricePerShare < 0 {
		return nil, fmt.Errorf("price_per_share must be non-negative")
	}

	sec, err := s.repo.GetSecurity(ctx, req.SecurityID)
	if err != nil {
		return nil, fmt.Errorf("get security: %w", err)
	}
	if sec.Status != "offering" && sec.Status != "draft" {
		return nil, fmt.Errorf("security %s is not in an issuable state (status=%s)", sec.ID, sec.Status)
	}

	entry := &LedgerEntry{
		ID:            fmt.Sprintf("led_%s_%d", req.SecurityID, time.Now().UnixNano()),
		SecurityID:    req.SecurityID,
		CompanyID:     sec.CompanyID,
		ToStakeholder: req.StakeholderID,
		Shares:        req.Shares,
		PricePerShare: req.PricePerShare,
		Action:        "issue",
		Timestamp:     time.Now().UTC(),
	}

	if err := s.repo.AppendLedger(ctx, entry); err != nil {
		return nil, fmt.Errorf("append ledger: %w", err)
	}

	// Update security counters.
	sec.AmountRaised += float64(req.Shares) * req.PricePerShare
	sec.CurrentInvestors++
	sec.UpdatedAt = time.Now().UTC()
	if err := s.repo.UpdateSecurity(ctx, sec); err != nil {
		return nil, fmt.Errorf("update security: %w", err)
	}

	return entry, nil
}

// Transfer moves shares between stakeholders and records a ledger entry.
func (s *Service) Transfer(ctx context.Context, req *TransferRequest) (*LedgerEntry, error) {
	if req.SecurityID == "" {
		return nil, fmt.Errorf("security_id is required")
	}
	if req.FromStakeholder == "" {
		return nil, fmt.Errorf("from_stakeholder is required")
	}
	if req.ToStakeholder == "" {
		return nil, fmt.Errorf("to_stakeholder is required")
	}
	if req.FromStakeholder == req.ToStakeholder {
		return nil, fmt.Errorf("from and to stakeholder must be different")
	}
	if req.Shares <= 0 {
		return nil, fmt.Errorf("shares must be positive")
	}

	sec, err := s.repo.GetSecurity(ctx, req.SecurityID)
	if err != nil {
		return nil, fmt.Errorf("get security: %w", err)
	}

	entry := &LedgerEntry{
		ID:              fmt.Sprintf("led_%s_%d", req.SecurityID, time.Now().UnixNano()),
		SecurityID:      req.SecurityID,
		CompanyID:       sec.CompanyID,
		FromStakeholder: req.FromStakeholder,
		ToStakeholder:   req.ToStakeholder,
		Shares:          req.Shares,
		PricePerShare:   req.PricePerShare,
		Action:          "transfer",
		Timestamp:       time.Now().UTC(),
	}

	if err := s.repo.AppendLedger(ctx, entry); err != nil {
		return nil, fmt.Errorf("append ledger: %w", err)
	}

	return entry, nil
}

// Cancel removes shares from a stakeholder and records a ledger entry.
func (s *Service) Cancel(ctx context.Context, req *CancellationRequest) (*LedgerEntry, error) {
	if req.SecurityID == "" {
		return nil, fmt.Errorf("security_id is required")
	}
	if req.StakeholderID == "" {
		return nil, fmt.Errorf("stakeholder_id is required")
	}
	if req.Shares <= 0 {
		return nil, fmt.Errorf("shares must be positive")
	}
	if req.Reason == "" {
		return nil, fmt.Errorf("reason is required")
	}

	sec, err := s.repo.GetSecurity(ctx, req.SecurityID)
	if err != nil {
		return nil, fmt.Errorf("get security: %w", err)
	}

	entry := &LedgerEntry{
		ID:              fmt.Sprintf("led_%s_%d", req.SecurityID, time.Now().UnixNano()),
		SecurityID:      req.SecurityID,
		CompanyID:       sec.CompanyID,
		FromStakeholder: req.StakeholderID,
		Shares:          req.Shares,
		Action:          "cancel",
		Timestamp:       time.Now().UTC(),
		Reference:       req.Reason,
	}

	if err := s.repo.AppendLedger(ctx, entry); err != nil {
		return nil, fmt.Errorf("append ledger: %w", err)
	}

	return entry, nil
}

// Convert transforms securities from one class to another (e.g., note to equity).
func (s *Service) Convert(ctx context.Context, req *ConversionRequest) (*LedgerEntry, error) {
	if req.SecurityID == "" {
		return nil, fmt.Errorf("security_id is required")
	}
	if req.StakeholderID == "" {
		return nil, fmt.Errorf("stakeholder_id is required")
	}
	if req.TargetClassID == "" {
		return nil, fmt.Errorf("target_class_id is required")
	}
	if req.ConversionShares <= 0 {
		return nil, fmt.Errorf("conversion_shares must be positive")
	}

	sec, err := s.repo.GetSecurity(ctx, req.SecurityID)
	if err != nil {
		return nil, fmt.Errorf("get security: %w", err)
	}

	entry := &LedgerEntry{
		ID:              fmt.Sprintf("led_%s_%d", req.SecurityID, time.Now().UnixNano()),
		SecurityID:      req.SecurityID,
		CompanyID:       sec.CompanyID,
		FromStakeholder: req.StakeholderID,
		ToStakeholder:   req.StakeholderID,
		Shares:          req.ConversionShares,
		PricePerShare:   req.ConversionPrice,
		Action:          "convert",
		Timestamp:       time.Now().UTC(),
		Reference:       fmt.Sprintf("target_class=%s", req.TargetClassID),
	}

	if err := s.repo.AppendLedger(ctx, entry); err != nil {
		return nil, fmt.Errorf("append ledger: %w", err)
	}

	return entry, nil
}

// GetLedger returns the full audit trail for a security.
func (s *Service) GetLedger(ctx context.Context, securityID string) ([]*LedgerEntry, error) {
	if securityID == "" {
		return nil, fmt.Errorf("security_id is required")
	}
	return s.repo.ListLedger(ctx, securityID)
}
