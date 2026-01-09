# 🌐 Live API Testing

**Retrofit guardrails on your existing AI APIs without code changes.**

---

## Overview

Live API Testing is the **brownfield workflow** for teams who already have AI APIs in production. Instead of rebuilding your application, you can:

1. **Import** your OpenAPI/Swagger spec
2. **Test** your live API endpoints against 20+ guardrails
3. **Evaluate** responses for PII, toxicity, prompt injection, and more
4. **Certify** compliant endpoints
5. **Fix** identified gaps

---

## Quick Start

### Prerequisites

- Node.js 14+
- EthicalZen API key ([Get one free](https://ethicalzen.ai))

### Run the Demo

```bash
# Clone the repo
git clone https://github.com/aiworksllc/ethicalzen-accelerators.git
cd ethicalzen-accelerators/live-api-testing

# Run with demo key (limited)
node src/demo.js

# Or use your own key
ETHICALZEN_API_KEY=sk-your-key node src/demo.js
```

### Expected Output

```
🌐 LIVE API TESTING DEMO

📋 Step 1: Loading guardrails...     ✅ 20 guardrails
📄 Step 2: Importing OpenAPI spec... ✅ 19 endpoints  
🧪 Step 3: Running live tests...     ✅ 100% pass rate
   ✅ Clean content: pass (allow)
   ✅ SSN in message: pass (block)
   ✅ Email address: pass (block)
📜 Step 4: Certificate ready

📊 SUMMARY
   Pass rate: 100%
```

---

## 🔍 Analyze Your API (Interactive)

Get a per-endpoint risk analysis and guardrail recommendations:

```bash
node src/analyze-api.js https://your-api.com/openapi.json
```

### Example Output

```
🔍 ETHICALZEN API ANALYZER

STEP 1: IMPORTING OPENAPI SPECIFICATION
✅ Successfully imported: Your API
   📊 Total Endpoints: 15
   🤖 AI Endpoints:    3

STEP 2: PER-ENDPOINT RISK ANALYSIS

┌─────────────────────────────────────────────────────────────────────┐
│ POST   /v1/chat/completions
│ Risk Level: 🔴 CRITICAL
└─────────────────────────────────────────────────────────────────────┘
   ⚠️  Key Issues:
      🔴 AI/LLM endpoint - requires input/output guardrails
      🔴 Data mutation endpoint - input validation needed

   🛡️  Recommended Guardrails:
      ✓ Prompt Injection Blocker
      ✓ Toxicity Detector
      ✓ PII Blocker
      ✓ Content Moderation

┌─────────────────────────────────────────────────────────────────────┐
│ GET    /user/{id}
│ Risk Level: 🟠 HIGH
└─────────────────────────────────────────────────────────────────────┘
   ⚠️  Key Issues:
      🟠 User data handling - PII exposure risk

   🛡️  Recommended Guardrails:
      ✓ PII Blocker
      ✓ Data Leakage Prevention

STEP 3: SUMMARY
   Critical Risk Endpoints: 1
   High Risk Endpoints:     2
   Unique Guardrails Needed: 6
```

---

## Files Included

| File | Description |
|------|-------------|
| `src/analyze-api.js` | **Interactive analyzer** - takes Swagger URL, shows per-endpoint breakdown |
| `src/demo.js` | Quick demo using httpbin |
| `src/test-your-api.js` | Template to test your own API |
| `src/ci-integration.js` | CI/CD pipeline integration |
| `src/interactive-demo.js` | Step-by-step walkthrough |
| `LIVE_API_TESTING_GUIDE.md` | Full documentation |

---

## Test Your Own API

1. Edit the configuration in `src/test-your-api.js`:

```javascript
const config = {
  ethicalzenApiKey: process.env.ETHICALZEN_API_KEY,
  apiEndpoint: 'https://your-api.com/v1/chat',
  method: 'POST',
  headers: {
    'Authorization': 'Bearer YOUR_API_TOKEN'
  },
  testCases: [
    { name: 'Safe content', input: {...}, expectedBehavior: 'allow' },
    { name: 'PII test', input: {...}, expectedBehavior: 'block' }
  ]
};
```

2. Run:

```bash
ETHICALZEN_API_KEY=sk-your-key node src/test-your-api.js
```

---

## CI/CD Integration

Add guardrail tests to your pipeline:

```bash
# GitHub Actions / Jenkins / etc.
ETHICALZEN_API_KEY=${{ secrets.ETHICALZEN_API_KEY }} \
API_ENDPOINT=https://your-api.com/chat \
MIN_PASS_RATE=100 \
node src/ci-integration.js
```

Exit codes:
- `0` = All tests passed
- `1` = Tests failed or pass rate below threshold

### GitHub Actions Example

```yaml
- name: Run Guardrail Tests
  run: |
    ETHICALZEN_API_KEY=${{ secrets.ETHICALZEN_API_KEY }} \
    API_ENDPOINT=${{ secrets.API_ENDPOINT }} \
    node live-api-testing/src/ci-integration.js
```

---

## API Endpoints Used

| Endpoint | Method | Description |
|----------|--------|-------------|
| `/api/liveapi/guardrails` | GET | List available guardrails |
| `/api/liveapi/openapi/analyze` | POST | Import OpenAPI spec |
| `/api/liveapi/test` | POST | Run live API tests |
| `/api/liveapi/openapi/certificates` | POST | Get certificate recommendations |

Base URL: `https://api.ethicalzen.ai`

---

## What Gets Tested?

| Guardrail | What It Detects |
|-----------|-----------------|
| PII Blocker | SSN, emails, phone numbers, addresses |
| Toxicity Detector | Hate speech, harassment, threats |
| Prompt Injection | Jailbreak attempts, instruction override |
| Financial Advice | Unauthorized investment recommendations |
| Medical Advice | Unauthorized health recommendations |
| Bias Detector | Discriminatory content |
| + 14 more | [See full list](https://ethicalzen.ai/docs) |

---

## Documentation

- [Full Guide](./LIVE_API_TESTING_GUIDE.md) - Complete walkthrough
- [Online Docs](https://ethicalzen.ai/docs#liveapi) - Web documentation
- [Dashboard](https://ethicalzen.ai/dashboard) - Manage guardrails

---

## Support

- 📧 Email: support@ethicalzen.ai
- 💬 Slack: [Join community](https://ethicalzen.ai/slack)
- 🐛 Issues: [GitHub Issues](https://github.com/aiworksllc/ethicalzen-accelerators/issues)

