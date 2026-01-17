# Legal Document Assistant - EthicalZen Accelerator

**Status:** ✅ **Tested & Fixed** (5/5 tests - 100%)  
**Industry:** Legal Services  
**Compliance:** Attorney-Client Privilege, GDPR

An AI legal assistant protected by EthicalZen guardrails.

## 🛡️ Active Guardrails

1. **Legal Advice Blocker** - Prevents unauthorized legal advice
2. **Confidentiality Protector** - Protects attorney-client privilege
3. **Prompt Injection Detector** - Blocks manipulation attempts
4. **PII Detector** - Prevents personal information leakage

## 🚀 Quick Start

```bash
# Clone and navigate
git clone https://github.com/aiworksllc/ethicalzen-accelerators.git
cd ethicalzen-accelerators/legal-document-assistant

# Configure environment
cp .env.example .env
# Edit .env and set your GROQ_API_KEY (or other LLM key)

# Start services
docker compose -f docker-compose.sdk.yml up -d

# Test
curl -X POST http://localhost:3000/chat \
  -H "Content-Type: application/json" \
  -d '{"message": "What is a non-disclosure agreement?"}'
```

## 📊 Architecture

```
Legal App (Port 3000)
    ↓
EthicalZen Gateway (Port 8080)
    ↓
Metrics Service (Port 9090)
```

## 🔧 Configuration

See [healthcare-patient-portal README](../healthcare-patient-portal/README.md) for detailed configuration options.

## 📜 License

MIT License
