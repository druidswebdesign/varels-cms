# Data Model & Schema Specification — varels_cms

This is the authoritative data model derived from the project docs. It defines the
SQLite schema that `internal/db/schema.sql` (sqlc input) and
`internal/db/migrations/*.sql` (goose) must implement.

**Source precedence** when docs disagree:

1. `docs/functions.md` — the functional spec.
2. `docs/auth.md`, `docs/architecture.md` — auth, stack, layout.
3. `adr.md` — binding decisions; ADR numbers are referenced inline as `(ADR-00xx)`.

> Note: the stub comments in `internal/db/schema.sql`, `internal/db/query.sql`, and
> `internal/db/migrations/0001_init.sql` still say `sales, sale_items,
restock_logs`. That naming is superseded — this document uses `orders` /
> `order_items` (ADR-0006).

---

## 1. Conventions

| Rule                 | Detail                                                                                                                            | Ref      |
| -------------------- | --------------------------------------------------------------------------------------------------------------------------------- | -------- |
| Money                | Stored as `INTEGER`, smallest ARS unit (centavos). Never `REAL`. Suffix: `_minor`.                                                | ADR-0005 |
| Currency             | Single currency, ARS. Format centrally in the view layer.                                                                         | ADR-0005 |
| Timestamps           | `TEXT` ISO-8601 UTC (e.g. `2026-09-16T21:00:00Z`); reported in `America/Argentina/Buenos_Aires` (UTC−3). SQLite has no date type. | ADR-0014 |
| IDs                  | `INTEGER PRIMARY KEY` (rowid alias) unless a natural key is noted.                                                                | —        |
| Soft delete          | Catalog rows use `is_archived`; financial/history rows are never deleted.                                                         | ADR-0010 |
| Immutability         | `stock_movements`, `audit_logs`, `order_items`, `payments`, `return_items` are append-only. Corrections are new rows.             | ADR-0010 |
| Enums                | `TEXT` + `CHECK (col IN (...))`. Allowed values are listed in §4.                                                                 | ADR-0009 |
| Timestamps of change | Every mutable table carries `created_at` and `updated_at`.                                                                        | —        |
| FKs                  | `PRAGMA foreign_keys = ON` per connection.                                                                                        | —        |
| Concurrency          | `journal_mode = WAL`, `busy_timeout = 5000`, `synchronous = NORMAL`.                                                              | ADR-0010 |
| Deployment           | Single app instance, one SQLite file on a persistent volume; no horizontal scaling.                                               | ADR-0015 |
| Cost basis           | FIFO via `cost_layers`; COGS consumes the oldest remaining layer.                                                                 | ADR-0012 |
| Tax                  | `retail_price_minor` is IVA-inclusive (21%); `tax_minor` is display-only.                                                         | ADR-0013 |
| Discounts            | `discounts.value` is integer basis points for `percent` (2000 = 20.00%).                                                          | ADR-0016 |
| Current stock        | Written only by triggers on `stock_movements`; never updated directly.                                                            | ADR-0017 |

Naming: tables plural snake_case; FKs `<table_singular>_id`; money `*_minor`;
"current" values live on the entity, history lives in ledger tables.

---

## 2. Relationship overview

```
approved_users ──< sessions
products ──< product_variants ──< stock_levels >── locations
   │              │  │             stock_movements ──< reason_codes
   │              │  └──< cost_history
   │              ├──< price_tiers
   ├──< product_media
   ├──< product_collections >── collections
   └── categories

suppliers ──< purchase_orders ──< purchase_order_items >── product_variants   (ADR-0011)
                   └── locations

customers ──< orders ──< order_items >── product_variants
                │  ├──< payments
                │  └──< returns ──< return_items >── order_items
                ├── sales_channels
                └── locations

stock_holds ──< product_variants / locations
expense_categories ──< expenses
settings · audit_logs   (standalone)
```

---

## 3. Tables

### 3.1 Auth (goose migration `0002_users`)

**`approved_users`** — Google OAuth whitelist plus break-glass local admin (ADR-0004).

| Column                  | Type    | Constraints                  | Notes                             |
| ----------------------- | ------- | ---------------------------- | --------------------------------- |
| id                      | INTEGER | PK                           |                                   |
| email                   | TEXT    | NOT NULL, UNIQUE, lowercased | matched against OAuth email       |
| display_name            | TEXT    |                              | from Google profile               |
| role                    | TEXT    | NOT NULL, CHECK              | `admin`, `staff` (ADR-0018)       |
| provider                | TEXT    | NOT NULL, DEFAULT `'google'` | `google` or `local` (break-glass) |
| password_hash           | TEXT    | NULL                         | only set for `provider = 'local'` |
| is_active               | INTEGER | NOT NULL, DEFAULT 1          | 0 = revoke access                 |
| invited_by              | INTEGER | FK `approved_users(id)`      | NULL for bootstrap owner          |
| last_login_at           | TEXT    |                              |                                   |
| created_at / updated_at | TEXT    | NOT NULL                     |                                   |

