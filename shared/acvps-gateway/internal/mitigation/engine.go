package mitigation

import (
	"encoding/json"
	"regexp"
	"strings"

	"github.com/ethicalzen/acvps-gateway/internal/blockchain"
	"github.com/ethicalzen/acvps-gateway/internal/config"
	log "github.com/sirupsen/logrus"
)

// Engine handles failure mode mitigation
type Engine struct {
	config *config.Config
	
	// PII patterns
	ssnPattern         *regexp.Regexp
	creditCardPattern  *regexp.Regexp
	emailPattern       *regexp.Regexp
	phonePattern       *regexp.Regexp
}

// New creates a new mitigation engine
func New(cfg *config.Config) *Engine {
	return &Engine{
		config:            cfg,
		ssnPattern:        regexp.MustCompile(`\b\d{3}-\d{2}-\d{4}\b`),
		creditCardPattern: regexp.MustCompile(`\b\d{4}[- ]?\d{4}[- ]?\d{4}[- ]?\d{4}\b`),
		emailPattern:      regexp.MustCompile(`\b[A-Za-z0-9._%+-]+@[A-Za-z0-9.-]+\.[A-Z|a-z]{2,}\b`),
		phonePattern:      regexp.MustCompile(`\b\d{3}[-.]?\d{3}[-.]?\d{4}\b`),
	}
}

// RedactPII redacts personally identifiable information from response body
func (e *Engine) RedactPII(responseBody []byte, contract *blockchain.Contract) ([]byte, bool, error) {
	if !e.config.Mitigation.PIIRedaction.Enabled {
		return responseBody, false, nil
	}

	// Try to parse as JSON
	var data interface{}
	if err := json.Unmarshal(responseBody, &data); err != nil {
		// Not JSON, treat as plain text
		return e.redactPIIFromText(responseBody), true, nil
	}

	// Redact from JSON structure
	modified, redacted := e.redactPIIFromJSON(data)
	
	if redacted {
		result, err := json.Marshal(modified)
		if err != nil {
			return responseBody, false, err
		}
		return result, true, nil
	}

	return responseBody, false, nil
}

// redactPIIFromText redacts PII from plain text
func (e *Engine) redactPIIFromText(text []byte) []byte {
	str := string(text)
	
	// Redact SSN
	str = e.ssnPattern.ReplaceAllString(str, "[REDACTED-SSN]")
	
	// Redact credit cards
	str = e.creditCardPattern.ReplaceAllString(str, "[REDACTED-CC]")
	
	// Redact emails
	str = e.emailPattern.ReplaceAllString(str, "[REDACTED-EMAIL]")
	
	// Redact phone numbers
	str = e.phonePattern.ReplaceAllString(str, "[REDACTED-PHONE]")
	
	return []byte(str)
}

// redactPIIFromJSON redacts PII from JSON structure
func (e *Engine) redactPIIFromJSON(data interface{}) (interface{}, bool) {
	redacted := false
	
	switch v := data.(type) {
	case map[string]interface{}:
		for key, value := range v {
			// Check if key is in always_redact list
			shouldRedact := false
			lowerKey := strings.ToLower(key)
			for _, redactKey := range e.config.Mitigation.PIIRedaction.AlwaysRedact {
				if strings.ToLower(redactKey) == lowerKey {
					shouldRedact = true
					break
				}
			}
			
			if shouldRedact {
				v[key] = "[REDACTED]"
				redacted = true
			} else {
				// Recursively check nested structures
				modified, wasRedacted := e.redactPIIFromJSON(value)
				if wasRedacted {
					v[key] = modified
					redacted = true
				}
			}
		}
		return v, redacted
		
	case []interface{}:
		for i, item := range v {
			modified, wasRedacted := e.redactPIIFromJSON(item)
			if wasRedacted {
				v[i] = modified
				redacted = true
			}
		}
		return v, redacted
		
	case string:
		// Check for PII patterns in string values
		original := v
		v = e.ssnPattern.ReplaceAllString(v, "[REDACTED-SSN]")
		v = e.creditCardPattern.ReplaceAllString(v, "[REDACTED-CC]")
		v = e.emailPattern.ReplaceAllString(v, "[REDACTED-EMAIL]")
		v = e.phonePattern.ReplaceAllString(v, "[REDACTED-PHONE]")
		
		if v != original {
			redacted = true
		}
		return v, redacted
		
	default:
		return data, false
	}
}

// CheckGrounding validates grounding confidence in response
func (e *Engine) CheckGrounding(responseBody []byte) (bool, float64, error) {
	if !e.config.Mitigation.Grounding.Enabled {
		return true, 1.0, nil
	}

	// Try to parse response as JSON
	var data map[string]interface{}
	if err := json.Unmarshal(responseBody, &data); err != nil {
		// Can't parse, assume grounded
		return true, 1.0, nil
	}

	// Look for grounding metadata
	if grounding, ok := data["grounding"].(map[string]interface{}); ok {
		if confidence, ok := grounding["confidence"].(float64); ok {
			meets := confidence >= e.config.Mitigation.Grounding.MinConfidence
			
			if !meets {
				log.WithFields(log.Fields{
					"confidence": confidence,
					"threshold":  e.config.Mitigation.Grounding.MinConfidence,
				}).Warn("Grounding confidence below threshold")
			}
			
			return meets, confidence, nil
		}
	}

	// No grounding metadata found, assume grounded (for backward compatibility)
	return true, 1.0, nil
}

// ApplyMitigation applies all configured mitigations to response
func (e *Engine) ApplyMitigation(responseBody []byte, contract *blockchain.Contract) ([]byte, []string, error) {
	warnings := []string{}
	modified := responseBody
	
	// 1. Check grounding
	if e.config.Mitigation.Grounding.Enabled {
		groundingOK, confidence, err := e.CheckGrounding(modified)
		if err != nil {
			log.WithError(err).Warn("Grounding check failed")
		} else if !groundingOK {
			warning := "Low grounding confidence"
			warnings = append(warnings, warning)
			
			// Apply action based on config
			if e.config.Mitigation.Grounding.Action == "block" {
				return nil, warnings, &MitigationError{Message: warning}
			} else if e.config.Mitigation.Grounding.Action == "inject_notice" {
				modified = e.injectGroundingNotice(modified, confidence)
			}
		}
	}
	
	// 2. Redact PII
	if e.config.Mitigation.PIIRedaction.Enabled {
		var err error
		var redacted bool
		modified, redacted, err = e.RedactPII(modified, contract)
		if err != nil {
			log.WithError(err).Warn("PII redaction failed")
		} else if redacted {
			warnings = append(warnings, "PII redacted")
		}
	}
	
	return modified, warnings, nil
}

// injectGroundingNotice injects a warning notice into the response
func (e *Engine) injectGroundingNotice(responseBody []byte, confidence float64) []byte {
	// Try to parse as JSON and inject warning
	var data map[string]interface{}
	if err := json.Unmarshal(responseBody, &data); err != nil {
		// Not JSON, can't inject
		return responseBody
	}

	// Add warning field
	data["_warning"] = map[string]interface{}{
		"type":       "LOW_GROUNDING_CONFIDENCE",
		"confidence": confidence,
		"threshold":  e.config.Mitigation.Grounding.MinConfidence,
		"message":    "This response may not be fully grounded in source data",
	}

	result, err := json.Marshal(data)
	if err != nil {
		return responseBody
	}

	return result
}

// MitigationError represents a mitigation-triggered error
type MitigationError struct {
	Message string
}

func (e *MitigationError) Error() string {
	return e.Message
}

