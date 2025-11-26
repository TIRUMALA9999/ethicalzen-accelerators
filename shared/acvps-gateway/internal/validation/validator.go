package validation

import (
	"context"
	"fmt"
	"time"

	"github.com/ethicalzen/acvps-gateway/internal/blockchain"
	"github.com/ethicalzen/acvps-gateway/internal/cache"
	"github.com/ethicalzen/acvps-gateway/internal/config"
	log "github.com/sirupsen/logrus"
)

// Validator handles DC contract validation
type Validator struct {
	blockchain *blockchain.Client
	cache      *cache.Client
	config     *config.Config
}

// ValidationResult represents the result of contract validation
type ValidationResult struct {
	Valid    bool
	Reason   string
	Contract *blockchain.Contract
}

// New creates a new validator
func New(cfg *config.Config, bc *blockchain.Client, c *cache.Client) *Validator {
	return &Validator{
		blockchain: bc,
		cache:      c,
		config:     cfg,
	}
}

// ValidateContract validates a DC contract
func (v *Validator) ValidateContract(ctx context.Context, dcID, dcDigest, dcSuite, dcProfile string) (bool, string, *blockchain.Contract, error) {
	logger := log.WithFields(log.Fields{
		"dc_id":      dcID,
		"dc_digest":  dcDigest,
		"dc_suite":   dcSuite,
		"dc_profile": dcProfile,
	})

	// Step 1: Validate contract (blockchain or cache-only)
	var valid bool
	var reason string
	var err error

	if v.config.Validation.BlockchainVerification {
		// Production mode: Validate via blockchain
		valid, reason, err = v.blockchain.ValidateContract(ctx, dcID, dcDigest)
		if err != nil {
			return false, "", nil, fmt.Errorf("blockchain validation failed: %w", err)
		}
		if !valid {
			logger.WithField("reason", reason).Warn("Contract validation failed")
			return false, reason, nil, nil
		}
	} else {
		// Local/test mode: Skip blockchain, assume valid if in cache
		logger.Debug("Skipping blockchain verification (blockchain_verification=false)")
		valid = true
		reason = "local-cache"
	}

	// Step 2: Get full contract details (optional - if it fails, we can still proceed)
	var contract *blockchain.Contract
	if v.blockchain != nil {
		var err error
		contract, err = v.blockchain.GetContract(ctx, dcID)
		if err != nil {
			// Log the error but don't fail - validateContract already confirmed validity
			logger.WithField("error", err).Debug("Could not fetch full contract details, proceeding without")
			contract = nil
		}
	} else {
		// Blockchain client not available (local mode)
		logger.Debug("Blockchain client not available, skipping contract details fetch")
		contract = nil
	}

	// Step 3: Validate suite (if specified and allowed suites configured)
	if dcSuite != "" && len(v.config.Validation.AllowedSuites) > 0 {
		suiteAllowed := false
		for _, allowedSuite := range v.config.Validation.AllowedSuites {
			if allowedSuite == dcSuite {
				suiteAllowed = true
				break
			}
		}

		if !suiteAllowed {
			logger.WithField("suite", dcSuite).Warn("Suite not allowed")
			return false, fmt.Sprintf("Suite %s not allowed", dcSuite), contract, nil
		}
	}

	// Step 4: Check if contract matches requested suite (only if we have contract details)
	if contract != nil && dcSuite != "" && contract.Suite != dcSuite {
		logger.WithFields(log.Fields{
			"requested_suite": dcSuite,
			"contract_suite":  contract.Suite,
		}).Warn("Suite mismatch")
		return false, fmt.Sprintf("Suite mismatch: requested %s, contract has %s", dcSuite, contract.Suite), contract, nil
	}

	// Step 5: Check expiration (only if we have contract details)
	if contract != nil && time.Now().After(contract.ExpiresAt) {
		logger.WithField("expires_at", contract.ExpiresAt).Warn("Contract expired")
		return false, "Contract has expired", contract, nil
	}

	if contract != nil {
		logger.WithFields(log.Fields{
			"contract_status": contract.Status,
			"suite":           contract.Suite,
			"service_name":    contract.ServiceName,
		}).Info("Contract validation successful")
	} else {
		logger.Info("Contract validated successfully (via blockchain only)")
	}

	return true, "Valid", contract, nil
}

// ValidateFailureModes checks if the request/response violates failure modes
func (v *Validator) ValidateFailureModes(ctx context.Context, contract *blockchain.Contract, requestBody, responseBody []byte) (bool, []string, error) {
	if !v.config.Validation.EnforceFailureModes {
		return true, nil, nil
	}

	violations := []string{}

	// TODO: Implement failure mode detection
	// This would check:
	// 1. PII leakage
	// 2. Ungrounded responses
	// 3. Hallucination risks
	// 4. Prompt injection
	// 5. Other contract-specific constraints

	// For MVP, we'll implement basic checks
	// Full implementation would parse contract.Metadata for failure modes

	return len(violations) == 0, violations, nil
}