Bootstrap: on first run, if the table is empty, insert `INITIAL_OWNER_EMAIL` as
`admin` (`seed.go`).

**`sessions`** — SCS session store.

| Column     | Type    | Constraints             | Notes                      |
| ---------- | ------- | ----------------------- | -------------------------- |
| token      | TEXT    | PK                      | SCS-managed                |
| user_id    | INTEGER | FK `approved_users(id)` |                            |
| data       | BLOB    |                         | serialized session payload |
| expires_at | TEXT    | NOT NULL                |                            |
| created_at | TEXT    | NOT NULL                |                            |

### 3.2 Catalog

**`categories`**

| Column     | Type    | Constraints               |
| ---------- | ------- | ------------------------- |
| id         | INTEGER | PK                        |
| name       | TEXT    | NOT NULL, UNIQUE          |
| slug       | TEXT    | NOT NULL, UNIQUE          |
| parent_id  | INTEGER | FK `categories(id)`, NULL |
| sort_order | INTEGER | NOT NULL, DEFAULT 0       |

**`collections`** — drops / seasons / essentials (`functions.md` §1).

| Column      | Type    | Constraints                                  |
| ----------- | ------- | -------------------------------------------- |
| id          | INTEGER | PK                                           |
| name        | TEXT    | NOT NULL, UNIQUE                             |
| slug        | TEXT    | NOT NULL, UNIQUE                             |
| kind        | TEXT    | NOT NULL, CHECK `drop`,`season`,`essentials` |
| season      | TEXT    | e.g. `Winter '26`                            |
| launch_date | TEXT    |                                              |
| is_archived | INTEGER | NOT NULL, DEFAULT 0                          |

**`product_collections`** — many-to-many. PK `(product_id, collection_id)`.

**`products`**

| Column                  | Type    | Constraints               | Notes                     |
| ----------------------- | ------- | ------------------------- | ------------------------- |
| id                      | INTEGER | PK                        |                           |
| name                    | TEXT    | NOT NULL                  |                           |
| slug                    | TEXT    | NOT NULL, UNIQUE          |                           |
| description             | TEXT    |                           |                           |
| category_id             | INTEGER | FK `categories(id)`, NULL |                           |
| is_archived             | INTEGER | NOT NULL, DEFAULT 0       | archive instead of delete |
| archived_at             | TEXT    |                           |                           |
| created_at / updated_at | TEXT    | NOT NULL                  |                           |

**`product_media`** — photos, galleries, variant images, size charts (`functions.md` §1).

| Column     | Type    | Constraints                                  | Notes                                   |
| ---------- | ------- | -------------------------------------------- | --------------------------------------- |
| id         | INTEGER | PK                                           |                                         |
| product_id | INTEGER | NOT NULL, FK `products(id)`                  |                                         |
| variant_id | INTEGER | FK `product_variants(id)`, NULL              | NULL = product-level                    |
| media_type | TEXT    | NOT NULL, CHECK `image`,`size_chart`,`video` |                                         |
| url        | TEXT    | NOT NULL                                     | served from `assets/` or object storage |
| alt_text   | TEXT    |                                              |                                         |
| sort_order | INTEGER | NOT NULL, DEFAULT 0                          |                                         |
| is_primary | INTEGER | NOT NULL, DEFAULT 0                          |                                         |

**`product_variants`** — the sellable unit; one row per size/color/SKU.

| Column                  | Type    | Constraints                 | Notes                    |
| ----------------------- | ------- | --------------------------- | ------------------------ |
| id                      | INTEGER | PK                          |                          |
| product_id              | INTEGER | NOT NULL, FK `products(id)` |                          |
| sku                     | TEXT    | NOT NULL, UNIQUE            |                          |
| size                    | TEXT    |                             | `S`,`M`,`L`…             |
| color                   | TEXT    |                             |                          |
| barcode                 | TEXT    | NULL                        |                          |
| cost_minor              | INTEGER | NOT NULL, CHECK >= 0        | current buy price / COGS |
| retail_price_minor      | INTEGER | NOT NULL, CHECK >= 0        | standard price           |
| wholesale_price_minor   | INTEGER | NULL, CHECK >= 0            | B2B tier default         |
| low_stock_threshold     | INTEGER | NOT NULL, DEFAULT 0         | alert level              |
| is_archived             | INTEGER | NOT NULL, DEFAULT 0         |                          |
| created_at / updated_at | TEXT    | NOT NULL                    |                          |

**`cost_history`** — cost changes over time; keeps historical COGS intact.

