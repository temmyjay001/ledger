-- sql/queries/webhooks.sql

-- name: CreateWebhookDelivery :one
INSERT INTO webhook_deliveries (
    tenant_id, event_id, webhook_url, max_attempts, next_retry_at
) VALUES (
    $1, $2, $3, $4, $5
) RETURNING *;

-- name: GetPendingWebhookDeliveries :many
SELECT * FROM webhook_deliveries
WHERE next_retry_at IS NOT NULL 
  AND next_retry_at <= NOW()
  AND attempts < max_attempts
ORDER BY created_at ASC
LIMIT $1;

-- name: UpdateWebhookDeliverySuccess :exec
UPDATE webhook_deliveries 
SET http_status_code = $2,
    response_body = $3,
    attempts = attempts + 1,
    delivered_at = NOW(),
    next_retry_at = NULL
WHERE id = $1;

-- name: UpdateWebhookDeliveryFailure :exec
UPDATE webhook_deliveries 
SET http_status_code = $2,
    response_body = $3,
    attempts = attempts + 1,
    next_retry_at = CASE 
        WHEN attempts + 1 >= max_attempts THEN NULL
        ELSE NOW() + (INTERVAL '1 minute' * POWER(2, attempts + 1))
    END,
    failed_at = CASE 
        WHEN attempts + 1 >= max_attempts THEN NOW()
        ELSE failed_at
    END
WHERE id = $1;