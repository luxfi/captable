package document

import "context"

// Repository defines storage for document operations.
type Repository interface {
	CreateDocument(ctx context.Context, d *Document) error
	GetDocument(ctx context.Context, id string) (*Document, error)
	UpdateDocument(ctx context.Context, d *Document) error
	ListDocuments(ctx context.Context, companyID string) ([]*Document, error)
	ListByType(ctx context.Context, companyID, docType string) ([]*Document, error)
	ListBySecurity(ctx context.Context, securityID string) ([]*Document, error)
	ListByStakeholder(ctx context.Context, stakeholderID string) ([]*Document, error)

	CreateDataRoom(ctx context.Context, dr *DataRoom) error
	GetDataRoom(ctx context.Context, id string) (*DataRoom, error)
	ListDataRooms(ctx context.Context, companyID string) ([]*DataRoom, error)

	CreateAccessGrant(ctx context.Context, ag *AccessGrant) error
	ListAccessGrants(ctx context.Context, dataRoomID string) ([]*AccessGrant, error)
	RevokeAccessGrant(ctx context.Context, id string) error

	AppendAudit(ctx context.Context, ae *AuditEntry) error
	ListAudit(ctx context.Context, documentID string) ([]*AuditEntry, error)
}
