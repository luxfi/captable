package vesting

import "context"

// Repository defines storage for vesting schedule operations.
type Repository interface {
	CreateSchedule(ctx context.Context, s *Schedule) error
	GetSchedule(ctx context.Context, id string) (*Schedule, error)
	UpdateSchedule(ctx context.Context, s *Schedule) error
	ListSchedules(ctx context.Context, companyID string) ([]*Schedule, error)
	ListSchedulesByPlan(ctx context.Context, equityPlanID string) ([]*Schedule, error)

	CreateTranche(ctx context.Context, t *Tranche) error
	ListTranches(ctx context.Context, scheduleID string) ([]*Tranche, error)

	CreateCondition(ctx context.Context, c *PerformanceCondition) error
	GetCondition(ctx context.Context, id string) (*PerformanceCondition, error)

	AppendEvent(ctx context.Context, e *Event) error
	ListEvents(ctx context.Context, trancheID string) ([]*Event, error)
	ListEventsBySchedule(ctx context.Context, scheduleID string) ([]*Event, error)
}
