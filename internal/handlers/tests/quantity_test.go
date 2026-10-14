package tests

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"

	"github.com/druidswebdesign/varels-cms/internal/handlers/catalog"
)

// TestSetQuantityAbsolute proves the product page can set a variant's sellable
// quantity to an absolute value (e.g. 8 -> 0), not just apply a signed delta.
func TestSetQuantityAbsolute(t *testing.T) {
	e := newTestEnv(t)
	catalogh := catalog.New(e.srv)

	r := chi.NewRouter()
	r.Post("/variants/{id}/quantity", catalogh.HandleVariantSetQuantity)

	path := "/variants/" + strconv.FormatInt(e.variantM, 10) + "/quantity"
	post := func(qty int64) *httptest.ResponseRecorder {
		t.Helper()
		form := url.Values{"quantity": {strconv.FormatInt(qty, 10)}}
		req := httptest.NewRequest(http.MethodPost, path, strings.NewReader(form.Encode()))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		rec := httptest.NewRecorder()
		r.ServeHTTP(rec, req)
		return rec
	}

	if got := e.sellableStock(e.variantM); got != 8 {
		t.Fatalf("precondition: sellable = %d, want 8", got)
	}

	// 8 -> 0 by typing 0 (not a -8 delta).
	if rec := post(0); rec.Code != http.StatusSeeOther {
		t.Fatalf("set 0: status = %d, want 303; body = %s", rec.Code, rec.Body.String())
	}
	if got := e.sellableStock(e.variantM); got != 0 {
		t.Errorf("after set 0: sellable = %d, want 0", got)
	}

	// 0 -> 25.
	if rec := post(25); rec.Code != http.StatusSeeOther {
		t.Fatalf("set 25: status = %d, want 303; body = %s", rec.Code, rec.Body.String())
	}
	if got := e.sellableStock(e.variantM); got != 25 {
		t.Errorf("after set 25: sellable = %d, want 25", got)
	}

	// Every write is a "recount" adjustment movement (append-only, ADR-0017).
	// The two sets above produce exactly two recount movements.
	var count int64
	if err := e.db.QueryRow(
		"SELECT COUNT(*) FROM stock_movements WHERE variant_id = ? AND reason_code = 'recount' AND ref_type = 'adjustment'",
		e.variantM).Scan(&count); err != nil {
		t.Fatalf("movement check: %v", err)
	}
	if count != 2 {
		t.Errorf("recount movements = %d, want 2", count)
	}
	var deltas int64
	if err := e.db.QueryRow(
		"SELECT COALESCE(SUM(quantity_delta),0) FROM stock_movements WHERE variant_id = ? AND reason_code = 'recount'",
		e.variantM).Scan(&deltas); err != nil {
		t.Fatalf("delta check: %v", err)
	}
	if deltas != 17 {
		t.Errorf("recount delta total = %d, want 17 (-8 then +25)", deltas)
	}
}
