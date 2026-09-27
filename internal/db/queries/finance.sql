-- Finance / OpEx, settings and audit queries (ADR-0007, ADR-0010).

-- name: ListExpenseCategories :many
SELECT * FROM expense_categories WHERE is_active = sqlc.arg(is_active) ORDER BY name;

-- name: CreateExpenseCategory :one
INSERT INTO expense_categories (name) VALUES (sqlc.arg(name)) RETURNING *;

-- name: ListExpenses :many
SELECT * FROM expenses
WHERE (sqlc.narg(from_ts) IS NULL OR incurred_at >= sqlc.narg(from_ts))
  AND (sqlc.narg(to_ts) IS NULL OR incurred_at <= sqlc.narg(to_ts))
ORDER BY incurred_at DESC, id DESC
LIMIT sqlc.arg(limit) OFFSET sqlc.arg(offset);

-- name: CreateExpense :one
INSERT INTO expenses (category_id, vendor, amount_minor, incurred_at, note, created_by)
VALUES (
    sqlc.arg(category_id),
    sqlc.narg(vendor),
    sqlc.arg(amount_minor),
    sqlc.arg(incurred_at),
    sqlc.narg(note),
    sqlc.narg(created_by)
)
RETURNING *;

-- name: SumExpenses :one
SELECT COALESCE(SUM(amount_minor), 0) AS total_minor
FROM expenses
WHERE incurred_at >= sqlc.arg(from_ts) AND incurred_at <= sqlc.arg(to_ts);

-- name: ExpenseTotalByCategory :many
SELECT ec.name AS category, COALESCE(SUM(e.amount_minor), 0) AS total_minor
FROM expense_categories ec
LEFT JOIN expenses e
    ON e.category_id = ec.id
   AND e.incurred_at >= sqlc.arg(from_ts)
   AND e.incurred_at <= sqlc.arg(to_ts)
GROUP BY ec.id
ORDER BY ec.name;

-- name: GetSetting :one
SELECT * FROM settings WHERE key = sqlc.arg(key);

-- name: ListSettings :many
SELECT * FROM settings ORDER BY key;

-- name: UpsertSetting :exec
INSERT INTO settings (key, value, updated_by)
VALUES (sqlc.arg(key), sqlc.arg(value), sqlc.narg(updated_by))
ON CONFLICT (key) DO UPDATE SET
    value = excluded.value,
    updated_at = strftime('%Y-%m-%dT%H:%M:%SZ','now'),
    updated_by = excluded.updated_by;

-- name: CreateAuditLog :one
INSERT INTO audit_logs (user_id, action, entity_type, entity_id, before_json, after_json, note, ip)
VALUES (
    sqlc.narg(user_id),
    sqlc.arg(action),
    sqlc.arg(entity_type),
    sqlc.narg(entity_id),
    sqlc.narg(before_json),
    sqlc.narg(after_json),
    sqlc.narg(note),
    sqlc.narg(ip)
)
RETURNING *;

-- name: ListAuditLogs :many
SELECT * FROM audit_logs
WHERE (sqlc.narg(user_id) IS NULL OR user_id = sqlc.narg(user_id))
  AND (sqlc.narg(entity_type) IS NULL OR entity_type = sqlc.narg(entity_type))
  AND (sqlc.narg(from_ts) IS NULL OR created_at >= sqlc.narg(from_ts))
  AND (sqlc.narg(to_ts) IS NULL OR created_at <= sqlc.narg(to_ts))
ORDER BY created_at DESC, id DESC
LIMIT sqlc.arg(limit) OFFSET sqlc.arg(offset);
