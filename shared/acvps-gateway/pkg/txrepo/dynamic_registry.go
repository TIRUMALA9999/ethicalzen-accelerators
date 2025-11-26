package txrepo

import (
	"fmt"
	"sync"
)

// DynamicRegistry manages runtime-registered guardrails
// Extends the static Registry with dynamic guardrail configurations
type DynamicRegistry struct {
	configs    map[string]*GuardrailConfig
	customImpl map[string]GuardrailFunc // Optional: custom Go implementations
	mu         sync.RWMutex
}

var dynamicRegistry *DynamicRegistry

func init() {
	dynamicRegistry = &DynamicRegistry{
		configs:    make(map[string]*GuardrailConfig),
		customImpl: make(map[string]GuardrailFunc),
	}
}

// RegisterConfig registers a guardrail configuration for use with GenericLLMGuardrail
func RegisterConfig(config *GuardrailConfig) error {
	dynamicRegistry.mu.Lock()
	defer dynamicRegistry.mu.Unlock()

	if config.ID == "" {
		return fmt.Errorf("guardrail ID cannot be empty")
	}

	// Set defaults
	if config.MetricName == "" {
		config.MetricName = "compliance_score"
	}
	if config.Threshold == 0 {
		config.Threshold = 75
	}

	dynamicRegistry.configs[config.ID] = config
	fmt.Printf("[DynamicRegistry] Registered guardrail config: %s\n", config.ID)
	return nil
}

// UnregisterConfig removes a guardrail configuration from the dynamic registry
func UnregisterConfig(id string) error {
	dynamicRegistry.mu.Lock()
	defer dynamicRegistry.mu.Unlock()

	if _, exists := dynamicRegistry.configs[id]; !exists {
		return fmt.Errorf("guardrail not found: %s", id)
	}

	delete(dynamicRegistry.configs, id)
	delete(dynamicRegistry.customImpl, id) // Also remove any custom implementation
	fmt.Printf("[DynamicRegistry] Unregistered guardrail: %s\n", id)
	return nil
}

// RegisterCustom registers a custom Go implementation (override)
// This allows customers to provide optimized implementations later
func RegisterCustom(id string, fn GuardrailFunc) error {
	dynamicRegistry.mu.Lock()
	defer dynamicRegistry.mu.Unlock()

	dynamicRegistry.customImpl[id] = fn
	fmt.Printf("[DynamicRegistry] Registered custom implementation: %s\n", id)
	return nil
}

// GetGuardrail returns guardrail function with priority:
// 1. Custom Go implementation (best performance)
// 2. Generic LLM template (flexibility)
// 3. Static built-in guardrails
// 4. nil (not found)
func GetGuardrail(id string) (GuardrailFunc, *ExtractorMetadata, error) {
	dynamicRegistry.mu.RLock()
	defer dynamicRegistry.mu.RUnlock()

	// Priority 1: Custom Go implementation (best performance)
	if customFn, exists := dynamicRegistry.customImpl[id]; exists {
		config := dynamicRegistry.configs[id]
		meta := &ExtractorMetadata{
			ID:          id,
			Version:     "1.0.0",
			Description: config.Description,
		}
		return customFn, meta, nil
	}

	// Priority 2: Generic LLM template (flexibility)
	if config, exists := dynamicRegistry.configs[id]; exists {
		// Return a closure that calls GenericLLMGuardrail with config
		fn := func(payload []byte) (MetricValues, error) {
			return GenericLLMGuardrail(payload, config)
		}
		meta := &ExtractorMetadata{
			ID:          config.ID,
			Version:     "1.0.0",
			Description: config.Description,
		}
		return fn, meta, nil
	}

	// Priority 3: Static built-in guardrails
	staticFn, staticMeta, err := GlobalRegistry.Get(id)
	if err == nil {
		return staticFn, &staticMeta, nil
	}

	// Not found
	return nil, nil, fmt.Errorf("guardrail not found: %s", id)
}

// GetConfig returns the configuration for a dynamic guardrail
func GetConfig(id string) (*GuardrailConfig, error) {
	dynamicRegistry.mu.RLock()
	defer dynamicRegistry.mu.RUnlock()

	config, exists := dynamicRegistry.configs[id]
	if !exists {
		return nil, fmt.Errorf("config not found: %s", id)
	}

	return config, nil
}

// ListAll returns all guardrails (static + dynamic)
func ListAll() []string {
	dynamicRegistry.mu.RLock()
	defer dynamicRegistry.mu.RUnlock()

	// Combine static and dynamic
	staticIDs := GlobalRegistry.List()

	allIDs := make(map[string]bool)
	for _, id := range staticIDs {
		allIDs[id] = true
	}
	for id := range dynamicRegistry.configs {
		allIDs[id] = true
	}

	result := make([]string, 0, len(allIDs))
	for id := range allIDs {
		result = append(result, id)
	}

	return result
}

// ListConfigs returns all dynamic guardrail configurations
func ListConfigs() []*GuardrailConfig {
	dynamicRegistry.mu.RLock()
	defer dynamicRegistry.mu.RUnlock()

	configs := make([]*GuardrailConfig, 0, len(dynamicRegistry.configs))
	for _, config := range dynamicRegistry.configs {
		configs = append(configs, config)
	}

	return configs
}

// HasCustomImplementation checks if a custom implementation exists
func HasCustomImplementation(id string) bool {
	dynamicRegistry.mu.RLock()
	defer dynamicRegistry.mu.RUnlock()

	_, exists := dynamicRegistry.customImpl[id]
	return exists
}

// ExportConfigs exports all configurations (for backup/migration)
func ExportConfigs() map[string]interface{} {
	dynamicRegistry.mu.RLock()
	defer dynamicRegistry.mu.RUnlock()

	return map[string]interface{}{
		"count":   len(dynamicRegistry.configs),
		"configs": dynamicRegistry.configs,
	}
}

