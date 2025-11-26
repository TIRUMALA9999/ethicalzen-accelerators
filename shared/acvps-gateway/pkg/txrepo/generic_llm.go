package txrepo

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"
)

// LLMResponse is the response from OpenAI API
type LLMResponse struct {
	ViolatesPolicy bool     `json:"violates_policy"`
	Confidence     float64  `json:"confidence"`
	Reasoning      string   `json:"reasoning"`
	Violations     []string `json:"violations"`
}

// MetricDefinition defines a single metric in the new schema
type MetricDefinition struct {
	Description string      `json:"description"`
	Type        string      `json:"type"`
	Range       []float64   `json:"range"`
	InvertScore bool        `json:"invert_score"`
	Threshold   *Threshold  `json:"threshold"`
}

// Threshold defines the acceptable range for a metric
type Threshold struct {
	Min    float64 `json:"min"`
	Max    float64 `json:"max"`
	Action string  `json:"action"`
}

// FeatureExtractor defines how to extract metrics from data
type FeatureExtractor struct {
	Type      string                 `json:"type"`
	Extractor map[string]interface{} `json:"extractor"`
}

// GuardrailConfig holds configuration for a dynamically registered guardrail
// Supports both legacy format (metric_name, keywords) and new schema (metrics, feature_extractors)
type GuardrailConfig struct {
	ID             string   `json:"id"`
	Name           string   `json:"name"`
	Description    string   `json:"description"`
	Version        string   `json:"version"`
	Category       string   `json:"category"`
	
	// Legacy format (backward compatibility)
	PromptTemplate string   `json:"prompt_template"` // Optional: custom LLM prompt
	Keywords       []string `json:"keywords"`        // Fallback pattern matching
	MetricName     string   `json:"metric_name"`     // Output metric (default: "compliance_score")
	Threshold      float64  `json:"threshold"`       // Min acceptable score
	InvertScore    bool     `json:"invert_score"`    // If true, high LLM confidence = low compliance
	
	// New schema format (GUARDRAIL_SCHEMA_V2)
	Metrics           map[string]MetricDefinition   `json:"metrics"`            // Metrics this guardrail extracts
	FeatureExtractors map[string]FeatureExtractor   `json:"feature_extractors"` // How to extract each metric
	
	RegisteredAt   string   `json:"registered_at"`
	Type           string   `json:"type"` // "dynamic" or "custom"
}

// GenericLLMGuardrail executes LLM-based validation with pattern fallback
func GenericLLMGuardrail(payload []byte, config *GuardrailConfig) (MetricValues, error) {
	text := extractTextFromJSON(payload)

	// SECURITY: Detect prompt injection attempts
	if isPromptInjection(text) {
		fmt.Printf("[GenericLLM:%s] SECURITY: Possible prompt injection detected, blocking\n", config.ID)
		// Return lowest score (violation detected)
		return blockingMetrics(config), nil
	}

	// Try LLM first if API key available
	apiKey := os.Getenv("OPENAI_API_KEY")
	if apiKey != "" {
		result, err := callLLM(text, config, apiKey)
		if err == nil {
			// SECURITY: Validate LLM response integrity
			if !validateLLMResponse(result) {
				fmt.Printf("[GenericLLM:%s] SECURITY: Invalid LLM response, using fallback\n", config.ID)
				return patternBasedCheck(text, config), nil
			}
			return convertLLMResult(result, config), nil
		}
		// Log error but continue to fallback
		fmt.Printf("[GenericLLM:%s] LLM failed: %v, using pattern fallback\n", config.ID, err)
	} else {
		fmt.Printf("[GenericLLM:%s] No OPENAI_API_KEY, using pattern fallback\n", config.ID)
	}

	// Fallback to pattern matching
	return patternBasedCheck(text, config), nil
}

