-- Analytics queries. Reporting buckets are Argentina-local (ADR-0014); callers
-- pass UTC boundaries computed in America/Argentina/Buenos_Aires. Money is ARS
-- minor units (ADR-0005). Cancelled orders are excluded everywhere.

-- name: SalesSummary :one
SELECT
    COALESCE(SUM(total_minor), 0) AS revenue_minor,
    COUNT(*)                      AS order_count
FROM orders
WHERE status <> 'cancelled'
  AND placed_at >= sqlc.arg(from_ts)
  AND placed_at <= sqlc.arg(to_ts);

-- name: COGSSummary :one
SELECT COALESCE(SUM(oi.unit_cost_minor * (oi.quantity - COALESCE(r.returned_qty, 0))), 0) AS cogs_minor
FROM order_items oi
JOIN orders o ON o.id = oi.order_id
LEFT JOIN (
    SELECT order_item_id, SUM(quantity) AS returned_qty
    FROM return_items
    GROUP BY order_item_id
) r ON r.order_item_id = oi.id
WHERE o.status <> 'cancelled'
  AND o.placed_at >= sqlc.arg(from_ts)
  AND o.placed_at <= sqlc.arg(to_ts);

-- name: RevenueByChannel :many
SELECT sc.name AS channel, COUNT(o.id) AS order_count, COALESCE(SUM(o.total_minor), 0) AS revenue_minor
FROM orders o
JOIN sales_channels sc ON sc.id = o.channel_id
WHERE o.status <> 'cancelled'
  AND o.placed_at >= sqlc.arg(from_ts)
  AND o.placed_at <= sqlc.arg(to_ts)
GROUP BY sc.id
ORDER BY revenue_minor DESC;

-- name: RevenueByLocation :many
SELECT l.name AS location, COUNT(o.id) AS order_count, COALESCE(SUM(o.total_minor), 0) AS revenue_minor
FROM orders o
JOIN locations l ON l.id = o.location_id
WHERE o.status <> 'cancelled'
  AND o.placed_at >= sqlc.arg(from_ts)
  AND o.placed_at <= sqlc.arg(to_ts)
GROUP BY l.id
ORDER BY revenue_minor DESC;

-- name: BestSellers :many
SELECT
    pv.id                    AS variant_id,
    pv.sku                   AS sku,
    p.name                   AS product_name,
    pv.size                  AS size,
    pv.color                 AS color,
    SUM(oi.quantity)         AS units_sold,
    SUM(oi.line_total_minor) AS revenue_minor
FROM order_items oi
JOIN orders o ON o.id = oi.order_id
JOIN product_variants pv ON pv.id = oi.variant_id
JOIN products p ON p.id = pv.product_id
WHERE o.status <> 'cancelled'
  AND o.placed_at >= sqlc.arg(from_ts)
  AND o.placed_at <= sqlc.arg(to_ts)
GROUP BY pv.id
ORDER BY units_sold DESC, revenue_minor DESC
LIMIT sqlc.arg(limit);

-- name: BestSellingSizes :many
SELECT pv.size AS size, SUM(oi.quantity) AS units_sold
FROM order_items oi
JOIN orders o ON o.id = oi.order_id
JOIN product_variants pv ON pv.id = oi.variant_id
WHERE o.status <> 'cancelled'
  AND o.placed_at >= sqlc.arg(from_ts)
  AND o.placed_at <= sqlc.arg(to_ts)
  AND pv.size IS NOT NULL
GROUP BY pv.size
ORDER BY units_sold DESC;

-- name: InventoryValuation :one
SELECT
    COALESCE(SUM(CASE WHEN sl.state = 'sellable' THEN sl.quantity * pv.cost_minor ELSE 0 END), 0)         AS inventory_cost_minor,
    COALESCE(SUM(CASE WHEN sl.state = 'sellable' THEN sl.quantity * pv.retail_price_minor ELSE 0 END), 0) AS potential_retail_minor,
    COALESCE(SUM(CASE WHEN sl.state <> 'sellable' THEN sl.quantity * pv.cost_minor ELSE 0 END), 0)        AS non_sellable_cost_minor
FROM stock_levels sl
JOIN product_variants pv ON pv.id = sl.variant_id
WHERE pv.is_archived = 0;

-- name: Deadstock :many
SELECT
    pv.id                            AS variant_id,
    pv.sku                           AS sku,
    p.name                           AS product_name,
    sl.quantity                      AS qty_on_hand,
    sl.quantity * pv.cost_minor      AS trapped_cash_minor
FROM stock_levels sl
JOIN product_variants pv ON pv.id = sl.variant_id
JOIN products p ON p.id = pv.product_id
WHERE sl.state = 'sellable'
  AND sl.quantity > 0
  AND pv.is_archived = 0
  AND NOT EXISTS (
      SELECT 1
      FROM order_items oi
      JOIN orders o ON o.id = oi.order_id
      WHERE oi.variant_id = sl.variant_id
        AND o.status <> 'cancelled'
        AND o.placed_at >= sqlc.arg(stale_since)
  )
ORDER BY trapped_cash_minor DESC;
