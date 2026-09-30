package tests

import (
	"net/http"
	"strconv"
	"testing"

	"github.com/druidswebdesign/varels-cms/internal/db/types"
)

// TestSale_FIFOStockAndPayment is the core money-path test: one sale must
// consume FIFO cost layers for unit_cost_minor, decrement sellable stock, and
// capture a payment, all atomically (ADR-0012, ADR-0019).
func TestSale_FIFOStockAndPayment(t *testing.T) {
	e := newTestEnv(t)
	e.addLayer(e.variantM, 3, 400000, "2026-01-01T00:00:00Z")
	e.addLayer(e.variantM, 2, 600000, "2026-02-01T00:00:00Z")

	before := e.sellableStock(e.variantM)

	form := e.baseSaleForm()
	form.Add("variant_id", strconv.FormatInt(e.variantM, 10))
	form.Add("quantity", "5")
	form.Add("unit_price", "12000") // 1.200.000 minor
	form.Add("line_discount", "0")

	rec := e.post("/sales", form)
	if rec.Code != http.StatusSeeOther {
		t.Fatalf("sale status = %d, want 303; body = %s", rec.Code, rec.Body.String())
	}
	orderID := e.redirectID(rec)

	var subtotal, total int64
	var status, paymentStatus string
	if err := e.db.QueryRow(
		"SELECT subtotal_minor, total_minor, status, payment_status FROM orders WHERE id = ?",
		orderID).Scan(&subtotal, &total, &status, &paymentStatus); err != nil {
		t.Fatalf("read order: %v", err)
	}
	if subtotal != 6_000_000 || total != 6_000_000 {
		t.Errorf("order totals = %d/%d, want 6.000.000", subtotal, total)
	}
	if status != string(types.OrderStatusCompleted) {
		t.Errorf("order status = %q, want completed", status)
	}
	if paymentStatus != string(types.PaymentStatusPaid) {
		t.Errorf("payment status = %q, want paid", paymentStatus)
	}

	// FIFO: 3@400000 + 2@600000 = 2.400.000 over 5 units = 480.000 unit cost.
	var cost int64
	if err := e.db.QueryRow(
		"SELECT unit_cost_minor FROM order_items WHERE order_id = ?", orderID).Scan(&cost); err != nil {
		t.Fatalf("read order item: %v", err)
	}
	if cost != 480000 {
		t.Errorf("unit_cost_minor = %d, want 480000 (FIFO average)", cost)
	}

	if got, want := e.sellableStock(e.variantM), before-5; got != want {
		t.Errorf("sellable stock = %d, want %d", got, want)
	}

	var paid int64
	if err := e.db.QueryRow(
		"SELECT amount_minor FROM payments WHERE order_id = ?", orderID).Scan(&paid); err != nil {
		t.Fatalf("read payment: %v", err)
	}
	if paid != 6_000_000 {
		t.Errorf("captured payment = %d, want 6.000.000", paid)
	}

	var remaining int64
	if err := e.db.QueryRow(
		"SELECT COALESCE(SUM(qty_remaining), 0) FROM cost_layers WHERE variant_id = ?",
		e.variantM).Scan(&remaining); err != nil {
		t.Fatalf("read layers: %v", err)
	}
	if remaining != 0 {
		t.Errorf("cost layers remaining = %d, want 0 (both consumed)", remaining)
	}
}

