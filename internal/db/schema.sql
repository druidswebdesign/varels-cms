-- sqlc schema input for varels-cms.
--
-- This file MUST mirror the applied goose migrations
-- (internal/db/migrations/0001_init.up.sql and 0002_users.up.sql). Keep the two
-- in sync when adding tables so generated structs match the live database.
-- It contains DDL only (no goose directives and no seed data).

-- ---------------------------------------------------------------------------
-- Catalog
-- ---------------------------------------------------------------------------

CREATE TABLE categories (
    id         INTEGER PRIMARY KEY,
    name       TEXT    NOT NULL UNIQUE,
    slug       TEXT    NOT NULL UNIQUE,
    parent_id  INTEGER REFERENCES categories(id),
    sort_order INTEGER NOT NULL DEFAULT 0
);

CREATE TABLE collections (
    id          INTEGER PRIMARY KEY,
    name        TEXT    NOT NULL UNIQUE,
    slug        TEXT    NOT NULL UNIQUE,
    kind        TEXT    NOT NULL CHECK (kind IN ('drop','season','essentials')),
    season      TEXT,
    launch_date TEXT,
    is_archived INTEGER NOT NULL DEFAULT 0
);

CREATE TABLE products (
    id          INTEGER PRIMARY KEY,
    name        TEXT    NOT NULL,
    slug        TEXT    NOT NULL UNIQUE,
    description TEXT,
    category_id INTEGER REFERENCES categories(id),
    is_archived INTEGER NOT NULL DEFAULT 0,
    archived_at TEXT,
    created_at  TEXT    NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%SZ','now')),
    updated_at  TEXT    NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%SZ','now'))
);

CREATE TABLE product_collections (
    product_id    INTEGER NOT NULL REFERENCES products(id),
    collection_id INTEGER NOT NULL REFERENCES collections(id),
    PRIMARY KEY (product_id, collection_id)
);

CREATE TABLE product_variants (
    id                    INTEGER PRIMARY KEY,
    product_id            INTEGER NOT NULL REFERENCES products(id),
    sku                   TEXT    NOT NULL UNIQUE,
    size                  TEXT,
    color                 TEXT,
    barcode               TEXT,
    cost_minor            INTEGER NOT NULL CHECK (cost_minor >= 0),
    retail_price_minor    INTEGER NOT NULL CHECK (retail_price_minor >= 0),
    wholesale_price_minor INTEGER CHECK (wholesale_price_minor >= 0),
    low_stock_threshold   INTEGER NOT NULL DEFAULT 0,
    is_archived           INTEGER NOT NULL DEFAULT 0,
    created_at            TEXT    NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%SZ','now')),
    updated_at            TEXT    NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%SZ','now'))
);

CREATE TABLE product_media (
    id         INTEGER PRIMARY KEY,
    product_id INTEGER NOT NULL REFERENCES products(id),
    variant_id INTEGER REFERENCES product_variants(id),
    media_type TEXT    NOT NULL CHECK (media_type IN ('image','size_chart','video')),
    url        TEXT    NOT NULL,
    alt_text   TEXT,
    sort_order INTEGER NOT NULL DEFAULT 0,
    is_primary INTEGER NOT NULL DEFAULT 0
);

CREATE TABLE cost_history (
    id             INTEGER PRIMARY KEY,
    variant_id     INTEGER NOT NULL REFERENCES product_variants(id),
    cost_minor     INTEGER NOT NULL CHECK (cost_minor >= 0),
    effective_from TEXT    NOT NULL,
    source         TEXT    NOT NULL CHECK (source IN ('manual','po_receipt')),
    note           TEXT,
    created_by     INTEGER REFERENCES approved_users(id),
    created_at     TEXT    NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%SZ','now'))
);

CREATE TABLE cost_layers (
    id              INTEGER PRIMARY KEY,
    variant_id      INTEGER NOT NULL REFERENCES product_variants(id),
    received_at     TEXT    NOT NULL,
    qty_received    INTEGER NOT NULL CHECK (qty_received > 0),
    qty_remaining   INTEGER NOT NULL CHECK (qty_remaining >= 0),
    unit_cost_minor INTEGER NOT NULL CHECK (unit_cost_minor >= 0),
    source_ref      TEXT,
    created_at      TEXT    NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%SZ','now'))
);

