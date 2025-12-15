package settlement

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"
)

// --- in-memory repository for tests ---

type memRepo struct {
	mu    sync.RWMutex
	items map[string]*SecondaryTrade
}

func newMemRepo() *memRepo {
	return &memRepo{items: make(map[string]*SecondaryTrade)}
}

func (r *memRepo) CreateTrade(_ context.Context, trade *SecondaryTrade) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, exists := r.items[trade.TransactionID]; exists {
		return fmt.Errorf("trade %s already exists", trade.TransactionID)
	}
	cp := *trade
	r.items[trade.TransactionID] = &cp
	return nil
}

func (r *memRepo) GetTrade(_ context.Context, transactionID string) (*SecondaryTrade, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	trade, ok := r.items[transactionID]
	if !ok {
		return nil, fmt.Errorf("trade %s not found", transactionID)
	}
	cp := *trade
	return &cp, nil
}

func (r *memRepo) UpdateTrade(_ context.Context, trade *SecondaryTrade) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, exists := r.items[trade.TransactionID]; !exists {
		return fmt.Errorf("trade %s not found", trade.TransactionID)
	}
	cp := *trade
	r.items[trade.TransactionID] = &cp
	return nil
}

func (r *memRepo) ListTrades(_ context.Context, limit, offset int) ([]*SecondaryTrade, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	var out []*SecondaryTrade
	for _, t := range r.items {
		cp := *t
		out = append(out, &cp)
	}
	if offset >= len(out) {
		return nil, nil
	}
	end := offset + limit
	if end > len(out) {
		end = len(out)
	}
	return out[offset:end], nil
}

// --- helper to build a valid trade ---

func baseTrade(txnID string) *SecondaryTrade {
	return &SecondaryTrade{
		TransactionID:    txnID,
		BuyerInvestorID:  "inv_buyer_001",
		SellerInvestorID: "inv_seller_001",
		BuyerBD: BrokerDealer{
			FirmName:    "Raymond James Financial",
			CRDNumber:   "705",
			FINRAMember: true,
		},
		SellerBD: BrokerDealer{
			FirmName:    "Edward Jones",
			CRDNumber:   "250",
			FINRAMember: true,
		},
		BuyerCompliance: ComplianceRef{
			Endpoint:    "GET /v1/investors/inv_buyer_001/compliance",
			Description: "Buyer compliance data",
		},
		SellerCompliance: ComplianceRef{
			Endpoint:    "GET /v1/investors/inv_seller_001/compliance",
			Description: "Seller compliance data",
		},
		TransferAgent: TransferAgent{
			FirmName:              "Computershare Trust Company",
			SECRegistered:         true,
			SECRegistrationNumber: "84-01234",
		},
		TransferAgentAck: TransferAgentAck{
			Acknowledged:          true,
			AcknowledgedAt:        time.Now().UTC(),
			TransferInstructionID: "ti_001",
			RecordDate:            time.Now().UTC(),
			UnitsToTransfer:       1000,
			Status:                "pending_transfer",
		},
		Security: SecurityDetail{
			AssetID:        "asset_001",
			AssetName:      "Bridgewater Capital Inc. - Series B Preferred Stock",
			SecurityClass:  SecurityPreferredStock,
			ShareClass:     "Series B",
			IssuerID:       "iss_001",
			IssuerName:     "Bridgewater Capital Inc.",
			IssuerType:     IssuerCorporation,
			NumberOfShares: 1000,
			PricePerShare:  45.00,
			Currency:       "USD",
			GrossAmount:    45000.00,
		},
		Commissions: TradeCommissions{
			BuyerBD: Commission{
				FirmName:         "Raymond James Financial",
				CRDNumber:        "705",
				CommissionType:   CommissionFlatFee,
				CommissionRate:   nil,
				CommissionAmount: "112.50",
				Currency:         "USD",
			},
			SellerBD: Commission{
				FirmName:         "Edward Jones",
				CRDNumber:        "250",
				CommissionType:   CommissionFlatFee,
				CommissionRate:   nil,
				CommissionAmount: "112.50",
				Currency:         "USD",
			},
			TotalCommissions: "225.00",
		},
		Settlement: TradeSettlement{
			SettlementDate: time.Now().Add(48 * time.Hour).UTC(),
			SettlementType: SettlementBilateral,
			Currency:       "USD",
		},
		Restrictions: TradeRestrictions{
			LegendRequired:          true,
			Rule144HoldingPeriodMet: false,
			TransferRestrictions:    "Subject to issuer consent",
		},
		Description: "Secondary market transfer of Series B Preferred Stock",
	}
}

