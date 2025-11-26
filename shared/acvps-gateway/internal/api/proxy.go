package api

import (
	"bytes"
	"compress/gzip"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/andybalholm/brotli"
	"github.com/ethicalzen/acvps-gateway/pkg/gateway"
	log "github.com/sirupsen/logrus"
)

// decompressResponse decompresses the response body based on Content-Encoding header
func decompressResponseBody(body []byte, contentEncoding string) ([]byte, error) {
	encoding := strings.ToLower(contentEncoding)
	
	switch encoding {
	case "br", "brotli":
		reader := brotli.NewReader(bytes.NewReader(body))
		decompressed, err := io.ReadAll(reader)
		if err != nil {
			return nil, fmt.Errorf("brotli decompression failed: %w", err)
		}
		return decompressed, nil
		
	case "gzip":
		reader, err := gzip.NewReader(bytes.NewReader(body))
		if err != nil {
			return nil, fmt.Errorf("gzip reader creation failed: %w", err)
		}
		defer reader.Close()
		decompressed, err := io.ReadAll(reader)
		if err != nil {
			return nil, fmt.Errorf("gzip decompression failed: %w", err)
		}
		return decompressed, nil
		
	case "", "identity":
		// No compression or identity encoding
		return body, nil
		
	default:
		// Unknown encoding, try to use as-is
		log.WithField("encoding", encoding).Warn("Unknown Content-Encoding, using body as-is")
		return body, nil
	}
}

