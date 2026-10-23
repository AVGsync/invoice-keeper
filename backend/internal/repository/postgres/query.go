package postgres

import (
	"context"
	"database/sql"
)

// queryer is implemented by both *sql.DB and *sql.Tx. Shared aggregate loaders
// use it so a mutation can return the exact state visible before its commit.
type queryer interface {
	ExecContext(context.Context, string, ...any) (sql.Result, error)
	QueryContext(context.Context, string, ...any) (*sql.Rows, error)
	QueryRowContext(context.Context, string, ...any) *sql.Row
}
