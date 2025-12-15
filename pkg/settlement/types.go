package settlement

import "time"

// IssuerType is the legal entity type of the security issuer.
type IssuerType string

const (
	IssuerCorporation         IssuerType = "corporation"
	IssuerLLC                 IssuerType = "llc"
	IssuerTrust               IssuerType = "trust"
	IssuerPartnership         IssuerType = "partnership"
	IssuerSoleProprietorship  IssuerType = "sole_proprietorship"
	IssuerPublicCompany       IssuerType = "public_company"
	IssuerSPV                 IssuerType = "spv"
)

// SecurityClass is the classification of the traded security.
type SecurityClass string

const (
	SecurityPreferredStock      SecurityClass = "preferred_stock"
	SecurityCommonStock         SecurityClass = "common_stock"
	SecurityMembershipUnits     SecurityClass = "membership_units"
	SecurityMembershipInterests SecurityClass = "membership_interests"
	SecurityLPUnits             SecurityClass = "lp_units"
	SecurityCommonShares        SecurityClass = "common_shares"
)

// TransactionType is the type of a secondary market transaction.
type TransactionType string

const (
	TransactionSecondaryMarketTransfer TransactionType = "secondary_market_transfer"
)

// TradeStatus represents the lifecycle status of a trade.
type TradeStatus string

const (
	TradeStatusPendingComplianceClearance TradeStatus = "pending_compliance_clearance"
	TradeStatusPendingTransfer           TradeStatus = "pending_transfer"
	TradeStatusExecuted                  TradeStatus = "executed"
	TradeStatusFailed                    TradeStatus = "failed"
	TradeStatusCancelled                 TradeStatus = "cancelled"
)

// CommissionType indicates how a commission is calculated.
type CommissionType string

const (
	CommissionFlatFee          CommissionType = "flat_fee"
	CommissionPercentageGross  CommissionType = "percentage_of_gross"
	CommissionPerShare         CommissionType = "per_share"
)

// SettlementType is the method of trade settlement.
type SettlementType string

const (
	SettlementBilateral   SettlementType = "bilateral"
	SettlementDVP         SettlementType = "dvp"
	SettlementFreeDelivery SettlementType = "free_delivery"
)

// SettlementStatus tracks the state of a settlement.
type SettlementStatus string

const (
	SettlementPending SettlementStatus = "pending"
	SettlementSettled SettlementStatus = "settled"
	SettlementFailed  SettlementStatus = "failed"
)

// WebhookRecipientRole identifies a webhook recipient's function.
type WebhookRecipientRole string

const (
	RoleBuyerBrokerDealer  WebhookRecipientRole = "buyer_broker_dealer"
	RoleSellerBrokerDealer WebhookRecipientRole = "seller_broker_dealer"
	RoleTransferAgent      WebhookRecipientRole = "transfer_agent"
)

// BrokerDealer is a FINRA-registered broker-dealer acting on behalf of a party.
type BrokerDealer struct {
	FirmName    string `json:"firm_name"`
	CRDNumber   string `json:"crd_number"`
	FINRAMember bool   `json:"finra_member"`
}

// TransferAgent is the SEC-registered transfer agent for the security.
type TransferAgent struct {
	FirmName              string `json:"firm_name"`
	SECRegistered         bool   `json:"sec_registered"`
	SECRegistrationNumber string `json:"sec_registration_number"`
}

// TransferAgentAck is the transfer agent's acknowledgment of a trade instruction.
type TransferAgentAck struct {
	Acknowledged          bool      `json:"acknowledged"`
	AcknowledgedAt        time.Time `json:"acknowledged_at"`
	TransferInstructionID string    `json:"transfer_instruction_id"`
	RecordDate            time.Time `json:"record_date"`
	UnitsToTransfer       int64     `json:"units_to_transfer"`
	Status                string    `json:"status"`
}

// Commission is a single broker-dealer commission on a trade.
type Commission struct {
	FirmName         string         `json:"firm_name"`
	CRDNumber        string         `json:"crd_number"`
	CommissionType   CommissionType `json:"commission_type"`
	CommissionRate   *float64       `json:"commission_rate"`   // e.g., 0.005 for 0.5%; nil for flat_fee
	CommissionAmount string         `json:"commission_amount"` // computed amount in currency
	Currency         string         `json:"currency"`
}

