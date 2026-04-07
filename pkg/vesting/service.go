package vesting

import (
	"context"
	"fmt"
	"time"
)

// Service handles vesting schedule management.
type Service struct {
	repo Repository
}

// NewService creates a vesting service backed by the given repository.
func NewService(repo Repository) *Service {
	return &Service{repo: repo}
}

// CreateSchedule validates and persists a new vesting schedule.
func (s *Service) CreateSchedule(ctx context.Context, sched *Schedule) error {
	if sched.CompanyID == "" {
		return fmt.Errorf("company_id is required")
	}
	if sched.ScheduleName == "" {
		return fmt.Errorf("schedule_name is required")
	}
	if sched.ScheduleType == "" {
		return fmt.Errorf("schedule_type is required")
	}
	if sched.IncludeCliff && sched.CliffLength == nil {
		return fmt.Errorf("cliff_length is required when include_cliff is true")
	}
	now := time.Now().UTC()
	sched.CreatedAt = now
	sched.UpdatedAt = now
	return s.repo.CreateSchedule(ctx, sched)
}

// GetSchedule retrieves a vesting schedule by ID.
func (s *Service) GetSchedule(ctx context.Context, id string) (*Schedule, error) {
	if id == "" {
		return nil, fmt.Errorf("schedule id is required")
	}
	return s.repo.GetSchedule(ctx, id)
}

// ListSchedules returns all vesting schedules for a company.
func (s *Service) ListSchedules(ctx context.Context, companyID string) ([]*Schedule, error) {
	if companyID == "" {
		return nil, fmt.Errorf("company_id is required")
	}
	return s.repo.ListSchedules(ctx, companyID)
}

// UpdateSchedule modifies an existing vesting schedule.
func (s *Service) UpdateSchedule(ctx context.Context, sched *Schedule) error {
	if sched.ID == "" {
		return fmt.Errorf("schedule id is required")
	}
	sched.UpdatedAt = time.Now().UTC()
	return s.repo.UpdateSchedule(ctx, sched)
}

// AddTranche adds a vesting tranche to a schedule.
func (s *Service) AddTranche(ctx context.Context, t *Tranche) error {
	if t.ScheduleID == "" {
		return fmt.Errorf("schedule_id is required")
	}
	if t.Percentage <= 0 || t.Percentage > 100 {
		return fmt.Errorf("percentage must be between 0 and 100")
	}
	now := time.Now().UTC()
	t.CreatedAt = now
	t.UpdatedAt = now
	return s.repo.CreateTranche(ctx, t)
}

// CreateCondition creates a performance condition.
func (s *Service) CreateCondition(ctx context.Context, c *PerformanceCondition) error {
	if c.Name == "" {
		return fmt.Errorf("name is required")
	}
	if c.ConditionType == "" {
		return fmt.Errorf("condition_type is required")
	}
	now := time.Now().UTC()
	c.CreatedAt = now
	c.UpdatedAt = now
	return s.repo.CreateCondition(ctx, c)
}

// RecordEvent records a vesting event (shares actually vesting).
func (s *Service) RecordEvent(ctx context.Context, e *Event) error {
	if e.TrancheID == "" {
		return fmt.Errorf("tranche_id is required")
	}
	if e.Percentage <= 0 {
		return fmt.Errorf("percentage must be positive")
	}
	e.CreatedAt = time.Now().UTC()
	return s.repo.AppendEvent(ctx, e)
}

// GetEvents returns all vesting events for a schedule.
func (s *Service) GetEvents(ctx context.Context, scheduleID string) ([]*Event, error) {
	if scheduleID == "" {
		return nil, fmt.Errorf("schedule_id is required")
	}
	return s.repo.ListEventsBySchedule(ctx, scheduleID)
}
