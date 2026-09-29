package service

import (
	"context"
	"database/sql"

	"github.com/yourname/varels_cms/internal/db/sqlc"
)

// ConsumeFIFO consumes qty from the oldest open cost layers (ADR-0012) and
// returns the rounded unit cost. Any quantity not covered by a layer falls back
// to fallbackCost (the variant's current cost). Rounding is half-up in integer
// arithmetic (ADR-0021).
func ConsumeFIFO(ctx context.Context, q *sqlc.Queries, variantID, qty, fallbackCost int64) (int64, error) {
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

// AddCostLayer records a new FIFO layer at unitCost (restock, void or return).
func AddCostLayer(ctx context.Context, q *sqlc.Queries, variantID, qty, unitCost int64, sourceRef string) error {
	if qty <= 0 {
		return nil
	}
	_, err := q.CreateCostLayer(ctx, sqlc.CreateCostLayerParams{
		VariantID:     variantID,
		ReceivedAt:    NowUTC(),
		QtyReceived:   qty,
		QtyRemaining:  qty,
		UnitCostMinor: unitCost,
		SourceRef:     sql.NullString{String: sourceRef, Valid: sourceRef != ""},
	})
	return err
}
