package db

import (
	"context"
	"fmt"
	"io/fs"
	"sort"
	"strings"

	"github.com/jackc/pgx/v5/pgxpool"
)

// DB wraps a pgxpool.Pool and implements Store.
type DB struct {
	pool *pgxpool.Pool
}

// New creates a pgx connection pool and runs migrations from the provided fs.FS.
func New(ctx context.Context, dsn string, migrationsFS fs.FS) (*DB, error) {
	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		return nil, fmt.Errorf("create pool: %w", err)
	}
	d := &DB{pool: pool}
	if err := d.runMigrations(ctx, migrationsFS); err != nil {
		pool.Close()
		return nil, fmt.Errorf("run migrations: %w", err)
	}
	return d, nil
}

// Close closes the connection pool.
func (d *DB) Close() {
	d.pool.Close()
}

func (d *DB) runMigrations(ctx context.Context, fsys fs.FS) error {
	entries, err := fs.ReadDir(fsys, ".")
	if err != nil {
		return fmt.Errorf("read migrations: %w", err)
	}
	sort.Slice(entries, func(i, j int) bool {
		return entries[i].Name() < entries[j].Name()
	})
	for _, e := range entries {
		if !strings.HasSuffix(e.Name(), ".sql") {
			continue
		}
		data, err := fs.ReadFile(fsys, e.Name())
		if err != nil {
			return fmt.Errorf("read %s: %w", e.Name(), err)
		}
		if _, err = d.pool.Exec(ctx, string(data)); err != nil {
			return fmt.Errorf("apply %s: %w", e.Name(), err)
		}
	}
	return nil
}

// Ping implements Store.
func (d *DB) Ping(ctx context.Context) error {
	_, err := d.pool.Exec(ctx, "SELECT 1")
	return err
}
