package api

import (
	"context"
	"fmt"
	"net/http"
	"strings"

	log "github.com/sirupsen/logrus"
)

// TenantContext key for storing tenant ID in context
type contextKey string

const TenantContextKey contextKey = "tenant_id"

// TenantConfig holds tenant authentication configuration
type TenantConfig struct {
	EnableAuth   bool
	APIKeys      map[string]string // apiKey -> tenantID mapping (legacy static keys)
	APIValidator *ApiKeyValidator  // Redis-based validator (preferred)
}

// DefaultTenantConfig returns a default configuration for local testing
func DefaultTenantConfig() *TenantConfig {
	return &TenantConfig{
		EnableAuth: false, // Disabled for local portal testing (enable in production)
		APIKeys: map[string]string{
			"demo-tenant-123-key": "demo-tenant-123",
			"demo-tenant-456-key": "demo-tenant-456",
			"test-api-key":        "default-tenant",
		},
	}
}

// TenantAuthMiddleware extracts and validates tenant from request
func TenantAuthMiddleware(config *TenantConfig) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Extract tenant ID from headers
			tenantID := r.Header.Get("X-Tenant-ID")
			apiKey := r.Header.Get("X-API-Key")

		// If auth is disabled, use default tenant
		if !config.EnableAuth {
			if tenantID == "" {
				tenantID = "default" // Changed from "default-tenant" to match contract loading
			}
			log.WithField("tenant_id", tenantID).Debug("Tenant auth disabled, using tenant from header or default")
			ctx := context.WithValue(r.Context(), TenantContextKey, tenantID)
			next.ServeHTTP(w, r.WithContext(ctx))
			return
		}

		// Validate API key if auth is enabled
		if apiKey == "" {
			log.Warn("Missing X-API-Key header")
			writeJSONError(w, http.StatusUnauthorized, "Missing X-API-Key header")
			return
		}

		var validTenantID string
		var ok bool

		// Try Redis validator first (preferred)
		if config.APIValidator != nil {
			var err error
			validTenantID, err = config.APIValidator.ValidateApiKey(apiKey)
			if err != nil {
				log.WithField("api_key", maskAPIKey(apiKey)).WithError(err).Warn("API key validation failed")
				writeJSONError(w, http.StatusUnauthorized, "Invalid API key")
				return
			}
			ok = true
		} else {
			// Fallback to static APIKeys map
			validTenantID, ok = config.APIKeys[apiKey]
			if !ok {
				log.WithField("api_key", maskAPIKey(apiKey)).Warn("Invalid API key")
				writeJSONError(w, http.StatusUnauthorized, "Invalid API key")
				return
			}
		}

			// If tenant ID is provided in header, verify it matches the API key
			if tenantID != "" && tenantID != validTenantID {
				log.WithFields(log.Fields{
					"header_tenant_id": tenantID,
					"valid_tenant_id":  validTenantID,
				}).Warn("Tenant ID mismatch")
				writeJSONError(w, http.StatusForbidden, "Tenant ID does not match API key")
				return
			}

			// Use the validated tenant ID
			tenantID = validTenantID

			log.WithFields(log.Fields{
				"tenant_id": tenantID,
				"path":      r.URL.Path,
			}).Debug("Tenant authenticated")

			// Add tenant to context
			ctx := context.WithValue(r.Context(), TenantContextKey, tenantID)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// GetTenantID extracts tenant ID from request context
func GetTenantID(r *http.Request) string {
	tenantID, ok := r.Context().Value(TenantContextKey).(string)
	if !ok || tenantID == "" {
		// Check header as fallback
		tenantID = r.Header.Get("X-Tenant-ID")
		if tenantID == "" {
			return "default" // Match the default from docker-compose
		}
	}
	return tenantID
}

// GetTenantContractKey returns the namespaced contract key for a tenant
func GetTenantContractKey(tenantID, contractID string) string {
	// Format: "contract:tenant-{tenantID}:{contractID}"
	return fmt.Sprintf("contract:tenant-%s:%s", tenantID, contractID)
}

// ParseTenantContractKey extracts tenant ID and contract ID from a namespaced key
func ParseTenantContractKey(key string) (tenantID string, contractID string, err error) {
	// Remove "contract:" prefix
	key = strings.TrimPrefix(key, "contract:")

	// Check if it's a tenant-namespaced key
	if !strings.HasPrefix(key, "tenant-") {
		// Legacy key format (no tenant namespace)
		return "default-tenant", key, nil
	}

	// Parse "tenant-{tenantID}:{contractID}"
	parts := strings.SplitN(key, ":", 2)
	if len(parts) != 2 {
		return "", "", fmt.Errorf("invalid tenant contract key format: %s", key)
	}

	// Extract tenant ID (remove "tenant-" prefix)
	tenantID = strings.TrimPrefix(parts[0], "tenant-")
	contractID = parts[1]

	return tenantID, contractID, nil
}

// maskAPIKey masks an API key for logging (shows only first/last 4 chars)
func maskAPIKey(apiKey string) string {
	if len(apiKey) <= 8 {
		return "****"
	}
	return apiKey[:4] + "****" + apiKey[len(apiKey)-4:]
}

// writeJSONError writes a JSON error response
func writeJSONError(w http.ResponseWriter, statusCode int, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	fmt.Fprintf(w, `{"error":"AUTHENTICATION_ERROR","message":"%s"}`, message)
}
