# Education Tutoring Bot - EthicalZen Accelerator

**Status:** ⏳ **Ready for Testing**  
**Industry:** Education  
**Compliance:** FERPA, COPPA

A FERPA-compliant education tutor protected by EthicalZen guardrails.

## 🛡️ Active Guardrails

1. **FERPA Compliance** - Protects student education records
2. **Academic Integrity Enforcer** - Prevents direct homework answers
3. **Prompt Injection Detector** - Blocks manipulation attempts
4. **PII Detector** - Prevents student information leakage

## 🚀 Quick Start

```bash
# Clone and navigate
git clone https://github.com/aiworksllc/ethicalzen-accelerators.git
cd ethicalzen-accelerators/education-tutoring-bot

# Configure environment
cp .env.example .env
# Edit .env and set your GROQ_API_KEY (or other LLM key)

# Start services
docker compose -f docker-compose.sdk.yml up -d

# Test
curl -X POST http://localhost:3000/chat \
  -H "Content-Type: application/json" \
  -d '{"message": "Can you explain photosynthesis?"}'
```

## 📊 Architecture

```
Education App (Port 3000)
    ↓
EthicalZen Gateway (Port 8080)
    ↓
Metrics Service (Port 9090)
```

## 🔧 Configuration

See [healthcare-patient-portal README](../healthcare-patient-portal/README.md) for detailed configuration options.

## 📜 License

MIT License
