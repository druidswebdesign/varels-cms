package tests

import (
	"net/http"
	"net/url"
	"strconv"
	"testing"

	"github.com/yourname/varels_cms/internal/db/types"
)

// TestStoreCredit_IssueRedeemOverspend walks the full store-credit lifecycle
// (ADR-0020): a store_credit return issues a signed ledger entry, a later sale
// redeems it, and an over-spend is rejected with no partial writes.
func TestStoreCredit_IssueRedeemOverspend(t *testing.T) {
	e := newTestEnv(t)
	customerID := e.createCustomer("Ana")

	// 1. Sell 2 units paid in cash, tied to the customer.
	saleID, itemID := e.sell(e.variantM, 2, types.PaymentMethodCash, customerID)

	// 2. Return both as store_credit: 2 × 1.200.000 = 2.400.000 issued.
	rec := e.post("/sales/"+strconv.FormatInt(saleID, 10)+"/return", url.Values{
		"resolution": {"store_credit"},
		"item_id":    {strconv.FormatInt(itemID, 10)},
		"qty":        {"2"},
		"condition":  {"sellable"},
		"reason":     {"changed_mind"},
	})
	if rec.Code != http.StatusSeeOther {
		t.Fatalf("return status = %d, want 303; body = %s", rec.Code, rec.Body.String())
	}
	if got := e.storeCreditBalance(customerID); got != 2_400_000 {
		t.Fatalf("balance after return = %d, want 2.400.000", got)
	}

	// 3. Redeem 1.200.000 on a new sale paid entirely in store credit.
	redeemID, _ := e.sell(e.variantM, 1, types.PaymentMethodStoreCredit, customerID)
	if got := e.storeCreditBalance(customerID); got != 1_200_000 {
		t.Errorf("balance after redemption = %d, want 1.200.000", got)
	}
	var redeemed int64
	if err := e.db.QueryRow(
		"SELECT amount_minor FROM payments WHERE order_id = ? AND method = 'store_credit'",
		redeemID).Scan(&redeemed); err != nil {
		t.Fatalf("read store-credit payment: %v", err)
	}
	if redeemed != 1_200_000 {
		t.Errorf("store-credit payment = %d, want 1.200.000", redeemed)
	}

	// 4. Over-spend: 5 units = 6.000.000 > 1.200.000 balance. Must roll back.
	ordersBefore := e.orderCount()
	balanceBefore := e.storeCreditBalance(customerID)
	stockNow := e.sellableStock(e.variantM)
	movementsBefore := e.movementCount()

	form := e.baseSaleForm()
	form.Set("payment_method", string(types.PaymentMethodStoreCredit))
	form.Set("customer_id", strconv.FormatInt(customerID, 10))
	form.Add("variant_id", strconv.FormatInt(e.variantM, 10))
	form.Add("quantity", "5")
	form.Add("unit_price", "12000")
	form.Add("line_discount", "0")

	rec = e.post("/sales", form)
	if rec.Code != http.StatusSeeOther {
		t.Fatalf("overspend status = %d, want 303", rec.Code)
	}
	if loc := rec.Header().Get("Location"); loc != "/sales/new?err=credit_balance" {
		t.Errorf("redirect = %q, want credit_balance error", loc)
	}
	if got := e.orderCount(); got != ordersBefore {
		t.Errorf("order count changed on overspend: %d -> %d", ordersBefore, got)
	}
	if got := e.storeCreditBalance(customerID); got != balanceBefore {
		t.Errorf("balance changed on overspend: %d -> %d", balanceBefore, got)
	}
	if got := e.sellableStock(e.variantM); got != stockNow {
		t.Errorf("stock changed on overspend: %d -> %d", stockNow, got)
	}
	if got := e.movementCount(); got != movementsBefore {
		t.Errorf("movements changed on overspend: %d -> %d", movementsBefore, got)
	}
}

// TestStoreCredit_RequiresCustomer rejects store-credit payment without a
// customer, since the ledger is keyed by customer.
func TestStoreCredit_RequiresCustomer(t *testing.T) {
	e := newTestEnv(t)
	ordersBefore := e.orderCount()

	form := e.baseSaleForm()
	form.Set("payment_method", string(types.PaymentMethodStoreCredit))
	form.Add("variant_id", strconv.FormatInt(e.variantM, 10))
	form.Add("quantity", "1")
	form.Add("unit_price", "12000")
	form.Add("line_discount", "0")

	rec := e.post("/sales", form)
	if rec.Code != http.StatusSeeOther {
		t.Fatalf("status = %d, want 303", rec.Code)
	}
	if loc := rec.Header().Get("Location"); loc != "/sales/new?err=credit_customer" {
		t.Errorf("redirect = %q, want credit_customer error", loc)
	}
	if got := e.orderCount(); got != ordersBefore {
		t.Errorf("order count changed: %d -> %d", ordersBefore, got)
	}
}
