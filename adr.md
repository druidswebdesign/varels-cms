# Architecture Decision Records — varels_cms

This file records the significant architectural and data-modeling decisions for
the internal inventory CMS. It is derived from `docs/` (`functions.md`,
`schema.md`, `routes.md`, `auth.md`, `architecture.md`) while `internal/` is
still a stub and the docs are the real spec.

Format: **Status · Context · Decision · Consequences.**
Statuses: `Accepted` (binding, build against it), `Proposed` (needs sign-off).

## Index

| ID       | Decision                                                           | Status   |
| -------- | ------------------------------------------------------------------ | -------- |
| ADR-0001 | Flat pragmatic folder structure over DDD/Clean Architecture        | Accepted |
| ADR-0002 | Stack: Go + chi + templ + HTMX + Tailwind (standalone)             | Accepted |
| ADR-0003 | SQLite + sqlc + goose for the data layer                           | Accepted |
| ADR-0004 | Invite-only Google OAuth whitelist + break-glass local admin       | Accepted |
| ADR-0005 | Money stored as INTEGER in ARS, formatted centrally                | Accepted |
| ADR-0006 | Orders/order_items transaction model, not single-item sales        | Accepted |
| ADR-0007 | Distinguish gross margin from net profit (OpEx tracking)           | Accepted |
| ADR-0008 | Model Location and Sales Channel on all stock and sales            | Accepted |
| ADR-0009 | Sellable vs non-sellable stock states + adjustment reason codes    | Accepted |
| ADR-0010 | Immutable audit log + SQLite durability (WAL, backups)             | Accepted |
| ADR-0011 | Supplier / Purchase Order scope                                    | Accepted |
| ADR-0012 | FIFO cost basis (cost layers) for COGS                             | Accepted |
| ADR-0013 | IVA-inclusive ARS retail pricing                                   | Accepted |
| ADR-0014 | Argentina timezone for all reporting                               | Accepted |
| ADR-0015 | Single-instance SQLite deployment (no scaling)                     | Accepted |
| ADR-0016 | Discounts stored as integer basis points                           | Accepted |
| ADR-0017 | Opening balances via adjustment movements + DB triggers            | Accepted |
| ADR-0018 | Two login roles (`admin`, `staff`); clients/providers are entities | Accepted |
| ADR-0019 | Stock commits at order creation; oversell is impossible            | Accepted |
| ADR-0020 | Store credit is a customer ledger                                  | Accepted |
| ADR-0021 | Discount stacking and rounding                                     | Accepted |
| ADR-0022 | Add Alpine.js; vendor HTMX + Alpine as local assets                | Accepted |

---

## ADR-0001 — Flat pragmatic folder structure over DDD/Clean Architecture

**Status:** Accepted

**Context.** A DDD layout (`domain/`, `application/`, `infrastructure/`,
`interfaces/`) was considered. With sqlc + HTMX + templ it forces DTO/entity
mapping boilerplate, spreads one feature across many files, and burns AI context
on indirection.

**Decision.** Adopt a flat, cohesive structure. sqlc-generated structs are used
directly — no `internal/models`, no `internal/repository`, no `internal/services`.
Business logic and validation live in `internal/handlers`. Features touch the
"golden triangle": `internal/db/query.sql` → `internal/handlers/*.go` →
`internal/views/**/*.templ`.

**Consequences.** Handlers call `db.GetX(ctx, id)` and pass the result straight
into a view (no mapping). Fast to write and AI-friendly. Trade-off: business
logic is coupled to HTTP handlers, and sqlc structs leak into views — acceptable
for a single internal app.

## ADR-0002 — Stack: Go + chi + templ + HTMX + Tailwind (standalone)

**Status:** Accepted

**Context.** Need a clean dashboard UI without a JS build pipeline or npm.

**Decision.** Go + go-chi router, templ for type-safe compiled components, HTMX
for partial-page updates, Tailwind via the standalone CLI (zero `node_modules`).
Air for live reload; charts via a CDN script tag.

**Consequences.** Single self-contained binary plus one SQLite file; deployable to
a cheap host. No npm toolchain to maintain. Trade-offs: HTMX limits rich client
interactivity, and the Tailwind CLI must be fetched/updated manually.

## ADR-0003 — SQLite + sqlc + goose for the data layer

**Status:** Accepted

**Context.** Financial and stock records must survive schema changes without
wiping history; raw SQL strings in Go are error-prone and heavy ORMs hide SQL.

**Decision.** SQLite as the datastore, sqlc to generate type-safe Go from
hand-written SQL, and goose for incremental migrations.

**Consequences.** Compile-time-checked queries with no ORM magic. Trade-offs:
schema and queries live in files outside the generated code, so the generate step
is mandatory; SQLite's single-writer model requires WAL (see ADR-0010).

