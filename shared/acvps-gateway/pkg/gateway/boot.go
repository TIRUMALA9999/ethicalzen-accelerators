package gateway

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/ethicalzen/acvps-gateway/internal/cache"
	"github.com/ethicalzen/acvps-gateway/pkg/contracts"
	"github.com/ethicalzen/acvps-gateway/pkg/txrepo"
	log "github.com/sirupsen/logrus"
)

// RuntimeBinding holds everything needed for runtime validation
type RuntimeBinding struct {
	Contract      *contracts.Contract
	ExtractorFunc txrepo.FeatureExtractorFunc
	ExtractorMeta txrepo.ExtractorMetadata
	LoadedAt      time.Time
}

// ContractRuntimeTable is the global runtime contract table
var (
	ContractRuntimeTable map[string]*RuntimeBinding
	tableMutex           sync.RWMutex
)

func init() {
	ContractRuntimeTable = make(map[string]*RuntimeBinding)
}

// LoadAllContractsAtBoot loads all contracts from Redis and prepares runtime table
func LoadAllContractsAtBoot(ctx context.Context, cacheClient *cache.Client) error {
	log.Info("🔄 Loading contracts at boot...")

	// Scan Redis for all tenant-scoped contract keys (new format: tenant:*:contract:*)
	keys, err := cacheClient.Scan(ctx, "tenant:*:contract:*")
	if err != nil {
		log.WithError(err).Warn("Failed to scan Redis for tenant-scoped contracts, trying legacy format...")
		// Fallback to legacy format
		keys, err = cacheClient.Scan(ctx, "contract:*")
		if err != nil {
			return fmt.Errorf("failed to scan Redis for contracts: %w", err)
		}
	}

	log.Infof("📊 Found %d contract keys in Redis", len(keys))

	loaded := 0
	skipped := 0
	approved := 0

	for _, key := range keys {
		// Extract tenant ID and contract ID from key
		var contractID string
		var tenantID string
		var runtimeKey string

		if strings.HasPrefix(key, "tenant:") {
			// New format: tenant:TENANT_ID:contract:CONTRACT_ID
			parts := strings.SplitN(key, ":contract:", 2)
			if len(parts) == 2 {
				contractID = parts[1]
				// Extract tenant ID from parts[0]: "tenant:TENANT_ID"
				tenantID = strings.TrimPrefix(parts[0], "tenant:")
				// Build runtime key: contract:tenant-TENANT_ID:CONTRACT_ID
				runtimeKey = fmt.Sprintf("contract:tenant-%s:%s", tenantID, contractID)
			} else {
				log.Warnf("Invalid tenant-scoped key format: %s", key)
				skipped++
				continue
			}
		} else {
			// Legacy format: contract:CONTRACT_ID
			contractID = strings.TrimPrefix(key, "contract:")
			tenantID = "default"
			runtimeKey = fmt.Sprintf("contract:tenant-%s:%s", tenantID, contractID)
		}

		// Load contract JSON from Redis
		contractJSON, err := cacheClient.Get(ctx, key)
		if err != nil {
			log.WithError(err).Warnf("Failed to load contract %s from key %s", contractID, key)
			skipped++
			continue
		}

		// Parse contract
		// CRITICAL: Backend stores envelope.constraints as ARRAY, but gateway expects MAP
		// Convert: [{"feature":"x","min":0.9,"max":1.0}] → {"x":{"min":0.9,"max":1.0}}
		var contractMap map[string]interface{}
		if err := json.Unmarshal([]byte(contractJSON), &contractMap); err != nil {
			log.WithError(err).Warnf("Failed to parse contract JSON %s", contractID)
			skipped++
			continue
		}

		// Convert envelope.constraints from array to map
		if envelope, ok := contractMap["envelope"].(map[string]interface{}); ok {
			if constraintsArr, ok := envelope["constraints"].([]interface{}); ok {
				constraintsMap := make(map[string]interface{})
				for _, c := range constraintsArr {
					if constraint, ok := c.(map[string]interface{}); ok {
						if feature, ok := constraint["feature"].(string); ok {
							constraintsMap[feature] = map[string]interface{}{
								"min":    constraint["min"],
								"max":    constraint["max"],
								"action": constraint["action"],
							}
						}
					}
				}
				envelope["constraints"] = constraintsMap
				log.Debugf("  Converted %d constraints from array→map for %s", len(constraintsMap), contractID)
			}
		}
		
		// CRITICAL: Convert feature_extractors (array) to feature_extractor (singular) for HasGuardrails()
		// Gateway's HasGuardrails() checks for feature_extractor.ID (singular) for backward compatibility
		if featureExtractors, ok := contractMap["feature_extractors"].([]interface{}); ok && len(featureExtractors) > 0 {
			if firstExtractor, ok := featureExtractors[0].(map[string]interface{}); ok {
				contractMap["feature_extractor"] = map[string]interface{}{
					"id":      firstExtractor["id"],
					"sha256":  firstExtractor["sha256"],
					"version": firstExtractor["version"],
					"weight":  firstExtractor["weight"],
					"enabled": firstExtractor["enabled"],
				}
				log.Debugf("  Set feature_extractor (singular) from first extractor for %s", contractID)
			}
		}

		// Re-marshal and unmarshal into Contract struct
		fixedJSON, err := json.Marshal(contractMap)
		if err != nil {
			log.WithError(err).Warnf("Failed to re-marshal contract %s", contractID)
			skipped++
			continue
		}

		var contract contracts.Contract
		if err := json.Unmarshal(fixedJSON, &contract); err != nil {
			log.WithError(err).Warnf("Failed to parse contract %s", contractID)
			skipped++
			continue
		}

		// Only load APPROVED/ACTIVE contracts
		if contract.Status != "active" && contract.Status != "approved" {
			log.Infof("⏭️  Skipping non-active contract: %s (status: %s)", contractID, contract.Status)
			skipped++
			continue
		}

		approved++

		// Load into runtime table using the correct runtime key format
		log.Infof("📋 Loading approved contract: %s (tenant: %s, runtime_key: %s, has_feature_extraction=%v)",
			contractID, tenantID, runtimeKey, contract.HasFeatureExtraction())
		if err := LoadContract(ctx, runtimeKey, &contract); err != nil {
			log.WithError(err).Warnf("Failed to load contract %s for feature extraction", contractID)
			skipped++
			continue
		}

		// Check if it was actually loaded
		if contract.HasFeatureExtraction() {
			loaded++
			log.Infof("✅ Loaded contract %s (tenant: %s) with extractor %s", contractID, tenantID, contract.FeatureExtractor.ID)
		} else {
			loaded++
			log.Infof("✅ Loaded contract %s (tenant: %s) - envelope-only validation", contractID, tenantID)
		}
	}

	log.Infof("🎉 Contract loading complete: %d loaded (%d approved), %d skipped", loaded, approved, skipped)
	return nil
}

