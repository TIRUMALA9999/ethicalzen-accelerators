# Guardrail Studio Backend Analysis Report

**Analysis Date**: 2026-02-08
**Analyzed by**: Claude (Super Moderator)
**Project Path**: `/Users/srinivasvaravooru/workspace/ethicalzen-accelerators/guardrail-studio/`

---

## Executive Summary

Guardrail Studio is a well-designed open-source UI for creating AI safety guardrails. The codebase shows **moderate production-readiness** with some **critical gaps** that need addressing before enterprise use.

| Area | Rating | Key Issues |
|------|--------|------------|
| API Client Architecture | ⚠️ **Fair** | Wrong API base URL, missing retry logic |
| Integration Examples | ✅ **Good** | Well-documented, but mock-heavy |
| YAML Templates | ⚠️ **Fair** | Schema mismatch with backend |
| Deployment Config | ✅ **Good** | Clean Docker/nginx/Cloud Build |
| Security | 🔴 **Needs Work** | Hardcoded demo key, missing auth patterns |
| Missing Features | 🔴 **Critical** | No SDK publish, no rate limiting docs |

---

## 1. API Client Architecture Analysis

### File: `src/lib/client.js`

#### Positive Findings

1. **Clean class-based design** - Good OOP structure with `EthicalZen` class
2. **Error handling** - Custom `EthicalZenError` class with status codes
3. **TypeScript JSDoc** - Good type definitions for IDE support
4. **Configurable base URL** - Allows custom API endpoints

#### Critical Issues

| Issue | Severity | Details |
|-------|----------|---------|
| **Wrong API Base URL** | 🔴 Critical | Client uses `https://api.ethicalzen.ai/v1` but actual backend uses `https://ethicalzen-backend-400782183161.us-central1.run.app` with `/api/` routes |
| **Wrong evaluate endpoint** | 🔴 Critical | Client calls `POST /evaluate` but backend has `POST /api/guardrails/evaluate` or `POST /api/sg/evaluate` |
| **Wrong auth header** | ⚠️ High | Client uses `Authorization: Bearer` but backend expects `X-API-Key` header |
| **No retry logic** | ⚠️ Medium | No exponential backoff for failed requests |
| **No timeout handling** | ⚠️ Medium | Relies on browser's default fetch timeout |
| **No request caching** | ℹ️ Low | Could benefit from caching frequent calls |

#### Code Analysis

```javascript
// ISSUE 1: Wrong API base
const DEFAULT_API_BASE = 'https://api.ethicalzen.ai/v1';
// Should be: 'https://ethicalzen-backend-400782183161.us-central1.run.app' or configurable

// ISSUE 2: Wrong evaluate endpoint
async evaluate(guardrailId, input) {
    const response = await this._request('POST', '/evaluate', {
        guardrail: guardrailId,
        input
    });
    // Should be: '/api/sg/evaluate' or '/api/guardrails/evaluate'
}

// ISSUE 3: Wrong auth header
headers: {
    'Authorization': `Bearer ${this.apiKey}`,  // Wrong!
    // Should be: 'X-API-Key': this.apiKey
}
```

#### Recommended Fixes

```javascript
// 1. Fix API base with fallback
const DEFAULT_API_BASE = process.env.ETHICALZEN_API_URL ||
    'https://ethicalzen-backend-400782183161.us-central1.run.app';

// 2. Fix endpoint paths
async evaluate(guardrailId, input) {
    return this._request('POST', '/api/sg/evaluate', {
        guardrail_id: guardrailId,  // Note: snake_case matches backend
        input
    });
}

// 3. Fix auth header
headers: {
    'X-API-Key': this.apiKey,
    'Content-Type': 'application/json',
    'User-Agent': 'ethicalzen-sdk/1.0.0'
}
```

---

## 2. Integration Examples Analysis

### 2.1 Express.js (`examples/express/index.js`)

| Aspect | Rating | Notes |
|--------|--------|-------|
| Structure | ✅ Good | Middleware pattern is correct |
| Auth | ⚠️ Issue | Uses `Authorization: Bearer` (wrong) |
| Error handling | ✅ Good | Fail-closed by default |
| Production-ready | ⚠️ Partial | Mock SDK, no real integration |

**Issues:**
- Uses `process.env.ETHICALZEN_API_KEY` but never validates
- Mock `EthicalZen` object instead of real SDK import
- No rate limiting or circuit breaker pattern
- `callYourLLM` is a stub - no guidance on real integration

### 2.2 Flask (`examples/python-flask/app.py`)

