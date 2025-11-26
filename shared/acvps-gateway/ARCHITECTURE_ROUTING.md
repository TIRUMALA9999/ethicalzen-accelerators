# ACVPS Gateway: HTTP Protocol Extension Architecture

## Overview

The ACVPS Gateway is an **HTTP protocol extension** (similar to how HTTPS extends HTTP) that provides transparent validation and enforcement of digital contracts for any HTTP service.

## Core Concept: Protocol Extension

ACVPS works like HTTPS - it's a wrapper around standard HTTP that adds security guarantees:

```
HTTP  → Standard protocol
HTTPS → HTTP + TLS (encryption)
ACVPS → HTTP + Contract Validation (safety guarantees)
```

## ❌ What It's NOT

```
❌ WRONG: Traditional Reverse Proxy with hardcoded backends
   Client → Gateway (config: backend=api.example.com) → Single Backend
```

## ✅ What It IS

```
✅ CORRECT: Protocol Extension - Client specifies destination
   Client → Gateway (validates + routes) → Any Backend
   
   The client tells the gateway WHERE to send the request.
   The gateway validates BOTH request and response against the contract.
```

## How It Works

### 1. Traditional HTTP Call (No Gateway)

```http
POST https://api.groq.com/v1/chat/completions
Content-Type: application/json

{
  "model": "llama3-8b-8192",
  "messages": [{"role": "user", "content": "Hello"}]
}
```

### 2. ACVPS-Protected Call (With Gateway)

```http
POST https://gateway.ethicalzen.ai/v1/chat/completions
X-Contract-ID: groq-llm/ai/us/v1.0
X-Target-Endpoint: https://api.groq.com
X-DC-Id: cert_abc123
X-DC-Digest: sha256:def456...
Content-Type: application/json

{
  "model": "llama3-8b-8192",
  "messages": [{"role": "user", "content": "Hello"}]
}
```

**Key differences:**
- Request goes to **gateway**, not the service
- Client provides **X-Target-Endpoint** (where to route)
- Client provides **X-Contract-ID** (what contract to enforce)
- Client provides **DC headers** (certificate proof)

### 3. Gateway Processing Flow

```
┌──────────────────────────────────────────────────────────────┐
│  Step 1: Request Arrives at Gateway                          │
└──────────────────────────────────────────────────────────────┘
   POST /v1/chat/completions
   X-Contract-ID: groq-llm/ai/us/v1.0
   X-Target-Endpoint: https://api.groq.com
   X-DC-Id: cert_abc123
   X-DC-Digest: sha256:def456...

           ↓

┌──────────────────────────────────────────────────────────────┐
│  Step 2: Extract Target from Request Headers                 │
└──────────────────────────────────────────────────────────────┘
   target_url = r.Header.Get("X-Target-Endpoint")
   // "https://api.groq.com"
   
   contract_id = r.Header.Get("X-Contract-ID")
   // "groq-llm/ai/us/v1.0"

           ↓

┌──────────────────────────────────────────────────────────────┐
│  Step 3: Validate Request (Certificate Check)                │
└──────────────────────────────────────────────────────────────┘
   ✅ Does X-DC-Id exist?
   ✅ Is X-DC-Digest valid?
   ✅ Does certificate match contract?
   ❌ If validation fails → Return 409 DC_REQUIRED or 403 FORBIDDEN

           ↓

┌──────────────────────────────────────────────────────────────┐
│  Step 4: Forward to Target Service                           │
└──────────────────────────────────────────────────────────────┘
   POST https://api.groq.com/v1/chat/completions
   (Original request, minus ACVPS headers)

           ↓

┌──────────────────────────────────────────────────────────────┐
│  Step 5: Receive Response from Target                        │
└──────────────────────────────────────────────────────────────┘
   200 OK
   { "choices": [...], "usage": {...} }

           ↓

┌──────────────────────────────────────────────────────────────┐
│  Step 6: Validate Response (Content Check)                   │
└──────────────────────────────────────────────────────────────┘
   ✅ Extract features (PII, toxicity, bias, etc.)
   ✅ Check against contract envelope bounds
   ✅ If PII detected and contract prohibits → BLOCK
   ✅ If toxicity > threshold → BLOCK
   ❌ If validation fails → Return 403 BLOCKED

           ↓

┌──────────────────────────────────────────────────────────────┐
│  Step 7: Return Response to Client (or Block)                │
└──────────────────────────────────────────────────────────────┘
   200 OK (if passed validation)
   OR
   403 FORBIDDEN (if blocked)
```

