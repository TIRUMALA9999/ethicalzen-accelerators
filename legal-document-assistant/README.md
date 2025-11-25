# Legal Document Assistant - EthicalZen Accelerator

**Status:** ✅ **TESTED & FIXED** (5/5 tests - 100%)  
**Industry:** Legal Services  
**Compliance:** Attorney-Client Privilege, GDPR  
**Test Results:** [TEST_RESULTS.txt](./TEST_RESULTS.txt)  
**Fix Applied:** Guardrail invert_score corrected

An AI legal assistant protected by EthicalZen guardrails.

## 🛡️ Active Guardrails

1. **Legal Advice Blocker** - Prevents unauthorized legal advice
2. **Confidentiality Protector** - Protects attorney-client privilege  
3. **Prompt Injection Detector** - Blocks manipulation attempts
4. **PII Detector** - Prevents personal information leakage

## 🚀 Quick Start

```bash
export GROQ_API_KEY="your-groq-key"
docker compose -f docker-compose.test.yml up -d
curl -X POST http://localhost:3000/chat \
  -H "Content-Type: application/json" \
  -d '{"message": "What is a non-disclosure agreement?"}'
```

See parent README and template guide for full setup instructions.