| Aspect | Rating | Notes |
|--------|--------|-------|
| Structure | ✅ Good | Decorator pattern is Pythonic |
| Auth | ⚠️ Issue | Uses `Authorization: Bearer` (wrong) |
| Error handling | ✅ Good | Fail-closed with proper logging |
| Production-ready | ⚠️ Partial | No async, no connection pooling |

**Issues:**
- Synchronous requests (should use `aiohttp` or `httpx`)
- No request session reuse (connection pooling)
- Warning printed but app still runs without API key
- No retry logic for network failures

### 2.3 Next.js Middleware (`examples/nextjs/middleware.ts`)

| Aspect | Rating | Notes |
|--------|--------|-------|
| Structure | ✅ Excellent | Proper Edge Runtime compatible |
| Auth | ⚠️ Issue | Uses `Authorization: Bearer` (wrong) |
| Type safety | ✅ Good | TypeScript interfaces defined |
| Production-ready | ✅ Good | Best of the examples |

**Issues:**
- Same auth header issue
- `request.json()` consumes body - may not work with middleware chain
- No caching of guardrail results for repeated requests

### 2.4 LangChain (`examples/langchain/guardrail_chain.py`)

| Aspect | Rating | Notes |
|--------|--------|-------|
| Structure | ✅ Good | Proper Chain implementation |
| Auth | ⚠️ Issue | Uses `Authorization: Bearer` (wrong) |
| Compatibility | ⚠️ Dated | Uses older LangChain APIs |
| Production-ready | ⚠️ Partial | Missing streaming support |

**Issues:**
- `langchain.chains.base.Chain` is deprecated pattern
- No streaming support (critical for LLM apps)
- Creates new `EthicalZenGuardrail` instance per call (inefficient)
- No async version of `evaluate()`

### All Examples - Common Auth Issue

```python
# ALL examples use this (WRONG):
headers={
    "Authorization": f"Bearer {ETHICALZEN_API_KEY}",
}

# Backend actually expects (CORRECT):
headers={
    "X-API-Key": ETHICALZEN_API_KEY,
    "Content-Type": "application/json"
}
```

---

## 3. YAML Template Accuracy

### Schema Comparison

| Field | Template Schema | Backend Schema (`schema.js`) | Match |
|-------|----------------|------------------------------|-------|
| `id` | `pii_blocker` | `^[a-z0-9_]+_v[0-9]+$` or flexible for `smart_guardrail` | ⚠️ Partial |
| `type` | `regex`, `smart_guardrail`, `hybrid` | `deterministic`, `llm_assisted`, `ml_model`, `smart_guardrail`, `hybrid` | ⚠️ Mismatch |
| `calibration` | `t_allow`, `t_block`, `model` | Not in schema, handled differently | ❌ Mismatch |
| `patterns` | Object with named patterns | Array of strings | ❌ Mismatch |
| `metrics` | Not defined | Required array/object | ❌ Missing |

### Template-by-Template Analysis

#### `pii-blocker.yaml`

```yaml
# Template uses:
type: regex
patterns:
  ssn: '\b\d{3}-\d{2}-\d{4}\b'

# Backend expects:
type: deterministic  # 'regex' not in enum
metrics: ["pii_risk"]  # Required field missing
implementation:
  type: deterministic
  patterns: ['\b\d{3}-\d{2}-\d{4}\b']  # Flat array
```

**Issues:**
1. `type: regex` should be `type: deterministic`
2. Missing required `metrics` field
3. `patterns` should be flat array, not object
4. Missing `version` format validation (should be `pii_blocker_v1`)

#### `financial-advice.yaml`

```yaml
# Template uses:
calibration:
  t_allow: 0.25
  t_block: 0.66

# Actual backend SmartGuardrail uses:
calibration:
  threshold: 0.5
  safe_examples: [...]
  unsafe_examples: [...]
```

**Issues:**
1. `t_allow`/`t_block` naming doesn't match backend
2. Missing required `metrics` field
3. ID should be `financial_advice_blocker` to match guardrail naming convention

#### `prompt-injection.yaml`

```yaml
type: hybrid
patterns:
  instruction_override:
    - 'ignore\s+(all\s+)?(previous|prior|above)'
keywords:
  - "ignore previous"
```

**Issues:**
1. `hybrid` type is valid but needs specific implementation structure
2. Nested patterns not supported by backend
3. `keywords` field not in backend schema

### Backend Schema Reference

From `portal/backend/src/guardrails/schema.js`:
```javascript
const GuardrailSchema = {
  required: ['id', 'name', 'version', 'type', 'metrics'],
  properties: {
    type: {
      enum: ['deterministic', 'llm_assisted', 'ml_model', 'smart_guardrail', 'hybrid']
    },
    metrics: {
      type: 'array',  // or object
      minItems: 1
    }
  }
};
```

