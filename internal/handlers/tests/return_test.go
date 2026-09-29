package tests

import (
	"net/http"
	"net/url"
	"strconv"
	"testing"

	"github.com/yourname/varels_cms/internal/db/types"
)

// TestReturn_PartialRestoresStockAndOriginalCost checks the ADR-0006/ADR-0012
// contract: a partial return restocks the returned units at the line's original
// cost, and the order/payment statuses become *partially* returned/refunded.
func TestReturn_PartialRestoresStockAndOriginalCost(t *testing.T) {
	e := newTestEnv(t)
	e.addLayer(e.variantM, 5, 400000, "2026-01-01T00:00:00Z")

	orderID, itemID := e.sell(e.variantM, 5, types.PaymentMethodCash, 0)
	if got := e.sellableStock(e.variantM); got != 3 {
		t.Fatalf("stock after sale = %d, want 3", got)
	}

	rec := e.post("/sales/"+strconv.FormatInt(orderID, 10)+"/return", url.Values{
		"resolution": {"refund"},
		"item_id":    {strconv.FormatInt(itemID, 10)},
		"qty":        {"2"},
		"condition":  {"sellable"},
		"reason":     {"wrong_size"},
	})
	if rec.Code != http.StatusSeeOther {
		t.Fatalf("return status = %d, want 303; body = %s", rec.Code, rec.Body.String())
	}

	if got, want := e.sellableStock(e.variantM), int64(5); got != want {
		t.Errorf("stock after return = %d, want %d", got, want)
	}

	// The sale consumed the 5@400000 layer; the return adds 2 units back at the
	// same original unit cost.
	var layers, layerCost int64
	if err := e.db.QueryRow(
		`SELECT COUNT(*), COALESCE(SUM(qty_remaining), 0) FROM cost_layers
		 WHERE variant_id = ? AND source_ref LIKE 'return:%'`, e.variantM).
		Scan(&layers, &layerCost); err != nil {
		t.Fatalf("read layers: %v", err)
	}
	if layers != 1 {
		t.Fatalf("return cost layers = %d, want 1", layers)
	}
	var unitCost int64
	if err := e.db.QueryRow(
		`SELECT unit_cost_minor FROM cost_layers
		 WHERE variant_id = ? AND source_ref LIKE 'return:%'`, e.variantM).Scan(&unitCost); err != nil {
		t.Fatalf("read return layer: %v", err)
	}
	if unitCost != 400000 {
		t.Errorf("returned layer unit cost = %d, want 400000 (original cost)", unitCost)
	}

	var status, ps string
	if err := e.db.QueryRow("SELECT status, payment_status FROM orders WHERE id = ?", orderID).
		Scan(&status, &ps); err != nil {
		t.Fatalf("read order: %v", err)
	}
	if status != string(types.OrderStatusPartiallyReturned) {
		t.Errorf("order status = %q, want partially_returned", status)
	}
	if ps != string(types.PaymentStatusPartiallyRefunded) {
		t.Errorf("payment status = %q, want partially_refunded", ps)
	}

	if got := e.scalarInt("SELECT COALESCE(SUM(quantity), 0) FROM return_items"); got != 2 {
		t.Errorf("returned quantity = %d, want 2", got)
	}
}

// TestReturn_DamagedGoesToDamagedState proves condition routing: a damaged
// return restocks into the `damaged` state, never back to sellable.
func TestReturn_DamagedGoesToDamagedState(t *testing.T) {
	e := newTestEnv(t)
	orderID, itemID := e.sell(e.variantM, 2, types.PaymentMethodCash, 0)
	sellableAfterSale := e.sellableStock(e.variantM)

	rec := e.post("/sales/"+strconv.FormatInt(orderID, 10)+"/return", url.Values{
		"resolution": {"refund"},
		"item_id":    {strconv.FormatInt(itemID, 10)},
		"qty":        {"1"},
		"condition":  {"damaged"},
		"reason":     {"defective"},
	})
	if rec.Code != http.StatusSeeOther {
		t.Fatalf("return status = %d, want 303; body = %s", rec.Code, rec.Body.String())
	}

	if got := e.sellableStock(e.variantM); got != sellableAfterSale {
		t.Errorf("sellable stock = %d, want unchanged %d", got, sellableAfterSale)
	}
	if got := e.stateStock(e.variantM, types.StockStateDamaged); got != 1 {
		t.Errorf("damaged stock = %d, want 1", got)
	}
}

// TestReturn_CannotOverReturn rejects a return larger than the remaining
// returnable quantity, leaving stock untouched.
func TestReturn_CannotOverReturn(t *testing.T) {
	e := newTestEnv(t)
	orderID, itemID := e.sell(e.variantM, 2, types.PaymentMethodCash, 0)
	stock := e.sellableStock(e.variantM)
	orders := e.orderCount()

	rec := e.post("/sales/"+strconv.FormatInt(orderID, 10)+"/return", url.Values{
		"resolution": {"refund"},
		"item_id":    {strconv.FormatInt(itemID, 10)},
		"qty":        {"99"},
		"condition":  {"sellable"},
		"reason":     {"wrong_size"},
	})
	if rec.Code != http.StatusSeeOther {
		t.Fatalf("return status = %d, want 303", rec.Code)
	}
	if loc := rec.Header().Get("Location"); loc != "/sales/"+strconv.FormatInt(orderID, 10)+"/return?err=quantity" {
		t.Errorf("redirect = %q, want quantity error", loc)
	}
	if got := e.sellableStock(e.variantM); got != stock {
		t.Errorf("stock = %d, want unchanged %d", got, stock)
	}
	if got := e.orderCount(); got != orders {
		t.Errorf("order count changed: %d -> %d", orders, got)
	}
	if got := e.scalarInt("SELECT COUNT(*) FROM return_items"); got != 0 {
		t.Errorf("return_items = %d, want 0", got)
	}
}
