-- sql/queries/users.sql

-- name: CreateUser :one
INSERT INTO users (
    email, password_hash, first_name, last_name
) VALUES (
    $1, $2, $3, $4
) RETURNING *;

-- name: GetUserByEmail :one
SELECT * FROM users 
WHERE email = $1 LIMIT 1;

-- name: GetUserByID :one
SELECT * FROM users 
WHERE id = $1 LIMIT 1;

-- name: UpdateUserLastLogin :exec
UPDATE users 
SET last_login_at = NOW(), failed_login_attempts = 0 
WHERE id = $1;

-- name: IncrementFailedLoginAttempts :exec
UPDATE users 
SET failed_login_attempts = failed_login_attempts + 1,
    locked_until = CASE 
        WHEN failed_login_attempts + 1 >= 5 
        THEN NOW() + INTERVAL '15 minutes'
        ELSE locked_until
    END
WHERE id = $1;

-- name: VerifyUserEmail :exec
UPDATE users 
SET email_verified = true, status = 'active'
WHERE id = $1;