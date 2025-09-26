package webhooks

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"strconv"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/temmyjay001/ledger-service/internal/storage"
	"github.com/temmyjay001/ledger-service/internal/storage/queries"
)

type Service struct {
	db         *storage.DB
	httpClient *http.Client
}

func NewService(db *storage.DB) *Service {
	return &Service{
		db: db,
		httpClient: &http.Client{
			Timeout: DefaultTimeoutSeconds * time.Second,
		},
	}
}

// QueueWebhookDelivery creates a webhook delivery record for an event
func (s *Service) QueueWebhookDelivery(ctx context.Context, event queries.Event) error {
	// Get tenant from database
	tenant, err := s.db.Queries.GetTenantByID(ctx, event.TenantID)
	if err != nil {
		return fmt.Errorf("failed to get tenant: %w", err)
	}

	// Parse webhook configuration from tenant metadata
	config, err := s.parseWebhookConfig(tenant.Metadata)
	if err != nil {
		log.Printf("No webhook config for tenant %s: %v", tenant.ID, err)
		return nil // Not an error - tenant just doesn't have webhooks configured
	}

	if !config.Enabled {
		log.Printf("Webhooks disabled for tenant %s", tenant.ID)
		return nil
	}

	// Check if this event type should trigger webhooks
	if !s.shouldDeliverEvent(config, event.EventType) {
		log.Printf("Event type %s not configured for webhook delivery", event.EventType)
		return nil
	}

	// Create webhook delivery record
	nextRetryAt := pgtype.Timestamptz{
		Time:  time.Now(),
		Valid: true,
	}

	_, err = s.db.Queries.CreateWebhookDelivery(ctx, queries.CreateWebhookDeliveryParams{
		TenantID:    event.TenantID,
		EventID:     event.EventID,
		WebhookUrl:  config.WebhookURL,
		MaxAttempts: pgtype.Int4{Int32: int32(DefaultMaxAttempts), Valid: true},
		NextRetryAt: nextRetryAt,
	})

	if err != nil {
		return fmt.Errorf("failed to create webhook delivery: %w", err)
	}

	log.Printf("Queued webhook delivery for event %s to %s", event.EventID, config.WebhookURL)
	return nil
}

// ProcessPendingDeliveries processes webhook deliveries that are ready to be sent
func (s *Service) ProcessPendingDeliveries(ctx context.Context, batchSize int32) error {
	deliveries, err := s.db.Queries.GetPendingWebhookDeliveries(ctx, batchSize)
	if err != nil {
		return fmt.Errorf("failed to get pending deliveries: %w", err)
	}

	if len(deliveries) == 0 {
		return nil // No deliveries to process
	}

	log.Printf("Processing %d pending webhook deliveries", len(deliveries))

	for _, delivery := range deliveries {
		if err := s.processDelivery(ctx, delivery); err != nil {
			log.Printf("Failed to process delivery %s: %v", delivery.ID, err)
		}
	}

	return nil
}

// processDelivery handles a single webhook delivery
func (s *Service) processDelivery(ctx context.Context, delivery queries.WebhookDelivery) error {
	// Get the event data
	event, err := s.db.Queries.GetEventByID(ctx, queries.GetEventByIDParams{
		TenantID: delivery.TenantID,
		EventID:  delivery.EventID,
	})
	if err != nil {
		return fmt.Errorf("failed to get event %s for tenant %s: %w", delivery.EventID, delivery.TenantID, err)
	}

	// Get tenant for webhook config
	tenant, err := s.db.Queries.GetTenantByID(ctx, delivery.TenantID)
	if err != nil {
		return fmt.Errorf("failed to get tenant: %w", err)
	}

	config, err := s.parseWebhookConfig(tenant.Metadata)
	if err != nil {
		return fmt.Errorf("failed to parse webhook config: %w", err)
	}

	// Create webhook payload
	payload := WebhookPayload{
		ID:       event.EventID.String(),
		Type:     event.EventType,
		Created:  event.CreatedAt.Unix(),
		Data:     event.EventData,
		TenantID: delivery.TenantID.String(),
		LiveMode: true,
	}

	// Attempt delivery
	result := s.deliverWebhook(ctx, config, payload)

	// Update delivery record based on result
	if result.Success {
		err = s.db.Queries.UpdateWebhookDeliverySuccess(ctx, queries.UpdateWebhookDeliverySuccessParams{
			ID:             delivery.ID,
			HttpStatusCode: pgtype.Int4{Int32: int32(result.StatusCode), Valid: true},
			ResponseBody:   pgtype.Text{String: result.ResponseBody, Valid: true},
		})
	} else {
		err = s.db.Queries.UpdateWebhookDeliveryFailure(ctx, queries.UpdateWebhookDeliveryFailureParams{
			ID:             delivery.ID,
			HttpStatusCode: pgtype.Int4{Int32: int32(result.StatusCode), Valid: true},
			ResponseBody:   pgtype.Text{String: result.ErrorMessage, Valid: true},
		})
	}

	if err != nil {
		log.Printf("Failed to update webhook delivery status: %v", err)
	}

	return nil
}

