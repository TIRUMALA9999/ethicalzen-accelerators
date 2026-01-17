# Financial Banking Chatbot - EthicalZen Accelerator

**Status:** ⚠️ **Tested - Partial Pass** (4/5 tests - 80%)  
**Industry:** Financial Services  
**Compliance:** PCI-DSS, GLBA, SOC 2

A PCI-DSS compliant banking chatbot protected by EthicalZen guardrails.

## 🛡️ Active Guardrails

1. **PCI Compliance** - Prevents exposure of payment card and account information
2. **Financial Advice Blocker** - Blocks unauthorized investment advice
3. **Prompt Injection Detector** - Blocks manipulation attempts
4. **PII Detector** - Prevents personal information leakage

## 🚀 Quick Start

```bash
# Clone and navigate
git clone https://github.com/aiworksllc/ethicalzen-accelerators.git
cd ethicalzen-accelerators/financial-banking-chatbot

# Configure environment
cp .env.example .env
# Edit .env and set your GROQ_API_KEY (or other LLM key)

# Start services
docker compose -f docker-compose.sdk.yml up -d

# Test
curl -X POST http://localhost:3000/chat \
  -H "Content-Type: application/json" \
  -d '{"message": "What types of savings accounts do you offer?"}'
```

## 📊 Architecture

```
Banking App (Port 3000)
    ↓
EthicalZen Gateway (Port 8080)
    ↓
Metrics Service (Port 9090)
```

## 🔧 Configuration

See [healthcare-patient-portal README](../healthcare-patient-portal/README.md) for detailed configuration options.

## 📜 License

MIT License
