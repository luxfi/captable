package kyc

import "time"

// SubmissionStatus tracks KYC submission lifecycle.
type SubmissionStatus string

const (
	StatusPending  SubmissionStatus = "pending"
	StatusApproved SubmissionStatus = "approved"
	StatusRejected SubmissionStatus = "rejected"
	StatusExpired  SubmissionStatus = "expired"
)

// SubmissionType distinguishes KYC from KYB.
type SubmissionType string

const (
	TypeKYC SubmissionType = "kyc" // individual
	TypeKYB SubmissionType = "kyb" // business entity
)

// ScreeningStatus tracks AML screening results.
type ScreeningStatus string

const (
	ScreeningClear   ScreeningStatus = "clear"
	ScreeningHit     ScreeningStatus = "hit"
	ScreeningPending ScreeningStatus = "pending"
	ScreeningError   ScreeningStatus = "error"
)

// AccreditationMethod defines how accreditation was verified.
type AccreditationMethod string

const (
	MethodIncome       AccreditationMethod = "income"
	MethodNetWorth     AccreditationMethod = "net_worth"
	MethodProfessional AccreditationMethod = "professional"
	MethodEntity       AccreditationMethod = "entity"
	MethodSelfCert     AccreditationMethod = "self_cert"
)

// Submission represents a KYC/KYB submission for a stakeholder.
type Submission struct {
	ID            string           `json:"id"`
	StakeholderID string           `json:"stakeholder_id"`
	CompanyID     string           `json:"company_id"`
	Type          SubmissionType   `json:"type"`
	Status        SubmissionStatus `json:"status"`
	ProviderID    string           `json:"provider_id,omitempty"`    // external provider reference
	ProviderName  string           `json:"provider_name,omitempty"` // e.g. "simplici"
	SubmittedAt   time.Time        `json:"submitted_at"`
	ReviewedAt    *time.Time       `json:"reviewed_at,omitempty"`
	ExpiresAt     *time.Time       `json:"expires_at,omitempty"`
	RejectionReason string         `json:"rejection_reason,omitempty"`
	CreatedAt     time.Time        `json:"created_at"`
	UpdatedAt     time.Time        `json:"updated_at"`
}

// AMLScreening represents an anti-money laundering screening result.
type AMLScreening struct {
	ID            string          `json:"id"`
	StakeholderID string          `json:"stakeholder_id"`
	CompanyID     string          `json:"company_id"`
	Status        ScreeningStatus `json:"status"`
	ProviderID    string          `json:"provider_id,omitempty"`
	ProviderName  string          `json:"provider_name,omitempty"`
	HitDetails    string          `json:"hit_details,omitempty"` // JSON or summary of matches
	ScreenedAt    time.Time       `json:"screened_at"`
	ExpiresAt     *time.Time      `json:"expires_at,omitempty"`
	CreatedAt     time.Time       `json:"created_at"`
}

// AccreditationVerification records a verified accreditation check.
type AccreditationVerification struct {
	ID            string              `json:"id"`
	StakeholderID string              `json:"stakeholder_id"`
	CompanyID     string              `json:"company_id"`
	Status        SubmissionStatus    `json:"status"`
	Method        AccreditationMethod `json:"method"`
	ProviderID    string              `json:"provider_id,omitempty"`
	ProviderName  string              `json:"provider_name,omitempty"`
	DocumentID    string              `json:"document_id,omitempty"` // reference to uploaded proof
	IsQualifiedPurchaser bool         `json:"is_qualified_purchaser"`
	VerifiedAt    *time.Time          `json:"verified_at,omitempty"`
	ExpiresAt     *time.Time          `json:"expires_at,omitempty"`
	CreatedAt     time.Time           `json:"created_at"`
	UpdatedAt     time.Time           `json:"updated_at"`
}

// Provider is the interface for external KYC/AML providers.
type Provider interface {
	// SubmitKYC initiates a KYC/KYB check with the external provider.
	SubmitKYC(stakeholderID string, submissionType SubmissionType) (providerID string, err error)
	// CheckKYCStatus polls the provider for the current status of a submission.
	CheckKYCStatus(providerID string) (SubmissionStatus, error)
	// ScreenAML runs an AML screening against the provider.
	ScreenAML(stakeholderID string) (*AMLScreening, error)
	// VerifyAccreditation checks accreditation status with the provider.
	VerifyAccreditation(stakeholderID string, method AccreditationMethod) (*AccreditationVerification, error)
}
