# Implementation Log — varels_cms

Single chronological record of the build: the milestone plan, what was actually
implemented, how it was verified, and what is still open.

How to use this file:

- **The plan table below is the master status.** Keep it updated as milestones
  land.
- **The log is append-only.** Add a new dated entry per work session; do not
  rewrite earlier entries. Newest entries go below older ones (chronological).
- **Specs are not duplicated here.** Binding specs live at the `docs/` root
  (`overview.md`, `schema.md`, `routes.md`, `auth.md`, `architecture.md`,
  `adr.md`, `db.md`, `functions.md`). This file only tracks execution.
- **Live status & tracking:** `docs/ai-tracking.md` holds the current route /
  wiring snapshot and how to measure AI changes; this file is the history.

---

## Milestone plan & status

Derived from the initial repository state: specs, migrations, `schema.sql`,
`queries/*.sql`, and generated `internal/db/sqlc/*.go` were complete; every
runtime file was a stub and `go.mod` had zero dependencies. Work proceeds
bottom-up.

Model tiers: **Flash** = fastest/cheap model, for mechanical work against
existing specs; **Pro-class** = strongest reasoning model, for
security/money/transaction correctness. (The "Pro Max" / "4.1 Flash" labels are
not verifiable DeepSeek model IDs; DeepSeek's published lineup is
`deepseek-chat` / `deepseek-reasoner`.)

| #   | Task                                                                                                                 | Files                                                                                     | Difficulty                         | Model     | Status                               |
| --- | -------------------------------------------------------------------------------------------------------------------- | ----------------------------------------------------------------------------------------- | ---------------------------------- | --------- | ------------------------------------ |
| 0   | Bootstrap deps: chi, templ, sqlc runtime, sqlite driver, goose; `go mod tidy`                                        | `go.mod`                                                                                  | Easy                               | Flash     | ✅ Done                              |
| 1   | DB open: driver, PRAGMAs (WAL, FK, busy_timeout), pool config                                                        | `internal/db/db.go`                                                                       | Medium                             | Flash     | ✅ Done                              |
| 2   | Seed: first-run admin bootstrap + dev fixtures                                                                       | `internal/db/seed.go`                                                                     | Medium                             | Flash     | ✅ Done                              |
| 3   | Auth core: SCS sessions, Goth Google OAuth, whitelist lookup, bcrypt break-glass, `RequireLogin`/`RequireRole`       | `internal/auth/*.go`                                                                      | **Hard (security)**                | Pro-class | ✅ Done                              |
| 4   | Router + middleware: `Logger/Recover/RequestID/CSRF`, chi groups, base layout/navbar/sidebar, `/healthz`             | `main.go`, `handlers/middleware.go`, `views/layouts`, `components`                        | Medium                             | Flash     | ✅ Done                              |
| 5   | Catalog CRUD: products, variants (media, collections deferred) + views                                               | `product_handler.go`, `variant_handler.go`, catalog views                                 | Medium                             | Flash     | ✅ Done (media/collections deferred) |
| 6   | Inventory: restock (`cost_layers` + `cost_history`), adjustments w/ reason codes, movements ledger, low-stock widget | `restock_handler.go`, `dashboard_handler.go`                                              | **Hard (trigger/txn)**             | Pro-class | ✅ Done                              |
| 7   | Sales/POS + returns: multi-line order, payments, atomic stock writes, receipts; partial returns                      | `sale_handler.go`                                                                         | **Hardest (atomicity, FIFO COGS)** | Pro-class | ✅ Done                              |
| 8   | Analytics/P&L/deadstock + CSV exports, plus OpEx entry                                                               | `analytics_handler.go`, `deadstock_handler.go`, `export_handler.go`, `expense_handler.go` | Hard (financial math)              | Pro-class | ✅ Done                              |
| 9   | Staff/admin + audit log UI                                                                                           | `staff_handler.go`                                                                        | Easy–Medium                        | Flash     | ✅ Done (audit UI pending)           |
| 10  | Ops: backup, restore drill, Docker/deploy                                                                            | —                                                                                         | Medium                             | Flash/Pro | ✅ Done                              |

