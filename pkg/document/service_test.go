package document

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"testing"
)

type memRepo struct {
	mu           sync.RWMutex
	documents    map[string]*Document
	dataRooms    map[string]*DataRoom
	accessGrants map[string]*AccessGrant
	audit        []*AuditEntry
}

func newMemRepo() *memRepo {
	return &memRepo{
		documents:    make(map[string]*Document),
		dataRooms:    make(map[string]*DataRoom),
		accessGrants: make(map[string]*AccessGrant),
	}
}

func (r *memRepo) CreateDocument(_ context.Context, d *Document) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	cp := *d
	r.documents[d.ID] = &cp
	return nil
}

func (r *memRepo) GetDocument(_ context.Context, id string) (*Document, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	d, ok := r.documents[id]
	if !ok {
		return nil, fmt.Errorf("document %s not found", id)
	}
	cp := *d
	return &cp, nil
}

func (r *memRepo) UpdateDocument(_ context.Context, d *Document) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	cp := *d
	r.documents[d.ID] = &cp
	return nil
}

func (r *memRepo) ListDocuments(_ context.Context, companyID string) ([]*Document, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	var out []*Document
	for _, d := range r.documents {
		if d.CompanyID == companyID {
			cp := *d
			out = append(out, &cp)
		}
	}
	return out, nil
}

func (r *memRepo) ListByType(_ context.Context, companyID, docType string) ([]*Document, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	var out []*Document
	for _, d := range r.documents {
		if d.CompanyID == companyID && d.Type == docType {
			cp := *d
			out = append(out, &cp)
		}
	}
	return out, nil
}

func (r *memRepo) ListBySecurity(_ context.Context, securityID string) ([]*Document, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	var out []*Document
	for _, d := range r.documents {
		if d.SecurityID == securityID {
			cp := *d
			out = append(out, &cp)
		}
	}
	return out, nil
}

func (r *memRepo) ListByStakeholder(_ context.Context, stakeholderID string) ([]*Document, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	var out []*Document
	for _, d := range r.documents {
		if d.StakeholderID == stakeholderID {
			cp := *d
			out = append(out, &cp)
		}
	}
	return out, nil
}

func (r *memRepo) CreateDataRoom(_ context.Context, dr *DataRoom) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	cp := *dr
	r.dataRooms[dr.ID] = &cp
	return nil
}

func (r *memRepo) GetDataRoom(_ context.Context, id string) (*DataRoom, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	dr, ok := r.dataRooms[id]
	if !ok {
		return nil, fmt.Errorf("data room %s not found", id)
	}
	cp := *dr
	return &cp, nil
}

func (r *memRepo) ListDataRooms(_ context.Context, companyID string) ([]*DataRoom, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	var out []*DataRoom
	for _, dr := range r.dataRooms {
		if dr.CompanyID == companyID {
			cp := *dr
			out = append(out, &cp)
		}
	}
	return out, nil
}

func (r *memRepo) CreateAccessGrant(_ context.Context, ag *AccessGrant) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	cp := *ag
	r.accessGrants[ag.ID] = &cp
	return nil
}

func (r *memRepo) ListAccessGrants(_ context.Context, dataRoomID string) ([]*AccessGrant, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	var out []*AccessGrant
	for _, ag := range r.accessGrants {
		if ag.DataRoomID == dataRoomID {
			cp := *ag
			out = append(out, &cp)
		}
	}
	return out, nil
}

func (r *memRepo) RevokeAccessGrant(_ context.Context, id string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if ag, ok := r.accessGrants[id]; ok {
		ag.Revoked = true
	}
	return nil
}

func (r *memRepo) AppendAudit(_ context.Context, ae *AuditEntry) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	cp := *ae
	r.audit = append(r.audit, &cp)
	return nil
}

func (r *memRepo) ListAudit(_ context.Context, documentID string) ([]*AuditEntry, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	var out []*AuditEntry
	for _, ae := range r.audit {
		if ae.DocumentID == documentID {
			cp := *ae
			out = append(out, &cp)
		}
	}
	return out, nil
}