| Column         | Type    | Constraints                           |
| -------------- | ------- | ------------------------------------- |
| id             | INTEGER | PK                                    |
| variant_id     | INTEGER | NOT NULL, FK `product_variants(id)`   |
| cost_minor     | INTEGER | NOT NULL, CHECK >= 0                  |
| effective_from | TEXT    | NOT NULL                              |
| source         | TEXT    | NOT NULL, CHECK `manual`,`po_receipt` |
| note           | TEXT    |                                       |
| created_by     | INTEGER | FK `approved_users(id)`               |
| created_at     | TEXT    | NOT NULL                              |

**`cost_layers`** — FIFO layers (ADR-0012). One row per receipt of stock at a cost.

| Column          | Type    | Constraints                         | Notes                                              |
| --------------- | ------- | ----------------------------------- | -------------------------------------------------- |
| id              | INTEGER | PK                                  |                                                    |
| variant_id      | INTEGER | NOT NULL, FK `product_variants(id)` |                                                    |
| received_at     | TEXT    | NOT NULL                            | FIFO order key                                     |
| qty_received    | INTEGER | NOT NULL, CHECK > 0                 |                                                    |
| qty_remaining   | INTEGER | NOT NULL, CHECK >= 0                | decremented as units sell                          |
| unit_cost_minor | INTEGER | NOT NULL, CHECK >= 0                |                                                    |
| source_ref      | TEXT    |                                     | `po_receipt` / `adjustment` / `opening_balance` id |
| created_at      | TEXT    | NOT NULL                            |                                                    |

**`price_tiers`** — wholesale / bulk pricing.

| Column      | Type    | Constraints                         |
| ----------- | ------- | ----------------------------------- |
| id          | INTEGER | PK                                  |
| variant_id  | INTEGER | NOT NULL, FK `product_variants(id)` |
| min_qty     | INTEGER | NOT NULL, CHECK > 0                 |
| price_minor | INTEGER | NOT NULL, CHECK >= 0                |
| label       | TEXT    | e.g. `Wholesale 50+`                |

**`discounts`** — temporary/permanent markdowns (`functions.md` §1).

| Column     | Type    | Constraints                                              | Notes                                   |
| ---------- | ------- | -------------------------------------------------------- | --------------------------------------- |
| id         | INTEGER | PK                                                       |                                         |
| name       | TEXT    | NOT NULL                                                 |                                         |
| scope      | TEXT    | NOT NULL, CHECK `product`,`variant`,`collection`,`order` |                                         |
| scope_id   | INTEGER |                                                          | id in the target table                  |
| type       | TEXT    | NOT NULL, CHECK `percent`,`amount`                       |                                         |
| value      | INTEGER | NOT NULL, CHECK > 0                                      | percent ×100 (basis points) or `_minor` |
| starts_at  | TEXT    |                                                          | NULL = immediate                        |
| ends_at    | TEXT    |                                                          | NULL = open-ended                       |
| is_active  | INTEGER | NOT NULL, DEFAULT 1                                      |                                         |
| created_by | INTEGER | FK `approved_users(id)`                                  |                                         |
| created_at | TEXT    | NOT NULL                                                 |                                         |

### 3.3 Locations, channels & inventory (ADR-0008, ADR-0009)

**`locations`**

| Column    | Type    | Constraints                              |
| --------- | ------- | ---------------------------------------- |
| id        | INTEGER | PK                                       |
| name      | TEXT    | NOT NULL, UNIQUE                         |
| kind      | TEXT    | NOT NULL, CHECK `warehouse`,`storefront` |
| address   | TEXT    |                                          |
| is_active | INTEGER | NOT NULL, DEFAULT 1                      |

**`sales_channels`** — seed: `In-Store`, `Web`, `Wholesale`, `Pop-up`.

| Column    | Type    | Constraints         |
| --------- | ------- | ------------------- |
| id        | INTEGER | PK                  |
| name      | TEXT    | NOT NULL, UNIQUE    |
| is_active | INTEGER | NOT NULL, DEFAULT 1 |

**`stock_levels`** — current on-hand quantity per variant / location / state.
State is part of the key so sellable and non-sellable quantities coexist.

| Column      | Type    | Constraints                         |
| ----------- | ------- | ----------------------------------- |
| id          | INTEGER | PK                                  |
| variant_id  | INTEGER | NOT NULL, FK `product_variants(id)` |
| location_id | INTEGER | NOT NULL, FK `locations(id)`        |
| state       | TEXT    | NOT NULL, CHECK (see §4)            |
| quantity    | INTEGER | NOT NULL, DEFAULT 0, CHECK >= 0     |
| updated_at  | TEXT    | NOT NULL                            |

`UNIQUE (variant_id, location_id, state)`.

**`stock_movements`** — append-only ledger; the source of truth for history.

