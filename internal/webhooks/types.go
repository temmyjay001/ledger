// internal/webhooks/types.go
package webhooks

import (
	"encoding/json"
	"time"

	"github.com/google/uuid"
)

// WebhookConfig represents tenant webhook configuration
type WebhookConfig struct {
	WebhookURL    string   `json:"webhook_url"`
	WebhookSecret string   `json:"webhook_secret"`
	WebhookEvents []string `json:"webhook_events"`
	Enabled       bool     `json:"enabled"`
}

// WebhookPayload represents the payload sent to webhook endpoints
type WebhookPayload struct {
	ID         string          `json:"id"`         // event_id
	Type       string          `json:"type"`       // event_type  
	Created    int64           `json:"created"`    // unix timestamp
	Data       json.RawMessage `json:"data"`       // event_data
	TenantID   string          `json:"tenant_id"`
	LiveMode   bool            `json:"livemode"`   // always true for now
}

// WebhookDeliveryRequest represents a webhook delivery request
type WebhookDeliveryRequest struct {
	TenantID    uuid.UUID `json:"tenant_id"`
	EventID     uuid.UUID `json:"event_id"`
	WebhookURL  string    `json:"webhook_url"`
	MaxAttempts int       `json:"max_attempts"`
	NextRetryAt time.Time `json:"next_retry_at"`
}

// WebhookDeliveryResult represents the result of a webhook delivery attempt
type WebhookDeliveryResult struct {
	Success        bool   `json:"success"`
	StatusCode     int    `json:"status_code"`
	ResponseBody   string `json:"response_body"`
	ErrorMessage   string `json:"error_message,omitempty"`
	DeliveryTimeMs int64  `json:"delivery_time_ms"`
}

// WebhookConfigRequest represents a request to configure webhooks
type WebhookConfigRequest struct {
	URL       string   `json:"url" validate:"required,url"`
	Secret    string   `json:"secret" validate:"required,min=32"`
	Events    []string `json:"events" validate:"required,min=1"`
	Enabled   bool     `json:"enabled"`
}

// WebhookConfigResponse represents webhook configuration response
type WebhookConfigResponse struct {
	URL       string   `json:"url"`
	Events    []string `json:"events"`
	Enabled   bool     `json:"enabled"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// WebhookDeliveryResponse represents a webhook delivery record
type WebhookDeliveryResponse struct {
	ID             string     `json:"id"`
	EventID        string     `json:"event_id"`
	EventType      string     `json:"event_type"`
	URL            string     `json:"url"`
	StatusCode     *int       `json:"status_code,omitempty"`
	Attempts       int        `json:"attempts"`
	MaxAttempts    int        `json:"max_attempts"`
	NextRetryAt    *time.Time `json:"next_retry_at,omitempty"`
	DeliveredAt    *time.Time `json:"delivered_at,omitempty"`
	FailedAt       *time.Time `json:"failed_at,omitempty"`
	CreatedAt      time.Time  `json:"created_at"`
}

// Default webhook configuration
const (
	DefaultMaxAttempts    = 3
	DefaultTimeoutSeconds = 30
	MaxWebhookURLLength   = 2048
	MaxWebhookSecretLength = 128
)

// Supported event types
var SupportedEventTypes = []string{
	"transaction.posted",
	"balance.updated",
	"account.created", 
	"account.updated",
}