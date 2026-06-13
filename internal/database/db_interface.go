package database

import (
	"context"
	"database/sql"
)

// DBQuerier defines the subset of *sql.DB methods used by handlers.
// This allows mocking database calls in tests.
type DBQuerier interface {
	QueryContext(ctx context.Context, query string, args ...interface{}) (*sql.Rows, error)
	QueryRowContext(ctx context.Context, query string, args ...interface{}) *sql.Row
}
