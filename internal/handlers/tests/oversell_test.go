package tests

import (
	"net/http"
	"strconv"
	"testing"
)

// TestOversell_RollsBackEverything proves the stock trigger's CHECK aborts the
// whole sale transaction: no order, no movement, unchanged stock (ADR-0019).
func TestOversell_RollsBackEverything(t *testing.T) {
	e := newTestEnv(t)
	stock := e.sellableStock(e.variantL) // 5 in the dev seed
	orders := e.orderCount()
	movements := e.movementCount()

	form := e.baseSaleForm()
	form.Add("variant_id", strconv.FormatInt(e.variantL, 10))
	form.Add("quantity", "9999")
	form.Add("unit_price", "12000")
	form.Add("line_discount", "0")

	rec := e.post("/sales", form)
	if rec.Code != http.StatusSeeOther {
		t.Fatalf("status = %d, want 303", rec.Code)
	}
	if loc := rec.Header().Get("Location"); loc != "/sales/new?err=stock" {
		t.Errorf("redirect = %q, want stock error", loc)
	}
	if got := e.orderCount(); got != orders {
		t.Errorf("order count changed: %d -> %d", orders, got)
	}
	if got := e.sellableStock(e.variantL); got != stock {
		t.Errorf("stock changed: %d -> %d", stock, got)
	}
	if got := e.movementCount(); got != movements {
		t.Errorf("movements changed: %d -> %d", movements, got)
	}
	if got := e.scalarInt("SELECT COUNT(*) FROM payments"); got != 0 {
		t.Errorf("payments written = %d, want 0", got)
	}
}

// TestOversell_MultiLineIsAtomic proves a valid first line is not committed when
// a later line oversells: the transaction rollback covers all lines.
func TestOversell_MultiLineIsAtomic(t *testing.T) {
	e := newTestEnv(t)
	stockM := e.sellableStock(e.variantM)
	stockL := e.sellableStock(e.variantL)
	orders := e.orderCount()
	movements := e.movementCount()

	form := e.baseSaleForm()
	form.Add("variant_id", strconv.FormatInt(e.variantM, 10)) // valid
	form.Add("quantity", "1")
	form.Add("unit_price", "12000")
	form.Add("line_discount", "0")
	form.Add("variant_id", strconv.FormatInt(e.variantL, 10)) // oversells
	form.Add("quantity", "9999")
	form.Add("unit_price", "12000")
	form.Add("line_discount", "0")

	rec := e.post("/sales", form)
	if rec.Code != http.StatusSeeOther {
		t.Fatalf("status = %d, want 303", rec.Code)
	}
	if loc := rec.Header().Get("Location"); loc != "/sales/new?err=stock" {
		t.Errorf("redirect = %q, want stock error", loc)
	}
	if got := e.sellableStock(e.variantM); got != stockM {
		t.Errorf("variant M stock changed: %d -> %d", stockM, got)
	}
	if got := e.sellableStock(e.variantL); got != stockL {
		t.Errorf("variant L stock changed: %d -> %d", stockL, got)
	}
	if got := e.orderCount(); got != orders {
		t.Errorf("order count changed: %d -> %d", orders, got)
	}
	if got := e.movementCount(); got != movements {
		t.Errorf("movements changed: %d -> %d", movements, got)
	}
}
