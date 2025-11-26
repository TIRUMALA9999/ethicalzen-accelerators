package gateway

import (
	"testing"

	"github.com/ethicalzen/acvps-gateway/pkg/contracts"
	"github.com/ethicalzen/acvps-gateway/pkg/txrepo"
)

func TestValidateThresholds(t *testing.T) {
	tests := []struct {
		name        string
		metrics     txrepo.MetricValues
		thresholds  map[string]contracts.Bounds
		expectError bool
	}{
		{
			name: "all metrics within bounds",
			metrics: txrepo.MetricValues{
				"pii_risk":      0.05,
				"toxicity":      0.02,
				"hallucination": 0.01,
			},
			thresholds: map[string]contracts.Bounds{
				"pii_risk":      {Min: 0.0, Max: 0.1},
				"toxicity":      {Min: 0.0, Max: 0.1},
				"hallucination": {Min: 0.0, Max: 0.1},
			},
			expectError: false,
		},
		{
			name: "pii_risk exceeds threshold",
			metrics: txrepo.MetricValues{
				"pii_risk": 0.95,
				"toxicity": 0.02,
			},
			thresholds: map[string]contracts.Bounds{
				"pii_risk": {Min: 0.0, Max: 0.1},
				"toxicity": {Min: 0.0, Max: 0.1},
			},
			expectError: true,
		},
		{
			name: "multiple violations",
			metrics: txrepo.MetricValues{
				"pii_risk":      0.95,
				"toxicity":      0.85,
				"hallucination": 0.92,
			},
			thresholds: map[string]contracts.Bounds{
				"pii_risk":      {Min: 0.0, Max: 0.1},
				"toxicity":      {Min: 0.0, Max: 0.1},
				"hallucination": {Min: 0.0, Max: 0.1},
			},
			expectError: true,
		},
		{
			name: "metric below minimum threshold",
			metrics: txrepo.MetricValues{
				"grounding_confidence": 0.3,
			},
			thresholds: map[string]contracts.Bounds{
				"grounding_confidence": {Min: 0.7, Max: 1.0},
			},
			expectError: true,
		},
		{
			name:    "empty metrics",
			metrics: txrepo.MetricValues{},
			thresholds: map[string]contracts.Bounds{
				"pii_risk": {Min: 0.0, Max: 0.1},
			},
			expectError: false, // Missing metrics are not violations
		},
		{
			name: "exact boundary values",
			metrics: txrepo.MetricValues{
				"pii_risk": 0.1,
				"toxicity": 0.0,
			},
			thresholds: map[string]contracts.Bounds{
				"pii_risk": {Min: 0.0, Max: 0.1},
				"toxicity": {Min: 0.0, Max: 0.1},
			},
			expectError: false, // Boundary values should pass
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateThresholds(tt.metrics, tt.thresholds)
			if (err != nil) != tt.expectError {
				t.Errorf("ValidateThresholds() error = %v, expectError %v", err, tt.expectError)
			}
		})
	}
}

func TestThresholdViolation_Error(t *testing.T) {
	tests := []struct {
		name      string
		violation ThresholdViolation
		expected  string
	}{
		{
			name: "using Metric field",
			violation: ThresholdViolation{
				Metric: "pii_risk",
				Value:  0.95,
				Bounds: contracts.Bounds{Min: 0.0, Max: 0.1},
			},
			expected: "metric 'pii_risk' value 0.9500 violates threshold [0.0000, 0.1000]",
		},
		{
			name: "using Feature field (legacy)",
			violation: ThresholdViolation{
				Feature: "toxicity",
				Value:   0.85,
				Bounds:  contracts.Bounds{Min: 0.0, Max: 0.1},
			},
			expected: "metric 'toxicity' value 0.8500 violates threshold [0.0000, 0.1000]",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.violation.Error()
			if got != tt.expected {
				t.Errorf("ThresholdViolation.Error() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestValidationResult(t *testing.T) {
	t.Run("validation result structure", func(t *testing.T) {
		result := ValidationResult{
			Valid: false,
			Violations: []ThresholdViolation{
				{
					Metric: "pii_risk",
					Value:  0.95,
					Bounds: contracts.Bounds{Min: 0.0, Max: 0.1},
				},
			},
			Metrics: txrepo.MetricValues{
				"pii_risk": 0.95,
				"toxicity": 0.02,
			},
		}

		if result.Valid {
			t.Error("Expected Valid to be false")
		}

		if len(result.Violations) != 1 {
			t.Errorf("Expected 1 violation, got %d", len(result.Violations))
		}

		if len(result.Metrics) != 2 {
			t.Errorf("Expected 2 metrics, got %d", len(result.Metrics))
		}
	})
}

func BenchmarkValidateThresholds(b *testing.B) {
	metrics := txrepo.MetricValues{
		"pii_risk":      0.05,
		"toxicity":      0.02,
		"hallucination": 0.01,
		"bias":          0.03,
		"off_topic":     0.01,
	}

	thresholds := map[string]contracts.Bounds{
		"pii_risk":      {Min: 0.0, Max: 0.1},
		"toxicity":      {Min: 0.0, Max: 0.1},
		"hallucination": {Min: 0.0, Max: 0.1},
		"bias":          {Min: 0.0, Max: 0.1},
		"off_topic":     {Min: 0.0, Max: 0.1},
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = ValidateThresholds(metrics, thresholds)
	}
}