// --- tests ---

func TestCorporationTradePreferredStock(t *testing.T) {
	svc := NewService(newMemRepo())
	ctx := context.Background()

	trade := baseTrade("txn_corp_001")
	trade.Security.IssuerType = IssuerCorporation
	trade.Security.SecurityClass = SecurityPreferredStock

	if err := svc.InitiateTrade(ctx, trade); err != nil {
		t.Fatalf("InitiateTrade: %v", err)
	}

	got, err := svc.repo.GetTrade(ctx, "txn_corp_001")
	if err != nil {
		t.Fatalf("GetTrade: %v", err)
	}
	if got.Status != TradeStatusPendingComplianceClearance {
		t.Fatalf("status = %s, want %s", got.Status, TradeStatusPendingComplianceClearance)
	}
	if got.TransactionType != TransactionSecondaryMarketTransfer {
		t.Fatalf("type = %s, want %s", got.TransactionType, TransactionSecondaryMarketTransfer)
	}
	if got.Security.IssuerType != IssuerCorporation {
		t.Fatalf("issuer_type = %s, want %s", got.Security.IssuerType, IssuerCorporation)
	}

	// Approve and execute.
	if err := svc.ApproveTrade(ctx, "txn_corp_001"); err != nil {
		t.Fatalf("ApproveTrade: %v", err)
	}

	event, err := svc.ExecuteTrade(ctx, "txn_corp_001", "0xabc123")
	if err != nil {
		t.Fatalf("ExecuteTrade: %v", err)
	}
	if event.EventType != "trade.executed" {
		t.Fatalf("event_type = %s, want trade.executed", event.EventType)
	}
	if len(event.Recipients) != 3 {
		t.Fatalf("recipients = %d, want 3", len(event.Recipients))
	}

	got, _ = svc.repo.GetTrade(ctx, "txn_corp_001")
	if got.Status != TradeStatusExecuted {
		t.Fatalf("status = %s, want %s", got.Status, TradeStatusExecuted)
	}
	if got.BlockchainTxHash != "0xabc123" {
		t.Fatalf("blockchain_tx_hash = %s, want 0xabc123", got.BlockchainTxHash)
	}
}

func TestLLCTradeMembershipUnits(t *testing.T) {
	svc := NewService(newMemRepo())
	ctx := context.Background()

	trade := baseTrade("txn_llc_001")
	trade.Security.IssuerType = IssuerLLC
	trade.Security.SecurityClass = SecurityMembershipUnits
	trade.Security.AssetName = "Acme Holdings LLC - Class A Units"
	trade.Security.IssuerName = "Acme Holdings LLC"
	trade.Security.NumberOfShares = 500
	trade.Security.PricePerShare = 30.00
	trade.Security.GrossAmount = 15000.00
	trade.Restrictions.Rule144HoldingPeriodMet = false
	trade.Restrictions.TransferRestrictions = "Subject to operating agreement transfer restrictions"

	if err := svc.InitiateTrade(ctx, trade); err != nil {
		t.Fatalf("InitiateTrade: %v", err)
	}

	got, _ := svc.repo.GetTrade(ctx, "txn_llc_001")
	if got.Security.IssuerType != IssuerLLC {
		t.Fatalf("issuer_type = %s, want %s", got.Security.IssuerType, IssuerLLC)
	}
	if got.Security.SecurityClass != SecurityMembershipUnits {
		t.Fatalf("security_class = %s, want %s", got.Security.SecurityClass, SecurityMembershipUnits)
	}

	if err := svc.ApproveTrade(ctx, "txn_llc_001"); err != nil {
		t.Fatalf("ApproveTrade: %v", err)
	}
	event, err := svc.ExecuteTrade(ctx, "txn_llc_001", "0xdef456")
	if err != nil {
		t.Fatalf("ExecuteTrade: %v", err)
	}
	if event.BlockchainTxHash != "0xdef456" {
		t.Fatalf("blockchain_tx = %s, want 0xdef456", event.BlockchainTxHash)
	}
}

