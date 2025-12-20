package document

import "time"

// Document represents a document in a data room or disclosure package.
type Document struct {
	ID            string    `json:"id"`
	CompanyID     string    `json:"company_id"`
	SecurityID    string    `json:"security_id,omitempty"`
	StakeholderID string    `json:"stakeholder_id,omitempty"`
	Name          string    `json:"name"`
	Type          string    `json:"type"` // subscription_agreement, operating_agreement, bylaws, board_resolution, investor_letter, side_letter, disclosure, tax_form, certificate
	Category      string    `json:"category"` // legal, financial, corporate, tax, disclosure
	MimeType      string    `json:"mime_type"`
	Size          int64     `json:"size"` // bytes
	StorageRef    string    `json:"storage_ref"` // reference to external storage (S3 key, etc.)
	Hash          string    `json:"hash,omitempty"` // SHA-256 content hash
	Version       int       `json:"version"`
	Status        string    `json:"status"` // draft, final, executed, superseded
	RequiresSign  bool      `json:"requires_sign"`
	SignedAt      *time.Time `json:"signed_at,omitempty"`
	SignedBy      string    `json:"signed_by,omitempty"`
	CreatedAt     time.Time `json:"created_at"`
	UpdatedAt     time.Time `json:"updated_at"`
}

// DataRoom is a collection of documents for an offering or transaction.
type DataRoom struct {
	ID          string    `json:"id"`
	CompanyID   string    `json:"company_id"`
	SecurityID  string    `json:"security_id,omitempty"`
	Name        string    `json:"name"`
	Description string    `json:"description,omitempty"`
	Status      string    `json:"status"` // open, closed, archived
	CreatedAt   time.Time `json:"created_at"`
}

// AccessGrant controls who can view documents in a data room.
type AccessGrant struct {
	ID            string     `json:"id"`
	DataRoomID    string     `json:"data_room_id"`
	StakeholderID string     `json:"stakeholder_id"`
	Permission    string     `json:"permission"` // view, download, upload
	GrantedAt     time.Time  `json:"granted_at"`
	ExpiresAt     *time.Time `json:"expires_at,omitempty"`
	Revoked       bool       `json:"revoked"`
}

// AuditEntry tracks document access for compliance.
type AuditEntry struct {
	ID            string    `json:"id"`
	DocumentID    string    `json:"document_id"`
	StakeholderID string    `json:"stakeholder_id"`
	Action        string    `json:"action"` // view, download, sign
	IPAddress     string    `json:"ip_address,omitempty"`
	UserAgent     string    `json:"user_agent,omitempty"`
	Timestamp     time.Time `json:"timestamp"`
}
