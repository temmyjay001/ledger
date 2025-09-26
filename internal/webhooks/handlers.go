// internal/webhooks/handlers.go
package webhooks

import (
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
	"github.com/go-playground/validator/v10"
	"github.com/google/uuid"
	"github.com/temmyjay001/ledger-service/pkg/api"
	cV "github.com/temmyjay001/ledger-service/pkg/validator"
)

type Handlers struct {
	service   *Service
	validator *validator.Validate
}

func NewHandlers(service *Service) *Handlers {
	return &Handlers{
		service:   service,
		validator: cV.GetValidator(),
	}
}

// ConfigureWebhookHandler handles webhook configuration for a tenant
func (h *Handlers) ConfigureWebhookHandler(w http.ResponseWriter, r *http.Request) {
	tenantSlug := chi.URLParam(r, "tenantSlug")

	var req WebhookConfigRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		api.WriteBadRequestResponse(w, "invalid JSON payload")
		return
	}

	if err := h.validator.Struct(req); err != nil {
		api.WriteValidationErrorResponse(w, err)
		return
	}

	// Call service method
	response, err := h.service.ConfigureWebhook(r.Context(), tenantSlug, req)
	if err != nil {
		api.WriteInternalErrorResponse(w, err.Error())
		return
	}

	api.WriteSuccessResponse(w, http.StatusOK, response)
}

// ListWebhookDeliveriesHandler returns webhook delivery history for a tenant
func (h *Handlers) ListWebhookDeliveriesHandler(w http.ResponseWriter, r *http.Request) {
	tenantSlug := chi.URLParam(r, "tenantSlug")

	// Parse query parameters
	limitStr := r.URL.Query().Get("limit")
	limit := 50 // default
	if limitStr != "" {
		if l, err := strconv.Atoi(limitStr); err == nil && l > 0 && l <= 100 {
			limit = l
		}
	}

	// Call service method
	deliveries, err := h.service.ListWebhookDeliveries(r.Context(), tenantSlug, limit)
	if err != nil {
		api.WriteInternalErrorResponse(w, err.Error())
		return
	}

	api.WriteSuccessResponse(w, http.StatusOK, map[string]interface{}{
		"deliveries": deliveries,
		"total":      len(deliveries),
	})
}

// GetWebhookDeliveryHandler returns details of a specific webhook delivery
func (h *Handlers) GetWebhookDeliveryHandler(w http.ResponseWriter, r *http.Request) {
	tenantSlug := chi.URLParam(r, "tenantSlug")
	deliveryID := chi.URLParam(r, "deliveryId")

	// Parse delivery ID
	deliveryUUID, err := uuid.Parse(deliveryID)
	if err != nil {
		api.WriteBadRequestResponse(w, "Invalid delivery ID")
		return
	}

	// Call service method
	delivery, err := h.service.GetWebhookDelivery(r.Context(), tenantSlug, deliveryUUID)
	if err != nil {
		api.WriteErrorResponse(w, http.StatusNotFound, "Webhook delivery not found")
		return
	}

	api.WriteSuccessResponse(w, http.StatusOK, delivery)
}

// RetryWebhookDeliveryHandler manually retries a failed webhook delivery
func (h *Handlers) RetryWebhookDeliveryHandler(w http.ResponseWriter, r *http.Request) {
	tenantSlug := chi.URLParam(r, "tenantSlug")
	deliveryID := chi.URLParam(r, "deliveryId")

	// Parse delivery ID
	deliveryUUID, err := uuid.Parse(deliveryID)
	if err != nil {
		api.WriteBadRequestResponse(w, "Invalid delivery ID")
		return
	}

	// Call service method
	err = h.service.RetryWebhookDelivery(r.Context(), tenantSlug, deliveryUUID)
	if err != nil {
		api.WriteBadRequestResponse(w, err.Error())
		return
	}

	api.WriteSuccessResponse(w, http.StatusOK, map[string]interface{}{
		"message":     "Webhook delivery scheduled for retry",
		"delivery_id": deliveryUUID,
	})
}

// TestWebhookHandler sends a test webhook to verify configuration
func (h *Handlers) TestWebhookHandler(w http.ResponseWriter, r *http.Request) {
	tenantSlug := chi.URLParam(r, "tenantSlug")

	// Call service method
	result, err := h.service.TestWebhook(r.Context(), tenantSlug)
	if err != nil {
		api.WriteBadRequestResponse(w, err.Error())
		return
	}

	// Return result
	response := map[string]interface{}{
		"success":          result.Success,
		"status_code":      result.StatusCode,
		"delivery_time_ms": result.DeliveryTimeMs,
	}

	if !result.Success {
		response["error"] = result.ErrorMessage
	} else {
		response["message"] = "Test webhook delivered successfully"
	}

	if result.Success {
		api.WriteSuccessResponse(w, http.StatusOK, response)
	} else {
		api.WriteErrorResponse(w, http.StatusBadRequest, "Test webhook failed")
	}
}
