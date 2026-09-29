# Routes & UI Flow — varels_cms

Authoritative HTTP surface for the app. Derived from the project docs and the
actual stub tree (`cmd/server/main.go` route TODO, `internal/handlers/*.go`,
`internal/views/**/*.templ`) plus the functional spec
(`docs/functions.md`) and `docs/schema.md`.

Router: **go-chi** (README / `architecture.md`). Route wiring lives in
`cmd/server/main.go` `routes()` — currently a placeholder `net/http.ServeMux`
(`cmd/server/main.go:67`); replace with chi groups.

> **Naming note:** internally sales are the `orders` / `order_items` model
> (ADR-0006), but the URL surface and handler/view filenames stay `/sales`
> (`sale_handler.go`, `sale_form.templ`, `sales_list.templ`) to match the
> existing tree and `main.go:64`.

---

## 1. Conventions

| Concern          | Rule                                                                                                                                                                                    |
| ---------------- | --------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| Base path        | App mounted at `/`. Local dev `http://localhost:8080`.                                                                                                                                  |
| Auth             | Everything except the public routes in §2 is behind `auth.RequireLogin`.                                                                                                                |
| Role ladder      | `staff` < `admin`. Only these two roles authenticate; "clients" are `customers` and "providers" are `suppliers`, and neither logs in (ADR-0018). `auth.RequireRole(min)` gates a group. |
| Financial gating | COGS, buy price, margin, net profit, OpEx and financial CSV exports require `admin` (`functions.md` §3).                                                                                |
| HTTP methods     | HTML forms only do GET/POST. Create/update/delete use `POST` with an explicit action segment (e.g. `/archive`); no reliance on `PUT`/`DELETE`.                                          |
| PRG              | Non-HTMX `POST` → `303 See Other` redirect, then a `GET` renders the page (flash in session).                                                                                           |
| HTMX             | Fragment endpoints detect `HX-Request: true` and return a partial with `200`; a direct browser hit renders the full page.                                                               |
| CSRF             | `handlers.CSRF` on every state-changing route; token exposed globally via `hx-headers` on `<body>`.                                                                                     |
| Flash            | Session-backed (SCS). Full page uses `partials.FlashMessages()`; HTMX uses an out-of-band swap `hx-swap-oob="true"` into `#flash`.                                                      |
| Money            | Integer ARS minor units end-to-end (ADR-0005); formatting only in views.                                                                                                                |
| Transactions     | Order create, restock, adjustment, and return commit in one SQLite transaction (WAL, `foreign_keys=ON`, ADR-0010).                                                                      |
| Reporting time   | Date filters (`?from=`, `?to=`, `?year=`, `?month=`) are interpreted in `America/Argentina/Buenos_Aires` (UTC−3); timestamps are stored UTC (ADR-0014).                                 |
| Scale            | Single app instance; no horizontal scaling or replicas (ADR-0015).                                                                                                                      |

### Global middleware chain

```
RequestID → Logger → Recover → CSRF → (route group: RequireLogin → RequireRole)
```

`internal/handlers/middleware/middleware.go` provides `Logger`, `Recover`, `RequestID`,
`CSRF`; `internal/auth/middleware.go` provides `RequireLogin`, `RequireRole`.

---

## 2. Public routes (auth_handler.go)

| Method | Path                    | Handler                | Renders                      | Notes                                                                                                |
| ------ | ----------------------- | ---------------------- | ---------------------------- | ---------------------------------------------------------------------------------------------------- |
| GET    | `/login`                | `HandleLoginPage`      | `pages.Login()`              | Single "Sign in with Google" button. If already authenticated → `/dashboard`.                        |
| GET    | `/auth/google`          | `HandleGoogleBegin`    | —                            | Goth OAuth begin; `state` cookie.                                                                    |
| GET    | `/auth/google/callback` | `HandleGoogleCallback` | redirect                     | Whitelist check against `approved_users`. Allowed → session + `/dashboard`; denied → `/auth/denied`. |
| GET    | `/auth/denied`          | `HandleAccessDenied`   | `pages.Login()` + flash      | "Access Denied: Your email is not authorized to view this system." (`auth.md`)                       |
| GET    | `/admin-login`          | `HandleAdminLoginPage` | `pages.Login()` (local form) | Break-glass route, intentionally **not** linked in the UI (`auth.md`).                               |
| POST   | `/admin-login`          | `HandleAdminLogin`     | redirect                     | Local email + password; rate-limited; audit-logged.                                                  |
| POST   | `/logout`               | `HandleLogout`         | redirect `/login`            | Destroys SCS session.                                                                                |

