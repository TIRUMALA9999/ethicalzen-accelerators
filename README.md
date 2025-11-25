# EthicalZen Accelerators

Production-ready AI applications with built-in safety guardrails. These accelerators demonstrate how to integrate EthicalZen's AI safety platform into real-world applications.

## 🚀 Available Accelerators

| Accelerator | Industry | Use Case | Guardrails | Status |
|------------|----------|----------|------------|--------|
| [Healthcare Patient Portal](./healthcare-patient-portal/) | Healthcare | Patient Q&A chatbot | HIPAA, Medical Advice, PHI Protection, Prompt Injection | ✅ Tested |
| [Financial Banking Chatbot](./financial-banking-chatbot/) | Financial | Banking support bot | PCI Compliance, Financial Advice, Prompt Injection | ✅ Tested |
| [Legal Document Assistant](./legal-document-assistant/) | Legal | Document Q&A | Legal Advice, Confidentiality, Prompt Injection | ✅ Tested |

## 📋 What You Get

Each accelerator includes:
- **Full source code** - See exactly how to integrate EthicalZen guardrails
- **Docker setup** - Run locally with one command
- **Example queries** - Test positive and negative cases
- **Configuration** - Customize for your use case
- **Documentation** - Complete setup guide

## 🎯 Quick Start (5 Minutes)

### Prerequisites