// deliverWebhook sends the webhook HTTP request
func (s *Service) deliverWebhook(ctx context.Context, config *WebhookConfig, payload WebhookPayload) WebhookDeliveryResult {
	startTime := time.Now()

	// Serialize payload
	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		return WebhookDeliveryResult{
			Success:      false,
			StatusCode:   0,
			ErrorMessage: fmt.Sprintf("Failed to serialize payload: %v", err),
		}
	}

	// Create HTTP request
	req, err := http.NewRequestWithContext(ctx, "POST", config.WebhookURL, bytes.NewBuffer(payloadBytes))
	if err != nil {
		return WebhookDeliveryResult{
			Success:      false,
			StatusCode:   0,
			ErrorMessage: fmt.Sprintf("Failed to create request: %v", err),
		}
	}

	// Add headers
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", "LedgerService-Webhooks/1.0")
	req.Header.Set("X-Ledger-Event-ID", payload.ID)
	req.Header.Set("X-Ledger-Timestamp", strconv.FormatInt(payload.Created, 10))

	// Add signature header
	signature := s.generateSignature(payloadBytes, config.WebhookSecret)
	req.Header.Set("X-Ledger-Signature", "sha256="+signature)

	// Send request
	resp, err := s.httpClient.Do(req)
	if err != nil {
		deliveryTime := time.Since(startTime).Milliseconds()
		return WebhookDeliveryResult{
			Success:        false,
			StatusCode:     0,
			ErrorMessage:   fmt.Sprintf("HTTP request failed: %v", err),
			DeliveryTimeMs: deliveryTime,
		}
	}
	defer resp.Body.Close()

	// Read response
	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		bodyBytes = []byte("Failed to read response body")
	}

	deliveryTime := time.Since(startTime).Milliseconds()
	responseBody := string(bodyBytes)

	// Check if delivery was successful (2xx status codes)
	success := resp.StatusCode >= 200 && resp.StatusCode < 300

	result := WebhookDeliveryResult{
		Success:        success,
		StatusCode:     resp.StatusCode,
		ResponseBody:   responseBody,
		DeliveryTimeMs: deliveryTime,
	}

	if !success {
		result.ErrorMessage = fmt.Sprintf("HTTP %d: %s", resp.StatusCode, responseBody)
	}

	log.Printf("Webhook delivery to %s: %d (%dms)", config.WebhookURL, resp.StatusCode, deliveryTime)
	return result
}

// generateSignature creates HMAC-SHA256 signature for webhook payload
func (s *Service) generateSignature(payload []byte, secret string) string {
	h := hmac.New(sha256.New, []byte(secret))
	h.Write(payload)
	return hex.EncodeToString(h.Sum(nil))
}

// parseWebhookConfig extracts webhook configuration from tenant metadata
func (s *Service) parseWebhookConfig(metadata json.RawMessage) (*WebhookConfig, error) {
	if len(metadata) == 0 {
		return nil, fmt.Errorf("no metadata found")
	}

	var tenantMeta map[string]interface{}
	if err := json.Unmarshal(metadata, &tenantMeta); err != nil {
		return nil, fmt.Errorf("failed to parse metadata: %w", err)
	}

	// Extract webhook configuration
	webhookURL, ok := tenantMeta["webhook_url"].(string)
	if !ok || webhookURL == "" {
		return nil, fmt.Errorf("webhook_url not found or empty")
	}

	webhookSecret, ok := tenantMeta["webhook_secret"].(string)
	if !ok || webhookSecret == "" {
		return nil, fmt.Errorf("webhook_secret not found or empty")
	}

	// Parse webhook events (optional, defaults to all events)
	var webhookEvents []string
	if events, ok := tenantMeta["webhook_events"].([]interface{}); ok {
		for _, event := range events {
			if eventStr, ok := event.(string); ok {
				webhookEvents = append(webhookEvents, eventStr)
			}
		}
	} else {
		webhookEvents = SupportedEventTypes // Default to all supported events
	}

	// Parse enabled flag (defaults to true)
	enabled := true
	if enabledVal, ok := tenantMeta["webhook_enabled"].(bool); ok {
		enabled = enabledVal
	}

	return &WebhookConfig{
		WebhookURL:    webhookURL,
		WebhookSecret: webhookSecret,
		WebhookEvents: webhookEvents,
		Enabled:       enabled,
	}, nil
}

