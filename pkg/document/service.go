package document

import (
	"context"
	"fmt"
	"time"
)

// Service handles document management, data rooms, access control, and audit trails.
type Service struct {
	repo Repository
}

// NewService creates a document service.
func NewService(repo Repository) *Service {
	return &Service{repo: repo}
}

// Upload registers a new document. The actual file storage is handled externally —
// this service manages metadata, access control, and audit.
func (s *Service) Upload(ctx context.Context, d *Document) error {
	if d.CompanyID == "" {
		return fmt.Errorf("company_id is required")
	}
	if d.Name == "" {
		return fmt.Errorf("name is required")
	}
	if d.Type == "" {
		return fmt.Errorf("type is required")
	}
	if d.StorageRef == "" {
		return fmt.Errorf("storage_ref is required")
	}
	now := time.Now().UTC()
	d.CreatedAt = now
	d.UpdatedAt = now
	if d.Version == 0 {
		d.Version = 1
	}
	if d.Status == "" {
		d.Status = "draft"
	}
	return s.repo.CreateDocument(ctx, d)
}

// Get retrieves a document by ID and records an audit entry.
func (s *Service) Get(ctx context.Context, docID, stakeholderID string) (*Document, error) {
	if docID == "" {
		return nil, fmt.Errorf("document id is required")
	}
	doc, err := s.repo.GetDocument(ctx, docID)
	if err != nil {
		return nil, err
	}

	// Record access.
	if stakeholderID != "" {
		s.repo.AppendAudit(ctx, &AuditEntry{
			ID:            fmt.Sprintf("aud_%s_%d", docID, time.Now().UnixNano()),
			DocumentID:    docID,
			StakeholderID: stakeholderID,
			Action:        "view",
			Timestamp:     time.Now().UTC(),
		})
	}

	return doc, nil
}

// List returns all documents for a company.
func (s *Service) List(ctx context.Context, companyID string) ([]*Document, error) {
	if companyID == "" {
		return nil, fmt.Errorf("company_id is required")
	}
	return s.repo.ListDocuments(ctx, companyID)
}

// MarkExecuted marks a document as signed/executed.
func (s *Service) MarkExecuted(ctx context.Context, docID, signedBy string) error {
	doc, err := s.repo.GetDocument(ctx, docID)
	if err != nil {
		return fmt.Errorf("get document: %w", err)
	}
	now := time.Now().UTC()
	doc.Status = "executed"
	doc.SignedAt = &now
	doc.SignedBy = signedBy
	doc.UpdatedAt = now
	return s.repo.UpdateDocument(ctx, doc)
}

// CreateDataRoom creates a new data room for an offering.
func (s *Service) CreateDataRoom(ctx context.Context, dr *DataRoom) error {
	if dr.CompanyID == "" {
		return fmt.Errorf("company_id is required")
	}
	if dr.Name == "" {
		return fmt.Errorf("name is required")
	}
	dr.CreatedAt = time.Now().UTC()
	if dr.Status == "" {
		dr.Status = "open"
	}
	return s.repo.CreateDataRoom(ctx, dr)
}

// GrantAccess gives a stakeholder access to a data room.
func (s *Service) GrantAccess(ctx context.Context, ag *AccessGrant) error {
	if ag.DataRoomID == "" {
		return fmt.Errorf("data_room_id is required")
	}
	if ag.StakeholderID == "" {
		return fmt.Errorf("stakeholder_id is required")
	}
	if ag.Permission == "" {
		ag.Permission = "view"
	}
	ag.GrantedAt = time.Now().UTC()
	return s.repo.CreateAccessGrant(ctx, ag)
}

// GetAuditTrail returns the access history for a document.
func (s *Service) GetAuditTrail(ctx context.Context, documentID string) ([]*AuditEntry, error) {
	if documentID == "" {
		return nil, fmt.Errorf("document_id is required")
	}
	return s.repo.ListAudit(ctx, documentID)
}
