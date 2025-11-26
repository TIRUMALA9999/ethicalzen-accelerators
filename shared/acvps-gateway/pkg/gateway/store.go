package gateway

import (
	"encoding/json"
	"fmt"
	"sync"

	"github.com/ethicalzen/acvps-gateway/pkg/contracts"
	log "github.com/sirupsen/logrus"
)

// In-Memory Contract Store
// Thread-safe storage for contracts without Redis dependency
// Contracts are loaded from backend API and stored in memory

var (
	// Global contract store (singleton)
	contractStore     = make(map[string]string) // key -> JSON string
	contractStoreLock sync.RWMutex

	// Metrics
	storeHits   uint64
	storeMisses uint64
)

// StoreContract stores a contract in memory
func StoreContract(key string, contractJSON string) {
	contractStoreLock.Lock()
	defer contractStoreLock.Unlock()

	contractStore[key] = contractJSON
	log.WithField("key", key).Debug("Contract stored in memory")
}

// GetContract retrieves a contract from memory
func GetContract(key string) (string, error) {
	contractStoreLock.RLock()
	defer contractStoreLock.RUnlock()

	if contractJSON, found := contractStore[key]; found {
		storeHits++
		log.WithFields(log.Fields{
			"key":  key,
			"hits": storeHits,
		}).Debug("Contract found in memory (cache hit)")
		return contractJSON, nil
	}

	storeMisses++
	log.WithFields(log.Fields{
		"key":    key,
		"misses": storeMisses,
	}).Debug("Contract not found in memory (cache miss)")
	return "", fmt.Errorf("contract not found: %s", key)
}

// GetContractParsed retrieves and parses a contract from memory
func GetContractParsed(key string) (*contracts.Contract, error) {
	contractJSON, err := GetContract(key)
	if err != nil {
		return nil, err
	}

	var contract contracts.Contract
	if err := json.Unmarshal([]byte(contractJSON), &contract); err != nil {
		return nil, fmt.Errorf("failed to parse contract: %w", err)
	}

	return &contract, nil
}

// DeleteContract removes a contract from memory
func DeleteContract(key string) {
	contractStoreLock.Lock()
	defer contractStoreLock.Unlock()

	delete(contractStore, key)
	log.WithField("key", key).Debug("Contract deleted from memory")
}

// ListContracts returns all contract keys
func ListContracts() []string {
	contractStoreLock.RLock()
	defer contractStoreLock.RUnlock()

	keys := make([]string, 0, len(contractStore))
	for key := range contractStore {
		keys = append(keys, key)
	}
	return keys
}

// GetStoreStats returns store statistics
func GetStoreStats() map[string]interface{} {
	contractStoreLock.RLock()
	defer contractStoreLock.RUnlock()

	hitRate := 0.0
	total := storeHits + storeMisses
	if total > 0 {
		hitRate = float64(storeHits) / float64(total)
	}

	return map[string]interface{}{
		"total_contracts": len(contractStore),
		"cache_hits":      storeHits,
		"cache_misses":    storeMisses,
		"hit_rate":        hitRate,
	}
}

// ClearStore clears all contracts (useful for testing)
func ClearStore() {
	contractStoreLock.Lock()
	defer contractStoreLock.Unlock()

	contractStore = make(map[string]string)
	storeHits = 0
	storeMisses = 0
	log.Info("In-memory contract store cleared")
}

// StoreContractObject stores a parsed contract object
func StoreContractObject(key string, contract *contracts.Contract) error {
	contractJSON, err := json.Marshal(contract)
	if err != nil {
		return fmt.Errorf("failed to marshal contract: %w", err)
	}

	StoreContract(key, string(contractJSON))
	return nil
}

// GetTenantContractKey builds a tenant-scoped contract key
// Format: contract:tenant-{tenantID}:{contractID}
func GetTenantContractKey(tenantID, contractID string) string {
	return fmt.Sprintf("contract:tenant-%s:%s", tenantID, contractID)
}

// GetBackendContractKey builds a backend-style contract key
// Format: tenant:{tenantID}:contract:{contractID}
func GetBackendContractKey(tenantID, contractID string) string {
	return fmt.Sprintf("tenant:%s:contract:%s", tenantID, contractID)
}

// TryBothKeyFormats attempts to get a contract using both key formats
// First tries runtime format, then backend format
func TryBothKeyFormats(tenantID, contractID string) (string, string, error) {
	// Try runtime key format first: contract:tenant-demo:CONTRACT_ID
	runtimeKey := GetTenantContractKey(tenantID, contractID)
	if contractJSON, err := GetContract(runtimeKey); err == nil {
		return contractJSON, runtimeKey, nil
	}

	// Try backend key format: tenant:demo:contract:CONTRACT_ID
	backendKey := GetBackendContractKey(tenantID, contractID)
	if contractJSON, err := GetContract(backendKey); err == nil {
		return contractJSON, backendKey, nil
	}

	return "", "", fmt.Errorf("contract not found in either format: %s (tenant: %s)", contractID, tenantID)
}

