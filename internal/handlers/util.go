package handlers

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/base32"
	"strconv"
	"strings"
	"time"

	"github.com/yourname/varels_cms/internal/db/sqlc"
)

// withTx runs fn inside a SQLite transaction, rolling back on error. Every
// stock/financial write goes through here (docs/routes.md §1 "Transactions").
func (s *Server) withTx(ctx context.Context, fn func(q *sqlc.Queries) error) error {
	tx, err := s.DB.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	qtx := s.Q.WithTx(tx)
	if err := fn(qtx); err != nil {
		_ = tx.Rollback()
		return err
	}
	return tx.Commit()
}

// consumeFIFO consumes qty from the oldest open cost layers (ADR-0012) and
// returns the rounded unit cost. Any quantity not covered by a layer falls back
// to fallbackCost (the variant's current cost). Rounding is half-up in integer
// arithmetic (ADR-0021).
func consumeFIFO(ctx context.Context, q *sqlc.Queries, variantID, qty, fallbackCost int64) (int64, error) {
	layers, err := q.ListOpenCostLayers(ctx, variantID)
	if err != nil {
		return 0, err
	}

	remaining := qty
	var total int64
	for _, l := range layers {
		if remaining == 0 {
			break
		}
		take := remaining
		if l.QtyRemaining < take {
			take = l.QtyRemaining
		}
		total += take * l.UnitCostMinor
		if err := q.DecrementCostLayer(ctx, sqlc.DecrementCostLayerParams{Qty: take, ID: l.ID}); err != nil {
			return 0, err
		}
		remaining -= take
	}
	if remaining > 0 {
		total += remaining * fallbackCost
	}
	if qty <= 0 {
		return 0, nil
	}
	return (total + qty/2) / qty, nil
}

// addCostLayer records a new FIFO layer at unitCost (restock, void or return).
func addCostLayer(ctx context.Context, q *sqlc.Queries, variantID, qty, unitCost int64, sourceRef string) error {
	if qty <= 0 {
		return nil
	}
	_, err := q.CreateCostLayer(ctx, sqlc.CreateCostLayerParams{
		VariantID:     variantID,
		ReceivedAt:    nowUTC(),
		QtyReceived:   qty,
		QtyRemaining:  qty,
		UnitCostMinor: unitCost,
		SourceRef:     sql.NullString{String: sourceRef, Valid: sourceRef != ""},
	})
	return err
}

// newOrderNumber returns a short random human-facing receipt id. It is not a
// sequence, so receipt ids do not leak order volume (docs/schema.md §8).
func newOrderNumber() string {
	b := make([]byte, 6)
	if _, err := rand.Read(b); err != nil {
		return "ORD-000000"
	}
	return "ORD-" + strings.ToUpper(base32.StdEncoding.WithPadding(base32.NoPadding).EncodeToString(b))
}

// art is the Argentina reporting zone (UTC-3, no DST) per ADR-0014.
var art = time.FixedZone("ART", -3*60*60)

// periodRange returns the inclusive UTC ISO bounds for an Argentina-local
// year+month. month == 0 means the whole year.
func periodRange(year, month int) (string, string) {
	if month == 0 {
		start := time.Date(year, 1, 1, 0, 0, 0, 0, art)
		end := time.Date(year+1, 1, 1, 0, 0, 0, 0, art)
		return isoUTC(start), isoUTC(end.Add(-time.Second))
	}
	start := time.Date(year, time.Month(month), 1, 0, 0, 0, 0, art)
	end := start.AddDate(0, 1, 0)
	return isoUTC(start), isoUTC(end.Add(-time.Second))
}

// allTimeRange covers everything up to now.
func allTimeRange() (string, string) {
	return "1970-01-01T00:00:00Z", isoUTC(time.Now().UTC())
}

// staleSince returns the UTC timestamp N days before now, for deadstock.
func staleSince(days int) string {
	return isoUTC(time.Now().UTC().AddDate(0, 0, -days))
}

func isoUTC(t time.Time) string {
	return t.UTC().Format("2006-01-02T15:04:05Z")
}

// parseInt64 parses a form value, returning ok=false when empty/invalid.
func parseInt64(s string) (int64, bool) {
	s = strings.TrimSpace(s)
	if s == "" {
		return 0, false
	}
	v, err := strconv.ParseInt(s, 10, 64)
	if err != nil {
		return 0, false
	}
	return v, true
}

// delta returns b-a, guarding against negatives.
func delta(b, a int64) int64 {
	d := b - a
	if d < 0 {
		return 0
	}
	return d
}

// asInt64 normalises the interface{} sqlc returns for aggregate projections.
func asInt64(v interface{}) int64 {
	switch n := v.(type) {
	case int64:
		return n
	case int:
		return int64(n)
	case int32:
		return int64(n)
	case float64:
		return int64(n)
	case []byte:
		return parseIntBytes(n)
	case string:
		if parsed, ok := parseInt64(n); ok {
			return parsed
		}
	}
	return 0
}

func parseIntBytes(b []byte) int64 {
	v, ok := parseInt64(string(b))
	if !ok {
		return 0
	}
	return v
}

// ivaTaxMinor derives the display-only IVA portion of an IVA-inclusive total
// (ADR-0013): total − round(total / 1.21), rounded half-up in integers.
func ivaTaxMinor(total int64) int64 {
	if total <= 0 {
		return 0
	}
	net := (total*100 + 60) / 121
	return total - net
}
