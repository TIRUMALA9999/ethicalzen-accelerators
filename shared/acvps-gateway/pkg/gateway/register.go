package gateway

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	log "github.com/sirupsen/logrus"
)

// RegisterGateway registers the gateway with control plane and gets a gateway API key
// This exchanges a CUSTOMER API KEY for a GATEWAY API KEY
func RegisterGateway(ctx context.Context, controlPlaneURL, customerAPIKey, gatewayName string) (string, error) {
	log.Info("🔐 [GATEWAY REGISTRATION] Registering gateway with control plane...")

	// Build request
	url := fmt.Sprintf("%s/api/gateway/register", controlPlaneURL)

	payload := map[string]string{
		"gateway_name": gatewayName,
	}

	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		return "", fmt.Errorf("failed to marshal payload: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(payloadBytes))
	if err != nil {
		return "", fmt.Errorf("failed to create request: %w", err)
	}

	// Use tenant API key for registration
	req.Header.Set("X-API-Key", customerAPIKey)
	req.Header.Set("Content-Type", "application/json")

	log.WithFields(log.Fields{
		"url":          url,
		"gateway_name": gatewayName,
	}).Info("📡 Registering gateway with customer API key...")

	// Make request
	client := &http.Client{
		Timeout: 30 * time.Second,
	}

	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("failed to register gateway: %w", err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("registration failed with status %d: %s", resp.StatusCode, string(body))
	}

	// Parse response
	var response struct {
		Success   bool   `json:"success"`
		Message   string `json:"message"`
		GatewayID string `json:"gateway_id"`
		APIKey    string `json:"api_key"` // Field is "api_key", not "gateway_api_key"
		TenantID  string `json:"tenant_id"`
	}

	if err := json.Unmarshal(body, &response); err != nil {
		return "", fmt.Errorf("failed to parse response: %w", err)
	}

	if !response.Success {
		return "", fmt.Errorf("registration failed: %s", response.Message)
	}

	if response.APIKey == "" {
		return "", fmt.Errorf("no gateway API key returned")
	}

	log.WithFields(log.Fields{
		"gateway_id": response.GatewayID,
		"tenant_id":  response.TenantID,
	}).Info("✅ [GATEWAY REGISTRATION] Successfully registered and received gateway API key")

	return response.APIKey, nil
}

// RegisterAndLoadContracts registers gateway and loads contracts using the gateway API key
func RegisterAndLoadContracts(ctx context.Context, controlPlaneURL, customerAPIKey, gatewayName, tenantID string) error {
	// Step 1: Register gateway and get gateway API key
	gatewayAPIKey, err := RegisterGateway(ctx, controlPlaneURL, customerAPIKey, gatewayName)
	if err != nil {
		return fmt.Errorf("gateway registration failed: %w", err)
	}

	log.Info("✅ [GATEWAY REGISTRATION] Gateway API key obtained")

	// Step 2: Load contracts using gateway API key
	if err := LoadContractsFromControlPlane(ctx, controlPlaneURL, gatewayAPIKey, tenantID); err != nil {
		return fmt.Errorf("failed to load contracts with gateway API key: %w", err)
	}

	log.Info("✅ [GATEWAY REGISTRATION] Contracts loaded successfully")

	return nil
}