// TradeCommissions contains the buyer and seller broker-dealer commissions.
type TradeCommissions struct {
	BuyerBD          Commission `json:"buyer_broker_dealer"`
	SellerBD         Commission `json:"seller_broker_dealer"`
	TotalCommissions string     `json:"total_commissions"`
}

// TradeRestrictions defines the regulatory restrictions on a trade.
type TradeRestrictions struct {
	LegendRequired          bool       `json:"legend_required"`
	Rule144HoldingPeriodMet bool       `json:"rule_144_holding_period_met"`
	TransferRestrictions    string     `json:"transfer_restrictions"`
	LockupExpiryDate        *time.Time `json:"lock_up_expiry_date,omitempty"`
}

// TradeSettlement contains the settlement terms of the trade.
type TradeSettlement struct {
	SettlementDate time.Time        `json:"settlement_date"`
	SettlementType SettlementType   `json:"settlement_type"`
	Status         SettlementStatus `json:"status"`
	Currency       string           `json:"currency"`
}

// SecurityDetail describes the security being traded.
type SecurityDetail struct {
	AssetID        string        `json:"asset_id"`
	AssetName      string        `json:"asset_name"`
	SecurityClass  SecurityClass `json:"security_class"`
	ShareClass     string        `json:"share_class"`
	CUSIP          string        `json:"cusip,omitempty"`
	ISIN           string        `json:"isin,omitempty"`
	IssuerID       string        `json:"issuer_id"`
	IssuerName     string        `json:"issuer_name"`
	IssuerType     IssuerType    `json:"issuer_type"`
	NumberOfShares int64         `json:"number_of_shares"`
	PricePerShare  float64       `json:"price_per_share"`
	Currency       string        `json:"currency"`
	GrossAmount    float64       `json:"gross_trade_amount"`
}

// ComplianceRef is a reference to an investor's compliance data endpoint.
type ComplianceRef struct {
	Endpoint    string `json:"endpoint"`
	Description string `json:"description"`
}

// SecondaryTrade is a cross-broker-dealer secondary market trade.
type SecondaryTrade struct {
	TransactionID    string           `json:"transaction_id"`
	TransactionType  TransactionType  `json:"transaction_type"`
	Status           TradeStatus      `json:"status"`
	Security         SecurityDetail   `json:"security"`
	BuyerInvestorID  string           `json:"buyer_investor_id"`
	SellerInvestorID string           `json:"seller_investor_id"`
	BuyerAccountID   string           `json:"buyer_account_id"`
	SellerAccountID  string           `json:"seller_account_id"`
	BuyerBD          BrokerDealer     `json:"buyer_broker_dealer"`
	SellerBD         BrokerDealer     `json:"seller_broker_dealer"`
	BuyerCompliance  ComplianceRef    `json:"buyer_compliance_ref"`
	SellerCompliance ComplianceRef    `json:"seller_compliance_ref"`
	TransferAgent    TransferAgent    `json:"transfer_agent"`
	TransferAgentAck TransferAgentAck `json:"transfer_agent_ack"`
	Commissions      TradeCommissions `json:"commissions"`
	Settlement       TradeSettlement  `json:"settlement"`
	Restrictions     TradeRestrictions `json:"restrictions"`
	BlockchainTxHash string           `json:"blockchain_tx_hash,omitempty"`
	Description      string           `json:"description"`
	InitiatedAt      time.Time        `json:"initiated_at"`
	ExecutedAt       *time.Time       `json:"executed_at,omitempty"`
}

// WebhookRecipient is a party notified via webhook of trade events.
type WebhookRecipient struct {
	RecipientID    string               `json:"recipient_id"`
	Name           string               `json:"name"`
	Role           WebhookRecipientRole `json:"role"`
	DeliveredAt    *time.Time           `json:"delivered_at,omitempty"`
	DeliveryStatus string               `json:"delivery_status"` // pending, delivered, failed
}

// WebhookEvent is the webhook notification payload for trade events.
type WebhookEvent struct {
	EventID          string             `json:"event_id"`
	EventType        string             `json:"event_type"` // trade.executed, trade.settled, trade.failed
	Timestamp        time.Time          `json:"timestamp"`
	Version          string             `json:"version"`
	TransactionType  string             `json:"transaction_type"`
	BlockchainTxHash string             `json:"blockchain_transaction_id"`
	Recipients       []WebhookRecipient `json:"recipients"`
}
