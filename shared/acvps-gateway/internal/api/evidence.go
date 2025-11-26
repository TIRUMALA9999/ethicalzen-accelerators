package api

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"time"

	"github.com/ethicalzen/acvps-gateway/pkg/gateway"
	log "github.com/sirupsen/logrus"
)

// EvidenceRecord represents evidence to be logged
type EvidenceRecord struct {
	TraceID      string             `json:"trace_id"`
	DCID         string             `json:"dc_id"`
	PolicyDigest string             `json:"policy_digest"`
	Suite        string             `json:"suite"`
	Profile      string             `json:"profile"`
	ModelID      string             `json:"model_id,omitempty"`
	SafetyScores map[string]float64 `json:"safety_scores"`
	LatencyMs    int64              `json:"latency_ms"`
	Status       string             `json:"status"` // "allowed" or "blocked"
	TenantID     string             `json:"tenant_id"`
	UseCase      string             `json:"use_case,omitempty"`
	Violations   []string           `json:"violations,omitempty"`
}

// EmitEvidence sends evidence to the backend evidence API
func (h *Handler) EmitEvidence(evidence EvidenceRecord) {
	// Don't block the response
	go func() {
		backendURL := getBackendURL() + "/api/dc/evidence"

		jsonData, err := json.Marshal(evidence)
		if err != nil {
			log.WithError(err).Error("Failed to marshal evidence")
			return
		}

		req, err := http.NewRequest("POST", backendURL, bytes.NewBuffer(jsonData))
		if err != nil {
			log.WithError(err).Error("Failed to create evidence request")
			return
		}

		req.Header.Set("Content-Type", "application/json")

		client := &http.Client{Timeout: 5 * time.Second}
		resp, err := client.Do(req)
		if err != nil {
			log.WithError(err).Warn("Failed to send evidence to backend")
			return
		}
		defer resp.Body.Close()

		if resp.StatusCode >= 400 {
			log.WithFields(log.Fields{
				"status_code": resp.StatusCode,
				"trace_id":    evidence.TraceID,
			}).Warn("Backend rejected evidence")
			return
		}

		log.WithFields(log.Fields{
			"trace_id": evidence.TraceID,
			"status":   evidence.Status,
		}).Debug("Evidence logged successfully")
	}()
}

// CreateEvidenceFromValidation creates an evidence record from a validation result
func CreateEvidenceFromValidation(
	traceID string,
	contractID string,
	tenantID string,
	result *gateway.ValidationResult,
	valid bool,
) EvidenceRecord {
	// Extract safety scores from features
	safetyScores := make(map[string]float64)
	if result != nil {
		for k, v := range result.Features {
			safetyScores[k] = v
		}
	}

	// Determine status
	status := "allowed"
	if !valid {
		status = "blocked"
	}

	// Extract violation details
	var violations []string
	if result != nil {
		for _, v := range result.Violations {
			violations = append(violations, fmt.Sprintf("%s: %.2f (expected: %.2f-%.2f)",
				v.Feature, v.Value, v.Bounds.Min, v.Bounds.Max))
		}
	}

	// Calculate total latency
	latencyMs := int64(0)
	if result != nil {
		latencyMs = result.ExtractionDuration.Milliseconds() + result.ValidationDuration.Milliseconds()
	}

	return EvidenceRecord{
		TraceID:      traceID,
		DCID:         contractID,
		PolicyDigest: "sha256:calculated_from_contract", // TODO: Calculate actual hash
		Suite:        "S0",                              // TODO: Get from contract
		Profile:      "balanced",                        // TODO: Get from contract
		SafetyScores: safetyScores,
		LatencyMs:    latencyMs,
		Status:       status,
		TenantID:     tenantID,
		Violations:   violations,
	}
}

// getBackendURL returns the backend URL from environment or default
func getBackendURL() string {
	// Check for control plane URL (local mode)
	if url := os.Getenv("CONTROL_PLANE_URL"); url != "" {
		return url
	}
	// Check for backend URL (cloud mode)
	if url := os.Getenv("BACKEND_URL"); url != "" {
		return url
	}
	// Default for local testing
	return "http://localhost:3002"
}
