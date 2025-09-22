-- sql/queries/api_keys.sql

-- name: CreateAPIKey :one
INSERT INTO api_keys (
    tenant_id, name, key_hash, key_prefix, scopes, expires_at
) VALUES (
    $1, $2, $3, $4, $5, $6
) RETURNING *;

-- name: GetAPIKeyByHash :one
SELECT ak.*, t.slug as tenant_slug 
FROM api_keys ak
JOIN tenants t ON ak.tenant_id = t.id
WHERE ak.key_hash = $1 AND (ak.expires_at IS NULL OR ak.expires_at > NOW())
LIMIT 1;

-- name: ListTenantAPIKeys :many
SELECT id, tenant_id, name, key_prefix, scopes, expires_at, last_used_at, created_at
FROM api_keys 
WHERE tenant_id = $1
ORDER BY created_at DESC;

-- name: UpdateAPIKeyLastUsed :exec
UPDATE api_keys 
SET last_used_at = NOW()
WHERE id = $1;

-- name: DeleteAPIKey :exec
DELETE FROM api_keys 
WHERE id = $1 AND tenant_id = $2;