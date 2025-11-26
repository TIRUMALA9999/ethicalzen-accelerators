package api

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/ethicalzen/acvps-gateway/internal/cache"
	"github.com/gorilla/mux"
)

func TestNew(t *testing.T) {
	c := cache.NewMockCache()
	handler := New(c)

	if handler == nil {
		t.Fatal("Expected non-nil handler")
	}

	if handler.cache == nil {
		t.Error("Expected cache to be set")
	}

	if handler.tenantConfig == nil {
		t.Error("Expected tenantConfig to be set")
	}
}

func TestHealthEndpoint(t *testing.T) {
	c := cache.NewMockCache()
	handler := New(c)

	req := httptest.NewRequest("GET", "/health", nil)
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}

	var response map[string]interface{}
	err := json.NewDecoder(w.Body).Decode(&response)
	if err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}

	if response["status"] != "ok" {
		t.Errorf("Expected status 'ok', got %v", response["status"])
	}
}

func TestValidateEndpoint(t *testing.T) {
	tests := []struct {
		name           string
		payload        map[string]interface{}
		expectedStatus int
	}{
		{
			name: "valid request with safe content",
			payload: map[string]interface{}{
				"contract_id": "test-service/general/us/v1.0",
				"payload": map[string]interface{}{
					"output": "This is safe content",
				},
			},
			expectedStatus: http.StatusOK,
		},
		{
			name: "request with PII",
			payload: map[string]interface{}{
				"contract_id": "test-service/healthcare/us/v1.0",
				"payload": map[string]interface{}{
					"output": "Patient John Doe, SSN: 123-45-6789",
				},
			},
			expectedStatus: http.StatusOK, // Still 200, but should have violations
		},
		{
			name: "missing contract_id",
			payload: map[string]interface{}{
				"payload": map[string]interface{}{
					"output": "Some content",
				},
			},
			expectedStatus: http.StatusBadRequest,
		},
		{
			name:           "empty payload",
			payload:        map[string]interface{}{},
			expectedStatus: http.StatusBadRequest,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := cache.NewMockCache()
			handler := New(c)

			body, _ := json.Marshal(tt.payload)
			req := httptest.NewRequest("POST", "/api/validate", bytes.NewReader(body))
			req.Header.Set("Content-Type", "application/json")
			w := httptest.NewRecorder()

			handler.ServeHTTP(w, req)

			if w.Code != tt.expectedStatus {
				t.Errorf("Expected status %d, got %d", tt.expectedStatus, w.Code)
			}
		})
	}
}

func TestAPIKeyValidation(t *testing.T) {
	t.Run("valid API key", func(t *testing.T) {
		c := cache.NewMockCache()
		validator := NewApiKeyValidator(nil) // Mock validator
		handler := NewWithValidator(c, validator)

		req := httptest.NewRequest("GET", "/api/test", nil)
		req.Header.Set("X-API-Key", "valid-test-key")
		req.Header.Set("X-Tenant-ID", "test-tenant")
		w := httptest.NewRecorder()

		handler.ServeHTTP(w, req)

		// Should not be 401 if key is valid
		if w.Code == http.StatusUnauthorized {
			t.Error("Expected request to be authorized with valid API key")
		}
	})

	t.Run("missing API key", func(t *testing.T) {
		c := cache.NewMockCache()
		validator := NewApiKeyValidator(nil)
		handler := NewWithValidator(c, validator)

		req := httptest.NewRequest("GET", "/api/test", nil)
		w := httptest.NewRecorder()

		handler.ServeHTTP(w, req)

		if w.Code != http.StatusUnauthorized {
			t.Errorf("Expected status 401, got %d", w.Code)
		}
	})
}

func TestCORSHeaders(t *testing.T) {
	c := cache.NewMockCache()
	handler := New(c)

	req := httptest.NewRequest("OPTIONS", "/api/validate", nil)
	req.Header.Set("Origin", "https://ethicalzen.ai")
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	if w.Header().Get("Access-Control-Allow-Origin") == "" {
		t.Error("Expected CORS headers to be set")
	}
}

func TestMetricsEndpoint(t *testing.T) {
	c := cache.NewMockCache()
	handler := New(c)

	req := httptest.NewRequest("GET", "/metrics", nil)
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}

	// Should return Prometheus format
	contentType := w.Header().Get("Content-Type")
	if contentType != "text/plain; version=0.0.4" {
		t.Errorf("Expected Prometheus content type, got %s", contentType)
	}
}

func BenchmarkValidateRequest(b *testing.B) {
	c := cache.NewMockCache()
	handler := New(c)

	payload := map[string]interface{}{
		"contract_id": "test-service/general/us/v1.0",
		"payload": map[string]interface{}{
			"output": "This is a test message for benchmarking",
		},
	}

	body, _ := json.Marshal(payload)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		req := httptest.NewRequest("POST", "/api/validate", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()

		handler.ServeHTTP(w, req)
	}
}

// Mock cache implementation for testing
func (c *cache.Client) NewMockCache() *cache.Client {
	// Return a mock cache client
	return &cache.Client{}
}

