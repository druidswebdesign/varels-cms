# varels_cms

Internal inventory-management CMS for a clothing brand.

## Purpose

Track stock across product variants (sizes/colors), record sales and restocks,
watch profit margins, and answer business questions (best sellers, deadstock,
trapped cash) without a heavyweight ERP.

## Features

- **Inventory**: products, variants (size/color/SKU), archive instead of delete.
- **Restock & adjustments**: logged restock history and manual stock corrections.
- **Low-stock alerts**: configurable thresholds, out-of-stock detection.
- **Sales**: record sales (atomic stock decrement), returns/refunds.
- **Pricing & profit**: unit profit, margin %, markup %, inventory valuation.
- **Analytics**: best sellers all-time / yearly / monthly, COGS, realized profit.
- **Deadstock**: stale-inventory detection and trapped-cash reporting.
- **Audit log**: who changed what, when, and why.
- **CSV export**: export any table for bookkeeping.
- **Auth**: invite-only Google OAuth whitelist plus a break-glass local admin.

## Tech stack

| Layer             | Tool                                    |
| ----------------- | --------------------------------------- |
| Language & router | Go + go-chi                             |
| Templating        | templ                                   |
| Frontend dynamics | HTMX + Alpine.js                        |
| Styling           | Tailwind CSS (standalone CLI, zero npm) |
| Database          | SQLite + sqlc                           |
| Migrations        | goose                                   |
| Auth              | Goth + SCS                              |
| Dev tooling       | Air                                     |

The app compiles to a single self-contained binary plus one SQLite file.

## Project layout

See [docs/architecture.md](docs/architecture.md).

```
cmd/server          entrypoint + route wiring
internal/db         db.go, seed.go, schema.sql, query.sql, migrations/, sqlc/
internal/auth       OAuth, sessions, whitelist, roles, auth middleware
internal/handlers   chi HTTP handlers + shared middleware
internal/views      templ layouts, components, pages, partials
assets/             Tailwind input/output CSS, JS, images
data/               SQLite database (gitignored)
```

## Getting started

Prerequisites are single binaries — no Node.js required:

- Go 1.23+
- [`templ`](https://github.com/a-h/templ)
- [`sqlc`](https://sqlc.dev)
- [`goose`](https://github.com/pressly/goose)
- [Tailwind standalone CLI](https://github.com/tailwindlabs/tailwindcss/releases)
- [`air`](https://github.com/air-verse/air) (dev only)

```bash
cp .env.example .env          # fill in secrets and Google OAuth credentials
go mod tidy
make generate                 # templ
make sqlc                     # generated Go query code
make css                      # build assets/css/app.css
make migrate                  # apply goose migrations
make dev                      # run with live reload
```

### Auth

Login is a Google OAuth whitelist backed by the `approved_users` table. The first
run bootstraps `INITIAL_OWNER_EMAIL` as the `admin` owner when the database is empty.
A hidden `/admin-login` route provides an emergency local password for the owner.

## Deployment

```bash
make build                    # bin/varels_cms
./bin/varels_cms              # serves on $PORT, DB at $DB_PATH
```

See `Dockerfile` for a container build.
