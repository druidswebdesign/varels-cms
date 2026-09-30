# varels-cms

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

- Go 1.26+
- [`templ`](https://github.com/a-h/templ)
- [`sqlc`](https://sqlc.dev)
- [`goose`](https://github.com/pressly/goose)
- [Tailwind standalone CLI](https://github.com/tailwindlabs/tailwindcss/releases)
- [`air`](https://github.com/air-verse/air) (dev only)

```bash
cp .env.example .env          # loaded automatically; real env vars take precedence
go mod tidy
make generate                 # templ
make sqlc                     # generated Go query code
make css                      # build assets/css/app.css
make migrate                  # apply goose migrations
make dev                      # run with live reload
```

Generated code (`*_templ.go`, `internal/db/sqlc/*.go`) and `assets/css/app.css` are
gitignored: a fresh checkout must run `make generate sqlc css` (CI and the
Dockerfile do this automatically).

### Auth

Login is a Google OAuth whitelist backed by the `approved_users` table. The first
run bootstraps `INITIAL_OWNER_EMAIL` as the `admin` owner when the database is empty.
A hidden `/admin-login` route provides an emergency local password for the owner.

## Deployment

```bash
make build                    # bin/varels-cms
./bin/varels-cms              # serves on $PORT, DB at $DB_PATH
```

The server migrates and seeds the database on startup, so no separate migrate
step is needed in production.

For a **private deployment**, the server refuses to start unless:
- `APP_ENV=production`
- `SESSION_SECRET` is at least 32 bytes (e.g. `openssl rand -hex 32`)
- `ADMIN_PASSWORD` is at least 12 characters (break-glass login)

and HTTPS terminates in front of the app (nginx/Caddy/Cloudflare). See
[`docs/auth.md`](docs/auth.md) §4 for the full security hardening and the
reverse-proxy note.

### Backups

Scheduled snapshots are on by default: one `VACUUM INTO` copy every 24h into
`<DB dir>/backups`, keeping the newest 7. Tune with `BACKUP_DIR`,
`BACKUP_INTERVAL` (e.g. `6h`) and `BACKUP_RETENTION`. The manual
`POST /admin/backup` and `POST /admin/restore-drill` routes remain available.

### Docker

```bash
docker compose up -d          # app + named volume for /app/data
```

Or without compose, mount a volume so the SQLite file survives redeploys:

```bash
docker build -t varels-cms .
docker run -p 8080:8080 --env-file .env -v varels-data:/app/data varels-cms
```

See `Dockerfile` for the container build.
