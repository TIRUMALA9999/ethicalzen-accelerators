package gateway

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	log "github.com/sirupsen/logrus"
)

// RegisterWebhook registers the gateway's webhook URL with the backend
// Backend will POST to this URL when events occur (contract_registered, etc.)
func RegisterWebhook(backendURL, apiKey, webhookURL, gatewayID string) error {
	url := fmt.Sprintf("%s/api/webhooks/register", backendURL)

	payload := map[string]string{
		"webhook_url": webhookURL,
		"gateway_id":  gatewayID,
	}

	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("failed to marshal webhook registration payload: %w", err)
	}

	req, err := http.NewRequest("POST", url, bytes.NewReader(payloadBytes))
	if err != nil {
		return fmt.Errorf("failed to create webhook registration request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-API-Key", apiKey)

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to register webhook: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("webhook registration failed with status: %d", resp.StatusCode)
	}

	var result map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return fmt.Errorf("failed to parse webhook registration response: %w", err)
	}

	log.WithFields(log.Fields{
		"webhook_url":    webhookURL,
		"backend_url":    backendURL,
		"total_webhooks": result["total_webhooks"],
	}).Info("🔔 Webhook registered with backend")

	return nil
}

// UnregisterWebhook unregisters the webhook URL from the backend
func UnregisterWebhook(backendURL, apiKey, webhookURL string) error {
	url := fmt.Sprintf("%s/api/webhooks/unregister", backendURL)

	payload := map[string]string{
		"webhook_url": webhookURL,
	}

	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("failed to marshal webhook unregistration payload: %w", err)
	}

	req, err := http.NewRequest("DELETE", url, bytes.NewReader(payloadBytes))
	if err != nil {
		return fmt.Errorf("failed to create webhook unregistration request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-API-Key", apiKey)

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to unregister webhook: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("webhook unregistration failed with status: %d", resp.StatusCode)
	}

	log.WithField("webhook_url", webhookURL).Info("🔔 Webhook unregistered from backend")

	return nil
}

