package api

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/ethicalzen/acvps-gateway/pkg/gateway"
	log "github.com/sirupsen/logrus"
)

// StreamHandler handles stream-based validation (raw bytes, no parsing)
type StreamHandler struct {
	// No dependencies needed - fully stateless
}

// NewStreamHandler creates a new stream handler
func NewStreamHandler() *StreamHandler {
	return &StreamHandler{}
}

// ValidateStream handles stream-based validation
// POST /api/stream/validate
//
// Headers:
//
//	X-Contract-ID: contract identifier
//	X-Target-Endpoint: backend URL to proxy to
//
// Body: Raw bytes (any format - JSON, Proto, XML, text, binary)
//
// Flow:
//  1. Validate request stream (raw bytes)
//  2. If valid, forward to backend (unchanged)
//  3. Validate response stream (raw bytes)
//  4. If valid, return to client (unchanged)
func (h *StreamHandler) ValidateStream(w http.ResponseWriter, r *http.Request) {
	log.WithFields(log.Fields{
		"method": r.Method,
		"path":   r.URL.Path,
		"remote": r.RemoteAddr,
	}).Info("🌊 Stream validation handler called")

	// Get contract ID from header
	contractID := r.Header.Get("X-Contract-ID")
	if contractID == "" {
		contractID = r.Header.Get("X-DC-Id") // Backward compat
	}
	if contractID == "" {
		h.writeError(w, http.StatusBadRequest, "Missing X-Contract-ID header")
		return
	}

	// Get target endpoint
	targetEndpoint := r.Header.Get("X-Target-Endpoint")
	if targetEndpoint == "" {
		h.writeError(w, http.StatusBadRequest, "Missing X-Target-Endpoint header")
		return
	}

	// Get tenant ID
	tenantID := GetTenantID(r)
	tenantContractKey := GetTenantContractKey(tenantID, contractID)

	// Read raw request stream
	requestStream, err := io.ReadAll(r.Body)
	if err != nil {
		log.WithError(err).Error("Failed to read request stream")
		h.writeError(w, http.StatusBadRequest, "Failed to read request stream")
		return
	}

	log.WithFields(log.Fields{
		"stream_size":  len(requestStream),
		"content_type": r.Header.Get("Content-Type"),
	}).Info("📥 Request stream received")

	// ========================================================================
	// STEP 1: VALIDATE REQUEST STREAM (raw bytes, no parsing)
	// ========================================================================
	traceIDRequest := fmt.Sprintf("stream-req-%d", time.Now().UnixNano())

	log.Info("🔍 Validating request stream with probabilistic guardrails")

	requestResult, err := gateway.ValidateStream(
		tenantContractKey,
		requestStream,
		traceIDRequest,
		gateway.StreamContext{
			Direction:   "request",
			ContentType: r.Header.Get("Content-Type"),
		},
	)

	if err != nil {
		// Request stream blocked
		log.WithError(err).Warn("🚫 Request stream BLOCKED by guardrails")

		h.writeStreamError(w, http.StatusForbidden, requestResult)
		return
	}

	log.WithFields(log.Fields{
		"metrics":    requestResult.Metrics,
		"confidence": requestResult.Confidence,
	}).Info("✅ Request stream validated")

	// ========================================================================
	// STEP 2: FORWARD TO BACKEND (unchanged stream)
	// ========================================================================
	log.WithField("target", targetEndpoint).Info("📤 Forwarding stream to backend")

	targetReq, err := http.NewRequestWithContext(r.Context(), r.Method, targetEndpoint, bytes.NewReader(requestStream))
	if err != nil {
		h.writeError(w, http.StatusInternalServerError, "Failed to create backend request")
		return
	}

	// Copy headers
	targetReq.Header = r.Header.Clone()

	client := &http.Client{Timeout: 30 * time.Second}
	backendResp, err := client.Do(targetReq)
	if err != nil {
		log.WithError(err).Error("❌ Backend request failed")
		h.writeError(w, http.StatusBadGateway, "Backend unavailable")
		return
	}
	defer backendResp.Body.Close()

	// Read response stream
	responseStream, err := io.ReadAll(backendResp.Body)
	if err != nil {
		h.writeError(w, http.StatusInternalServerError, "Failed to read backend response")
		return
	}

	log.WithFields(log.Fields{
		"status":      backendResp.StatusCode,
		"stream_size": len(responseStream),
	}).Info("📥 Response stream received from backend")

	// ========================================================================
	// STEP 3: VALIDATE RESPONSE STREAM (raw bytes, no parsing)
	// ========================================================================
	traceIDResponse := fmt.Sprintf("stream-resp-%d", time.Now().UnixNano())

	log.Info("🔍 Validating response stream with probabilistic guardrails")

	responseResult, err := gateway.ValidateStream(
		tenantContractKey,
		responseStream,
		traceIDResponse,
		gateway.StreamContext{
			Direction:   "response",
			ContentType: backendResp.Header.Get("Content-Type"),
		},
	)

	if err != nil {
		// Response stream blocked
		log.WithError(err).Warn("🚫 Response stream BLOCKED by guardrails")

		h.writeStreamError(w, http.StatusForbidden, responseResult)
		return
	}

	log.WithFields(log.Fields{
		"metrics":    responseResult.Metrics,
		"confidence": responseResult.Confidence,
	}).Info("✅ Response stream validated")

	// ========================================================================
	// STEP 4: RETURN TO CLIENT (unchanged stream)
	// ========================================================================
	log.Info("✅ Stream validation complete, returning to client")

	// Copy response headers
	for key, values := range backendResp.Header {
		for _, value := range values {
			w.Header().Add(key, value)
		}
	}

	// Add EthicalZen validation headers
	w.Header().Set("X-EthicalZen-Validated", "true")
	w.Header().Set("X-EthicalZen-Trace-Request", traceIDRequest)
	w.Header().Set("X-EthicalZen-Trace-Response", traceIDResponse)
	w.Header().Set("X-EthicalZen-Mode", "stream")
	w.Header().Set("X-EthicalZen-Request-Confidence", fmt.Sprintf("%.2f", requestResult.Confidence))
	w.Header().Set("X-EthicalZen-Response-Confidence", fmt.Sprintf("%.2f", responseResult.Confidence))

	// Write response
	w.WriteHeader(backendResp.StatusCode)
	w.Write(responseStream)
}

func (h *StreamHandler) writeError(w http.ResponseWriter, status int, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	fmt.Fprintf(w, `{"error":"%s"}`, message)
}

func (h *StreamHandler) writeStreamError(w http.ResponseWriter, status int, result *gateway.StreamValidationResult) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)

	// Convert metrics to JSON
	metricsJSON, _ := json.Marshal(result.Metrics)

	// Build violations array
	violationsJSON := "[]"
	if len(result.Violations) > 0 {
		violations := make([]map[string]interface{}, len(result.Violations))
		for i, v := range result.Violations {
			violations[i] = map[string]interface{}{
				"metric":     v.Metric,
				"value":      v.Value,
				"threshold":  v.Threshold,
				"confidence": v.Confidence,
			}
		}
		vJSON, _ := json.Marshal(violations)
		violationsJSON = string(vJSON)
	}

	// Return validation details
	fmt.Fprintf(w, `{
		"valid": false,
		"blocked_by": "stream_guardrails",
		"metrics": %s,
		"confidence": %.2f,
		"violations": %s,
		"calculation_time_ms": %d,
		"validation_time_ms": %d
	}`, metricsJSON, result.Confidence, violationsJSON,
		result.CalculationTime.Milliseconds(),
		result.ValidationTime.Milliseconds())
}
