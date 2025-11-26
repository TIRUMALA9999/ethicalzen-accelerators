package api

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/ethicalzen/acvps-gateway/internal/cache"
	"github.com/ethicalzen/acvps-gateway/pkg/contracts"
	"github.com/ethicalzen/acvps-gateway/pkg/gateway"
	"github.com/ethicalzen/acvps-gateway/pkg/txrepo"
	"github.com/gorilla/mux"
	log "github.com/sirupsen/logrus"
)

// Handler provides API endpoints for testing and integration
type Handler struct {
	cache          *cache.Client
	tenantConfig   *TenantConfig
	apiValidator   *ApiKeyValidator
	webhookHandler *WebhookHandler
}

// New creates a new API handler
func New(c *cache.Client) *Handler {
	return &Handler{
		cache:        c,
		tenantConfig: DefaultTenantConfig(),
		apiValidator: nil,
	}
}

// NewWithValidator creates a new API handler with API key validator
func NewWithValidator(c *cache.Client, validator *ApiKeyValidator) *Handler {
	// Enable authentication when validator is provided
	tenantConfig := DefaultTenantConfig()
	tenantConfig.EnableAuth = true
	tenantConfig.APIValidator = validator // Set the validator in tenantConfig

	return &Handler{
		cache:        c,
		tenantConfig: tenantConfig,
		apiValidator: validator,
	}
}

// NewWithTenantConfig creates a new API handler with custom tenant configuration
func NewWithTenantConfig(c *cache.Client, tenantConfig *TenantConfig) *Handler {
	return &Handler{
		cache:        c,
		tenantConfig: tenantConfig,
	}
}

// SetWebhookHandler sets the webhook handler for processing backend notifications
func (h *Handler) SetWebhookHandler(webhookHandler *WebhookHandler) {
	h.webhookHandler = webhookHandler
}

// HandleWebhook delegates to the webhook handler
func (h *Handler) HandleWebhook(w http.ResponseWriter, r *http.Request) {
	if h.webhookHandler == nil {
		http.Error(w, "Webhook handler not configured", http.StatusInternalServerError)
		return
	}
	h.webhookHandler.HandleWebhook(w, r)
}

// CORSMiddleware adds CORS headers to allow browser requests
func CORSMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Allow requests from any origin in dev mode
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization, X-DC-Id, X-DC-Suite, X-DC-Profile, X-DC-Digest")
		w.Header().Set("Access-Control-Max-Age", "3600")

		// Handle preflight requests
		if r.Method == "OPTIONS" {
			w.WriteHeader(http.StatusOK)
			return
		}

		next.ServeHTTP(w, r)
	})
}

// RegisterRoutes registers all API routes
func (h *Handler) RegisterRoutes(router *mux.Router) {
	// Public endpoints (no auth required)
	router.HandleFunc("/health", h.HealthCheck).Methods("GET", "OPTIONS")
	router.HandleFunc("/discovery/guardrails", h.DiscoverGuardrails).Methods("GET", "OPTIONS")
	
	// Webhook endpoint (validated via headers, not tenant middleware)
	router.HandleFunc("/api/webhooks", h.HandleWebhook).Methods("POST", "OPTIONS")

	api := router.PathPrefix("/api").Subrouter()

	// Apply tenant authentication middleware to all API routes
	api.Use(TenantAuthMiddleware(h.tenantConfig))

	// Transparent proxy endpoint (production mode)
	api.HandleFunc("/proxy", h.ProxyRequest).Methods("POST", "GET", "PUT", "DELETE", "PATCH")

	// Explicit validation endpoint (testing mode)
	api.HandleFunc("/extract-features", h.ExtractFeatures).Methods("POST")
	log.Info("✅ Registered route: POST /api/extract-features")
	api.HandleFunc("/validate", h.Validate).Methods("POST")
	log.Info("✅ Registered route: POST /api/validate")

	// Contract management
	api.HandleFunc("/contracts", h.RegisterContract).Methods("POST")
	api.HandleFunc("/contracts/{id}", h.GetContract).Methods("GET")
	api.HandleFunc("/contracts", h.ListContracts).Methods("GET")

	// Guardrail management (dynamic registration)
	api.HandleFunc("/guardrails/register", h.RegisterGuardrail).Methods("POST")
	api.HandleFunc("/guardrails/configs", h.ListGuardrailConfigs).Methods("GET")
	api.HandleFunc("/guardrails/configs/{id}", h.GetGuardrailConfig).Methods("GET")
	api.HandleFunc("/guardrails/list", h.ListAllGuardrails).Methods("GET")
	api.HandleFunc("/guardrails/{id}", h.DeleteGuardrail).Methods("DELETE", "OPTIONS")
	log.Info("✅ Registered route: DELETE /api/guardrails/{id}")

	// Tenant management
	api.HandleFunc("/tenants/info", h.GetTenantInfo).Methods("GET")
}