### Resolve before the dependent task

From `docs/schema.md` §9 — design decisions, not coding; use the Pro-class model:

1. **Store-credit ledger** (issued vs redeemed) — before returns (#7).
2. **Stock lifecycle**: when stock commits, `stock_holds` transitions,
   negative/oversell policy — before checkout (#7).
3. **Discount stacking + rounding**: line vs order vs channel, rounding step —
   before checkout (#7).

---

## Log

### 2026-09-25 — Flash scope implemented (#0, #1, #2, #4, #5, #9, #10)

Implemented the Flash and Flash+Pro milestones; #3/#6/#7/#8 left untouched by
design. Build order followed: dependencies → DB → web shell → catalog → staff →
ops.

#### #0 Dependencies

Added to `go.mod` / `go.sum`:

| Module                        | Used for                         |
| ----------------------------- | -------------------------------- |
| `github.com/go-chi/chi/v5`    | HTTP router                      |
| `github.com/a-h/templ`        | template runtime                 |
| `modernc.org/sqlite`          | pure-Go SQLite driver (CGO-free) |
| `github.com/pressly/goose/v3` | embedded migrations at startup   |

#### #1 Database foundation — `internal/db/db.go`

- `Open(path)` registers the `sqlite` (modernc) driver, creates the data
  directory, and applies the ADR-0010 PRAGMAs through the DSN so every pooled
  connection gets them: `busy_timeout=5000`, `journal_mode=WAL`,
  `foreign_keys=1`, `synchronous=NORMAL`.
- Pool pinned to a single connection (`MaxOpenConns`/`MaxIdleConns = 1`) to keep
  the single-instance SQLite deployment (ADR-0015) deterministic.
- `Migrate(db)` runs goose over `//go:embed migrations/*.sql`, so the binary is
  self-contained and idempotent.

#### #2 Seeding — `internal/db/seed.go`

- `Seed(ctx, db, initialOwnerEmail, dev)`.
- `bootstrapAdmin`: when `approved_users` is empty, inserts
  `INITIAL_OWNER_EMAIL` as `admin` (ADR-0004). Fails closed if the email is unset.
- `seedDev` (only when `APP_ENV=development` and no products exist): one
  category, one product, two variants, and `opening_balance` stock movements so
  the trigger populates `stock_levels`.
- Reference data (channels, locations, reason codes, expense categories,
  settings) already comes from the migration files, so it is not duplicated.

#### #4 Web shell

**Middleware — `internal/handlers/middleware.go`**

- `RequestID` (context + `X-Request-Id`), `Logger` (method/path/status/duration),
  `Recover` (panic → 500).
- `CSRF`: double-submit cookie (`csrf_token`) with `crypto/subtle` compare,
  accepting either the `X-CSRF-Token` header or the `csrf_token` form field.
  Needs no server-side session store.

**Shared handler plumbing**

- `internal/handlers/server.go` — `Server{DB, Q, DBPath, Env}` and `NewServer`.
- `internal/handlers/helpers.go` — templ `render`, `urlID`, `parsePesos`
  (integer-only minor-unit parsing, ADR-0005), `nullString`/`nullInt64`,
  `slugify`, `nowUTC`, `itoa`, `isUniqueViolation`, `serverError`, `isHX`.

**Router — `cmd/server/main.go`**

- chi router with the global chain `RequestID → Logger → Recover → CSRF`,
  graceful shutdown, and startup wiring (`db.Open` → `db.Migrate` → `db.Seed` →
  `handlers.NewServer`).
- Public routes `/login`, `/auth/denied`, `/logout`; `/assets/*`; `/healthz`.
- Auth-gated group uses `auth.RequireLogin`; admin group uses
  `auth.RequireRole(auth.RoleAdmin)` (both currently pass-through stubs).

**Views**

- `internal/views/format/format.go` — `Money` (es-AR `$1.234,56`),
  `PercentFromBps`, `NullString`, `NullInt64`, `GrossMarginPct`.
- `layouts/base.templ` — full-page shell with HTMX, ApexCharts CDN, Tailwind
  output, body-level `hx-headers` CSRF token.
- `components/{navbar,sidebar,badge,button,table,modal,flash}.templ`.

#### #5 Catalog CRUD

**Handlers**

- `product_handler.go`: list/search/archived, new/create, detail, edit/update,
  archive/restore. Duplicate slug renders a 422 form error; missing rows 404.
- `variant_handler.go`: HTMX variant table, create, archive, threshold.
- `dashboard_handler.go`: minimal landing page (product/variant counts,
  low-stock count) — financial cards belong to #8.

**Views**

- `pages/products_list.templ`, `pages/product_form.templ`,
  `pages/product_detail.templ`.
- `partials/product_row.templ`, `partials/variant_table.templ`.
- Variant price inputs accept whole/decimal ARS and are stored as centavos.

**Deferred (still marked _planned_ in `docs/routes.md`):** product media
upload/delete and collections assignment.

#### #9 Staff / whitelist — `staff_handler.go`, `pages/staff.templ`

- List whitelist, invite (email lowercased, role validated), role change,
  disable/enable (instant offboarding). Duplicate email → redirect with error.
- Audit-log UI not included: no `audit_logs` writes yet (belongs with the
  inventory/finance phases that must log mutations).

#### #10 Ops

- `internal/db/backup.go`: `Backup` (`VACUUM INTO`, timestamped, refuses to
  overwrite), `RestoreDrill` (opens the newest backup with `query_only(1)` and
  runs `PRAGMA integrity_check`).
- `ops_handler.go`: `GET /healthz`, `POST /admin/backup`,
  `POST /admin/restore-drill`.
- `Dockerfile`: Go 1.26 builder, copies `go.sum`, `CGO_ENABLED=0` (pure-Go
  SQLite), runtime image unchanged.
- `.gitignore`: ignore `/data/backups/`.

#### Verification

- `go build ./...` — clean
- `go vet ./...` — clean
- `templ generate` — clean
- `gofmt` — clean on handwritten files
- Live server smoke test (temp DB, `APP_ENV=development`):
  - `/healthz` 200; `/login` 200; `/dashboard` renders counts.
  - Dev seed present; product create 303 → detail; money renders `12.000,00`.
  - Variant create with `retail=12000.50` stores/renders `12.000,50`.
  - HTMX `HX-Request` variant-table fragment returns rows.
  - `POST` without CSRF token → 403; with token → 303.
  - Product archive/restore and archived filter work; unknown id → 404.
  - Staff invite (email lowercased), role change, disable all succeed.
  - `/admin/backup` writes a file; `/admin/restore-drill` returns `ok`.

#### Run

```bash
export INITIAL_OWNER_EMAIL=owner@example.com
make dev              # or: go run ./cmd/server
```

`INITIAL_OWNER_EMAIL` is required only on a fresh database.

#### Seams left for the Pro tasks

- **#3 auth** — was pass-through stubs at the end of this session (resolved in
  the next session — see below).
- **#6 inventory** — restock/adjustment writes (`cost_layers`, `cost_history`,
  stock-movements ledger UI) and low-stock widget.
- **#7 sales/returns** — POS, payments, atomic stock writes, returns.
- **#8 analytics** — P&L, best sellers, deadstock, chart partials; dashboard
  stays a count-only landing page until then.
- Unresolved `docs/schema.md` §9 decisions (store-credit ledger, stock
  lifecycle, discount rounding) still gate #7.

### 2026-09-25 (session 2) — Auth core implemented (#3)

Implemented the Pro-class auth milestone. The Flash scope from the previous
session is unchanged.

#### Added

- `internal/auth/user.go` — `User` type + context helpers.
- `internal/auth/session.go` — `Manager` wrapping SCS (`scs.SessionManager`)
  with a custom `sessionStore` persisting to the app's `sessions` table via sqlc
  (`GetSession`/`CreateSession`/`DeleteSession`). 24h lifetime; `varels_session`
  cookie (HttpOnly, SameSite=Lax, Secure outside development).
- `internal/auth/middleware.go` — `(*Manager).RequireLogin` (reloads the user
  from `approved_users` on every request, giving instant offboarding) and
  `(*Manager).RequireRole(min)`.
- `internal/auth/oauth.go` — gothic cookie-store setup, Google provider,
  `WithProvider`, `RandomSecret`.
- `internal/auth/password.go` — bcrypt hash/verify plus `CheckDummy` to
  timing-equalise failed logins.
- `internal/auth/ratelimit.go` — in-memory `LoginLimiter` (5 attempts / 15 min
  per IP).
- `internal/auth/whitelist.go` — `FindActiveByEmail`, the OAuth whitelist lookup.
- `pages/login.templ` — standalone login page (Google button, or the break-glass
  form at `/admin-login`).

#### Changed

- `handlers/auth_handler.go` — Google begin/callback, `/admin-login` GET/POST,
  `/login`, `/auth/denied`, `/logout`, plus login audit rows.
- `handlers/server.go` — `Sessions`, `OAuthEnabled`, `Limiter` fields.
- `handlers/helpers.go` — `currentUserID` reads the context user, so audit /
  `invited_by` / `created_by` are wired.
- `db/seed.go` — `SeedOptions`; bootstraps the Google admin and the break-glass
  local admin (`ADMIN_EMAIL` / `ADMIN_PASSWORD`, bcrypt-hashed).
- `cmd/server/main.go` — new config, auth wiring, `LoadAndSave` + method
  middleware on route groups, public auth routes.
- `.env.example` — `ADMIN_EMAIL`, `ADMIN_PASSWORD`.
- Dependencies: `markbates/goth`, `gorilla/sessions`, `alexedwards/scs/v2`,
  `golang.org/x/crypto`.

#### Decisions / notes

- Session identity lives in the SCS payload; `sessions.user_id` is left NULL
  (the generic SCS store cannot see the user at commit time). The column stays
  available for a future direct-session model.
- The break-glass account uses a different email from the Google owner (email is
  UNIQUE) and is reachable only at the unlinked `/admin-login`.
- With no Google credentials the app logs a warning and disables `/auth/google`;
  `/admin-login` is then the login path.
- An empty `SESSION_SECRET` in development generates an ephemeral secret
  (sessions do not survive a restart). Set it for anything real.

#### Verification (live server, temp DB, dev + local admin)

- Unauthenticated `/dashboard` → 303 `/login`; `/auth/google` (OAuth off) → 303
  `/login?err=oauth`.
- `/admin-login` renders with CSRF; wrong password → 401; correct → 303
  `/dashboard` and a `varels_session` cookie.
- Authenticated `/dashboard` 200 and `/staff` 200; logout 303, after which
  requests redirect to `/login`.
- Role gate: local user set to `staff` → `/staff` 403, `/dashboard` 200.
- Instant offboarding: `is_active=0` while a session is live → next request 303
  `/login`.
- Rate limit: five failures 401, sixth → 429.
- `approved_users` holds the Google admin + local admin; a login row lands in
  `audit_logs`; the `sessions` row is deleted on logout.
- `go build ./...`, `go vet ./...`, `gofmt` all clean.

#### Remaining seams

- #6 inventory, #7 sales/returns, #8 analytics remain.
- `docs/schema.md` §9 decisions still gate #7.

### 2026-09-25 (session 3) — Inventory, sales/returns, analytics (#6, #7, #8)

Implemented the remaining Pro milestones plus the OpEx entry screen that P&L
needs. Resolved the three open `docs/schema.md` §9 decisions as ADR-0019
(stock lifecycle/oversell), ADR-0020 (store-credit ledger) and ADR-0021
(discount stacking/rounding).

#### Schema / generated code

- New migration `0003_store_credit.sql` + `schema.sql` mirror:
  `store_credit_ledger` (append-only signed entries per customer) with
  `types.StoreCreditReason` and a sqlc override.
- New query `ListSellableVariants` (variant + product name) for POS/restock
  selectors; `CreateStoreCreditEntry`, `StoreCreditBalance`,
  `ListStoreCreditEntries`. `sqlc generate` re-run.

#### #6 Inventory — `restock_handler.go

- `GET /variants/{id}/restock`: restock + adjustment forms; `POST` restock
  writes a positive movement and a FIFO `cost_layer`, refreshes `cost_history`
  and the variant cost when it changed. `POST .../adjust` requires a reason code.
- `GET /restocks` and `GET /stock-movements` (filterable ledger);
  `GET /dashboard/low-stock` HTMX widget. Views: `pages/restock_form`,
  `pages/restocks_list`, `pages/stock_movements`, `partials/low_stock_widget`,
  `partials/restock_row`.

#### #7 Sales / POS / returns — `sale_handler.go`

- One atomic transaction on `POST /sales`: creates the order, items, negative
  stock movements, consumes FIFO `cost_layers` for `unit_cost_minor`, and the
  payment. Supports multi-line carts, optional new-customer creation, order
  discount, shipping, IVA-inclusive `tax_minor` (ADR-0013), and store-credit
  redemption with balance check.
- `GET /sales`, `GET /sales/new` (POS with a small vanilla-JS line editor),
  `GET /sales/{id}` receipt, `POST /sales/{id}/void` (reverse + restock at the
  original cost), `GET|POST /sales/{id}/return` (partial, per-line condition and
  reason; `store_credit` issues ledger credit). Oversell is rejected by the
  stock trigger and surfaced as `?err=stock`.
- View models in `internal/views/vm`; views `pages/sale_form`, `sales_list`,
  `sale_detail`, `return_form`.

#### #8 Analytics / deadstock / exports — `analytics_handler.go`, `deadstock_handler.go`, `export_handler.go`

- `GET /analytics` (year/month), HTMX fragments `/analytics/{best-sellers,
margin, chart, sizes}`, `/dashboard/profit-cards`; P&L = revenue − COGS − OpEx.
- `GET /deadstock` + `/deadstock/table` with trapped cash by stale window.
- `GET /export/{resource}.csv` for products, variants, sales, stock-movements
  (staff+) and margins, pl, expenses (admin+, also enforced in-handler).
- Argentina-local reporting windows via a fixed UTC−3 zone (ADR-0014).
- `expense_handler.go` + `pages/expenses.templ`: OpEx list/create so net profit
  is real.

#### Verification (live server, temp DB)

- Restock updated variant cost, wrote a `cost_layer`; low-stock and ledger pages 200.
- Sale with `qty=2` sealed a 2-unit order via FIFO (`unit_cost_minor` = layer
  cost), decremented `stock_levels`, recorded cash payment.
- Oversell (`qty=9999`) → `303 /sales/new?err=stock` with no partial write
  (transaction rolled back).
- Partial return restored stock and a cost layer at the original cost, set the
  order to `partially_returned` / payment `partially_refunded`.
- Store credit: issue 12.000 on a `store_credit` return, redeem 5.000 on a later
  sale, balance 7.000; over-spend → `?err=credit_balance`.
- Analytics / deadstock / all HTMX fragments / profit cards 200; analytics shows
  revenue, COGS, gross, OpEx, net.
- CSV exports return headers + rows; unauthenticated `/analytics` → 303 `/login`.
- Added OpEx (rent) → `incurred_at` stored as ART-midnight UTC; `pl.csv` shows
  revenue 4.100.000, COGS 1.100.000, gross 3.000.000, OpEx 2.500.000, net 500.000.
- `go build ./...`, `go vet ./...`, `gofmt` clean.

#### Still deferred (planned modules in `docs/routes.md` §3.10)

- Suppliers & PO UI (ADR-0011), stock-hold UI, product media upload,
  collections assignment, dedicated customers page (POS creates customers
  inline), audit-log page, low-stock email digest.

### 2026-09-27 — Missing routes implemented

Filled the routes that `docs/routes.md` still listed as planned or that were
specced but unwired. All use existing sqlc methods except three small additions.

#### Queries added (`make sqlc`)

- `GetProductMedia` — needed for the media-delete redirect.
- `ListOrdersByCustomer` — customer order history.
- `ListStockHolds` — global hold list joined with variant/product/location.

#### New handlers

- `variant_handler.go` — `HandleVariantForm`, `HandleVariantUpdate`
  (`GET /variants/{id}/edit`, `POST /variants/{id}`).
- `product_handler.go` — `HandleProductsSearch` (`GET /products/search`, HTMX).
- `collection_handler.go` — collections list/create/archive and
  `HandleProductCollections` (`/collections`, `POST /products/{id}/collections`).
- `media_handler.go` — `HandleMediaCreate`, `HandleMediaDelete`
  (`POST /products/{id}/media`, `POST /media/{id}/delete`).
- `customer_handler.go` — list/search, create, detail (orders + store-credit
  ledger), update (`/customers`, `/customers/{id}`).
- `audit_handler.go` — `HandleAuditLog` (`GET /audit`, filtered).
- `supplier_handler.go` — suppliers CRUD + purchase orders
  (`/suppliers`, `/purchase-orders`, add item, status, receive). Receiving runs
  in one transaction: `stock_movements` (+, `po_receipt`) + `cost_layers` +
  `ReceivePurchaseOrderItem`, then advances `draft→partial→received`.
- `hold_handler.go` — hold list/create/release (`/holds`,
  `POST /holds/{id}/release`); `ExpireStockHolds` sweeps on list.

#### Extended handlers

- `sale_handler.go` — `HandleSalesFilter` (`GET /sales/filter`, HTMX rows) and
  `HandlePaymentCreate` (`POST /sales/{id}/payments`, recomputes `payment_status`).
  Shared `listOrdersFromQuery` now parses `?from=&to=` in Argentina-local time.
- `expense_handler.go` — `HandleExpenseCategories`, `HandleExpenseCategoryCreate`.
- `staff_handler.go` — `HandleStaffResetPassword` (break-glass local only).

#### Views added

- `pages/collections.templ`, `variant_form.templ`, `customers.templ`,
  `customer_detail.templ`, `audit.templ`, `expense_categories.templ`,
  `suppliers.templ`, `purchase_orders.templ`, `purchase_order_detail.templ`,
  `holds.templ`.
- `partials/product_rows.templ`, `partials/sale_rows.templ`.
- `product_detail.templ` gained collection assignment + media add/delete;
  `sidebar.templ` gained the new sections; `staff.templ` gained password reset.

#### Verification (live server, temp DB, dev + local admin)

- All new GET pages return 200 (dashboard, products, collections, customers,
  suppliers, purchase-orders, holds, audit, expense-categories, expenses, staff,
  sales).
- Variant edit 200 → save 303; live search 200.
- Collection create → assign to product → product page reflects it.
- Media add → delete; customer create → detail (200) → update.
- PO create → add line → receive: status `received`, a `po_receipt` stock
  movement appears in the ledger.
- Hold create → release; sale create → two split payments.
- Password reset: reset local admin, old password 401, new password 303.
- `go build ./...`, `go vet ./...`, `gofmt` clean; no 500s or panics in logs.

#### Still deferred

- Daily low-stock email digest (scheduled job, not a route).
- `docs/routes.md` §6 lists an HTMX error-toast contract; responses currently use
  redirects + `?err=` query params rather than out-of-band toasts.

### 2026-09-27 — Frontend: real HTMX + Alpine.js vendored

`assets/js/htmx.min.js` was a 98-byte placeholder and `app.js` was a TODO, so no
client-side interactivity actually worked. HTMX and Alpine are browser assets,
not Go modules, so neither belongs in `go.mod`.

- Vendored real builds into `assets/js/`: HTMX 2.0.4 (`htmx.min.js`, ~50 KB) and
  Alpine.js 3.14.9 (`alpine.min.js`, ~44 KB) — loaded locally, no CDN, no npm
  (ADR-0022).
- `layouts/base.templ` now loads `htmx.min.js`, then `alpine.min.js` and
  `app.js` (both `defer`), keeping ApexCharts on CDN.
- `assets/js/app.js` implemented: `htmx:afterSwap` re-initialises Alpine on
  swapped fragments; `htmx:responseError`/`htmx:sendError` surface toasts in
  `#flash`; `$store.app.flash()` exposed on `alpine:init`.
- Docs updated: `adr.md` (ADR-0022 + index), `docs/architecture.md` (folder tree,
  new §4.6, summary table), `docs/routes.md` (asset list).

Verified: assets served with real sizes, `templ generate` + `go build` + `go vet`
clean; page load includes both scripts.

### 2026-09-27 — Frontend interactivity: HTMX + Alpine wired through

Connected the previously unused interactivity layer. Backend routes were already
in place; this wires templ views to them.

#### Flash (new)

- `internal/views/flash` — `Flash` type + request-context helpers.
- `auth.Manager.SetFlash`/`PopFlash` (SCS session); `handlers.FlashFromSession`
  middleware moves the pending flash into the request context.
- `layouts/base.templ` renders it once into `#flash`; `partials.FlashOOB`
  provides the out-of-band swap for HTMX responses.
- Handlers now set success/error flashes on product, variant, collection, media,
  customer, supplier, PO, hold, expense, staff and sale/payment/return
  mutations (replacing the silent `?err=` redirects).

#### HTMX wiring

- Live product search: `products_list.templ` → `hx-get="/products/search"`
  (`#product-rows`, keyup-debounced).
- Variant table: create/archive return the fragment + OOB flash when
  `HX-Request` is set (`renderVariantTable`); `variant_table.templ` and the
  add-variant form use `hx-post`/`hx-target="#variant-table"`.
- Analytics: chart and size-curve refresh on `#year`/`#month` change
  (`rangeFromPeriod` now falls back to `?year=&month=`); best-sellers period tabs
  already swapped.
- Deadstock: window links swap `#deadstock-table`.

#### Alpine

- `app.js` registers `Alpine.data("pos", …)`: the POS line editor (add/remove
  lines, price autofill from the variant, reactive subtotal/order-discount/
  shipping/total). `sale_form.templ` rewritten to `x-data="pos()"` +
  `<template x-for>`; submitted field names are unchanged so the handler still
  reads `variant_id`/`quantity`/`unit_price`/`line_discount` arrays. The old
  inline vanilla `<script>` is gone.

#### Verification (live server, temp DB)

- Flash renders once after `POST /collections` and is gone on the next GET.
- `/products/search?q=tee` returns `product-row-*` rows; `HX-Request` variant
  create returns `#variant-table` + `hx-swap-oob` + "Variant added.".
- `/analytics/chart|sizes?year=&month=` and `/deadstock/table?stale_days=` 200.
- `/sales/new` contains `x-data="pos()"`, `x-for="(line, index) in lines"`,
  `x-text="money(total())"`; `alpine.min.js` serves (44,758 B).
- `templ generate`, `go build`, `go vet`, `gofmt`, `node --check app.js` clean;
  no 500s/panics in the log.