// LoadContract loads a single contract into the runtime table
func LoadContract(ctx context.Context, contractID string, contract *contracts.Contract) error {
	tableMutex.Lock()
	defer tableMutex.Unlock()

	// Check if already loaded
	if _, exists := ContractRuntimeTable[contractID]; exists {
		log.WithField("contract_id", contractID).Debug("Contract already loaded")
		return nil
	}

	// Check if contract has feature extraction enabled
	if !contract.HasFeatureExtraction() {
		log.WithField("contract_id", contractID).Debug("Contract does not have feature extraction enabled - loading without guardrail")

		// Still add to runtime table, but without extractor
		binding := &RuntimeBinding{
			Contract:      contract,
			ExtractorFunc: nil, // No feature extraction
			LoadedAt:      time.Now(),
		}

		ContractRuntimeTable[contractID] = binding

		log.WithFields(log.Fields{
			"contract_id": contractID,
			"constraints": len(contract.Envelope.Constraints),
		}).Info("Contract loaded into runtime table (no feature extraction)")

		return nil
	}

	// Load ALL guardrails needed based on envelope constraints
	// Each constraint defines a required metric, which maps to a guardrail
	// Map metrics to guardrail IDs by checking which guardrail produces each metric
	guardrailFuncs := make(map[string]txrepo.FeatureExtractorFunc)
	guardrailNames := make([]string, 0)

	// Get all registered guardrail configs to find metric→guardrail mapping
	allConfigs := txrepo.ListConfigs()
	metricToGuardrailID := make(map[string]string)
	for _, config := range allConfigs {
		if config.MetricName != "" {
			metricToGuardrailID[config.MetricName] = config.ID
		}
	}

	for metricName := range contract.Envelope.Constraints {
		// Find which guardrail produces this metric
		guardrailID, found := metricToGuardrailID[metricName]
		if !found {
			// Fallback: try exact match or _v1 suffix
			guardrailID = metricName
			extractorFunc, _, err := txrepo.GetGuardrail(guardrailID)
			if err != nil {
				guardrailID = metricName + "_v1"
				extractorFunc, _, err = txrepo.GetGuardrail(guardrailID)
				if err != nil {
					log.WithFields(log.Fields{
						"contract_id": contractID,
						"metric":      metricName,
					}).Warn("Guardrail not found for metric - will fail at validation")
					continue
				}
				guardrailFuncs[metricName] = extractorFunc
				guardrailNames = append(guardrailNames, guardrailID)
				continue
			}
			guardrailFuncs[metricName] = extractorFunc
			guardrailNames = append(guardrailNames, guardrailID)
			continue
		}

		// Load the guardrail
		extractorFunc, _, err := txrepo.GetGuardrail(guardrailID)
		if err != nil {
			log.WithFields(log.Fields{
				"contract_id": contractID,
				"metric":      metricName,
				"guardrail":   guardrailID,
			}).Warn("Guardrail not found - will fail at validation")
			continue
		}

		guardrailFuncs[metricName] = extractorFunc
		guardrailNames = append(guardrailNames, guardrailID)
	}

	if len(guardrailFuncs) == 0 {
		log.WithField("contract_id", contractID).Warn("No guardrails loaded for contract")
		return fmt.Errorf("no guardrails available for contract %s", contractID)
	}

	// Create composite extractor function that runs ALL guardrails
	compositeExtractor := func(payload []byte) (txrepo.MetricValues, error) {
		allMetrics := make(txrepo.MetricValues)

		for metricName, extractorFunc := range guardrailFuncs {
			metrics, err := extractorFunc(payload)
			if err != nil {
				log.WithFields(log.Fields{
					"metric": metricName,
					"error":  err,
				}).Warn("Guardrail execution failed")
				continue
			}

			// Merge metrics from this guardrail
			for k, v := range metrics {
				allMetrics[k] = v
			}
		}

		return allMetrics, nil
	}

	// Add to runtime table
	binding := &RuntimeBinding{
		Contract:      contract,
		ExtractorFunc: compositeExtractor,
		ExtractorMeta: txrepo.ExtractorMetadata{
			ID:          "composite",
			Version:     "1.0.0",
			Description: fmt.Sprintf("Composite guardrail (%d guardrails)", len(guardrailFuncs)),
		},
		LoadedAt: time.Now(),
	}

	ContractRuntimeTable[contractID] = binding

	log.WithFields(log.Fields{
		"contract_id": contractID,
		"guardrails":  guardrailNames,
		"constraints": len(contract.Envelope.Constraints),
	}).Info("Contract loaded into runtime table")

	return nil
}