| Column         | Type    | Constraints                                                        | Notes                    |
| -------------- | ------- | ------------------------------------------------------------------ | ------------------------ |
| id             | INTEGER | PK                                                                 |                          |
| variant_id     | INTEGER | NOT NULL, FK `product_variants(id)`                                |                          |
| location_id    | INTEGER | NOT NULL, FK `locations(id)`                                       |                          |
| state          | TEXT    | NOT NULL, CHECK (§4)                                               |                          |
| quantity_delta | INTEGER | NOT NULL                                                           | signed; +in / −out       |
| reason_code    | TEXT    | FK `reason_codes(code)`                                            | required for adjustments |
| ref_type       | TEXT    | CHECK `order`,`return`,`adjustment`,`po_receipt`,`transfer`,`hold` |                          |
| ref_id         | INTEGER |                                                                    | id in `ref_type` table   |
| note           | TEXT    |                                                                    |                          |
| created_by     | INTEGER | FK `approved_users(id)`                                            |                          |
| created_at     | TEXT    | NOT NULL                                                           |                          |

**`reason_codes`** — taxonomy for adjustments and returns (`functions.md` §2).

| Column    | Type    | Constraints                                 |
| --------- | ------- | ------------------------------------------- |
| code      | TEXT    | PK                                          |
| label     | TEXT    | NOT NULL                                    |
| kind      | TEXT    | NOT NULL, CHECK `stock_adjustment`,`return` |
| is_active | INTEGER | NOT NULL, DEFAULT 1                         |

Seed examples — stock: `damaged`, `pr_gift`, `shrinkage_stolen`, `sample`,
`personal`, `recount`, `opening_balance` (ADR-0017). Return: `wrong_size`,
`defective`, `changed_mind`.

**`stock_holds`** — reservations/pre-orders to prevent overselling on drops.

| Column      | Type    | Constraints                                              |
| ----------- | ------- | -------------------------------------------------------- |
| id          | INTEGER | PK                                                       |
| variant_id  | INTEGER | NOT NULL, FK `product_variants(id)`                      |
| location_id | INTEGER | NOT NULL, FK `locations(id)`                             |
| quantity    | INTEGER | NOT NULL, CHECK > 0                                      |
| source      | TEXT    | NOT NULL, CHECK `cart`,`preorder`,`manual`               |
| ref_id      | TEXT    |                                                          | cart/order identifier |
| status      | TEXT    | NOT NULL, CHECK `active`,`released`,`consumed`,`expired` |
| expires_at  | TEXT    |                                                          |
| created_at  | TEXT    | NOT NULL                                                 |

### 3.4 Suppliers & procurement (ADR-0011)

In scope for `0001_init`; UI and Go handler code are deferred to Phase 3. These
`suppliers` are the business "providers".

**`suppliers`**: id, name (NOT NULL), contact_name, email, phone,
`lead_time_days` INTEGER, `terms` TEXT, notes, is_active, created_at.

**`purchase_orders`**: id, `supplier_id` FK, `location_id` FK,
status CHECK `draft`,`ordered`,`partial`,`received`,`cancelled`,
ordered_at, expected_at, received_at, notes, `created_by`, created_at.

**`purchase_order_items`**: id, `purchase_order_id` FK, `variant_id` FK,
`quantity_ordered` INTEGER, `quantity_received` INTEGER DEFAULT 0,
`unit_cost_minor` INTEGER, `line_total_minor` INTEGER. UNIQUE `(po, variant)`.

On-order quantity = `Σ (quantity_ordered − quantity_received)` over open POs.
Receiving a PO creates `stock_movements` (`ref_type = 'po_receipt'`), a
`cost_layers` row at the incoming cost (ADR-0012), and a `cost_history` row when
the incoming cost differs.

### 3.5 Customers

**`customers`** — light profile; these are the business "clients".

| Column                  | Type    | Constraints |
| ----------------------- | ------- | ----------- |
| id                      | INTEGER | PK          |
| name                    | TEXT    | NOT NULL    |
| phone                   | TEXT    |             |
| email                   | TEXT    |             |
| instagram               | TEXT    |             |
| notes                   | TEXT    |             |
| created_at / updated_at | TEXT    | NOT NULL    |

### 3.6 Orders & sales (ADR-0006)

**`orders`** — one customer, several items, one payment.

