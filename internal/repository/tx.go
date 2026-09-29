package repository

import (
	"context"
	"database/sql"

	"github.com/yourname/varels_cms/internal/db/sqlc"
)

// WithTx runs fn inside a SQLite transaction, rolling back on error. Every
// stock/financial write goes through here (docs/routes.md §1 "Transactions").
func WithTx(ctx context.Context, db *sql.DB, q *sqlc.Queries, fn func(q *sqlc.Queries) error) error {
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	qtx := q.WithTx(tx)
	if err := fn(qtx); err != nil {
		_ = tx.Rollback()
		return err
	}
	return tx.Commit()
}