func TestUploadValidation(t *testing.T) {
	svc := NewService(newMemRepo())
	ctx := context.Background()

	tests := []struct {
		name    string
		doc     Document
		wantErr string
	}{
		{"missing company", Document{ID: "d1", Name: "X", Type: "disclosure", StorageRef: "s3://x"}, "company_id"},
		{"missing name", Document{ID: "d1", CompanyID: "c1", Type: "disclosure", StorageRef: "s3://x"}, "name"},
		{"missing type", Document{ID: "d1", CompanyID: "c1", Name: "X", StorageRef: "s3://x"}, "type"},
		{"missing storage", Document{ID: "d1", CompanyID: "c1", Name: "X", Type: "disclosure"}, "storage_ref"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := svc.Upload(ctx, &tt.doc)
			if err == nil {
				t.Fatal("expected error")
			}
			if !strings.Contains(err.Error(), tt.wantErr) {
				t.Fatalf("error %q does not contain %q", err.Error(), tt.wantErr)
			}
		})
	}
}

func TestUploadSuccess(t *testing.T) {
	svc := NewService(newMemRepo())
	ctx := context.Background()

	doc := &Document{
		ID:         "d1",
		CompanyID:  "c1",
		Name:       "Subscription Agreement",
		Type:       "subscription_agreement",
		MimeType:   "application/pdf",
		StorageRef: "s3://docs/sub-agreement.pdf",
		Size:       102400,
	}
	if err := svc.Upload(ctx, doc); err != nil {
		t.Fatalf("Upload: %v", err)
	}
	if doc.Status != "draft" {
		t.Fatalf("status = %q, want draft", doc.Status)
	}
	if doc.Version != 1 {
		t.Fatalf("version = %d, want 1", doc.Version)
	}
}

func TestGetRecordsAudit(t *testing.T) {
	repo := newMemRepo()
	svc := NewService(repo)
	ctx := context.Background()

	repo.CreateDocument(ctx, &Document{
		ID: "d1", CompanyID: "c1", Name: "Doc", Type: "disclosure", StorageRef: "s3://x",
	})

	_, err := svc.Get(ctx, "d1", "sh1")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}

	trail, _ := repo.ListAudit(ctx, "d1")
	if len(trail) != 1 {
		t.Fatalf("audit trail len = %d, want 1", len(trail))
	}
	if trail[0].Action != "view" {
		t.Fatalf("action = %q, want view", trail[0].Action)
	}
	if trail[0].StakeholderID != "sh1" {
		t.Fatalf("stakeholder = %q, want sh1", trail[0].StakeholderID)
	}
}

func TestMarkExecuted(t *testing.T) {
	repo := newMemRepo()
	svc := NewService(repo)
	ctx := context.Background()

	repo.CreateDocument(ctx, &Document{
		ID: "d1", CompanyID: "c1", Name: "Doc", Type: "subscription_agreement",
		StorageRef: "s3://x", Status: "draft",
	})

	if err := svc.MarkExecuted(ctx, "d1", "Jane Doe"); err != nil {
		t.Fatalf("MarkExecuted: %v", err)
	}

	doc, _ := repo.GetDocument(ctx, "d1")
	if doc.Status != "executed" {
		t.Fatalf("status = %q, want executed", doc.Status)
	}
	if doc.SignedBy != "Jane Doe" {
		t.Fatalf("signed_by = %q, want Jane Doe", doc.SignedBy)
	}
	if doc.SignedAt == nil {
		t.Fatal("signed_at should be set")
	}
}

func TestCreateDataRoom(t *testing.T) {
	svc := NewService(newMemRepo())
	ctx := context.Background()

	dr := &DataRoom{
		ID:        "dr1",
		CompanyID: "c1",
		Name:      "Series A Data Room",
	}
	if err := svc.CreateDataRoom(ctx, dr); err != nil {
		t.Fatalf("CreateDataRoom: %v", err)
	}
	if dr.Status != "open" {
		t.Fatalf("status = %q, want open", dr.Status)
	}
}

func TestGrantAccess(t *testing.T) {
	svc := NewService(newMemRepo())
	ctx := context.Background()

	ag := &AccessGrant{
		ID:            "ag1",
		DataRoomID:    "dr1",
		StakeholderID: "sh1",
	}
	if err := svc.GrantAccess(ctx, ag); err != nil {
		t.Fatalf("GrantAccess: %v", err)
	}
	if ag.Permission != "view" {
		t.Fatalf("permission = %q, want view (default)", ag.Permission)
	}
}
