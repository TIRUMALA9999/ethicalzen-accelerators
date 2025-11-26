package txrepo

import (
	"math"
	"regexp"
	"strings"
)

// Guardrail Functions: Safety validators for AI responses
// These functions calculate safety metrics from AI response payloads
// Terminology:
//   - Guardrail: A safety mechanism (e.g., "PII Detection")
//   - Metric: A quantitative measurement (e.g., "pii_risk": 0.02)
//   - Threshold: Acceptable range for a metric (defined in contract)

// PII Detection Patterns
var (
	ssnRegex        = regexp.MustCompile(`\b\d{3}-\d{2}-\d{4}\b`)
	emailRegex      = regexp.MustCompile(`\b[A-Za-z0-9._%+-]+@[A-Za-z0-9.-]+\.[A-Z|a-z]{2,}\b`)
	phoneRegex      = regexp.MustCompile(`\b(?:\+?1[-.]?)?\(?([0-9]{3})\)?[-.]?([0-9]{3})[-.]?([0-9]{4})\b`)
	creditCardRegex = regexp.MustCompile(`\b\d{4}[-\s]?\d{4}[-\s]?\d{4}[-\s]?\d{4}\b`)
	zipCodeRegex    = regexp.MustCompile(`\b\d{5}(?:-\d{4})?\b`)
)

// Citation Patterns
var (
	numberedCitationRegex      = regexp.MustCompile(`\[\d+\]`)
	parentheticalCitationRegex = regexp.MustCompile(`\([A-Za-z]+\s+\d{4}\)`)
	urlRegex                   = regexp.MustCompile(`https?://[^\s]+`)
	sourceKeywordRegex         = regexp.MustCompile(`(?i)source:|reference:|cited from:`)
	sentenceRegex              = regexp.MustCompile(`[.!?]+[\s\n]`)
)

// Vague words indicating potential hallucination
var vagueWords = []string{
	"might", "possibly", "perhaps", "maybe", "unclear",
	"uncertain", "could be", "may be", "seems like",
	"appears to", "probably", "likely",
	"i think", "i believe", "not sure",
	"hard to say", "difficult to determine",
}

// Specific fact patterns
var (
	numberPattern      = regexp.MustCompile(`\b\d+(?:\.\d+)?(?:%|kg|lb|cm|mm|ml|mg)?\b`)
	datePattern        = regexp.MustCompile(`\b\d{1,2}[/-]\d{1,2}[/-]\d{2,4}\b|\b(?:Jan|Feb|Mar|Apr|May|Jun|Jul|Aug|Sep|Oct|Nov|Dec)[a-z]*\s+\d{1,2},?\s+\d{4}\b`)
	properNounPattern  = regexp.MustCompile(`\b[A-Z][a-z]+(?:\s+[A-Z][a-z]+)*\b`)
	measurementPattern = regexp.MustCompile(`\b\d+(?:\.\d+)?\s*(?:mg|ml|kg|lb|cm|mm|ft|in|m|km|mi)\b`)
)

