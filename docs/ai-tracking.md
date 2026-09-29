# AI Work Tracking — varels_cms

> Snapshot of what has been built and how to track future AI changes. This is a
> living reference, not a chronological log — the dated build history lives in
> [`implementation.md`](implementation.md). Update this file when the shape of
> the project (routes, wiring, tooling) changes.

**Last updated:** 2026-09-29 (shadcn dashboard shell + design tokens).

---

## 1. Why this file exists

`implementation.md` is append-only history. This file answers "what exists right
now and how do I measure AI changes?", so an assistant (or a human) can orient
without reading every log entry.

---

## 2. Current status snapshot

| Area           | State                                                                                                                |
| -------------- | -------------------------------------------------------------------------------------------------------------------- |
| Backend routes | Complete for the documented spec: **86 routes wired** in `cmd/server/main.go`                                        |
| Handlers       | **22 files**, **85 `Handle*` methods** under `internal/handlers/`                                                    |
| Views          | **48 templ files** (`pages/`, `partials/`, `components/`, `layouts/`)                                                |
| DB layer       | sqlc-generated + goose migrations `0001`–`0003`, `schema.sql` mirrored                                               |
| Tests          | **13** integration tests in `internal/handlers/*_test.go` (money paths + ledger invariants); see `docs/testing.md`    |
| Source size    | ~**10,650** hand-written Go lines, ~**2,869** templ lines (generated `*_templ.go` and `internal/db/sqlc/*` excluded) |

Only genuinely unimplemented feature: the daily low-stock email digest
(scheduled job, not a route; see `implementation.md`).

---

## 3. Frontend connectivity matrix

Browser libraries are vendored in `assets/js/` (not Go modules): HTMX 2.0.4 and
Alpine.js 3.14.9, loaded from `layouts/base.templ` (ADR-0022).

### Design system (shadcn-style)

Full spec — tokens, shell, components, conventions and migration status — lives
in [`docs/frontend.md`](frontend.md). Summary:

`assets/css/input.css` defines shadcn design tokens (`--background`, `--card`,
`--primary`, `--muted`, `--border`, `--sidebar-*`, `--radius`) with light and
`.dark` values, exposed as Tailwind utilities via `@theme inline`. Use the
semantic classes (`bg-card`, `text-muted-foreground`, `border-border`,
`bg-sidebar`, …) in new markup instead of raw `gray-*`/`white`.

The shell in `layouts/base.templ` is a fixed icon sidebar (`components/sidebar.templ`,
grouped, active state, Alpine drawer under `lg`) plus a sticky header
(`components/navbar.templ`: title, product search, theme toggle, sign-out).
Reusable primitives live in `components/`: `card.templ` (`Card*`, `StatCard`),
`icons.templ` (`Icon` inline lucide set). Dark mode is toggled by
`Alpine.data("theme")` in `app.js` and applied pre-paint by an inline `<head>`
script. Migration of the older pages from `gray-*` to tokens is in progress.

### HTMX (`hx-*`, 35 attributes)

| Interaction            | Element / view                                                              | Endpoint                                                      |
| ---------------------- | --------------------------------------------------------------------------- | ------------------------------------------------------------- |
| Live product search    | `pages/products_list.templ` → `#product-rows`                               | `GET /products/search`                                        |
| Variant create/archive | `product_detail.templ`, `partials/variant_table.templ` → `#variant-table`   | `POST /products/{id}/variants`, `POST /variants/{id}/archive` |
| Sales filter           | `pages/sales_list.templ` → `#sale-rows`                                     | `GET /sales/filter`                                           |
| Analytics period tabs  | `partials/best_sellers_table.templ` → `#best-sellers`                       | `GET /analytics/best-sellers?period=`                         |
| Analytics chart/sizes  | `partials/sales_chart.templ`, `size_curve.templ` on `#year`/`#month` change | `GET /analytics/chart`, `/analytics/sizes`                    |
| Deadstock window       | `pages/deadstock.templ` → `#deadstock-table`                                | `GET /deadstock/table?stale_days=`                            |
| Dashboard cards        | `pages/dashboard.templ` `hx-trigger="load"`                                 | `GET /dashboard/profit-cards`, `/dashboard/low-stock`         |
| CSRF                   | `layouts/base.templ` `hx-headers`                                           | all HTMX requests                                             |

### Alpine (`x-*`, 10 attributes)

- **POS line editor** — `Alpine.data("pos", …)` in `assets/js/app.js`, used by
  `pages/sale_form.templ` (`x-data="pos()"`, `<template x-for>`): add/remove
  lines, price autofill, reactive total.
- **Toast store** — `$store.app.flash(msg)` registered on `alpine:init`.
- `app.js` re-initialises Alpine on `htmx:afterSwap` and toasts HTMX failures.

### Flash messages

Session-backed (`auth.Manager.SetFlash`/`PopFlash`), moved into the request
context by `handlers.FlashFromSession`, rendered once into `#flash` by
`layouts/base.templ`; `partials.FlashOOB` provides the out-of-band swap for HTMX
responses. Handlers set success/error flashes instead of silent `?err=` redirects.

### Not yet converted

- Restock/adjust forms (`restock_form.templ`) are still full-page forms + flash.
- A few `?err=` query params remain on the sale/return forms (they render inline).

---

### Test layer

`internal/handlers/testenv_test.go` builds an isolated SQLite DB per test and
mounts the real handlers. Covered today: FIFO sale, totals, partial/damaged
returns, store credit (issue/redeem/over-spend), oversell atomicity, void
compensation, and the `stock_levels == Σ stock_movements` invariant. Full
strategy, ordered plan and coverage matrix: `docs/testing.md`.

---

## 4. How to track AI changes

### Preferred: git

The project is **not** a git repository yet, so there is currently no baseline.
Once initialised, per-task commits make every change measurable:

```bash
git init && git add -A && git commit -m "baseline"   # explicit, once
# after a task:
git show --stat HEAD
git diff --numstat <base>..HEAD | awk '{a+=$1;d+=$2} END{printf "+%d -%d\n",a,d}'
git log --grep='AI:' --oneline
```

Attribute AI work with a commit trailer:

```text
Add collections/customers/suppliers/audit routes

AI: deepseek-flash
Task: missing-routes
```

### Always: the dated log

Add one `### YYYY-MM-DD — <task>` entry per work session to
`implementation.md`, listing files touched, verification, and anything deferred.

### Line counting (exclude generated code)

```bash
# hand-written only
find . -name '*.go' ! -name '*_templ.go' ! -path './vendor/*' | xargs wc -l | tail -1
cloc --not-match-f='_templ\.go$|\.sql\.go$' .
```

Exclude `*_templ.go`, `internal/db/sqlc/*`, `go.sum`, and `assets/css/app.css` —
they are generated and inflate any count.

---

## 5. Verification gate

Every change should end with:

```bash
templ generate && go build ./... && go vet ./... && gofmt -l internal cmd
go test ./...
node --check assets/js/app.js
```

and, for anything touching routing or interactivity, a live smoke test against a
temp DB (`APP_ENV=development`, `DB_PATH=/tmp/…`).
