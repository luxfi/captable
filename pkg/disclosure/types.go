package disclosure

import "time"

// Disclosure is an investor disclosure document (PPM, subscription agreement, supplement).
type Disclosure struct {
	ID          string                `json:"id"`
	CompanyID   string                `json:"company_id"`
	Name        string                `json:"name"`
	Type        string                `json:"type"` // ppm, subscription_agreement, supplement, annual_report
	DocumentURL string                `json:"document_url"`
	Recipients  []DisclosureRecipient `json:"recipients,omitempty"`
	CreatedAt   time.Time             `json:"created_at"`
}

// DisclosureRecipient tracks delivery and acknowledgment per stakeholder.
type DisclosureRecipient struct {
	StakeholderID  string     `json:"stakeholder_id"`
	DeliveredAt    *time.Time `json:"delivered_at,omitempty"`
	ViewedAt       *time.Time `json:"viewed_at,omitempty"`
	AcknowledgedAt *time.Time `json:"acknowledged_at,omitempty"`
}

// DisclosureFilter for listing disclosures.
type DisclosureFilter struct {
	CompanyID     string `json:"company_id,omitempty"`
	Type          string `json:"type,omitempty"`
	StakeholderID string `json:"stakeholder_id,omitempty"`
}
