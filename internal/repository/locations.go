// Package repository holds read helpers over sqlc queries that are shared by
// more than one handler package. sqlc remains the generated data-access layer;
// this package only adds thin, reusable projections.
package repository

import (
	"context"

	"github.com/yourname/varels_cms/internal/db/sqlc"
)

// LocationNames maps location id -> name.
func LocationNames(ctx context.Context, q *sqlc.Queries) (map[int64]string, error) {
	locations, err := q.ListAllLocations(ctx)
	if err != nil {
		return map[int64]string{}, err
	}
	out := make(map[int64]string, len(locations))
	for _, l := range locations {
		out[l.ID] = l.Name
	}
	return out, nil
}