// TestSale_OrderDiscountAndShipping checks the total arithmetic an owner would
// notice on the receipt: subtotal − order discount + shipping.
func TestSale_OrderDiscountAndShipping(t *testing.T) {
	e := newTestEnv(t)

	form := e.baseSaleForm()
	form.Add("variant_id", strconv.FormatInt(e.variantM, 10))
	form.Add("quantity", "2")
	form.Add("unit_price", "12000") // 2.400.000 subtotal
	form.Add("line_discount", "0")
	form.Set("order_discount", "400")
	form.Set("shipping", "1500")

	rec := e.post("/sales", form)
	if rec.Code != http.StatusSeeOther {
		t.Fatalf("sale status = %d, want 303; body = %s", rec.Code, rec.Body.String())
	}
	orderID := e.redirectID(rec)

	var subtotal, discount, shipping, total int64
	if err := e.db.QueryRow(
		"SELECT subtotal_minor, discount_minor, shipping_minor, total_minor FROM orders WHERE id = ?",
		orderID).Scan(&subtotal, &discount, &shipping, &total); err != nil {
		t.Fatalf("read order: %v", err)
	}
	if subtotal != 2_400_000 {
		t.Errorf("subtotal = %d, want 2.400.000", subtotal)
	}
	if discount != 40_000 {
		t.Errorf("discount = %d, want 40.000", discount)
	}
	if shipping != 150_000 {
		t.Errorf("shipping = %d, want 150.000", shipping)
	}
	if want := subtotal - discount + shipping; total != want {
		t.Errorf("total = %d, want %d", total, want)
	}
}

// TestSale_MultiLineSinglePayment checks that multiple variants land on one
// order with one captured payment for the summed total.
func TestSale_MultiLineSinglePayment(t *testing.T) {
	e := newTestEnv(t)

	form := e.baseSaleForm()
	form.Add("variant_id", strconv.FormatInt(e.variantM, 10))
	form.Add("quantity", "2")
	form.Add("unit_price", "12000")
	form.Add("line_discount", "0")
	form.Add("variant_id", strconv.FormatInt(e.variantL, 10))
	form.Add("quantity", "1")
	form.Add("unit_price", "12000")
	form.Add("line_discount", "0")

	rec := e.post("/sales", form)
	if rec.Code != http.StatusSeeOther {
		t.Fatalf("sale status = %d, want 303; body = %s", rec.Code, rec.Body.String())
	}
	orderID := e.redirectID(rec)

	if got := e.scalarInt("SELECT COUNT(*) FROM order_items WHERE order_id = ?", orderID); got != 2 {
		t.Errorf("order items = %d, want 2", got)
	}
	if got := e.scalarInt("SELECT amount_minor FROM payments WHERE order_id = ?", orderID); got != 3_600_000 {
		t.Errorf("payment = %d, want 3.600.000", got)
	}
	if got, want := e.sellableStock(e.variantM), int64(6); got != want {
		t.Errorf("variant M stock = %d, want %d", got, want)
	}
	if got, want := e.sellableStock(e.variantL), int64(4); got != want {
		t.Errorf("variant L stock = %d, want %d", got, want)
	}
}

// TestVoid_RestocksAndCompensates proves a void never deletes the original
// sale: it appends a compensating movement, restocks at the original cost and
// marks the order cancelled / payment refunded.
func TestVoid_RestocksAndCompensates(t *testing.T) {
	e := newTestEnv(t)
	e.addLayer(e.variantM, 5, 400000, "2026-01-01T00:00:00Z")

	orderID, _ := e.sell(e.variantM, 3, types.PaymentMethodCash, 0)
	stockAfterSale := e.sellableStock(e.variantM)
	movementsBefore := e.movementCount()

	rec := e.post("/sales/"+strconv.FormatInt(orderID, 10)+"/void", nil)
	if rec.Code != http.StatusSeeOther {
		t.Fatalf("void status = %d, want 303; body = %s", rec.Code, rec.Body.String())
	}

	if got, want := e.sellableStock(e.variantM), stockAfterSale+3; got != want {
		t.Errorf("stock after void = %d, want %d", got, want)
	}
	if got := e.movementCount(); got != movementsBefore+1 {
		t.Errorf("movements = %d, want %d (compensating row appended, none deleted)", got, movementsBefore+1)
	}
	var status, ps string
	if err := e.db.QueryRow("SELECT status, payment_status FROM orders WHERE id = ?", orderID).
		Scan(&status, &ps); err != nil {
		t.Fatalf("read order: %v", err)
	}
	if status != string(types.OrderStatusCancelled) || ps != string(types.PaymentStatusRefunded) {
		t.Errorf("order = %s/%s, want cancelled/refunded", status, ps)
	}
}
