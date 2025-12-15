package exercise

import "context"

// Repository defines storage for option exercise operations.
type Repository interface {
	CreateResult(ctx context.Context, r *ExerciseResult) error
	GetResult(ctx context.Context, id string) (*ExerciseResult, error)
	ListResults(ctx context.Context, grantID string) ([]*ExerciseResult, error)
}
