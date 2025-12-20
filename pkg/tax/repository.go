package tax

import "context"

// Repository defines storage for tax form operations.
type Repository interface {
	CreateForm1099DIV(ctx context.Context, f *Form1099DIV) error
	GetForm1099DIV(ctx context.Context, id string) (*Form1099DIV, error)
	ListForms1099DIV(ctx context.Context, taxYear int, companyID string) ([]*Form1099DIV, error)

	CreateForm1099B(ctx context.Context, f *Form1099B) error
	GetForm1099B(ctx context.Context, id string) (*Form1099B, error)
	ListForms1099B(ctx context.Context, taxYear int, companyID string) ([]*Form1099B, error)

	CreateScheduleK1(ctx context.Context, k *ScheduleK1) error
	GetScheduleK1(ctx context.Context, id string) (*ScheduleK1, error)
	ListScheduleK1s(ctx context.Context, taxYear int, companyID string) ([]*ScheduleK1, error)
}