CREATE TABLE price_tiers (
    id          INTEGER PRIMARY KEY,
    variant_id  INTEGER NOT NULL REFERENCES product_variants(id),
    min_qty     INTEGER NOT NULL CHECK (min_qty > 0),
    price_minor INTEGER NOT NULL CHECK (price_minor >= 0),
    label       TEXT
);

CREATE TABLE discounts (
    id         INTEGER PRIMARY KEY,
    name       TEXT    NOT NULL,
    scope      TEXT    NOT NULL CHECK (scope IN ('product','variant','collection','order')),
    scope_id   INTEGER,
    type       TEXT    NOT NULL CHECK (type IN ('percent','amount')),
    value      INTEGER NOT NULL CHECK (value > 0),
    starts_at  TEXT,
    ends_at    TEXT,
    is_active  INTEGER NOT NULL DEFAULT 1,
    created_by INTEGER REFERENCES approved_users(id),
    created_at TEXT    NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%SZ','now'))
);

-- ---------------------------------------------------------------------------
-- Locations, channels & inventory
-- ---------------------------------------------------------------------------

CREATE TABLE locations (
    id        INTEGER PRIMARY KEY,
    name      TEXT    NOT NULL UNIQUE,
    kind      TEXT    NOT NULL CHECK (kind IN ('warehouse','storefront')),
    address   TEXT,
    is_active INTEGER NOT NULL DEFAULT 1
);

CREATE TABLE sales_channels (
    id        INTEGER PRIMARY KEY,
    name      TEXT    NOT NULL UNIQUE,
    is_active INTEGER NOT NULL DEFAULT 1
);

CREATE TABLE stock_levels (
    id          INTEGER PRIMARY KEY,
    variant_id  INTEGER NOT NULL REFERENCES product_variants(id),
    location_id INTEGER NOT NULL REFERENCES locations(id),
    state       TEXT    NOT NULL CHECK (state IN ('sellable','reserved','damaged','sample','gift','personal')),
    quantity    INTEGER NOT NULL DEFAULT 0 CHECK (quantity >= 0),
    updated_at  TEXT    NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%SZ','now')),
    UNIQUE (variant_id, location_id, state)
);

CREATE TABLE reason_codes (
    code      TEXT    PRIMARY KEY,
    label     TEXT    NOT NULL,
    kind      TEXT    NOT NULL CHECK (kind IN ('stock_adjustment','return')),
    is_active INTEGER NOT NULL DEFAULT 1
);

CREATE TABLE stock_movements (
    id             INTEGER PRIMARY KEY,
    variant_id     INTEGER NOT NULL REFERENCES product_variants(id),
    location_id    INTEGER NOT NULL REFERENCES locations(id),
    state          TEXT    NOT NULL CHECK (state IN ('sellable','reserved','damaged','sample','gift','personal')),
    quantity_delta INTEGER NOT NULL,
    reason_code    TEXT    REFERENCES reason_codes(code),
    ref_type       TEXT    CHECK (ref_type IN ('order','return','adjustment','po_receipt','transfer','hold')),
    ref_id         INTEGER,
    note           TEXT,
    created_by     INTEGER REFERENCES approved_users(id),
    created_at     TEXT    NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%SZ','now')),
    CHECK (ref_type <> 'adjustment' OR reason_code IS NOT NULL)
);

CREATE TABLE stock_holds (
    id          INTEGER PRIMARY KEY,
    variant_id  INTEGER NOT NULL REFERENCES product_variants(id),
    location_id INTEGER NOT NULL REFERENCES locations(id),
    quantity    INTEGER NOT NULL CHECK (quantity > 0),
    source      TEXT    NOT NULL CHECK (source IN ('cart','preorder','manual')),
    ref_id      TEXT,
    status      TEXT    NOT NULL CHECK (status IN ('active','released','consumed','expired')),
    expires_at  TEXT,
    created_at  TEXT    NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%SZ','now'))
);

-- ---------------------------------------------------------------------------
-- Suppliers & procurement (ADR-0011)
-- ---------------------------------------------------------------------------

