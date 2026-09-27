package db

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log"

	"github.com/yourname/varels_cms/internal/auth"
	"github.com/yourname/varels_cms/internal/db/sqlc"
	"github.com/yourname/varels_cms/internal/db/types"
)

// SeedOptions controls first-run bootstrapping.
type SeedOptions struct {
	// InitialOwnerEmail becomes the first Google admin when approved_users is
	// empty (ADR-0004).
	InitialOwnerEmail string
	// AdminEmail + AdminPassword create the break-glass local admin. Optional;
	// when the password is empty no local account is managed.
	AdminEmail    string
	AdminPassword string
	// Dev inserts a small sample catalog.
	Dev bool
}

// Seed performs the first-run bootstrap and, when Dev is set, inserts a small
// sample catalog so the UI has something to render.
//
// The migration files already seed the reference data (sales channels,
// locations, reason codes, expense categories, settings), so this only handles
// the parts that depend on runtime configuration.
func Seed(ctx context.Context, sqldb *sql.DB, opts SeedOptions) error {
	q := sqlc.New(sqldb)

	if err := bootstrapAdmin(ctx, q, opts.InitialOwnerEmail); err != nil {
		return err
	}
	if err := bootstrapLocalAdmin(ctx, q, opts.AdminEmail, opts.AdminPassword); err != nil {
		return err
	}
	if opts.Dev {
		if err := seedDev(ctx, q); err != nil {
			return err
		}
	}
	return nil
}

// bootstrapAdmin creates the first admin from INITIAL_OWNER_EMAIL when the
// approved_users table is empty (ADR-0004, docs/auth.md).
func bootstrapAdmin(ctx context.Context, q *sqlc.Queries, email string) error {
	n, err := q.CountApprovedUsers(ctx)
	if err != nil {
		return fmt.Errorf("seed: count approved_users: %w", err)
	}
	if n > 0 {
		return nil
	}
	if email == "" {
		return errors.New("seed: approved_users is empty and INITIAL_OWNER_EMAIL is not set")
	}

	user, err := q.CreateApprovedUser(ctx, sqlc.CreateApprovedUserParams{
		Email:    email,
		Role:     types.UserRoleAdmin,
		Provider: types.AuthProviderGoogle,
	})
	if err != nil {
		return fmt.Errorf("seed: bootstrap admin %q: %w", email, err)
	}
	log.Printf("seed: bootstrapped first admin %s (id=%d)", user.Email, user.ID)
	return nil
}

// bootstrapLocalAdmin ensures the break-glass local admin exists with the
// configured password (auth.md, ADR-0004). It is a no-op when no password is
// configured. The account is an active admin, reachable only through the
// unlinked /admin-login route.
func bootstrapLocalAdmin(ctx context.Context, q *sqlc.Queries, email, password string) error {
	if password == "" {
		return nil
	}
	if email == "" {
		return errors.New("seed: ADMIN_PASSWORD is set but ADMIN_EMAIL is empty")
	}

	hash, err := auth.HashPassword(password)
	if err != nil {
		return fmt.Errorf("seed: hash admin password: %w", err)
	}
	hashNS := sql.NullString{String: hash, Valid: true}

	existing, err := q.GetApprovedUserByEmail(ctx, email)
	if errors.Is(err, sql.ErrNoRows) {
		user, err := q.CreateApprovedUser(ctx, sqlc.CreateApprovedUserParams{
			Email:        email,
			DisplayName:  sql.NullString{String: "Break-glass admin", Valid: true},
			Role:         types.UserRoleAdmin,
			Provider:     types.AuthProviderLocal,
			PasswordHash: hashNS,
		})
		if err != nil {
			return fmt.Errorf("seed: create local admin %q: %w", email, err)
		}
		log.Printf("seed: created break-glass local admin %s (id=%d)", user.Email, user.ID)
		return nil
	}
	if err != nil {
		return fmt.Errorf("seed: lookup local admin %q: %w", email, err)
	}
	if existing.Provider != types.AuthProviderLocal {
		return fmt.Errorf("seed: %q already exists as a %s account; cannot use it for the local admin", email, existing.Provider)
	}

	if err := q.SetApprovedUserPassword(ctx, sqlc.SetApprovedUserPasswordParams{
		PasswordHash: hashNS,
		ID:           existing.ID,
	}); err != nil {
		return fmt.Errorf("seed: update local admin password: %w", err)
	}
	log.Printf("seed: refreshed break-glass local admin %s (id=%d)", email, existing.ID)
	return nil
}

// seedDev inserts one category, one product with two variants, and an
// opening-balance movement so stock levels are populated in development.
func seedDev(ctx context.Context, q *sqlc.Queries) error {
	products, err := q.ListProducts(ctx, sqlc.ListProductsParams{
		IsArchived: 0,
		CategoryID: nil,
		Offset:     0,
		Limit:      1,
	})
	if err != nil {
		return fmt.Errorf("seed: list products: %w", err)
	}
	if len(products) > 0 {
		return nil
	}

	cat, err := q.CreateCategory(ctx, sqlc.CreateCategoryParams{
		Name:      "Tops",
		Slug:      "tops",
		SortOrder: 0,
	})
	if err != nil {
		return fmt.Errorf("seed: create category: %w", err)
	}

	product, err := q.CreateProduct(ctx, sqlc.CreateProductParams{
		Name:        "Classic Tee",
		Slug:        "classic-tee",
		Description: sql.NullString{String: "Heavyweight cotton tee.", Valid: true},
		CategoryID:  sql.NullInt64{Int64: cat.ID, Valid: true},
	})
	if err != nil {
		return fmt.Errorf("seed: create product: %w", err)
	}

	locations, err := q.ListLocations(ctx, 1)
	if err != nil {
		return fmt.Errorf("seed: list locations: %w", err)
	}
	if len(locations) == 0 {
		return errors.New("seed: no active location; run migrations first")
	}
	locationID := locations[0].ID

	samples := []struct {
		sku   string
		size  string
		color string
		qty   int64
	}{
		{"TEE-BLK-M", "M", "Black", 8},
		{"TEE-BLK-L", "L", "Black", 5},
	}

	for _, s := range samples {
		variant, err := q.CreateVariant(ctx, sqlc.CreateVariantParams{
			ProductID:         product.ID,
			Sku:               s.sku,
			Size:              sql.NullString{String: s.size, Valid: true},
			Color:             sql.NullString{String: s.color, Valid: true},
			CostMinor:         500000,
			RetailPriceMinor:  1200000,
			LowStockThreshold: 3,
		})
		if err != nil {
			return fmt.Errorf("seed: create variant %s: %w", s.sku, err)
		}
		if _, err := q.CreateStockMovement(ctx, sqlc.CreateStockMovementParams{
			VariantID:     variant.ID,
			LocationID:    locationID,
			State:         types.StockStateSellable,
			QuantityDelta: s.qty,
			ReasonCode:    sql.NullString{String: "opening_balance", Valid: true},
			RefType:       types.StockRefAdjustment,
			Note:          sql.NullString{String: "dev seed", Valid: true},
		}); err != nil {
			return fmt.Errorf("seed: opening balance %s: %w", s.sku, err)
		}
	}

	log.Printf("seed: inserted dev catalog (%s)", product.Slug)
	return nil
}