## Protocol Extension Details

### Client Perspective

From the client's point of view, using ACVPS is simple:

**Without ACVPS:**
```javascript
fetch('https://api.groq.com/v1/chat/completions', {
  method: 'POST',
  headers: {
    'Authorization': 'Bearer gsk_...',
    'Content-Type': 'application/json'
  },
  body: JSON.stringify({ messages: [...] })
});
```

**With ACVPS:**
```javascript
const acvpsClient = new ACVPSClient({
  gatewayUrl: 'https://gateway.ethicalzen.ai',
  contractId: 'groq-llm/ai/us/v1.0',
  certificateId: 'cert_abc123'
});

// Client specifies the target in the call
acvpsClient.proxy({
  targetUrl: 'https://api.groq.com/v1/chat/completions',
  method: 'POST',
  headers: {
    'Authorization': 'Bearer gsk_...',
    'Content-Type': 'application/json'
  },
  body: JSON.stringify({ messages: [...] })
});

// SDK internally adds X-Target-Endpoint, X-Contract-ID, and DC headers
```

### Gateway Has No Hardcoded Backends

The gateway **does not know** where requests will go until they arrive:

```go
// Gateway handler (simplified)
func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
    // 1. Extract target from CLIENT's request
    targetEndpoint := r.Header.Get("X-Target-Endpoint")
    if targetEndpoint == "" {
        // Fallback to contract (optional)
        targetEndpoint = contract.TargetEndpoint
    }
    
    // 2. Validate request against contract
    if !h.validateRequest(r, contract) {
        http.Error(w, "DC_REQUIRED", 409)
        return
    }
    
    // 3. Forward to CLIENT-SPECIFIED target
    resp := h.proxyTo(targetEndpoint, r)
    
    // 4. Validate response
    if !h.validateResponse(resp, contract) {
        http.Error(w, "BLOCKED", 403)
        return
    }
    
    // 5. Return validated response
    w.Write(resp.Body)
}
```

## Multi-Tenant Routing

Each tenant can call **any service**, and the gateway enforces **tenant-scoped contracts**:

```
Tenant A (Hospital) - Contract: patient-records/healthcare/us/v1.0
  ├─ Call 1: X-Target-Endpoint: https://hospital-a.com/records
  └─ Call 2: X-Target-Endpoint: https://pharmacy-a.com/prescriptions
       ↓ Same contract, different backends!

Tenant B (Bank) - Contract: loan-service/finance/us/v1.0
  ├─ Call 1: X-Target-Endpoint: https://bank-b.com/loans
  └─ Call 2: X-Target-Endpoint: https://ml-service.bank-b.com/fraud
       ↓ Same contract, different backends!
```

**Key insight:** The contract defines **WHAT** to validate (PII, toxicity, etc.), not **WHERE** to send the request.

## Why This Architecture?

### Advantages

✅ **Flexibility**: Client chooses the backend (Groq, OpenAI, local service, etc.)
✅ **Multi-backend**: Same contract can protect calls to different services
✅ **No config changes**: Gateway doesn't need updates when backends change
✅ **True protocol extension**: Just like HTTPS, it wraps any HTTP call
✅ **LLM agnostic**: Works with any API (OpenAI, Groq, Anthropic, custom, etc.)
✅ **Service agnostic**: Not just for AI - works with any HTTP service

### Real-World Example

A healthcare app might use one contract but call multiple services:

```
Contract: patient-data/healthcare/us/v1.0
  Validates: No PII in logs, HIPAA compliance, no PHI leakage

Calls protected by this contract:
  ├─ https://ehr-system.hospital.com/patients
  ├─ https://lab-results.labcorp.com/results
  ├─ https://api.openai.com/v1/chat (for medical summaries)
  ├─ https://api.groq.com/v1/chat (for triage)
  └─ https://imaging-ai.radiology.com/analyze

All validated with the same contract, different backends!
```