CREATE TABLE suppliers (
    id             INTEGER PRIMARY KEY,
    name           TEXT    NOT NULL,
    contact_name   TEXT,
    email          TEXT,
    phone          TEXT,
    lead_time_days INTEGER,
    terms          TEXT,
    notes          TEXT,
    is_active      INTEGER NOT NULL DEFAULT 1,
    created_at     TEXT    NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%SZ','now'))
);

CREATE TABLE purchase_orders (
    id          INTEGER PRIMARY KEY,
    supplier_id INTEGER NOT NULL REFERENCES suppliers(id),
    location_id INTEGER NOT NULL REFERENCES locations(id),
    status      TEXT    NOT NULL CHECK (status IN ('draft','ordered','partial','received','cancelled')),
    ordered_at  TEXT,
    expected_at TEXT,
    received_at TEXT,
    notes       TEXT,
    created_by  INTEGER REFERENCES approved_users(id),
    created_at  TEXT    NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%SZ','now'))
);

CREATE TABLE purchase_order_items (
    id                INTEGER PRIMARY KEY,
    purchase_order_id INTEGER NOT NULL REFERENCES purchase_orders(id),
    variant_id        INTEGER NOT NULL REFERENCES product_variants(id),
    quantity_ordered  INTEGER NOT NULL DEFAULT 0 CHECK (quantity_ordered >= 0),
    quantity_received INTEGER NOT NULL DEFAULT 0 CHECK (quantity_received >= 0),
    unit_cost_minor   INTEGER NOT NULL CHECK (unit_cost_minor >= 0),
    line_total_minor  INTEGER NOT NULL DEFAULT 0 CHECK (line_total_minor >= 0),
    UNIQUE (purchase_order_id, variant_id)
);

-- ---------------------------------------------------------------------------
-- Customers
-- ---------------------------------------------------------------------------

CREATE TABLE customers (
    id         INTEGER PRIMARY KEY,
    name       TEXT    NOT NULL,
    phone      TEXT,
    email      TEXT,
    instagram  TEXT,
    notes      TEXT,
    created_at TEXT    NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%SZ','now')),
    updated_at TEXT    NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%SZ','now'))
);

-- ---------------------------------------------------------------------------
-- Orders, sales, returns (ADR-0006)
-- ---------------------------------------------------------------------------

CREATE TABLE orders (
    id             INTEGER PRIMARY KEY,
    order_number   TEXT    NOT NULL UNIQUE,
    customer_id    INTEGER REFERENCES customers(id),
    channel_id     INTEGER NOT NULL REFERENCES sales_channels(id),
    location_id    INTEGER NOT NULL REFERENCES locations(id),
    status         TEXT    NOT NULL CHECK (status IN ('draft','completed','partially_returned','returned','cancelled')),
    subtotal_minor INTEGER NOT NULL DEFAULT 0,
    discount_minor INTEGER NOT NULL DEFAULT 0,
    tax_minor      INTEGER NOT NULL DEFAULT 0,
    shipping_minor INTEGER NOT NULL DEFAULT 0,
    total_minor    INTEGER NOT NULL DEFAULT 0,
    payment_status TEXT    NOT NULL CHECK (payment_status IN ('unpaid','paid','partially_refunded','refunded')),
    placed_at      TEXT    NOT NULL,
    note           TEXT,
    created_by     INTEGER REFERENCES approved_users(id),
    created_at     TEXT    NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%SZ','now')),
    updated_at     TEXT    NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%SZ','now'))
);

CREATE TABLE order_items (
    id               INTEGER PRIMARY KEY,
    order_id         INTEGER NOT NULL REFERENCES orders(id),
    variant_id       INTEGER NOT NULL REFERENCES product_variants(id),
    quantity         INTEGER NOT NULL CHECK (quantity > 0),
    unit_price_minor INTEGER NOT NULL CHECK (unit_price_minor >= 0),
    unit_cost_minor  INTEGER NOT NULL CHECK (unit_cost_minor >= 0),
    discount_minor   INTEGER NOT NULL DEFAULT 0,
    line_total_minor INTEGER NOT NULL
);

