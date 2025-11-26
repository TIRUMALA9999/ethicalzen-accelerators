package txrepo

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
)

// StreamContext provides context about the stream being validated
type StreamContext struct {
	Direction   string // "request" or "response"
	ContentType string // MIME type hint (optional)
}

// StreamGuardrailConfig defines a stream-based guardrail
type StreamGuardrailConfig struct {
	ID          string   `json:"id"`
	Name        string   `json:"name"`
	Description string   `json:"description"`
	Type        string   `json:"type"` // "stream_llm", "stream_probabilistic"
	MetricName  string   `json:"metric_name"`
	Threshold   float64  `json:"threshold"`

	// LLM configuration
	LLMPrompt   string `json:"llm_prompt"`
	LLMProvider string `json:"llm_provider"` // "openai", "groq"

	// Probabilistic configuration
	Keywords []string `json:"keywords"`
	Patterns []string `json:"patterns"`
}

// ProbabilisticStreamGuardrail analyzes raw byte stream probabilistically
// No JSON parsing, no structure assumptions - pure byte stream analysis
func ProbabilisticStreamGuardrail(stream []byte, ctx StreamContext, config *StreamGuardrailConfig) (MetricValues, float64, error) {
	// Convert bytes to string (handle any encoding)
	text := string(stream)

	fmt.Printf("[StreamGuardrail:%s] Analyzing %d bytes (%s)\n",
		config.ID, len(stream), ctx.Direction)

	// Check if we should use LLM
	apiKey := os.Getenv("OPENAI_API_KEY")
	if apiKey == "" {
		apiKey = os.Getenv("GROQ_API_KEY")
	}

	if apiKey != "" && config.Type == "stream_llm" {
		fmt.Printf("[StreamGuardrail:%s] Using LLM analysis\n", config.ID)
		return llmStreamAnalysis(text, config, apiKey)
	}

	// Fallback: Probabilistic pattern matching
	fmt.Printf("[StreamGuardrail:%s] Using probabilistic pattern matching\n", config.ID)
	return probabilisticPatternMatch(text, config)
}

// llmStreamAnalysis uses LLM for probabilistic analysis
func llmStreamAnalysis(text string, config *StreamGuardrailConfig, apiKey string) (MetricValues, float64, error) {
	// Prepare prompt
	prompt := fmt.Sprintf(`%s

Text to analyze (raw stream):
---
%s
---

Return JSON with:
{
  "risk_score": 0.0-1.0,
  "confidence": 0.0-1.0,
  "reasoning": "brief explanation"
}`, config.LLMPrompt, text)

	// Call LLM (OpenAI or Groq)
	response, err := callLLMForStream(prompt, config.LLMProvider, apiKey)
	if err != nil {
		fmt.Printf("[StreamLLM:%s] LLM call failed: %v, falling back to probabilistic\n", config.ID, err)
		return probabilisticPatternMatch(text, config)
	}

	// Parse response
	var result struct {
		RiskScore  float64 `json:"risk_score"`
		Confidence float64 `json:"confidence"`
		Reasoning  string  `json:"reasoning"`
	}

	if err := json.Unmarshal([]byte(response), &result); err != nil {
		fmt.Printf("[StreamLLM:%s] Failed to parse LLM response: %v\n", config.ID, err)
		return probabilisticPatternMatch(text, config)
	}

	fmt.Printf("[StreamLLM:%s] risk=%.2f, confidence=%.2f: %s\n",
		config.ID, result.RiskScore, result.Confidence, result.Reasoning)

	metrics := MetricValues{
		config.MetricName: result.RiskScore,
	}

	return metrics, result.Confidence, nil
}

// probabilisticPatternMatch uses statistical pattern matching
// This is format-agnostic - works on any byte stream
func probabilisticPatternMatch(text string, config *StreamGuardrailConfig) (MetricValues, float64, error) {
	textLower := strings.ToLower(text)
	textLen := len(text)

	if textLen == 0 {
		return MetricValues{config.MetricName: 0.0}, 1.0, nil
	}

	// Count keyword matches
	matches := 0
	matchPositions := []int{}

	for _, keyword := range config.Keywords {
		keywordLower := strings.ToLower(keyword)
		count := strings.Count(textLower, keywordLower)
		matches += count

		// Track positions for clustering analysis
		if count > 0 {
			idx := strings.Index(textLower, keywordLower)
			if idx >= 0 {
				matchPositions = append(matchPositions, idx)
			}
		}
	}

	// Calculate risk score (probabilistic)
	// Factors:
	// 1. Match density (matches per 1000 chars)
	// 2. Match clustering (are matches grouped together?)
	// 3. Text length (longer text = context matters more)

	matchDensity := float64(matches) / (float64(textLen) / 1000.0)

	// Clustering factor: if matches are close together, higher risk
	clusteringFactor := 1.0
	if len(matchPositions) > 1 {
		// Calculate average distance between matches
		totalDistance := 0
		for i := 1; i < len(matchPositions); i++ {
			totalDistance += matchPositions[i] - matchPositions[i-1]
		}
		avgDistance := float64(totalDistance) / float64(len(matchPositions)-1)

		// If matches are clustered (small distance), increase risk
		if avgDistance < 100 {
			clusteringFactor = 2.0
		} else if avgDistance < 500 {
			clusteringFactor = 1.5
		}
	}

	// Calculate risk score
	riskScore := matchDensity * clusteringFactor * 0.1
	if riskScore > 1.0 {
		riskScore = 1.0
	}

	// Confidence calculation:
	// - High confidence for clear signals (many matches or no matches)
	// - Low confidence for ambiguous cases (few matches in large text)
	confidence := 0.9
	if textLen > 10000 {
		confidence = 0.7 // Harder to analyze long streams
	} else if textLen > 50000 {
		confidence = 0.5 // Very long streams are uncertain
	}

	// If we have matches but not many, reduce confidence
	if matches > 0 && matches < 3 && textLen > 1000 {
		confidence = 0.6 // Ambiguous - need more context
	}

	fmt.Printf("[StreamProb:%s] len=%d, matches=%d, density=%.2f, cluster=%.1fx, risk=%.2f, conf=%.2f\n",
		config.ID, textLen, matches, matchDensity, clusteringFactor, riskScore, confidence)

	metrics := MetricValues{
		config.MetricName: riskScore,
	}

	return metrics, confidence, nil
}

// callLLMForStream calls LLM API for stream analysis
func callLLMForStream(prompt, provider, apiKey string) (string, error) {
	// TODO: Implement LLM calls similar to generic_llm.go
	// Support both OpenAI and Groq

	// For now, return error to trigger fallback
	return "", fmt.Errorf("LLM stream analysis not yet implemented - use probabilistic fallback")
}

