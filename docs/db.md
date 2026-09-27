# The `internal/db` Folder — How It Works

`internal/db` is the entire data layer of the app. It owns the schema, the
migrations, the SQL queries, the generated Go data-access code, and the shared
enum types. Handlers call into it; nothing outside this folder (except
`cmd/server`) talks to the database directly.

It implements the "golden triangle" data vertex from `docs/architecture.md` and
ADR-0001: **no models, no repository, no services** — sqlc-generated structs are
passed straight into handlers and views.

```
internal/db/
├── db.go              # open the SQLite connection + PRAGMAs        (stub)
├── seed.go            # first-run admin bootstrap + dev seed         (stub)
├── schema.sql         # sqlc input schema (mirrors migrations)
├── migrations/        # goose migration files (Up/Down)
│   ├── 0001_init.sql
│   └── 0002_users.sql
├── queries/           # hand-written SQL that sqlc turns into Go
│   ├── auth.sql
│   ├── catalog.sql
│   ├── inventory.sql
│   ├── sales.sql
│   ├── finance.sql
│   └── analytics.sql
├── types/             # named Go string types for CHECK enums
│   └── types.go
└── sqlc/              # GENERATED — do not edit
    ├── db.go          # DBTX interface, Queries struct, WithTx
    ├── querier.go     # Querier interface (every generated method)
    ├── models.go      # struct per table
    ├── {domain}.sql.go
    └── doc.go
```

---

## 1. `schema.sql` — the source of truth for generated code

`schema.sql` is the **sqlc input schema**. sqlc reads it (plus `queries/`) to
generate type-safe Go structs and query methods.

- It is **DDL only** — no goose directives, no seed data.
- It **must mirror the applied goose migrations** (`0001_init` + `0002_users`).
  If you add a table or column, change _both_ `schema.sql` and the migration, or
  the generated structs will not match the live database. This is the one
  maintenance contract that matters most in this folder.
- It contains all `CREATE TABLE`, the `stock_movements_after_insert` trigger
  (ADR-0017), and the indexes.

Table groups (see `docs/schema.md` §3 for full column docs):

| Group            | Tables                                                                                                                                                         |
| ---------------- | -------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| Catalog          | `categories`, `collections`, `products`, `product_collections`, `product_variants`, `product_media`, `cost_history`, `cost_layers`, `price_tiers`, `discounts` |
| Inventory        | `locations`, `sales_channels`, `stock_levels`, `reason_codes`, `stock_movements`, `stock_holds`                                                                |
| Procurement      | `suppliers`, `purchase_orders`, `purchase_order_items`                                                                                                         |
| Customers        | `customers`                                                                                                                                                    |
| Orders / returns | `orders`, `order_items`, `payments`, `returns`, `return_items`                                                                                                 |
| Finance          | `expense_categories`, `expenses`                                                                                                                               |
| System           | `audit_logs`, `settings`                                                                                                                                       |
| Auth             | `approved_users`, `sessions`                                                                                                                                   |

### The stock trigger (ADR-0017)

`schema.sql` (and `0001_init` migration) defines:

```sql
CREATE TRIGGER stock_movements_after_insert
AFTER INSERT ON stock_movements
BEGIN
    INSERT OR IGNORE INTO stock_levels (... quantity = 0 ...);
    UPDATE stock_levels
       SET quantity = quantity + NEW.quantity_delta, ...
     WHERE variant_id  = NEW.variant_id
       AND location_id = NEW.location_id
       AND state       = NEW.state;
END;
```

`stock_movements` is the **single writer** of current stock. Handlers insert a
movement row; the trigger keeps `stock_levels.quantity` in sync. `stock_levels`
is never written directly (ADR-0017).

---

## 2. `migrations/` — goose

Migrations are applied with `make migrate` →
`goose -dir internal/db/migrations sqlite3 data/app.db up`.

Each file is a plain `.sql` with both directions in one file via goose
directives:

```sql
-- +goose Up
CREATE TABLE ...;

-- +goose StatementBegin
CREATE TRIGGER ...;      -- multi-statement blocks need StatementBegin/End
-- +goose StatementEnd

-- +goose Down
DROP TRIGGER ...;
DROP TABLE ...;
```

- `0001_init.sql` — catalog, inventory, orders/sales, returns, finance/OpEx,
  audit, suppliers & purchase orders (the PO tables ship now but their UI is
  deferred per ADR-0011).
- `0002_users.sql` — `approved_users` + `sessions` (auth, ADR-0004).

**Rule:** schema changes go here first, then are mirrored into `schema.sql`.

---

## 3. `queries/` — hand-written SQL, split by domain

Each file holds annotated queries that sqlc compiles into Go methods. A query is
written in plain SQL with a `-- name:` directive and a result shape:

| Annotation                     | Meaning                             |
| ------------------------------ | ----------------------------------- |
| `-- name: ListProducts :many`  | returns `[]Product` (multiple rows) |
| `-- name: GetProduct :one`     | returns `Product` (single row)      |
| `-- name: UpdateProduct :exec` | returns only `error` (no rows)      |

Parameter placeholders use sqlc's named syntax:

- `sqlc.arg(name)` — required parameter
- `sqlc.narg(name)` — **nullable** parameter (becomes a `sql.Null*` / pointer,
  and can be passed `NULL`)

Example from `catalog.sql`:

```sql
-- name: SearchProducts :many
SELECT * FROM products
WHERE is_archived = 0
  AND (name LIKE sqlc.arg(q) OR slug LIKE sqlc.arg(q))
ORDER BY name
LIMIT sqlc.arg(limit);
```