| Column                  | Type    | Constraints                       | Notes                                |
| ----------------------- | ------- | --------------------------------- | ------------------------------------ |
| id                      | INTEGER | PK                                |                                      |
| order_number            | TEXT    | NOT NULL, UNIQUE                  | human-facing receipt id              |
| customer_id             | INTEGER | FK `customers(id)`, NULL          | anonymous sale allowed               |
| channel_id              | INTEGER | NOT NULL, FK `sales_channels(id)` | ADR-0008                             |
| location_id             | INTEGER | NOT NULL, FK `locations(id)`      | ADR-0008                             |
| status                  | TEXT    | NOT NULL, CHECK §4                |                                      |
| subtotal_minor          | INTEGER | NOT NULL, DEFAULT 0               | Σ line totals before order discount  |
| discount_minor          | INTEGER | NOT NULL, DEFAULT 0               | order-level discount                 |
| tax_minor               | INTEGER | NOT NULL, DEFAULT 0               |                                      |
| shipping_minor          | INTEGER | NOT NULL, DEFAULT 0               |                                      |
| total_minor             | INTEGER | NOT NULL, DEFAULT 0               | subtotal − discount + tax + shipping |
| payment_status          | TEXT    | NOT NULL, CHECK §4                |                                      |
| placed_at               | TEXT    | NOT NULL                          |                                      |
| note                    | TEXT    |                                   |                                      |
| created_by              | INTEGER | FK `approved_users(id)`           |                                      |
| created_at / updated_at | TEXT    | NOT NULL                          |                                      |

**`order_items`** — append-only; snapshots price **and cost** for stable COGS.

| Column           | Type    | Constraints                         | Notes                         |
| ---------------- | ------- | ----------------------------------- | ----------------------------- |
| id               | INTEGER | PK                                  |                               |
| order_id         | INTEGER | NOT NULL, FK `orders(id)`           |                               |
| variant_id       | INTEGER | NOT NULL, FK `product_variants(id)` |                               |
| quantity         | INTEGER | NOT NULL, CHECK > 0                 |                               |
| unit_price_minor | INTEGER | NOT NULL, CHECK >= 0                | price actually charged        |
| unit_cost_minor  | INTEGER | NOT NULL, CHECK >= 0                | COGS snapshot at sale time    |
| discount_minor   | INTEGER | NOT NULL, DEFAULT 0                 | line-level discount           |
| line_total_minor | INTEGER | NOT NULL                            | `unit_price × qty − discount` |

**`payments`** — supports per-order and split tender, refunds.

| Column       | Type    | Constraints                                                           |
| ------------ | ------- | --------------------------------------------------------------------- |
| id           | INTEGER | PK                                                                    |
| order_id     | INTEGER | NOT NULL, FK `orders(id)`                                             |
| method       | TEXT    | NOT NULL, CHECK `cash`,`card`,`mercadopago`,`transfer`,`store_credit` |
| amount_minor | INTEGER | NOT NULL                                                              |
| status       | TEXT    | NOT NULL, CHECK `captured`,`refunded`,`partial_refund`                |
| reference    | TEXT    | gateway/txn reference                                                 |
| paid_at      | TEXT    | NOT NULL                                                              |
| created_by   | INTEGER | FK `approved_users(id)`                                               |

### 3.7 Returns & exchanges (ADR-0006, ADR-0009)

**`returns`**

| Column             | Type    | Constraints                                        |
| ------------------ | ------- | -------------------------------------------------- |
| id                 | INTEGER | PK                                                 |
| order_id           | INTEGER | NOT NULL, FK `orders(id)`                          |
| customer_id        | INTEGER | FK `customers(id)`, NULL                           |
| resolution         | TEXT    | NOT NULL, CHECK `refund`,`store_credit`,`exchange` |
| status             | TEXT    | NOT NULL, CHECK `pending`,`completed`              |
| total_refund_minor | INTEGER | NOT NULL, DEFAULT 0                                |
| note               | TEXT    |                                                    |
| processed_at       | TEXT    |                                                    |
| created_by         | INTEGER | FK `approved_users(id)`                            |
| created_at         | TEXT    | NOT NULL                                           |

**`return_items`** — per line, supports partial returns and condition.

| Column              | Type    | Constraints                                        | Notes                    |
| ------------------- | ------- | -------------------------------------------------- | ------------------------ |
| id                  | INTEGER | PK                                                 |                          |
| return_id           | INTEGER | NOT NULL, FK `returns(id)`                         |                          |
| order_item_id       | INTEGER | NOT NULL, FK `order_items(id)`                     |                          |
| quantity            | INTEGER | NOT NULL, CHECK > 0                                | partial return allowed   |
| condition_state     | TEXT    | NOT NULL, CHECK `sellable`,`damaged`               | drives destination state |
| resolution          | TEXT    | NOT NULL, CHECK `refund`,`store_credit`,`exchange` |                          |
| refund_amount_minor | INTEGER | NOT NULL, DEFAULT 0                                |                          |
| reason_code         | TEXT    | NOT NULL, FK `reason_codes(code)`                  |                          |
| created_at          | TEXT    | NOT NULL                                           |                          |

Effect: `sellable` restocks to sellable state, `damaged` to non-sellable; both
write `stock_movements` with `ref_type = 'return'`.

