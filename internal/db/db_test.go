package db

import (
	"context"
	"database/sql"
	"os"
	"path/filepath"
	"testing"
)

// openTestDB returns a migrated database in a throwaway directory.
func openTestDB(t *testing.T) (ctx context.Context, sqldb *sql.DB, path string) {
	t.Helper()
	path = filepath.Join(t.TempDir(), "test.db")
	db, err := Open(path)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	if err := Migrate(db); err != nil {
		t.Fatalf("Migrate: %v", err)
	}
	return context.Background(), db, path
}

func TestOpenCreatesDurableWALDatabase(t *testing.T) {
	_, db, path := openTestDB(t)

	if _, err := os.Stat(path); err != nil {
		t.Fatalf("database file not created: %v", err)
	}

	var journal string
	if err := db.QueryRow("PRAGMA journal_mode").Scan(&journal); err != nil {
		t.Fatalf("read journal_mode: %v", err)
	}
	if journal != "wal" {
		t.Errorf("journal_mode = %q, want wal", journal)
	}

	var fk int
	if err := db.QueryRow("PRAGMA foreign_keys").Scan(&fk); err != nil {
		t.Fatalf("read foreign_keys: %v", err)
	}
	if fk != 1 {
		t.Errorf("foreign_keys = %d, want 1", fk)
	}
}

func TestMigrateIsIdempotent(t *testing.T) {
	_, db, _ := openTestDB(t)

	// Reference data from the migrations must be present.
	var channels int
	if err := db.QueryRow("SELECT COUNT(*) FROM sales_channels").Scan(&channels); err != nil {
		t.Fatalf("count sales_channels: %v", err)
	}
	if channels == 0 {
		t.Fatal("migrations seeded no sales channels")
	}

	// Running again must neither fail nor duplicate reference rows.
	if err := Migrate(db); err != nil {
		t.Fatalf("second Migrate: %v", err)
	}
	var channelsAfter int
	if err := db.QueryRow("SELECT COUNT(*) FROM sales_channels").Scan(&channelsAfter); err != nil {
		t.Fatalf("count sales_channels after re-migrate: %v", err)
	}
	if channelsAfter != channels {
		t.Errorf("sales_channels changed from %d to %d after re-migrate", channels, channelsAfter)
	}
}

func TestForeignKeysAreEnforced(t *testing.T) {
	_, db, _ := openTestDB(t)

	// product_variants.product_id references products(id): a dangling id must fail.
	_, err := db.Exec(
		`INSERT INTO product_variants (product_id, sku, cost_minor, retail_price_minor)
		 VALUES (999999, 'DANGLING', 100, 200)`)
	if err == nil {
		t.Fatal("insert with dangling product_id succeeded; foreign_keys not enforced")
	}

	// A CHECK constraint on cost_minor must also hold.
	product, err := db.Exec(
		`INSERT INTO products (name, slug) VALUES ('Tee', 'tee')`)
	if err != nil {
		t.Fatalf("insert product: %v", err)
	}
	pid, _ := product.LastInsertId()
	if _, err := db.Exec(
		`INSERT INTO product_variants (product_id, sku, cost_minor, retail_price_minor)
		 VALUES (?, 'NEG', -1, 200)`, pid); err == nil {
		t.Fatal("negative cost_minor accepted; CHECK not enforced")
	}
}

func TestBackupAndRestoreDrill(t *testing.T) {
	ctx, db, _ := openTestDB(t)

	dest := filepath.Join(t.TempDir(), "backup.db")
	if err := Backup(ctx, db, dest); err != nil {
		t.Fatalf("Backup: %v", err)
	}
	if _, err := os.Stat(dest); err != nil {
		t.Fatalf("backup file missing: %v", err)
	}

	result, err := RestoreDrill(ctx, dest)
	if err != nil {
		t.Fatalf("RestoreDrill: %v", err)
	}
	if result != "ok" {
		t.Errorf("integrity_check = %q, want ok", result)
	}
}

func TestBackupRefusesToOverwrite(t *testing.T) {
	ctx, db, _ := openTestDB(t)

	dest := filepath.Join(t.TempDir(), "backup.db")
	if err := Backup(ctx, db, dest); err != nil {
		t.Fatalf("first Backup: %v", err)
	}
	if err := Backup(ctx, db, dest); err == nil {
		t.Fatal("second Backup to the same path succeeded, want refusal")
	}
}

func TestRestoreDrillMissingFile(t *testing.T) {
	if _, err := RestoreDrill(context.Background(), filepath.Join(t.TempDir(), "nope.db")); err == nil {
		t.Fatal("RestoreDrill on a missing file succeeded")
	}
}
