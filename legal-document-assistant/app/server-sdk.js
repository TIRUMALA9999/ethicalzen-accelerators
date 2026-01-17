/**
 * Legal Document Assistant - Secure AI Assistant
 * Protected by EthicalZen guardrails for legal compliance
 */

const { EthicalZenProxyClient } = require('@ethicalzen/sdk');
const express = require('express');
const cors = require('cors');
const dotenv = require('dotenv');
const helmet = require('helmet');
const rateLimit = require('express-rate-limit');

dotenv.config();

const app = express();
const PORT = process.env.PORT || 3000;

const GATEWAY_URL = process.env.ETHICALZEN_GATEWAY_URL || 'http://localhost:8080';
const CERTIFICATE_ID = process.env.ETHICALZEN_CERTIFICATE_ID || 'Legal Document Assistant/legal/us/v1.0';
const TENANT_ID = process.env.ETHICALZEN_TENANT_ID || 'demo';
const ETHICALZEN_API_KEY = process.env.ETHICALZEN_API_KEY || 'sk-demo-public-playground-ethicalzen';

const LLM_PROVIDER = process.env.LLM_PROVIDER || 'groq';
const LLM_MODEL = process.env.LLM_MODEL || 'llama-3.3-70b-versatile';

function getLLMConfig() {
  switch (LLM_PROVIDER.toLowerCase()) {
    case 'openai':
      return { endpoint: 'https://api.openai.com/v1/chat/completions', apiKey: process.env.OPENAI_API_KEY };
    case 'anthropic':
      return { endpoint: 'https://api.anthropic.com/v1/messages', apiKey: process.env.ANTHROPIC_API_KEY };
    case 'groq':
      return { endpoint: 'https://api.groq.com/openai/v1/chat/completions', apiKey: process.env.GROQ_API_KEY };
    default:
      throw new Error(`Unsupported LLM provider: ${LLM_PROVIDER}`);
  }
}

const llmConfig = getLLMConfig();

const ezClient = new EthicalZenProxyClient({
  gatewayURL: GATEWAY_URL,
  certificateId: CERTIFICATE_ID,
  tenantId: TENANT_ID,
  apiKey: ETHICALZEN_API_KEY,
  timeout: 30000,
});

console.log(`✅ EthicalZen Proxy Client initialized`);
console.log(`   Gateway: ${GATEWAY_URL}`);
console.log(`   Certificate: ${CERTIFICATE_ID}`);
console.log(`   LLM Provider: ${LLM_PROVIDER}`);

app.use(helmet());
app.use(cors());
app.use(express.json());

const limiter = rateLimit({
  windowMs: 1 * 60 * 1000,
  max: 60,
  message: 'Too many requests, please try again later.',
});
app.use('/chat', limiter);

app.get('/health', (req, res) => {
  res.json({
    status: 'healthy',
    timestamp: new Date().toISOString(),
    ethicalzen: { mode: 'proxy-client-sdk', gateway: GATEWAY_URL, certificate: CERTIFICATE_ID, tenant: TENANT_ID },
    llm: { provider: LLM_PROVIDER, model: LLM_MODEL },
  });
});

app.post('/chat', async (req, res) => {
  try {
    const { message, user_id } = req.body;

    if (!message) {
      return res.status(400).json({ error: 'BAD_REQUEST', message: 'Message is required' });
    }

    console.log(`[${new Date().toISOString()}] Chat request from user ${user_id || 'anonymous'}`);
    const startTime = Date.now();

    const response = await ezClient.post(
      llmConfig.endpoint,
      {
        messages: [
          {
            role: 'system',
            content: `You are a helpful legal document assistant. Provide general information about legal concepts and document preparation.

IMPORTANT RULES:
- NEVER provide specific legal advice or opinions on ongoing cases
- NEVER draft binding legal documents without proper disclaimers
- Always recommend users consult with a licensed attorney
- Provide general educational content about legal processes
- Help with document organization and general formatting guidance`,
          },
          { role: 'user', content: message },
        ],
        model: LLM_MODEL,
        max_tokens: 500,
      },
      { headers: { 'Authorization': `Bearer ${llmConfig.apiKey}` } }
    );

    const latency = Date.now() - startTime;
    let responseContent = '';
    if (response.data.choices?.[0]?.message?.content) {
      responseContent = response.data.choices[0].message.content;
    } else if (response.data.response) {
      responseContent = response.data.response;
    }

    console.log(`  ✅ Response validated in ${latency}ms`);

    res.json({
      response: responseContent,
      ethicalzen: {
        status: response.data.status || 'APPROVED',
        gateway_validated: true,
        certificate: CERTIFICATE_ID,
      },
      metadata: { latency_ms: latency, timestamp: new Date().toISOString(), model: LLM_MODEL, usage: response.data.usage },
    });
  } catch (error) {
    console.error(`  ❌ Error: ${error.message}`);
    if (error.response?.status === 403) {
      return res.status(403).json({
        error: 'BLOCKED_BY_ETHICALZEN',
        message: 'Request blocked by EthicalZen security policy',
        details: error.response.data,
        status: 'BLOCKED',
        blocked: true,
      });
    }
    res.status(500).json({ error: 'INTERNAL_ERROR', message: 'An error occurred processing your request' });
  }
});

async function startServer() {
  try {
    if (!llmConfig.apiKey) {
      throw new Error(`LLM API key not configured for ${LLM_PROVIDER}`);
    }
    app.listen(PORT, () => {
      console.log('\n' + '='.repeat(60));
      console.log('⚖️ Legal Document Assistant - EthicalZen Protected');
      console.log('='.repeat(60));
      console.log(`📍 Server: http://localhost:${PORT}`);
      console.log(`🛡️  Protected by EthicalZen`);
      console.log(`🔑 BYOK: ${LLM_PROVIDER}`);
      console.log('='.repeat(60) + '\n');
    });
  } catch (error) {
    console.error(`❌ Failed to start server: ${error.message}`);
    process.exit(1);
  }
}

startServer();
module.exports = app;