This generates `SearchProducts(ctx, SearchProductsParams)` returning
`[]sqlc.Product`.

The six domains:

| File            | Covers                                                                                          |
| --------------- | ----------------------------------------------------------------------------------------------- |
| `auth.sql`      | OAuth whitelist lookup, local admin password, sessions, `last_login_at`                         |
| `catalog.sql`   | categories, collections, products, variants, media, price tiers, cost history/layers, discounts |
| `inventory.sql` | locations, channels, stock levels/movements, reason codes, holds, suppliers, purchase orders    |
| `sales.sql`     | customers, orders, order items, payments, returns                                               |
| `finance.sql`   | expenses, expense categories, settings, audit log                                               |
| `analytics.sql` | best sellers, sizes, COGS, revenue by channel/location, deadstock, valuation                    |

---

## 4. `types/` — named enum types

`types/types.go` defines a named Go `string` type per CHECK-constrained TEXT
column, e.g.:

```go
type OrderStatus string
const (
    OrderStatusCompleted OrderStatus = "completed"
    ...
)
```

`sqlc.yaml` maps each enum column to its type via `overrides`, so generated
structs use `types.OrderStatus` instead of a bare `string`. The override imports
the module path (`github.com/yourname/varels_cms/internal/db/types`).

This gives compile-time safety on enum values and is the companion to
`internal/auth/roles.go`, which holds the runtime role helpers
(`CanViewFinancials`) for `admin` vs `staff`.

> Note: `internal/auth` also defines a `Role` type for the _request_ layer.
> `types.UserRole` is the _database_ representation; they carry the same string
> values and are kept separate because sqlc owns `types`, not `auth`.

---

## 5. `sqlc/` — generated code (do not edit)

Produced by `make sqlc` → `sqlc generate`. The `sqlc.yaml` config sets
`emit_interface`, `emit_json_tags`, `emit_empty_slices`, and
`emit_prepared_queries: false`.

| File              | Contents                                                                                                                                                         |
| ----------------- | ---------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| `db.go`           | `DBTX` interface (a subset of `*sql.DB`/`*sql.Tx`), the `Queries` struct, `New(db)` constructor, and `WithTx(tx)` for running a query set inside one transaction |
| `querier.go`      | the `Querier` interface — the full catalogue of generated methods (~160), one per annotated query                                                                |
| `models.go`       | one struct per table, with json tags and the enum overrides applied                                                                                              |
| `{domain}.sql.go` | the method implementations + their `Params` / `Row` structs                                                                                                      |

### How a handler uses it

```go
q := sqlc.New(db)                       // whole app, or:
qtx := q.WithTx(tx)                     // same API, bound to one transaction

product, err := q.GetProduct(ctx, id)   // sqlc.Product
variants, err := q.ListVariantsByProduct(ctx, sqlc.ListVariantsByProductParams{
    ProductID: id, IsArchived: 0,
})
```

`Queries` is deliberately tiny to construct — the canonical pattern is a single
`*sqlc.Queries` stored on the handler server struct, and a fresh `WithTx` scope
inside each multi-write handler (order create, restock, adjustment, return —
see `docs/routes.md` §1 "Transactions").

---

## 6. `db.go` and `seed.go` — connection & bootstrap (currently stubs)

These two are the only _runtime_ Go files in the folder and are not yet
implemented (both carry `TODO` markers).

- `db.Open(path)` — intended to register the SQLite driver, open the connection,
  and apply the durability PRAGMAs from ADR-0010:
  - `journal_mode=WAL`
  - `foreign_keys=ON`
  - `busy_timeout` (single-instance SQLite, ADR-0015)
  - configure the `database/sql` connection pool
- `seed.go` — intended to:
  - bootstrap the first admin: when `approved_users` is empty, create
    `INITIAL_OWNER_EMAIL` with role `admin` (ADR-0004)
  - optionally seed dev fixtures (reason codes like `opening_balance`,
    default location/channel, sample categories)

Until these are implemented, the server can't open a real database
(`cmd/server/main.go` has a matching TODO).

---

## 7. The workflow when you add a feature

For a new query or table the change flows through exactly these files:

1. **Migrate** — add/alter the table in `migrations/NNNN_*.sql` (Up + Down).
2. **Mirror** — reflect the same change in `schema.sql`.
3. **Query** — add the annotated SQL to the relevant `queries/*.sql`.
4. **Enum** (if a new CHECK column) — add the type + constants in
   `types/types.go` and an `override` in `sqlc.yaml`.
5. **Generate** — `make sqlc` (regenerates `internal/db/sqlc`).
6. **Consume** — call the new method from a handler in `internal/handlers`.

`make generate` (templ) and `make css` are unrelated to this folder; only
`make sqlc` and `make migrate` touch `internal/db`.

---

## 8. Conventions that live here

These are enforced by the schema/queries rather than documented twice — see the
ADRs for rationale:

- **Money** = `INTEGER` ARS centavos (`*_minor` columns), never float (ADR-0005).
- **Timestamps** = ISO-8601 UTC `TEXT` via `strftime('%Y-%m-%dT%H:%M:%SZ','now')`;
  business grouping is Argentina-local and done in the caller (ADR-0014).
- **Soft delete** = `is_archived` / `is_active` flags, not `DELETE` (ADR-0010).
- **Discounts** = integer basis points (`discounts.value`, ADR-0016).
- **FIFO COGS** = `cost_layers` consumed oldest-first (ADR-0012).
- **IVA** = retail prices are IVA-inclusive; `tax_minor` is display-only (ADR-0013).
- **Audit** = every financial/stock write also writes `audit_logs` (ADR-0010).
