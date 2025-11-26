package gateway

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	log "github.com/sirupsen/logrus"
)

// SubscribeToBackendEvents subscribes to Server-Sent Events (SSE) from the backend
// This replaces polling with real-time push notifications for:
// - New contract registrations
// - Contract updates/approvals
// - Guardrail deployments
// - Policy changes
//
// Benefits over polling:
// - Real-time updates (no 5-minute delay)
// - Reduced network traffic
// - Lower backend load
// - Better user experience
func SubscribeToBackendEvents(ctx context.Context, backendURL, apiKey string) error {
	if backendURL == "" {
		return fmt.Errorf("backend URL is required")
	}

	// Build SSE endpoint URL
	sseURL := fmt.Sprintf("%s/api/gateway/events", backendURL)

	log.WithFields(log.Fields{
		"backend_url": backendURL,
		"sse_url":     sseURL,
	}).Info("📡 Subscribing to backend events (SSE)...")

	// Create HTTP request with SSE headers
	req, err := http.NewRequestWithContext(ctx, "GET", sseURL, nil)
	if err != nil {
		return fmt.Errorf("failed to create SSE request: %w", err)
	}

	// Add API key for authentication
	if apiKey != "" {
		req.Header.Set("X-API-Key", apiKey)
	}

	// SSE-specific headers
	req.Header.Set("Accept", "text/event-stream")
	req.Header.Set("Cache-Control", "no-cache")
	req.Header.Set("Connection", "keep-alive")

	// Send request
	client := &http.Client{
		Timeout: 0, // No timeout for SSE (long-lived connection)
	}

	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to connect to backend SSE: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		resp.Body.Close()
		return fmt.Errorf("backend SSE failed: HTTP %d", resp.StatusCode)
	}

	log.Info("✅ Connected to backend event stream")

	// Start reading events
	go func() {
		defer resp.Body.Close()

		scanner := bufio.NewScanner(resp.Body)
		var eventType string
		var eventData string

		for scanner.Scan() {
			line := scanner.Text()

			// SSE format:
			// event: contract_registered
			// data: {"contract_id": "..."}
			//
			// (blank line marks end of event)

			if line == "" {
				// End of event - process it
				if eventType != "" && eventData != "" {
					handleBackendEvent(eventType, eventData)
					eventType = ""
					eventData = ""
				}
				continue
			}

			if len(line) > 6 && line[:6] == "event:" {
				eventType = line[7:]
			} else if len(line) > 5 && line[:5] == "data:" {
				eventData = line[6:]
			}
		}

		if err := scanner.Err(); err != nil {
			log.WithError(err).Warn("⚠️  SSE connection closed")
		}

		log.Info("📡 Backend event stream disconnected")
	}()

	return nil
}

// handleBackendEvent processes events from the backend SSE stream
func handleBackendEvent(eventType, eventData string) {
	log.WithFields(log.Fields{
		"event_type": eventType,
		"data_len":   len(eventData),
	}).Debug("📨 Received backend event")

	switch eventType {
	case "contract_registered":
		handleContractRegistered(eventData)

	case "contract_updated":
		handleContractUpdated(eventData)

	case "guardrail_deployed":
		handleGuardrailDeployed(eventData)

	case "policy_updated":
		handlePolicyUpdated(eventData)

	case "ping":
		// Keepalive ping, no action needed
		log.Debug("📡 Received keepalive ping")

	default:
		log.WithField("event_type", eventType).Warn("⚠️  Unknown event type")
	}
}

// handleContractRegistered processes contract registration events
func handleContractRegistered(eventData string) {
	var event struct {
		ContractID string                 `json:"contract_id"`
		TenantID   string                 `json:"tenant_id"`
		Contract   map[string]interface{} `json:"contract"`
	}

	if err := json.Unmarshal([]byte(eventData), &event); err != nil {
		log.WithError(err).Warn("⚠️  Failed to parse contract_registered event")
		return
	}

	log.WithFields(log.Fields{
		"contract_id": event.ContractID,
		"tenant_id":   event.TenantID,
	}).Info("📝 New contract registered (via SSE)")

	// Marshal contract back to JSON
	contractJSON, err := json.Marshal(event.Contract)
	if err != nil {
		log.WithError(err).Warn("⚠️  Failed to marshal contract")
		return
	}

	// Store using both key formats for compatibility
	runtimeKey := GetTenantContractKey(event.TenantID, event.ContractID)
	backendKey := GetBackendContractKey(event.TenantID, event.ContractID)

	StoreContract(runtimeKey, string(contractJSON))
	StoreContract(backendKey, string(contractJSON))

	log.WithField("contract_id", event.ContractID).Info("✅ Contract loaded from SSE event")

	// Log store stats
	stats := GetStoreStats()
	log.WithField("total_contracts", stats["total_contracts"]).Debug("📊 In-memory store updated")
}

// handleContractUpdated processes contract update events
func handleContractUpdated(eventData string) {
	// Similar to handleContractRegistered
	handleContractRegistered(eventData) // Reuse same logic for now
}

// handleGuardrailDeployed processes guardrail deployment events
func handleGuardrailDeployed(eventData string) {
	var event struct {
		GuardrailID string `json:"guardrail_id"`
		TenantID    string `json:"tenant_id"`
	}

	if err := json.Unmarshal([]byte(eventData), &event); err != nil {
		log.WithError(err).Warn("⚠️  Failed to parse guardrail_deployed event")
		return
	}

	log.WithFields(log.Fields{
		"guardrail_id": event.GuardrailID,
		"tenant_id":    event.TenantID,
	}).Info("🛡️  New guardrail deployed (via SSE)")

	// TODO: Trigger guardrail reload for this tenant
	// This would call txrepo.LoadTenantGuardrails(event.TenantID)
}

// handlePolicyUpdated processes policy update events
func handlePolicyUpdated(eventData string) {
	var event struct {
		PolicyID string `json:"policy_id"`
		TenantID string `json:"tenant_id"`
	}

	if err := json.Unmarshal([]byte(eventData), &event); err != nil {
		log.WithError(err).Warn("⚠️  Failed to parse policy_updated event")
		return
	}

	log.WithFields(log.Fields{
		"policy_id": event.PolicyID,
		"tenant_id": event.TenantID,
	}).Info("📜 Policy updated (via SSE)")

	// TODO: Trigger contract reload for affected contracts
}

// StartBackendEventSubscription starts the SSE subscription and auto-reconnects on disconnect
func StartBackendEventSubscription(ctx context.Context, backendURL, apiKey string) {
	log.Info("🔄 Starting backend event subscription with auto-reconnect...")

	for {
		select {
		case <-ctx.Done():
			log.Info("🛑 Stopping backend event subscription")
			return

		default:
			err := SubscribeToBackendEvents(ctx, backendURL, apiKey)
			if err != nil {
				log.WithError(err).Warn("⚠️  Failed to subscribe to backend events, retrying in 30s...")
				time.Sleep(30 * time.Second)
			} else {
				// If subscription returns without error, it means the connection closed cleanly
				log.Info("📡 Backend event stream closed, reconnecting in 5s...")
				time.Sleep(5 * time.Second)
			}
		}
	}
}