## ADR-0004 — Invite-only Google OAuth whitelist + break-glass local admin

**Status:** Accepted

**Context.** A private brand tool must not be reachable by arbitrary Google
accounts, and password auth would require reset flows, SMTP, and 2FA we don't
want to build.

**Decision.** Google OAuth against an `approved_users` whitelist table; first run
bootstraps `INITIAL_OWNER_EMAIL` as the `admin` owner when the DB is empty. A
hidden `/admin-login` password route (Goth + SCS sessions) exists as the
emergency break-glass account.

**Consequences.** Instant offboarding by deleting a whitelist row; inherits
Google 2FA for free. Trade-offs: dependence on Google availability (mitigated by
the local admin) and a manually seeded first user.

## ADR-0005 — Money stored as INTEGER in ARS, formatted centrally

**Status:** Accepted

**Context.** Floating-point money corrupts financial totals, and the business
operates in Argentinian pesos.

**Decision.** Store all money as `INTEGER` in the smallest currency unit (ARS
centavos). Never use float. Currency formatting is set globally to ARS.

**Consequences.** Exact arithmetic on prices, COGS, and profit. Trade-offs: every
input/output boundary must convert to and from integer minor units, and any
future multi-currency support changes this decision.

## ADR-0006 — Orders/order_items transaction model, not single-item sales

**Status:** Accepted

**Context.** The original spec decremented one variant per "sale". Real checkout
is one customer, several items, one payment; single-item rows cannot express
receipts, order discounts, partial returns, or per-order payment.

**Decision.** Model sales as `orders` with `order_items`. `order_items` reference
the variant; returns are handled against line items, supporting full or partial
returns.

**Consequences.** Enables receipts, discounts, partial returns, and payment
methods. Trade-offs: more tables and joins; analytics query order_items rather
than a flat sales table.

## ADR-0007 — Distinguish gross margin from net profit (OpEx tracking)

**Status:** Accepted

**Context.** The original "General Brand Profit %" was `(revenue − COGS) /
revenue`, which ignores rent, salaries, packaging, shipping-out, and payment
fees — so it overstates profit.

**Decision.** Report **gross margin** (revenue − COGS) separately from **net
profit**, and track operational expenses (OpEx). P&L = revenue − COGS = gross
profit; gross profit − OpEx = net cash profit.

**Consequences.** The dashboard tells the owner the truth. Trade-offs: requires
an expenses model and data entry, and staff-visible gross margins must be gated
by role (see ADR-0010).

## ADR-0008 — Model Location and Sales Channel on all stock and sales

**Status:** Accepted

**Context.** Stock physically sits in specific places (warehouse, storefront) and
sales arrive through distinct channels (in-store, web, wholesale, pop-up). The
original spec mentioned locations but never stored them.

**Decision.** Every unit of stock belongs to a Location; every order records a
Sales Channel and Location. Reports break out by channel and location.

**Consequences.** Enables per-channel/location analytics and inventory valuation.
Trade-offs: every stock and sales write must carry a location/channel, and
defaults must be defined for single-location operation.

## ADR-0009 — Sellable vs non-sellable stock states + adjustment reason codes

**Status:** Accepted

**Context.** Damaged, sample, gift, personal, and reserved units inflate
inventory valuation and "trapped cash" if counted as sellable. Coarse "adjust
stock" loses the reason for the change.

**Decision.** Separate stock into Sellable and Non-Sellable (damaged, sample,
gift, reserved). Manual adjustments require a strict reason-code taxonomy
(e.g. damaged, PR/gift, shrinkage/stolen).

**Consequences.** Accurate valuation and auditability; supports reservations/holds
and pre-order allocation. Trade-offs: more states to manage in UI and queries.

## ADR-0010 — Immutable audit log + SQLite durability (WAL, backups)

**Status:** Accepted

**Context.** The system holds the financial truth of the business. Deletes and
non-durable SQLite put sales and stock history at risk.

**Decision.** Financial and stock records are append-only: mistakes are corrected
via an adjustment row, never by deleting history. Enable SQLite WAL mode,
schedule automated nightly backups, support a manual restore drill, and log "who
changed what, when, and why". Financial fields are role-gated.

**Consequences.** Recoverable, auditable history and safe concurrent reads/writes.
Trade-offs: no hard deletes (rows are soft-archived), storage grows monotonically,
and backup/restore requires ops discipline.

## ADR-0011 — Supplier / Purchase Order scope

**Status:** Accepted

**Context.** `functions.md` §2 adds suppliers, purchase orders, lead times,
and an "on-order" quantity. SQLite's `ALTER TABLE` is limited, so adding these
tables later is more painful than creating them up front.

**Decision.** Create `suppliers`, `purchase_orders`, and `purchase_order_items`
in `0001_init` (schema present in v1), but **defer the UI and Go handler code**
until Phase 3.

