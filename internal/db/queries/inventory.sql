-- Inventory queries: locations, channels, stock levels/movements, reason codes,
-- holds, suppliers and purchase orders.

-- name: ListLocations :many
SELECT * FROM locations WHERE is_active = sqlc.arg(is_active) ORDER BY name;

-- name: ListAllLocations :many
SELECT * FROM locations ORDER BY name;

-- name: GetLocation :one
SELECT * FROM locations WHERE id = sqlc.arg(id);

-- name: CreateLocation :one
INSERT INTO locations (name, kind, address)
VALUES (sqlc.arg(name), sqlc.arg(kind), sqlc.narg(address))
RETURNING *;

-- name: UpdateLocation :exec
UPDATE locations
SET name = sqlc.arg(name),
    kind = sqlc.arg(kind),
    address = sqlc.narg(address),
    is_active = sqlc.arg(is_active)
WHERE id = sqlc.arg(id);

-- name: ListSalesChannels :many
SELECT * FROM sales_channels WHERE is_active = sqlc.arg(is_active) ORDER BY name;

-- name: CreateSalesChannel :one
INSERT INTO sales_channels (name) VALUES (sqlc.arg(name)) RETURNING *;

-- name: GetStockLevel :one
SELECT * FROM stock_levels
WHERE variant_id = sqlc.arg(variant_id)
  AND location_id = sqlc.arg(location_id)
  AND state = sqlc.arg(state);

-- name: ListStockByVariant :many
SELECT * FROM stock_levels
WHERE variant_id = sqlc.arg(variant_id)
ORDER BY location_id, state;

-- name: ListStockByLocation :many
SELECT * FROM stock_levels
WHERE location_id = sqlc.arg(location_id)
ORDER BY variant_id, state;

-- name: ListLowStock :many
SELECT sl.*
FROM stock_levels sl
JOIN product_variants pv ON pv.id = sl.variant_id
WHERE sl.state = 'sellable'
  AND pv.is_archived = 0
  AND sl.quantity <= pv.low_stock_threshold
ORDER BY sl.quantity, pv.sku;

-- name: TotalSellableByVariant :one
SELECT COALESCE(SUM(quantity), 0) AS quantity
FROM stock_levels
WHERE variant_id = sqlc.arg(variant_id) AND state = 'sellable';

-- name: CreateStockMovement :one
INSERT INTO stock_movements (
    variant_id, location_id, state, quantity_delta, reason_code,
    ref_type, ref_id, note, created_by
)
VALUES (
    sqlc.arg(variant_id),
    sqlc.arg(location_id),
    sqlc.arg(state),
    sqlc.arg(quantity_delta),
    sqlc.narg(reason_code),
    sqlc.narg(ref_type),
    sqlc.narg(ref_id),
    sqlc.narg(note),
    sqlc.narg(created_by)
)
RETURNING *;

-- name: ListStockMovements :many
SELECT * FROM stock_movements
WHERE (sqlc.narg(variant_id) IS NULL OR variant_id = sqlc.narg(variant_id))
  AND (sqlc.narg(location_id) IS NULL OR location_id = sqlc.narg(location_id))
  AND (sqlc.narg(ref_type) IS NULL OR ref_type = sqlc.narg(ref_type))
  AND (sqlc.narg(from_ts) IS NULL OR created_at >= sqlc.narg(from_ts))
  AND (sqlc.narg(to_ts) IS NULL OR created_at <= sqlc.narg(to_ts))
ORDER BY created_at DESC, id DESC
LIMIT sqlc.arg(limit) OFFSET sqlc.arg(offset);

-- name: ListRestocks :many
SELECT * FROM stock_movements
WHERE quantity_delta > 0 AND ref_type IN ('po_receipt', 'adjustment')
ORDER BY created_at DESC, id DESC
LIMIT sqlc.arg(limit) OFFSET sqlc.arg(offset);

-- name: ListReasonCodes :many
SELECT * FROM reason_codes
WHERE kind = sqlc.arg(kind) AND is_active = 1
ORDER BY label;

-- name: CreateReasonCode :exec
INSERT INTO reason_codes (code, label, kind)
VALUES (sqlc.arg(code), sqlc.arg(label), sqlc.arg(kind));

-- name: CreateStockHold :one
INSERT INTO stock_holds (variant_id, location_id, quantity, source, ref_id, status, expires_at)
VALUES (
    sqlc.arg(variant_id),
    sqlc.arg(location_id),
    sqlc.arg(quantity),
    sqlc.arg(source),
    sqlc.narg(ref_id),
    sqlc.arg(status),
    sqlc.narg(expires_at)
)
RETURNING *;

-- name: GetStockHold :one
SELECT * FROM stock_holds WHERE id = sqlc.arg(id);