// ExtractFeaturesRequest represents a feature extraction request
type ExtractFeaturesRequest struct {
	ContractID         string                 `json:"contract_id"`
	FeatureExtractorID string                 `json:"feature_extractor_id"`
	Payload            map[string]interface{} `json:"payload"`
}

// ExtractFeaturesResponse represents the feature extraction response
type ExtractFeaturesResponse struct {
	Features         map[string]float64 `json:"features"`
	DetectionMethod  string             `json:"detection_method"`
	ExtractionTimeMs int64              `json:"extraction_time_ms"`
}

// ExtractFeatures handles feature extraction requests
func (h *Handler) ExtractFeatures(w http.ResponseWriter, r *http.Request) {
	var req ExtractFeaturesRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.writeError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	// Get tenant ID from context
	tenantID := GetTenantID(r)

	startTime := time.Now()

	// Get guardrail from dynamic registry (supports custom, LLM-template, and built-in)
	extractorFunc, extractorMeta, err := txrepo.GetGuardrail(req.FeatureExtractorID)
	if err != nil {
		h.writeError(w, http.StatusNotFound, fmt.Sprintf("Guardrail not found: %s", req.FeatureExtractorID))
		return
	}

	// Extract output from payload
	output, ok := req.Payload["output"].(string)
	if !ok {
		h.writeError(w, http.StatusBadRequest, "Missing 'output' field in payload")
		return
	}

	// Run feature extraction
	features, err := extractorFunc([]byte(output))
	if err != nil {
		h.writeError(w, http.StatusInternalServerError, fmt.Sprintf("Feature extraction failed: %v", err))
		return
	}

	duration := time.Since(startTime)

	log.WithFields(log.Fields{
		"tenant_id":    tenantID,
		"contract_id":  req.ContractID,
		"extractor_id": req.FeatureExtractorID,
		"description":  extractorMeta.Description,
		"features":     len(features),
		"duration_ms":  duration.Milliseconds(),
	}).Info("Feature extraction completed")

	response := ExtractFeaturesResponse{
		Features:         features,
		DetectionMethod:  "acvps_gateway",
		ExtractionTimeMs: duration.Milliseconds(),
	}

	h.writeJSON(w, http.StatusOK, response)
}

// ValidateRequest represents a validation request
type ValidateRequest struct {
	ContractID string                 `json:"contract_id"`
	Payload    map[string]interface{} `json:"payload"`
}

// ValidateResponse represents the validation response
type ValidateResponse struct {
	Valid            bool               `json:"valid"`
	Features         map[string]float64 `json:"features"`
	Violations       []ViolationInfo    `json:"violations,omitempty"`
	ExtractionTimeMs int64              `json:"extraction_time_ms"`
	ValidationTimeMs int64              `json:"validation_time_ms"`
	DetectionMethod  string             `json:"detection_method"`
}

// ViolationInfo represents an envelope violation
type ViolationInfo struct {
	Feature string  `json:"feature"`
	Value   float64 `json:"value"`
	Min     float64 `json:"min"`
	Max     float64 `json:"max"`
}

