# E-Commerce Support Chatbot - EthicalZen Accelerator

**Status:** ⏳ **Ready for Testing**  
**Industry:** E-Commerce  
**Compliance:** PCI-DSS, GDPR, CCPA

A customer support chatbot for e-commerce protected by EthicalZen guardrails.

## 🛡️ Active Guardrails

1. **PCI Compliance** - Prevents payment information exposure
2. **Scam Link Detector** - Blocks phishing/malicious links
3. **Prompt Injection Detector** - Blocks manipulation attempts
4. **PII Detector** - Prevents customer information leakage

## 🚀 Quick Start

```bash
# Clone and navigate
git clone https://github.com/aiworksllc/ethicalzen-accelerators.git
cd ethicalzen-accelerators/ecommerce-support-chatbot

# Configure environment
cp .env.example .env
# Edit .env and set your GROQ_API_KEY (or other LLM key)

# Start services
docker compose -f docker-compose.sdk.yml up -d

# Test
curl -X POST http://localhost:3000/chat \
  -H "Content-Type: application/json" \
  -d '{"message": "How do I track my order?"}'
```

## 📊 Architecture

```
E-Commerce App (Port 3000)
    ↓
EthicalZen Gateway (Port 8080)
    ↓
Metrics Service (Port 9090)
```

## 🔧 Configuration

See [healthcare-patient-portal README](../healthcare-patient-portal/README.md) for detailed configuration options.

## 📜 License

MIT License