---

## 3. Application routes (authenticated)

### 3.1 Dashboard — `dashboard_handler.go`

| Method | Path                      | Handler                | Renders                     | HTMX | Notes                             |
| ------ | ------------------------- | ---------------------- | --------------------------- | ---- | --------------------------------- |
| GET    | `/`                       | `HandleIndex`          | redirect                    | —    | → `/dashboard`.                   |
| GET    | `/dashboard`              | `HandleDashboard`      | `pages.Dashboard()`         | —    | Cards + low-stock + latest sales. |
| GET    | `/dashboard/low-stock`    | `HandleLowStockWidget` | `partials.LowStockWidget()` | yes  | Refresh after restock.            |
| GET    | `/dashboard/profit-cards` | `HandleProfitCards`    | `partials.ProfitCards()`    | yes  | `admin`+.                         |

### 3.2 Products — `product_handler.go`

Views: `pages/products_list.templ`, `pages/product_form.templ`,
`pages/product_detail.templ`, `partials/product_row.templ`.

| Method | Path                         | Handler                    | Renders                 | Notes                                                 |
| ------ | ---------------------------- | -------------------------- | ----------------------- | ----------------------------------------------------- |
| GET    | `/products`                  | `HandleProductsList`       | `pages.ProductsList()`  | Filters `?q=&category=&collection=&archived=&page=`.  |
| GET    | `/products/search`           | `HandleProductsSearch`     | rows (HTMX)             | Live search; returns `partials.ProductRow`.           |
| GET    | `/products/new`              | `HandleProductForm`        | `pages.ProductForm()`   | Create.                                               |
| POST   | `/products`                  | `HandleProductCreate`      | redirect/row            | Errors re-render form with field messages.            |
| GET    | `/products/{id}`             | `HandleProductDetail`      | `pages.ProductDetail()` | Variants, media, margin badge.                        |
| GET    | `/products/{id}/edit`        | `HandleProductEdit`        | `pages.ProductForm()`   |                                                       |
| POST   | `/products/{id}`             | `HandleProductUpdate`      | redirect                |                                                       |
| POST   | `/products/{id}/archive`     | `HandleProductArchive`     | redirect                | Soft delete (`is_archived`, ADR-0010).                |
| POST   | `/products/{id}/restore`     | `HandleProductRestore`     | redirect                |                                                       |
| GET    | `/collections`               | `HandleCollectionsList`    | `pages.Collections()`   | Drops/seasons/essentials.                             |
| POST   | `/collections`               | `HandleCollectionCreate`   | redirect                | `name`,`slug`,`kind`,`season`,`launch_date`.          |
| POST   | `/collections/{id}/archive`  | `HandleCollectionArchive`  | redirect                | `SetCollectionArchived`.                              |
| POST   | `/products/{id}/collections` | `HandleProductCollections` | redirect                | Posted `collection_id` values become the product set. |
| POST   | `/products/{id}/media`       | `HandleMediaCreate`        | redirect                | `is_primary` clears prior primaries.                  |
| POST   | `/media/{id}/delete`         | `HandleMediaDelete`        | redirect                |                                                       |

### 3.3 Variants — `variant_handler.go`

View: `partials/variant_table.templ`, `partials/restock_row.templ`,
`components/modal.templ`.

| Method | Path                       | Handler                  | Renders                          | Notes                     |
| ------ | -------------------------- | ------------------------ | -------------------------------- | ------------------------- |
| GET    | `/products/{id}/variants`  | `HandleVariantTable`     | `partials.VariantTable()` (HTMX) |                           |
| POST   | `/products/{id}/variants`  | `HandleVariantCreate`    | row (HTMX)                       | SKU unique.               |
| GET    | `/variants/{id}/edit`      | `HandleVariantForm`      | `pages.VariantForm()`            |                           |
| POST   | `/variants/{id}`           | `HandleVariantUpdate`    | redirect                         | Size, color, prices, SKU. |
| POST   | `/variants/{id}/threshold` | `HandleVariantThreshold` | inline (HTMX)                    | `low_stock_threshold`.    |
| POST   | `/variants/{id}/archive`   | `HandleVariantArchive`   | row                              |                           |

