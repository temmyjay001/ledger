-- sql/queries/transactions.sql
-- Transaction Management Queries for sqlc

-- Basic Transaction Operations
-- name: CreateTransaction :one
INSERT INTO transactions (
    idempotency_key, description, reference, metadata
) VALUES (
    $1, $2, $3, $4
) RETURNING *;

-- name: GetTransactionByIdempotencyKey :one
SELECT * FROM transactions 
WHERE idempotency_key = $1 LIMIT 1;

-- name: GetTransactionByID :one
SELECT * FROM transactions 
WHERE id = $1;

-- name: UpdateTransactionStatus :one
UPDATE transactions
SET 
    status = $2,
    posted_at = CASE WHEN $2 = 'posted' ::public.transaction_status_enum THEN NOW() ELSE posted_at END
WHERE id = $1
RETURNING *;

-- Advanced Transaction Queries
-- name: ListTransactions :many
SELECT * FROM transactions
ORDER BY created_at DESC
LIMIT $1 OFFSET $2;

-- name: ListTransactionsByAccount :many
SELECT DISTINCT t.* FROM transactions t
JOIN transaction_lines tl ON t.id = tl.transaction_id
JOIN accounts a ON tl.account_id = a.id
WHERE a.code = $1
ORDER BY t.created_at DESC
LIMIT $2 OFFSET $3;

-- name: ListTransactionsByDateRange :many
SELECT * FROM transactions
WHERE posted_at BETWEEN $1 AND $2
ORDER BY posted_at DESC
LIMIT $3 OFFSET $4;

-- name: ListTransactionsByAccountAndDateRange :many
SELECT DISTINCT t.* FROM transactions t
JOIN transaction_lines tl ON t.id = tl.transaction_id
JOIN accounts a ON tl.account_id = a.id
WHERE a.code = $1 
AND t.posted_at BETWEEN $2 AND $3
ORDER BY t.created_at DESC
LIMIT $4 OFFSET $5;

-- Transaction Line Operations
-- name: CreateTransactionLine :one
INSERT INTO transaction_lines (
    transaction_id, account_id, amount, side, currency, metadata
) VALUES (
    $1, $2, $3, $4, $5, $6
) RETURNING *;

-- name: GetTransactionLines :many
SELECT 
    tl.*,
    a.code as account_code,
    a.name as account_name
FROM transaction_lines tl
JOIN accounts a ON tl.account_id = a.id
WHERE tl.transaction_id = $1
ORDER BY tl.created_at;

-- name: GetTransactionWithLines :one
SELECT 
    t.*,
    COALESCE(
        JSON_AGG(
            JSON_BUILD_OBJECT(
                'id', tl.id,
                'account_id', tl.account_id,
                'account_code', a.code,
                'account_name', a.name,
                'amount', tl.amount,
                'side', tl.side,
                'currency', tl.currency,
                'metadata', tl.metadata,
                'created_at', tl.created_at
            ) ORDER BY tl.created_at
        ) FILTER (WHERE tl.id IS NOT NULL),
        '[]'::json
    ) as lines
FROM transactions t
LEFT JOIN transaction_lines tl ON t.id = tl.transaction_id
LEFT JOIN accounts a ON tl.account_id = a.id
WHERE t.id = $1
GROUP BY t.id, t.idempotency_key, t.description, t.reference, t.status, t.posted_at, t.metadata, t.created_at;