### 3.8 Finance / OpEx (ADR-0007)

**`expense_categories`**: id, name (NOT NULL, UNIQUE), is_active.

**`expenses`**

| Column       | Type    | Constraints                           | Notes                                         |
| ------------ | ------- | ------------------------------------- | --------------------------------------------- |
| id           | INTEGER | PK                                    |                                               |
| category_id  | INTEGER | NOT NULL, FK `expense_categories(id)` | rent, salaries, packaging, shipping-out, fees |
| vendor       | TEXT    |                                       |                                               |
| amount_minor | INTEGER | NOT NULL, CHECK >= 0                  |                                               |
| incurred_at  | TEXT    | NOT NULL                              |                                               |
| note         | TEXT    |                                       |                                               |
| created_by   | INTEGER | FK `approved_users(id)`               |                                               |
| created_at   | TEXT    | NOT NULL                              |                                               |

### 3.9 System & audit (ADR-0010)

**`audit_logs`** — append-only; "who changed what, when, why".

| Column      | Type    | Constraints                                                |
| ----------- | ------- | ---------------------------------------------------------- |
| id          | INTEGER | PK                                                         |
| user_id     | INTEGER | FK `approved_users(id)`, NULL (system)                     |
| action      | TEXT    | NOT NULL, e.g. `create`,`update`,`adjust`,`login`,`return` |
| entity_type | TEXT    | NOT NULL, e.g. `product_variant`,`order`                   |
| entity_id   | INTEGER |                                                            |
| before_json | TEXT    |                                                            |
| after_json  | TEXT    |                                                            |
| note        | TEXT    | reason / context                                           |
| ip          | TEXT    |                                                            |
| created_at  | TEXT    | NOT NULL                                                   |

**`settings`** — key/value config (thresholds, currency display, backup schedule).

| Column     | Type    | Constraints             |
| ---------- | ------- | ----------------------- |
| key        | TEXT    | PK                      |
| value      | TEXT    | NOT NULL                |
| updated_at | TEXT    | NOT NULL                |
| updated_by | INTEGER | FK `approved_users(id)` |

---

## 4. Enumerated values (CHECK constraints)

| Domain                                           | Allowed values                                                      |
| ------------------------------------------------ | ------------------------------------------------------------------- |
| `approved_users.role`                            | `admin`, `staff` (ADR-0018)                                         |
| `approved_users.provider`                        | `google`, `local`                                                   |
| `stock_levels.state` / `stock_movements.state`   | `sellable`, `reserved`, `damaged`, `sample`, `gift`, `personal`     |
| `stock_movements.ref_type`                       | `order`, `return`, `adjustment`, `po_receipt`, `transfer`, `hold`   |
| `stock_holds.source`                             | `cart`, `preorder`, `manual`                                        |
| `stock_holds.status`                             | `active`, `released`, `consumed`, `expired`                         |
| `reason_codes.kind`                              | `stock_adjustment`, `return`                                        |
| `orders.status`                                  | `draft`, `completed`, `partially_returned`, `returned`, `cancelled` |
| `orders.payment_status`                          | `unpaid`, `paid`, `partially_refunded`, `refunded`                  |
| `payments.method`                                | `cash`, `card`, `mercadopago`, `transfer`, `store_credit`           |
| `payments.status`                                | `captured`, `refunded`, `partial_refund`                            |
| `returns.resolution` / `return_items.resolution` | `refund`, `store_credit`, `exchange`                                |
| `returns.status`                                 | `pending`, `completed`                                              |
| `return_items.condition_state`                   | `sellable`, `damaged`                                               |
| `discounts.scope`                                | `product`, `variant`, `collection`, `order`                         |
| `discounts.type`                                 | `percent`, `amount`                                                 |
| `locations.kind`                                 | `warehouse`, `storefront`                                           |
| `collections.kind`                               | `drop`, `season`, `essentials`                                      |
| `product_media.media_type`                       | `image`, `size_chart`, `video`                                      |

Non-sellable states for valuation: everything in `stock_levels.state` except
`sellable` (reserved is non-sellable for valuation but not lost cost).

---

## 5. Indexes

```
products(slug)                              product_variants(product_id)
product_variants(sku)                       product_media(product_id, sort_order)
product_collections(collection_id)          cost_history(variant_id, effective_from)
price_tiers(variant_id, min_qty)            discounts(scope, scope_id, is_active)

stock_levels(variant_id)                    stock_levels(location_id, state)
stock_movements(variant_id, created_at)     stock_movements(ref_type, ref_id)
stock_holds(variant_id, status)             stock_holds(expires_at)

orders(placed_at)                           orders(customer_id)
orders(channel_id, placed_at)               orders(location_id, placed_at)
order_items(order_id)                       order_items(variant_id)
payments(order_id)                          returns(order_id)
return_items(return_id)                     return_items(order_item_id)
expenses(incurred_at, category_id)          audit_logs(entity_type, entity_id)
audit_logs(user_id, created_at)             sessions(expires_at)
suppliers(name)                             purchase_orders(supplier_id, status)
```

