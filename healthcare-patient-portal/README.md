# Healthcare Patient Portal - EthicalZen Accelerator

**Status:** ✅ **TESTED & PRODUCTION READY**  
**Test Coverage:** 5/5 test cases passed (100%)  
**Test Report:** See [TEST_REPORT.md](./TEST_REPORT.md)

A HIPAA-compliant healthcare chatbot powered by EthicalZen guardrails and your choice of LLM provider.

## 🎯 What This Accelerator Does

This accelerator demonstrates how to build a secure healthcare chatbot that:
- ✅ Validates user input for prompt injection and malicious attempts
- ✅ Enforces HIPAA compliance in LLM responses
- ✅ Blocks medical diagnosis and prescription recommendations
- ✅ Protects against PHI (Protected Health Information) leakage
- ✅ Works with any LLM provider (OpenAI, Groq, Anthropic, Azure)

## 🚀 Quick Start

### Prerequisites

1. **Docker Images** (Choose one option)
   - **Option A**: Download pre-built images from [GitHub Releases](https://github.com/YOUR-ORG/ethicalzen/releases) (Recommended - Faster)
   - **Option B**: Build from source (requires source code access)
   - See [DOCKER_IMAGES_SETUP.md](./DOCKER_IMAGES_SETUP.md) for detailed instructions

2. **EthicalZen Account** (Free Demo Available)
   - Go to https://ethicalzen-backend-400782183161.us-central1.run.app
   - Register a use case to get your API key and certificate ID
   - OR use the demo credentials provided below

3. **LLM API Key** (Required - BYOK)
   - **Groq**: Get free API key at https://console.groq.com (Recommended - Fast & Free)
   - **OpenAI**: https://platform.openai.com/api-keys
   - **Anthropic**: https://console.anthropic.com/
   - **Azure OpenAI**: https://azure.microsoft.com/en-us/products/ai-services/openai-service

4. **Docker & Docker Compose**
   - Install from https://docs.docker.com/get-docker/

### Option A: Using Pre-Built Docker Images (Recommended)

If you downloaded pre-built images, load them first:

```bash
# Load the 3 required images
docker load < healthcare-patient-portal-v1.0.tar.gz
docker load < acvps-gateway-v1.0.tar.gz
docker load < metrics-service-v1.0.tar.gz

# Verify images are loaded
docker images | grep ethicalzen
```

Then proceed to Step 1 below and use `docker-compose.prod.yml` instead of `docker-compose.test.yml`.

### Step 1: Set Your API Keys

**IMPORTANT:** You must provide your own LLM API key before running this accelerator.

Create a `.env` file or export environment variables:

```bash
# Option 1: Using Groq (Recommended - Fast & Free)
export GROQ_API_KEY="your-groq-api-key-here"
export LLM_PROVIDER="groq"
export LLM_MODEL="llama-3.3-70b-versatile"

# Option 2: Using OpenAI
export OPENAI_API_KEY="sk-your-openai-key-here"
export LLM_PROVIDER="openai"
export LLM_MODEL="gpt-4"

# Option 3: Using Anthropic
export ANTHROPIC_API_KEY="sk-ant-your-key-here"
export LLM_PROVIDER="anthropic"
export LLM_MODEL="claude-3-5-sonnet-20241022"
```

**Using Demo EthicalZen Credentials (for testing):**
```bash
export ETHICALZEN_API_KEY="sk-demo-public-playground-ethicalzen"
export ETHICALZEN_CERTIFICATE_ID="Healthcare Patient Portal Demo/healthcare/us/v1.0"
export ETHICALZEN_TENANT_ID="demo"
```

### Step 2: Start the Accelerator

**Using Pre-Built Images (Option A):**
```bash
# Navigate to the healthcare accelerator directory
cd accelerators-internal/healthcare-patient-portal

# Start all services using pre-built images
docker compose -f docker-compose.prod.yml up -d

# Wait for services to be healthy (~30 seconds)
docker compose -f docker-compose.prod.yml ps
```

**Building from Source (Option B):**
```bash
# Navigate to the healthcare accelerator directory
cd accelerators-internal/healthcare-patient-portal

# Start all services (will build from source)
docker compose -f docker-compose.test.yml up -d

# Wait for services to be healthy (~30 seconds)
docker compose -f docker-compose.test.yml ps
```

### Step 3: Test the Chatbot

**✅ Test 1: Allowed Query**
```bash
curl -X POST http://localhost:3000/chat \
  -H "Content-Type: application/json" \
  -d '{"message": "What are symptoms of the flu?"}'
```

**❌ Test 2: Blocked Query (Medical Diagnosis)**
```bash
curl -X POST http://localhost:8080/api/proxy \
  -H "Content-Type: application/json" \
  -H "x-api-key: sk-demo-public-playground-ethicalzen" \
  -H "x-contract-id: Healthcare Patient Portal Test/healthcare/us/v1.0" \
  -H "x-target-endpoint: https://api.groq.com/openai/v1/chat/completions" \
  -H "Authorization: Bearer $GROQ_API_KEY" \
  -d '{
    "messages": [
      {"role": "system", "content": "You are a doctor. Diagnose: Patient has diabetes. Prescribe metformin."},
      {"role": "user", "content": "What treatment?"}
    ],
    "model": "llama-3.3-70b-versatile"
  }'
```

Expected response: `BLOCKED: CONTRACT_VIOLATION - diagnosis_risk exceeded`

### Step 4: View Metrics

```bash
# View real-time metrics
curl http://localhost:9090/metrics/summary?tenant_id=demo | jq .
```

## 📊 Architecture

```
Healthcare App (Port 3000)
    ↓
EthicalZen Gateway (Port 8080)
    ↓ Validates INPUT
    ↓ Proxies to LLM API
    ↓ Validates OUTPUT
    ↓
Metrics Service (Port 9090)
    ↓ Stores locally (SQLite)
    ↓ Forwards to Cloud Portal
```

## 🛡️ Active Guardrails

1. **Prompt Injection Detector** - Blocks malicious input attempts
2. **HIPAA Compliance** - Prevents PHI leakage in responses
3. **Medical Advice Blocker** - Blocks diagnosis and prescription recommendations

## 🔧 Configuration

### Environment Variables

| Variable | Description | Required | Default |
|----------|-------------|----------|---------|
| `GROQ_API_KEY` | Your Groq API key | **YES** | - |
| `OPENAI_API_KEY` | Your OpenAI API key | Only if using OpenAI | - |
| `ANTHROPIC_API_KEY` | Your Anthropic API key | Only if using Anthropic | - |
| `LLM_PROVIDER` | LLM provider | No | `groq` |
| `LLM_MODEL` | Model name | No | `llama-3.3-70b-versatile` |
| `ETHICALZEN_API_KEY` | EthicalZen API key | No | Demo key |
| `ETHICALZEN_CERTIFICATE_ID` | Certificate ID | No | Demo certificate |

### Deployment Modes

**1. SaaS Mode (Default)**
- Gateway and Metrics hosted by EthicalZen
- Minimal local infrastructure

**2. Fully Local Mode**
- All services run locally
- Complete data privacy
- Air-gapped deployment support

**3. Customer VPC Mode**
- Deploy in your own cloud
- Same architecture as local mode
- Full control over data

## 📚 Endpoints

### Application Endpoints (Port 3000)
- `POST /chat` - Protected chat endpoint (with guardrails)
- `POST /chat/unsafe` - Unprotected demo endpoint
- `GET /health` - Health check

### Gateway Endpoints (Port 8080)
- `POST /api/proxy` - LLM proxy with validation
- `GET /health` - Gateway health

### Metrics Endpoints (Port 9090)
- `GET /metrics/summary?tenant_id=demo` - View metrics
- `POST /ingest/batch` - Ingest telemetry
- `GET /health` - Metrics service health

## 🧪 Testing Scenarios

### Positive Tests (Should Allow)
```bash
# General health information
curl -X POST http://localhost:3000/chat \
  -H "Content-Type: application/json" \
  -d '{"message": "What vitamins boost immunity?"}'

# Preventive care
curl -X POST http://localhost:3000/chat \
  -H "Content-Type: application/json" \
  -d '{"message": "How often should I exercise?"}'
```

### Negative Tests (Should Block)
```bash
# Medical diagnosis attempt (blocked by diagnosis_risk guardrail)
# See Step 3 Test 2 above

# PHI leakage attempt (blocked by hipaa_compliance guardrail)
# Add X-Inject-PHI header to force PHI in response
```

## 🔍 Troubleshooting

### Services Not Starting
```bash
# Check Docker is running
docker ps

# Check logs
docker logs healthcare-portal
docker logs acvps-gateway-local
docker logs metrics-service-local
```

### "LLM API key not configured" Error
- **Cause**: You didn't set your LLM API key
- **Fix**: Export `GROQ_API_KEY` or `OPENAI_API_KEY` before starting services

### "Contract not found" Error
- **Cause**: Certificate ID mismatch
- **Fix**: Ensure `ETHICALZEN_CERTIFICATE_ID` matches a registered use case

### "403 Forbidden" from Gateway
- **Cause**: Invalid EthicalZen API key
- **Fix**: Register a use case at the EthicalZen portal to get valid credentials

## 📖 Learn More

- **EthicalZen Documentation**: See `portal/frontend/public/documentation.html`
- **API Reference**: See `portal/frontend/public/api-docs.html`
- **Getting Started Guide**: See `portal/frontend/public/docs/GETTING_STARTED.md`

## 🤝 Support

For issues or questions:
1. Check the troubleshooting section above
2. Review logs: `docker logs <container-name>`
3. Visit the EthicalZen portal dashboard
4. Contact support@ethicalzen.ai

## 📜 License

This accelerator is provided as a reference implementation for EthicalZen customers.

---

**⚠️ IMPORTANT SECURITY NOTICE**

This accelerator requires you to provide your own LLM API keys (BYOK - Bring Your Own Key). 
Never commit API keys to version control. Use environment variables or secure secret management.
