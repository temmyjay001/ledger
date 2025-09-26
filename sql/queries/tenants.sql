-- sql/queries/tenants.sql

-- name: CreateTenant :one
INSERT INTO tenants (
    name, slug, business_type, country_code, base_currency, timezone
) VALUES (
    $1, $2, $3, $4, $5, $6
) RETURNING *;

-- name: GetTenantByID :one
SELECT * FROM tenants 
WHERE id = $1 LIMIT 1;

-- name: GetTenantBySlug :one
SELECT * FROM tenants 
WHERE slug = $1 LIMIT 1;

-- name: ListTenantsByUser :many
SELECT t.* FROM tenants t
JOIN tenant_users tu ON t.id = tu.tenant_id
WHERE tu.user_id = $1
ORDER BY t.created_at DESC;

-- name: UpdateTenantMetadata :one
UPDATE tenants 
SET metadata = $2, updated_at = NOW()
WHERE id = $1
RETURNING *;