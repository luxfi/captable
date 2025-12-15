package settlement

import (
	"context"
	"fmt"
	"time"
)

// Service manages cross-broker-dealer secondary market trade settlement.
type Service struct {
	repo Repository
}

// NewService creates a secondary trade settlement service.
func NewService(repo Repository) *Service {
	return &Service{repo: repo}
}

// InitiateTrade creates a new secondary market trade in pending_compliance_clearance status.
// It validates that both buyer and seller have compliance references and broker-dealers.
func (s *Service) InitiateTrade(ctx context.Context, trade *SecondaryTrade) error {
	if trade.TransactionID == "" {
		return fmt.Errorf("transaction_id is required")
	}
	if trade.BuyerInvestorID == "" {
		return fmt.Errorf("buyer_investor_id is required")
	}
	if trade.SellerInvestorID == "" {
		return fmt.Errorf("seller_investor_id is required")
	}
	if trade.BuyerBD.FirmName == "" || trade.BuyerBD.CRDNumber == "" {
		return fmt.Errorf("buyer broker-dealer firm_name and crd_number are required")
	}
	if trade.SellerBD.FirmName == "" || trade.SellerBD.CRDNumber == "" {
		return fmt.Errorf("seller broker-dealer firm_name and crd_number are required")
	}
	if trade.BuyerCompliance.Endpoint == "" {
		return fmt.Errorf("buyer compliance_ref endpoint is required")
	}
	if trade.SellerCompliance.Endpoint == "" {
		return fmt.Errorf("seller compliance_ref endpoint is required")
	}
	if trade.Security.NumberOfShares <= 0 {
		return fmt.Errorf("number_of_shares must be positive")
	}
	if trade.Security.PricePerShare <= 0 {
		return fmt.Errorf("price_per_share must be positive")
	}

	trade.TransactionType = TransactionSecondaryMarketTransfer
	trade.Status = TradeStatusPendingComplianceClearance
	trade.InitiatedAt = time.Now().UTC()
	trade.Settlement.Status = SettlementPending

	return s.repo.CreateTrade(ctx, trade)
}

// ApproveTrade moves a trade from pending_compliance_clearance to pending_transfer.
func (s *Service) ApproveTrade(ctx context.Context, transactionID string) error {
	trade, err := s.repo.GetTrade(ctx, transactionID)
	if err != nil {
		return fmt.Errorf("get trade: %w", err)
	}
	if trade.Status != TradeStatusPendingComplianceClearance {
		return fmt.Errorf("trade %s is in status %s, expected %s",
			transactionID, trade.Status, TradeStatusPendingComplianceClearance)
	}
	trade.Status = TradeStatusPendingTransfer
	return s.repo.UpdateTrade(ctx, trade)
}

// ExecuteTrade finalizes the trade: sets status to executed, records blockchain tx hash,
// and returns a webhook event for notification.
func (s *Service) ExecuteTrade(ctx context.Context, transactionID, blockchainTxHash string) (*WebhookEvent, error) {
	trade, err := s.repo.GetTrade(ctx, transactionID)
	if err != nil {
		return nil, fmt.Errorf("get trade: %w", err)
	}
	if trade.Status != TradeStatusPendingTransfer {
		return nil, fmt.Errorf("trade %s is in status %s, expected %s",
			transactionID, trade.Status, TradeStatusPendingTransfer)
	}

	// Validate restrictions before execution.
	if err := s.ValidateRestrictions(trade); err != nil {
		return nil, fmt.Errorf("restriction check failed: %w", err)
	}

	now := time.Now().UTC()
	trade.Status = TradeStatusExecuted
	trade.BlockchainTxHash = blockchainTxHash
	trade.ExecutedAt = &now
	trade.Settlement.Status = SettlementSettled

	if err := s.repo.UpdateTrade(ctx, trade); err != nil {
		return nil, fmt.Errorf("update trade: %w", err)
	}

	event := s.CreateWebhookEvent(trade)
	return event, nil
}

// ValidateRestrictions checks Rule 144, lockup, and legend requirements.
// Returns an error if the trade is blocked by any restriction.
func (s *Service) ValidateRestrictions(trade *SecondaryTrade) error {
	// Check lockup period.
	if trade.Restrictions.LockupExpiryDate != nil {
		if time.Now().Before(*trade.Restrictions.LockupExpiryDate) {
			return fmt.Errorf("trade blocked: lockup period has not expired (expires %s)",
				trade.Restrictions.LockupExpiryDate.Format("2006-01-02"))
		}
	}

	// Check Rule 144 holding period.
	if trade.Restrictions.LegendRequired && !trade.Restrictions.Rule144HoldingPeriodMet {
		// Legend-required shares that haven't met Rule 144 holding period
		// require additional compliance review. This is a warning, not a block,
		// because exemptions may apply. The transfer agent must acknowledge.
		if !trade.TransferAgentAck.Acknowledged {
			return fmt.Errorf("trade blocked: legend-required shares with Rule 144 holding period not met require transfer agent acknowledgment")
		}
	}

	return nil
}

// CreateWebhookEvent builds a webhook notification payload for a trade.
func (s *Service) CreateWebhookEvent(trade *SecondaryTrade) *WebhookEvent {
	now := time.Now().UTC()
	return &WebhookEvent{
		EventID:          fmt.Sprintf("evt_%s", trade.TransactionID),
		EventType:        "trade.executed",
		Timestamp:        now,
		Version:          "2.0.0",
		TransactionType:  "trade",
		BlockchainTxHash: trade.BlockchainTxHash,
		Recipients: []WebhookRecipient{
			{
				RecipientID:    fmt.Sprintf("rec_%s_buyer_bd", trade.TransactionID),
				Name:           trade.BuyerBD.FirmName,
				Role:           RoleBuyerBrokerDealer,
				DeliveredAt:    &now,
				DeliveryStatus: "pending",
			},
			{
				RecipientID:    fmt.Sprintf("rec_%s_seller_bd", trade.TransactionID),
				Name:           trade.SellerBD.FirmName,
				Role:           RoleSellerBrokerDealer,
				DeliveredAt:    &now,
				DeliveryStatus: "pending",
			},
			{
				RecipientID:    fmt.Sprintf("rec_%s_ta", trade.TransactionID),
				Name:           trade.TransferAgent.FirmName,
				Role:           RoleTransferAgent,
				DeliveredAt:    &now,
				DeliveryStatus: "pending",
			},
		},
	}
}
