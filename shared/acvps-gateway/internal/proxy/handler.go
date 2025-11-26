package proxy

import (
	"bytes"
	"compress/gzip"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httputil"
	"net/url"
	"strings"
	"time"

	"github.com/andybalholm/brotli"
	"github.com/ethicalzen/acvps-gateway/internal/blockchain"
	"github.com/ethicalzen/acvps-gateway/internal/cache"
	"github.com/ethicalzen/acvps-gateway/internal/config"
	"github.com/ethicalzen/acvps-gateway/internal/validation"
	"github.com/ethicalzen/acvps-gateway/pkg/gateway"
	"github.com/ethicalzen/acvps-gateway/pkg/telemetry"
	log "github.com/sirupsen/logrus"
	"google.golang.org/api/idtoken"
)

// decompressResponse decompresses the response body based on Content-Encoding header
func decompressResponse(body []byte, contentEncoding string) ([]byte, error) {
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

// Handler represents the ACVPS proxy handler
type Handler struct {
	proxy      *httputil.ReverseProxy
	backend    *url.URL
	blockchain *blockchain.Client
	cache      *cache.Client
	validator  *validation.Validator
	config     *config.Config
	transport  *http.Transport
}

// New creates a new proxy handler
func New(cfg *config.Config, bc *blockchain.Client, c *cache.Client) *Handler {
	// Parse backend URL
	backendURL, err := url.Parse(cfg.Backend.URL)
	if err != nil {
		log.Fatalf("Invalid backend URL: %v", err)
	}

	// Create custom transport with connection pooling
	transport := &http.Transport{
		MaxIdleConns:        cfg.Backend.MaxIdleConns,
		MaxIdleConnsPerHost: cfg.Backend.MaxConnsPerHost,
		IdleConnTimeout:     time.Duration(cfg.Backend.IdleConnTimeout) * time.Second,
		DisableKeepAlives:   false,
	}

	// Create validator
	validator := validation.New(cfg, bc, c)

	// Service port mapping for dynamic routing
	servicePortMap := map[string]string{
		"patient-records": "9001",
		"medical-summary": "9002",
		"prescription":    "9003",
	}

	// Create reverse proxy with dynamic service routing
	proxy := &httputil.ReverseProxy{
		Director: func(req *http.Request) {
			// For paths starting with /api, pass through unchanged (no service routing)
			if strings.HasPrefix(req.URL.Path, "/api/") {
				req.URL.Scheme = backendURL.Scheme
				req.URL.Host = backendURL.Host
				// Keep original path unchanged
				req.Host = backendURL.Host

				log.WithFields(log.Fields{
					"method":      req.Method,
					"path":        req.URL.Path,
					"target_host": backendURL.Host,
					"mode":        "passthrough",
				}).Debug("Proxying /api/* request (passthrough mode)")
				return
			}

			// Extract service name from path (e.g., /patient-records/api/health)
			// Path format: /{service-name}/{rest-of-path}
			pathParts := strings.Split(req.URL.Path, "/")
			var serviceName string
			var newPath string

			if len(pathParts) >= 2 {
				serviceName = pathParts[1]
				newPath = "/" + strings.Join(pathParts[2:], "/")
			} else {
				serviceName = ""
				newPath = req.URL.Path
			}

			// Determine target backend
			targetHost := backendURL.Host
			if port, ok := servicePortMap[serviceName]; ok {
				targetHost = "localhost:" + port
			}

			req.URL.Scheme = backendURL.Scheme
			req.URL.Host = targetHost
			req.URL.Path = newPath
			req.Host = targetHost

			// Add forwarding headers
			req.Header.Set("X-Forwarded-Host", req.Header.Get("Host"))
			req.Header.Set("X-Forwarded-Proto", "https")

			log.WithFields(log.Fields{
				"method":        req.Method,
				"original_path": "/" + serviceName + newPath,
				"service":       serviceName,
				"target_host":   targetHost,
				"new_path":      newPath,
			}).Debug("Proxying request with dynamic routing")
		},
		Transport: transport,
		ErrorHandler: func(w http.ResponseWriter, r *http.Request, err error) {
			log.WithFields(log.Fields{
				"error": err,
				"path":  r.URL.Path,
			}).Error("Proxy error")

			http.Error(w, "Bad Gateway", http.StatusBadGateway)
		},
	}

	log.WithFields(log.Fields{
		"backend_url":        backendURL.String(),
		"max_idle_conns":     cfg.Backend.MaxIdleConns,
		"max_conns_per_host": cfg.Backend.MaxConnsPerHost,
	}).Info("Proxy handler initialized")

	return &Handler{
		proxy:      proxy,
		backend:    backendURL,
		blockchain: bc,
		cache:      c,
		validator:  validator,
		config:     cfg,
		transport:  transport,
	}
}

// proxyToBackend makes a request to the backend service using http.Client
// This gives us full control over when we write to the client (after validation)
func (h *Handler) proxyToBackend(r *http.Request) (*http.Response, error) {
	// ============================================================================
	// DYNAMIC ROUTING: Extract target URL from request
	// ============================================================================
	// Priority:
	// 1. X-Target-Endpoint header (explicit routing from SDK)
	// 2. Request URL (if using HTTP CONNECT proxy mode)
	// 3. Fallback to configured backend.url (for legacy/testing)

	var targetURL string

	// Check for explicit target endpoint header
	if targetEndpoint := r.Header.Get("X-Target-Endpoint"); targetEndpoint != "" {
		// SDK provided explicit target
		targetURL = targetEndpoint
		if !strings.Contains(targetURL, "://") {
			// Relative URL - add scheme
			targetURL = "https://" + targetURL
		}
		// Append path and query if not already included
		if !strings.Contains(targetURL, r.URL.Path) {
			targetURL += r.URL.Path
		}
		if r.URL.RawQuery != "" && !strings.Contains(targetURL, "?") {
			targetURL += "?" + r.URL.RawQuery
		}
		log.WithFields(log.Fields{
			"target": targetURL,
			"source": "X-Target-Endpoint header",
		}).Debug("🎯 Routing to explicit target endpoint")
	} else if r.URL.IsAbs() {
		// Absolute URL in request (HTTP CONNECT proxy mode)
		targetURL = r.URL.String()
		log.WithFields(log.Fields{
			"target": targetURL,
			"source": "Absolute URL in request",
		}).Debug("🎯 Routing to absolute URL")
	} else {
		// Fallback to configured backend (for legacy compatibility)
		targetURL = fmt.Sprintf("%s%s", h.backend.String(), r.URL.Path)
		if r.URL.RawQuery != "" {
			targetURL += "?" + r.URL.RawQuery
		}
		log.WithFields(log.Fields{
			"target": targetURL,
			"source": "Configured backend (fallback)",
		}).Warn("⚠️  No explicit target - using fallback backend")
	}

	// ============================================================================
	// DOCKER LOCALHOST TRANSLATION
	// ============================================================================
	// When gateway runs in Docker and client sends "localhost", translate to
	// "host.docker.internal" so gateway can reach host machine services
	//
	// This is needed because:
	// - Client SDK runs on host machine (e.g., localhost:8443 → gateway)
	// - Client specifies backend target: X-Target-Endpoint: http://localhost:4500
	// - Gateway inside Docker sees "localhost" = container itself, not host!
	// - Solution: Translate "localhost" → "host.docker.internal" in Docker

	if strings.Contains(targetURL, "://localhost:") || strings.Contains(targetURL, "://localhost/") {
		originalURL := targetURL
		targetURL = strings.ReplaceAll(targetURL, "://localhost:", "://host.docker.internal:")
		targetURL = strings.ReplaceAll(targetURL, "://localhost/", "://host.docker.internal/")
		log.WithFields(log.Fields{
			"original":   originalURL,
			"translated": targetURL,
		}).Debug("🔄 Translated localhost → host.docker.internal (Docker mode)")
	}

	// Read request body (if any)
	var requestBody []byte
	if r.Body != nil {
		body, err := io.ReadAll(r.Body)
		if err != nil {
			return nil, fmt.Errorf("failed to read request body: %w", err)
		}
		requestBody = body
		// Restore body for potential re-reading
		r.Body = io.NopCloser(bytes.NewBuffer(body))
	}

	// Create new request to target backend
	req, err := http.NewRequestWithContext(r.Context(), r.Method, targetURL, bytes.NewBuffer(requestBody))
	if err != nil {
		return nil, fmt.Errorf("failed to create backend request: %w", err)
	}

	// Copy headers (except Host and Authorization to avoid conflicts)
	for key, values := range r.Header {
		// Skip certain headers that might cause issues with Cloud Run
		lowerKey := strings.ToLower(key)
		if lowerKey == "host" || lowerKey == "authorization" {
			continue
		}
		// Skip ACVPS headers - these are for gateway, not backend
		if strings.HasPrefix(lowerKey, "x-dc-") ||
			strings.HasPrefix(lowerKey, "x-target-") ||
			lowerKey == "x-tenant-id" ||
			lowerKey == "x-certificate-id" {
			continue
		}
		for _, value := range values {
			req.Header.Add(key, value)
		}
	}

	// Parse target URL to get host
	targetURLParsed, err := url.Parse(targetURL)
	if err != nil {
		return nil, fmt.Errorf("failed to parse target URL: %w", err)
	}

	// Set proper Host header for target backend
	req.Host = targetURLParsed.Host
	req.Header.Set("X-Forwarded-For", r.RemoteAddr)
	req.Header.Set("X-Forwarded-Proto", "https")

	// For Cloud Run backends, authentication happens automatically via the default credentials
	// The http.Client will use the service account's identity token when calling other Cloud Run services
	// No need to manually add Authorization header - it's handled by the transport layer

	// Debug logging
	log.WithFields(log.Fields{
		"method":       req.Method,
		"url":          targetURL,
		"host":         req.Host,
		"content_type": req.Header.Get("Content-Type"),
	}).Debug("🔄 Sending request to backend")

	// Create HTTP client with Cloud Run authentication for service-to-service calls
	ctx := req.Context()

	// If the backend is a Cloud Run service (*.run.app), add identity token
	if strings.Contains(h.backend.Host, ".run.app") {
		// Generate an identity token for the backend service
		tokenSource, err := idtoken.NewTokenSource(ctx, h.backend.String())
		if err != nil {
			log.WithError(err).Warn("⚠️  Failed to create token source, proceeding without authentication")
		} else {
			// Get the token and add it to the Authorization header
			token, err := tokenSource.Token()
			if err != nil {
				log.WithError(err).Warn("⚠️  Failed to get identity token, proceeding without authentication")
			} else {
				req.Header.Set("Authorization", "Bearer "+token.AccessToken)
				log.Debug("✅ Added identity token for Cloud Run backend authentication")
			}
		}
	}

	// Create standard HTTP client
	client := &http.Client{
		Transport: h.transport,
		Timeout:   time.Duration(h.config.Backend.Timeout) * time.Second,
	}

	// Make the request
	return client.Do(req)
}

// ServeHTTP handles incoming HTTP requests
func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	startTime := time.Now()

	// Create request context with timeout
	ctx, cancel := context.WithTimeout(r.Context(), time.Duration(h.config.Backend.Timeout)*time.Second)
	defer cancel()

	r = r.WithContext(ctx)

	// Check for certificate-based authentication (new preferred method)
	certificateID := r.Header.Get("X-Certificate-ID")
	tenantID := r.Header.Get("X-Tenant-ID")

	// Extract DC headers (legacy method or used with certificates)
	dcID := r.Header.Get("X-DC-Id")
	dcDigest := r.Header.Get("X-DC-Digest")
	dcSuite := r.Header.Get("X-DC-Suite")
	dcProfile := r.Header.Get("X-DC-Profile")
	dcTrace := r.Header.Get("X-DC-Trace")

	// Generate trace ID if not provided
	if dcTrace == "" {
		dcTrace = fmt.Sprintf("trace-%d", time.Now().UnixNano())
		r.Header.Set("X-DC-Trace", dcTrace)
	}

	// If certificate ID is provided, lookup contract details from portal
	if certificateID != "" {
		logger := log.WithFields(log.Fields{
			"certificate_id": certificateID,
			"trace_id":       dcTrace,
		})

		logger.Info("🔐 Certificate-based authentication detected")

		// Call portal API to lookup certificate
		certDetails, err := h.lookupCertificate(ctx, certificateID)
		if err != nil {
			logger.WithError(err).Error("Certificate lookup failed")
			h.writeJSONError(w, http.StatusUnauthorized, "CERT_LOOKUP_FAILED", "Failed to validate certificate", dcTrace)
			return
		}

		if certDetails == nil {
			logger.Warn("Certificate not found")
			h.writeJSONError(w, http.StatusUnauthorized, "CERT_NOT_FOUND", "Certificate not found or expired", dcTrace)
			return
		}

		// Override DC headers with certificate details
		dcID = certDetails.ContractID
		dcDigest = "cert-validated" // Certificate already validates the contract
		dcSuite = certDetails.Suite
		dcProfile = certDetails.Profile
		tenantID = certDetails.TenantID

		logger.WithFields(log.Fields{
			"contract_id": dcID,
			"tenant_id":   tenantID,
			"suite":       dcSuite,
		}).Info("✅ Certificate validated successfully")
	}

	logger := log.WithFields(log.Fields{
		"trace_id": dcTrace,
		"method":   r.Method,
		"path":     r.URL.Path,
		"dc_id":    dcID,
	})

	// Check if DC headers are present
	if dcID == "" || dcDigest == "" {
		if h.config.Validation.RequireDCHeaders {
			logger.Warn("Missing DC headers, rejecting request")
			h.writeJSONError(w, http.StatusConflict, "DC_REQUIRED", "Determinism Contract headers required", dcTrace)
			return
		}

		// Allow non-DC requests in observe mode
		logger.Info("Non-DC request, forwarding without validation")
		h.proxy.ServeHTTP(w, r)
		return
	}

	// Validate contract via blockchain
	validationStart := time.Now()
	valid, reason, contract, err := h.validator.ValidateContract(ctx, dcID, dcDigest, dcSuite, dcProfile)
	validationDuration := time.Since(validationStart)

	if err != nil {
		logger.WithError(err).Error("Contract validation failed")
		h.writeJSONError(w, http.StatusInternalServerError, "VALIDATION_ERROR", "Failed to validate contract", dcTrace)
		return
	}

	if !valid {
		logger.WithField("reason", reason).Warn("Contract validation failed")
		h.writeJSONError(w, http.StatusConflict, "DC_INVALID", reason, dcTrace)
		return
	}

	if contract != nil {
		logger.WithFields(log.Fields{
			"validation_ms": validationDuration.Milliseconds(),
			"suite":         contract.Suite,
			"status":        contract.Status,
		}).Info("Contract validated successfully")
	} else {
		logger.WithFields(log.Fields{
			"validation_ms": validationDuration.Milliseconds(),
		}).Info("Contract validated successfully (via blockchain)")
	}

	// Store contract in context for response inspection
	ctx = context.WithValue(ctx, "contract", contract)
	ctx = context.WithValue(ctx, "validation_duration", validationDuration)
	r = r.WithContext(ctx)

	// Check if contract is already loaded in runtime table (has feature extraction)
	// This is the SOURCE OF TRUTH for runtime validation
	//
	// Multi-tenant support: Contracts are stored with "contract:tenant-{tenant_id}:{contract_id}" key
	// If not already set by certificate auth, get from header
	if tenantID == "" {
		tenantID = r.Header.Get("X-Tenant-ID")
	}
	var binding *gateway.RuntimeBinding
	var bindingErr error

	// Try multiple lookup strategies
	lookupKeys := []string{}

	if tenantID != "" {
		// Strategy 1: Full key with contract prefix (what registration uses)
		lookupKeys = append(lookupKeys, fmt.Sprintf("contract:tenant-%s:%s", tenantID, dcID))
		// Strategy 2: Without contract prefix (what boot loader uses)
		lookupKeys = append(lookupKeys, fmt.Sprintf("tenant-%s:%s", tenantID, dcID))
	}

	// Strategy 3: Direct contract ID (backward compatibility)
	lookupKeys = append(lookupKeys, dcID)

	// Try each lookup key until we find the contract
	logger.WithFields(log.Fields{
		"dcID":       dcID,
		"tenantID":   tenantID,
		"lookupKeys": lookupKeys,
	}).Info("🔍 Looking up contract in runtime table")

	for _, key := range lookupKeys {
		binding, bindingErr = gateway.GetBinding(key)
		if bindingErr == nil && binding != nil {
			logger.WithFields(log.Fields{
				"lookup_key":   key,
				"extractor_id": binding.Contract.FeatureExtractor.ID,
			}).Info("✅ Found contract in runtime table")
			break
		} else {
			logger.WithFields(log.Fields{
				"lookup_key": key,
				"error":      fmt.Sprintf("%v", bindingErr),
			}).Warn("❌ Key not found in runtime table")
		}
	}

	hasFeatureExtraction := (binding != nil && bindingErr == nil)

	if hasFeatureExtraction {
		logger.WithField("extractor_id", binding.Contract.FeatureExtractor.ID).Info("✅ Contract has feature extraction enabled")
	} else {
		logger.Warn("⚠️ Contract does not have feature extraction or not loaded in runtime table")
	}

	// Make request to backend using custom client (full control over response)
	logger.WithField("backend_url", fmt.Sprintf("%s%s", h.backend.String(), r.URL.Path)).Info("🔄 Proxying request to backend...")
	backendResp, err := h.proxyToBackend(r)
	if err != nil {
		logger.WithError(err).Error("❌ Failed to proxy request to backend")
		h.writeJSONError(w, http.StatusBadGateway, "PROXY_ERROR", "Failed to reach backend service", dcTrace)
		return
	}
	defer backendResp.Body.Close()

	logger.WithFields(log.Fields{
		"status_code": backendResp.StatusCode,
		"headers":     backendResp.Header,
	}).Info("✅ Backend responded")

	// Read the entire response body
	responseBody, err := io.ReadAll(backendResp.Body)
	if err != nil {
		logger.WithError(err).Error("Failed to read backend response")
		h.writeJSONError(w, http.StatusBadGateway, "PROXY_ERROR", "Failed to read backend response", dcTrace)
		return
	}

	// Store response metadata
	backendStatusCode := backendResp.StatusCode
	backendHeaders := backendResp.Header

	// Validate response if contract has feature extraction
	if hasFeatureExtraction && backendStatusCode == http.StatusOK {
		logger.Info("🔍 Running feature extraction validation...")

		// Use the same contract ID that we found in the lookup
		contractIDForValidation := dcID
		if binding != nil {
			for _, key := range lookupKeys {
				testBinding, _ := gateway.GetBinding(key)
				if testBinding == binding {
					contractIDForValidation = key
					break
				}
			}
		}

		// Decompress response body for validation if needed
		contentEncoding := backendHeaders.Get("Content-Encoding")
		validationBody := responseBody
		if contentEncoding != "" && contentEncoding != "identity" {
			logger.WithField("content_encoding", contentEncoding).Info("📦 Decompressing response for validation...")
			decompressedBody, err := decompressResponse(responseBody, contentEncoding)
			if err != nil {
				logger.WithError(err).Warn("Failed to decompress response, validating compressed body")
			} else {
				validationBody = decompressedBody
				logger.WithFields(log.Fields{
					"compressed_size":   len(responseBody),
					"decompressed_size": len(validationBody),
				}).Info("✅ Response decompressed for validation")
			}
		}

		validationResult, err := gateway.ValidateResponse(contractIDForValidation, validationBody, dcTrace)
		if err != nil {
			// Check if it's an envelope violation
			if violation, ok := err.(*gateway.EnvelopeViolation); ok {
				logger.WithFields(log.Fields{
					"feature": violation.Feature,
					"value":   violation.Value,
					"bounds":  fmt.Sprintf("[%.4f, %.4f]", violation.Bounds.Min, violation.Bounds.Max),
				}).Warn("Envelope violation detected")

				// Write violation error instead of backend response
				h.writeEnvelopeViolationError(w, dcID, validationResult, dcTrace)
				return
			}

			// Other extraction errors
			logger.WithError(err).Error("Feature extraction failed")
			h.writeJSONError(w, http.StatusUnprocessableEntity, "EXTRACTION_ERROR", err.Error(), dcTrace)
			return
		}

		// Log validation success
		logger.WithFields(log.Fields{
			"extraction_ms": validationResult.ExtractionDuration.Milliseconds(),
			"validation_ms": validationResult.ValidationDuration.Milliseconds(),
			"features":      len(validationResult.Features),
		}).Info("Response validation passed")
	}

	// Validation passed (or no validation needed) - forward backend response to client
	// Copy headers
	for k, v := range backendHeaders {
		for _, val := range v {
			w.Header().Add(k, val)
		}
	}
	// Write status code
	w.WriteHeader(backendStatusCode)
	// Write body
	w.Write(responseBody)

	// Log request completion
	duration := time.Since(startTime)
	logger.WithFields(log.Fields{
		"status_code":   backendStatusCode,
		"duration_ms":   duration.Milliseconds(),
		"validation_ms": validationDuration.Milliseconds(),
		"proxy_ms":      (duration - validationDuration).Milliseconds(),
	}).Info("Request completed")

	// ============================================================================
	// TELEMETRY: Emit metrics to sidecar service
	// ============================================================================
	collector := telemetry.GetCollector()

	// Emit request event
	collector.AddRequest(telemetry.RequestEvent{
		Timestamp:         startTime.Format(time.RFC3339),
		TenantID:          tenantID,
		TraceID:           dcTrace,
		ContractID:        dcID,
		CertificateID:     certificateID,
		Method:            r.Method,
		Path:              r.URL.Path,
		StatusCode:        backendStatusCode,
		ResponseTimeMs:    duration.Milliseconds(),
		RequestSizeBytes:  r.ContentLength,
		ResponseSizeBytes: int64(len(responseBody)),
		IPAddress:         r.RemoteAddr,
		UserAgent:         r.UserAgent(),
	})

	// Emit violation events (if any)
	if hasFeatureExtraction && backendStatusCode == http.StatusOK {
		// Check if validation result has violations
		validationResult, _ := gateway.ValidateResponse(dcID, responseBody, dcTrace)
		if validationResult != nil && len(validationResult.Violations) > 0 {
			for _, v := range validationResult.Violations {
				// Map violation type
				violationType := "unknown"
				if v.Metric == "pii_risk" || v.Feature == "pii_risk" {
					violationType = "pii_leakage"
				} else if v.Metric == "grounding_confidence" || v.Feature == "grounding_confidence" {
					violationType = "low_grounding"
				} else if v.Metric == "hallucination_risk" || v.Feature == "hallucination_risk" {
					violationType = "hallucination"
				}

				// Determine severity based on how far outside bounds
				severity := "medium"
				if v.Value > v.Bounds.Max*1.5 || v.Value < v.Bounds.Min*0.5 {
					severity = "high"
				} else if v.Value > v.Bounds.Max*1.2 || v.Value < v.Bounds.Min*0.8 {
					severity = "medium"
				} else {
					severity = "low"
				}

				collector.AddViolation(telemetry.ViolationEvent{
					Timestamp:     time.Now().Format(time.RFC3339),
					TenantID:      tenantID,
					TraceID:       dcTrace,
					ContractID:    dcID,
					CertificateID: certificateID,
					ViolationType: violationType,
					MetricName:    v.Metric,
					MetricValue:   v.Value,
					ThresholdMin:  v.Bounds.Min,
					ThresholdMax:  v.Bounds.Max,
					Severity:      severity,
					Details:       fmt.Sprintf("%s outside bounds [%.4f, %.4f]", v.Metric, v.Bounds.Min, v.Bounds.Max),
				})
			}
		}
	}
}

