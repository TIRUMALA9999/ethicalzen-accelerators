package gateway

import (
	"fmt"
	"time"

	"github.com/ethicalzen/acvps-gateway/pkg/txrepo"
	log "github.com/sirupsen/logrus"
)

// StreamContext is an alias to txrepo.StreamContext for convenience
type StreamContext = txrepo.StreamContext

// StreamValidationResult holds the result of stream validation
type StreamValidationResult struct {
	Valid           bool
	Metrics         txrepo.MetricValues
	Confidence      float64 // Overall confidence (0.0-1.0)
	Violations      []StreamViolation
	CalculationTime time.Duration
	ValidationTime  time.Duration
}

// StreamViolation represents a detected issue in the stream
type StreamViolation struct {
	Metric     string
	Value      float64
	Threshold  float64
	Confidence float64
}

// ValidateStream validates a byte stream using probabilistic guardrails
// This is different from ValidateResponse - it doesn't assume JSON structure
// Works with ANY format: JSON, Protobuf, XML, raw text, binary, etc.
func ValidateStream(contractID string, stream []byte, traceID string, ctx StreamContext) (*StreamValidationResult, error) {
	startTime := time.Now()
	result := &StreamValidationResult{}

	log.WithFields(log.Fields{
		"trace_id":   traceID,
		"contract":   contractID,
		"direction":  ctx.Direction,
		"stream_len": len(stream),
	}).Info("🌊 Starting stream validation")

	// Get contract binding
	binding, err := GetBinding(contractID)
	if err != nil {
		return nil, fmt.Errorf("contract not loaded: %w", err)
	}

	// Get stream-based guardrails (probabilistic/LLM-based)
	streamGuardrails := getStreamGuardrails(binding)

	if len(streamGuardrails) == 0 {
		log.Info("No stream-specific guardrails configured, using standard guardrails as fallback")
		
		// Fallback to standard validation
		// Standard guardrails will handle JSON parsing internally
		stdResult, err := ValidateResponse(contractID, stream, traceID)
		
		// Convert to stream result
		result.Valid = stdResult.Valid
		result.Metrics = stdResult.Metrics
		result.Confidence = 0.9 // High confidence for pattern-based
		result.CalculationTime = stdResult.CalculationTime
		result.ValidationTime = stdResult.ValidationTime
		
		if err != nil {
			// Check if it's an envelope violation
			if violation, ok := err.(*EnvelopeViolation); ok {
				result.Violations = []StreamViolation{{
					Metric:     violation.Metric,
					Value:      violation.Value,
					Threshold:  violation.Bounds.Max,
					Confidence: result.Confidence,
				}}
				return result, err
			}
			return nil, err
		}
		
		return result, nil
	}

	// Execute stream guardrails (probabilistic/LLM-based)
	calculationStart := time.Now()

	allMetrics := make(txrepo.MetricValues)
	confidences := make([]float64, 0)

	for _, guardrail := range streamGuardrails {
		log.WithField("guardrail", guardrail.ID).Debug("Executing stream guardrail")

		metrics, confidence, err := guardrail.Func(stream, ctx)
		if err != nil {
			log.WithError(err).Warn("Stream guardrail failed, continuing with others")
			continue
		}

		// Merge metrics
		for k, v := range metrics {
			allMetrics[k] = v
		}

		confidences = append(confidences, confidence)

		log.WithFields(log.Fields{
			"guardrail":  guardrail.ID,
			"metrics":    metrics,
			"confidence": confidence,
		}).Debug("Stream guardrail executed")
	}

	result.Metrics = allMetrics
	result.CalculationTime = time.Since(calculationStart)

	// Calculate average confidence
	if len(confidences) > 0 {
		sum := 0.0
		for _, c := range confidences {
			sum += c
		}
		result.Confidence = sum / float64(len(confidences))
	} else {
		result.Confidence = 0.5 // Medium confidence if no guardrails executed
	}

	// Validate against thresholds
	validationStart := time.Now()
	thresholds := binding.Contract.GetThresholds()

	violations := []StreamViolation{}
	for metricName, bounds := range thresholds {
		if value, ok := allMetrics[metricName]; ok {
			if value < bounds.Min || value > bounds.Max {
				violations = append(violations, StreamViolation{
					Metric:     metricName,
					Value:      value,
					Threshold:  bounds.Max,
					Confidence: result.Confidence,
				})
			}
		}
	}

	result.Violations = violations
	result.ValidationTime = time.Since(validationStart)
	result.Valid = len(violations) == 0

	totalTime := time.Since(startTime)

	log.WithFields(log.Fields{
		"trace_id":       traceID,
		"valid":          result.Valid,
		"violations":     len(violations),
		"confidence":     result.Confidence,
		"calculation_ms": result.CalculationTime.Milliseconds(),
		"validation_ms":  result.ValidationTime.Milliseconds(),
		"total_ms":       totalTime.Milliseconds(),
	}).Info("🌊 Stream validation complete")

	if !result.Valid {
		return result, fmt.Errorf("stream validation failed: %d violations", len(violations))
	}

	return result, nil
}

// StreamGuardrail represents a guardrail that operates on raw byte streams
type StreamGuardrail struct {
	ID   string
	Name string
	Func func([]byte, txrepo.StreamContext) (txrepo.MetricValues, float64, error)
}

// getStreamGuardrails returns guardrails configured for stream validation
// These are probabilistic/LLM-based guardrails that work on raw bytes
func getStreamGuardrails(binding *RuntimeBinding) []StreamGuardrail {
	// Check if contract specifies stream-based guardrails
	// For now, return empty - will be populated when stream guardrails are registered
	
	// TODO: Load from contract's guardrail list where type="stream_llm" or "stream_probabilistic"
	// Example:
	// for _, g := range binding.Contract.Guardrails {
	//     if g.Type == "stream_llm" || g.Type == "stream_probabilistic" {
	//         streamGuardrails = append(streamGuardrails, loadStreamGuardrail(g.ID))
	//     }
	// }
	
	log.Debug("Stream guardrails not yet configured, using standard guardrails")
	return []StreamGuardrail{}
}

