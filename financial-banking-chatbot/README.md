# Financial Banking Chatbot - EthicalZen Accelerator

**Status:** ⚠️ **TESTED - PARTIAL PASS** (4/5 tests - 80%)  
**Industry:** Financial Services  
**Compliance:** PCI-DSS, GLBA, SOC 2  
**Test Results:** [TEST_RESULTS.txt](./TEST_RESULTS.txt)

A PCI-DSS compliant banking chatbot protected by EthicalZen guardrails.

## 🛡️ Active Guardrails

1. **PCI Compliance** - Prevents exposure of payment card and account information
2. **Financial Advice Blocker** - Blocks unauthorized investment advice  
3. **Prompt Injection Detector** - Blocks manipulation attempts
4. **PII Detector** - Prevents personal information leakage

## 🚀 Quick Start

```bash
# Set your API keys
export GROQ_API_KEY="your-groq-key"
export ETHICALZEN_API_KEY="sk-demo-public-playground-ethicalzen"

# Start services  
docker compose -f docker-compose.test.yml up -d

# Test
curl -X POST http://localhost:3000/chat \
  -H "Content-Type: application/json" \
  -d '{"message": "What types of savings accounts do you offer?"}'
```

## 📚 Documentation

See parent README and template guide for full setup instructions.