// GetBinding returns a runtime binding for a contract (thread-safe)
func GetBinding(contractID string) (*RuntimeBinding, error) {
	tableMutex.RLock()
	binding, ok := ContractRuntimeTable[contractID]
	tableMutex.RUnlock()

	if !ok {
		return nil, ErrContractNotLoaded
	}

	return binding, nil
}

// GetOrLoadBinding gets a binding or attempts to load it if contract is provided
func GetOrLoadBinding(ctx context.Context, contractID string, contract *contracts.Contract) (*RuntimeBinding, error) {
	// Try to get existing binding
	binding, err := GetBinding(contractID)
	if err == nil {
		return binding, nil
	}

	// Not loaded yet - try to load it if contract is provided
	if contract != nil {
		if err := LoadContract(ctx, contractID, contract); err != nil {
			return nil, err
		}
		return GetBinding(contractID)
	}

	return nil, ErrContractNotLoaded
}

// UnloadContract removes a contract from the runtime table
func UnloadContract(contractID string) {
	tableMutex.Lock()
	defer tableMutex.Unlock()

	delete(ContractRuntimeTable, contractID)
	log.WithField("contract_id", contractID).Info("Contract unloaded from runtime table")
}

// GetLoadedContracts returns a list of all loaded contract IDs
func GetLoadedContracts() []string {
	tableMutex.RLock()
	defer tableMutex.RUnlock()

	contracts := make([]string, 0, len(ContractRuntimeTable))
	for id := range ContractRuntimeTable {
		contracts = append(contracts, id)
	}

	return contracts
}
