# 🛡️ Dynamic Guardrail System for Go ACVPS Gateway

**Design Document for Generic LLM-Assisted Guardrails with Override Capability**

---

## 🎯 **OBJECTIVE**

Enable customers to:
1. **Add guardrails via Dashboard** (no code deployment)
2. **Use generic LLM template** (works out-of-box with just a prompt)
3. **Override with custom Go code** (for performance/accuracy)

---

## 📐 **ARCHITECTURE**

```
Dashboard
    │
    │ POST /api/guardrails/add
    │ {id, name, description, prompt, keywords, threshold}
    │
    ▼
Portal Backend (Node.js)
    │
    │ POST {gateway_url}/guardrails/register
    │ {config}
    │
    ▼
ACVPS Gateway (Go)
    │
    ├─> Dynamic Registry
    │   ├─> Check: Custom implementation exists?
    │   │   YES ──> Use custom Go code (fast, accurate)
    │   │   NO  ──> Use generic LLM template (flexible)
    │   │
    │   └─> Runtime: Contract specifies guardrail_id
    │       ├─> Extract metrics using guardrail
    │       ├─> Validate against envelope
    │       └─> Block/Allow request
    │
    └─> Response
```

---

## 📁 **FILE STRUCTURE**

```
acvps-gateway/
├── pkg/txrepo/
│   ├── extractors.go          # Built-in guardrails (PII, grounding, etc.)
│   ├── txrepo.go              # Registry (static)
│   ├── dynamic_registry.go    # NEW: Dynamic registration
│   ├── generic_llm.go         # NEW: Generic LLM template
│   └── smoking_checker.go     # NEW: Example custom override
│
├── internal/api/
│   └── guardrails.go          # NEW: API handlers for registration
│
└── cmd/gateway/
    └── main.go                # Mount /guardrails/* routes
```

---

## 🔧 **IMPLEMENTATION**

### **1. Dynamic Registry (`pkg/txrepo/dynamic_registry.go`)**

```go
package txrepo

import (
    "fmt"
    "sync"
)

// GuardrailConfig holds configuration for a dynamically registered guardrail
type GuardrailConfig struct {
    ID             string   `json:"id"`
    Name           string   `json:"name"`
    Description    string   `json:"description"`
    PromptTemplate string   `json:"prompt_template"` // LLM prompt
    Keywords       []string `json:"keywords"`        // Fallback pattern matching
    MetricName     string   `json:"metric_name"`     // Output metric (default: "compliance_score")
    Threshold      float64  `json:"threshold"`       // Min acceptable score (default: 75)
    InvertScore    bool     `json:"invert_score"`    // If true, high LLM confidence = low compliance
    RegisteredAt   string   `json:"registered_at"`
    Type           string   `json:"type"`            // "dynamic" or "custom"
}

// DynamicRegistry manages runtime-registered guardrails
type DynamicRegistry struct {
    configs     map[string]*GuardrailConfig
    extractors  map[string]GuardrailFunc  // Override functions
    mu          sync.RWMutex
}

var dynamicRegistry *DynamicRegistry

func init() {
    dynamicRegistry = &DynamicRegistry{
        configs:    make(map[string]*GuardrailConfig),
        extractors: make(map[string]GuardrailFunc),
    }
}

// RegisterConfig registers a guardrail configuration
func RegisterConfig(config *GuardrailConfig) error {
    dynamicRegistry.mu.Lock()
    defer dynamicRegistry.mu.Unlock()
    
    if config.ID == "" {
        return fmt.Errorf("guardrail ID cannot be empty")
    }
    
    dynamicRegistry.configs[config.ID] = config
    return nil
}

// RegisterCustom registers a custom Go implementation (override)
func RegisterCustom(id string, fn GuardrailFunc) error {
    dynamicRegistry.mu.Lock()
    defer dynamicRegistry.mu.Unlock()
    
    dynamicRegistry.extractors[id] = fn
    return nil
}

// GetGuardrail returns guardrail func (custom > generic LLM > static > nil)
func GetGuardrail(id string) (GuardrailFunc, *GuardrailConfig, error) {
    dynamicRegistry.mu.RLock()
    defer dynamicRegistry.mu.RUnlock()
    
    // Priority 1: Custom Go implementation (best performance)
    if customFn, exists := dynamicRegistry.extractors[id]; exists {
        config := dynamicRegistry.configs[id]
        return customFn, config, nil
    }
    
    // Priority 2: Generic LLM template (flexibility)
    if config, exists := dynamicRegistry.configs[id]; exists {
        // Return a closure that calls GenericLLMGuardrail with config
        fn := func(payload []byte) (MetricValues, error) {
            return GenericLLMGuardrail(payload, config)
        }
        return fn, config, nil
    }
    
    // Priority 3: Static built-in guardrails
    staticFn, staticMeta, err := GlobalRegistry.Get(id)
    if err == nil {
        // Convert ExtractorMetadata to GuardrailConfig
        config := &GuardrailConfig{
            ID:          staticMeta.ID,
            Name:        staticMeta.ID,
            Description: staticMeta.Description,
            Type:        "static",
        }
        return staticFn, config, nil
    }
    
    // Not found
    return nil, nil, fmt.Errorf("guardrail not found: %s", id)
}

// ListAll returns all guardrails (static + dynamic)
func ListAll() []string {
    dynamicRegistry.mu.RLock()
    defer dynamicRegistry.mu.RUnlock()
    
    // Combine static and dynamic
    staticIDs := GlobalRegistry.List()
    
    allIDs := make(map[string]bool)
    for _, id := range staticIDs {
        allIDs[id] = true
    }
    for id := range dynamicRegistry.configs {
        allIDs[id] = true
    }
    
    result := make([]string, 0, len(allIDs))
    for id := range allIDs {
        result = append(result, id)
    }
    
    return result
}
```

