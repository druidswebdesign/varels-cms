# Spec-Driven Development with AI — varels-cms

In Spec-Driven Development (SDD) with AI, front-loading context is everything. You
have your **What** (functions/features, `docs/functions.md`) and your **How**
(stack: Go, SQLite, HTMX, templ).

If you just tell the AI "build this" right now, it will hallucinate the database,
put all the code in one massive `main.go`, and break context limits.

Below are the exact documents to write next to guide the AI successfully, ordered
by priority.

---

## 1. Database schema — `docs/schema.md`

**Why:** the AI needs an exact blueprint of your data. Without defined tables, the
Go structs won't match the database and the HTMX templates won't match the Go
structs.

**What to write:**

- List every SQL table.
- Define the foreign-key relationships (e.g. `order_items` belongs to `orders`
  and `variants`).
- **Crucial for this app:** write down the money rules explicitly (e.g. "all
  prices are ARS stored as `INTEGER`; `buy_price INTEGER NOT NULL`").
- Include a concrete example:

  ```text
  Table: variants
  - id (UUID, PK)
  - product_id (UUID, FK -> products.id)
  - size (VARCHAR - S, M, L, XL)
  - color (VARCHAR)
  - sku (VARCHAR, UNIQUE)
  - sellable_stock (INTEGER)
  ```

## 2. URL routes & UI flow — `docs/routes.md`

**Why:** with HTMX, the Go server is your frontend router. The AI must know which
URLs render full pages and which return small HTMX HTML fragments.

**What to write:**

- **Full pages:** `GET /dashboard`, `GET /inventory`, `GET /pos` (point of sale).
- **HTMX fragments:** `POST /inventory/restock/{id}` (returns an updated table
  row), `GET /analytics/margin?month=11` (returns just the chart/number).
- Define error behavior (e.g. "if stock is insufficient, return an HTMX fragment
  with a red error toast").

## 3. Project structure — `docs/architecture.md`

**Why:** Go is opinionated, but AI can be lazy. Tell it exactly where files go so
the project stays maintainable.

**What to write:**

- Tell the AI to use the Standard Go Project Layout.
- Specify where SQLite queries, templ files, and HTTP handlers live.
- Example text for the AI:

  ```text
  /cmd/server/main.go       (app entry point)
  /internal/database/       (SQLite queries, structs)
  /internal/handlers/       (HTTP routes)
  /internal/views/          (templ component files)
  /static/                  (CSS, JS, images)
  ```

## 4. Implementation milestones — `docs/milestones.md`

**Why:** LLMs have output limits and lose focus on massive tasks. You cannot ask
for the whole app at once. Write a step-by-step build plan and prompt one
milestone at a time.

**What to write:**

- **Phase 1 — Database foundation.** _Prompt:_ "Read `schema.md` and generate the
  SQLite migration files and basic Go structs. Do nothing else."
- **Phase 2 — Core inventory CRUD.** _Prompt:_ "Build the Go handlers and templ
  views to add products and variants."
- **Phase 3 — Checkout & orders.** _Prompt:_ "Build the order creation logic.
  Ensure stock is deducted using SQL transactions."
- **Phase 4 — Financial analytics.** _Prompt:_ "Build the P&L math and dashboard
  widgets."

> In this repo the milestone plan lives inside `docs/implementation.md` rather
> than a separate `milestones.md`.

## 5. System prompt / coding rules — `docs/rules.md`

**Why:** to stop the AI using outdated libraries, writing insecure code, or
ignoring the tech-stack decisions.

**What to write:**

- "No JavaScript frameworks (React/Vue). Use purely HTMX."
- "Always use SQLite WAL mode and strict foreign keys."
- "Use Go `context.Context` for all database calls."
- "Never use floats for money — integers only."
- "Return semantic HTML via templ, not JSON, for all standard routes."

> The binding rules for this repo live in `adr.md` and `docs/overview.md` §7
> ("Non-negotiable principles").

---

## 6. How to execute this

Once these Markdown files are in `/docs`, open an AI coding assistant (Cursor,
Windsurf, or GitHub Copilot Workspace), point it at the docs folder, and say:

> "I am building a retail management system for a clothing brand. Please read all
> files in `/docs`. We will build this step by step. Acknowledge that you
> understand the architecture, data model, and business logic. Once you
> acknowledge, I will give you the prompt for Milestone 1."

By doing this, the AI becomes a senior developer executing your exact vision,
rather than a junior developer guessing how a clothing brand's finances work.
