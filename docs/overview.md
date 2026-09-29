# varels_cms — Project Overview

The single narrative entry point for the project: what it is, why it exists, who
it serves, and what is deliberately out of scope. Read this first, then the
sibling specs in §10 for the _how_.

---

## 1. One-liner

`varels_cms` is a private, self-hosted **inventory and profit CMS for an
independent Argentine clothing brand** — it tracks every garment from purchase to
sale, tells the owner what stock exists and where, what it cost, what it earns,
and what is quietly trapping cash.

It runs as **one Go binary plus one SQLite file** on a single cheap server.

---

## 2. Main purpose

Give the owner **financial and inventory truth in real time**, without an ERP.

Concretely, the app exists to answer, at any moment:

- What do we own right now, in which size/color/location, and what did it cost?
- What actually sold, through which channel, and what was the real profit?
- What is _not_ selling, and how much cash is locked up in it?
- What do we need to reorder before we run out?
- Who changed the stock count, when, and why?

The business runs today on spreadsheets and memory. The gap this closes is not
"a website" — it is **trustworthy numbers** about physical goods and margins.

---

## 3. Who it is for

Invite-only. There is no public signup (`docs/auth.md`).

Only two roles can log in (ADR-0018):

| Role          | Cares about                                   | Sees financials?        |
| ------------- | --------------------------------------------- | ----------------------- |
| Admin / Owner | Everything — profit, P&L, suppliers, staff    | Yes                     |
| Staff         | Stock, sales, receiving, adjustments, lookups | No (cost/margin hidden) |

"Clients" and "providers" are **not** login roles: clients are `customers` and
providers are `suppliers`, both managed by staff inside the CMS.

Access is a Google OAuth whitelist (`approved_users`); offboarding = disabling one
row. A hidden `/admin-login` break-glass account exists so the owner is never
locked out if Google or the OAuth credentials fail (ADR-0004).

---

## 4. The problems it solves

1. **Stock truth.** One product, many size/color variants, spread across a
   warehouse and a storefront. Physical ≠ recorded is the normal state of a brand
   with no system.
2. **Margin vs profit confusion.** Owners usually quote _gross margin_ and call it
   profit, ignoring rent, salaries, packaging, shipping-out, and MercadoPago fees.
   This app separates gross margin from net profit and tracks OpEx (ADR-0007).
3. **Trapped cash / deadstock.** Money sitting in unsold stock that will never be
   recovered unless it is detected and marked down.
4. **Money precision.** Inflation currency and float math destroy financial
   records. All money is integer ARS (ADR-0005).
5. **No audit trail.** "Count says 10, shelf has 7." Every stock and financial
   change is attributed, timestamped, and append-only (ADR-0010).
6. **Return/exchange reality.** Returns, partial returns, damaged vs resellable,
   refunds vs store credit — modeled as real transactions, not a single decrement
   (ADR-0006).

---

## 5. What it does (capability map)

Derived from `docs/functions.md`, `docs/schema.md`, `docs/routes.md`.

- **Catalog** — products, variants (size/color/SKU), categories, collections/drops,
  product media and size charts, archive-instead-of-delete.
- **Pricing & cost** — retail (IVA-inclusive), wholesale tiers, markdowns,
  cost history, FIFO cost layers.
- **Inventory** — stock per variant × location × state (sellable, reserved,
  damaged, sample, gift, personal), an append-only movement ledger, restock, and
  manual adjustments with reason codes.
- **Procurement** _(schema in v1, UI later)_ — suppliers, purchase orders,
  on-order quantity, lead times, PO receiving.
- **Sales** — multi-line orders over a customer + channel + location, payments
  (cash/card/MercadoPago/transfer/store credit), receipts.
- **Returns & exchanges** — full/partial, with condition, reason code, and
  resolution.
- **Customers** — lightweight profile and purchase history.
- **Analytics** — best sellers (all-time/year/month), size/color curves, inventory
  turnover, deadstock and trapped cash, real P&L.
- **Alerts** — low-stock/out-of-stock thresholds, daily digest (scope TBD).
- **Ops** — CSV/Excel export of any table, immutable audit log, WAL + automated
  backups + restore drill.

---

## 6. What it is _not_ (explicit non-goals)

