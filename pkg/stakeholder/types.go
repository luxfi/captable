package stakeholder

import "time"

// Stakeholder represents an investor, founder, employee, or other equity holder.
type Stakeholder struct {
	ID            string            `json:"id"`
	CompanyID     string            `json:"company_id"`
	TenantID      string            `json:"tenant_id"`
	Type          string            `json:"type"` // individual, entity, trust
	Role          string            `json:"role"` // founder, investor, employee, advisor, board_member
	Name          string            `json:"name"`
	Email         string            `json:"email,omitempty"`
	Phone         string            `json:"phone,omitempty"`
	TaxID         string            `json:"tax_id,omitempty"`
	TaxIDType     string            `json:"tax_id_type,omitempty"` // ssn, ein, itin
	Address       *Address          `json:"address,omitempty"`
	Accreditation *Accreditation    `json:"accreditation,omitempty"`
	Status        string            `json:"status"` // active, inactive, departed
	Meta          map[string]string `json:"meta,omitempty"`
	CreatedAt     time.Time         `json:"created_at"`
	UpdatedAt     time.Time         `json:"updated_at"`
}

// Address is a physical or mailing address.
type Address struct {
	Street1    string `json:"street1"`
	Street2    string `json:"street2,omitempty"`
	City       string `json:"city"`
	State      string `json:"state"`
	PostalCode string `json:"postal_code"`
	Country    string `json:"country"`
}

// Accreditation tracks whether a stakeholder is an accredited investor.
type Accreditation struct {
	IsAccredited      bool       `json:"is_accredited"`
	Method            string     `json:"method,omitempty"`    // income, net_worth, professional, entity
	VerifiedAt        *time.Time `json:"verified_at,omitempty"`
	ExpiresAt         *time.Time `json:"expires_at,omitempty"`
	VerificationDoc   string     `json:"verification_doc,omitempty"`
	QualifiedPurchaser bool      `json:"qualified_purchaser,omitempty"`
}

// Holding is a computed view of a stakeholder's position across all share classes.
type Holding struct {
	StakeholderID    string           `json:"stakeholder_id"`
	CompanyID        string           `json:"company_id"`
	TotalShares      int64            `json:"total_shares"`
	TotalValue       float64          `json:"total_value"`
	OwnershipPercent float64          `json:"ownership_percent"`
	ByClass          []ClassHolding   `json:"by_class"`
}

// ClassHolding is a per-share-class holding for a stakeholder.
type ClassHolding struct {
	ShareClassID string  `json:"share_class_id"`
	ClassName    string  `json:"class_name"`
	Shares       int64   `json:"shares"`
	Value        float64 `json:"value"`
	Percent      float64 `json:"percent"`
}