1. **Docker & Docker Compose** - [Install Docker](https://docs.docker.com/get-docker/)
2. **LLM API Key** - Get a free key from:
   - [Groq](https://console.groq.com) (Recommended - Fast & Free)
   - [OpenAI](https://platform.openai.com/api-keys)
   - [Anthropic](https://console.anthropic.com)

3. **EthicalZen Platform Images** - Download from [Releases](https://github.com/aiworksllc/ethicalzen-accelerators/releases):
   ```bash
   # Download 2 pre-built platform images
   curl -LO https://github.com/aiworksllc/ethicalzen-accelerators/releases/download/v1.0/acvps-gateway-v1.0.tar.gz
   curl -LO https://github.com/aiworksllc/ethicalzen-accelerators/releases/download/v1.0/metrics-service-v1.0.tar.gz
   
   # Load into Docker
   docker load < acvps-gateway-v1.0.tar.gz
   docker load < metrics-service-v1.0.tar.gz
   ```

### Run Healthcare Accelerator

```bash
# Clone this repo
git clone https://github.com/aiworksllc/ethicalzen-accelerators
cd ethicalzen-accelerators/healthcare-patient-portal

# Set your LLM API key
export GROQ_API_KEY="your-groq-api-key-here"

# Start services (builds app from source, uses pre-built gateway/metrics)
docker compose up -d

# Wait 30 seconds for services to be ready
docker compose ps

# Test it!
curl -X POST http://localhost:3000/chat \
  -H "Content-Type: application/json" \
  -d '{"message": "What are symptoms of the flu?"}'
```

**Expected response:**
```json
{
  "response": "Common flu symptoms include fever, chills, cough...",
  "ethicalzen": {
    "status": "APPROVED",
    "guardrails_checked": ["prompt_injection", "hipaa_compliance", "medical_advice"],
    "validation_time_ms": 125
  }
}
```

## 🛡️ How Guardrails Work

EthicalZen validates both **input** (user requests) and **output** (LLM responses):

```
User Request
    ↓
Healthcare App (you build this - full source code provided)
    ↓
EthicalZen Gateway (validates INPUT)
    ├─ ✅ Prompt injection check
    ├─ ✅ Malicious content check
    └─ ✅ Contract compliance
    ↓
LLM API (OpenAI, Groq, Anthropic - your key)
    ↓
EthicalZen Gateway (validates OUTPUT)
    ├─ ✅ Medical diagnosis check
    ├─ ✅ PHI leakage check
    └─ ✅ Safety guardrails
    ↓
Response to User
```

### Example: Blocking Medical Diagnosis

```bash
# This query will be BLOCKED
curl -X POST http://localhost:3000/chat \
  -H "Content-Type: application/json" \
  -d '{"message": "I have chest pain. Diagnose me and prescribe medication."}'
```

**Response:**
```json
{
  "error": "INPUT_BLOCKED",
  "message": "Request blocked by security policy",
  "details": {
    "contract_id": "Healthcare Patient Portal Demo/healthcare/us/v1.0",
    "violation": "diagnosis_risk",
    "value": 0.8,
    "threshold": 0.1
  }
}
```

## 📂 What's Open Source vs Proprietary?

### Open Source (This Repo)
✅ **Accelerator application code** (`app/server.js`) - Full implementation  
✅ **Docker configuration** - How everything connects  
✅ **Guardrail configuration** (`ethicalzen-config.json`)  
✅ **Integration examples** - How to call the gateway  
✅ **Test cases** - Positive and negative examples  

### Proprietary (Pre-built Images)
🔒 **EthicalZen Gateway** - Validation engine (Go binary)  
🔒 **Metrics Service** - Telemetry collector (Node.js compiled)  
🔒 **Portal Backend** - Contract management (cloud-hosted)  

**You get**: Working examples + integration patterns  
**We protect**: Guardrail algorithms + detection logic

## 🔑 BYOK (Bring Your Own Key)

All accelerators use **your** LLM API keys - your data never touches EthicalZen servers:
- ✅ You control your LLM provider (OpenAI, Groq, Anthropic, Azure)
- ✅ You control your data (stays with your LLM provider)
- ✅ You control your costs (pay your LLM provider directly)

**EthicalZen only receives**: Request metadata for guardrail monitoring (no prompts/responses)

## 📖 Documentation

Each accelerator includes:
- **README.md** - Setup guide and testing instructions
- **.env.example** - Environment variables template
- **ethicalzen-config.json** - Guardrail configuration
- **app/server.js** - Full source code with comments

## 🏗️ Architecture

```yaml
services:
  # Your app - builds from source (full code provided)
  app:
    build: .
    
  # EthicalZen Gateway - pre-built image (proprietary)
  gateway:
    image: ethicalzen/acvps-gateway:v1.0
    
  # Metrics Service - pre-built image (proprietary)
  metrics:
    image: ethicalzen/metrics-service:v1.0
```

## 🚀 Next Steps

1. **Try an accelerator** - Healthcare is the most tested
2. **Read the source code** - See how to integrate guardrails
3. **Test positive & negative cases** - See guardrails in action
4. **Customize for your use case** - Modify the app code
5. **Register your own use case** - Get custom guardrails at [EthicalZen Portal](https://ethicalzen-backend-400782183161.us-central1.run.app)

## 📊 Monitoring

View real-time metrics locally:
```bash
curl http://localhost:9090/metrics/summary?tenant_id=demo | jq .
```

Or view in the cloud dashboard (if using your own account):
https://ethicalzen-backend-400782183161.us-central1.run.app/dashboard

## 🆘 Troubleshooting

### "Image not found" error
```bash
# Make sure you loaded the platform images
docker images | grep ethicalzen
# Should show: acvps-gateway:v1.0 and metrics-service:v1.0
```

### "LLM API key not configured"
```bash
# Set your API key before starting
export GROQ_API_KEY="your-key-here"
docker compose restart app
```

### Services won't start
```bash
# Check Docker is running
docker ps

# Check logs
docker logs healthcare-portal
docker logs acvps-gateway-local
docker logs metrics-service-local
```

## 🤝 Support

- **Issues**: [GitHub Issues](https://github.com/aiworksllc/ethicalzen-accelerators/issues)
- **Documentation**: See individual accelerator READMEs
- **Email**: support@ethicalzen.ai
- **Portal**: https://ethicalzen-backend-400782183161.us-central1.run.app

## 📜 License

Accelerator source code: MIT License  
Pre-built platform images: Proprietary - contact sales@ethicalzen.ai for licensing

## 🌟 What's Next?

Building your own AI application? These accelerators show you:
- ✅ How to integrate EthicalZen guardrails
- ✅ How to handle validation responses
- ✅ How to configure industry-specific rules
- ✅ How to test safety controls
- ✅ How to deploy with Docker

Start with the [Healthcare Patient Portal](./healthcare-patient-portal/) - it's the most complete example!

---

**Built with ❤️ by [EthicalZen](https://ethicalzen.ai) | Making AI safer, one guardrail at a time**
