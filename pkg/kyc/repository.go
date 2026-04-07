package kyc

import "context"

// Repository defines storage for KYC/AML/accreditation operations.
type Repository interface {
	CreateSubmission(ctx context.Context, s *Submission) error
	GetSubmission(ctx context.Context, id string) (*Submission, error)
	UpdateSubmission(ctx context.Context, s *Submission) error
	ListSubmissions(ctx context.Context, stakeholderID string) ([]*Submission, error)
	ListSubmissionsByCompany(ctx context.Context, companyID string) ([]*Submission, error)

	CreateScreening(ctx context.Context, s *AMLScreening) error
	GetScreening(ctx context.Context, id string) (*AMLScreening, error)
	ListScreenings(ctx context.Context, stakeholderID string) ([]*AMLScreening, error)

	CreateAccreditation(ctx context.Context, a *AccreditationVerification) error
	GetAccreditation(ctx context.Context, id string) (*AccreditationVerification, error)
	UpdateAccreditation(ctx context.Context, a *AccreditationVerification) error
	ListAccreditations(ctx context.Context, stakeholderID string) ([]*AccreditationVerification, error)
}
