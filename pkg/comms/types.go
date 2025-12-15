package comms

import "time"

// Notice is an investor communication (proxy notice, dividend announcement, regulatory update).
type Notice struct {
	ID         string            `json:"id"`
	CompanyID  string            `json:"company_id"`
	Subject    string            `json:"subject"`
	Body       string            `json:"body"`
	Type       string            `json:"type"` // general, proxy, dividend, regulatory, k1
	Recipients []NoticeRecipient `json:"recipients,omitempty"`
	SentAt     *time.Time        `json:"sent_at,omitempty"`
	CreatedAt  time.Time         `json:"created_at"`
}

// NoticeRecipient tracks delivery to a specific stakeholder.
type NoticeRecipient struct {
	StakeholderID string     `json:"stakeholder_id"`
	Email         string     `json:"email"`
	DeliveredAt   *time.Time `json:"delivered_at,omitempty"`
	ReadAt        *time.Time `json:"read_at,omitempty"`
}

// NoticeFilter for listing notices.
type NoticeFilter struct {
	CompanyID string `json:"company_id,omitempty"`
	Type      string `json:"type,omitempty"`
}