// PIIDetectorV1 calculates PII risk metrics from AI response payloads
// This is a combined guardrail that calculates multiple metrics:
//   - pii_risk: 0.0-1.0 (higher = more PII detected)
//   - grounding_confidence: 0.0-1.0 (higher = better citations)
//   - hallucination_risk: 0.0-1.0 (higher = more vague language)
func PIIDetectorV1(payload []byte) (MetricValues, error) {
	text := extractTextFromJSON(payload)

	// ===== PII Detection =====
	ssnCount := countPattern(text, ssnRegex)
	emailCount := countPattern(text, emailRegex)
	phoneCount := countPattern(text, phoneRegex)
	creditCardCount := countPattern(text, creditCardRegex)
	zipCodeCount := countPattern(text, zipCodeRegex)

	totalPII := ssnCount + emailCount + phoneCount + creditCardCount + zipCodeCount
	piiRisk := math.Min(float64(totalPII)/5.0, 1.0)

	// ===== Grounding Analysis =====
	numberedCitations := countPattern(text, numberedCitationRegex)
	parentheticalCitations := countPattern(text, parentheticalCitationRegex)
	urls := countPattern(text, urlRegex)
	sourceKeywords := countPattern(text, sourceKeywordRegex)
	totalCitations := numberedCitations + parentheticalCitations + urls + sourceKeywords

	sentences := countPattern(text, sentenceRegex)
	if sentences == 0 {
		sentences = 1
	}
	groundingConfidence := math.Min(float64(totalCitations)/float64(sentences), 1.0)

	// ===== Hallucination Detection =====
	lowerText := strings.ToLower(text)
	vagueCount := 0
	for _, word := range vagueWords {
		vagueCount += strings.Count(lowerText, word)
	}

	specificCount := 0
	specificCount += countPattern(text, numberPattern)
	specificCount += countPattern(text, datePattern)
	specificCount += countPattern(text, properNounPattern)
	specificCount += countPattern(text, measurementPattern)

	total := vagueCount + specificCount
	var hallucinationRisk float64
	if total > 0 {
		hallucinationRisk = float64(vagueCount) / float64(total)
	} else {
		hallucinationRisk = 0.5
	}

	return MetricValues{
		"pii_risk":             piiRisk,
		"grounding_confidence": groundingConfidence,
		"hallucination_risk":   hallucinationRisk,
	}, nil
}

// GroundingAnalyzerV1 calculates citation grounding metrics
// Metric: grounding_confidence (0.0-1.0, higher = better citations)
func GroundingAnalyzerV1(payload []byte) (MetricValues, error) {
	text := extractTextFromJSON(payload)

	// Count citations
	numberedCitations := countPattern(text, numberedCitationRegex)
	parentheticalCitations := countPattern(text, parentheticalCitationRegex)
	urls := countPattern(text, urlRegex)
	sourceKeywords := countPattern(text, sourceKeywordRegex)

	totalCitations := numberedCitations + parentheticalCitations + urls + sourceKeywords

	// Count sentences
	sentences := countPattern(text, sentenceRegex)
	if sentences == 0 {
		sentences = 1 // Avoid division by zero
	}

	// Calculate confidence [0.0-1.0]
	// Higher citation density = higher confidence
	// Cap at 1.0 (e.g., 1 citation per sentence = perfect grounding)
	confidence := math.Min(float64(totalCitations)/float64(sentences), 1.0)

	return MetricValues{
		"grounding_confidence": confidence,
	}, nil
}

// HallucinationDetectorV1 calculates hallucination risk metrics
// Metric: hallucination_risk (0.0-1.0, higher = more vague/uncertain language)
func HallucinationDetectorV1(payload []byte) (MetricValues, error) {
	text := extractTextFromJSON(payload)
	lowerText := strings.ToLower(text)

	// Count vague words
	vagueCount := 0
	for _, word := range vagueWords {
		vagueCount += strings.Count(lowerText, word)
	}

	// Count specific facts
	specificCount := 0
	specificCount += countPattern(text, numberPattern)
	specificCount += countPattern(text, datePattern)
	specificCount += countPattern(text, properNounPattern)
	specificCount += countPattern(text, measurementPattern)

	// Calculate hallucination risk [0.0-1.0]
	// High vague/low specific = high risk
	// Low vague/high specific = low risk
	total := vagueCount + specificCount
	var risk float64
	if total > 0 {
		risk = float64(vagueCount) / float64(total)
	} else {
		risk = 0.5 // Default to medium risk if no indicators
	}

	return MetricValues{
		"hallucination_risk": risk,
	}, nil
}

// countPattern counts regex pattern matches in text
func countPattern(text string, pattern *regexp.Regexp) int {
	matches := pattern.FindAllString(text, -1)
	return len(matches)
}
