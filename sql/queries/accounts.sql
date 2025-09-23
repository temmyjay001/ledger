-- sql/queries/accounts.sql
-- Account Management Queries for sqlc

-- name: CreateAccount :one
INSERT INTO accounts (
    code,
    name, 
    account_type,
    parent_id,
    currency,
    metadata
) VALUES (
    $1, $2, $3, $4, $5, $6
) RETURNING *;

-- name: GetAccountByID :one
SELECT * FROM accounts 
WHERE id = $1 AND is_active = true;

-- name: GetAccountByCode :one
SELECT * FROM accounts
WHERE code = $1 AND is_active = true;

-- name: ListAccounts :many
SELECT * FROM accounts
WHERE is_active = true
ORDER BY code ASC;

-- name: ListAccountsByType :many
SELECT * FROM accounts
WHERE account_type = $1 AND is_active = true
ORDER BY code ASC;

-- name: ListAccountsByParent :many
SELECT * FROM accounts
WHERE parent_id = $1 AND is_active = true
ORDER BY code ASC;

-- name: ListAccountsByParentCode :many
SELECT a.* FROM accounts a
JOIN accounts parent ON a.parent_id = parent.id
WHERE parent.code = $1 AND a.is_active = true
ORDER BY a.code ASC;

-- name: UpdateAccount :one
UPDATE accounts
SET 
    name = COALESCE($2, name),
    metadata = COALESCE($3, metadata),
    updated_at = NOW()
WHERE id = $1 AND is_active = true
RETURNING *;

-- name: DeactivateAccount :one
UPDATE accounts
SET 
    is_active = false,
    updated_at = NOW()
WHERE id = $1
RETURNING *;

-- name: GetAccountHierarchy :many
WITH RECURSIVE account_hierarchy AS (
    -- Base case: start with root accounts (no parent)
    SELECT 
        id,
        code,
        name,
        account_type,
        parent_id,
        currency,
        metadata,
        is_active,
        created_at,
        updated_at,
        0 as level,
        code::text as path
    FROM accounts
    WHERE parent_id IS NULL AND is_active = true
    
    UNION ALL
    
    -- Recursive case: get children
    SELECT 
        a.id,
        a.code,
        a.name,
        a.account_type,
        a.parent_id,
        a.currency,
        a.metadata,
        a.is_active,
        a.created_at,
        a.updated_at,
        ah.level + 1,
        ah.path || '/' || a.code
    FROM accounts a
    JOIN account_hierarchy ah ON a.parent_id = ah.id
    WHERE a.is_active = true
)
SELECT * FROM account_hierarchy
ORDER BY path;

-- name: ValidateAccountCode :one
SELECT EXISTS(
    SELECT 1 FROM accounts 
    WHERE code = $1 AND is_active = true
) as exists;

-- name: ValidateParentAccount :one
SELECT 
    id,
    account_type,
    is_active
FROM accounts
WHERE id = $1;

-- Account Balance Operations

-- name: CreateAccountBalance :one
INSERT INTO account_balances (
    account_id,
    currency,
    balance,
    version
) VALUES (
    $1, $2, $3, 1
) RETURNING *;

-- name: GetAccountBalance :one
SELECT * FROM account_balances
WHERE account_id = $1 AND currency = $2;

-- name: GetAccountBalances :many
SELECT * FROM account_balances
WHERE account_id = $1
ORDER BY currency;

-- name: UpdateAccountBalance :one
UPDATE account_balances
SET 
    balance = $3,
    version = version + 1,
    updated_at = NOW()
WHERE account_id = $1 AND currency = $2 AND version = $4
RETURNING *;

-- name: GetAccountBalanceForUpdate :one
SELECT * FROM account_balances
WHERE account_id = $1 AND currency = $2
FOR UPDATE;

-- name: ListAccountBalancesByCurrency :many
SELECT 
    a.id as account_id,
    a.code,
    a.name,
    a.account_type,
    ab.currency,
    ab.balance,
    ab.version,
    ab.updated_at as balance_updated_at
FROM accounts a
LEFT JOIN account_balances ab ON a.id = ab.account_id
WHERE ab.currency = $1 AND a.is_active = true
ORDER BY a.code;

-- name: GetAccountBalanceSummary :many
SELECT 
    a.account_type,
    ab.currency,
    SUM(ab.balance) as total_balance,
    COUNT(a.id) as account_count
FROM accounts a
JOIN account_balances ab ON a.id = ab.account_id
WHERE a.is_active = true
GROUP BY a.account_type, ab.currency
ORDER BY a.account_type, ab.currency;

-- Utility queries for reporting and validation

-- name: GetAccountWithBalance :one
SELECT 
    a.*,
    ab.balance,
    ab.currency as balance_currency,
    ab.version as balance_version,
    ab.updated_at as balance_updated_at
FROM accounts a
LEFT JOIN account_balances ab ON a.id = ab.account_id AND ab.currency = $2
WHERE a.id = $1 AND a.is_active = true;

-- name: ListAccountsWithBalances :many
SELECT 
    a.id,
    a.code,
    a.name,
    a.account_type,
    a.parent_id,
    a.currency as default_currency,
    a.metadata,
    a.created_at,
    COALESCE(
        JSON_AGG(
            JSON_BUILD_OBJECT(
                'currency', ab.currency,
                'balance', ab.balance,
                'version', ab.version,
                'updated_at', ab.updated_at
            ) ORDER BY ab.currency
        ) FILTER (WHERE ab.currency IS NOT NULL),
        '[]'::json
    ) as balances
FROM accounts a
LEFT JOIN account_balances ab ON a.id = ab.account_id
WHERE a.is_active = true
GROUP BY a.id, a.code, a.name, a.account_type, a.parent_id, a.currency, a.metadata, a.created_at
ORDER BY a.code;

-- name: SearchAccounts :many
SELECT * FROM accounts
WHERE 
    is_active = true AND
    (
        code ILIKE '%' || $1 || '%' OR
        name ILIKE '%' || $1 || '%'
    )
ORDER BY code
LIMIT $2;

-- name: GetAccountStats :one
SELECT 
    COUNT(*) as total_accounts,
    COUNT(*) FILTER (WHERE account_type = 'asset') as asset_accounts,
    COUNT(*) FILTER (WHERE account_type = 'liability') as liability_accounts,
    COUNT(*) FILTER (WHERE account_type = 'equity') as equity_accounts,
    COUNT(*) FILTER (WHERE account_type = 'revenue') as revenue_accounts,
    COUNT(*) FILTER (WHERE account_type = 'expense') as expense_accounts,
    COUNT(DISTINCT currency) as currencies_count
FROM accounts
WHERE is_active = true;