---

### **2. Generic LLM Template (`pkg/txrepo/generic_llm.go`)**

```go
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

// GenericLLMGuardrail executes LLM-based validation with pattern fallback
func GenericLLMGuardrail(payload []byte, config *GuardrailConfig) (MetricValues, error) {
    text := extractTextFromJSON(payload)
    
    // Try LLM first
    apiKey := os.Getenv("OPENAI_API_KEY")
    if apiKey != "" {
        result, err := callLLM(text, config, apiKey)
        if err == nil {
            return convertLLMResult(result, config), nil
        }
        // Log error but continue to fallback
        fmt.Printf("[GenericLLM:%s] LLM failed: %v, using fallback\n", config.ID, err)
    }
    
    // Fallback to pattern matching
    return patternBasedCheck(text, config), nil
}

// callLLM makes API call to OpenAI
func callLLM(text string, config *GuardrailConfig, apiKey string) (*LLMResponse, error) {
    // Build prompt
    systemPrompt := config.PromptTemplate
    if systemPrompt == "" {
        systemPrompt = buildDefaultPrompt(config)
    }
    
    // Prepare request
    reqBody := map[string]interface{}{
        "model": "gpt-4",
        "messages": []map[string]string{
            {"role": "system", "content": systemPrompt},
            {"role": "user", "content": fmt.Sprintf("Analyze this content:\n\n%s", text[:min(3000, len(text))])},
        },
        "temperature": 0.1,
        "max_tokens":  400,
    }
    
    jsonBody, _ := json.Marshal(reqBody)
    
    req, _ := http.NewRequest("POST", "https://api.openai.com/v1/chat/completions", bytes.NewBuffer(jsonBody))
    req.Header.Set("Authorization", "Bearer "+apiKey)
    req.Header.Set("Content-Type", "application/json")
    
    client := &http.Client{Timeout: 15 * time.Second}
    resp, err := client.Do(req)
    if err != nil {
        return nil, err
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
        return nil, err
    }
    
    if len(apiResp.Choices) == 0 {
        return nil, fmt.Errorf("no response from OpenAI")
    }
    
    // Parse LLM response
    var llmResp LLMResponse
    if err := json.Unmarshal([]byte(apiResp.Choices[0].Message.Content), &llmResp); err != nil {
        return nil, err
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
        score = (1 - result.Confidence) * 100
    } else {
        // High compliance confidence = high compliance score
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
        // Default bad keywords
        keywords = []string{"inappropriate", "offensive", "illegal", "prohibited"}
    }
    
    for _, keyword := range keywords {
        matches += strings.Count(lowerText, strings.ToLower(keyword))
    }
    
    // Calculate score (inverse relationship with matches)
    score := 100.0 - float64(matches)*15.0
    if score < 20 {
        score = 20
    }
    
    metricName := config.MetricName
    if metricName == "" {
        metricName = "compliance_score"
    }
    
    return MetricValues{
        metricName: score,
    }
}

// buildDefaultPrompt creates a prompt from description
func buildDefaultPrompt(config *GuardrailConfig) string {
    return fmt.Sprintf(`You are a compliance checker. %s

Analyze the content and determine if it violates the policy.