CREATE TABLE payments (
    id           INTEGER PRIMARY KEY,
    order_id     INTEGER NOT NULL REFERENCES orders(id),
    method       TEXT    NOT NULL CHECK (method IN ('cash','card','mercadopago','transfer','store_credit')),
    amount_minor INTEGER NOT NULL,
    status       TEXT    NOT NULL CHECK (status IN ('captured','refunded','partial_refund')),
    reference    TEXT,
    paid_at      TEXT    NOT NULL,
    created_by   INTEGER REFERENCES approved_users(id)
);

CREATE TABLE returns (
    id                 INTEGER PRIMARY KEY,
    order_id           INTEGER NOT NULL REFERENCES orders(id),
    customer_id        INTEGER REFERENCES customers(id),
    resolution         TEXT    NOT NULL CHECK (resolution IN ('refund','store_credit','exchange')),
    status             TEXT    NOT NULL CHECK (status IN ('pending','completed')),
    total_refund_minor INTEGER NOT NULL DEFAULT 0,
    note               TEXT,
    processed_at       TEXT,
    created_by         INTEGER REFERENCES approved_users(id),
    created_at         TEXT    NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%SZ','now'))
);

CREATE TABLE return_items (
    id                  INTEGER PRIMARY KEY,
    return_id           INTEGER NOT NULL REFERENCES returns(id),
    order_item_id       INTEGER NOT NULL REFERENCES order_items(id),
    quantity            INTEGER NOT NULL CHECK (quantity > 0),
    condition_state     TEXT    NOT NULL CHECK (condition_state IN ('sellable','damaged')),
    resolution          TEXT    NOT NULL CHECK (resolution IN ('refund','store_credit','exchange')),
    refund_amount_minor INTEGER NOT NULL DEFAULT 0,
    reason_code         TEXT    NOT NULL REFERENCES reason_codes(code),
    created_at          TEXT    NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%SZ','now'))
);

-- ---------------------------------------------------------------------------
-- Finance / OpEx (ADR-0007)
-- ---------------------------------------------------------------------------

CREATE TABLE expense_categories (
    id        INTEGER PRIMARY KEY,
    name      TEXT    NOT NULL UNIQUE,
    is_active INTEGER NOT NULL DEFAULT 1
);

CREATE TABLE expenses (
    id           INTEGER PRIMARY KEY,
    category_id  INTEGER NOT NULL REFERENCES expense_categories(id),
    vendor       TEXT,
    amount_minor INTEGER NOT NULL CHECK (amount_minor >= 0),
    incurred_at  TEXT    NOT NULL,
    note         TEXT,
    created_by   INTEGER REFERENCES approved_users(id),
    created_at   TEXT    NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%SZ','now'))
);

-- ---------------------------------------------------------------------------
-- System & audit (ADR-0010)
-- ---------------------------------------------------------------------------

CREATE TABLE audit_logs (
    id          INTEGER PRIMARY KEY,
    user_id     INTEGER REFERENCES approved_users(id),
    action      TEXT    NOT NULL,
    entity_type TEXT    NOT NULL,
    entity_id   INTEGER,
    before_json TEXT,
    after_json  TEXT,
    note        TEXT,
    ip          TEXT,
    created_at  TEXT    NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%SZ','now'))
);

CREATE TABLE settings (
    key        TEXT    PRIMARY KEY,
    value      TEXT    NOT NULL,
    updated_at TEXT    NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%SZ','now')),
    updated_by INTEGER REFERENCES approved_users(id)
);

-- ---------------------------------------------------------------------------
-- Auth (migration 0002)
-- ---------------------------------------------------------------------------

CREATE TABLE approved_users (
    id            INTEGER PRIMARY KEY,
    email         TEXT    NOT NULL UNIQUE,
    display_name  TEXT,
    role          TEXT    NOT NULL CHECK (role IN ('admin','staff')),
    provider      TEXT    NOT NULL DEFAULT 'google' CHECK (provider IN ('google','local')),
    password_hash TEXT,
    is_active     INTEGER NOT NULL DEFAULT 1,
    invited_by    INTEGER REFERENCES approved_users(id),
    last_login_at TEXT,
    created_at    TEXT    NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%SZ','now')),
    updated_at    TEXT    NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%SZ','now')),
    CHECK (email = lower(email)),
    CHECK (provider <> 'local' OR password_hash IS NOT NULL)
);