-- name: ListActiveHolds :many
SELECT * FROM stock_holds
WHERE variant_id = sqlc.arg(variant_id) AND status = 'active'
ORDER BY created_at;

-- name: SetStockHoldStatus :exec
UPDATE stock_holds SET status = sqlc.arg(status) WHERE id = sqlc.arg(id);

-- name: ExpireStockHolds :exec
UPDATE stock_holds
SET status = 'expired'
WHERE status = 'active'
  AND expires_at IS NOT NULL
  AND expires_at < sqlc.arg(now);

-- name: ListStockHolds :many
SELECT sh.*, pv.sku AS sku, p.name AS product_name, l.name AS location_name
FROM stock_holds sh
JOIN product_variants pv ON pv.id = sh.variant_id
JOIN products p ON p.id = pv.product_id
JOIN locations l ON l.id = sh.location_id
WHERE (sqlc.narg(status) IS NULL OR sh.status = sqlc.narg(status))
ORDER BY sh.created_at DESC, sh.id DESC
LIMIT sqlc.arg(limit) OFFSET sqlc.arg(offset);

-- name: ListSuppliers :many
SELECT * FROM suppliers WHERE is_active = sqlc.arg(is_active) ORDER BY name;

-- name: GetSupplier :one
SELECT * FROM suppliers WHERE id = sqlc.arg(id);

-- name: CreateSupplier :one
INSERT INTO suppliers (name, contact_name, email, phone, lead_time_days, terms, notes)
VALUES (
    sqlc.arg(name),
    sqlc.narg(contact_name),
    sqlc.narg(email),
    sqlc.narg(phone),
    sqlc.narg(lead_time_days),
    sqlc.narg(terms),
    sqlc.narg(notes)
)
RETURNING *;

-- name: UpdateSupplier :exec
UPDATE suppliers
SET name = sqlc.arg(name),
    contact_name = sqlc.narg(contact_name),
    email = sqlc.narg(email),
    phone = sqlc.narg(phone),
    lead_time_days = sqlc.narg(lead_time_days),
    terms = sqlc.narg(terms),
    notes = sqlc.narg(notes),
    is_active = sqlc.arg(is_active)
WHERE id = sqlc.arg(id);

-- name: ListPurchaseOrders :many
SELECT * FROM purchase_orders
WHERE (sqlc.narg(supplier_id) IS NULL OR supplier_id = sqlc.narg(supplier_id))
  AND (sqlc.narg(status) IS NULL OR status = sqlc.narg(status))
ORDER BY created_at DESC
LIMIT sqlc.arg(limit) OFFSET sqlc.arg(offset);

-- name: GetPurchaseOrder :one
SELECT * FROM purchase_orders WHERE id = sqlc.arg(id);

-- name: CreatePurchaseOrder :one
INSERT INTO purchase_orders (supplier_id, location_id, status, ordered_at, expected_at, notes, created_by)
VALUES (
    sqlc.arg(supplier_id),
    sqlc.arg(location_id),
    sqlc.arg(status),
    sqlc.narg(ordered_at),
    sqlc.narg(expected_at),
    sqlc.narg(notes),
    sqlc.narg(created_by)
)
RETURNING *;

-- name: SetPurchaseOrderStatus :exec
UPDATE purchase_orders
SET status = sqlc.arg(status), received_at = sqlc.narg(received_at)
WHERE id = sqlc.arg(id);

-- name: ListPurchaseOrderItems :many
SELECT * FROM purchase_order_items
WHERE purchase_order_id = sqlc.arg(purchase_order_id)
ORDER BY id;

-- name: UpsertPurchaseOrderItem :exec
INSERT INTO purchase_order_items (
    purchase_order_id, variant_id, quantity_ordered, unit_cost_minor, line_total_minor
)
VALUES (
    sqlc.arg(purchase_order_id),
    sqlc.arg(variant_id),
    sqlc.arg(quantity_ordered),
    sqlc.arg(unit_cost_minor),
    sqlc.arg(line_total_minor)
)
ON CONFLICT (purchase_order_id, variant_id) DO UPDATE SET
    quantity_ordered = excluded.quantity_ordered,
    unit_cost_minor = excluded.unit_cost_minor,
    line_total_minor = excluded.line_total_minor;

-- name: ReceivePurchaseOrderItem :exec
UPDATE purchase_order_items
SET quantity_received = quantity_received + sqlc.arg(qty)
WHERE id = sqlc.arg(id);

-- name: OnOrderQuantity :one
SELECT COALESCE(SUM(poi.quantity_ordered - poi.quantity_received), 0) AS on_order
FROM purchase_order_items poi
JOIN purchase_orders po ON po.id = poi.purchase_order_id
WHERE poi.variant_id = sqlc.arg(variant_id)
  AND po.status IN ('ordered', 'partial');
