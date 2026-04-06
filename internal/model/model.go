package model

import "time"

// Link represents a shortened URL.
type Link struct {
	ID          int64
	Slug        string
	Destination string
	Label       string
	ExpiresAt   *time.Time
	CreatedAt   time.Time
}

// LinkWithCount is a Link with its total click count, used on the dashboard.
type LinkWithCount struct {
	Link
	TotalClicks int64
}

// DailyClicks is one data point in the clicks-over-time chart.
type DailyClicks struct {
	Day    time.Time
	Clicks int64
}

// ReferrerCount is one row in the top-referrers table.
type ReferrerCount struct {
	Referrer string
	Clicks   int64
}
