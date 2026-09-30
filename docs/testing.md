# Testing — varels-cms

The binding strategy for verifying the money and inventory paths, and the
ordered plan for closing the remaining gaps. Read this with `docs/overview.md`
§7 ("Non-negotiable principles"): **financial truth first** — a wrong number is
worse than a missing feature, so the riskiest code (FIFO COGS, atomic sales,
store credit, ledger integrity) is the first to be pinned by tests.

Run everything with:

```bash
go test ./...
# or
make test
```

---

## 1. Why tests come before more frontend

- The route/handler surface is complete (`86 routes`, `ai-tracking.md` §2) but
  is only "done in theory" until the money math is pinned.
- The frontend interactivity layer is already wired (HTMX + Alpine); what
  remains is polish, not missing capability.
- Therefore: characterize the backend **first**, then convert the last
  non-HTMX forms, then add the scheduled email digest.

---

## 2. Ordered plan (execution order)

| # | Order                              | Scope                                                                                                     | Status        |
| - | ---------------------------------- | --------------------------------------------------------------------------------------------------------- | ------------- |
| 1 | Money-path integration tests       | Sale (FIFO cost, stock decrement, payment), partial return + restock at original cost, store credit, oversell rollback | ✅ Done       |
| 2 | Ledger invariant tests             | `stock_levels == Σ stock_movements` per (variant, location, state); append-only movements                  | ✅ Done       |
| 3 | Frontend polish                    | Convert restock/adjust forms to HTMX fragments; replace remaining `?err=` redirects with flash/OOB toasts  | ⏳ Next       |
| 4 | Low-stock email digest             | Scheduled job (the only unimplemented feature)                                                             | ⏳ Later      |

Orders 3 and 4 are deliberately after 1–2: they are low-risk and can land on a
verified financial core.

---

## 3. Test harness

Tests live next to the handlers in package `handlers`, in files named
`*_test.go`, so they can call unexported handler methods directly.

- `internal/handlers/tests/testenv_test.go` — `newTestEnv(t)`:
  1. opens a fresh SQLite DB in `t.TempDir()` via `db.Open`,
  2. runs `db.Migrate` + `db.Seed` (dev fixtures: one product, two variants —
     `TEE-BLK-M` 8 units, `TEE-BLK-L` 5 units — at the first active location),
  3. wires the real handlers behind a `chi` router,
  4. exposes helpers (`post`, `get`, `sell`, `addLayer`, `scalarInt`,
     `sellableStock`, `stateStock`, …).

Deliberate choices:

- **No auth or CSRF middleware** in the harness. These tests exercise business
  logic, not the auth surface; the auth flow has its own live smoke checks in
  `implementation.md`. `Server.Sessions` is left `nil`, so flashes are no-ops.
- **Real DB, real handlers.** No mocks: the stock trigger, CHECK constraints,
  and transaction rollback are exactly what is under test.
- **Deterministic FIFO.** Cost layers are inserted with explicit `received_at`
  timestamps (`addLayer`) so ordering never depends on insert speed.

---

## 4. Coverage matrix

| Test                                          | Proves                                                                 |
| --------------------------------------------- | ---------------------------------------------------------------------- |
| `TestSale_FIFOStockAndPayment`                | FIFO average cost (3@400000 + 2@600000 → 480000), stock decrement, captured payment, layers fully consumed (ADR-0012, ADR-0019) |
| `TestSale_OrderDiscountAndShipping`           | `total = subtotal − discount + shipping` in integer minor units (ADR-0005) |
| `TestSale_MultiLineSinglePayment`             | Multi-line order, one payment for the sum, both stocks decremented     |
| `TestVoid_RestocksAndCompensates`             | Void appends compensation, restocks, marks cancelled/refunded — never deletes (ADR-0010) |
| `TestReturn_PartialRestoresStockAndOriginalCost` | Partial return restocks at the line's original cost; order/payment become partially returned/refunded (ADR-0006) |
| `TestReturn_DamagedGoesToDamagedState`        | Damaged returns restock into `damaged`, not `sellable`                 |
| `TestReturn_CannotOverReturn`                 | Returning more than remains is rejected with no writes                 |
| `TestStoreCredit_IssueRedeemOverspend`        | Issue on store_credit return, redeem on a later sale, over-spend rejected with a full rollback (ADR-0020) |
| `TestStoreCredit_RequiresCustomer`            | Store-credit payment without a customer is rejected                    |
| `TestOversell_RollsBackEverything`            | Oversell aborts the whole transaction: no order/movement/payment       |
| `TestOversell_MultiLineIsAtomic`              | A valid line is not committed when a later line oversells              |
| `TestLedger_StockLevelsEqualMovements`        | `stock_levels == Σ movements` after restock→sale→return→void; existing movement ids survive (ADR-0017) |
| `TestLedger_AdjustmentRequiresReason`         | Adjustment without a reason code is rejected; with one it decrements stock |

---

## 5. When adding tests

- Add to the harness, don't fork it. A new scenario is a `newTestEnv(t)` plus
  handler calls.
- Assert on **DB state and status codes**, not rendered HTML, unless the test is
  specifically about a partial/HTMX response.
- Every money or stock test should end with `e.assertStockLevelsMatchLedger()`
  when it has mutated inventory.
- Keep the coverage matrix above in sync.

---

## 6. Not yet covered

- Auth/OAuth/session lifecycle (currently verified by live smoke checks only).
- Analytics / P&L math and CSV export contents.
- Purchase-order receiving transaction.
- HTMX fragment/response contract (order 3 will touch this).