### 3.4 Restock & adjustments — `restock_handler.go`

Views: `pages/restock_form.templ`, `partials/restock_row.templ`.

| Method | Path                     | Handler                | Renders               | Notes                                                                                                                                                                    |
| ------ | ------------------------ | ---------------------- | --------------------- | ------------------------------------------------------------------------------------------------------------------------------------------------------------------------ |
| GET    | `/restocks`              | `HandleRestocksList`   | list page             | History from `stock_movements` (`restock`/`po_receipt`).                                                                                                                 |
| GET    | `/variants/{id}/restock` | `HandleRestockForm`    | `pages.RestockForm()` | Modal/page; choose location.                                                                                                                                             |
| POST   | `/variants/{id}/restock` | `HandleRestock`        | updated row + flash   | Writes `stock_movements` (+) and a `cost_layers` row at the incoming cost (ADR-0012); `cost_history` row only if the cost changed. Current stock comes from the trigger. |
| POST   | `/variants/{id}/adjust`  | `HandleAdjustStock`    | row + flash (HTMX)    | Requires `reason_code` (`damaged`, `pr_gift`, `shrinkage_stolen`, `sample`, `personal`, `recount`).                                                                      |
| GET    | `/stock-movements`       | `HandleStockMovements` | ledger page           | Filters `?variant=&location=&ref_type=&from=&to=`. `staff`+.                                                                                                             |

### 3.5 Sales / checkout / returns — `sale_handler.go`

Views: `pages/sale_form.templ` (POS), `pages/sales_list.templ`,
`partials/sales_chart.templ`.

| Method | Path                   | Handler                | Renders                  | Notes                                                                                                                                  |
| ------ | ---------------------- | ---------------------- | ------------------------ | -------------------------------------------------------------------------------------------------------------------------------------- |
| GET    | `/sales`               | `HandleSalesList`      | `pages.SalesList()`      | Filters `?from=&to=&channel=&location=&status=`.                                                                                       |
| GET    | `/sales/filter`        | `HandleSalesFilter`    | rows (HTMX)              |                                                                                                                                        |
| GET    | `/sales/new`           | `HandleSaleForm`       | `pages.SaleForm()`       | POS: customer (optional), channel, location, multi-line items.                                                                         |
| POST   | `/sales`               | `HandleSaleCreate`     | redirect `/sales/{id}`   | Atomic: reserve/commit stock, write `stock_movements`, create `payments`, decrement `stock_levels`.                                    |
| GET    | `/sales/{id}`          | `HandleSaleDetail`     | receipt/detail           |                                                                                                                                        |
| POST   | `/sales/{id}/void`     | `HandleSaleVoid`       | redirect                 | Restock + `status='cancelled'`; never deletes the order.                                                                               |
| GET    | `/sales/{id}/return`   | `HandleReturnForm`     | return form              | Partial return UI.                                                                                                                     |
| POST   | `/sales/{id}/return`   | `HandleReturnCreate`   | receipt/detail           | Per line: qty, `condition_state` (sellable/damaged), `resolution` (refund/store_credit/exchange), `reason_code`; restocks accordingly. |
| POST   | `/sales/{id}/payments` | `HandlePaymentCreate`  | redirect                 | Split tender; recomputes `payment_status`.                                                                                             |
| GET    | `/customers`           | `HandleCustomersList`  | `pages.Customers()`      | `?q=` search. Business "clients" (ADR-0018).                                                                                           |
| POST   | `/customers`           | `HandleCustomerCreate` | redirect to detail       |                                                                                                                                        |
| GET    | `/customers/{id}`      | `HandleCustomerDetail` | `pages.CustomerDetail()` | Profile, order history, store-credit ledger.                                                                                           |
| POST   | `/customers/{id}`      | `HandleCustomerUpdate` | redirect to detail       |                                                                                                                                        |

### 3.6 Analytics — `analytics_handler.go` (`admin`+)

Views: `pages/analytics.templ`, `partials/best_sellers_table.templ`,
`partials/sales_chart.templ`, `partials/profit_cards.templ`.

