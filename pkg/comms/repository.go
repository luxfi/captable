package comms

import "context"

// Repository is the storage interface for communications.
type Repository interface {
	CreateNotice(ctx context.Context, n *Notice) error
	GetNotice(ctx context.Context, id string) (*Notice, error)
	ListNotices(ctx context.Context, filter NoticeFilter) ([]*Notice, error)
	MarkSent(ctx context.Context, id string) error
}
