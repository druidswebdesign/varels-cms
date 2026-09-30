package tests

import (
	"context"
	"database/sql"
	"net/http"
	"net/http/httptest"
	"net/url"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"

	"github.com/druidswebdesign/varels-cms/internal/db"
	"github.com/druidswebdesign/varels-cms/internal/db/sqlc"
	"github.com/druidswebdesign/varels-cms/internal/db/types"
	"github.com/druidswebdesign/varels-cms/internal/handlers/common"
	"github.com/druidswebdesign/varels-cms/internal/handlers/inventory"
	"github.com/druidswebdesign/varels-cms/internal/handlers/sales"
)

// testEnv is a fresh migrated+seeded SQLite database wired to the real HTTP
// handlers behind a chi router. Authentication and CSRF middleware are omitted
// on purpose: these tests exercise the money/inventory logic in isolation, not
// the auth surface (covered by its own smoke checks).
type testEnv struct {
	t      *testing.T
	srv    *common.Server
	db     *sql.DB
	router http.Handler

	locationID int64
	channelID  int64
	variantM   int64 // dev seed TEE-BLK-M, 8 sellable units at locationID
	variantL   int64 // dev seed TEE-BLK-L, 5 sellable units at locationID
}

// newTestEnv builds an isolated environment. Dev seeding gives one product with
// two variants (M: 8 units, L: 5 units), shipped to the first active location.
func newTestEnv(t *testing.T) *testEnv {
	t.Helper()
	ctx := context.Background()

	sqldb, err := db.Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatalf("db.Open: %v", err)
	}
	t.Cleanup(func() { _ = sqldb.Close() })

	if err := db.Migrate(sqldb); err != nil {
		t.Fatalf("db.Migrate: %v", err)
	}
	if err := db.Seed(ctx, sqldb, db.SeedOptions{
		Dev: true,
	}); err != nil {
		t.Fatalf("db.Seed: %v", err)
	}

	srv := common.NewServer(sqldb, "", "test")
	srv.Sessions = nil // flash becomes a no-op; no session middleware under test

	e := &testEnv{t: t, srv: srv, db: sqldb}
	e.variantM = e.scalarInt("SELECT id FROM product_variants WHERE sku = 'TEE-BLK-M'")
	e.variantL = e.scalarInt("SELECT id FROM product_variants WHERE sku = 'TEE-BLK-L'")
	e.locationID = e.scalarInt(
		"SELECT location_id FROM stock_levels WHERE variant_id = ? AND state = 'sellable' LIMIT 1",
		e.variantM)
	e.channelID = e.scalarInt("SELECT id FROM sales_channels WHERE name = 'In-Store'")

	salesh := sales.New(srv)
	inventoryh := inventory.New(srv)

	r := chi.NewRouter()
	r.Post("/sales", salesh.HandleSaleCreate)
	r.Get("/sales/{id}", salesh.HandleSaleDetail)
	r.Post("/sales/{id}/void", salesh.HandleSaleVoid)
	r.Get("/sales/{id}/return", salesh.HandleReturnForm)
	r.Post("/sales/{id}/return", salesh.HandleReturnCreate)
	r.Post("/sales/{id}/payments", salesh.HandlePaymentCreate)
	r.Post("/variants/{id}/restock", inventoryh.HandleRestock)
	r.Post("/variants/{id}/adjust", inventoryh.HandleAdjustStock)
	r.Get("/variants/{id}/restock", inventoryh.HandleRestockForm)
	e.router = r

	return e
}

