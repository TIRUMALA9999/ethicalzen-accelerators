package txrepo

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
	"sync"
)

// MetricValues is a map of metric names to float64 values
// Metrics are quantitative measurements calculated by guardrails
// Examples: pii_risk: 0.02, grounding_confidence: 0.87, hallucination_risk: 0.03
type MetricValues map[string]float64

// FeatureVector (legacy) - kept for backward compatibility
type FeatureVector = MetricValues

// GuardrailFunc is a pure function that calculates metrics from AI response payload
// Guardrails are safety mechanisms that analyze responses for compliance
type GuardrailFunc func(payload []byte) (MetricValues, error)

// FeatureExtractorFunc (legacy) - kept for backward compatibility
type FeatureExtractorFunc = GuardrailFunc

// GuardrailMetadata holds metadata about a guardrail
type GuardrailMetadata struct {
	ID          string   // e.g., "pii_detection_v1"
	Name        string   // e.g., "PII Detection"
	Version     string   // e.g., "1.0.0"
	Description string   // Human-readable description
	SourceHash  string   // SHA-256 of normalized source code
	Metrics     []string // Metrics calculated by this guardrail
}

// ExtractorMetadata (legacy) - kept for backward compatibility
type ExtractorMetadata struct {
	ID          string // e.g., "pii_detector_v1"
	Version     string // e.g., "1.0.0"
	Description string
	SourceHash  string // SHA-256 of normalized source code
}

// Registry holds all guardrails (safety validation functions)
// Legacy name: "extractors" - but they actually validate safety, not extract features
type Registry struct {
	extractors map[string]GuardrailFunc     // Guardrail functions (legacy name)
	metadata   map[string]ExtractorMetadata // Legacy metadata format
	mu         sync.RWMutex
}

// GlobalRegistry is the global extractor registry instance
var GlobalRegistry *Registry

func init() {
	GlobalRegistry = NewRegistry()

	// Register built-in extractors
	GlobalRegistry.Register("pii_detector_v1", PIIDetectorV1, ExtractorMetadata{
		ID:          "pii_detector_v1",
		Version:     "1.0.0",
		Description: "Detects PII (SSN, email, phone) in responses",
		SourceHash:  ComputeHash(piiDetectorV1Source),
	})

	GlobalRegistry.Register("grounding_analyzer_v1", GroundingAnalyzerV1, ExtractorMetadata{
		ID:          "grounding_analyzer_v1",
		Version:     "1.0.0",
		Description: "Analyzes grounding/citation quality",
		SourceHash:  ComputeHash(groundingAnalyzerV1Source),
	})

	GlobalRegistry.Register("hallucination_detector_v1", HallucinationDetectorV1, ExtractorMetadata{
		ID:          "hallucination_detector_v1",
		Version:     "1.0.0",
		Description: "Detects likely hallucinations in AI responses",
		SourceHash:  ComputeHash(hallucinationDetectorV1Source),
	})
}

// NewRegistry creates a new extractor registry
func NewRegistry() *Registry {
	return &Registry{
		extractors: make(map[string]FeatureExtractorFunc),
		metadata:   make(map[string]ExtractorMetadata),
	}
}

// Register adds an extractor to the registry
func (r *Registry) Register(id string, fn FeatureExtractorFunc, meta ExtractorMetadata) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if _, exists := r.extractors[id]; exists {
		return fmt.Errorf("extractor already registered: %s", id)
	}

	r.extractors[id] = fn
	r.metadata[id] = meta

	return nil
}

// Get returns an extractor by ID
func (r *Registry) Get(id string) (FeatureExtractorFunc, ExtractorMetadata, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	fn, fnExists := r.extractors[id]
	meta, metaExists := r.metadata[id]

	if !fnExists || !metaExists {
		return nil, ExtractorMetadata{}, fmt.Errorf("extractor not found: %s", id)
	}

	return fn, meta, nil
}

// List returns all registered extractor IDs
func (r *Registry) List() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()

	ids := make([]string, 0, len(r.extractors))
	for id := range r.extractors {
		ids = append(ids, id)
	}

	return ids
}

// ComputeHash computes SHA-256 of normalized source code
func ComputeHash(sourceCode string) string {
	// Normalize the source code
	normalized := normalizeSource(sourceCode)

	// Compute SHA-256
	hash := sha256.Sum256([]byte(normalized))

	// Return as hex string
	return hex.EncodeToString(hash[:])
}

// normalizeSource normalizes source code for consistent hashing
func normalizeSource(source string) string {
	// Convert to lowercase
	normalized := strings.ToLower(source)

	// Remove comments (single-line and multi-line)
	commentRegex := regexp.MustCompile(`//.*?$|/\*.*?\*/`)
	normalized = commentRegex.ReplaceAllString(normalized, "")

	// Normalize whitespace (replace multiple spaces/tabs/newlines with single space)
	whitespaceRegex := regexp.MustCompile(`\s+`)
	normalized = whitespaceRegex.ReplaceAllString(normalized, " ")

	// Trim leading/trailing whitespace
	normalized = strings.TrimSpace(normalized)

	return normalized
}

// Helper: extract text from JSON payload
func extractTextFromJSON(payload []byte) string {
	var data interface{}
	if err := json.Unmarshal(payload, &data); err != nil {
		// If not JSON, return as string
		return string(payload)
	}

	// Recursively extract all string values
	return extractStringsRecursive(data)
}

func extractStringsRecursive(data interface{}) string {
	var result strings.Builder

	switch v := data.(type) {
	case map[string]interface{}:
		for key, val := range v {
			// Include relevant key names to help with pattern matching
			lowerKey := strings.ToLower(key)
			if lowerKey == "source" || lowerKey == "sources" ||
				lowerKey == "reference" || lowerKey == "references" ||
				lowerKey == "citation" || lowerKey == "citations" {
				result.WriteString(key)
				result.WriteString(": ")
			}

			text := extractStringsRecursive(val)
			if text != "" {
				result.WriteString(text)
				result.WriteString(" ")
			}
		}
	case []interface{}:
		for _, val := range v {
			text := extractStringsRecursive(val)
			if text != "" {
				result.WriteString(text)
				result.WriteString(" ")
			}
		}
	case string:
		result.WriteString(v)
	}

	return result.String()
}

// Source code strings for hashing (simplified representations)
const piiDetectorV1Source = `
func PIIDetectorV1(payload []byte) (FeatureVector, error) {
	text := extractTextFromJSON(payload)
	ssnCount := countPattern(text, ssnRegex)
	emailCount := countPattern(text, emailRegex)
	phoneCount := countPattern(text, phoneRegex)
	totalPII := ssnCount + emailCount + phoneCount
	risk := math.Min(float64(totalPII) / 5.0, 1.0)
	return FeatureVector{"pii_risk": risk}, nil
}
`

const groundingAnalyzerV1Source = `
func GroundingAnalyzerV1(payload []byte) (FeatureVector, error) {
	text := extractTextFromJSON(payload)
	citations := countPattern(text, citationRegex)
	sentences := countSentences(text)
	confidence := math.Min(float64(citations) / float64(sentences), 1.0)
	return FeatureVector{"grounding_confidence": confidence}, nil
}
`

const hallucinationDetectorV1Source = `
func HallucinationDetectorV1(payload []byte) (FeatureVector, error) {
	text := extractTextFromJSON(payload)
	vagueCount := countVagueWords(text)
	specificCount := countSpecificFacts(text)
	risk := float64(vagueCount) / float64(vagueCount + specificCount)
	return FeatureVector{"hallucination_risk": risk}, nil
}
`
