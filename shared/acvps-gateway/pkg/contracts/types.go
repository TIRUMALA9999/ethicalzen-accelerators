package contracts

import (
	"encoding/json"
	"time"
)

// Contract represents a deterministic contract with guardrails and metric thresholds
// Guardrails are safety mechanisms that validate AI responses
// Metrics are quantitative measurements (e.g., pii_risk, grounding_confidence)
// Thresholds define acceptable ranges for each metric
type Contract struct {
	ContractID        string            `json:"contract_id,omitempty"` // Preferred field name
	ID                string            `json:"id,omitempty"`          // Alternative field name (from backend)
	PolicyDigest      string            `json:"policy_digest"`
	Suite             string            `json:"suite"`                        // S0, S1, S2
	Profile           string            `json:"profile"`                      // observe, balanced, strict
	ServiceName       string            `json:"service_name,omitempty"`       // From backend
	Domain            string            `json:"domain,omitempty"`             // From backend
	Region            string            `json:"region,omitempty"`             // From backend
	UseCase           string            `json:"use_case,omitempty"`           // From backend
	Description       string            `json:"description,omitempty"`        // From backend
	TestMode          string            `json:"test_mode,omitempty"`          // From backend
	Approved          bool              `json:"approved,omitempty"`           // From backend
	TenantID          string            `json:"tenant_id,omitempty"`          // From backend
	Version           string            `json:"version,omitempty"`            // From backend
	BackendURL        string            `json:"backend_url,omitempty"`        // Target backend endpoint for proxy mode
	RiskAnalysis      json.RawMessage   `json:"risk_analysis,omitempty"`      // From backend (store as raw)
	FeatureExtractors []json.RawMessage `json:"feature_extractors,omitempty"` // From backend (array)

	// New terminology (preferred)
	Guardrails []GuardrailSpec `json:"guardrails,omitempty"` // Safety guardrails to enforce
	Thresholds Thresholds      `json:"thresholds,omitempty"` // Metric thresholds

	// Legacy fields (for backward compatibility)
	FeatureExtractor ExtractorSpec `json:"feature_extractor,omitempty"` // Deprecated: Use Guardrails
	Envelope         Envelope      `json:"envelope,omitempty"`          // Deprecated: Use Thresholds

	IssuedAt  time.Time      `json:"issued_at"`
	ExpiresAt time.Time      `json:"expires_at"`
	Status    ContractStatus `json:"status"`
}

// GuardrailSpec specifies a safety guardrail to enforce
type GuardrailSpec struct {
	ID          string   `json:"id"`          // e.g., "pii_detection_v1"
	Name        string   `json:"name"`        // e.g., "PII Detection"
	Metrics     []string `json:"metrics"`     // e.g., ["pii_risk"]
	Version     string   `json:"version"`     // e.g., "1.0.0"
	SHA256      string   `json:"sha256"`      // Hash of guardrail source code
	Description string   `json:"description"` // Human-readable description
}

// ExtractorSpec (legacy) - kept for backward compatibility
type ExtractorSpec struct {
	ID          string `json:"id"`          // e.g., "pii_detector_v1"
	Version     string `json:"version"`     // e.g., "1.0.0"
	SHA256      string `json:"sha256"`      // Hash of extractor source code
	Description string `json:"description"` // Human-readable description
}

// Thresholds defines acceptable ranges for metrics
type Thresholds struct {
	Limits map[string]Bounds `json:"limits,omitempty"` // metric_name -> acceptable range

	// Legacy field (for backward compatibility)
	Constraints map[string]Bounds `json:"constraints,omitempty"` // Deprecated: Use Limits
}

// Envelope (legacy) - kept for backward compatibility
type Envelope struct {
	Constraints map[string]Bounds `json:"constraints"` // feature_name -> bounds
}

// Bounds defines min/max acceptable range for a metric
type Bounds struct {
	Min float64 `json:"min"`
	Max float64 `json:"max"`
}

// ContractStatus represents the current status of a contract
type ContractStatus string

const (
	StatusDraft   ContractStatus = "draft"
	StatusActive  ContractStatus = "active"
	StatusRevoked ContractStatus = "revoked"
	StatusExpired ContractStatus = "expired"
)

// HasGuardrails returns true if the contract has guardrails configured
func (c *Contract) HasGuardrails() bool {
	// Check new format first
	if len(c.Guardrails) > 0 {
		return true
	}
	// Check backend format (feature_extractors array from backend)
	if len(c.FeatureExtractors) > 0 && len(c.Envelope.Constraints) > 0 {
		return true
	}
	// Fallback to legacy singular format
	return c.FeatureExtractor.ID != "" && len(c.Envelope.Constraints) > 0
}

// HasFeatureExtraction (legacy) - kept for backward compatibility
func (c *Contract) HasFeatureExtraction() bool {
	return c.HasGuardrails()
}

// GetThresholds returns thresholds, handling both new and legacy formats
func (c *Contract) GetThresholds() map[string]Bounds {
	// Try new format first
	if len(c.Thresholds.Limits) > 0 {
		return c.Thresholds.Limits
	}
	if len(c.Thresholds.Constraints) > 0 {
		return c.Thresholds.Constraints
	}
	// Fallback to legacy Envelope
	if len(c.Envelope.Constraints) > 0 {
		return c.Envelope.Constraints
	}
	return nil
}

// GetGuardrailIDs returns IDs of all configured guardrails
func (c *Contract) GetGuardrailIDs() []string {
	ids := []string{}

	// Check new format
	for _, g := range c.Guardrails {
		if g.ID != "" {
			ids = append(ids, g.ID)
		}
	}

	// Fallback to legacy format
	if len(ids) == 0 && c.FeatureExtractor.ID != "" {
		ids = append(ids, c.FeatureExtractor.ID)
	}

	return ids
}

// IsValid returns true if the contract is in a valid state for use
func (c *Contract) IsValid() bool {
	if c.Status != StatusActive {
		return false
	}

	if time.Now().After(c.ExpiresAt) {
		return false
	}

	return true
}