| Method | Path                      | Handler             | Renders                              | Notes                                             |
| ------ | ------------------------- | ------------------- | ------------------------------------ | ------------------------------------------------- |
| GET    | `/analytics`              | `HandleAnalytics`   | `pages.Analytics()`                  | `?year=&month=`.                                  |
| GET    | `/analytics/best-sellers` | `HandleBestSellers` | `partials.BestSellersTable()` (HTMX) | all-time / year / month.                          |
| GET    | `/analytics/margin`       | `HandleMargin`      | numbers (HTMX)                       | Matches `spec-driven-dev.md` example `?month=11`. |
| GET    | `/analytics/chart`        | `HandleSalesChart`  | `partials.SalesChart()` (HTMX)       | ApexCharts data or prebuilt markup.               |
| GET    | `/analytics/pl`           | `HandleProfitLoss`  | _planned_                            | Revenue − COGS − OpEx (ADR-0007).                 |
| GET    | `/analytics/sizes`        | `HandleSizeCurve`   | _planned_                            | Best-selling sizes/colors.                        |

### 3.7 Deadstock — `deadstock_handler.go` (`admin`+)

View: `pages/deadstock.templ`.

| Method | Path               | Handler                | Renders             | Notes                               |
| ------ | ------------------ | ---------------------- | ------------------- | ----------------------------------- |
| GET    | `/deadstock`       | `HandleDeadstock`      | `pages.Deadstock()` | `?stale_days=30\|60\|90`.           |
| GET    | `/deadstock/table` | `HandleDeadstockTable` | fragment (HTMX)     | Trapped cash, clearance candidates. |

### 3.8 Staff, roles & audit — `staff_handler.go`, `audit_handler.go` (`admin`+)

View: `pages/staff.templ`, `pages/audit.templ`.

| Method | Path                         | Handler                    | Renders         | Notes                                               |
| ------ | ---------------------------- | -------------------------- | --------------- | --------------------------------------------------- |
| GET    | `/staff`                     | `HandleStaffList`          | `pages.Staff()` | Whitelist + invite form.                            |
| POST   | `/staff/invite`              | `HandleStaffInvite`        | redirect        | Insert `approved_users` (`auth.md`).                |
| POST   | `/staff/{id}/role`           | `HandleStaffRole`          | redirect        | Role change.                                        |
| POST   | `/staff/{id}/disable`        | `HandleStaffDisable`       | redirect        | `is_active = 0` — instant offboarding.              |
| POST   | `/staff/{id}/enable`         | `HandleStaffEnable`        | redirect        |                                                     |
| POST   | `/staff/{id}/reset-password` | `HandleStaffResetPassword` | redirect        | Break-glass local accounts only (`provider=local`). |
| GET    | `/audit`                     | `HandleAuditLog`           | `pages.Audit()` | `?user=&entity=&from=&to=`.                         |

### 3.9 Suppliers & purchase orders — `supplier_handler.go` (`admin`+)

Views: `pages/suppliers.templ`, `pages/purchase_orders.templ`,
`pages/purchase_order_detail.templ`.

| Method | Path                            | Handler                      | Renders                       | Notes                                                                                             |
| ------ | ------------------------------- | ---------------------------- | ----------------------------- | ------------------------------------------------------------------------------------------------- |
| GET    | `/suppliers`                    | `HandleSuppliersList`        | `pages.Suppliers()`           | Create + inline edit.                                                                             |
| POST   | `/suppliers`                    | `HandleSupplierCreate`       | redirect                      |                                                                                                   |
| POST   | `/suppliers/{id}`               | `HandleSupplierUpdate`       | redirect                      |                                                                                                   |
| GET    | `/purchase-orders`              | `HandlePurchaseOrdersList`   | `pages.PurchaseOrders()`      | Filters `?supplier=&status=`.                                                                     |
| POST   | `/purchase-orders`              | `HandlePurchaseOrderCreate`  | redirect to detail            | Starts as `draft`.                                                                                |
| GET    | `/purchase-orders/{id}`         | `HandlePurchaseOrderDetail`  | `pages.PurchaseOrderDetail()` | Lines + add-line + receive forms.                                                                 |
| POST   | `/purchase-orders/{id}/items`   | `HandlePurchaseOrderAddItem` | redirect                      | `UpsertPurchaseOrderItem` (upserts by `(po, variant)`).                                           |
| POST   | `/purchase-orders/{id}/status`  | `HandlePurchaseOrderStatus`  | redirect                      | `draft`/`ordered`/`partial`/`received`/`cancelled`.                                               |
| POST   | `/purchase-orders/{id}/receive` | `HandlePurchaseOrderReceive` | redirect                      | One transaction: `stock_movements` (+, `po_receipt`) + `cost_layers`; advances status (ADR-0012). |

