package gateway

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/ethicalzen/acvps-gateway/pkg/contracts"
	log "github.com/sirupsen/logrus"
)

// LoadContractsFromControlPlane loads contracts via HTTP API (for local mode)
func LoadContractsFromControlPlane(ctx context.Context, controlPlaneURL, apiKey, tenantID string) error {
	log.Info("🌐 [LOCAL MODE] Loading contracts from control plane...")

	// Build request
	url := fmt.Sprintf("%s/api/gateway/contracts", controlPlaneURL)
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	// Add tenant API key for authentication
	req.Header.Set("X-API-Key", apiKey)

	log.WithFields(log.Fields{
		"url":       url,
		"tenant_id": tenantID,
	}).Info("📡 Syncing contracts from control plane...")

	// Make request
	client := &http.Client{
		Timeout: 30 * time.Second,
	}

	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to fetch contracts: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("control plane returned status %d: %s", resp.StatusCode, string(body))
	}

	// Parse response
	var response struct {
		Success       bool                 `json:"success"`
		TenantID      string               `json:"tenant_id"`
		ContractCount int                  `json:"contract_count"`
		Contracts     []contracts.Contract `json:"contracts"`
		SyncedAt      string               `json:"synced_at"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&response); err != nil {
		return fmt.Errorf("failed to parse response: %w", err)
	}

	if !response.Success {
		return fmt.Errorf("control plane returned success=false")
	}

	log.WithFields(log.Fields{
		"tenant_id":      response.TenantID,
		"contract_count": response.ContractCount,
		"synced_at":      response.SyncedAt,
	}).Info("✅ [LOCAL MODE] Contracts fetched from control plane")

	// Load each contract into runtime table
	loaded := 0
	skipped := 0

	for _, contract := range response.Contracts {
		// Get contract ID (use ID field if ContractID is empty)
		contractID := contract.ContractID
		if contractID == "" {
			contractID = contract.ID
		}

		// Skip if no ID found
		if contractID == "" {
			log.Warn("⏭️  Skipping contract with no ID")
			skipped++
			continue
		}

		// Normalize ContractID field for consistency
		if contract.ContractID == "" && contract.ID != "" {
			contract.ContractID = contract.ID
		}

		// Only load approved/active contracts
		if contract.Status != "approved" && contract.Status != "active" && !contract.Approved {
			log.Infof("⏭️  Skipping non-active contract: %s (status: %s, approved: %v)", contractID, contract.Status, contract.Approved)
			skipped++
			continue
		}

		// Build tenant-scoped key (must match what validation handler expects)
		// Format: contract:tenant-{TENANT_ID}:{CONTRACT_ID}
		tenantScopedKey := fmt.Sprintf("contract:tenant-%s:%s", tenantID, contractID)

		// Load into runtime table with tenant-scoped key
		if err := LoadContract(ctx, tenantScopedKey, &contract); err != nil {
			log.WithError(err).Warnf("Failed to load contract %s", contractID)
			skipped++
			continue
		}

		loaded++
		log.Infof("✅ Loaded contract: %s (suite: %s)", contractID, contract.Suite)
	}

	log.WithFields(log.Fields{
		"loaded":  loaded,
		"skipped": skipped,
		"total":   len(response.Contracts),
	}).Info("📊 [LOCAL MODE] Contract loading complete")

	return nil
}

// StartPeriodicSync periodically syncs contracts from control plane (for local mode)
func StartPeriodicSync(ctx context.Context, controlPlaneURL, apiKey, tenantID string, interval time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	log.Infof("🔄 [LOCAL MODE] Periodic sync enabled: checking for contract updates every %v", interval)

	for {
		select {
		case <-ctx.Done():
			log.Info("[LOCAL MODE] Periodic sync stopped")
			return
		case <-ticker.C:
			log.Debug("🔍 [LOCAL MODE] Checking for contract updates...")
			if err := LoadContractsFromControlPlane(ctx, controlPlaneURL, apiKey, tenantID); err != nil {
				log.WithError(err).Warn("[LOCAL MODE] Failed to sync contracts from control plane")
			} else {
				log.Info("✅ [LOCAL MODE] Contracts synced successfully")
			}
		}
	}
}