// ProxyRequest handles transparent proxying with contract enforcement
// This is the production-ready mode where clients call the gateway
// and it automatically validates responses before returning them
func (h *Handler) ProxyRequest(w http.ResponseWriter, r *http.Request) {
	// Get tenant ID and contract ID from headers
	tenantID := GetTenantID(r)
	contractID := r.Header.Get("X-Contract-ID")
	if contractID == "" {
		contractID = r.Header.Get("X-DC-Id") // SDK uses X-DC-Id
	}

	if contractID == "" {
		h.writeError(w, http.StatusBadRequest, "Missing X-Contract-ID or X-DC-Id header")
		return
	}

	// Build tenant-scoped contract key
	tenantContractKey := GetTenantContractKey(tenantID, contractID)

	// Load contract to get target endpoint
	ctx := r.Context()
	var contractJSON string
	var err error

	// Try Redis cache first (if enabled)
	if h.cache != nil {
		// Try runtime key format first: contract:tenant-demo:CONTRACT_ID
		contractJSON, err = h.cache.Get(ctx, tenantContractKey)

		// If not found, try backend key format: tenant:demo:contract:CONTRACT_ID
		if err != nil {
			backendKey := fmt.Sprintf("tenant:%s:contract:%s", tenantID, contractID)
			log.WithField("backend_key", backendKey).Debug("Runtime key not found, trying backend key format...")

			contractJSON, err = h.cache.Get(ctx, backendKey)
			if err != nil {
				log.WithFields(log.Fields{
					"tenant_contract_key": tenantContractKey,
					"backend_key":         backendKey,
					"error":               err,
				}).Error("Contract not found in Redis (tried both formats)")
				h.writeError(w, http.StatusNotFound, fmt.Sprintf("Contract not found: %s (tenant: %s)", contractID, tenantID))
				return
			}
			log.WithField("backend_key", backendKey).Info("✅ Contract found using backend key format")
		}
	} else {
		// Cache is disabled, use in-memory store
		log.Debug("Cache disabled, using in-memory store")
		var keyUsed string
		contractJSON, keyUsed, err = gateway.TryBothKeyFormats(tenantID, contractID)
		if err != nil {
			log.WithFields(log.Fields{
				"contract_id": contractID,
				"tenant_id":   tenantID,
				"error":       err,
			}).Error("Contract not found in memory store")
			h.writeError(w, http.StatusNotFound, fmt.Sprintf("Contract not found: %s (tenant: %s)", contractID, tenantID))
			return
		}
		log.WithField("key", keyUsed).Debug("✅ Contract found in memory store")
	}

	var contractData map[string]interface{}
	if err := json.Unmarshal([]byte(contractJSON), &contractData); err != nil {
		h.writeError(w, http.StatusInternalServerError, "Failed to parse contract")
		return
	}

	// Get target endpoint from contract or header
	// Try backend_url first (new field), then target_endpoint (legacy)
	targetEndpoint, ok := contractData["backend_url"].(string)
	if !ok || targetEndpoint == "" {
		targetEndpoint, ok = contractData["target_endpoint"].(string)
	}

	// If not in contract, check header
	if !ok || targetEndpoint == "" {
		targetEndpoint = r.Header.Get("X-Target-Endpoint")
		if targetEndpoint == "" {
			h.writeError(w, http.StatusBadRequest, "No target endpoint configured")
			return
		}
	}

	startTime := time.Now()
	traceID := fmt.Sprintf("proxy-%d", time.Now().UnixNano())

	// Read request body once
	var requestBody []byte
	if r.Body != nil {
		requestBody, err = io.ReadAll(r.Body)
		if err != nil {
			h.writeError(w, http.StatusBadRequest, "Failed to read request body")
			return
		}
	}

	log.WithFields(log.Fields{
		"trace_id":        traceID,
		"tenant_id":       tenantID,
		"contract_id":     contractID,
		"method":          r.Method,
		"target_endpoint": targetEndpoint,
	}).Info("Proxying request to AI service")

	// Step 0: Validate REQUEST input (before forwarding to LLM)
	var inputText string
	if len(requestBody) > 0 {
		var reqJSON map[string]interface{}
		if err := json.Unmarshal(requestBody, &reqJSON); err == nil {
			// Try to extract input field
			if input, ok := reqJSON["input"].(string); ok {
				inputText = input
			} else if query, ok := reqJSON["query"].(string); ok {
				inputText = query
			} else if prompt, ok := reqJSON["prompt"].(string); ok {
				inputText = prompt
			}
		}
	}

	if inputText != "" {
		// Validate request input
		reqTraceID := fmt.Sprintf("req-%d", time.Now().UnixNano())
		reqResult, reqErr := gateway.ValidateResponse(tenantContractKey, []byte(inputText), reqTraceID)
		
		if reqErr != nil {
			if violation, ok := reqErr.(*gateway.EnvelopeViolation); ok {
				log.WithFields(log.Fields{
					"trace_id":   reqTraceID,
					"feature":    violation.Feature,
					"value":      violation.Value,
					"violations": len(reqResult.Violations),
				}).Warn("🚫 REQUEST BLOCKED: Malicious input detected")

				violations := make([]ViolationInfo, len(reqResult.Violations))
				for i, v := range reqResult.Violations {
					violations[i] = ViolationInfo{
						Feature: v.Feature,
						Value:   v.Value,
						Min:     v.Bounds.Min,
						Max:     v.Bounds.Max,
					}
				}

				blockResponse := map[string]interface{}{
					"error":   "INPUT_BLOCKED",
					"message": "Request blocked by security policy",
					"details": map[string]interface{}{
						"contract_id": contractID,
						"trace_id":    reqTraceID,
						"violations":  violations,
					},
				}

				w.Header().Set("Content-Type", "application/json")
				w.Header().Set("X-ACVPS-Status", "blocked")
				w.WriteHeader(http.StatusForbidden)
				json.NewEncoder(w).Encode(blockResponse)
				return
			}
		}
	}

	// Step 1: Forward request to AI service
	// Create request to target AI service
	targetReq, err := http.NewRequestWithContext(ctx, r.Method, targetEndpoint, bytes.NewReader(requestBody))
	if err != nil {
		h.writeError(w, http.StatusInternalServerError, "Failed to create target request")
		return
	}

	// Copy headers (except our internal ones)
	for key, values := range r.Header {
		// Skip EthicalZen-specific headers
		if key == "X-Contract-ID" || key == "X-Tenant-ID" || key == "X-Target-Endpoint" ||
			key == "X-Dc-Id" || key == "X-Dc-Digest" || key == "X-Dc-Suite" || key == "X-Api-Key" {
			continue
		}
		for _, value := range values {
			targetReq.Header.Add(key, value)
		}
	}
	
	// Log outgoing request details for debugging
	bodyPreview := string(requestBody)
	if len(bodyPreview) > 200 {
		bodyPreview = bodyPreview[:200] + "..."
	}
	log.WithFields(log.Fields{
		"trace_id":     traceID,
		"target_url":   targetEndpoint,
		"method":       targetReq.Method,
		"content_type": targetReq.Header.Get("Content-Type"),
		"has_auth":     targetReq.Header.Get("Authorization") != "",
		"body_len":     len(requestBody),
		"body_preview": bodyPreview,
	}).Info("📤 Forwarding request to AI service")

	// Call target AI service
	client := &http.Client{Timeout: 30 * time.Second}
	aiResponse, err := client.Do(targetReq)
	if err != nil {
		log.WithError(err).Error("Failed to call target AI service")
		h.writeError(w, http.StatusBadGateway, fmt.Sprintf("AI service unavailable: %v", err))
		return
	}
	defer aiResponse.Body.Close()

	// Read AI response body
	aiResponseBody, err := io.ReadAll(aiResponse.Body)
	if err != nil {
		h.writeError(w, http.StatusInternalServerError, "Failed to read AI response")
		return
	}

	forwardDuration := time.Since(startTime)

	log.WithFields(log.Fields{
		"trace_id":       traceID,
		"status_code":    aiResponse.StatusCode,
		"response_bytes": len(aiResponseBody),
		"forward_ms":     forwardDuration.Milliseconds(),
	}).Info("Received response from AI service")
	
	// Decompress response if needed (Groq and others use brotli/gzip)
	contentEncoding := aiResponse.Header.Get("Content-Encoding")
	decompressedBody := aiResponseBody
	if contentEncoding != "" && contentEncoding != "identity" {
		log.WithFields(log.Fields{
			"trace_id":         traceID,
			"content_encoding": contentEncoding,
			"compressed_size":  len(aiResponseBody),
		}).Info("📦 Decompressing AI response...")
		
		var decErr error
		decompressedBody, decErr = decompressResponseBody(aiResponseBody, contentEncoding)
		if decErr != nil {
			log.WithError(decErr).Warn("Failed to decompress response, using compressed body")
			decompressedBody = aiResponseBody
		} else {
			log.WithFields(log.Fields{
				"trace_id":          traceID,
				"compressed_size":   len(aiResponseBody),
				"decompressed_size": len(decompressedBody),
			}).Info("✅ Response decompressed successfully")
		}
	}
	
	// Log error responses for debugging
	if aiResponse.StatusCode >= 400 {
		log.WithFields(log.Fields{
			"trace_id":      traceID,
			"status_code":   aiResponse.StatusCode,
			"response_body": string(decompressedBody),
		}).Warn("AI service returned error response")
	}

	// Step 2: Extract output for validation
	// Try to parse as JSON to extract the response field
	var aiResponseJSON map[string]interface{}
	var outputText string

	if err := json.Unmarshal(decompressedBody, &aiResponseJSON); err == nil {
		// Successfully parsed as JSON - try multiple formats
		if response, ok := aiResponseJSON["response"].(string); ok {
			// Simple response format
			outputText = response
		} else if output, ok := aiResponseJSON["output"].(string); ok {
			// Output format
			outputText = output
		} else if message, ok := aiResponseJSON["message"].(string); ok {
			// Message format
			outputText = message
		} else if choices, ok := aiResponseJSON["choices"].([]interface{}); ok && len(choices) > 0 {
			// OpenAI/Groq format: choices[0].message.content
			if choice, ok := choices[0].(map[string]interface{}); ok {
				if msg, ok := choice["message"].(map[string]interface{}); ok {
					if content, ok := msg["content"].(string); ok {
						outputText = content
						log.WithField("content_length", len(content)).Debug("Extracted content from OpenAI/Groq format")
					}
				}
			}
		}
		
		// Fallback: use entire decompressed response as text
		if outputText == "" {
			outputText = string(decompressedBody)
		}
	} else {
		// Not JSON, use as-is
		outputText = string(aiResponseBody)
	}

	// Step 3: Validate response through ACVPS Gateway
	validationStart := time.Now()

	// Verify contract is loaded
	_, err = gateway.GetBinding(tenantContractKey)
	if err != nil {
		log.WithError(err).Warn("Contract not loaded in runtime table - allowing request through")
		// Return original response without validation
		w.Header().Set("X-ACVPS-Status", "not-validated")
		w.Header().Set("X-ACVPS-Reason", "contract-not-loaded")
		w.WriteHeader(aiResponse.StatusCode)
		w.Write(aiResponseBody)
		return
	}

	// Run validation
	result, err := gateway.ValidateResponse(tenantContractKey, []byte(outputText), traceID)
	validationDuration := time.Since(validationStart)

	if err != nil {
		// Check if it's an envelope violation
		if violation, ok := err.(*gateway.EnvelopeViolation); ok {
			log.WithFields(log.Fields{
				"trace_id":      traceID,
				"contract_id":   contractID,
				"feature":       violation.Feature,
				"value":         violation.Value,
				"bounds":        violation.Bounds,
				"violations":    len(result.Violations),
				"validation_ms": validationDuration.Milliseconds(),
			}).Warn("🚨 ACVPS BLOCKED - Contract violation detected")

			// Build violation response
			violations := make([]ViolationInfo, len(result.Violations))
			for i, v := range result.Violations {
				violations[i] = ViolationInfo{
					Feature: v.Feature,
					Value:   v.Value,
					Min:     v.Bounds.Min,
					Max:     v.Bounds.Max,
				}
			}

			blockResponse := map[string]interface{}{
				"error":   "CONTRACT_VIOLATION",
				"message": fmt.Sprintf("Response blocked by contract: %s", violation.Feature),
				"details": map[string]interface{}{
					"contract_id":        contractID,
					"trace_id":           traceID,
					"violations":         violations,
					"features":           result.Features,
					"validation_time_ms": validationDuration.Milliseconds(),
				},
			}

			// Return 403 Forbidden with violation details
			w.Header().Set("Content-Type", "application/json")
			w.Header().Set("X-ACVPS-Status", "blocked")
			w.Header().Set("X-ACVPS-Trace-ID", traceID)
			w.WriteHeader(http.StatusForbidden)
			json.NewEncoder(w).Encode(blockResponse)
			return
		}

		// Other validation errors - log but allow through
		log.WithError(err).Warn("Validation error - allowing request through")
		w.Header().Set("X-ACVPS-Status", "validation-error")
		w.Header().Set("X-ACVPS-Error", err.Error())
		w.WriteHeader(aiResponse.StatusCode)
		w.Write(aiResponseBody)
		return
	}

	// Step 4: Validation passed - return original AI response
	totalDuration := time.Since(startTime)

	log.WithFields(log.Fields{
		"trace_id":      traceID,
		"contract_id":   contractID,
		"features":      result.Features,
		"forward_ms":    forwardDuration.Milliseconds(),
		"validation_ms": validationDuration.Milliseconds(),
		"total_ms":      totalDuration.Milliseconds(),
		"overhead_ms":   validationDuration.Milliseconds(), // Pure ACVPS overhead
	}).Info("✅ ACVPS PASSED - Response validated and allowed")

	// Add ACVPS headers for observability
	w.Header().Set("X-ACVPS-Status", "passed")
	w.Header().Set("X-ACVPS-Trace-ID", traceID)
	w.Header().Set("X-ACVPS-Validation-Ms", fmt.Sprintf("%d", validationDuration.Milliseconds()))

	// Add feature scores as headers (for monitoring)
	for feature, score := range result.Features {
		headerName := fmt.Sprintf("X-ACVPS-Feature-%s", feature)
		w.Header().Set(headerName, fmt.Sprintf("%.4f", score))
	}

	// Copy original response headers
	for key, values := range aiResponse.Header {
		for _, value := range values {
			w.Header().Add(key, value)
		}
	}

	// Return original AI response with validation headers
	w.WriteHeader(aiResponse.StatusCode)
	w.Write(aiResponseBody)
}
