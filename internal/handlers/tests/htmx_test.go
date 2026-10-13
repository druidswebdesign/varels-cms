package tests

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"strings"
	"testing"
)

// postHX issues a form-encoded POST flagged as an HTMX request.
func (e *testEnv) postHX(path string, form url.Values) *httptest.ResponseRecorder {
	e.t.Helper()
	req := httptest.NewRequest(http.MethodPost, path, strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("HX-Request", "true")
	rec := httptest.NewRecorder()
	e.router.ServeHTTP(rec, req)
	return rec
}

// TestHTMX_RestockPanelFragment pins the fragment/OOB contract from
// docs/frontend.md §6: an HTMX restock re-renders #restock-panel with an
// out-of-band flash instead of redirecting (docs/testing.md order 3).
func TestHTMX_RestockPanelFragment(t *testing.T) {
	e := newTestEnv(t)

	rec := e.postHX("/variants/"+strconv.FormatInt(e.variantM, 10)+"/restock", url.Values{
		"quantity":    {"2"},
		"cost":        {"5000"},
		"location_id": {strconv.FormatInt(e.locationID, 10)},
	})
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body = %s", rec.Code, rec.Body.String())
	}
	body := rec.Body.String()
	if !strings.Contains(body, `id="restock-panel"`) {
		t.Errorf("HTMX restock response missing #restock-panel")
	}
	if !strings.Contains(body, `hx-swap-oob="true"`) {
		t.Errorf("HTMX restock response missing out-of-band flash slot")
	}
	if got, want := e.sellableStock(e.variantM), int64(10); got != want {
		t.Errorf("stock after restock = %d, want %d", got, want)
	}
	e.assertStockLevelsMatchLedger()
}

// TestHTMX_AdjustErrorReturnsPanel proves a rejected HTMX adjustment returns the
// same fragment with an error flash and writes no movement.
func TestHTMX_AdjustErrorReturnsPanel(t *testing.T) {
	e := newTestEnv(t)
	before := e.movementCount()

	rec := e.postHX("/variants/"+strconv.FormatInt(e.variantM, 10)+"/adjust", url.Values{
		"quantity":    {"-1"},
		"location_id": {strconv.FormatInt(e.locationID, 10)},
	})
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body = %s", rec.Code, rec.Body.String())
	}
	body := rec.Body.String()
	if !strings.Contains(body, `id="restock-panel"`) {
		t.Errorf("HTMX adjust error response missing #restock-panel")
	}
	if !strings.Contains(body, `hx-swap-oob="true"`) {
		t.Errorf("HTMX adjust error response missing out-of-band flash slot")
	}
	if got := e.movementCount(); got != before {
		t.Errorf("movements = %d, want unchanged %d", got, before)
	}
}