### 3.10 Stock holds — `hold_handler.go`

View: `pages/holds.templ`.

| Method | Path                  | Handler             | Renders         | Notes                                                        |
| ------ | --------------------- | ------------------- | --------------- | ------------------------------------------------------------ |
| GET    | `/holds`              | `HandleHoldsList`   | `pages.Holds()` | Sweeps `ExpireStockHolds`, then lists (filter `?status=`).   |
| POST   | `/holds`              | `HandleHoldCreate`  | redirect        | `variant_id`,`location_id`,`quantity`,`source`,`expires_at`. |
| POST   | `/holds/{id}/release` | `HandleHoldRelease` | redirect        | `SetStockHoldStatus(released)`.                              |

### 3.11 Exports — `export_handler.go`

| Method | Path                     | Handler        | Access   | Notes                                                                            |
| ------ | ------------------------ | -------------- | -------- | -------------------------------------------------------------------------------- |
| GET    | `/export/{resource}.csv` | `HandleExport` | `staff`+ | `resource` ∈ `products`, `variants`, `sales`, `stock-movements`, `best-sellers`. |
|        |                          |                | `admin`+ | `margins`, `pl`, `expenses`.                                                     |

Responses set `Content-Type: text/csv; charset=utf-8` and
`Content-Disposition: attachment; filename="<resource>-<date>.csv"`.

### 3.12 Expenses & OpEx — `expense_handler.go` (`admin`+)

Views: `pages/expenses.templ`, `pages/expense_categories.templ`.

| Method | Path                  | Handler                       | Renders                     | Notes                              |
| ------ | --------------------- | ----------------------------- | --------------------------- | ---------------------------------- |
| GET    | `/expenses`           | `HandleExpensesList`          | `pages.Expenses()`          |                                    |
| POST   | `/expenses`           | `HandleExpenseCreate`         | redirect                    | `incurred_at` interpreted ART→UTC. |
| GET    | `/expense-categories` | `HandleExpenseCategories`     | `pages.ExpenseCategories()` |                                    |
| POST   | `/expense-categories` | `HandleExpenseCategoryCreate` | redirect                    |                                    |

### 3.13 Planned modules (no handler yet)

| Area          | Routes                                          | Ref             |
| ------------- | ----------------------------------------------- | --------------- |
| Notifications | daily low-stock digest (scheduled, not a route) | functions.md §5 |

---

## 4. Non-HTML endpoints

| Method | Path        | Handler                               | Notes                                                                                                                                                       |
| ------ | ----------- | ------------------------------------- | ----------------------------------------------------------------------------------------------------------------------------------------------------------- |
| GET    | `/assets/*` | `http.FileServer(http.Dir("assets"))` | Already wired in `cmd/server/main.go:70`; mount via chi `Handle("/assets/*", ...)`. Serves `app.css`, `htmx.min.js`, `alpine.min.js`, `app.js`, `logo.svg`. |
| GET    | `/healthz`  | `HandleHealth`                        | 200 when DB reachable; for the container/host check.                                                                                                        |

---

## 5. Auth flow

1. `GET /login` → "Sign in with Google" → `GET /auth/google` → Google.
2. Google → `GET /auth/google/callback` → Goth exchanges the code for the email.
3. Look up `approved_users.email` where `is_active = 1`.
   - **Allowed** → create SCS session (`sessions` table), set `last_login_at`, redirect `/dashboard`.
   - **Denied** → redirect `/auth/denied` with the access-denied flash.
4. Break-glass: `GET /admin-login` → `POST /admin-login` → local `password_hash`
   check (`provider='local'`) → session. Covered by ADR-0004.
5. Bootstrap: first run with empty `approved_users` seeds `INITIAL_OWNER_EMAIL`
   as `admin` (`internal/db/seed.go`, `auth.md`).

---

## 6. Error & HTMX contract

| Condition                   | Status | Full-page response                | HTMX response                                                               |
| --------------------------- | ------ | --------------------------------- | --------------------------------------------------------------------------- |
| Not authenticated           | 401    | redirect `/login`                 | `HX-Redirect: /login`                                                       |
| Authenticated, role too low | 403    | render `/dashboard` + error flash | return toast fragment, no swap of target                                    |
| Validation failure          | 422    | re-render form with field errors  | swap form with errors                                                       |
| Not found                   | 404    | `pages` 404                       | empty state fragment                                                        |
| Insufficient stock          | 409    | re-render POS + error flash       | **red error toast** fragment, no state change (per `spec-driven-dev.md` §2) |
| Duplicate SKU / email       | 409    | form error                        | inline field error                                                          |
| DB / panic                  | 500    | error page (Recover)              | error toast                                                                 |

