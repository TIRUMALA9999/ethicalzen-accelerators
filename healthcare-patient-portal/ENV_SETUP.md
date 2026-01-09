# Environment Setup Guide

## Required Environment Variables

Before running the Healthcare Patient Portal accelerator, you **MUST** provide your own LLM API key.

### Step 1: Get an LLM API Key (BYOK - Bring Your Own Key)

Choose one of the following providers:

#### Option A: Groq (Recommended - Fast & Free)
1. Go to https://console.groq.com
2. Sign up for a free account
3. Navigate to API Keys
4. Create a new API key
5. Copy your key (starts with `gsk_...`)

#### Option B: OpenAI
1. Go to https://platform.openai.com/api-keys
2. Sign in or create an account
3. Create a new API key
4. Copy your key (starts with `sk-...`)

#### Option C: Anthropic
1. Go to https://console.anthropic.com/
2. Sign up for an account
3. Navigate to API Keys
4. Create a new key
5. Copy your key (starts with `sk-ant-...`)

### Step 2: Set Environment Variables

Create a `.env` file in this directory with the following content:

```bash
# ============================================================================
# REQUIRED: LLM Provider API Key
# ============================================================================
# You MUST provide at least one of these API keys

# Option 1: Groq (Recommended)
GROQ_API_KEY=your-groq-api-key-here
LLM_PROVIDER=groq
LLM_MODEL=llama-3.3-70b-versatile

# Option 2: OpenAI (Comment out Groq above and uncomment below)
# OPENAI_API_KEY=sk-your-openai-key-here
# LLM_PROVIDER=openai
# LLM_MODEL=gpt-4

# Option 3: Anthropic (Comment out Groq above and uncomment below)
# ANTHROPIC_API_KEY=sk-ant-your-key-here
# LLM_PROVIDER=anthropic
# LLM_MODEL=claude-3-5-sonnet-20241022

# ============================================================================
# EthicalZen Configuration (Demo credentials - works out of the box)
# ============================================================================
ETHICALZEN_API_KEY=sk-demo-public-playground-ethicalzen
ETHICALZEN_CERTIFICATE_ID=Healthcare Patient Portal Test/healthcare/us/v1.0
ETHICALZEN_TENANT_ID=demo
ETHICALZEN_PORTAL_URL=https://api.ethicalzen.ai

# ============================================================================
# Application Settings (Optional - defaults provided)
# ============================================================================
PORT=3000
NODE_ENV=development
USE_CASE=patient-portal
INDUSTRY=healthcare
REGION=us
DATA_SENSITIVITY=PHI,PII
```

### Step 3: Alternative - Export Variables Directly

Instead of creating a `.env` file, you can export environment variables directly:

```bash
# Using Groq (Recommended)
export GROQ_API_KEY="your-groq-api-key-here"
export LLM_PROVIDER="groq"
export LLM_MODEL="llama-3.3-70b-versatile"

# Using demo EthicalZen credentials
export ETHICALZEN_API_KEY="sk-demo-public-playground-ethicalzen"
export ETHICALZEN_CERTIFICATE_ID="Healthcare Patient Portal Test/healthcare/us/v1.0"
export ETHICALZEN_TENANT_ID="demo"
```

### Step 4: Verify Setup

Test that your environment is configured correctly:

```bash
# Check if variables are set
echo $GROQ_API_KEY
echo $LLM_PROVIDER

# If they print your values, you're ready to go!
```

## ⚠️ SECURITY WARNING

**NEVER commit API keys to version control!**

- The `.env` file is already in `.gitignore`
- Never hardcode API keys in source code
- Use environment variables or secret management systems
- Rotate keys regularly
- Use separate keys for development and production

## Production Setup

For production deployments:

1. **Register your own EthicalZen account**
   - Go to https://api.ethicalzen.ai
   - Register a use case for your application
   - Get your production API key and certificate ID

2. **Use a secure secret manager**
   - AWS Secrets Manager
   - Google Cloud Secret Manager
   - Azure Key Vault
   - HashiCorp Vault

3. **Set production environment variables**
   ```bash
   ETHICALZEN_API_KEY=sk-prod-your-real-key
   ETHICALZEN_CERTIFICATE_ID=your-registered-certificate-id
   ETHICALZEN_TENANT_ID=your-tenant-id
   GROQ_API_KEY=your-production-groq-key
   NODE_ENV=production
   ```

## Troubleshooting

### Error: "LLM API key not configured"
**Cause**: You didn't set your LLM API key  
**Fix**: Export `GROQ_API_KEY` or create a `.env` file with your key

### Error: "Request failed with status code 401"
**Cause**: Invalid or expired API key  
**Fix**: Verify your API key is correct and hasn't been revoked

### Error: "Contract not found"
**Cause**: Certificate ID doesn't match a registered use case  
**Fix**: Use the demo certificate ID or register a new use case

## Next Steps

Once your environment is configured:

1. Start the accelerator:
   ```bash
   docker compose -f docker-compose.test.yml up -d
   ```

2. Test the chatbot:
   ```bash
   curl -X POST http://localhost:3000/chat \
     -H "Content-Type: application/json" \
     -d '{"message": "What are symptoms of the flu?"}'
   ```

3. View metrics:
   ```bash
   curl http://localhost:9090/metrics/summary?tenant_id=demo | jq .
   ```

See `README.md` for complete documentation.

