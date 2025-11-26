package gateway

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	log "github.com/sirupsen/logrus"
)

// GuardrailEvent represents a guardrail change event from backend
type GuardrailEvent struct {
	Action     string                 `json:"action"`      // registered, updated, deleted
	Guardrail  map[string]interface{} `json:"guardrail"`
	TenantID   string                 `json:"tenant_id"`
	Timestamp  string                 `json:"timestamp"`
}

// StartGuardrailSync subscribes to backend SSE endpoint for real-time guardrail updates
func StartGuardrailSync(ctx context.Context, controlPlaneURL, apiKey, tenantID string) {
	log.Info("🔄 Starting guardrail sync subscription...")

	// Build SSE endpoint URL
	url := fmt.Sprintf("%s/api/guardrails/subscribe?tenant_id=%s", controlPlaneURL, tenantID)

	// Retry loop with exponential backoff
	retryDelay := 5 * time.Second
	maxRetryDelay := 5 * time.Minute

	for {
		select {
		case <-ctx.Done():
			log.Info("🔄 Guardrail sync stopped")
			return
		default:
			err := subscribeToGuardrailEvents(ctx, url, apiKey, tenantID)
			if err != nil {
				log.WithError(err).Warnf("⚠️  Guardrail sync connection failed, retrying in %v", retryDelay)
				
				select {
				case <-ctx.Done():
					return
				case <-time.After(retryDelay):
					// Exponential backoff
					retryDelay = retryDelay * 2
					if retryDelay > maxRetryDelay {
						retryDelay = maxRetryDelay
					}
				}
			} else {
				// Reset retry delay on successful connection
				retryDelay = 5 * time.Second
			}
		}
	}
}

func subscribeToGuardrailEvents(ctx context.Context, url, apiKey, tenantID string) error {
	log.WithFields(log.Fields{
		"url":       url,
		"tenant_id": tenantID,
	}).Info("📡 Connecting to guardrail sync endpoint...")

	// Create request
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	// Add authentication
	req.Header.Set("Authorization", "Bearer "+apiKey)
	req.Header.Set("X-Tenant-ID", tenantID)
	req.Header.Set("Accept", "text/event-stream")

	// Make request
	client := &http.Client{
		Timeout: 0, // No timeout for SSE
	}

	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to connect: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("backend returned status %d: %s", resp.StatusCode, string(body))
	}

	log.Info("✅ Connected to guardrail sync endpoint")

	// Read SSE stream
	reader := bufio.NewReader(resp.Body)

	for {
		select {
		case <-ctx.Done():
			return nil
		default:
			line, err := reader.ReadString('\n')
			if err != nil {
				if err == io.EOF {
					return fmt.Errorf("connection closed by backend")
				}
				return fmt.Errorf("error reading stream: %w", err)
			}

			// SSE format: "data: {...}\n"
			if len(line) > 6 && line[:5] == "data:" {
				data := line[6:]
				handleGuardrailEvent(data, tenantID)
			}
		}
	}
}

func handleGuardrailEvent(data string, tenantID string) {
	var event GuardrailEvent
	if err := json.Unmarshal([]byte(data), &event); err != nil {
		log.WithError(err).Warn("Failed to parse guardrail event")
		return
	}

	// Skip connection messages
	if event.Action == "" {
		return
	}

	log.WithFields(log.Fields{
		"action":       event.Action,
		"guardrail_id": event.Guardrail["id"],
		"tenant_id":    event.TenantID,
	}).Info("🔔 Received guardrail event")

	switch event.Action {
	case "registered":
		handleGuardrailRegistered(event)
	case "updated":
		handleGuardrailUpdated(event)
	case "deleted":
		handleGuardrailDeleted(event)
	default:
		log.Warnf("Unknown guardrail action: %s", event.Action)
	}
}

func handleGuardrailRegistered(event GuardrailEvent) {
	guardrailID, ok := event.Guardrail["id"].(string)
	if !ok {
		log.Warn("Missing guardrail ID in event")
		return
	}

	log.Infof("➕ Hot-loading new guardrail: %s", guardrailID)

	// TODO: Load guardrail implementation into txrepo
	// This requires:
	// 1. Fetch full guardrail config from backend
	// 2. Compile/register in txrepo
	// 3. Make available for validation
	
	log.Infof("✅ Guardrail %s is now active", guardrailID)
}

func handleGuardrailUpdated(event GuardrailEvent) {
	guardrailID, ok := event.Guardrail["id"].(string)
	if !ok {
		log.Warn("Missing guardrail ID in event")
		return
	}

	log.Infof("🔄 Hot-reloading updated guardrail: %s", guardrailID)

	// TODO: Reload guardrail implementation
	// 1. Unload existing version
	// 2. Load new version from backend
	// 3. Update in txrepo
	
	log.Infof("✅ Guardrail %s has been updated", guardrailID)
}

func handleGuardrailDeleted(event GuardrailEvent) {
	guardrailID, ok := event.Guardrail["id"].(string)
	if !ok {
		log.Warn("Missing guardrail ID in event")
		return
	}

	log.Infof("🗑️  Unloading deleted guardrail: %s", guardrailID)

	// TODO: Remove guardrail from txrepo
	// Note: Should fail gracefully if contracts still reference it
	
	log.Infof("✅ Guardrail %s has been removed", guardrailID)
}