**Consequences.** Tables are ready without a migration rewrite; "on-order"
quantity and lead-time-aware low-stock digests become reachable. Trade-off:
unused tables exist before their feature ships, so `schema.sql` and migrations
must stay in sync.

## ADR-0012 — FIFO cost basis (cost layers) for COGS

**Status:** Accepted

**Context.** Cost changes over time (factory price changes, PO receipts), and
COGS must be deterministic. Weighted average is awkward to combine with returns
and manual adjustments in SQLite; a pure `cost_history` price log cannot express
"how much of each cost layer is still on hand."

**Decision.** Use **FIFO**. Introduce a `cost_layers` table
(`variant_id`, `received_at`, `qty_received`, `qty_remaining`, `unit_cost_minor`,
`source_ref`). Sales consume the oldest layer with `qty_remaining > 0`;
`order_items.unit_cost_minor` records the consumed layer cost at sale time.
Returns re-enter at their original layer cost.

**Consequences.** Reproducible COGS and stable historical margin. Trade-offs:
extra table and ledger maintenance; a sale can span multiple layers, so
`order_items` may need one row per consumed layer (or an internal consumption
detail table).

## ADR-0013 — IVA-inclusive ARS retail pricing

**Status:** Accepted

**Context.** In Argentina, retail prices to _Consumidor Final_ are almost always
IVA-inclusive (tax in the sticker price). Storing tax-exclusive prices and adding
tax at checkout creates pervasive margin confusion.

**Decision.** All `retail_price_minor` values are **IVA-inclusive (21%)**.
`orders.tax_minor` is display/receipt-only and computed at checkout as
`total − (total / 1.21)`. It is never added on top of the price.

**Consequences.** Pricing and margins match what the customer pays. Trade-offs:
wholesale/invoice-grade reporting needs the IVA split derived, not stored;
`tax_minor` is informational and must not be summed into revenue.

## ADR-0014 — Argentina timezone for all reporting

**Status:** Accepted

**Context.** Timestamps are stored as UTC, but "sales this month" and
business-day boundaries are Argentina local time (UTC−3). Grouping UTC
timestamps by month silently misfiles late-evening sales.

**Decision.** Store all timestamps in **UTC** (ISO-8601). All business grouping,
filtering, and display use **`America/Argentina/Buenos_Aires` (UTC−3)**, applied
in the query/view layer. Day/month/year buckets are Argentina-local.

**Consequences.** Correct daily/monthly analytics and receipts. Trade-offs:
queries must convert (`datetime(ts, '-3 hours')` or `time.LoadLocation` in Go);
never assume naive `strftime` on a UTC column equals a local bucket.

## ADR-0015 — Single-instance SQLite deployment (no scaling)

**Status:** Accepted

**Context.** SQLite with WAL is excellent for one process on one node and breaks
with multiple writers/replicas.

**Decision.** Run **exactly one** application instance against one SQLite file.
No horizontal scaling, no multiple replicas. `data/` lives on a persistent volume;
WAL is enabled with `busy_timeout`.

**Consequences.** Simple, fast, cheap deployment (Hetzner/Fly single node).
Trade-off: no HA and no rolling deploys — an explicit non-goal for this internal
tool; scaling would require migrating to a client/server DB.

## ADR-0016 — Discounts stored as integer basis points

**Status:** Accepted

**Context.** Percent discounts need an exact representation to keep margin math
in integers (ADR-0005).

**Decision.** `discounts.type='percent'` stores `value` as **integer basis
points** (`2000` = 20.00%; `10000` = 100%). The view layer divides by 10000.
`type='amount'` stores `value` as ARS `_minor`.

**Consequences.** No float in discount math. Trade-off: every display/edit path
must convert; `value` meaning depends on `type`.

## ADR-0017 — Opening balances via adjustment movements + DB triggers

**Status:** Accepted

**Context.** Launching with existing physical stock needs an opening entry, and
`stock_movements` (ledger) plus `stock_levels` (current) must never desync.

**Decision.** Seed a `reason_code` `opening_balance`; initial stock is inserted as
`stock_movements` rows (`ref_type='adjustment'`, `reason_code='opening_balance'`).
Add SQLite `AFTER INSERT` triggers on `stock_movements` that `UPSERT`
`stock_levels`, so the ledger is the single writer of current stock.

**Consequences.** Go code stays simpler and ledger/current stock cannot diverge.
Trade-offs: triggers add hidden write behaviour and must be covered by tests;
`stock_levels.quantity` can no longer be written directly by handlers.

## ADR-0018 — Two login roles (`admin`, `staff`); clients/providers are entities

**Status:** Accepted