// post issues a form-encoded POST through the router.
func (e *testEnv) post(path string, form url.Values) *httptest.ResponseRecorder {
	e.t.Helper()
	req := httptest.NewRequest(http.MethodPost, path, strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec := httptest.NewRecorder()
	e.router.ServeHTTP(rec, req)
	return rec
}

// get issues a GET through the router.
func (e *testEnv) get(path string) *httptest.ResponseRecorder {
	e.t.Helper()
	req := httptest.NewRequest(http.MethodGet, path, nil)
	rec := httptest.NewRecorder()
	e.router.ServeHTTP(rec, req)
	return rec
}

// scalarInt runs a single-value query that must return exactly one integer.
func (e *testEnv) scalarInt(query string, args ...any) int64 {
	e.t.Helper()
	var v int64
	if err := e.db.QueryRow(query, args...).Scan(&v); err != nil {
		e.t.Fatalf("scalarInt %q: %v", query, err)
	}
	return v
}

// scalarIntOK runs a single-value query that may return no rows.
func (e *testEnv) scalarIntOK(query string, args ...any) (int64, bool) {
	e.t.Helper()
	var v int64
	err := e.db.QueryRow(query, args...).Scan(&v)
	if err == sql.ErrNoRows {
		return 0, false
	}
	if err != nil {
		e.t.Fatalf("scalarIntOK %q: %v", query, err)
	}
	return v, true
}

// sellableStock returns the sellable quantity for a variant at locationID.
func (e *testEnv) sellableStock(variantID int64) int64 {
	e.t.Helper()
	v, _ := e.scalarIntOK(
		"SELECT quantity FROM stock_levels WHERE variant_id = ? AND location_id = ? AND state = 'sellable'",
		variantID, e.locationID)
	return v
}

// stateStock returns the quantity for a variant at locationID in a given state.
func (e *testEnv) stateStock(variantID int64, state types.StockState) int64 {
	e.t.Helper()
	v, _ := e.scalarIntOK(
		"SELECT quantity FROM stock_levels WHERE variant_id = ? AND location_id = ? AND state = ?",
		variantID, e.locationID, string(state))
	return v
}

// orderCount returns the number of orders (used to prove rollbacks wrote nothing).
func (e *testEnv) orderCount() int64 {
	e.t.Helper()
	return e.scalarInt("SELECT COUNT(*) FROM orders")
}

// movementCount returns the number of stock movements.
func (e *testEnv) movementCount() int64 {
	e.t.Helper()
	return e.scalarInt("SELECT COUNT(*) FROM stock_movements")
}

// addLayer inserts a FIFO cost layer with an explicit received_at so ordering
// is deterministic regardless of insert speed.
func (e *testEnv) addLayer(variantID, qty, unitCost int64, receivedAt string) {
	e.t.Helper()
	if _, err := e.srv.Q.CreateCostLayer(context.Background(), sqlc.CreateCostLayerParams{
		VariantID:     variantID,
		ReceivedAt:    receivedAt,
		QtyReceived:   qty,
		QtyRemaining:  qty,
		UnitCostMinor: unitCost,
		SourceRef:     sql.NullString{String: "test", Valid: true},
	}); err != nil {
		e.t.Fatalf("CreateCostLayer: %v", err)
	}
}

// createCustomer inserts a customer and returns its id.
func (e *testEnv) createCustomer(name string) int64 {
	e.t.Helper()
	c, err := e.srv.Q.CreateCustomer(context.Background(), sqlc.CreateCustomerParams{
		Name:  name,
		Phone: sql.NullString{},
	})
	if err != nil {
		e.t.Fatalf("CreateCustomer: %v", err)
	}
	return c.ID
}

// baseSaleForm returns the form fields shared by every POS sale in the tests.
func (e *testEnv) baseSaleForm() url.Values {
	f := url.Values{}
	f.Set("channel_id", strconv.FormatInt(e.channelID, 10))
	f.Set("location_id", strconv.FormatInt(e.locationID, 10))
	f.Set("payment_method", string(types.PaymentMethodCash))
	return f
}

// sell posts a one-line cash/store-credit sale and returns (orderID, itemID).
func (e *testEnv) sell(variantID, qty int64, method types.PaymentMethod, customerID int64) (int64, int64) {
	e.t.Helper()
	form := e.baseSaleForm()
	form.Set("payment_method", string(method))
	if customerID != 0 {
		form.Set("customer_id", strconv.FormatInt(customerID, 10))
	}
	form.Add("variant_id", strconv.FormatInt(variantID, 10))
	form.Add("quantity", strconv.FormatInt(qty, 10))
	form.Add("unit_price", "12000") // 1.200.000 minor, the dev retail price
	form.Add("line_discount", "0")

	rec := e.post("/sales", form)
	if rec.Code != http.StatusSeeOther {
		e.t.Fatalf("sell: status = %d, body = %s", rec.Code, rec.Body.String())
	}
	loc := rec.Header().Get("Location")
	if !strings.HasPrefix(loc, "/sales/") {
		e.t.Fatalf("sell: redirect = %q", loc)
	}
	orderID, err := strconv.ParseInt(strings.TrimPrefix(loc, "/sales/"), 10, 64)
	if err != nil {
		e.t.Fatalf("sell: parse order id %q: %v", loc, err)
	}
	var itemID int64
	if err := e.db.QueryRow("SELECT id FROM order_items WHERE order_id = ? LIMIT 1", orderID).Scan(&itemID); err != nil {
		e.t.Fatalf("sell: order item: %v", err)
	}
	return orderID, itemID
}

// redirectID parses the numeric id from a 303 Location like "/sales/7".
func (e *testEnv) redirectID(rec *httptest.ResponseRecorder) int64 {
	e.t.Helper()
	loc := rec.Header().Get("Location")
	base := loc[strings.LastIndex(loc, "/")+1:]
	id, err := strconv.ParseInt(base, 10, 64)
	if err != nil {
		e.t.Fatalf("redirect Location %q has no numeric id: %v", loc, err)
	}
	return id
}

// storeCreditBalance returns a customer's ledger balance.
func (e *testEnv) storeCreditBalance(customerID int64) int64 {
	e.t.Helper()
	var v int64
	if err := e.db.QueryRow(
		"SELECT COALESCE(SUM(delta_minor),0) FROM store_credit_ledger WHERE customer_id = ?",
		customerID).Scan(&v); err != nil {
		e.t.Fatalf("store credit balance: %v", err)
	}
	return v
}