// callLLM makes API call to OpenAI GPT-4
func callLLM(text string, config *GuardrailConfig, apiKey string) (*LLMResponse, error) {
	// SECURITY: Sanitize input before sending to LLM
	text = sanitizeInput(text)

	// Build prompt
	systemPrompt := config.PromptTemplate
	if systemPrompt == "" {
		systemPrompt = buildDefaultPrompt(config)
	}

	// Truncate text to avoid token limits
	maxLen := 3000
	if len(text) > maxLen {
		text = text[:maxLen]
	}

	// Prepare request
	reqBody := map[string]interface{}{
		"model": "gpt-4",
		"messages": []map[string]string{
			{"role": "system", "content": systemPrompt},
			{"role": "user", "content": fmt.Sprintf("Analyze this content:\n\n%s", text)},
		},
		"temperature": 0.1,
		"max_tokens":  400,
	}

	jsonBody, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	req, err := http.NewRequest("POST", "https://api.openai.com/v1/chat/completions", bytes.NewBuffer(jsonBody))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+apiKey)
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{Timeout: 15 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("API call failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("OpenAI API error %d: %s", resp.StatusCode, string(body))
	}

	// Parse response
	var apiResp struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&apiResp); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	if len(apiResp.Choices) == 0 {
		return nil, fmt.Errorf("no response from OpenAI")
	}

	// Parse LLM response (expecting JSON)
	var llmResp LLMResponse
	content := apiResp.Choices[0].Message.Content
	if err := json.Unmarshal([]byte(content), &llmResp); err != nil {
		return nil, fmt.Errorf("failed to parse LLM JSON response: %w (content: %s)", err, content)
	}

	return &llmResp, nil
}

// convertLLMResult converts LLM response to metrics
func convertLLMResult(result *LLMResponse, config *GuardrailConfig) MetricValues {
	metricName := config.MetricName
	if metricName == "" {
		metricName = "compliance_score"
	}

	var score float64
	if config.InvertScore {
		// High violation confidence = low compliance score
		// Example: 0.95 confidence of violation → 5% compliance
		score = (1 - result.Confidence) * 100
	} else {
		// High compliance confidence = high compliance score
		// Example: 0.95 confidence of compliance → 95% score
		score = result.Confidence * 100
	}

	return MetricValues{
		metricName: score,
	}
}