## Setup Implications

### What the Gateway Needs

The gateway only needs:
1. **Gateway API Key** (to sync contracts from control plane)
2. **Tenant ID** (to load tenant-scoped contracts)
3. **Control Plane URL** (to fetch contracts and push evidence)

### What the Gateway Does NOT Need

❌ Backend URLs (clients provide via `X-Target-Endpoint`)
❌ Service-specific configuration
❌ Hardcoded routing rules

### Setup Script

```bash
./setup-local-gateway.sh

# Prompts for:
#   - Email/password (or API key) ✅
#   - Gateway name ✅
#   - Deployment mode ✅
#
# Does NOT prompt for:
#   - Backend URL ❌ (comes from client requests!)
```

## Contracts: Validation Rules, Not Routing Rules

Contracts define **WHAT** to validate, not **WHERE** to route:

```json
{
  "id": "groq-llm/ai/us/v1.0",
  "service_name": "groq-llm",
  "tenant_id": "tenant_abc123",
  
  "feature_extractors": {
    "pii_detector": { "enabled": true },
    "toxicity_detector": { "enabled": true },
    "bias_detector": { "enabled": true }
  },
  
  "envelope": {
    "pii_risk": { "max": 0.0 },
    "toxicity_score": { "max": 0.3 },
    "bias_score": { "max": 0.2 }
  },
  
  "target_endpoint": "https://api.groq.com"  // ⚠️ Optional fallback only!
}
```

**Note:** `target_endpoint` in the contract is an **optional fallback**. The primary source is the `X-Target-Endpoint` header from the client.

## Comparison to Other Technologies

### ACVPS vs API Gateway

| Feature | Traditional API Gateway | ACVPS Gateway |
|---------|------------------------|---------------|
| **Routing** | Configured in gateway | Client specifies in request |
| **Validation** | Basic (rate limiting, auth) | Deep (PII, toxicity, bias, content) |
| **Configuration** | Per-backend rules | Per-contract rules |
| **Flexibility** | Requires reconfiguration | Dynamic routing |
| **Use case** | Internal routing | External safety layer |

### ACVPS vs Service Mesh

| Feature | Service Mesh | ACVPS Gateway |
|---------|-------------|---------------|
| **Deployment** | In-cluster (K8s) | Standalone HTTP proxy |
| **Scope** | Internal services | External + internal |
| **Validation** | Network-level | Content-level (AI safety) |
| **Client changes** | None (transparent) | Minimal (SDK adds headers) |

### ACVPS vs HTTPS

| Feature | HTTPS | ACVPS |
|---------|-------|-------|
| **Purpose** | Encryption | Content validation |
| **Guarantees** | Confidentiality | Safety (no PII, no toxicity) |
| **Protocol** | TLS wrapper | HTTP proxy with headers |
| **Transparency** | Fully transparent | Requires SDK (header injection) |

## Summary

```
┌─────────────────────────────────────────────────────────────┐
│  ACVPS = HTTP Protocol Extension for AI Safety              │
│                                                              │
│  ✅ Client specifies target via X-Target-Endpoint           │
│  ✅ Gateway validates request (DC certificate check)        │
│  ✅ Gateway forwards to client-specified target             │
│  ✅ Gateway validates response (content safety check)       │
│  ✅ Works with ANY HTTP service (LLM agnostic)              │
│  ✅ No hardcoded backends in gateway config                 │
│  ✅ Contracts define validation, not routing                │
│                                                              │
│  Think: HTTPS for encryption, ACVPS for safety guarantees   │
└─────────────────────────────────────────────────────────────┘
```

## Further Reading

- **Architectural Challenges & Solutions**: `/docs/ARCHITECTURE_CHALLENGES.md` ⭐
- **Getting Started Guide**: `/docs/GETTING_STARTED.md`
- **Client SDK Documentation**: `/sdk/README.md`
- **Gateway Deployment Patterns**: `/docs/GATEWAY_DEPLOYMENT_PATTERNS.md`
- **Local Development Guide**: `/docs/LOCAL_DEVELOPMENT_GUIDE.md`
- **Contract Schema Reference**: `/docs/CONTRACT_SCHEMA.md`
