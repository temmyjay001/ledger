-- sql/queries/tenant_users.sql

-- name: AddUserToTenant :one
INSERT INTO tenant_users (
    tenant_id, user_id, role, permissions
) VALUES (
    $1, $2, $3, $4
) RETURNING *;

-- name: GetTenantUser :one
SELECT * FROM tenant_users
WHERE tenant_id = $1 AND user_id = $2 LIMIT 1;

-- name: ListTenantUsers :many
SELECT tu.*, u.email, u.first_name, u.last_name
FROM tenant_users tu
JOIN users u ON tu.user_id = u.id
WHERE tu.tenant_id = $1
ORDER BY tu.created_at DESC;

-- name: UpdateTenantUserRole :exec
UPDATE tenant_users 
SET role = $3, permissions = $4
WHERE tenant_id = $1 AND user_id = $2;

-- name: RemoveUserFromTenant :exec
DELETE FROM tenant_users 
WHERE tenant_id = $1 AND user_id = $2;