**Context.** The earlier drafts proposed a five-role ladder
(`staff` < `warehouse` < `manager` < `admin` < `master_admin`). The business only
has an owner and general staff who log into the back office; "clients" and
"providers" describe commercial counterparties, not system users.

**Decision.** `approved_users.role` allows exactly two values: `admin` and
`staff`. `admin` sees financials (buy price, margin, net profit, OpEx, supplier
terms, financial exports); `staff` does not. Clients are modeled as `customers`
and providers as `suppliers`; neither authenticates and neither appears in
`approved_users`.

**Consequences.** Simple, unambiguous RBAC and no client/provider portal to
build, consistent with the "internal back office" non-goal. Trade-offs: any
future need for client- or supplier-facing self-service requires a new table and
a new auth path; the finer `warehouse`/`manager` separation is dropped, so
financial gating is binary.

## ADR-0019 — Stock commits at order creation; oversell is impossible

**Status:** Accepted

**Context.** `docs/schema.md` §9 left open when stock commits (order create vs
payment vs fulfilment), how `stock_holds` transitions, and the oversell policy.

**Decision.** In v1, a sale (`POST /sales`) commits stock at order creation:
inside one SQLite transaction it writes negative `stock_movements` (sellable,
`ref_type='order'`) for each line. The POS is immediate fulfilment, so there is
no separate pick step and `stock_holds` is not used by POS. The table remains
for a future web cart. Oversell is prevented by the `stock_levels.quantity`
`CHECK (quantity >= 0)` and the movement trigger: a movement that would drive a
level negative aborts the transaction, so no negative stock and no partial order
can exist. `stock_holds` rows (when used) transition
`active → consumed | released | expired`.

**Consequences.** POS sales are atomic and stock truth is preserved by
construction. Trade-offs: a cart cannot reserve stock in v1, so a future web
checkout must add hold/commit logic on top of this path.

## ADR-0020 — Store credit is a customer ledger

**Status:** Accepted

**Context.** `returns.resolution` and `payments.method` allow `store_credit`, but
there was no balance model (docs/schema.md §9).

**Decision.** Add `store_credit_ledger`: an append-only table of signed
`delta_minor` entries per `customers.id`, with `reason` `issued` or `redeemed`,
and optional `order_id` / `return_id`. A return resolved as `store_credit`
writes a positive entry; paying with `store_credit` writes a negative entry.
Balance = `SUM(delta_minor)` for the customer. Store credit therefore requires a
customer on the order/return.

**Consequences.** Balances are reproducible and auditable from the ledger, and
cannot be spent twice (redemption checks the balance inside the same
transaction). Trade-offs: anonymous sales cannot use store credit.

## ADR-0021 — Discount stacking and rounding

**Status:** Accepted

**Context.** `docs/schema.md` §9 left open line vs order discounts and the
rounding step.

**Decision.** Apply line-level discounts first, then one order-level discount.
`order_items.discount_minor` is an absolute minor amount per line; the order
discount is `orders.discount_minor`. Percent discounts are stored as basis
points (ADR-0016) and converted to whole centavos with half-up rounding at each
application step. `orders.subtotal_minor = Σ line_total_minor` (already net of
line discounts); `orders.total_minor = subtotal_minor − discount_minor +
shipping_minor`. `tax_minor` remains display-only IVA-inclusive:
`total − round(total / 1.21)` (ADR-0013). The `discounts` table is not applied
automatically in POS v1; staff enter the discount manually, and the table drives
catalog/collection markdowns later.

**Consequences.** One deterministic rounding rule and an order of operations
that never produces negative line totals. Trade-offs: automatic stacking of
multiple `discounts` rows is deferred.

## ADR-0022 — Add Alpine.js; vendor HTMX + Alpine as local assets

**Status:** Accepted

**Context.** ADR-0002 chose HTMX with "no Alpine". In practice the UI needs light
client-side state (dropdowns, multi-line POS editor, toggles, toasts) that is
awkward in pure HTMX, and the original `assets/js/htmx.min.js` shipped as a
placeholder stub, so no front-end scripting actually worked.

**Decision.** Keep HTMX for partial-page updates and add **Alpine.js 3.x** for
local component state. Both are browser assets, not Go dependencies, so they do
**not** appear in `go.mod`. Vendor them into `assets/js/` (`htmx.min.js`,
`alpine.min.js`) and load them locally from `layouts/base.templ` instead of a
CDN, matching the self-hosted, zero-npm approach of ADR-0002. A small
`assets/js/app.js` wires the two together: `htmx:afterSwap` re-initialises Alpine
on swapped fragments, and HTMX request failures surface as toasts in `#flash`.

**Consequences.** Richer interactivity without a JS build pipeline. Trade-offs:
two vendored libraries to update manually, and a CSP would need to allow the
Alpine inline-evaluation model (the default build is not CSP-safe). ApexCharts
remains a CDN script (ADR-0002).