---

## 6. Money & margin rules

All values `_minor` are ARS centavos. `retail_price_minor` is **IVA-inclusive
(21%)**; `orders.tax_minor` is display-only, computed as
`total − (total / 1.21)` and never added on top (ADR-0013). Derived, never stored:

| Metric                   | Formula                                                          |
| ------------------------ | ---------------------------------------------------------------- |
| Unit gross profit        | `retail_price_minor − cost_minor`                                |
| Unit gross margin %      | `unit_gross_profit / retail_price_minor × 100`                   |
| Markup %                 | `unit_gross_profit / cost_minor × 100`                           |
| Discount margin impact   | recompute margin at the discounted price                         |
| Total inventory cost     | `Σ cost_minor × sellable qty`                                    |
| Potential retail value   | `Σ retail_price_minor × sellable qty`                            |
| Non-sellable asset value | `Σ cost_minor × non-sellable qty`                                |
| Realized revenue         | `Σ orders.total_minor` for completed orders in period            |
| COGS                     | `Σ order_items.unit_cost_minor × qty` minus returned quantities  |
| Gross profit             | `revenue − COGS` (ADR-0007)                                      |
| Net cash profit          | `gross profit − Σ expenses` (ADR-0007)                           |
| Brand gross margin %     | `gross profit / revenue × 100`                                   |
| Net profit %             | `net cash profit / revenue × 100`                                |
| Inventory turnover       | `COGS / average inventory cost` over the period                  |
| Trapped cash / clearance | `Σ cost × sellable qty` for variants with zero sales in 60+ days |

Cost basis is **FIFO** (ADR-0012): each sale consumes the oldest `cost_layers`
row with `qty_remaining > 0`, and `order_items.unit_cost_minor` snapshots the
consumed cost. `cost_history` records price changes; `cost_layers` records
remaining quantity per cost so COGS is reproducible.

---

## 7. Durability & integrity

- `PRAGMA journal_mode = WAL`, `foreign_keys = ON`, `busy_timeout = 5000` (ADR-0010).
- **Single instance only**: one process against one SQLite file on a persistent
  volume; no replicas or horizontal scaling (ADR-0015).
- `AFTER INSERT` triggers on `stock_movements` UPSERT `stock_levels`; current
  stock is never written directly by handlers (ADR-0017).
- Nightly backup job + a documented manual restore drill.
- Financial and history tables reject `DELETE`/`UPDATE` of posted rows at the
  application layer; corrections use adjustment rows (`stock_movements`,
  `reason_codes`) and status transitions, never mutation of history.
- Catalog deletion = `is_archived = 1`, preserving order history.

---

## 8. sqlc / migration workflow

- `sqlc.yaml` points at `internal/db/schema.sql` (sqlc) and generates
  `internal/db/sqlc` (`emit_interface`, `emit_json_tags`, `emit_empty_slices`).
- `internal/db/schema.sql` **must mirror** the applied goose migrations; keep them
  in sync when adding tables so generated structs match the live DB.
- Named queries live in `internal/db/queries/*.sql`, split per domain
  (`auth`, `catalog`, `inventory`, `sales`, `finance`, `analytics`), and produce
  methods such as `Queries.CreateOrder`, `Queries.GetBestSellers`.
- Migrations: `0001_init` (catalog + inventory + orders + finance + audit +
  suppliers/PO tables), `0002_users` (auth). Suppliers/PO schema ships in v1 with
  UI deferred (ADR-0011).
- Use sqlc `overrides` to map CHECK-constrained TEXT columns (`orders.status`,
  `stock_levels.state`, `approved_users.role`, …) to named Go string types for
  type safety instead of bare `string`.
- `orders.order_number` is a cryptographically random short id (NanoID / short
  UUID), not a sequence, so receipt ids do not leak daily order volume.

---

## 9. Traceability & open questions

| Doc feature                   | Tables                                                                       |
| ----------------------------- | ---------------------------------------------------------------------------- |
| Product/variant management    | `products`, `product_variants`, `categories`, `collections`, `product_media` |
| Pricing, discounts, wholesale | `price_tiers`, `discounts`, `cost_history`, `cost_layers`                    |
| Restock & manual adjustments  | `stock_movements`, `reason_codes`, `stock_levels`                            |
| Low-stock / out-of-stock      | `product_variants.low_stock_threshold`, `stock_levels`                       |
| Orders, checkout, receipts    | `orders`, `order_items`, `payments`, `stock_holds`                           |
| Returns / exchanges           | `returns`, `return_items`                                                    |
| Customers                     | `customers`                                                                  |
| Suppliers / POs               | `suppliers`, `purchase_orders`, `purchase_order_items` (ADR-0011)            |
| P&L, COGS, OpEx               | `expenses`, `expense_categories`, `order_items.unit_cost_minor`              |
| Deadstock / trapped cash      | derived from `stock_levels` + `order_items` history                          |
| Audit & roles                 | `audit_logs`, `approved_users`                                               |
| Backups / WAL                 | §7 (ops, not a table)                                                        |

