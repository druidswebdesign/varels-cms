# Functions & Product Specification — varels_cms

> Functional spec for the CMS: the system rules, the calculations, and every
> user-facing capability grouped by domain. See `docs/overview.md` for context and
> `docs/schema.md` for how each function maps to tables.

---

## 0. Core system rules

Before any function runs, these financial and architectural rules govern the
system:

- **Money precision (Argentine pesos, ARS).** All money is stored as an `INTEGER`
  representing the smallest currency unit, avoiding floating-point rounding
  errors. Currency formatting is globally set to ARS.
- **Locations & channels.** Every piece of stock sits in a specific **Location**
  (Warehouse, Storefront). Every sale happens via a **Sales Channel** (In-Store,
  Web, Wholesale, Pop-up).
- **Stock states.** Stock is strictly separated into **Sellable** and
  **Non-Sellable** (Damaged, Sample, Gift, Reserved).

---

## 1. Pricing, tiers & cost setup

Define and save financial data for each garment:

- **Set buy price / COGS.** Saves the exact cost to produce or purchase one unit
  (manufacturing, fabric, shipping-in).
- **Set retail / resell price.** Saves the standard price the customer pays.
- **Set wholesale tiers.** Special pricing for B2B resellers (e.g. buying in bulk
  to sell in their own shops).
- **Apply markdowns / discounts.** Applies a temporary or permanent discount to an
  item or collection, tracking the exact impact on margin.
- **Update cost history.** Change the buy price when a factory raises its rates,
  while keeping the historical cost of older stock intact (FIFO or average cost
  basis).

## 2. Single-item margin calculations

Run automatic math whenever an item is viewed or sold:

- **Calculate unit gross profit (ARS).** Resell price minus buy price
  (e.g. `$30,000 ARS` resell − `$10,000 ARS` buy = `$20,000 ARS` gross profit).
- **Calculate unit gross margin (%).** The share of the final sale price that is
  gross profit.
  - _Formula:_ `(gross profit / resell price) × 100`.
- **Calculate margin impact of discount.** How a 20% sale reduces the gross margin
  percentage.
- **Calculate markup (%).** How many times over the original cost you are pricing
  the item.

## 3. Combined inventory valuation

Look across locations for a financial snapshot of stock:

- **Calculate total inventory cost.** Each item's buy price × sellable stock count
  (how much cash is sitting on your shelves).
- **Calculate total potential retail value.** Resell price × current sellable
  stock.
- **Evaluate non-sellable asset value.** The lost cost tied up in damaged, sample,
  or personal items.

## 4. Realized financial analytics & bookkeeping (new P&L)

Calculate real profit over time — beyond simple margins, into actual business
accounting:

- **Calculate total realized revenue.** All cash brought in from orders across all
  channels.
- **Calculate total cost of goods sold (COGS).** The exact buy prices of the
  specific items that left the door.
- **Track operational expenses (OpEx).** Logs business expenses (rent, salaries,
  packaging, shipping-out, payment-gateway fees).
- **Generate real P&L (profit & loss).**
  - `revenue − COGS = gross profit`
  - `gross profit − OpEx = net cash profit` (the money the brand actually made)
- **Calculate brand gross margin % vs net profit %.** Displays the difference
  between the margin on your clothes and the profit of the actual business.

---

## 5. Product & order functions

### 5.1 Products, variants, media & collections

Clothing is unique, visual, and seasonal.

- **Create new product.** Adds the main item with descriptions and tags.
- **Manage product media.** Uploads and links photos, galleries, variant-specific
  images (red shirt vs. blue shirt), and size charts.
- **Create / update variants.** Assigns specific sizes (S, M, L) and colors, each
  with a unique SKU.
- **Assign to collections / seasons.** Groups items into drops (e.g.
  "Winter '24 Drop", "Essentials") — how clothing brands actually report and sell.
- **Archive / delete item.** Hides an item without breaking historical order data.

### 5.2 Suppliers, purchase orders & restocking

Track relationships with factories and incoming stock.

