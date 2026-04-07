package kyc

import (
	"context"
	"fmt"
	"time"
)

// Service handles KYC, AML screening, and accreditation verification.
type Service struct {
	repo Repository
}

// NewService creates a KYC service backed by the given repository.
func NewService(repo Repository) *Service {
	return &Service{repo: repo}
}

// CreateSubmission validates and persists a new KYC/KYB submission.
func (s *Service) CreateSubmission(ctx context.Context, sub *Submission) error {
	if sub.StakeholderID == "" {
		return fmt.Errorf("stakeholder_id is required")
	}
	if sub.CompanyID == "" {
		return fmt.Errorf("company_id is required")
	}
	if sub.Type == "" {
		return fmt.Errorf("type is required")
	}
	now := time.Now().UTC()
	sub.CreatedAt = now
	sub.UpdatedAt = now
	sub.SubmittedAt = now
	if sub.Status == "" {
		sub.Status = StatusPending
	}
	return s.repo.CreateSubmission(ctx, sub)
}

// GetSubmission retrieves a KYC submission by ID.
func (s *Service) GetSubmission(ctx context.Context, id string) (*Submission, error) {
	if id == "" {
		return nil, fmt.Errorf("submission id is required")
	}
	return s.repo.GetSubmission(ctx, id)
}

// Approve marks a KYC submission as approved.
func (s *Service) Approve(ctx context.Context, id string) error {
	sub, err := s.repo.GetSubmission(ctx, id)
	if err != nil {
		return fmt.Errorf("get submission: %w", err)
	}
	if sub.Status != StatusPending {
		return fmt.Errorf("submission %s is not pending (status=%s)", sub.ID, sub.Status)
	}
	now := time.Now().UTC()
	sub.Status = StatusApproved
	sub.ReviewedAt = &now
	sub.UpdatedAt = now
	return s.repo.UpdateSubmission(ctx, sub)
}

// Reject marks a KYC submission as rejected.
func (s *Service) Reject(ctx context.Context, id string, reason string) error {
	sub, err := s.repo.GetSubmission(ctx, id)
	if err != nil {
		return fmt.Errorf("get submission: %w", err)
	}
	if sub.Status != StatusPending {
		return fmt.Errorf("submission %s is not pending (status=%s)", sub.ID, sub.Status)
	}
	if reason == "" {
		return fmt.Errorf("rejection reason is required")
	}
	now := time.Now().UTC()
	sub.Status = StatusRejected
	sub.ReviewedAt = &now
	sub.RejectionReason = reason
	sub.UpdatedAt = now
	return s.repo.UpdateSubmission(ctx, sub)
}

// RecordScreening persists an AML screening result.
func (s *Service) RecordScreening(ctx context.Context, scr *AMLScreening) error {
	if scr.StakeholderID == "" {
		return fmt.Errorf("stakeholder_id is required")
	}
	if scr.CompanyID == "" {
		return fmt.Errorf("company_id is required")
	}
	scr.ScreenedAt = time.Now().UTC()
	scr.CreatedAt = scr.ScreenedAt
	if scr.Status == "" {
		scr.Status = ScreeningPending
	}
	return s.repo.CreateScreening(ctx, scr)
}

// RecordAccreditation persists an accreditation verification result.
func (s *Service) RecordAccreditation(ctx context.Context, a *AccreditationVerification) error {
	if a.StakeholderID == "" {
		return fmt.Errorf("stakeholder_id is required")
	}
	if a.CompanyID == "" {
		return fmt.Errorf("company_id is required")
	}
	if a.Method == "" {
		return fmt.Errorf("method is required")
	}
	now := time.Now().UTC()
	a.CreatedAt = now
	a.UpdatedAt = now
	if a.Status == "" {
		a.Status = StatusPending
	}
	return s.repo.CreateAccreditation(ctx, a)
}

// ListSubmissions returns all KYC submissions for a stakeholder.
func (s *Service) ListSubmissions(ctx context.Context, stakeholderID string) ([]*Submission, error) {
	if stakeholderID == "" {
		return nil, fmt.Errorf("stakeholder_id is required")
	}
	return s.repo.ListSubmissions(ctx, stakeholderID)
}