// Validate handles validation requests
func (h *Handler) Validate(w http.ResponseWriter, r *http.Request) {
	log.WithFields(log.Fields{
		"method": r.Method,
		"path":   r.URL.Path,
		"remote": r.RemoteAddr,
	}).Info("🔍 Validate handler called")

	// Read the raw request body first (we need it for forwarding in proxy mode)
	rawBody, err := io.ReadAll(r.Body)
	if err != nil {
		log.WithField("error", err).Error("Failed to read request body")
		h.writeError(w, http.StatusBadRequest, "Failed to read request body")
		return
	}

	// Decode the request from the raw body
	var req ValidateRequest
	if err := json.Unmarshal(rawBody, &req); err != nil {
		log.WithField("error", err).Error("Failed to decode request")
		h.writeError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	// If contract_id not in body, try to get from headers (support both X-DC-Id and X-Contract-ID)
	if req.ContractID == "" {
		req.ContractID = r.Header.Get("X-DC-Id")
		if req.ContractID == "" {
			req.ContractID = r.Header.Get("X-Contract-ID")
		}
		log.WithField("contract_id_from_header", req.ContractID).Info("Using contract ID from header")
	}

	log.WithField("contract_id", req.ContractID).Info("Request decoded successfully")

	// Get tenant ID from context
	tenantID := GetTenantID(r)
	log.WithField("tenant_id", tenantID).Info("Got tenant ID from context")

	// Build tenant-scoped contract key
	tenantContractKey := GetTenantContractKey(tenantID, req.ContractID)
	log.WithField("tenant_contract_key", tenantContractKey).Info("Built tenant contract key")

	// Check if contract is in runtime table
	_, err = gateway.GetBinding(tenantContractKey)
	if err != nil {
		// Contract not in runtime, try loading from cache or in-memory store
		log.WithField("tenant_contract_key", tenantContractKey).Info("Contract not in runtime, loading from storage...")

		ctx := r.Context()
		var contractJSON string

		// Try Redis cache first (if enabled)
		if h.cache != nil {
			// Try runtime key format first: contract:tenant-demo:CONTRACT_ID
			contractJSON, err = h.cache.Get(ctx, tenantContractKey)

			// If not found, try backend key format: tenant:demo:contract:CONTRACT_ID
			if err != nil {
				backendKey := fmt.Sprintf("tenant:%s:contract:%s", tenantID, req.ContractID)
				log.WithField("backend_key", backendKey).Debug("Runtime key not found, trying backend key format...")

				contractJSON, err = h.cache.Get(ctx, backendKey)
				if err != nil {
					log.WithFields(log.Fields{
						"tenant_contract_key": tenantContractKey,
						"backend_key":         backendKey,
						"error":               err,
					}).Error("Contract not found in Redis (tried both formats)")
					h.writeError(w, http.StatusNotFound, fmt.Sprintf("Contract not found: %s (tenant: %s)", req.ContractID, tenantID))
					return
				}
				log.WithField("backend_key", backendKey).Info("✅ Contract found using backend key format")
			}
		} else {
			// Cache is disabled, use in-memory store
			log.Debug("Cache disabled, using in-memory store")
			var keyUsed string
			contractJSON, keyUsed, err = gateway.TryBothKeyFormats(tenantID, req.ContractID)
			if err != nil {
				log.WithFields(log.Fields{
					"contract_id": req.ContractID,
					"tenant_id":   tenantID,
					"error":       err,
				}).Error("Contract not found in memory store")
				h.writeError(w, http.StatusNotFound, fmt.Sprintf("Contract not found: %s (tenant: %s)", req.ContractID, tenantID))
				return
			}
			log.WithField("key", keyUsed).Debug("✅ Contract found in memory store")
		}

		var contract contracts.Contract
		if err := json.Unmarshal([]byte(contractJSON), &contract); err != nil {
			log.WithField("error", err).Error("Failed to parse contract")
			h.writeError(w, http.StatusInternalServerError, "Failed to parse contract")
			return
		}

		// Load contract into runtime table
		if err := gateway.LoadContract(ctx, tenantContractKey, &contract); err != nil {
			log.WithField("error", err).Error("Failed to load contract into runtime")
			h.writeError(w, http.StatusInternalServerError, fmt.Sprintf("Failed to load contract: %v", err))
			return
		}

		// Verify it was actually loaded
		if _, err := gateway.GetBinding(tenantContractKey); err != nil {
			log.WithField("error", err).Error("Contract was loaded but GetBinding still fails - key mismatch?")
			h.writeError(w, http.StatusInternalServerError, "Contract key mismatch")
			return
		}

		log.Info("Contract loaded and verified in runtime successfully")
	}

	log.Info("Contract ready, proceeding with validation")

	// ============================================================================
	// STEP 1: VALIDATE REQUEST (before forwarding to backend)
	// ============================================================================
	// Extract input from request for validation (e.g., "query" field)
	// Check both req.Payload (nested) and top-level fields (direct)
	var requestPayload map[string]interface{}
	if len(req.Payload) > 0 {
		requestPayload = req.Payload
	} else {
		// If Payload is empty, use the entire raw body as the payload
		json.Unmarshal(rawBody, &requestPayload)
	}

	var requestInput string
	if queryField, ok := requestPayload["query"].(string); ok {
		requestInput = queryField
	} else if inputField, ok := requestPayload["input"].(string); ok {
		requestInput = inputField
	} else if promptField, ok := requestPayload["prompt"].(string); ok {
		requestInput = promptField
	}

	if requestInput != "" {
		log.WithField("request_input_length", len(requestInput)).Info("🔍 Validating request input before forwarding")

		// Run request validation using the same contract/guardrails
		traceIDRequest := fmt.Sprintf("req-%d", time.Now().UnixNano())
		requestResult, err := gateway.ValidateResponse(tenantContractKey, []byte(requestInput), traceIDRequest)

		if err != nil {
			log.WithField("error", err).Error("Request validation returned error")
			// Check if it's an envelope violation
			if violation, ok := err.(*gateway.EnvelopeViolation); ok {
				// Build violations list
				violations := make([]ViolationInfo, len(requestResult.Violations))
				for i, v := range requestResult.Violations {
					violations[i] = ViolationInfo{
						Feature: v.Feature,
						Value:   v.Value,
						Min:     v.Bounds.Min,
						Max:     v.Bounds.Max,
					}
				}

				// Build response
				response := ValidateResponse{
					Valid:            false,
					Features:         requestResult.Features,
					Violations:       violations,
					ExtractionTimeMs: requestResult.ExtractionDuration.Milliseconds(),
					ValidationTimeMs: requestResult.ValidationDuration.Milliseconds(),
					DetectionMethod:  "request_validation",
				}

				log.WithFields(log.Fields{
					"tenant_id":   tenantID,
					"contract_id": req.ContractID,
					"violations":  len(violations),
					"feature":     violation.Feature,
					"value":       violation.Value,
				}).Warn("🚫 REQUEST BLOCKED: Policy violation detected in incoming request")

				// Get contract to check failover profile
				binding, err := gateway.GetBinding(tenantContractKey)
				failoverProfile := "observe" // default
				if err == nil && binding != nil && binding.Contract != nil {
					if binding.Contract.Profile != "" {
						failoverProfile = binding.Contract.Profile
					}
				}

				// Determine response based on failover profile
				statusCode := http.StatusOK
				if failoverProfile == "strict" {
					statusCode = http.StatusForbidden
					log.WithField("profile", failoverProfile).Info("Blocking request due to strict failover profile")
				} else if failoverProfile == "balanced" {
					statusCode = http.StatusForbidden
					log.WithField("profile", failoverProfile).Info("Blocking request due to balanced failover profile")
				} else {
					log.WithField("profile", failoverProfile).Info("Allowing request due to observe profile (violations logged)")
				}

				h.writeJSON(w, statusCode, response)
				return
			}

			// Other errors
			h.writeError(w, http.StatusInternalServerError, fmt.Sprintf("Request validation failed: %v", err))
			return
		}

		log.Info("✅ Request validation passed, forwarding to backend")
	} else {
		log.Warn("No input field found in request (query/input/prompt), skipping request validation")
	}

	// ============================================================================
	// STEP 2: FORWARD TO BACKEND (if request is clean)
	// ============================================================================
	// Determine target endpoint for proxy mode:
	// Priority: 1) X-Target-Endpoint header (override)  2) Contract's backend_url (future)
	targetEndpoint := r.Header.Get("X-Target-Endpoint")

	// Note: backend_url from contract will be supported in future enhancement
	// For now, X-Target-Endpoint header is required

	var output string

	if targetEndpoint != "" {
		// PROXY MODE: Forward ORIGINAL request to backend and validate response
		log.WithField("target_endpoint", targetEndpoint).Info("Proxy mode detected, forwarding request to backend")

		log.WithFields(log.Fields{
			"target_endpoint":     targetEndpoint,
			"request_body_length": len(rawBody),
			"request_body":        string(rawBody),
		}).Info("📤 Forwarding ORIGINAL request to backend")

		// Create request to target with ORIGINAL body
		ctx := r.Context()
		targetReq, err := http.NewRequestWithContext(ctx, "POST", targetEndpoint, bytes.NewReader(rawBody))
		if err != nil {
			h.writeError(w, http.StatusInternalServerError, "Failed to create target request")
			return
		}
		targetReq.Header.Set("Content-Type", "application/json")

		// Forward relevant headers to backend (like X-Inject-PHI for testing)
		for key, values := range r.Header {
			if key == "X-Inject-Phi" || key == "X-Inject-PHI" {
				for _, value := range values {
					targetReq.Header.Add(key, value)
					log.WithFields(log.Fields{
						"header": key,
						"value":  value,
					}).Info("📋 Forwarding header to backend")
				}
			}
		}

		// Call target backend
		client := &http.Client{Timeout: 30 * time.Second}
		backendResp, err := client.Do(targetReq)
		if err != nil {
			log.WithError(err).Error("❌ Failed to call backend")
			h.writeError(w, http.StatusBadGateway, fmt.Sprintf("Backend unavailable: %v", err))
			return
		}
		defer backendResp.Body.Close()

		log.WithFields(log.Fields{
			"status_code": backendResp.StatusCode,
			"status":      backendResp.Status,
		}).Info("📥 Got response from backend")

		// Read backend response
		backendBody, err := io.ReadAll(backendResp.Body)
		if err != nil {
			h.writeError(w, http.StatusInternalServerError, "Failed to read backend response")
			return
		}

		log.WithFields(log.Fields{
			"response_length": len(backendBody),
			"first_100_chars": string(backendBody)[:min(100, len(backendBody))],
		}).Info("📄 Backend response received")

		// For guardrail validation, we need to check the ENTIRE backend response,
		// not just a single field. PHI could be in any field (phi_data, patient_info, etc.)
		output = string(backendBody)

		log.WithField("output_length", len(output)).Info("Using full backend response for validation")
	} else {
		// EXPLICIT MODE: Extract output from payload
		outputField, ok := req.Payload["output"].(string)
		if !ok {
			log.Error("Missing 'output' field in payload")
			h.writeError(w, http.StatusBadRequest, "Missing 'output' field in payload. Use X-Target-Endpoint header for proxy mode or provide 'output' in payload for explicit validation.")
			return
		}
		output = outputField
		log.WithField("output_length", len(output)).Info("Extracted output from payload")
	}

	// ============================================================================
	// STEP 3: VALIDATE RESPONSE (from backend)
	// ============================================================================
	log.Info("🔍 Validating response from backend")

	// Run validation using tenant-scoped contract key
	traceID := fmt.Sprintf("resp-%d", time.Now().UnixNano())
	log.WithFields(log.Fields{
		"tenant_contract_key": tenantContractKey,
		"trace_id":            traceID,
	}).Info("Calling gateway.ValidateResponse")

	result, err := gateway.ValidateResponse(tenantContractKey, []byte(output), traceID)
	if err != nil {
		log.WithField("error", err).Error("ValidateResponse returned error")
		// Check if it's an envelope violation
		if violation, ok := err.(*gateway.EnvelopeViolation); ok {
			// Still return the full result with violation details
			violations := make([]ViolationInfo, len(result.Violations))
			for i, v := range result.Violations {
				violations[i] = ViolationInfo{
					Feature: v.Feature,
					Value:   v.Value,
					Min:     v.Bounds.Min,
					Max:     v.Bounds.Max,
				}
			}

			response := ValidateResponse{
				Valid:            false,
				Features:         result.Features,
				Violations:       violations,
				ExtractionTimeMs: result.ExtractionDuration.Milliseconds(),
				ValidationTimeMs: result.ValidationDuration.Milliseconds(),
				DetectionMethod:  "acvps_gateway",
			}

			log.WithFields(log.Fields{
				"tenant_id":   tenantID,
				"contract_id": req.ContractID,
				"violations":  len(violations),
				"feature":     violation.Feature,
				"value":       violation.Value,
			}).Warn("🚫 RESPONSE BLOCKED: Policy violation detected in backend response")

			// Emit evidence for blocked request
			evidence := CreateEvidenceFromValidation(traceID, req.ContractID, tenantID, result, false)
			h.EmitEvidence(evidence)

			// Get contract to check failover profile
			binding, err := gateway.GetBinding(tenantContractKey)
			failoverProfile := "observe" // default
			if err == nil && binding != nil && binding.Contract != nil {
				if binding.Contract.Profile != "" {
					failoverProfile = binding.Contract.Profile
				}
			}

			// Determine response based on failover profile
			// - strict: Block (403)
			// - balanced: Block with degraded response
			// - observe: Allow but log (200)
			statusCode := http.StatusOK
			if failoverProfile == "strict" {
				statusCode = http.StatusForbidden
				log.WithField("profile", failoverProfile).Info("Blocking response due to strict failover profile")
			} else if failoverProfile == "balanced" {
				statusCode = http.StatusForbidden // Also block for balanced
				log.WithField("profile", failoverProfile).Info("Blocking response due to balanced failover profile")
			} else {
				log.WithField("profile", failoverProfile).Info("Allowing request due to observe profile (violations logged)")
			}

			h.writeJSON(w, statusCode, response)
			return
		}

		// Other errors
		h.writeError(w, http.StatusInternalServerError, fmt.Sprintf("Validation failed: %v", err))
		return
	}

	// ✅ SUCCESS: Both request and response validation passed
	log.Info("✅ Validation complete: Request and response are both clean")

	response := ValidateResponse{
		Valid:            true,
		Features:         result.Features,
		Violations:       []ViolationInfo{},
		ExtractionTimeMs: result.ExtractionDuration.Milliseconds(),
		ValidationTimeMs: result.ValidationDuration.Milliseconds(),
		DetectionMethod:  "acvps_gateway",
	}

	log.WithFields(log.Fields{
		"tenant_id":     tenantID,
		"contract_id":   req.ContractID,
		"extraction_ms": result.ExtractionDuration.Milliseconds(),
		"validation_ms": result.ValidationDuration.Milliseconds(),
	}).Info("Validation passed")

	// Emit evidence for allowed request
	evidence := CreateEvidenceFromValidation(traceID, req.ContractID, tenantID, result, true)
	h.EmitEvidence(evidence)

	h.writeJSON(w, http.StatusOK, response)
}

// RegisterContractRequest represents a contract registration request
type RegisterContractRequest struct {
	ContractID string             `json:"contract_id"`
	Contract   contracts.Contract `json:"contract"`
}

// RegisterContract handles contract registration
func (h *Handler) RegisterContract(w http.ResponseWriter, r *http.Request) {
	var req RegisterContractRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.writeError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	ctx := r.Context()

	// Get tenant ID from context
	tenantID := GetTenantID(r)

	// Build tenant-scoped contract key
	tenantContractKey := GetTenantContractKey(tenantID, req.ContractID)

	// Store contract in cache with tenant namespace
	contractJSON, err := json.Marshal(req.Contract)
	if err != nil {
		h.writeError(w, http.StatusInternalServerError, "Failed to serialize contract")
		return
	}

	if err := h.cache.Set(ctx, tenantContractKey, string(contractJSON), 0); err != nil {
		h.writeError(w, http.StatusInternalServerError, "Failed to store contract")
		return
	}

	// Load contract into runtime table using tenant-scoped key
	if err := gateway.LoadContract(ctx, tenantContractKey, &req.Contract); err != nil {
		log.WithError(err).Warn("Failed to load contract into runtime table (might not have feature extraction)")
	}

	log.WithFields(log.Fields{
		"tenant_id":              tenantID,
		"contract_id":            req.ContractID,
		"tenant_contract_key":    tenantContractKey,
		"has_feature_extraction": req.Contract.HasFeatureExtraction(),
		"suite":                  req.Contract.Suite,
	}).Info("Contract registered")

	h.writeJSON(w, http.StatusCreated, map[string]interface{}{
		"success":     true,
		"tenant_id":   tenantID,
		"contract_id": req.ContractID,
		"loaded":      req.Contract.HasFeatureExtraction(),
	})
}

// GetContract retrieves a contract by ID
func (h *Handler) GetContract(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	contractID := vars["id"]

	ctx := r.Context()

	// Get tenant ID from context
	tenantID := GetTenantID(r)

	// Build tenant-scoped contract key
	tenantContractKey := GetTenantContractKey(tenantID, contractID)

	// Get from cache using tenant-scoped key
	contractJSON, err := h.cache.Get(ctx, tenantContractKey)
	if err != nil {
		h.writeError(w, http.StatusNotFound, fmt.Sprintf("Contract not found: %s (tenant: %s)", contractID, tenantID))
		return
	}

	var contract contracts.Contract
	if err := json.Unmarshal([]byte(contractJSON), &contract); err != nil {
		h.writeError(w, http.StatusInternalServerError, "Failed to parse contract")
		return
	}

	h.writeJSON(w, http.StatusOK, contract)
}

// ListContracts lists all registered contracts for the tenant
func (h *Handler) ListContracts(w http.ResponseWriter, r *http.Request) {
	// Get tenant ID from context
	tenantID := GetTenantID(r)

	// Get all loaded contracts
	allLoadedContracts := gateway.GetLoadedContracts()

	// Filter contracts for this tenant
	tenantPrefix := fmt.Sprintf("contract:tenant-%s:", tenantID)
	tenantContracts := []string{}

	for _, contractKey := range allLoadedContracts {
		if strings.HasPrefix(contractKey, tenantPrefix) {
			// Extract just the contract ID (remove tenant prefix)
			_, contractID, err := ParseTenantContractKey(contractKey)
			if err == nil {
				tenantContracts = append(tenantContracts, contractID)
			}
		}
	}

	h.writeJSON(w, http.StatusOK, map[string]interface{}{
		"tenant_id":        tenantID,
		"loaded_contracts": tenantContracts,
		"count":            len(tenantContracts),
	})
}

// GetTenantInfo returns information about the current tenant
func (h *Handler) GetTenantInfo(w http.ResponseWriter, r *http.Request) {
	tenantID := GetTenantID(r)

	// Get all loaded contracts
	allLoadedContracts := gateway.GetLoadedContracts()

	// Filter contracts for this tenant
	tenantPrefix := fmt.Sprintf("contract:tenant-%s:", tenantID)
	tenantContractCount := 0

	for _, contractKey := range allLoadedContracts {
		if strings.HasPrefix(contractKey, tenantPrefix) {
			tenantContractCount++
		}
	}

	h.writeJSON(w, http.StatusOK, map[string]interface{}{
		"tenant_id":      tenantID,
		"contract_count": tenantContractCount,
		"auth_enabled":   h.tenantConfig.EnableAuth,
	})
}

// Helper methods
func (h *Handler) writeJSON(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(data)
}

func (h *Handler) writeError(w http.ResponseWriter, status int, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(map[string]string{
		"error":   "API_ERROR",
		"message": message,
	})
}

// ============================================================================
// Guardrail Management Handlers
// ============================================================================

// RegisterGuardrail handles POST /api/guardrails/register
func (h *Handler) RegisterGuardrail(w http.ResponseWriter, r *http.Request) {
	var config txrepo.GuardrailConfig
	if err := json.NewDecoder(r.Body).Decode(&config); err != nil {
		h.writeError(w, http.StatusBadRequest, fmt.Sprintf("Invalid JSON: %v", err))
		return
	}

	// Validate required fields
	if config.ID == "" || config.Name == "" || config.Description == "" {
		h.writeError(w, http.StatusBadRequest, "Missing required fields: id, name, description")
		return
	}

	// Set timestamp
	config.RegisteredAt = time.Now().Format(time.RFC3339)
	config.Type = "dynamic"

	// Register the configuration
	if err := txrepo.RegisterConfig(&config); err != nil {
		h.writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	// Save to filesystem repository for persistence
	guardrailRepoPath := os.Getenv("GUARDRAIL_REPO_PATH")
	if guardrailRepoPath == "" {
		guardrailRepoPath = "../guardrail_repo"
	}
	if err := txrepo.SaveGuardrailToRepository(&config, guardrailRepoPath); err != nil {
		log.WithError(err).Warn("Failed to save guardrail to repository (continuing anyway)")
	}

	// Check if custom implementation exists
	hasCustom := txrepo.HasCustomImplementation(config.ID)
	source := "generic_llm_template"
	if hasCustom {
		source = "custom_override"
	}

	log.WithFields(log.Fields{
		"guardrail_id": config.ID,
		"name":         config.Name,
		"source":       source,
	}).Info("Guardrail registered")

	h.writeJSON(w, http.StatusOK, map[string]interface{}{
		"success":      true,
		"guardrail_id": config.ID,
		"message":      fmt.Sprintf("Guardrail '%s' registered successfully", config.Name),
		"source":       source,
	})
}

// DeleteGuardrail handles DELETE /api/guardrails/{id}
func (h *Handler) DeleteGuardrail(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	id := vars["id"]

	if id == "" {
		h.writeError(w, http.StatusBadRequest, "Guardrail ID is required")
		return
	}

	// Check if it's a static guardrail (built-in) - these cannot be deleted
	if _, _, err := txrepo.GlobalRegistry.Get(id); err == nil {
		h.writeError(w, http.StatusForbidden, "Cannot delete built-in guardrails")
		return
	}

	// Check if guardrail exists and is dynamic (not built-in)
	config, err := txrepo.GetConfig(id)
	if err != nil {
		h.writeError(w, http.StatusNotFound, fmt.Sprintf("Guardrail not found: %s", id))
		return
	}

	// Only allow deletion of dynamic guardrails
	if config.Type != "dynamic" {
		h.writeError(w, http.StatusForbidden, "Cannot delete built-in guardrails")
		return
	}

	// Unregister from in-memory registry
	if err := txrepo.UnregisterConfig(id); err != nil {
		h.writeError(w, http.StatusInternalServerError, fmt.Sprintf("Failed to unregister guardrail: %v", err))
		return
	}

	// Delete from filesystem repository
	guardrailRepoPath := os.Getenv("GUARDRAIL_REPO_PATH")
	if guardrailRepoPath == "" {
		guardrailRepoPath = "../guardrail_repo"
	}

	// Try to delete from repository (not fatal if it doesn't exist)
	if err := txrepo.DeleteGuardrailFromRepository(id, guardrailRepoPath); err != nil {
		log.WithError(err).Warn("Failed to delete guardrail from repository (continuing anyway)")
	}

	log.WithFields(log.Fields{
		"guardrail_id": id,
	}).Info("Guardrail deleted")

	h.writeJSON(w, http.StatusOK, map[string]interface{}{
		"success": true,
		"message": fmt.Sprintf("Guardrail '%s' deleted successfully", id),
	})
}

// ListGuardrailConfigs handles GET /api/guardrails/configs
func (h *Handler) ListGuardrailConfigs(w http.ResponseWriter, r *http.Request) {
	configs := txrepo.ListConfigs()

	h.writeJSON(w, http.StatusOK, map[string]interface{}{
		"success": true,
		"count":   len(configs),
		"configs": configs,
	})
}

// GetGuardrailConfig handles GET /api/guardrails/configs/{id}
func (h *Handler) GetGuardrailConfig(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	id := vars["id"]

	config, err := txrepo.GetConfig(id)
	if err != nil {
		h.writeError(w, http.StatusNotFound, fmt.Sprintf("Guardrail config not found: %s", id))
		return
	}

	h.writeJSON(w, http.StatusOK, map[string]interface{}{
		"success": true,
		"config":  config,
	})
}

// ListAllGuardrails handles GET /api/guardrails/list
func (h *Handler) ListAllGuardrails(w http.ResponseWriter, r *http.Request) {
	allIDs := txrepo.ListAll()

	// Get metadata for each guardrail
	guardrails := make([]map[string]interface{}, 0, len(allIDs))
	for _, id := range allIDs {
		_, meta, err := txrepo.GetGuardrail(id)
		if err != nil {
			continue
		}

		guardrailInfo := map[string]interface{}{
			"id":          id,
			"description": meta.Description,
			"version":     meta.Version,
		}

		// Check if it's dynamic or static
		if config, err := txrepo.GetConfig(id); err == nil {
			guardrailInfo["type"] = "dynamic"
			guardrailInfo["name"] = config.Name
			guardrailInfo["metric_name"] = config.MetricName
			guardrailInfo["has_custom"] = txrepo.HasCustomImplementation(id)
		} else {
			guardrailInfo["type"] = "static"
			guardrailInfo["name"] = meta.ID
		}

		guardrails = append(guardrails, guardrailInfo)
	}

	h.writeJSON(w, http.StatusOK, map[string]interface{}{
		"success":    true,
		"count":      len(guardrails),
		"guardrails": guardrails,
	})
}

// HealthCheck provides a public health check endpoint
func (h *Handler) HealthCheck(w http.ResponseWriter, r *http.Request) {
	h.writeJSON(w, http.StatusOK, map[string]interface{}{
		"status":    "healthy",
		"service":   "acvps-gateway",
		"version":   "1.0.0",
		"timestamp": time.Now().UTC().Format(time.RFC3339),
	})
}

// DiscoverGuardrails provides a public endpoint to discover available guardrails
func (h *Handler) DiscoverGuardrails(w http.ResponseWriter, r *http.Request) {
	// Get all available guardrails (both static and dynamic)
	allIDs := txrepo.ListAll()

	var guardrails []map[string]interface{}
	for _, id := range allIDs {
		_, meta, err := txrepo.GetGuardrail(id)
		if err != nil {
			continue
		}

		guardrailInfo := map[string]interface{}{
			"id":          id,
			"description": meta.Description,
			"version":     meta.Version,
		}

		// Check if it's dynamic or static
		if config, err := txrepo.GetConfig(id); err == nil {
			guardrailInfo["type"] = "llm_assisted"
			guardrailInfo["name"] = config.Name
			guardrailInfo["llm_provider"] = "openai"
		} else {
			guardrailInfo["type"] = "deterministic"
			guardrailInfo["name"] = meta.ID
		}

		guardrails = append(guardrails, guardrailInfo)
	}

	h.writeJSON(w, http.StatusOK, map[string]interface{}{
		"success":    true,
		"count":      len(guardrails),
		"extractors": guardrails, // Use "extractors" for backward compatibility
		"guardrails": guardrails,
	})
}