**Resolved decisions** (rationale in §10; recorded as ADRs):

1. Cost basis = **FIFO** via `cost_layers` (ADR-0012).
2. Suppliers/PO tables ship in `0001_init`; UI deferred (ADR-0011).
3. `discounts.value` = **integer basis points** for percent (ADR-0016).
4. Opening balance = `opening_balance` reason code + `stock_movements` +
   trigger (ADR-0017).
5. Retail prices are **IVA-inclusive (21%)**; `tax_minor` is display-only
   (ADR-0013).
6. Reporting timezone = `America/Argentina/Buenos_Aires` (ADR-0014).
7. Deployment = single instance, no scaling (ADR-0015).
8. Roles = only `admin` and `staff` authenticate; "clients" are `customers`,
   "providers" are `suppliers`, and neither logs in (ADR-0018).

**Still open** (decide before the related handler is built):

1. **Store credit has no balance model.** `returns.resolution` and
   `payments.method` allow `store_credit`, but there is no ledger to track
   issued vs redeemed credit. Needs a `store_credit_ledger`
   (`customer_id`, issued/redeemed, `order_id`/`return_id`).
2. **Stock lifecycle.** When stock commits (order create vs payment vs
   fulfilment), `stock_holds` state transitions, and negative/oversell policy.
3. **Discount stacking and rounding.** Line vs order vs channel discounts, and
   the step at which a percent is rounded to integer centavos.

## 10. Decision rationale (review notes)

1. Cost basis: FIFO vs weighted average?

   Decision: Go with FIFO (First-In, First-Out). Weighted average gets very messy with returns and manual adjustments in SQLite. FIFO is easier to calculate linearly based on stock_movements timestamps. The unit_cost_minor in order_items just grabs the oldest available cost_minor for that variant.

2. ADR-0011: Suppliers/PO in v1 or later?

   Decision: Put the tables in v1 (0001_init), but defer the UI/Go code for it until Phase 3. It's much easier to have the tables ready than to write complex ALTER TABLE migrations in SQLite later (SQLite has limited ALTER TABLE capabilities).

3. Discounts: percent encoding (basis points)?

   Decision: Yes, use basis points. 2000 = 20.00%. Make sure you explicitly add this rule: "Basis points are integers. Divide by 10000 in the view layer to get the decimal."

4. Opening stock / Migration of existing data?

   Decision: Yes, seed a reason_code called opening_balance. When you launch, you will do a giant loop inserting rows into stock_movements with ref_type = 'adjustment', reason_code = 'opening_balance'. The SQLite triggers (mentioned above) will automatically populate stock_levels.

5. Tax handling (AR IVA rules)?

   Context: In Argentina, retail prices to standard consumers (Consumidor Final) are almost always tax-inclusive (IVA included in the sticker price).

   Decision: Add a rule to the doc: "All retail_price_minor values are IVA-inclusive (21%). The orders.tax_minor column is strictly for display/receipt generation and is calculated dynamically at checkout (Total - (Total / 1.21))." This saves you from massive pricing headaches.

   1. Managing Enums with sqlc
      SQLite does not have native ENUM types; it uses TEXT with CHECK constraints (which you did perfectly). However, sqlc will generate standard Go string types for these columns.

   The Fix: In your docs/rules.md or sqlc.yaml, tell the AI to use overrides to force Go custom types.

   Example: "AI, map the orders.status column to a custom Go type OrderStatus string in sqlc.yaml so we have type safety in our Go code."

6. Keeping stock_movements and stock_levels in sync
   If the Go application handles a sale, it has to insert a stock_movement AND update stock_levels. If the Go transaction fails midway, your ledger and current stock will desync.

   The Fix: Have the AI write SQLite Triggers.

   Prompt addition: "Create SQLite AFTER INSERT triggers on stock_movements that automatically UPSERT the stock_levels table. This keeps the Go code simpler and guarantees the ledger and current stock never desync."

7. Sequential IDs vs UUIDs
   You used INTEGER PRIMARY KEY for IDs. This is highly performant in SQLite. You also smartly added orders.order_number (TEXT) for human-facing receipts.

   The Fix: Just ensure order_number uses a cryptographically secure random string (like a NanoID or short UUID) so competitors can't guess how many orders you get a day by looking at their receipt.
