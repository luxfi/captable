package settlement

import "context"

// Repository defines storage for secondary market trades.
type Repository interface {
	CreateTrade(ctx context.Context, trade *SecondaryTrade) error
	GetTrade(ctx context.Context, transactionID string) (*SecondaryTrade, error)
	UpdateTrade(ctx context.Context, trade *SecondaryTrade) error
	ListTrades(ctx context.Context, limit, offset int) ([]*SecondaryTrade, error)
}