CREATE TABLE sessions (
    token      TEXT    PRIMARY KEY,
    user_id    INTEGER REFERENCES approved_users(id),
    data       BLOB,
    expires_at TEXT    NOT NULL,
    created_at TEXT    NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%SZ','now'))
);

-- ---------------------------------------------------------------------------
-- Current stock is owned by the ledger (ADR-0017)
-- ---------------------------------------------------------------------------

CREATE TRIGGER stock_movements_after_insert
AFTER INSERT ON stock_movements
BEGIN
    INSERT OR IGNORE INTO stock_levels (variant_id, location_id, state, quantity, updated_at)
    VALUES (NEW.variant_id, NEW.location_id, NEW.state, 0,
            strftime('%Y-%m-%dT%H:%M:%SZ','now'));

    UPDATE stock_levels
    SET quantity   = quantity + NEW.quantity_delta,
        updated_at = strftime('%Y-%m-%dT%H:%M:%SZ','now')
    WHERE variant_id  = NEW.variant_id
      AND location_id = NEW.location_id
      AND state       = NEW.state;
END;

-- ---------------------------------------------------------------------------
-- Indexes
-- ---------------------------------------------------------------------------

CREATE INDEX idx_product_variants_product_id ON product_variants(product_id);
CREATE INDEX idx_product_media_product ON product_media(product_id, sort_order);
CREATE INDEX idx_product_collections_collection_id ON product_collections(collection_id);
CREATE INDEX idx_cost_history_variant ON cost_history(variant_id, effective_from);
CREATE INDEX idx_cost_layers_variant_fifo ON cost_layers(variant_id, received_at);
CREATE INDEX idx_price_tiers_variant_min_qty ON price_tiers(variant_id, min_qty);
CREATE INDEX idx_discounts_scope ON discounts(scope, scope_id, is_active);

CREATE INDEX idx_stock_levels_location_state ON stock_levels(location_id, state);
CREATE INDEX idx_stock_movements_variant_created ON stock_movements(variant_id, created_at);
CREATE INDEX idx_stock_movements_ref ON stock_movements(ref_type, ref_id);
CREATE INDEX idx_stock_holds_variant_status ON stock_holds(variant_id, status);
CREATE INDEX idx_stock_holds_expires_at ON stock_holds(expires_at);

CREATE INDEX idx_orders_placed_at ON orders(placed_at);
CREATE INDEX idx_orders_customer_id ON orders(customer_id);
CREATE INDEX idx_orders_channel_placed ON orders(channel_id, placed_at);
CREATE INDEX idx_orders_location_placed ON orders(location_id, placed_at);
CREATE INDEX idx_order_items_order_id ON order_items(order_id);
CREATE INDEX idx_order_items_variant_id ON order_items(variant_id);
CREATE INDEX idx_payments_order_id ON payments(order_id);
CREATE INDEX idx_returns_order_id ON returns(order_id);
CREATE INDEX idx_return_items_return_id ON return_items(return_id);
CREATE INDEX idx_return_items_order_item_id ON return_items(order_item_id);
CREATE INDEX idx_expenses_incurred_category ON expenses(incurred_at, category_id);
CREATE INDEX idx_audit_logs_entity ON audit_logs(entity_type, entity_id);
CREATE INDEX idx_audit_logs_user_created ON audit_logs(user_id, created_at);
CREATE INDEX idx_suppliers_name ON suppliers(name);
CREATE INDEX idx_purchase_orders_supplier_status ON purchase_orders(supplier_id, status);
CREATE INDEX idx_sessions_expires_at ON sessions(expires_at);

CREATE TABLE store_credit_ledger (
    id           INTEGER PRIMARY KEY,
    customer_id  INTEGER NOT NULL REFERENCES customers(id),
    delta_minor  INTEGER NOT NULL,
    reason       TEXT    NOT NULL CHECK (reason IN ('issued','redeemed')),
    order_id     INTEGER REFERENCES orders(id),
    return_id    INTEGER REFERENCES returns(id),
    note         TEXT,
    created_by   INTEGER REFERENCES approved_users(id),
    created_at   TEXT    NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%SZ','now'))
);

CREATE INDEX idx_store_credit_customer ON store_credit_ledger(customer_id, created_at);
