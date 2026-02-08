# API Reference

This document describes the EthicalZen API endpoints used by Guardrail Studio.

## Base URL

```
https://api.ethicalzen.ai/v1
```

## Authentication

All API requests require an API key in the X-API-Key header:

```
X-API-Key: your-api-key
```

Get your API key at [ethicalzen.ai/dashboard](https://ethicalzen.ai/dashboard).

---

## Endpoints

### POST /evaluate

Evaluate content against a guardrail.

**Request:**

```json
{
  "guardrail": "medical_advice_smart",
  "input": "Take 500mg ibuprofen twice daily"
}
```

**Response:**

```json
{
  "decision": "block",
  "score": 0.82,
  "threshold": 0.65,
  "reason": "Medical prescription detected",
  "details": {
    "embedding_score": 0.78,
    "lexical_score": 0.91,
    "triggered_patterns": ["dosage_pattern", "imperative_take"]
  },
  "latency_ms": 45
}
```

**Parameters:**

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| guardrail | string | Yes | Guardrail ID (e.g., `medical_advice_smart`) |
| input | string | Yes | Content to evaluate |
| tenant_id | string | No | Multi-tenant identifier |
| context | object | No | Additional context for evaluation |

**Response Fields:**

| Field | Type | Description |
|-------|------|-------------|
| decision | string | `allow`, `block`, or `review` |
| score | number | Risk score (0.0 - 1.0) |
| threshold | number | Threshold used for decision |
| reason | string | Human-readable explanation |
| details | object | Debugging information |
| latency_ms | number | Processing time in milliseconds |

---

### POST /evaluate-batch

Evaluate content against multiple guardrails in parallel.

**Request:**

```json
{
  "guardrails": ["pii_blocker", "prompt_injection_blocker", "medical_advice_smart"],
  "input": "My SSN is 123-45-6789",
  "mode": "all_must_pass"
}
```

**Response:**

```json
{
  "overall_decision": "block",
  "blocked_by": ["pii_blocker"],
  "results": [
    {
      "guardrail": "pii_blocker",
      "decision": "block",
      "score": 0.9,
      "reason": "PII detected: ssn"
    },
    {
      "guardrail": "prompt_injection_blocker",
      "decision": "allow",
      "score": 0.0,
      "reason": "No injection patterns"
    },
    {
      "guardrail": "medical_advice_smart",
      "decision": "allow",
      "score": 0.15,
      "reason": "No medical advice detected"
    }
  ],
  "latency_ms": 52
}
```

**Parameters:**

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| guardrails | string[] | Yes | List of guardrail IDs |
| input | string | Yes | Content to evaluate |
| mode | string | No | `all_must_pass` (default) or `any_can_block` |

---

### POST /generate

Generate a new guardrail from description and examples.

**Request:**

```json
{
  "name": "Insurance Claim Detector",
  "description": "Block AI from processing or approving insurance claims. Allow general information about insurance.",
  "safe_examples": [
    "What is comprehensive auto insurance?",
    "How do deductibles work?"
  ],
  "unsafe_examples": [
    "I approve your claim for $5000",
    "Your claim has been processed and payment will be sent"
  ]
}
```

**Response:**

```json
{
  "guardrail_id": "insurance_claim_detector_v1",
  "calibration": {
    "t_allow": 0.28,
    "t_block": 0.62,
    "embedding_weight": 0.6,
    "lexical_weight": 0.4
  },
  "metrics": {
    "accuracy": 0.85,
    "precision": 0.88,
    "recall": 0.82,
    "f1": 0.85
  },
  "yaml": "id: insurance_claim_detector_v1\nname: Insurance Claim Detector\n..."
}
```

---

### GET /guardrails

List available guardrails.

**Response:**

```json
{
  "guardrails": [
    {
      "id": "medical_advice_smart",
      "name": "Medical Advice Blocker",
      "type": "smart_guardrail",
      "accuracy": 0.77,
      "category": "healthcare"
    },
    {
      "id": "financial_advice_smart",
      "name": "Financial Advice Blocker",
      "type": "smart_guardrail",
      "accuracy": 0.99,
      "category": "finance"
    }
  ]
}
```

---

### GET /guardrails/{id}

Get details of a specific guardrail.

**Response:**

```json
{
  "id": "medical_advice_smart",
  "name": "Medical Advice Blocker",
  "version": "1.0.0",
  "type": "smart_guardrail",
  "description": "Blocks diagnoses and prescriptions...",
  "calibration": {
    "t_allow": 0.25,
    "t_block": 0.65
  },
  "metrics": {
    "accuracy": 0.77,
    "precision": 0.79,
    "recall": 0.76
  },
  "metadata": {
    "category": "healthcare",
    "compliance": ["HIPAA"]
  }
}
```

---

### POST /sdk/recommend

Get template recommendations based on description.

**Request:**

```json
{
  "description": "Block personal financial advice and investment recommendations",
  "examples": ["Should I buy Tesla stock?"]
}
```

**Response:**

```json
{
  "recommendation_type": "USE_EXISTING",
  "confidence": 0.95,
  "matched_templates": [
    {
      "id": "financial_advice_smart",
      "name": "Financial Advice Blocker",
      "accuracy": 0.99,
      "match_score": 0.95
    }
  ],
  "message": "Existing template 'financial_advice_smart' covers this use case with 99% accuracy."
}
```

---

## Error Responses

### 400 Bad Request

```json
{
  "error": "invalid_request",
  "message": "guardrail field is required"
}
```

### 401 Unauthorized

```json
{
  "error": "unauthorized",
  "message": "Invalid or missing API key"
}
```

### 404 Not Found

```json
{
  "error": "not_found",
  "message": "Guardrail 'unknown_id' not found"
}
```

### 429 Rate Limited

```json
{
  "error": "rate_limited",
  "message": "Rate limit exceeded. Retry after 60 seconds.",
  "retry_after": 60
}
```

### 500 Internal Error

```json
{
  "error": "internal_error",
  "message": "An unexpected error occurred"
}
```

---

## Rate Limits

| Plan | Evaluations/month | Requests/second |
|------|-------------------|-----------------|
| Free | 1,000 | 10 |
| Starter | 50,000 | 100 |
| Pro | 500,000 | 500 |
| Enterprise | Unlimited | Custom |

---

## SDKs

Official SDKs are available for:

- **JavaScript/TypeScript**: `npm install @ethicalzen/sdk`
- **Python**: `pip install ethicalzen`
- **Go**: `go get github.com/ethicalzen/go-sdk`

See [SDK Documentation](https://docs.ethicalzen.ai/sdks) for details.