func TestSPVTradeMembershipInterests(t *testing.T) {
	svc := NewService(newMemRepo())
	ctx := context.Background()

	// SPV with lockup that has already expired.
	pastLockup := time.Now().Add(-24 * time.Hour)
	trade := baseTrade("txn_spv_001")
	trade.Security.IssuerType = IssuerSPV
	trade.Security.SecurityClass = SecurityMembershipInterests
	trade.Security.AssetName = "Irongate SPV I LLC - Membership Interests"
	trade.Security.IssuerName = "Irongate SPV I LLC"
	trade.Security.NumberOfShares = 100
	trade.Security.PricePerShare = 1000.00
	trade.Security.GrossAmount = 100000.00
	trade.Restrictions.LockupExpiryDate = &pastLockup
	trade.Restrictions.TransferRestrictions = "Transfer permitted only with managing member written consent"

	if err := svc.InitiateTrade(ctx, trade); err != nil {
		t.Fatalf("InitiateTrade: %v", err)
	}

	got, _ := svc.repo.GetTrade(ctx, "txn_spv_001")
	if got.Security.IssuerType != IssuerSPV {
		t.Fatalf("issuer_type = %s, want %s", got.Security.IssuerType, IssuerSPV)
	}
	if got.Security.SecurityClass != SecurityMembershipInterests {
		t.Fatalf("security_class = %s, want %s", got.Security.SecurityClass, SecurityMembershipInterests)
	}

	if err := svc.ApproveTrade(ctx, "txn_spv_001"); err != nil {
		t.Fatalf("ApproveTrade: %v", err)
	}
	event, err := svc.ExecuteTrade(ctx, "txn_spv_001", "0x9d0e1f")
	if err != nil {
		t.Fatalf("ExecuteTrade: %v", err)
	}
	if event.Version != "2.0.0" {
		t.Fatalf("version = %s, want 2.0.0", event.Version)
	}
}

func TestTradeRule144RestrictionCheck(t *testing.T) {
	svc := NewService(newMemRepo())
	ctx := context.Background()

	// Trade with legend-required shares, Rule 144 not met, and NO transfer agent ack.
	trade := baseTrade("txn_rule144_001")
	trade.Restrictions.LegendRequired = true
	trade.Restrictions.Rule144HoldingPeriodMet = false
	trade.TransferAgentAck.Acknowledged = false

	if err := svc.InitiateTrade(ctx, trade); err != nil {
		t.Fatalf("InitiateTrade: %v", err)
	}
	if err := svc.ApproveTrade(ctx, "txn_rule144_001"); err != nil {
		t.Fatalf("ApproveTrade: %v", err)
	}

	// Should fail at execution due to Rule 144 restriction.
	_, err := svc.ExecuteTrade(ctx, "txn_rule144_001", "0xbadbeef")
	if err == nil {
		t.Fatal("expected error: Rule 144 restriction should block trade without TA ack")
	}

	// Verify trade was NOT moved to executed.
	got, _ := svc.repo.GetTrade(ctx, "txn_rule144_001")
	if got.Status != TradeStatusPendingTransfer {
		t.Fatalf("status = %s, want %s (trade should remain pending)", got.Status, TradeStatusPendingTransfer)
	}
}

func TestTradeLockupPeriodBlocking(t *testing.T) {
	svc := NewService(newMemRepo())
	ctx := context.Background()

	// Trade with active lockup period (expires in the future).
	futureLockup := time.Now().Add(365 * 24 * time.Hour)
	trade := baseTrade("txn_lockup_001")
	trade.Restrictions.LockupExpiryDate = &futureLockup
	// Transfer agent has acknowledged, Rule 144 is met - but lockup blocks.
	trade.Restrictions.LegendRequired = false
	trade.Restrictions.Rule144HoldingPeriodMet = true
	trade.TransferAgentAck.Acknowledged = true

	if err := svc.InitiateTrade(ctx, trade); err != nil {
		t.Fatalf("InitiateTrade: %v", err)
	}
	if err := svc.ApproveTrade(ctx, "txn_lockup_001"); err != nil {
		t.Fatalf("ApproveTrade: %v", err)
	}

	// Should fail at execution due to lockup.
	_, err := svc.ExecuteTrade(ctx, "txn_lockup_001", "0xlockedout")
	if err == nil {
		t.Fatal("expected error: lockup period should block trade")
	}

	// Verify trade stays in pending_transfer.
	got, _ := svc.repo.GetTrade(ctx, "txn_lockup_001")
	if got.Status != TradeStatusPendingTransfer {
		t.Fatalf("status = %s, want %s", got.Status, TradeStatusPendingTransfer)
	}
}

