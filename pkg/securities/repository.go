package securities

import "context"

// Repository defines storage for securities operations.
type Repository interface {
	CreateSecurity(ctx context.Context, s *Security) error
	GetSecurity(ctx context.Context, id string) (*Security, error)
	UpdateSecurity(ctx context.Context, s *Security) error
	ListSecurities(ctx context.Context, companyID string) ([]*Security, error)

	CreateCertificate(ctx context.Context, c *Certificate) error
	GetCertificate(ctx context.Context, id string) (*Certificate, error)
	UpdateCertificate(ctx context.Context, c *Certificate) error
	ListCertificates(ctx context.Context, securityID string) ([]*Certificate, error)

	AppendLedger(ctx context.Context, entry *LedgerEntry) error
	ListLedger(ctx context.Context, securityID string) ([]*LedgerEntry, error)
	ListLedgerByCompany(ctx context.Context, companyID string) ([]*LedgerEntry, error)
}
