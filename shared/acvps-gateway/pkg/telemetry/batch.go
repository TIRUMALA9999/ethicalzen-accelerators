package telemetry

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"sync"
	"time"
)

// TelemetryBatch holds telemetry events to be sent in batches
type TelemetryBatch struct {
	Requests   []RequestEvent   `json:"requests"`
	Violations []ViolationEvent `json:"violations"`
}

// RequestEvent represents a single request telemetry event
type RequestEvent struct {
	Timestamp         string `json:"timestamp"`
	TenantID          string `json:"tenant_id"`
	TraceID           string `json:"trace_id"`
	ContractID        string `json:"contract_id"`
	CertificateID     string `json:"certificate_id"`
	Method            string `json:"method"`
	Path              string `json:"path"`
	StatusCode        int    `json:"status_code"`
	ResponseTimeMs    int64  `json:"response_time_ms"`
	RequestSizeBytes  int64  `json:"request_size_bytes"`
	ResponseSizeBytes int64  `json:"response_size_bytes"`
	IPAddress         string `json:"ip_address,omitempty"`
	UserAgent         string `json:"user_agent,omitempty"`
}

// ViolationEvent represents a single violation telemetry event
type ViolationEvent struct {
	Timestamp     string  `json:"timestamp"`
	TenantID      string  `json:"tenant_id"`
	TraceID       string  `json:"trace_id"`
	ContractID    string  `json:"contract_id"`
	CertificateID string  `json:"certificate_id"`
	ViolationType string  `json:"violation_type"`
	MetricName    string  `json:"metric_name"`
	MetricValue   float64 `json:"metric_value"`
	ThresholdMin  float64 `json:"threshold_min,omitempty"`
	ThresholdMax  float64 `json:"threshold_max,omitempty"`
	Severity      string  `json:"severity"`
	Details       string  `json:"details,omitempty"`
}

// BatchCollector collects telemetry events and sends them in batches
type BatchCollector struct {
	metricsServiceURL string
	apiKey            string
	batchSize         int
	batchInterval     time.Duration
	maxBufferSize     int

	mu         sync.Mutex
	requests   []RequestEvent
	violations []ViolationEvent

	enabled bool
}

var (
	defaultCollector *BatchCollector
	once             sync.Once
)

// InitCollector initializes the global telemetry collector
func InitCollector() *BatchCollector {
	once.Do(func() {
		metricsURL := os.Getenv("METRICS_SERVICE_URL")
		if metricsURL == "" {
			metricsURL = "http://localhost:8090"
		}

		enabled := os.Getenv("METRICS_ENABLED") != "false"

		batchSize := 100 // Default
		if bs := os.Getenv("METRICS_BATCH_SIZE"); bs != "" {
			fmt.Sscanf(bs, "%d", &batchSize)
		}

		batchInterval := 5 * time.Second // Default
		if bi := os.Getenv("METRICS_BATCH_INTERVAL"); bi != "" {
			if d, err := time.ParseDuration(bi); err == nil {
				batchInterval = d
			}
		}

		maxBufferSize := 1000 // Default
		if mbs := os.Getenv("METRICS_BUFFER_SIZE"); mbs != "" {
			fmt.Sscanf(mbs, "%d", &maxBufferSize)
		}

		defaultCollector = &BatchCollector{
			metricsServiceURL: metricsURL,
			apiKey:            os.Getenv("METRICS_API_KEY"),
			batchSize:         batchSize,
			batchInterval:     batchInterval,
			maxBufferSize:     maxBufferSize,
			requests:          make([]RequestEvent, 0, batchSize),
			violations:        make([]ViolationEvent, 0, batchSize),
			enabled:           enabled,
		}

		if enabled {
			fmt.Printf("📊 Telemetry Collector initialized: %s (batch: %d events or %v)\n",
				metricsURL, batchSize, batchInterval)

			// Start background batch sender
			go defaultCollector.batchSender()
		} else {
			fmt.Println("📊 Telemetry disabled via METRICS_ENABLED=false")
		}
	})

	return defaultCollector
}

// GetCollector returns the global telemetry collector
func GetCollector() *BatchCollector {
	if defaultCollector == nil {
		return InitCollector()
	}
	return defaultCollector
}

// AddRequest adds a request event to the batch
func (c *BatchCollector) AddRequest(event RequestEvent) {
	if !c.enabled {
		return
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	// Drop oldest if buffer full (backpressure protection)
	if len(c.requests) >= c.maxBufferSize {
		c.requests = c.requests[1:]
	}

	c.requests = append(c.requests, event)

	// Trigger immediate send if batch size reached
	if len(c.requests) >= c.batchSize {
		go c.flush()
	}
}

// AddViolation adds a violation event to the batch
func (c *BatchCollector) AddViolation(event ViolationEvent) {
	if !c.enabled {
		return
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	// Drop oldest if buffer full
	if len(c.violations) >= c.maxBufferSize {
		c.violations = c.violations[1:]
	}

	c.violations = append(c.violations, event)

	// Trigger immediate send if batch size reached
	if len(c.violations) >= c.batchSize {
		go c.flush()
	}
}

// batchSender runs in background and sends batches at regular intervals
func (c *BatchCollector) batchSender() {
	ticker := time.NewTicker(c.batchInterval)
	defer ticker.Stop()

	for range ticker.C {
		c.flush()
	}
}

// flush sends the current batch to metrics service
func (c *BatchCollector) flush() {
	c.mu.Lock()

	// Nothing to send
	if len(c.requests) == 0 && len(c.violations) == 0 {
		c.mu.Unlock()
		return
	}

	// Create batch
	batch := TelemetryBatch{
		Requests:   c.requests,
		Violations: c.violations,
	}

	// Reset buffers
	c.requests = make([]RequestEvent, 0, c.batchSize)
	c.violations = make([]ViolationEvent, 0, c.batchSize)

	c.mu.Unlock()

	// Send batch (non-blocking, fail-open)
	go c.sendBatch(batch)
}

// sendBatch sends a batch to the metrics service
func (c *BatchCollector) sendBatch(batch TelemetryBatch) {
	payload, err := json.Marshal(batch)
	if err != nil {
		fmt.Printf("⚠️  Failed to marshal telemetry batch: %v\n", err)
		return
	}

	req, err := http.NewRequest("POST", c.metricsServiceURL+"/ingest/batch", bytes.NewReader(payload))
	if err != nil {
		fmt.Printf("⚠️  Failed to create telemetry request: %v\n", err)
		return
	}

	req.Header.Set("Content-Type", "application/json")
	if c.apiKey != "" {
		req.Header.Set("X-API-Key", c.apiKey)
	}

	client := &http.Client{Timeout: 5 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		// Silent failure - telemetry should never block gateway
		fmt.Printf("⚠️  Telemetry send failed (metrics service down?): %v\n", err)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		fmt.Printf("⚠️  Telemetry send failed with status %d\n", resp.StatusCode)
		return
	}

	// Success - no logging to avoid spam
}

// Flush forces an immediate flush of buffered events
func (c *BatchCollector) Flush() {
	c.flush()
}

