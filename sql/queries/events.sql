-- sql/queries/events.sql

-- name: CreateEvent :one
INSERT INTO events (
    tenant_id, aggregate_id, aggregate_type, event_type, 
    event_version, event_data, metadata
) VALUES (
    $1, $2, $3, $4, $5, $6, $7
) RETURNING *;

-- name: GetEventsByAggregate :many
SELECT * FROM events 
WHERE tenant_id = $1 AND aggregate_id = $2
ORDER BY event_version ASC;

-- name: GetEventsByType :many
SELECT * FROM events 
WHERE tenant_id = $1 AND event_type = $2
ORDER BY created_at DESC
LIMIT $3 OFFSET $4;

-- name: GetEventsAfterSequence :many
SELECT * FROM events 
WHERE sequence_number > $1
ORDER BY sequence_number ASC
LIMIT $2;