// shouldDeliverEvent checks if the event type should trigger webhook delivery
func (s *Service) shouldDeliverEvent(config *WebhookConfig, eventType string) bool {
	for _, configuredEvent := range config.WebhookEvents {
		if configuredEvent == eventType {
			return true
		}
	}
	return false
}

// ConfigureWebhook updates webhook configuration for a tenant
func (s *Service) ConfigureWebhook(ctx context.Context, tenantSlug string, req WebhookConfigRequest) (*WebhookConfigResponse, error) {
	// Get tenant
	tenant, err := s.db.Queries.GetTenantBySlug(ctx, tenantSlug)
	if err != nil {
		return nil, fmt.Errorf("tenant not found: %w", err)
	}

	// Parse existing metadata
	var metadata map[string]interface{}
	if len(tenant.Metadata) > 0 {
		if err := json.Unmarshal(tenant.Metadata, &metadata); err != nil {
			return nil, fmt.Errorf("failed to parse existing metadata: %w", err)
		}
	} else {
		metadata = make(map[string]interface{})
	}

	// Update with webhook configuration
	metadata["webhook_url"] = req.URL
	metadata["webhook_secret"] = req.Secret
	metadata["webhook_events"] = req.Events
	metadata["webhook_enabled"] = req.Enabled

	// Serialize updated metadata
	updatedMetadata, err := json.Marshal(metadata)
	if err != nil {
		return nil, fmt.Errorf("failed to serialize metadata: %w", err)
	}

	// Update tenant metadata in database
	updatedTenant, err := s.db.Queries.UpdateTenantMetadata(ctx, queries.UpdateTenantMetadataParams{
		ID:       tenant.ID,
		Metadata: updatedMetadata,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to update tenant metadata: %w", err)
	}

	response := &WebhookConfigResponse{
		URL:       req.URL,
		Events:    req.Events,
		Enabled:   req.Enabled,
		CreatedAt: tenant.CreatedAt,
		UpdatedAt: updatedTenant.UpdatedAt,
	}

	log.Printf("Webhook configured for tenant %s: %s (events: %v)", tenantSlug, req.URL, req.Events)
	return response, nil
}

// ListWebhookDeliveries returns webhook delivery history for a tenant
func (s *Service) ListWebhookDeliveries(ctx context.Context, tenantSlug string, limit int) ([]WebhookDeliveryResponse, error) {
	// Get tenant
	tenant, err := s.db.Queries.GetTenantBySlug(ctx, tenantSlug)
	if err != nil {
		return nil, fmt.Errorf("tenant not found: %w", err)
	}

	// Get webhook deliveries for this tenant
	deliveries, err := s.db.Queries.GetWebhookDeliveriesByTenant(ctx, queries.GetWebhookDeliveriesByTenantParams{
		TenantID: tenant.ID,
		Limit:    int32(limit),
	})
	if err != nil {
		return nil, fmt.Errorf("failed to get webhook deliveries: %w", err)
	}

	// Convert to response format
	var response []WebhookDeliveryResponse
	for _, delivery := range deliveries {
		// Get event details for this delivery
		event, err := s.db.Queries.GetEventByID(ctx, queries.GetEventByIDParams{
			TenantID: delivery.TenantID,
			EventID:  delivery.EventID,
		})
		if err != nil {
			continue // Skip if event not found
		}

		deliveryResponse := WebhookDeliveryResponse{
			ID:          delivery.ID.String(),
			EventID:     delivery.EventID.String(),
			EventType:   event.EventType,
			URL:         delivery.WebhookUrl,
			Attempts:    int(delivery.Attempts.Int32),
			MaxAttempts: int(delivery.MaxAttempts.Int32),
			CreatedAt:   delivery.CreatedAt,
		}

		if delivery.HttpStatusCode.Valid {
			statusCode := int(delivery.HttpStatusCode.Int32)
			deliveryResponse.StatusCode = &statusCode
		}

		if delivery.NextRetryAt.Valid {
			deliveryResponse.NextRetryAt = &delivery.NextRetryAt.Time
		}

		if delivery.DeliveredAt.Valid {
			deliveryResponse.DeliveredAt = &delivery.DeliveredAt.Time
		}

		if delivery.FailedAt.Valid {
			deliveryResponse.FailedAt = &delivery.FailedAt.Time
		}

		response = append(response, deliveryResponse)
	}

	return response, nil
}

// GetWebhookDelivery returns details of a specific webhook delivery
func (s *Service) GetWebhookDelivery(ctx context.Context, tenantSlug string, deliveryID uuid.UUID) (*WebhookDeliveryResponse, error) {
	// Get tenant
	tenant, err := s.db.Queries.GetTenantBySlug(ctx, tenantSlug)
	if err != nil {
		return nil, fmt.Errorf("tenant not found: %w", err)
	}

	// Get specific webhook delivery
	delivery, err := s.db.Queries.GetWebhookDeliveryByID(ctx, queries.GetWebhookDeliveryByIDParams{
		ID:       deliveryID,
		TenantID: tenant.ID,
	})
	if err != nil {
		return nil, fmt.Errorf("webhook delivery not found: %w", err)
	}

	// Get event details
	event, err := s.db.Queries.GetEventByID(ctx, queries.GetEventByIDParams{
		TenantID: delivery.TenantID,
		EventID:  delivery.EventID,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to get event details: %w", err)
	}

	// Build response
	response := &WebhookDeliveryResponse{
		ID:          delivery.ID.String(),
		EventID:     delivery.EventID.String(),
		EventType:   event.EventType,
		URL:         delivery.WebhookUrl,
		Attempts:    int(delivery.Attempts.Int32),
		MaxAttempts: int(delivery.MaxAttempts.Int32),
		CreatedAt:   delivery.CreatedAt,
	}

	if delivery.HttpStatusCode.Valid {
		statusCode := int(delivery.HttpStatusCode.Int32)
		response.StatusCode = &statusCode
	}

	if delivery.NextRetryAt.Valid {
		response.NextRetryAt = &delivery.NextRetryAt.Time
	}

	if delivery.DeliveredAt.Valid {
		response.DeliveredAt = &delivery.DeliveredAt.Time
	}

	if delivery.FailedAt.Valid {
		response.FailedAt = &delivery.FailedAt.Time
	}

	return response, nil
}

// RetryWebhookDelivery manually retries a failed webhook delivery
func (s *Service) RetryWebhookDelivery(ctx context.Context, tenantSlug string, deliveryID uuid.UUID) error {
	// Get tenant
	tenant, err := s.db.Queries.GetTenantBySlug(ctx, tenantSlug)
	if err != nil {
		return fmt.Errorf("tenant not found: %w", err)
	}

	// Get webhook delivery
	delivery, err := s.db.Queries.GetWebhookDeliveryByID(ctx, queries.GetWebhookDeliveryByIDParams{
		ID:       deliveryID,
		TenantID: tenant.ID,
	})
	if err != nil {
		return fmt.Errorf("webhook delivery not found: %w", err)
	}

	// Check if delivery can be retried
	if delivery.DeliveredAt.Valid {
		return fmt.Errorf("cannot retry successfully delivered webhook")
	}

	if delivery.Attempts.Int32 >= delivery.MaxAttempts.Int32 {
		return fmt.Errorf("maximum retry attempts exceeded")
	}

	// Reset delivery for immediate retry
	err = s.db.Queries.ResetWebhookDeliveryForRetry(ctx, delivery.ID)
	if err != nil {
		return fmt.Errorf("failed to reset delivery for retry: %w", err)
	}

	log.Printf("Webhook delivery %s reset for retry", deliveryID)
	return nil
}

// TestWebhook sends a test webhook to verify configuration
func (s *Service) TestWebhook(ctx context.Context, tenantSlug string) (*WebhookDeliveryResult, error) {
	// Get tenant
	tenant, err := s.db.Queries.GetTenantBySlug(ctx, tenantSlug)
	if err != nil {
		return nil, fmt.Errorf("tenant not found: %w", err)
	}

	// Parse webhook configuration
	config, err := s.parseWebhookConfig(tenant.Metadata)
	if err != nil {
		return nil, fmt.Errorf("no webhook configuration found for tenant: %w", err)
	}

	// Create test webhook payload
	testPayload := WebhookPayload{
		ID:       "evt_test_" + uuid.New().String()[:8],
		Type:     "webhook.test",
		Created:  tenant.CreatedAt.Unix(),
		Data:     json.RawMessage(`{"message": "This is a test webhook from LedgerService"}`),
		TenantID: tenant.ID.String(),
		LiveMode: false, // Test webhooks are not live mode
	}

	// Send test webhook
	result := s.deliverWebhook(ctx, config, testPayload)

	log.Printf("Test webhook sent to %s: success=%t, status=%d", config.WebhookURL, result.Success, result.StatusCode)
	return &result, nil
}