Response format (JSON only):
{
  "violates_policy": true/false,
  "confidence": 0.0-1.0,
  "reasoning": "brief explanation",
  "violations": ["specific issues found"]
}

Be strict but fair. If uncertain, err on the side of caution.`, config.Description)
}

func min(a, b int) int {
    if a < b {
        return a
    }
    return b
}
```

---

### **3. API Handlers (`internal/api/guardrails.go`)**

```go
package api

import (
    "encoding/json"
    "net/http"
    "time"
    
    "acvps-gateway/pkg/txrepo"
)

// RegisterGuardrailHandler handles POST /guardrails/register
func RegisterGuardrailHandler(w http.ResponseWriter, r *http.Request) {
    if r.Method != http.MethodPost {
        http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
        return
    }
    
    var config txrepo.GuardrailConfig
    if err := json.NewDecoder(r.Body).Decode(&config); err != nil {
        http.Error(w, fmt.Sprintf("Invalid JSON: %v", err), http.StatusBadRequest)
        return
    }
    
    // Validate required fields
    if config.ID == "" || config.Name == "" || config.Description == "" {
        http.Error(w, "Missing required fields: id, name, description", http.StatusBadRequest)
        return
    }
    
    // Set defaults
    if config.MetricName == "" {
        config.MetricName = "compliance_score"
    }
    if config.Threshold == 0 {
        config.Threshold = 75
    }
    config.RegisteredAt = time.Now().Format(time.RFC3339)
    config.Type = "dynamic"
    
    // Register
    if err := txrepo.RegisterConfig(&config); err != nil {
        http.Error(w, err.Error(), http.StatusInternalServerError)
        return
    }
    
    // Check if custom implementation exists
    hasCustom := false
    if _, _, err := txrepo.GetGuardrail(config.ID); err == nil {
        hasCustom = true
    }
    
    source := "generic_template"
    if hasCustom {
        source = "custom_override"
    }
    
    w.Header().Set("Content-Type", "application/json")
    json.NewEncoder(w).Encode(map[string]interface{}{
        "success":      true,
        "guardrail_id": config.ID,
        "message":      fmt.Sprintf("Guardrail '%s' registered successfully", config.Name),
        "source":       source,
    })
}

// ListGuardrailsHandler handles GET /guardrails/configs
func ListGuardrailsHandler(w http.ResponseWriter, r *http.Request) {
    // Implementation here...
}
```

---

## 🚀 **USAGE FLOW**

### **Step 1: Customer adds guardrail in Dashboard**

```javascript
// Dashboard sends to portal backend
POST /api/guardrails/add
{
  "id": "custom_llm_assisted_smoking_compliance_checker",
  "name": "Smoking Compliance Checker",
  "description": "Check for any content that directly or indirectly promotes smoking",
  "keywords": ["cigarette", "tobacco", "smoking", "marlboro"],
  "metric_name": "confidence_score",
  "threshold": 75
}
```

### **Step 2: Portal backend registers with gateway**

```go
// Portal calls ACVPS Gateway
POST http://acvps-gateway:8443/guardrails/register
{
  "id": "custom_llm_assisted_smoking_compliance_checker",
  "name": "Smoking Compliance Checker",
  "description": "...",
  "keywords": ["..."],
  "metric_name": "confidence_score",
  "threshold": 75
}
```

### **Step 3: Gateway uses guardrail at runtime**

```go
// Contract specifies guardrail
contract := {
    "extractors": [
        {"id": "custom_llm_assisted_smoking_compliance_checker"}
    ],
    "envelope": {
        "constraints": {
            "confidence_score": {"min": 75}
        }
    }
}

// Gateway validates request
guardrailFn, config, _ := GetGuardrail("custom_llm_assisted_smoking_compliance_checker")
metrics, _ := guardrailFn(responsePayload)
// metrics = {"confidence_score": 82.5}

// Check envelope
if metrics["confidence_score"] < 75 {
    // BLOCK REQUEST
}
```

---

## ✅ **BENEFITS**

1. **Zero Deployment**: Add guardrails via Dashboard UI
2. **LLM Flexibility**: Works for any policy with just a description
3. **Performance Path**: Override with Go code when needed
4. **Pattern Fallback**: Works without OpenAI API
5. **Cost Control**: LLM calls only when needed

---

## 📋 **NEXT STEPS**

1. Implement `dynamic_registry.go`
2. Implement `generic_llm.go`
3. Add API handlers in `internal/api/guardrails.go`
4. Mount routes in `cmd/gateway/main.go`
5. Update portal backend to call gateway registration API
6. Test end-to-end flow


