package disclosure

import "context"

// Repository is the storage interface for disclosures.
type Repository interface {
	Create(ctx context.Context, d *Disclosure) error
	Get(ctx context.Context, id string) (*Disclosure, error)
	List(ctx context.Context, filter DisclosureFilter) ([]*Disclosure, error)
	MarkDelivered(ctx context.Context, id, stakeholderID string) error
	Acknowledge(ctx context.Context, id, stakeholderID string) error
}
