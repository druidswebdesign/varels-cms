package service

import (
	"context"
	"database/sql"
	"path/filepath"
	"testing"

	appdb "github.com/druidswebdesign/varels-cms/internal/db"
	"github.com/druidswebdesign/varels-cms/internal/db/sqlc"
)

// inventoryFixture is a migrated+dev-seeded database plus one variant to cost.
type inventoryFixture struct {
	t         *testing.T
	db        *sql.DB
	q         *sqlc.Queries
	variantID int64
}

func newInventoryFixture(t *testing.T) *inventoryFixture {
	t.Helper()
	ctx := context.Background()

	sqldb, err := appdb.Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	t.Cleanup(func() { _ = sqldb.Close() })
	if err := appdb.Migrate(sqldb); err != nil {
		t.Fatalf("Migrate: %v", err)
	}
	if err := appdb.Seed(ctx, sqldb, appdb.SeedOptions{
		Dev: true,
	}); err != nil {
		t.Fatalf("Seed: %v", err)
	}

	var id int64
	if err := sqldb.QueryRow("SELECT id FROM product_variants WHERE sku = 'TEE-BLK-M'").Scan(&id); err != nil {
		t.Fatalf("find variant: %v", err)
	}
	return &inventoryFixture{t: t, db: sqldb, q: sqlc.New(sqldb), variantID: id}
}

func (f *inventoryFixture) addLayer(qty, unitCost int64, receivedAt string) {
	f.t.Helper()
	if _, err := f.q.CreateCostLayer(context.Background(), sqlc.CreateCostLayerParams{
		VariantID:     f.variantID,
		ReceivedAt:    receivedAt,
		QtyReceived:   qty,
		QtyRemaining:  qty,
		UnitCostMinor: unitCost,
		SourceRef:     sql.NullString{String: "test", Valid: true},
	}); err != nil {
		f.t.Fatalf("CreateCostLayer: %v", err)
	}
}

func (f *inventoryFixture) openLayerRemaining() int64 {
	f.t.Helper()
	var remaining int64
	if err := f.db.QueryRow(
		"SELECT COALESCE(SUM(qty_remaining),0) FROM cost_layers WHERE variant_id = ?",
		f.variantID).Scan(&remaining); err != nil {
		f.t.Fatalf("sum layers: %v", err)
	}
	return remaining
}

func TestConsumeFIFOAverageCost(t *testing.T) {
	f := newInventoryFixture(t)
	f.addLayer(3, 400000, "2026-01-01T00:00:00Z")
	f.addLayer(2, 600000, "2026-02-01T00:00:00Z")

	got, err := ConsumeFIFO(context.Background(), f.q, f.variantID, 5, 999999)
	if err != nil {
		t.Fatalf("ConsumeFIFO: %v", err)
	}
	// (3*400000 + 2*600000) / 5 = 480000
	if got != 480000 {
		t.Errorf("unit cost = %d, want 480000", got)
	}
	if rem := f.openLayerRemaining(); rem != 0 {
		t.Errorf("layers remaining = %d, want 0", rem)
	}
}

func TestConsumeFIFOPartialLeavesNewestLayer(t *testing.T) {
	f := newInventoryFixture(t)
	f.addLayer(3, 400000, "2026-01-01T00:00:00Z")
	f.addLayer(2, 600000, "2026-02-01T00:00:00Z")

	got, err := ConsumeFIFO(context.Background(), f.q, f.variantID, 4, 999999)
	if err != nil {
		t.Fatalf("ConsumeFIFO: %v", err)
	}
	// (3*400000 + 1*600000) / 4 = 450000
	if got != 450000 {
		t.Errorf("unit cost = %d, want 450000", got)
	}
	if rem := f.openLayerRemaining(); rem != 1 {
		t.Errorf("layers remaining = %d, want 1 (newest layer partly open)", rem)
	}
}

func TestConsumeFIFOOnlyConsumesOldestFirst(t *testing.T) {
	f := newInventoryFixture(t)
	f.addLayer(2, 100000, "2026-01-01T00:00:00Z")
	f.addLayer(2, 900000, "2026-02-01T00:00:00Z")

	if _, err := ConsumeFIFO(context.Background(), f.q, f.variantID, 2, 0); err != nil {
		t.Fatalf("ConsumeFIFO: %v", err)
	}

	// The expensive layer must be untouched: 900000 is still fully open.
	var newest int64
	if err := f.db.QueryRow(
		"SELECT qty_remaining FROM cost_layers WHERE variant_id = ? AND unit_cost_minor = 900000",
		f.variantID).Scan(&newest); err != nil {
		t.Fatalf("read newest layer: %v", err)
	}
	if newest != 2 {
		t.Errorf("newest layer remaining = %d, want 2", newest)
	}
}

func TestConsumeFIFOFallsBackToCurrentCost(t *testing.T) {
	f := newInventoryFixture(t)

	got, err := ConsumeFIFO(context.Background(), f.q, f.variantID, 3, 500000)
	if err != nil {
		t.Fatalf("ConsumeFIFO: %v", err)
	}
	if got != 500000 {
		t.Errorf("unit cost = %d, want fallback 500000", got)
	}
}

func TestConsumeFIFOFallsBackForUncoveredQuantity(t *testing.T) {
	f := newInventoryFixture(t)
	f.addLayer(2, 400000, "2026-01-01T00:00:00Z")

	got, err := ConsumeFIFO(context.Background(), f.q, f.variantID, 4, 600000)
	if err != nil {
		t.Fatalf("ConsumeFIFO: %v", err)
	}
	// (2*400000 + 2*600000) / 4 = 500000
	if got != 500000 {
		t.Errorf("unit cost = %d, want 500000", got)
	}
}

func TestConsumeFIFONonPositiveQuantityIsNoop(t *testing.T) {
	f := newInventoryFixture(t)
	f.addLayer(2, 400000, "2026-01-01T00:00:00Z")

	got, err := ConsumeFIFO(context.Background(), f.q, f.variantID, 0, 999999)
	if err != nil {
		t.Fatalf("ConsumeFIFO: %v", err)
	}
	if got != 0 {
		t.Errorf("unit cost = %d, want 0", got)
	}
	if rem := f.openLayerRemaining(); rem != 2 {
		t.Errorf("layers remaining = %d, want 2 (untouched)", rem)
	}
}

func TestAddCostLayerIgnoresNonPositiveQuantity(t *testing.T) {
	f := newInventoryFixture(t)
	if err := AddCostLayer(context.Background(), f.q, f.variantID, 0, 400000, "test"); err != nil {
		t.Fatalf("AddCostLayer: %v", err)
	}
	if rem := f.openLayerRemaining(); rem != 0 {
		t.Errorf("layers remaining = %d, want 0", rem)
	}
}
