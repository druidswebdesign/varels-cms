package tests

import (
	"net/http"
	"net/url"
	"strconv"
	"testing"

	"github.com/druidswebdesign/varels-cms/internal/db/types"
)

// TestLedger_StockLevelsEqualMovements is the central invariant of ADR-0017:
// current stock is derived only from the append-only movement ledger. After a
// mixed workflow (restock → sale → return → void) every stock_levels row must
// equal the sum of its movements, and no level may be negative.
func TestLedger_StockLevelsEqualMovements(t *testing.T) {
	e := newTestEnv(t)

	// Restock through the real handler: +4 at 5.000,00 (500.000 minor).
	rec := e.post("/variants/"+strconv.FormatInt(e.variantM, 10)+"/restock", url.Values{
		"quantity":    {"4"},
		"cost":        {"5000"},
		"location_id": {strconv.FormatInt(e.locationID, 10)},
		"note":        {"test restock"},
	})
	if rec.Code != http.StatusSeeOther {
		e.t.Fatalf("restock status = %d, want 303; body = %s", rec.Code, rec.Body.String())
	}

	// A sale, then a partial damaged return.
	orderID, itemID := e.sell(e.variantM, 2, types.PaymentMethodCash, 0)
	rec = e.post("/sales/"+strconv.FormatInt(orderID, 10)+"/return", url.Values{
		"resolution": {"refund"},
		"item_id":    {strconv.FormatInt(itemID, 10)},
		"qty":        {"1"},
		"condition":  {"damaged"},
		"reason":     {"defective"},
	})
	if rec.Code != http.StatusSeeOther {
		e.t.Fatalf("return status = %d, want 303", rec.Code)
	}

	// Void appends compensating rows; it must not touch existing movement rows.
	existing := e.movementIDs()
	rec = e.post("/sales/"+strconv.FormatInt(orderID, 10)+"/void", nil)
	if rec.Code != http.StatusSeeOther {
		e.t.Fatalf("void status = %d, want 303", rec.Code)
	}
	after := e.movementIDs()
	if len(after) <= len(existing) {
		e.t.Errorf("void did not append movements: %d -> %d", len(existing), len(after))
	}
	for id := range existing {
		if _, ok := after[id]; !ok {
			e.t.Errorf("void deleted existing movement row id=%d (ledger must be append-only)", id)
		}
	}

	e.assertStockLevelsMatchLedger()
}

// TestLedger_AdjustmentRequiresReason proves the schema-level rule that an
// adjustment movement must carry a reason code.
func TestLedger_AdjustmentRequiresReason(t *testing.T) {
	e := newTestEnv(t)
	before := e.movementCount()

	// Missing reason_code: handler rejects before touching the DB.
	rec := e.post("/variants/"+strconv.FormatInt(e.variantM, 10)+"/adjust", url.Values{
		"quantity":    {"-1"},
		"location_id": {strconv.FormatInt(e.locationID, 10)},
	})
	if rec.Code != http.StatusSeeOther {
		e.t.Fatalf("adjust status = %d, want 303", rec.Code)
	}
	if loc := rec.Header().Get("Location"); loc == "" || loc[0:1] != "/" {
		e.t.Errorf("adjust redirect = %q", loc)
	}
	if got := e.movementCount(); got != before {
		e.t.Errorf("movements = %d, want unchanged %d", got, before)
	}

	// With a reason code the same adjustment succeeds and decrements stock.
	rec = e.post("/variants/"+strconv.FormatInt(e.variantM, 10)+"/adjust", url.Values{
		"quantity":    {"-1"},
		"location_id": {strconv.FormatInt(e.locationID, 10)},
		"reason_code": {"recount"},
		"state":       {"sellable"},
	})
	if rec.Code != http.StatusSeeOther {
		e.t.Fatalf("adjust status = %d, want 303", rec.Code)
	}
	if got, want := e.sellableStock(e.variantM), int64(7); got != want {
		e.t.Errorf("stock after adjustment = %d, want %d", got, want)
	}
	e.assertStockLevelsMatchLedger()
}

// movementIDs returns the set of stock_movements ids.
func (e *testEnv) movementIDs() map[int64]struct{} {
	e.t.Helper()
	rows, err := e.db.Query("SELECT id FROM stock_movements")
	if err != nil {
		e.t.Fatalf("movement ids: %v", err)
	}
	defer rows.Close()
	out := make(map[int64]struct{})
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			e.t.Fatalf("scan movement id: %v", err)
		}
		out[id] = struct{}{}
	}
	if err := rows.Err(); err != nil {
		e.t.Fatalf("movement ids rows: %v", err)
	}
	return out
}

// assertStockLevelsMatchLedger checks the ADR-0017 invariant on every row.
func (e *testEnv) assertStockLevelsMatchLedger() {
	e.t.Helper()
	rows, err := e.db.Query(`
		SELECT sl.variant_id, sl.location_id, sl.state, sl.quantity,
		       COALESCE((
		           SELECT SUM(m.quantity_delta) FROM stock_movements m
		           WHERE m.variant_id = sl.variant_id
		             AND m.location_id = sl.location_id
		             AND m.state = sl.state
		       ), 0) AS ledger
		FROM stock_levels sl`)
	if err != nil {
		e.t.Fatalf("invariant query: %v", err)
	}
	defer rows.Close()

	checked := 0
	for rows.Next() {
		var variantID, locationID, quantity, ledger int64
		var state string
		if err := rows.Scan(&variantID, &locationID, &state, &quantity, &ledger); err != nil {
			e.t.Fatalf("scan invariant row: %v", err)
		}
		checked++
		if quantity != ledger {
			e.t.Errorf("stock_levels mismatch for variant=%d location=%d state=%s: level=%d ledger=%d",
				variantID, locationID, state, quantity, ledger)
		}
		if ledger < 0 {
			e.t.Errorf("negative ledger for variant=%d location=%d state=%s: %d",
				variantID, locationID, state, ledger)
		}
		if quantity < 0 {
			e.t.Errorf("negative stock_levels for variant=%d location=%d state=%s: %d",
				variantID, locationID, state, quantity)
		}
	}
	if err := rows.Err(); err != nil {
		e.t.Fatalf("invariant rows: %v", err)
	}
	if checked == 0 {
		e.t.Fatal("invariant query returned no stock_levels rows; seed is broken")
	}
}
