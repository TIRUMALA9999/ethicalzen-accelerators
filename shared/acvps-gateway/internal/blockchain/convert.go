package blockchain

import (
	"encoding/json"

	"github.com/ethicalzen/acvps-gateway/pkg/contracts"
)

// Metadata holds the extended contract fields stored in blockchain metadata
type Metadata struct {
	UseCase          string                   `json:"use_case,omitempty"`
	RiskScore        float64                  `json:"risk_score,omitempty"`
	Version          string                   `json:"version,omitempty"`
	FeatureExtractor *contracts.ExtractorSpec `json:"feature_extractor,omitempty"`
	Envelope         *contracts.Envelope      `json:"envelope,omitempty"`
}

// ToContractsContract converts blockchain.Contract to contracts.Contract
// This extracts feature_extractor and envelope from the Metadata JSON field
func (c *Contract) ToContractsContract() (*contracts.Contract, error) {
	if c == nil {
		return nil, nil
	}

	// Parse metadata if present
	var metadata Metadata
	if c.Metadata != "" {
		if err := json.Unmarshal([]byte(c.Metadata), &metadata); err != nil {
			// If metadata doesn't parse, continue without feature extraction
			metadata = Metadata{}
		}
	}

	// Map status string to ContractStatus
	status := contracts.StatusActive
	switch c.Status {
	case "Draft":
		status = contracts.StatusDraft
	case "Active":
		status = contracts.StatusActive
	case "Revoked":
		status = contracts.StatusRevoked
	case "Expired":
		status = contracts.StatusExpired
	}

	result := &contracts.Contract{
		ContractID:   c.ID,
		PolicyDigest: c.PolicyDigest,
		Suite:        c.Suite,
		Profile:      "balanced", // Default if not specified
		IssuedAt:     c.IssuedAt,
		ExpiresAt:    c.ExpiresAt,
		Status:       status,
	}

	// Add feature extraction fields if present in metadata
	if metadata.FeatureExtractor != nil {
		result.FeatureExtractor = *metadata.FeatureExtractor
	}

	if metadata.Envelope != nil {
		result.Envelope = *metadata.Envelope
	}

	return result, nil
}