// patternBasedCheck fallback using keywords
func patternBasedCheck(text string, config *GuardrailConfig) MetricValues {
	lowerText := strings.ToLower(text)
	matches := 0

	keywords := config.Keywords
	if len(keywords) == 0 {
		// Default keywords if none provided
		keywords = []string{"inappropriate", "offensive", "illegal", "prohibited"}
	}

	fmt.Printf("[patternBasedCheck:%s] Checking text (len=%d, first 200 chars): %s...\n", config.ID, len(text), text[:min(200, len(text))])
	fmt.Printf("[patternBasedCheck:%s] Keywords to check: %v\n", config.ID, keywords)

	for _, keyword := range keywords {
		count := strings.Count(lowerText, strings.ToLower(keyword))
		if count > 0 {
			fmt.Printf("[patternBasedCheck] Keyword '%s' found %d times\n", keyword, count)
		}
		matches += count
	}

	// Calculate score normalized to 0.0-1.0 range
	// The score calculation depends on invert_score:
	// - invert_score=false (RISK metrics): 0=safe, 1=risk detected
	//   → matches found = higher score (more risk)
	// - invert_score=true (COMPLIANCE metrics): 0=violation, 1=compliant
	//   → matches found = lower score (less compliant)
	
	var score float64
	if config.InvertScore {
		// COMPLIANCE: matches = bad, no matches = good (score 1.0)
		score = 1.0 - (float64(matches) * 0.15)
		if score < 0.0 {
			score = 0.0
		}
	} else {
		// RISK: matches = bad (score 1.0), no matches = good (score 0.0)
		score = float64(matches) * 0.2
		if score > 1.0 {
			score = 1.0
		}
	}

	fmt.Printf("[patternBasedCheck:%s] text_len=%d, matches=%d, invert=%v, score=%.2f\n", 
		config.ID, len(text), matches, config.InvertScore, score)

	metricName := config.MetricName
	if metricName == "" {
		metricName = "compliance_score"
	}

	return MetricValues{
		metricName: score,
	}
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// buildDefaultPrompt creates a prompt from description
func buildDefaultPrompt(config *GuardrailConfig) string {
	return fmt.Sprintf(`You are a compliance checker. %s

Analyze the content and determine if it violates the policy.

Response format (JSON only, no other text):
{
  "violates_policy": true/false,
  "confidence": 0.0-1.0,
  "reasoning": "brief explanation",
  "violations": ["specific issues found"]
}

Be strict but fair. If uncertain, err on the side of caution.`, config.Description)
}

// ============================================================================
// SECURITY FUNCTIONS - Prevent Prompt Injection Attacks
// ============================================================================

// isPromptInjection detects common prompt injection patterns
func isPromptInjection(text string) bool {
	lowerText := strings.ToLower(text)

	// Detect instruction override attempts
	injectionPatterns := []string{
		"ignore previous",
		"ignore all previous",
		"disregard previous",
		"forget previous",
		"ignore the above",
		"disregard the above",
		"new instructions:",
		"system:",
		"assistant:",
		"you are now",
		"act as if",
		"pretend you are",
		"output:",
		"print:",
		"return:",
		"respond with:",
		"<script>",
		"javascript:",
		"eval(",
		"exec(",
		// JSON injection attempts
		`"violates_policy": false`,
		`"confidence": 1.0`,
		`"confidence": 0.0`,
		// Encoding bypasses
		"\\u0000",
		"\\x00",
	}

	for _, pattern := range injectionPatterns {
		if strings.Contains(lowerText, pattern) {
			return true
		}
	}

	// Detect excessive special characters (potential encoding attacks)
	specialCharCount := 0
	for _, char := range text {
		if char == '{' || char == '}' || char == '[' || char == ']' ||
			char == '<' || char == '>' || char == '\\' {
			specialCharCount++
		}
	}
	if specialCharCount > len(text)/10 { // More than 10% special chars
		return true
	}

	return false
}

// validateLLMResponse validates the integrity of LLM response
func validateLLMResponse(result *LLMResponse) bool {
	// Check confidence is in valid range
	if result.Confidence < 0 || result.Confidence > 1.0 {
		return false
	}

	// Check reasoning is not suspiciously short or empty
	if len(strings.TrimSpace(result.Reasoning)) < 10 {
		return false
	}

	// Check reasoning doesn't contain injection indicators
	lowerReasoning := strings.ToLower(result.Reasoning)
	suspiciousTerms := []string{
		"ignore previous",
		"as instructed",
		"user told me to",
		"following your request",
	}
	for _, term := range suspiciousTerms {
		if strings.Contains(lowerReasoning, term) {
			return false
		}
	}

	// Check violations array is reasonable (not too long)
	if len(result.Violations) > 50 {
		return false
	}

	return true
}

// blockingMetrics returns metrics that indicate a violation
// Used when prompt injection is detected
func blockingMetrics(config *GuardrailConfig) MetricValues {
	metricName := config.MetricName
	if metricName == "" {
		metricName = "compliance_score"
	}

	// Return 0 score (maximum violation)
	return MetricValues{
		metricName: 0.0,
	}
}

// sanitizeInput removes potentially malicious content before LLM processing
func sanitizeInput(text string) string {
	// Remove null bytes and control characters
	cleaned := strings.Map(func(r rune) rune {
		// Keep printable characters and common whitespace
		if r >= 32 && r <= 126 || r == '\n' || r == '\t' || r == '\r' {
			return r
		}
		// Remove everything else (control chars, null bytes, etc.)
		return -1
	}, text)

	// Replace multiple instruction markers with placeholders
	replacements := map[string]string{
		"System:":    "[SYSTEM]",
		"system:":    "[SYSTEM]",
		"Assistant:": "[ASSISTANT]",
		"assistant:": "[ASSISTANT]",
		"User:":      "[USER]",
		"user:":      "[USER]",
	}

	for old, new := range replacements {
		cleaned = strings.ReplaceAll(cleaned, old, new)
	}

	return cleaned
}