### Recommended Template Structure

```yaml
# Fixed template example
id: pii_blocker_v1
name: PII Blocker
version: 1.0.0
type: deterministic

metrics:
  - pii_risk

implementation:
  type: deterministic
  patterns:
    - '\b\d{3}-\d{2}-\d{4}\b'  # SSN
    - '\b[A-Za-z0-9._%+-]+@[A-Za-z0-9.-]+\.[A-Z|a-z]{2,}\b'  # Email

safe_examples: [...]
unsafe_examples: [...]

metadata:
  category: privacy
  risk_level: critical
```

---

## 4. Deployment Configuration Analysis

### Dockerfile

| Aspect | Rating | Notes |
|--------|--------|-------|
| Base image | ✅ Good | `nginx:alpine` is minimal |
| Security | ✅ Good | Non-root by default in alpine nginx |
| Health check | ✅ Good | Proper `HEALTHCHECK` directive |
| Size | ✅ Excellent | ~20MB image |

**Minor Issues:**
- No `.dockerignore` file (may copy unnecessary files)
- Could add labels for versioning

### nginx.conf

| Aspect | Rating | Notes |
|--------|--------|-------|
| Security headers | ✅ Good | X-Frame-Options, XSS, nosniff |
| Compression | ✅ Good | Gzip enabled |
| Caching | ✅ Good | 1-hour cache for static |
| SPA support | ✅ Good | Fallback to index.html |

**Issues:**
- Missing `Content-Security-Policy` header
- Missing `Strict-Transport-Security` (HSTS)
- Could add `/api` proxy for local dev

**Recommended additions:**
```nginx
# Add these security headers
add_header Content-Security-Policy "default-src 'self'; script-src 'self' 'unsafe-inline'; style-src 'self' 'unsafe-inline' fonts.googleapis.com; font-src fonts.gstatic.com" always;
add_header Strict-Transport-Security "max-age=31536000; includeSubDomains" always;
```

### cloudbuild.yaml

| Aspect | Rating | Notes |
|--------|--------|-------|
| Structure | ✅ Good | Proper multi-step build |
| Tagging | ✅ Good | Both SHA and :latest tags |
| Logging | ✅ Good | CLOUD_LOGGING_ONLY |
| Timeout | ✅ Good | 10 min is reasonable |

**Issue:**
- `dir: 'guardrail-studio'` assumes monorepo structure
- Should add build-time args for versioning

### deploy.sh

| Aspect | Rating | Notes |
|--------|--------|-------|
| Error handling | ✅ Good | `set -e` stops on failure |
| Configurability | ✅ Good | Accepts project ID arg |
| Documentation | ✅ Good | Clear echo statements |

**Issues:**
- Hardcoded region (`us-central1`)
- No rollback mechanism
- Uses `--allow-unauthenticated` (public access)

---

## 5. Security Audit

### Critical Security Issues

| Issue | Severity | Location | Risk |
|-------|----------|----------|------|
| **Hardcoded demo API key** | ✅ Fixed | `index.html` | SCRUM-112: Removed hardcoded key |
| **Wrong auth header in SDK** | ✅ Fixed | `client.js` | SCRUM-109: Now uses X-API-Key |
| **No rate limiting docs** | ⚠️ High | README | DoS risk |
| **Allow-unauthenticated deployment** | ⚠️ High | `deploy.sh:32` | Public exposure |
| **CORS allows all origins** | ⚠️ Medium | Backend `server.js:178` | CSRF risk |

### Detailed Findings

#### 1. Hardcoded Demo API Key — ✅ FIXED (SCRUM-112)

~~```javascript
// index.html line 1967 (OLD - REMOVED)
const DEMO_API_KEY = 'removed-for-security';
```~~

**Status**: Fixed. Hardcoded demo API key has been removed. Users must now configure their own API key on first load.

**Changes made**:
- Removed `DEMO_API_KEY` constant
- API key defaults to empty string
- Modal shown on first load when no key configured
- Button shows warning state when unconfigured

#### 2. Authentication Header Mismatch — ✅ FIXED (SCRUM-109)

~~SDK and all examples use `Authorization: Bearer` but backend expects `X-API-Key`~~

**Status**: Fixed. SDK and all examples now use correct `X-API-Key` header:

```javascript
// SDK and all examples (FIXED)
'X-API-Key': this.apiKey
```

#### 3. No Security Documentation

README lacks:
- Rate limiting policies
- API key scoping (what can demo key do?)
- Data retention policies
- PII handling in templates

#### 4. Missing Content Security Policy

