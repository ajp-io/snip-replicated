package db

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/ajp-io/snips-replicated/internal/model"
)

// ErrNotFound is returned when a record is not found.
var ErrNotFound = errors.New("not found")

func (d *DB) CreateLink(ctx context.Context, slug, destination, label string, expiresAt *time.Time) (*model.Link, error) {
	link := &model.Link{}
	err := d.pool.QueryRow(ctx,
		`INSERT INTO links (slug, destination, label, expires_at)
		 VALUES ($1, $2, NULLIF($3, ''), $4)
		 RETURNING id, slug, destination, label, expires_at, created_at`,
		slug, destination, label, expiresAt,
	).Scan(&link.ID, &link.Slug, &link.Destination, &link.Label, &link.ExpiresAt, &link.CreatedAt)
	if err != nil {
		return nil, err
	}
	return link, nil
}

func (d *DB) GetLinkBySlug(ctx context.Context, slug string) (*model.Link, error) {
	link := &model.Link{}
	err := d.pool.QueryRow(ctx,
		`SELECT id, slug, destination, label, expires_at, created_at FROM links WHERE slug = $1`,
		slug,
	).Scan(&link.ID, &link.Slug, &link.Destination, &link.Label, &link.ExpiresAt, &link.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	return link, err
}

func (d *DB) GetLinkByID(ctx context.Context, id int64) (*model.Link, error) {
	link := &model.Link{}
	err := d.pool.QueryRow(ctx,
		`SELECT id, slug, destination, label, expires_at, created_at FROM links WHERE id = $1`,
		id,
	).Scan(&link.ID, &link.Slug, &link.Destination, &link.Label, &link.ExpiresAt, &link.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	return link, err
}

func (d *DB) ListLinksWithClickCounts(ctx context.Context) ([]model.LinkWithCount, error) {
	rows, err := d.pool.Query(ctx,
		`SELECT l.id, l.slug, l.destination, l.label, l.expires_at, l.created_at,
		        COUNT(c.id) AS total_clicks
		 FROM links l
		 LEFT JOIN click_events c ON c.link_id = l.id
		 GROUP BY l.id
		 ORDER BY total_clicks DESC, l.created_at DESC`,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var results []model.LinkWithCount
	for rows.Next() {
		var lwc model.LinkWithCount
		if err := rows.Scan(
			&lwc.ID, &lwc.Slug, &lwc.Destination, &lwc.Label,
			&lwc.ExpiresAt, &lwc.CreatedAt, &lwc.TotalClicks,
		); err != nil {
			return nil, err
		}
		results = append(results, lwc)
	}
	return results, rows.Err()
}

func (d *DB) DeleteLink(ctx context.Context, id int64) error {
	tag, err := d.pool.Exec(ctx, `DELETE FROM links WHERE id = $1`, id)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

func (d *DB) RecordClick(ctx context.Context, linkID int64, referrer string) error {
	_, err := d.pool.Exec(ctx,
		`INSERT INTO click_events (link_id, referrer) VALUES ($1, NULLIF($2, ''))`,
		linkID, referrer,
	)
	return err
}

func (d *DB) GetDailyClicks(ctx context.Context, linkID int64) ([]model.DailyClicks, error) {
	rows, err := d.pool.Query(ctx,
		`SELECT DATE(clicked_at) AS day, COUNT(*) AS clicks
		 FROM click_events
		 WHERE link_id = $1 AND clicked_at > NOW() - INTERVAL '30 days'
		 GROUP BY day ORDER BY day`,
		linkID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var results []model.DailyClicks
	for rows.Next() {
		var dc model.DailyClicks
		if err := rows.Scan(&dc.Day, &dc.Clicks); err != nil {
			return nil, err
		}
		results = append(results, dc)
	}
	return results, rows.Err()
}

func (d *DB) GetTopReferrers(ctx context.Context, linkID int64) ([]model.ReferrerCount, error) {
	rows, err := d.pool.Query(ctx,
		`SELECT COALESCE(NULLIF(referrer, ''), '(direct)') AS referrer, COUNT(*) AS clicks
		 FROM click_events
		 WHERE link_id = $1
		 GROUP BY referrer ORDER BY clicks DESC LIMIT 10`,
		linkID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var results []model.ReferrerCount
	for rows.Next() {
		var rc model.ReferrerCount
		if err := rows.Scan(&rc.Referrer, &rc.Clicks); err != nil {
			return nil, err
		}
		results = append(results, rc)
	}
	return results, rows.Err()
}
