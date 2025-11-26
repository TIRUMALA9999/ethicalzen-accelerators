package gateway

import (
	"fmt"
	"time"

	"github.com/ethicalzen/acvps-gateway/pkg/contracts"
	"github.com/ethicalzen/acvps-gateway/pkg/txrepo"
	log "github.com/sirupsen/logrus"
)

// ThresholdViolation represents a metric threshold violation
type ThresholdViolation struct {
	Metric  string           // Metric name (e.g., "pii_risk", "grounding_confidence") [Preferred]
	Feature string           // Legacy field (alias for Metric, for backward compatibility) [Deprecated]
	Value   float64          // Actual value
	Bounds  contracts.Bounds // Acceptable range
}

func (t *ThresholdViolation) Error() string {
	// Use Metric if set, otherwise fall back to Feature for legacy code
	name := t.Metric
	if name == "" {
		name = t.Feature
	}
	return fmt.Sprintf("metric '%s' value %.4f violates threshold [%.4f, %.4f]",
		name, t.Value, t.Bounds.Min, t.Bounds.Max)
}

// Legacy alias for backward compatibility
type EnvelopeViolation = ThresholdViolation

// ValidationResult holds the result of guardrail validation
type ValidationResult struct {
	Valid           bool
	Violations      []ThresholdViolation // Metrics that violated thresholds
	Metrics         txrepo.MetricValues  // All calculated metrics
	CalculationTime time.Duration        // Time to calculate metrics
	ValidationTime  time.Duration        // Time to validate thresholds

	// Legacy aliases (for backward compatibility)
	Features           txrepo.FeatureVector // Deprecated: Use Metrics
	ExtractionDuration time.Duration        // Deprecated: Use CalculationTime
	ValidationDuration time.Duration        // Deprecated: Use ValidationTime
}

// ValidateThresholds validates metric values against thresholds
func ValidateThresholds(metrics txrepo.MetricValues, thresholds map[string]contracts.Bounds) error {
	for metricName, bounds := range thresholds {
		value, ok := metrics[metricName]
		if !ok {
			return fmt.Errorf("missing required metric: %s", metricName)
		}

		if value < bounds.Min || value > bounds.Max {
			return &ThresholdViolation{
				Metric:  metricName,
				Feature: metricName, // Populate both for backward compatibility
				Value:   value,
				Bounds:  bounds,
			}
		}
	}

	return nil
}

// ValidateEnvelope (legacy) - kept for backward compatibility
func ValidateEnvelope(features txrepo.FeatureVector, envelope contracts.Envelope) error {
	return ValidateThresholds(txrepo.MetricValues(features), envelope.Constraints)
}

// ValidateResponse validates an AI response against its contract guardrails
// This is the main enforcement function called at runtime:
// 1. Loads contract from certificate
// 2. Calculates metrics using configured guardrails
// 3. Validates metrics against thresholds
// 4. Returns PASS or BLOCK with detailed results
func ValidateResponse(contractID string, responseBody []byte, traceID string) (*ValidationResult, error) {
	startTime := time.Now()
	result := &ValidationResult{}

	// 1. Get runtime binding (contract loaded from certificate)
	binding, err := GetBinding(contractID)
	if err != nil {
		// Contract not loaded or doesn't have guardrails configured
		return nil, err
	}

	// 2. Calculate metrics using guardrail functions (if available)
	calculationStart := time.Now()
	var metrics map[string]float64
	
	if binding.ExtractorFunc != nil {
		// Feature extraction enabled - calculate metrics
		var err error
		metrics, err = binding.ExtractorFunc(responseBody)
		if err != nil {
			return nil, fmt.Errorf("calculate metrics: %w", err)
		}
		result.Metrics = txrepo.MetricValues(metrics)
		result.Features = metrics // Legacy field
	} else {
		// No feature extraction - use empty metrics
		metrics = make(map[string]float64)
		result.Metrics = txrepo.MetricValues(metrics)
		result.Features = metrics
	}
	
	result.CalculationTime = time.Since(calculationStart)
	result.ExtractionDuration = result.CalculationTime // Legacy field

	// 3. Get thresholds from contract (handles both old and new formats)
	thresholds := binding.Contract.GetThresholds()
	if thresholds == nil {
		// No thresholds configured - contract passes by default
		result.Valid = true
		result.ValidationTime = 0
		return result, nil
	}

	// 4. Validate metrics against thresholds
	validationStart := time.Now()
	if err := ValidateThresholds(result.Metrics, thresholds); err != nil {
		result.Valid = false

		// Collect all violations (not just the first one)
		violations := []ThresholdViolation{}
		for metricName, bounds := range thresholds {
			if value, ok := result.Metrics[metricName]; ok {
				if value < bounds.Min || value > bounds.Max {
					violations = append(violations, ThresholdViolation{
						Metric:  metricName,
						Feature: metricName, // Populate both for backward compatibility
						Value:   value,
						Bounds:  bounds,
					})
				}
			}
		}
		result.Violations = violations
		result.ValidationTime = time.Since(validationStart)
		result.ValidationDuration = result.ValidationTime // Legacy field

		log.WithFields(log.Fields{
			"trace_id":    traceID,
			"contract_id": contractID,
			"violations":  len(violations),
			"guardrails":  len(binding.Contract.GetGuardrailIDs()),
		}).Warn("Guardrail validation failed - BLOCKED")

		return result, err
	}

	result.Valid = true
	result.ValidationTime = time.Since(validationStart)
	result.ValidationDuration = result.ValidationTime // Legacy field
	totalDuration := time.Since(startTime)

	// Log success
	log.WithFields(log.Fields{
		"trace_id":       traceID,
		"contract_id":    contractID,
		"calculation_ms": result.CalculationTime.Milliseconds(),
		"validation_ms":  result.ValidationTime.Milliseconds(),
		"total_ms":       totalDuration.Milliseconds(),
		"guardrails":     len(binding.Contract.GetGuardrailIDs()),
	}).Debug("Guardrail validation passed - ALLOWED")

	// Check performance budget (log warning if exceeded)
	if totalDuration > 15*time.Millisecond {
		log.WithFields(log.Fields{
			"trace_id": traceID,
			"duration": totalDuration,
			"budget":   "15ms",
		}).Warn("Validation exceeded performance budget")
	}

	return result, nil
}
