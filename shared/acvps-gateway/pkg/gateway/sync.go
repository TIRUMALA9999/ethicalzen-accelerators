package gateway

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	log "github.com/sirupsen/logrus"
)

// SyncContractsFromBackend fetches contracts from the backend API and stores them in memory
// This is used when Redis is disabled to populate the in-memory store
func SyncContractsFromBackend(ctx context.Context, backendURL, apiKey string) error {
	if backendURL == "" {
		return fmt.Errorf("backend URL is required")
	}

	// Build sync endpoint URL
	syncURL := fmt.Sprintf("%s/api/gateway/sync", backendURL)

	log.WithFields(log.Fields{
		"backend_url": backendURL,
		"sync_url":    syncURL,
	}).Info("📡 Syncing contracts from backend API...")

	// Create HTTP request
	req, err := http.NewRequestWithContext(ctx, "GET", syncURL, nil)
	if err != nil {
		return fmt.Errorf("failed to create sync request: %w", err)
	}

	// Add API key if provided
	if apiKey != "" {
		req.Header.Set("X-API-Key", apiKey)
	}

	// Send request
	client := &http.Client{
		Timeout: 30 * time.Second,
	}

	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to sync from backend: %w", err)
	}
	defer resp.Body.Close()

	// Check response status
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("backend sync failed: HTTP %d - %s", resp.StatusCode, string(body))
	}

	// Parse response
	var syncResponse struct {
		Success    bool                     `json:"success"`
		Data       *SyncData                `json:"data"`
		Contracts  []map[string]interface{} `json:"contracts"`  // Alternative format
		Guardrails []map[string]interface{} `json:"guardrails"` // Alternative format
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("failed to read sync response: %w", err)
	}

	if err := json.Unmarshal(body, &syncResponse); err != nil {
		return fmt.Errorf("failed to parse sync response: %w", err)
	}

	// Extract contracts (handle both formats)
	var contracts []map[string]interface{}
	if syncResponse.Data != nil && len(syncResponse.Data.Contracts) > 0 {
		contracts = syncResponse.Data.Contracts
	} else if len(syncResponse.Contracts) > 0 {
		contracts = syncResponse.Contracts
	} else {
		log.Warn("⚠️  No contracts returned from backend sync")
		return nil
	}

	// Store each contract in memory
	storedCount := 0
	for _, contractData := range contracts {
		// Extract contract ID and tenant ID
		contractID, _ := contractData["id"].(string)
		if contractID == "" {
			contractID, _ = contractData["contract_id"].(string)
		}

		tenantID, _ := contractData["tenant_id"].(string)
		if tenantID == "" {
			tenantID = "demo" // Default tenant
		}

		if contractID == "" {
			log.Warn("⚠️  Skipping contract with no ID")
			continue
		}

		// Marshal contract back to JSON
		contractJSON, err := json.Marshal(contractData)
		if err != nil {
			log.WithError(err).Warnf("Failed to marshal contract %s", contractID)
			continue
		}

		// Store using both key formats for compatibility
		runtimeKey := GetTenantContractKey(tenantID, contractID)
		backendKey := GetBackendContractKey(tenantID, contractID)

		StoreContract(runtimeKey, string(contractJSON))
		StoreContract(backendKey, string(contractJSON))

		// ✅ CRITICAL: Load contract into runtime binding table for validation
		// Parse contract using the store's helper function
		contract, err := GetContractParsed(runtimeKey)
		if err != nil {
			log.WithError(err).Warnf("Failed to parse contract %s", contractID)
			continue
		}

		if err := LoadContract(ctx, runtimeKey, contract); err != nil {
			log.WithError(err).Warnf("Failed to load contract %s into runtime", contractID)
		} else {
			log.WithFields(log.Fields{
				"contract_id": contractID,
				"tenant_id":   tenantID,
			}).Debug("✅ Contract loaded into runtime table")
		}

		storedCount++
	}

	log.WithFields(log.Fields{
		"total_contracts": len(contracts),
		"stored":          storedCount,
	}).Info("✅ Contracts synced from backend")

	// Log store stats
	stats := GetStoreStats()
	log.WithFields(log.Fields{
		"total_contracts": stats["total_contracts"],
	}).Info("📊 In-memory store updated")

	return nil
}

// SyncData represents the response from /api/gateway/sync
type SyncData struct {
	Contracts  []map[string]interface{} `json:"contracts"`
	Guardrails []map[string]interface{} `json:"guardrails"`
}

// StartPeriodicBackendSync starts a goroutine that periodically syncs contracts from backend
func StartPeriodicBackendSync(ctx context.Context, backendURL, apiKey string, interval time.Duration) {
	if backendURL == "" {
		log.Warn("⚠️  Backend URL not configured, periodic sync disabled")
		return
	}

	log.WithField("interval", interval).Info("🔄 Starting periodic backend sync")

	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			log.Info("🛑 Stopping periodic backend sync")
			return

		case <-ticker.C:
			log.Debug("⏰ Periodic sync triggered")
			if err := SyncContractsFromBackend(ctx, backendURL, apiKey); err != nil {
				log.WithError(err).Warn("⚠️  Periodic sync failed")
			}
		}
	}
}

