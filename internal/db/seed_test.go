package db

import (
	"database/sql"
	"testing"

	"github.com/druidswebdesign/varels-cms/internal/auth"
)

func countApproved(t *testing.T, db *sql.DB) int {
	t.Helper()
	var n int
	if err := db.QueryRow("SELECT COUNT(*) FROM approved_users").Scan(&n); err != nil {
		t.Fatalf("count approved_users: %v", err)
	}
	return n
}

// TestSeedCreatesLocalAdmin verifies the first admin is a local email + bcrypt
// account (Google OAuth bootstrap was removed).
func TestSeedCreatesLocalAdmin(t *testing.T) {
	ctx, db, _ := openTestDB(t)

	if countApproved(t, db) != 0 {
		t.Fatal("approved_users not empty after migrations; cannot test bootstrap")
	}
	opts := SeedOptions{AdminEmail: "owner@example.com", AdminPassword: "s3cret-pass"}
	if err := Seed(ctx, db, opts); err != nil {
		t.Fatalf("Seed: %v", err)
	}
	if got := countApproved(t, db); got != 1 {
		t.Fatalf("approved_users = %d, want 1", got)
	}

	var role, provider, hash string
	if err := db.QueryRow(
		"SELECT role, provider, password_hash FROM approved_users WHERE email = ?",
		"owner@example.com").Scan(&role, &provider, &hash); err != nil {
		t.Fatalf("read admin: %v", err)
	}
	if role != "admin" {
		t.Errorf("role = %q, want admin", role)
	}
	if provider != "local" {
		t.Errorf("provider = %q, want local", provider)
	}
	if !auth.CheckPassword(hash, "s3cret-pass") {
		t.Error("stored hash does not verify the configured password")
	}
}

// TestSeedLocalAdminIdempotent verifies re-seeding rotates the password without
// duplicating the row.
func TestSeedLocalAdminIdempotent(t *testing.T) {
	ctx, db, _ := openTestDB(t)

	opts := SeedOptions{AdminEmail: "owner@example.com", AdminPassword: "s3cret-pass"}
	if err := Seed(ctx, db, opts); err != nil {
		t.Fatalf("first Seed: %v", err)
	}
	opts.AdminPassword = "rotated-pass"
	if err := Seed(ctx, db, opts); err != nil {
		t.Fatalf("second Seed: %v", err)
	}
	if got := countApproved(t, db); got != 1 {
		t.Errorf("approved_users = %d after re-seed, want 1", got)
	}

	var hash string
	if err := db.QueryRow(
		"SELECT password_hash FROM approved_users WHERE email = ?",
		"owner@example.com").Scan(&hash); err != nil {
		t.Fatalf("read hash: %v", err)
	}
	if !auth.CheckPassword(hash, "rotated-pass") {
		t.Error("password was not rotated on re-seed")
	}
}

// TestSeedNoConfigIsNoop verifies seeding without credentials is a no-op: there
// is no public signup and no implicit first user.
func TestSeedNoConfigIsNoop(t *testing.T) {
	ctx, db, _ := openTestDB(t)

	if err := Seed(ctx, db, SeedOptions{}); err != nil {
		t.Fatalf("Seed: %v", err)
	}
	if got := countApproved(t, db); got != 0 {
		t.Errorf("approved_users = %d, want 0", got)
	}
}

func TestSeedRejectsLocalAdminWithoutEmail(t *testing.T) {
	ctx, db, _ := openTestDB(t)

	err := Seed(ctx, db, SeedOptions{AdminPassword: "s3cret-pass"})
	if err == nil {
		t.Fatal("Seed accepted ADMIN_PASSWORD without ADMIN_EMAIL")
	}
}

func TestSeedDevCatalog(t *testing.T) {
	ctx, db, _ := openTestDB(t)

	if err := Seed(ctx, db, SeedOptions{Dev: true}); err != nil {
		t.Fatalf("Seed: %v", err)
	}

	var variants int
	if err := db.QueryRow("SELECT COUNT(*) FROM product_variants").Scan(&variants); err != nil {
		t.Fatalf("count variants: %v", err)
	}
	if variants != 2 {
		t.Fatalf("dev variants = %d, want 2", variants)
	}

	var stock int
	if err := db.QueryRow(
		"SELECT COALESCE(SUM(quantity),0) FROM stock_levels WHERE state = 'sellable'").Scan(&stock); err != nil {
		t.Fatalf("sum stock: %v", err)
	}
	if stock != 13 { // M:8 + L:5 opening balances
		t.Errorf("sellable stock = %d, want 13", stock)
	}

	// Re-running the dev seed must not add a second catalog.
	if err := Seed(ctx, db, SeedOptions{Dev: true}); err != nil {
		t.Fatalf("re-seed dev: %v", err)
	}
	if err := db.QueryRow("SELECT COUNT(*) FROM product_variants").Scan(&variants); err != nil {
		t.Fatalf("recount variants: %v", err)
	}
	if variants != 2 {
		t.Errorf("dev variants = %d after re-seed, want 2", variants)
	}
}