// CertificateDetails represents certificate information from the portal
type CertificateDetails struct {
	CertificateID string `json:"certificateId"`
	ContractID    string `json:"contractId"`
	TenantID      string `json:"tenantId"`
	Suite         string `json:"suite"`
	Profile       string `json:"profile"`
	Status        string `json:"status"`
	IssuedAt      string `json:"issuedAt"`
	ExpiresAt     string `json:"expiresAt"`
}

// lookupCertificate calls the portal API to validate a certificate and get contract details
func (h *Handler) lookupCertificate(ctx context.Context, certificateID string) (*CertificateDetails, error) {
	// Get portal URL from config
	portalURL := h.config.Evidence.ControlPlaneURL
	if portalURL == "" {
		portalURL = "http://localhost:8080" // Fallback for local development
	}

	// Call portal certificate API
	certURL := fmt.Sprintf("%s/api/certificates/%s", portalURL, certificateID)

	req, err := http.NewRequestWithContext(ctx, "GET", certURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	client := &http.Client{Timeout: 5 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to call portal API: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return nil, nil // Certificate not found
	}

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("portal API returned %d: %s", resp.StatusCode, string(body))
	}

	// Parse response
	var apiResp struct {
		Success     bool `json:"success"`
		Certificate struct {
			CertificateID string `json:"certificateId"`
			ContractID    string `json:"contractId"`
			TenantID      string `json:"tenantId"`
			Status        string `json:"status"`
			IssuedAt      string `json:"issuedAt"`
			ExpiresAt     string `json:"expiresAt"`
		} `json:"certificate"`
		Contract map[string]interface{} `json:"contract"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&apiResp); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	if !apiResp.Success {
		return nil, fmt.Errorf("certificate validation failed")
	}

	// Extract suite and profile from contract
	suite := "S0"         // Default
	profile := "balanced" // Default

	if s, ok := apiResp.Contract["suite"].(string); ok {
		suite = s
	}
	if p, ok := apiResp.Contract["failover_profile"].(string); ok {
		profile = p
	} else if p, ok := apiResp.Contract["profile"].(string); ok {
		profile = p
	}

	return &CertificateDetails{
		CertificateID: apiResp.Certificate.CertificateID,
		ContractID:    apiResp.Certificate.ContractID,
		TenantID:      apiResp.Certificate.TenantID,
		Suite:         suite,
		Profile:       profile,
		Status:        apiResp.Certificate.Status,
		IssuedAt:      apiResp.Certificate.IssuedAt,
		ExpiresAt:     apiResp.Certificate.ExpiresAt,
	}, nil
}

// writeJSONError writes a JSON error response
func (h *Handler) writeJSONError(w http.ResponseWriter, statusCode int, errorCode, message, traceID string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)

	response := fmt.Sprintf(`{"error":"%s","message":"%s","trace_id":"%s"}`, errorCode, message, traceID)
	io.WriteString(w, response)
}

// writeEnvelopeViolationError writes a detailed envelope violation error
func (h *Handler) writeEnvelopeViolationError(w http.ResponseWriter, contractID string, result *gateway.ValidationResult, traceID string) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("X-ACVPS-Error", "ENVELOPE_VIOLATION")
	w.Header().Set("X-Trace-ID", traceID)
	w.WriteHeader(http.StatusConflict)

	// Build violations JSON
	violationsJSON := "["
	for i, v := range result.Violations {
		if i > 0 {
			violationsJSON += ","
		}
		violationsJSON += fmt.Sprintf(`{"feature":"%s","value":%.4f,"bounds":{"min":%.4f,"max":%.4f}}`,
			v.Feature, v.Value, v.Bounds.Min, v.Bounds.Max)
	}
	violationsJSON += "]"

	response := fmt.Sprintf(`{"error":"ENVELOPE_VIOLATION","contract_id":"%s","violations":%s,"trace_id":"%s","blocked":true}`,
		contractID, violationsJSON, traceID)
	io.WriteString(w, response)
}

// dummyWriter is a no-op writer that prevents ReverseProxy from writing to the real client
type dummyWriter struct{}

func (d *dummyWriter) Header() http.Header {
	return make(http.Header)
}

func (d *dummyWriter) Write(b []byte) (int, error) {
	return len(b), nil // Pretend to write successfully
}

func (d *dummyWriter) WriteHeader(statusCode int) {
	// Do nothing
}

// responseWriter wraps http.ResponseWriter to capture status code and body
// It buffers the response and only writes to the client after validation
type responseWriter struct {
	http.ResponseWriter                     // Dummy writer (not the real one)
	realWriter          http.ResponseWriter // The actual client connection
	statusCode          int
	body                *bytes.Buffer
	headers             http.Header
	headerWritten       bool
	blocked             bool // Set to true if response should not be flushed (e.g., violation detected)
	flushEnabled        bool // Set to true only after validation passes
}

func (rw *responseWriter) WriteHeader(code int) {
	if !rw.headerWritten {
		rw.statusCode = code
		rw.headerWritten = true
		// DON'T write to underlying ResponseWriter yet - buffer it
	}
}

func (rw *responseWriter) Write(b []byte) (int, error) {
	// Only capture body - DON'T write to client yet
	return rw.body.Write(b)
}

func (rw *responseWriter) Header() http.Header {
	return rw.headers
}

// Flush writes the buffered response to the actual client
func (rw *responseWriter) Flush() {
	// Only flush if explicitly enabled (after validation passes)
	if !rw.flushEnabled || rw.blocked {
		return
	}

	// Copy captured headers to REAL response writer
	for k, v := range rw.headers {
		for _, val := range v {
			rw.realWriter.Header().Add(k, val)
		}
	}
	// Write status code to REAL writer
	rw.realWriter.WriteHeader(rw.statusCode)
	// Write body to REAL writer
	rw.realWriter.Write(rw.body.Bytes())
}
