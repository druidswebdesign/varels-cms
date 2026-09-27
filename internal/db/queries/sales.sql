-- Sales queries: customers, orders, order items, payments, returns (ADR-0006).
-- The HTTP surface stays /sales; the data model is orders/order_items.

-- name: ListCustomers :many
SELECT * FROM customers ORDER BY name LIMIT sqlc.arg(limit) OFFSET sqlc.arg(offset);

-- name: SearchCustomers :many
SELECT * FROM customers
WHERE name LIKE sqlc.arg(q) OR phone LIKE sqlc.arg(q) OR email LIKE sqlc.arg(q)
ORDER BY name
LIMIT sqlc.arg(limit);

-- name: GetCustomer :one
SELECT * FROM customers WHERE id = sqlc.arg(id);

-- name: CreateCustomer :one
INSERT INTO customers (name, phone, email, instagram, notes)
VALUES (sqlc.arg(name), sqlc.narg(phone), sqlc.narg(email), sqlc.narg(instagram), sqlc.narg(notes))
RETURNING *;

-- name: UpdateCustomer :exec
UPDATE customers
SET name = sqlc.arg(name),
    phone = sqlc.narg(phone),
    email = sqlc.narg(email),
    instagram = sqlc.narg(instagram),
    notes = sqlc.narg(notes),
    updated_at = strftime('%Y-%m-%dT%H:%M:%SZ','now')
WHERE id = sqlc.arg(id);

-- name: CreateOrder :one
INSERT INTO orders (
    order_number, customer_id, channel_id, location_id, status,
    subtotal_minor, discount_minor, tax_minor, shipping_minor, total_minor,
    payment_status, placed_at, note, created_by
)
VALUES (
    sqlc.arg(order_number),
    sqlc.narg(customer_id),
    sqlc.arg(channel_id),
    sqlc.arg(location_id),
    sqlc.arg(status),
    sqlc.arg(subtotal_minor),
    sqlc.arg(discount_minor),
    sqlc.arg(tax_minor),
    sqlc.arg(shipping_minor),
    sqlc.arg(total_minor),
    sqlc.arg(payment_status),
    sqlc.arg(placed_at),
    sqlc.narg(note),
    sqlc.narg(created_by)
)
RETURNING *;

-- name: GetOrder :one
SELECT * FROM orders WHERE id = sqlc.arg(id);

-- name: GetOrderByNumber :one
SELECT * FROM orders WHERE order_number = sqlc.arg(order_number);

-- name: ListOrders :many
SELECT * FROM orders
WHERE (sqlc.narg(from_ts) IS NULL OR placed_at >= sqlc.narg(from_ts))
  AND (sqlc.narg(to_ts) IS NULL OR placed_at <= sqlc.narg(to_ts))
  AND (sqlc.narg(channel_id) IS NULL OR channel_id = sqlc.narg(channel_id))
  AND (sqlc.narg(location_id) IS NULL OR location_id = sqlc.narg(location_id))
  AND (sqlc.narg(status) IS NULL OR status = sqlc.narg(status))
ORDER BY placed_at DESC, id DESC
LIMIT sqlc.arg(limit) OFFSET sqlc.arg(offset);

-- name: ListOrdersByCustomer :many
SELECT * FROM orders
WHERE customer_id = sqlc.arg(customer_id)
ORDER BY placed_at DESC, id DESC
LIMIT sqlc.arg(limit);

-- name: SetOrderStatus :exec
UPDATE orders
SET status = sqlc.arg(status),
    updated_at = strftime('%Y-%m-%dT%H:%M:%SZ','now')
WHERE id = sqlc.arg(id);

-- name: SetOrderPaymentStatus :exec
UPDATE orders
SET payment_status = sqlc.arg(payment_status),
    updated_at = strftime('%Y-%m-%dT%H:%M:%SZ','now')
WHERE id = sqlc.arg(id);