- **Not an e-commerce storefront.** Sales may arrive from a web channel, but this
  is the back office, not the shop.
- **Not accounting / tax software.** No AFIP e-invoicing, no ledger of account
  codes. It produces bookkeeping-grade exports for an accountant.
- **Not multi-tenant SaaS.** One brand, one database.
- **Not high-availability or horizontally scalable.** Deliberately one instance,
  one SQLite file (ADR-0015).
- **Not multi-currency.** ARS only (ADR-0005).
- **Not real-time collaborative** — no websockets, no live cursors; HTMX refreshes.

---

## 7. Non-negotiable principles

These are binding across every feature and every contributor (human or AI):

1. **Financial truth first.** A wrong number is worse than a missing feature.
2. **Integer money, ARS only.** No floats, ever (ADR-0005).
3. **Append-only history.** Stock and financial records are never edited or
   deleted; mistakes are corrected by compensating rows (ADR-0010).
4. **One source of truth per fact.** The stock ledger owns current stock; sqlc
   structs go straight to views with no DTO mapping (ADR-0001, ADR-0017).
5. **Financial data is role-gated.** Buy price, margin, net profit, OpEx, supplier
   terms are owner/admin-only.
6. **Argentina by default.** IVA-inclusive retail pricing (ADR-0013) and
   `America/Argentina/Buenos_Aires` for all day/month/year grouping (ADR-0014).
7. **Boring, simple deployment.** One binary, one file, one server.
8. **AI-readable, flat structure.** A feature lives in its "golden triangle":
   query → handler → templ.

---

## 8. Core workflows (the app in motion)

1. **Bootstrap** — first run seeds `INITIAL_OWNER_EMAIL` as the `admin` owner.
2. **Onboard stock** — existing physical stock entered as `opening_balance`
   adjustment movements; triggers populate current stock.
3. **A day of selling** — POS order: pick customer/channel/location, add variants
   as lines, take payment; stock movements post atomically; receipt printed.
4. **Restock** — receive a PO (or plain restock), a cost layer is created, stock
   increases at a location.
5. **Return** — partial return against an order line: choose reason, condition
   (resellable vs damaged), resolution (refund / store credit / exchange).
6. **Monthly review** — dashboard shows gross margin and net profit, best sellers,
   low-stock list, and deadstock with trapped cash.
7. **Correction** — a miscount is fixed by an adjustment with a reason code, never
   by editing history.
8. **Offboarding** — disable a staff member's whitelist row; access ends instantly.

---

## 9. Success criteria

- The owner can state, without a spreadsheet: current inventory value, this
  month's revenue, COGS, gross margin, net profit, top sellers, and trapped cash.
- Stock recorded equals stock on the shelf, and every divergence has an
  attributed reason.
- A sale, a return, and a restock never leave the ledger and current stock out of
  sync.
- The whole app deploys and backs up on a single low-cost host, and can be
  restored from backup in a documented drill.
- Onboarding a new feature means touching query + handler + templ, and the AI
  assistant has enough spec context to do it without guessing.

---

## 10. Where to go next (document map)

| Question                                 | Document                  |
| ---------------------------------------- | ------------------------- |
| What is this and why?                    | **this file**             |
| What features/functions must exist?      | `docs/functions.md`       |
| How is data modeled?                     | `docs/schema.md`          |
| What HTTP surface and UI flows?          | `docs/routes.md`          |
| What was decided and why?                | `adr.md`                  |
| Stack & structure                        | `docs/architecture.md`    |
| Auth model                               | `docs/auth.md`            |
| How we build it with AI                  | `docs/spec-driven-dev.md` |
| What exists now & how AI work is tracked | `docs/ai-tracking.md`     |
| How money/inventory paths are verified   | `docs/testing.md`         |

**Status:** implemented through analytics and the full route surface — auth,
catalog (products, variants, collections, media), inventory (restock,
adjustments, ledger, stock holds), sales/POS/payments/returns, customers,
suppliers & purchase orders, OpEx, P&L, deadstock and CSV exports, staff &
audit. The §9 open questions are resolved in ADR-0019..0021. Only the daily
low-stock email digest remains planned; see the end of `docs/implementation.md`.