- **Manage suppliers.** Saves factory/supplier contact details, standard lead
  times, and terms.
- **Create purchase order (PO).** Logs a request to a supplier for new stock
  (e.g. ordering 500 tees).
- **Track "on-order" quantity.** Shows how many units are being manufactured or
  shipped to you, preventing panic re-ordering.
- **Receive PO (restock).** Moves items from "on-order" to physical inventory at a
  specific location, logging the date and incoming cost.
- **Adjust stock manually (with reason codes).** Corrects inventory using a strict
  taxonomy (e.g. −1 for "Damaged", −1 for "Sent as PR/Gift", −1 for
  "Shrinkage/Stolen").

### 5.3 Customer & loyalty management

Without this, returns and client history are impossible.

- **Create / update customer profile.** Stores lightweight info (name, phone,
  email, Instagram handle).
- **View customer purchase history.** Shows lifetime value (LTV), past orders, and
  typical sizing (great for VIP clienteling).

### 5.4 Orders, cart & returns

Sales are transactions, not single-item decrements.

- **Create order (checkout):**
  - Links to a customer.
  - Records the sales channel (Web, Store) and location (Warehouse A).
  - Adds multiple order line items (e.g. 2 shirts, 1 hat).
  - Applies any order-level discounts.
  - Logs the payment method (Cash, Card, MercadoPago) and creates a receipt.
- **Reserve / hold stock.** Temporarily locks inventory when an item is in a web
  cart or allocated for a pre-order, preventing overselling during a high-traffic
  "drop".
- **Process return / exchange:**
  - Handles full or partial returns (returning 1 item from a 3-item order).
  - Requires a return reason code (wrong size, defective, changed mind).
  - Requires a condition state (back to sellable vs. damaged).
  - Processes resolution (refund vs. store credit vs. size exchange).

### 5.5 Alerts & notifications

- **Set thresholds.** Sets warning levels per variant.
- **Generate daily low-stock digest.** Sends an automated email to the owner
  summarizing items to reorder, referencing supplier lead times.

### 5.6 Advanced reports & analytics

- **Get best-sellers by channel/location.** Reveals whether items sell better on
  the Web vs. in-store.
- **Get best-selling sizes/colors.** Dictates your sizing curve for the next
  purchase order.
- **Calculate inventory turnover.** Shows how fast you sell through stock (flags
  over-ordering).
- **Calculate trapped cash & clearance candidates.** Identifies items with zero
  sales in 60+ days and suggests markdowns to free up ARS.

---

## 6. System, web & ops functions

### 6.1 Database & backup operations (high-risk mitigation)

The system holds the financial truth of the business, so SQLite must be
bulletproofed:

- **Enable WAL mode.** Lets the database handle multiple web reads and writes
  (an employee updating stock while a customer views the site) without locking.
- **Schedule automated backups.** Dumps a copy of the SQLite database nightly to a
  secure location.
- **Trigger manual restore drill.** Tests recovering the database from a backup
  file.
- **Enforce immutable audit logs.** All financial and stock transactions are
  write-only. Mistakes are corrected via an "Adjustment" row, never by deleting
  the original history.

### 6.2 Web & UI handlers (HTMX & templ)

- **Render dashboard / P&L page.** Loads top-level financial health cards and
  channel breakdowns.
- **Serve search/filter requests (HTMX).** Filters orders, customers, or drops
  dynamically without a page reload.
- **Process checkout / POS form (HTMX).** Submits the multi-item order, processes
  the stock reservation, commits the transaction, and returns the receipt view.
- **Export financials.** Generates CSV/Excel exports of the P&L, inventory
  valuation, and tax-ready transaction logs for the bookkeeper.

### 6.3 Role permissions & audit trails

- **Log system activity.** Records who did what (e.g. "Employee Alex processed a
  return for Order #1043 on Tuesday").
- **Enforce role access.** Hides OpEx, buy prices, net profit, and supplier
  contacts from `staff`, restricting them to `admin` (ADR-0018).