-- name: CreateOrderItem :one
INSERT INTO order_items (
    order_id, variant_id, quantity, unit_price_minor, unit_cost_minor,
    discount_minor, line_total_minor
)
VALUES (
    sqlc.arg(order_id),
    sqlc.arg(variant_id),
    sqlc.arg(quantity),
    sqlc.arg(unit_price_minor),
    sqlc.arg(unit_cost_minor),
    sqlc.arg(discount_minor),
    sqlc.arg(line_total_minor)
)
RETURNING *;

-- name: GetOrderItem :one
SELECT * FROM order_items WHERE id = sqlc.arg(id);

-- name: ListOrderItems :many
SELECT * FROM order_items WHERE order_id = sqlc.arg(order_id) ORDER BY id;

-- name: CreatePayment :one
INSERT INTO payments (order_id, method, amount_minor, status, reference, paid_at, created_by)
VALUES (
    sqlc.arg(order_id),
    sqlc.arg(method),
    sqlc.arg(amount_minor),
    sqlc.arg(status),
    sqlc.narg(reference),
    sqlc.arg(paid_at),
    sqlc.narg(created_by)
)
RETURNING *;

-- name: ListPaymentsByOrder :many
SELECT * FROM payments WHERE order_id = sqlc.arg(order_id) ORDER BY paid_at, id;

-- name: SetPaymentStatus :exec
UPDATE payments SET status = sqlc.arg(status) WHERE id = sqlc.arg(id);

-- name: CreateReturn :one
INSERT INTO returns (
    order_id, customer_id, resolution, status, total_refund_minor,
    note, processed_at, created_by
)
VALUES (
    sqlc.arg(order_id),
    sqlc.narg(customer_id),
    sqlc.arg(resolution),
    sqlc.arg(status),
    sqlc.arg(total_refund_minor),
    sqlc.narg(note),
    sqlc.narg(processed_at),
    sqlc.narg(created_by)
)
RETURNING *;

-- name: GetReturn :one
SELECT * FROM returns WHERE id = sqlc.arg(id);

-- name: ListReturnsByOrder :many
SELECT * FROM returns WHERE order_id = sqlc.arg(order_id) ORDER BY created_at DESC;

-- name: SetReturnStatus :exec
UPDATE returns
SET status = sqlc.arg(status), processed_at = sqlc.narg(processed_at)
WHERE id = sqlc.arg(id);

-- name: CreateReturnItem :one
INSERT INTO return_items (
    return_id, order_item_id, quantity, condition_state, resolution,
    refund_amount_minor, reason_code
)
VALUES (
    sqlc.arg(return_id),
    sqlc.arg(order_item_id),
    sqlc.arg(quantity),
    sqlc.arg(condition_state),
    sqlc.arg(resolution),
    sqlc.arg(refund_amount_minor),
    sqlc.arg(reason_code)
)
RETURNING *;

-- name: ListReturnItems :many
SELECT * FROM return_items WHERE return_id = sqlc.arg(return_id) ORDER BY id;

-- name: ReturnedQuantityByOrderItem :one
SELECT COALESCE(SUM(quantity), 0) AS quantity
FROM return_items
WHERE order_item_id = sqlc.arg(order_item_id);

-- Store credit ledger (ADR-0020).

-- name: CreateStoreCreditEntry :one
INSERT INTO store_credit_ledger (customer_id, delta_minor, reason, order_id, return_id, note, created_by)
VALUES (
    sqlc.arg(customer_id),
    sqlc.arg(delta_minor),
    sqlc.arg(reason),
    sqlc.narg(order_id),
    sqlc.narg(return_id),
    sqlc.narg(note),
    sqlc.narg(created_by)
)
RETURNING *;

-- name: StoreCreditBalance :one
SELECT COALESCE(SUM(delta_minor), 0) AS balance_minor
FROM store_credit_ledger
WHERE customer_id = sqlc.arg(customer_id);

-- name: ListStoreCreditEntries :many
SELECT * FROM store_credit_ledger
WHERE customer_id = sqlc.arg(customer_id)
ORDER BY created_at DESC, id DESC;
