package db

import (
	"context"
	"time"

	"github.com/ajp-io/snips-replicated/internal/model"
)

// Store is the interface handlers use for all database operations.
type Store interface {
	CreateLink(ctx context.Context, slug, destination, label string, expiresAt *time.Time) (*model.Link, error)
	GetLinkBySlug(ctx context.Context, slug string) (*model.Link, error)
	GetLinkByID(ctx context.Context, id int64) (*model.Link, error)
	ListLinksWithClickCounts(ctx context.Context) ([]model.LinkWithCount, error)
	DeleteLink(ctx context.Context, id int64) error
	RecordClick(ctx context.Context, linkID int64, referrer string) error
	GetDailyClicks(ctx context.Context, linkID int64) ([]model.DailyClicks, error)
	GetTopReferrers(ctx context.Context, linkID int64) ([]model.ReferrerCount, error)
	Ping(ctx context.Context) error
}
