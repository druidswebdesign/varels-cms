-- Catalog queries: categories, collections, products, variants, media, pricing.

-- name: ListCategories :many
SELECT * FROM categories ORDER BY sort_order, name;

-- name: GetCategory :one
SELECT * FROM categories WHERE id = sqlc.arg(id);

-- name: CreateCategory :one
INSERT INTO categories (name, slug, parent_id, sort_order)
VALUES (sqlc.arg(name), sqlc.arg(slug), sqlc.narg(parent_id), sqlc.arg(sort_order))
RETURNING *;

-- name: UpdateCategory :exec
UPDATE categories
SET name = sqlc.arg(name),
    slug = sqlc.arg(slug),
    parent_id = sqlc.narg(parent_id),
    sort_order = sqlc.arg(sort_order)
WHERE id = sqlc.arg(id);

-- name: DeleteCategory :exec
DELETE FROM categories WHERE id = sqlc.arg(id);

-- name: ListCollections :many
SELECT * FROM collections WHERE is_archived = sqlc.arg(is_archived) ORDER BY name;

-- name: GetCollection :one
SELECT * FROM collections WHERE id = sqlc.arg(id);

-- name: CreateCollection :one
INSERT INTO collections (name, slug, kind, season, launch_date)
VALUES (sqlc.arg(name), sqlc.arg(slug), sqlc.arg(kind), sqlc.narg(season), sqlc.narg(launch_date))
RETURNING *;

-- name: UpdateCollection :exec
UPDATE collections
SET name = sqlc.arg(name),
    slug = sqlc.arg(slug),
    kind = sqlc.arg(kind),
    season = sqlc.narg(season),
    launch_date = sqlc.narg(launch_date)
WHERE id = sqlc.arg(id);

-- name: SetCollectionArchived :exec
UPDATE collections SET is_archived = sqlc.arg(is_archived) WHERE id = sqlc.arg(id);

-- name: ListProductCollections :many
SELECT c.* FROM collections c
JOIN product_collections pc ON pc.collection_id = c.id
WHERE pc.product_id = sqlc.arg(product_id)
ORDER BY c.name;

-- name: AddProductToCollection :exec
INSERT OR IGNORE INTO product_collections (product_id, collection_id)
VALUES (sqlc.arg(product_id), sqlc.arg(collection_id));

-- name: RemoveProductFromCollection :exec
DELETE FROM product_collections
WHERE product_id = sqlc.arg(product_id) AND collection_id = sqlc.arg(collection_id);

-- name: ListProducts :many
SELECT * FROM products
WHERE is_archived = sqlc.arg(is_archived)
  AND (sqlc.narg(category_id) IS NULL OR category_id = sqlc.narg(category_id))
ORDER BY name
LIMIT sqlc.arg(limit) OFFSET sqlc.arg(offset);

-- name: SearchProducts :many
SELECT * FROM products
WHERE is_archived = 0
  AND (name LIKE sqlc.arg(q) OR slug LIKE sqlc.arg(q))
ORDER BY name
LIMIT sqlc.arg(limit);

-- name: GetProduct :one
SELECT * FROM products WHERE id = sqlc.arg(id);

-- name: GetProductBySlug :one
SELECT * FROM products WHERE slug = sqlc.arg(slug);

-- name: CreateProduct :one
INSERT INTO products (name, slug, description, category_id)
VALUES (sqlc.arg(name), sqlc.arg(slug), sqlc.narg(description), sqlc.narg(category_id))
RETURNING *;

-- name: UpdateProduct :exec
UPDATE products
SET name = sqlc.arg(name),
    slug = sqlc.arg(slug),
    description = sqlc.narg(description),
    category_id = sqlc.narg(category_id),
    updated_at = strftime('%Y-%m-%dT%H:%M:%SZ','now')
WHERE id = sqlc.arg(id);

-- name: SetProductArchived :exec
UPDATE products
SET is_archived = sqlc.arg(is_archived),
    archived_at = sqlc.narg(archived_at),
    updated_at = strftime('%Y-%m-%dT%H:%M:%SZ','now')
WHERE id = sqlc.arg(id);

-- name: ListVariantsByProduct :many
SELECT * FROM product_variants
WHERE product_id = sqlc.arg(product_id) AND is_archived = sqlc.arg(is_archived)
ORDER BY size, color, sku;

-- name: GetVariant :one
SELECT * FROM product_variants WHERE id = sqlc.arg(id);

-- name: GetVariantBySKU :one
SELECT * FROM product_variants WHERE sku = sqlc.arg(sku);

-- name: CreateVariant :one
INSERT INTO product_variants (
    product_id, sku, size, color, barcode, cost_minor, retail_price_minor,
    wholesale_price_minor, low_stock_threshold
)
VALUES (
    sqlc.arg(product_id),
    sqlc.arg(sku),
    sqlc.narg(size),
    sqlc.narg(color),
    sqlc.narg(barcode),
    sqlc.arg(cost_minor),
    sqlc.arg(retail_price_minor),
    sqlc.narg(wholesale_price_minor),
    sqlc.arg(low_stock_threshold)
)
RETURNING *;

