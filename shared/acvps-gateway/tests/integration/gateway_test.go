package integration

import (
	"bytes"
	"encoding/json"
	"net/http"
	"os"
	"testing"
	"time"
)

var (
	gatewayURL string
)

func TestMain(m *testing.M) {
	// Get gateway URL from environment or use default
	gatewayURL = os.Getenv("GATEWAY_URL")
	if gatewayURL == "" {
		gatewayURL = "http://localhost:8443"
	}

	// Wait for gateway to be ready
	if !waitForGateway(gatewayURL, 30*time.Second) {
		os.Exit(1)
	}

	// Run tests
	code := m.Run()
	os.Exit(code)
}

func waitForGateway(url string, timeout time.Duration) bool {
	client := &http.Client{Timeout: 2 * time.Second}
	deadline := time.Now().Add(timeout)

	for time.Now().Before(deadline) {
		resp, err := client.Get(url + "/health")
		if err == nil && resp.StatusCode == http.StatusOK {
			resp.Body.Close()
			return true
		}
		time.Sleep(1 * time.Second)
	}
	return false
}

func TestHealthEndpoint(t *testing.T) {
	resp, err := http.Get(gatewayURL + "/health")
	if err != nil {
		t.Fatalf("Failed to call health endpoint: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("Expected status 200, got %d", resp.StatusCode)
	}

	var health map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&health); err != nil {
		t.Fatalf("Failed to decode health response: %v", err)
	}

	if health["status"] != "ok" {
		t.Errorf("Expected status 'ok', got %v", health["status"])
	}
}

func TestValidateEndpoint_SafeContent(t *testing.T) {
	payload := map[string]interface{}{
		"contract_id": "test-service/general/us/v1.0",
		"payload": map[string]interface{}{
			"output": "This is safe content without any violations",
		},
	}

	body, _ := json.Marshal(payload)
	resp, err := http.Post(
		gatewayURL+"/api/validate",
		"application/json",
		bytes.NewReader(body),
	)
	if err != nil {
		t.Fatalf("Failed to call validate endpoint: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("Expected status 200, got %d", resp.StatusCode)
	}

	var result map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}

	if !result["valid"].(bool) {
		t.Error("Expected content to be valid")
	}

	violations, ok := result["violations"].([]interface{})
	if !ok || len(violations) > 0 {
		t.Errorf("Expected no violations, got %v", violations)
	}
}

func TestValidateEndpoint_PIIDetection(t *testing.T) {
	payload := map[string]interface{}{
		"contract_id": "test-service/healthcare/us/v1.0",
		"payload": map[string]interface{}{
			"output": "Patient John Doe, SSN: 123-45-6789, Phone: 555-1234",
		},
	}

	body, _ := json.Marshal(payload)
	resp, err := http.Post(
		gatewayURL+"/api/validate",
		"application/json",
		bytes.NewReader(body),
	)
	if err != nil {
		t.Fatalf("Failed to call validate endpoint: %v", err)
	}
	defer resp.Body.Close()

	var result map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}

	// Should detect PII
	features, ok := result["features"].(map[string]interface{})
	if !ok {
		t.Fatal("Expected features in response")
	}

	piiRisk, ok := features["pii_risk"].(float64)
	if !ok || piiRisk < 0.5 {
		t.Errorf("Expected high PII risk, got %v", piiRisk)
	}
}

func TestValidateEndpoint_MissingContract(t *testing.T) {
	payload := map[string]interface{}{
		"contract_id": "nonexistent-service/general/us/v1.0",
		"payload": map[string]interface{}{
			"output": "Some content",
		},
	}

	body, _ := json.Marshal(payload)
	resp, err := http.Post(
		gatewayURL+"/api/validate",
		"application/json",
		bytes.NewReader(body),
	)
	if err != nil {
		t.Fatalf("Failed to call validate endpoint: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusNotFound && resp.StatusCode != http.StatusBadRequest {
		t.Errorf("Expected status 404 or 400, got %d", resp.StatusCode)
	}
}

func TestValidateEndpoint_InvalidPayload(t *testing.T) {
	resp, err := http.Post(
		gatewayURL+"/api/validate",
		"application/json",
		bytes.NewReader([]byte("invalid json{")),
	)
	if err != nil {
		t.Fatalf("Failed to call validate endpoint: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("Expected status 400, got %d", resp.StatusCode)
	}
}

func TestAPIKeyAuthentication(t *testing.T) {
	// Test with valid API key
	req, _ := http.NewRequest("POST", gatewayURL+"/api/validate", nil)
	req.Header.Set("X-API-Key", os.Getenv("TEST_API_KEY"))
	req.Header.Set("X-Tenant-ID", "test-tenant")

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("Failed to make authenticated request: %v", err)
	}
	defer resp.Body.Close()

	// Should not be 401 with valid key
	if resp.StatusCode == http.StatusUnauthorized {
		t.Error("Valid API key was rejected")
	}
}

func TestMetricsEndpoint(t *testing.T) {
	resp, err := http.Get(gatewayURL + "/metrics")
	if err != nil {
		t.Fatalf("Failed to call metrics endpoint: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("Expected status 200, got %d", resp.StatusCode)
	}

	contentType := resp.Header.Get("Content-Type")
	if contentType != "text/plain; version=0.0.4; charset=utf-8" && 
	   contentType != "text/plain; version=0.0.4" {
		t.Errorf("Expected Prometheus content type, got %s", contentType)
	}
}

func TestConcurrentRequests(t *testing.T) {
	const numRequests = 50

	results := make(chan error, numRequests)

	for i := 0; i < numRequests; i++ {
		go func() {
			payload := map[string]interface{}{
				"contract_id": "test-service/general/us/v1.0",
				"payload": map[string]interface{}{
					"output": "Concurrent test message",
				},
			}

			body, _ := json.Marshal(payload)
			resp, err := http.Post(
				gatewayURL+"/api/validate",
				"application/json",
				bytes.NewReader(body),
			)
			if err != nil {
				results <- err
				return
			}
			resp.Body.Close()

			if resp.StatusCode != http.StatusOK {
				results <- http.ErrAbortHandler
				return
			}

			results <- nil
		}()
	}

	// Wait for all requests to complete
	for i := 0; i < numRequests; i++ {
		if err := <-results; err != nil {
			t.Errorf("Concurrent request failed: %v", err)
		}
	}
}

func BenchmarkValidateEndpoint(b *testing.B) {
	payload := map[string]interface{}{
		"contract_id": "test-service/general/us/v1.0",
		"payload": map[string]interface{}{
			"output": "Benchmark test message for performance testing",
		},
	}

	body, _ := json.Marshal(payload)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		resp, err := http.Post(
			gatewayURL+"/api/validate",
			"application/json",
			bytes.NewReader(body),
		)
		if err != nil {
			b.Fatal(err)
		}
		resp.Body.Close()
	}
}