`nginx.conf` doesn't include CSP, allowing potential XSS:

```nginx
# Missing:
add_header Content-Security-Policy "default-src 'self'..." always;
```

#### 5. CORS Configuration

Backend allows all origins:
```javascript
callback(null, true); // Allow all for now
```

This is acceptable for a demo but risky for production.

---

## 6. Missing Backend Features

### Critical Missing Features

| Feature | Impact | Effort | Priority |
|---------|--------|--------|----------|
| **SDK not published to npm** | Can't `npm install` | Medium | 🔴 P0 |
| **API versioning** | Breaking changes risk | Low | 🔴 P0 |
| **Webhook support** | No async notifications | High | ⚠️ P1 |
| **Batch evaluation** | Performance issues | Medium | ⚠️ P1 |
| **API key scopes** | Security risk | Medium | ⚠️ P1 |

### Detailed Analysis

#### 1. SDK Not Published

`package.json` references `@ethicalzen/guardrail-studio` but this isn't on npm:
```bash
$ npm view @ethicalzen/guardrail-studio
npm ERR! code E404
```

**Required steps:**
1. Separate `src/lib/client.js` into own package
2. Add build step (transpile to CJS/ESM)
3. Publish to npm registry

#### 2. No API Versioning

Client assumes `/v1` but backend doesn't version:
```javascript
const DEFAULT_API_BASE = 'https://api.ethicalzen.ai/v1';  // /v1 doesn't exist
```

**Recommendation**: Add version prefix to all routes (`/api/v1/...`)

#### 3. Missing Batch Evaluation

Current: One request per guardrail per input
Needed: Batch multiple inputs in single request

```javascript
// Needed API:
POST /api/sg/evaluate/batch
{
  "guardrail_id": "pii_blocker",
  "inputs": ["text1", "text2", "text3"]
}
```

#### 4. No Streaming Evaluation

For LLM apps that stream responses, need SSE evaluation:
```javascript
// Needed:
POST /api/sg/evaluate/stream
Content-Type: text/event-stream
```

Backend has `/api/stream/sse` but SDK doesn't support it.

#### 5. Missing Features in SDK

```javascript
// Currently missing in client.js:

// 1. Streaming support
async evaluateStream(guardrailId, input, onChunk) { ... }

// 2. Batch evaluation
async evaluateBatch(guardrailId, inputs) { ... }

// 3. Guardrail import from YAML
async importFromYaml(yamlContent) { ... }

// 4. Health check
async health() { ... }

// 5. Retry with backoff
async _requestWithRetry(method, path, body, retries = 3) { ... }
```

---

## 7. Recommendations Summary

### Immediate Actions (P0)

1. **Fix API client auth header** - Change from `Bearer` to `X-API-Key`
2. **Fix API endpoints** - Match actual backend routes
3. **Fix template schemas** - Add required `metrics` field
4. **Publish SDK to npm** - Enable `npm install @ethicalzen/sdk`

### Short-term (P1)

1. **Add CSP header** to nginx.conf
2. **Document rate limits** in README
3. **Add retry logic** to SDK
4. **Create proper test suite** for SDK

### Medium-term (P2)

1. **Add batch evaluation** endpoint and SDK method
2. **Add streaming support** for LLM apps
3. **Create Python SDK** (separate package)
4. **Add API versioning** (`/api/v1/...`)

### Long-term (P3)

1. **Webhook support** for async notifications
2. **API key scopes** for granular permissions
3. **SDK caching layer** for repeated calls
4. **OpenTelemetry integration** for observability

---

## Appendix: File Inventory

| File | Purpose | Status |
|------|---------|--------|
| `src/lib/client.js` | JavaScript SDK | ⚠️ Needs fixes |
| `examples/express/index.js` | Express integration | ⚠️ Needs fixes |
| `examples/python-flask/app.py` | Flask integration | ⚠️ Needs fixes |
| `examples/nextjs/middleware.ts` | Next.js middleware | ⚠️ Needs fixes |
| `examples/langchain/guardrail_chain.py` | LangChain integration | ⚠️ Needs fixes |
| `templates/*.yaml` | Guardrail templates | ⚠️ Schema mismatch |
| `Dockerfile` | Container build | ✅ Good |
| `nginx.conf` | Web server config | ⚠️ Missing CSP |
| `deploy.sh` | Deployment script | ✅ Good |
| `cloudbuild.yaml` | CI/CD config | ✅ Good |
| `package.json` | Package manifest | ✅ Good |
| `index.html` | Main UI | ✅ Good |

---

*Report generated by Claude Super Moderator*
*For questions, contact the EthicalZen development team*