-- name: UpdateVariant :exec
UPDATE product_variants
SET sku = sqlc.arg(sku),
    size = sqlc.narg(size),
    color = sqlc.narg(color),
    barcode = sqlc.narg(barcode),
    cost_minor = sqlc.arg(cost_minor),
    retail_price_minor = sqlc.arg(retail_price_minor),
    wholesale_price_minor = sqlc.narg(wholesale_price_minor),
    updated_at = strftime('%Y-%m-%dT%H:%M:%SZ','now')
WHERE id = sqlc.arg(id);

-- name: SetVariantThreshold :exec
UPDATE product_variants
SET low_stock_threshold = sqlc.arg(low_stock_threshold),
    updated_at = strftime('%Y-%m-%dT%H:%M:%SZ','now')
WHERE id = sqlc.arg(id);

-- name: SetVariantArchived :exec
UPDATE product_variants
SET is_archived = sqlc.arg(is_archived),
    updated_at = strftime('%Y-%m-%dT%H:%M:%SZ','now')
WHERE id = sqlc.arg(id);

-- name: ListProductMedia :many
SELECT * FROM product_media
WHERE product_id = sqlc.arg(product_id)
ORDER BY sort_order, id;

-- name: GetProductMedia :one
SELECT * FROM product_media WHERE id = sqlc.arg(id);

-- name: CreateProductMedia :one
INSERT INTO product_media (product_id, variant_id, media_type, url, alt_text, sort_order, is_primary)
VALUES (
    sqlc.arg(product_id),
    sqlc.narg(variant_id),
    sqlc.arg(media_type),
    sqlc.arg(url),
    sqlc.narg(alt_text),
    sqlc.arg(sort_order),
    sqlc.arg(is_primary)
)
RETURNING *;

-- name: ClearPrimaryMedia :exec
UPDATE product_media SET is_primary = 0 WHERE product_id = sqlc.arg(product_id);

-- name: DeleteProductMedia :exec
DELETE FROM product_media WHERE id = sqlc.arg(id);

-- name: ListPriceTiers :many
SELECT * FROM price_tiers WHERE variant_id = sqlc.arg(variant_id) ORDER BY min_qty;

-- name: CreatePriceTier :one
INSERT INTO price_tiers (variant_id, min_qty, price_minor, label)
VALUES (sqlc.arg(variant_id), sqlc.arg(min_qty), sqlc.arg(price_minor), sqlc.narg(label))
RETURNING *;

-- name: DeletePriceTier :exec
DELETE FROM price_tiers WHERE id = sqlc.arg(id);

-- name: ListCostHistory :many
SELECT * FROM cost_history
WHERE variant_id = sqlc.arg(variant_id)
ORDER BY effective_from DESC, id DESC;

-- name: CreateCostHistory :one
INSERT INTO cost_history (variant_id, cost_minor, effective_from, source, note, created_by)
VALUES (
    sqlc.arg(variant_id),
    sqlc.arg(cost_minor),
    sqlc.arg(effective_from),
    sqlc.arg(source),
    sqlc.narg(note),
    sqlc.narg(created_by)
)
RETURNING *;

-- name: CreateCostLayer :one
INSERT INTO cost_layers (variant_id, received_at, qty_received, qty_remaining, unit_cost_minor, source_ref)
VALUES (
    sqlc.arg(variant_id),
    sqlc.arg(received_at),
    sqlc.arg(qty_received),
    sqlc.arg(qty_remaining),
    sqlc.arg(unit_cost_minor),
    sqlc.narg(source_ref)
)
RETURNING *;

-- name: ListOpenCostLayers :many
SELECT * FROM cost_layers
WHERE variant_id = sqlc.arg(variant_id) AND qty_remaining > 0
ORDER BY received_at, id;

-- name: DecrementCostLayer :exec
UPDATE cost_layers
SET qty_remaining = qty_remaining - sqlc.arg(qty)
WHERE id = sqlc.arg(id);

-- name: ListActiveDiscounts :many
SELECT * FROM discounts
WHERE is_active = 1
  AND (starts_at IS NULL OR starts_at <= sqlc.arg(now))
  AND (ends_at IS NULL OR ends_at >= sqlc.arg(now))
ORDER BY name;

-- name: CreateDiscount :one
INSERT INTO discounts (name, scope, scope_id, type, value, starts_at, ends_at, created_by)
VALUES (
    sqlc.arg(name),
    sqlc.arg(scope),
    sqlc.narg(scope_id),
    sqlc.arg(type),
    sqlc.arg(value),
    sqlc.narg(starts_at),
    sqlc.narg(ends_at),
    sqlc.narg(created_by)
)
RETURNING *;

-- name: SetDiscountActive :exec
UPDATE discounts SET is_active = sqlc.arg(is_active) WHERE id = sqlc.arg(id);

-- name: ListSellableVariants :many
SELECT pv.*, p.name AS product_name
FROM product_variants pv
JOIN products p ON p.id = pv.product_id
WHERE pv.is_archived = 0 AND p.is_archived = 0
ORDER BY p.name, pv.size, pv.color, pv.sku;
