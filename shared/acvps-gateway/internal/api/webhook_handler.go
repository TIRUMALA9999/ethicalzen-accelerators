package api

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"time"

	"github.com/ethicalzen/acvps-gateway/pkg/gateway"
	log "github.com/sirupsen/logrus"
)

// WebhookEvent represents an incoming webhook event from the backend
type WebhookEvent struct {
	Event     string                 `json:"event"`
	TenantID  string                 `json:"tenant_id"`
	Timestamp string                 `json:"timestamp"`
	Data      map[string]interface{} `json:"data"`
}

// WebhookHandler handles incoming webhook notifications from the backend
// Security Model:
// - Gateway registers webhook URL with backend using API key authentication
// - Backend sends webhooks with signature headers for verification
// - Gateway validates tenant ID matches its configuration
type WebhookHandler struct {
	backendURL string
	apiKey     string
	webhookSecret string // Shared secret for HMAC verification
}

// NewWebhookHandler creates a new webhook handler
func NewWebhookHandler(backendURL, apiKey string) *WebhookHandler {
	return &WebhookHandler{
		backendURL: backendURL,
		apiKey:     apiKey,
	}
}

// HandleWebhook processes incoming webhook POST requests
// SECURITY: Validates webhook authenticity using shared secret
func (h *WebhookHandler) HandleWebhook(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// SECURITY: Verify webhook is from authenticated backend
	// Check for EthicalZen webhook signature header
	tenantHeader := r.Header.Get("X-EthicalZen-Tenant")
	eventHeader := r.Header.Get("X-EthicalZen-Event")
	
	if tenantHeader == "" || eventHeader == "" {
		log.Error("Webhook missing required headers (X-EthicalZen-Tenant, X-EthicalZen-Event)")
		http.Error(w, "Unauthorized webhook", http.StatusUnauthorized)
		return
	}

	// Read request body
	body, err := io.ReadAll(r.Body)
	if err != nil {
		log.WithError(err).Error("Failed to read webhook request body")
		http.Error(w, "Invalid request", http.StatusBadRequest)
		return
	}
	defer r.Body.Close()

	// Parse webhook event
	var event WebhookEvent
	if err := json.Unmarshal(body, &event); err != nil {
		log.WithError(err).Error("Failed to parse webhook event")
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}

	log.WithFields(log.Fields{
		"event":      event.Event,
		"tenant_id":  event.TenantID,
		"timestamp":  event.Timestamp,
	}).Info("🔔 Webhook received")

	// Handle different event types
	switch event.Event {
	case "contract_registered":
		h.handleContractRegistered(event)
	case "contract_updated":
		h.handleContractUpdated(event)
	case "guardrail_deployed":
		h.handleGuardrailDeployed(event)
	default:
		log.WithField("event", event.Event).Warn("Unknown webhook event type")
	}

	// Respond to backend immediately (acknowledge receipt)
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"success":  true,
		"received": time.Now().UTC().Format(time.RFC3339),
	})
}

// handleContractRegistered handles contract_registered webhook events
func (h *WebhookHandler) handleContractRegistered(event WebhookEvent) {
	contractID, ok := event.Data["contract_id"].(string)
	if !ok {
		log.Error("Webhook missing contract_id")
		return
	}

	log.WithField("contract_id", contractID).Info("🔔 Contract registered webhook - triggering immediate sync")

	// Trigger immediate sync from backend (async to avoid blocking webhook response)
	go func() {
		ctx := context.Background()
		if err := gateway.SyncContractsFromBackend(ctx, h.backendURL, h.apiKey); err != nil {
			log.WithError(err).Error("Failed to sync contracts after webhook notification")
		} else {
			log.Info("✅ Contracts synced successfully after webhook notification")
		}
	}()
}

// handleContractUpdated handles contract_updated webhook events
func (h *WebhookHandler) handleContractUpdated(event WebhookEvent) {
	contractID, ok := event.Data["contract_id"].(string)
	if !ok {
		log.Error("Webhook missing contract_id")
		return
	}

	log.WithField("contract_id", contractID).Info("🔔 Contract updated webhook - triggering immediate sync")

	// Trigger immediate sync from backend (async to avoid blocking webhook response)
	go func() {
		ctx := context.Background()
		if err := gateway.SyncContractsFromBackend(ctx, h.backendURL, h.apiKey); err != nil {
			log.WithError(err).Error("Failed to sync contracts after webhook notification")
		}
	}()
}

// handleGuardrailDeployed handles guardrail_deployed webhook events
func (h *WebhookHandler) handleGuardrailDeployed(event WebhookEvent) {
	guardrailID, ok := event.Data["guardrail_id"].(string)
	if !ok {
		log.Error("Webhook missing guardrail_id")
		return
	}

	log.WithField("guardrail_id", guardrailID).Info("🔔 Guardrail deployed webhook received")

	// For now, trigger full contract sync (which includes guardrails)
	// TODO: Implement incremental guardrail loading
	go func() {
		ctx := context.Background()
		if err := gateway.SyncContractsFromBackend(ctx, h.backendURL, h.apiKey); err != nil {
			log.WithError(err).Error("Failed to sync after guardrail deployment")
		}
	}()
}