Rules:

- Stock-affecting `POST`s validate availability **inside** the transaction; on
  failure the transaction rolls back and no `stock_movements` row is written.
- Every stock/financial mutation writes an `audit_logs` row and is append-only
  (ADR-0010) — a "wrong" sale or adjustment is reversed by a new compensating
  row, never edited or deleted.
- **Flash** is session-backed (`auth.Manager.SetFlash`/`PopFlash`): handlers set a
  message before a `303`, `flash.FlashFromSession` moves it into the request
  context, and the layout renders it once into `#flash`. HTMX mutation responses
  append `partials.FlashOOB` for an out-of-band swap.
- HTMX is wired for: live product search (`#product-rows`), variant
  create/archive (`#variant-table`), the sales filter (`#sale-rows`), analytics
  best-sellers period tabs and chart/size on `#year`/`#month` change, the
  deadstock window (`#deadstock-table`), and dashboard profit-cards/low-stock on
  load. Alpine drives the POS line editor and can raise toasts via
  `$store.app.flash()`.

HTMX targets use stable ids: `#flash`, `#product-rows`, `#variant-table`,
`#sale-rows`, `#low-stock`, `#profit-cards`, `#best-sellers`, `#sales-chart`,
`#size-curve`, `#deadstock-table`.

---

## 7. Route → file map

| Group       | Handler file                                                     | Views                                                                                                                                  |
| ----------- | ---------------------------------------------------------------- | -------------------------------------------------------------------------------------------------------------------------------------- |
| Auth        | `internal/handlers/authweb/auth_handler.go`                      | `pages/login.templ`                                                                                                                    |
| Dashboard   | `dashboard_handler.go`                                           | `pages/dashboard.templ`, `partials/low_stock_widget.templ`, `partials/profit_cards.templ`                                              |
| Products    | `product_handler.go`                                             | `pages/products_list.templ`, `product_form.templ`, `product_detail.templ`, `partials/product_row.templ`, `partials/product_rows.templ` |
| Collections | `collection_handler.go`                                          | `pages/collections.templ`                                                                                                              |
| Media       | `media_handler.go`                                               | (rendered inside `product_detail.templ`)                                                                                               |
| Variants    | `variant_handler.go`                                             | `partials/variant_table.templ`, `pages/variant_form.templ`, `components/modal.templ`                                                   |
| Restock     | `restock_handler.go`                                             | `pages/restock_form.templ`, `partials/restock_row.templ`                                                                               |
| Sales       | `sale_handler.go`                                                | `pages/sale_form.templ`, `pages/sales_list.templ`, `partials/sale_rows.templ`                                                          |
| Customers   | `customer_handler.go`                                            | `pages/customers.templ`, `pages/customer_detail.templ`                                                                                 |
| Suppliers   | `supplier_handler.go`                                            | `pages/suppliers.templ`, `pages/purchase_orders.templ`, `pages/purchase_order_detail.templ`                                            |
| Holds       | `hold_handler.go`                                                | `pages/holds.templ`                                                                                                                    |
| Analytics   | `analytics_handler.go`                                           | `pages/analytics.templ`, `partials/best_sellers_table.templ`, `partials/sales_chart.templ`                                             |
| Deadstock   | `deadstock_handler.go`                                           | `pages/deadstock.templ`                                                                                                                |
| Expenses    | `expense_handler.go`                                             | `pages/expenses.templ`, `pages/expense_categories.templ`                                                                               |
| Staff       | `staff_handler.go`                                               | `pages/staff.templ`                                                                                                                    |
| Audit       | `audit_handler.go`                                               | `pages/audit.templ`                                                                                                                    |
| Export      | `export_handler.go`                                              | — (CSV)                                                                                                                                |
| Shared      | `internal/handlers/middleware/middleware.go`, `internal/auth/middleware.go` | `layouts/base.templ`, `components/{navbar,sidebar,table,badge,button,flash,modal}.templ`, `partials/flash_messages.templ`              |
