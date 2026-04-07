package warrant

import (
	"context"
	"fmt"
	"time"
)

// Service handles warrant lifecycle management.
type Service struct {
	repo Repository
}

// NewService creates a warrant service backed by the given repository.
func NewService(repo Repository) *Service {
	return &Service{repo: repo}
}

// Create validates and persists a new warrant.
func (s *Service) Create(ctx context.Context, w *Warrant) error {
	if w.CompanyID == "" {
		return fmt.Errorf("company_id is required")
	}
	if w.StakeholderID == "" {
		return fmt.Errorf("stakeholder_id is required")
	}
	if w.ShareClassID == "" {
		return fmt.Errorf("share_class_id is required")
	}
	if w.ExercisePrice <= 0 {
		return fmt.Errorf("exercise_price must be positive")
	}
	if w.Quantity <= 0 {
		return fmt.Errorf("quantity must be positive")
	}
	if w.ExpirationDate.Before(w.IssueDate) {
		return fmt.Errorf("expiration_date must be after issue_date")
	}
	now := time.Now().UTC()
	w.CreatedAt = now
	w.UpdatedAt = now
	if w.Status == "" {
		w.Status = StatusDraft
	}
	return s.repo.Create(ctx, w)
}

// Get retrieves a warrant by ID.
func (s *Service) Get(ctx context.Context, id string) (*Warrant, error) {
	if id == "" {
		return nil, fmt.Errorf("warrant id is required")
	}
	return s.repo.Get(ctx, id)
}

// List returns all warrants for a company.
func (s *Service) List(ctx context.Context, companyID string) ([]*Warrant, error) {
	if companyID == "" {
		return nil, fmt.Errorf("company_id is required")
	}
	return s.repo.List(ctx, companyID)
}

// Update modifies an existing warrant.
func (s *Service) Update(ctx context.Context, w *Warrant) error {
	if w.ID == "" {
		return fmt.Errorf("warrant id is required")
	}
	w.UpdatedAt = time.Now().UTC()
	return s.repo.Update(ctx, w)
}

// Exercise processes a warrant exercise request.
func (s *Service) Exercise(ctx context.Context, req *ExerciseRequest) (*ExerciseResult, error) {
	if req.WarrantID == "" {
		return nil, fmt.Errorf("warrant_id is required")
	}
	if req.Quantity <= 0 {
		return nil, fmt.Errorf("quantity must be positive")
	}

	w, err := s.repo.Get(ctx, req.WarrantID)
	if err != nil {
		return nil, fmt.Errorf("get warrant: %w", err)
	}
	if w.Status != StatusActive {
		return nil, fmt.Errorf("warrant %s is not active (status=%s)", w.ID, w.Status)
	}
	if time.Now().After(w.ExpirationDate) {
		return nil, fmt.Errorf("warrant %s has expired", w.ID)
	}

	remaining := w.Quantity - w.QuantityExercised
	if req.Quantity > remaining {
		return nil, fmt.Errorf("requested %d shares but only %d remaining", req.Quantity, remaining)
	}

	exerciseType := req.ExerciseType
	if exerciseType == "" {
		exerciseType = w.ExerciseType
	}

	result := &ExerciseResult{
		WarrantID: w.ID,
	}

	switch exerciseType {
	case ExerciseCash:
		result.SharesIssued = req.Quantity
		result.CashPaid = float64(req.Quantity) * w.ExercisePrice
	case ExerciseCashless, ExerciseNet:
		if req.FMVAtExercise <= w.ExercisePrice {
			return nil, fmt.Errorf("fmv_at_exercise must exceed exercise_price for cashless exercise")
		}
		// Net shares = quantity * (FMV - exercise price) / FMV
		netShares := int64(float64(req.Quantity) * (req.FMVAtExercise - w.ExercisePrice) / req.FMVAtExercise)
		result.SharesIssued = netShares
	default:
		return nil, fmt.Errorf("unsupported exercise type: %s", exerciseType)
	}

	w.QuantityExercised += req.Quantity
	result.Remaining = w.Quantity - w.QuantityExercised
	if w.QuantityExercised >= w.Quantity {
		w.Status = StatusExercised
	}
	w.UpdatedAt = time.Now().UTC()

	if err := s.repo.Update(ctx, w); err != nil {
		return nil, fmt.Errorf("update warrant: %w", err)
	}

	return result, nil
}

// Cancel marks a warrant as cancelled.
func (s *Service) Cancel(ctx context.Context, id string) error {
	w, err := s.repo.Get(ctx, id)
	if err != nil {
		return fmt.Errorf("get warrant: %w", err)
	}
	if w.Status == StatusExercised {
		return fmt.Errorf("cannot cancel an exercised warrant")
	}
	w.Status = StatusCancelled
	w.UpdatedAt = time.Now().UTC()
	return s.repo.Update(ctx, w)
}