func TestInitiateTradeValidation(t *testing.T) {
	svc := NewService(newMemRepo())
	ctx := context.Background()

	tests := []struct {
		name  string
		setup func(*SecondaryTrade)
	}{
		{"empty transaction_id", func(tr *SecondaryTrade) { tr.TransactionID = "" }},
		{"empty buyer_investor_id", func(tr *SecondaryTrade) { tr.BuyerInvestorID = "" }},
		{"empty seller_investor_id", func(tr *SecondaryTrade) { tr.SellerInvestorID = "" }},
		{"empty buyer BD firm", func(tr *SecondaryTrade) { tr.BuyerBD.FirmName = "" }},
		{"empty seller BD CRD", func(tr *SecondaryTrade) { tr.SellerBD.CRDNumber = "" }},
		{"empty buyer compliance", func(tr *SecondaryTrade) { tr.BuyerCompliance.Endpoint = "" }},
		{"empty seller compliance", func(tr *SecondaryTrade) { tr.SellerCompliance.Endpoint = "" }},
		{"zero shares", func(tr *SecondaryTrade) { tr.Security.NumberOfShares = 0 }},
		{"negative price", func(tr *SecondaryTrade) { tr.Security.PricePerShare = -1 }},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			trade := baseTrade("txn_val_" + tt.name)
			tt.setup(trade)
			if err := svc.InitiateTrade(ctx, trade); err == nil {
				t.Errorf("expected error for %s", tt.name)
			}
		})
	}
}

func TestApproveTradeWrongStatus(t *testing.T) {
	svc := NewService(newMemRepo())
	ctx := context.Background()

	trade := baseTrade("txn_bad_approve")
	if err := svc.InitiateTrade(ctx, trade); err != nil {
		t.Fatalf("InitiateTrade: %v", err)
	}
	// Approve once.
	if err := svc.ApproveTrade(ctx, "txn_bad_approve"); err != nil {
		t.Fatalf("ApproveTrade: %v", err)
	}
	// Approve again should fail.
	if err := svc.ApproveTrade(ctx, "txn_bad_approve"); err == nil {
		t.Fatal("expected error: trade is no longer pending_compliance_clearance")
	}
}

func TestWebhookEventStructure(t *testing.T) {
	svc := NewService(newMemRepo())

	trade := baseTrade("txn_webhook_001")
	trade.BlockchainTxHash = "0xhash123"

	event := svc.CreateWebhookEvent(trade)

	if event.EventID != "evt_txn_webhook_001" {
		t.Fatalf("event_id = %s, want evt_txn_webhook_001", event.EventID)
	}
	if event.EventType != "trade.executed" {
		t.Fatalf("event_type = %s, want trade.executed", event.EventType)
	}
	if event.Version != "2.0.0" {
		t.Fatalf("version = %s, want 2.0.0", event.Version)
	}
	if event.TransactionType != "trade" {
		t.Fatalf("transaction_type = %s, want trade", event.TransactionType)
	}
	if event.BlockchainTxHash != "0xhash123" {
		t.Fatalf("blockchain_tx = %s, want 0xhash123", event.BlockchainTxHash)
	}
	if len(event.Recipients) != 3 {
		t.Fatalf("recipients = %d, want 3", len(event.Recipients))
	}

	roles := map[WebhookRecipientRole]bool{}
	for _, r := range event.Recipients {
		roles[r.Role] = true
	}
	if !roles[RoleBuyerBrokerDealer] {
		t.Error("missing buyer_broker_dealer recipient")
	}
	if !roles[RoleSellerBrokerDealer] {
		t.Error("missing seller_broker_dealer recipient")
	}
	if !roles[RoleTransferAgent] {
		t.Error("missing transfer_agent recipient")
	}
}
