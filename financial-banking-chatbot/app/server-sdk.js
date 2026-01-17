/**
 * Financial Banking Chatbot - Secure AI Assistant
 * Protected by EthicalZen guardrails for PCI-DSS compliance
 * 
 * Uses EthicalZen Proxy Client SDK for transparent protection
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

// =============================================================================
// EthicalZen Gateway Configuration
// =============================================================================

const GATEWAY_URL = process.env.ETHICALZEN_GATEWAY_URL || 'http://localhost:8080';
const CERTIFICATE_ID = process.env.ETHICALZEN_CERTIFICATE_ID || 'Financial Banking Chatbot/financial/us/v1.0';
const TENANT_ID = process.env.ETHICALZEN_TENANT_ID || 'demo';
const ETHICALZEN_API_KEY = process.env.ETHICALZEN_API_KEY || 'sk-demo-public-playground-ethicalzen';

// =============================================================================
// LLM Configuration
// =============================================================================

const LLM_PROVIDER = process.env.LLM_PROVIDER || 'groq';
const LLM_MODEL = process.env.LLM_MODEL || 'llama-3.3-70b-versatile';

function getLLMConfig() {
  switch (LLM_PROVIDER.toLowerCase()) {
    case 'openai':
      return {
        endpoint: 'https://api.openai.com/v1/chat/completions',
        apiKey: process.env.OPENAI_API_KEY,
      };
    case 'anthropic':
      return {
        endpoint: 'https://api.anthropic.com/v1/messages',
        apiKey: process.env.ANTHROPIC_API_KEY,
      };
    case 'groq':
      return {
        endpoint: 'https://api.groq.com/openai/v1/chat/completions',
        apiKey: process.env.GROQ_API_KEY,
      };
    case 'azure':
      return {
        endpoint: process.env.AZURE_OPENAI_ENDPOINT,
        apiKey: process.env.AZURE_OPENAI_API_KEY,
      };
    default:
      throw new Error(`Unsupported LLM provider: ${LLM_PROVIDER}`);
  }
}

// =============================================================================
// Initialize EthicalZen Proxy Client
// =============================================================================

const llmConfig = getLLMConfig();

const ezClient = new EthicalZenProxyClient({
  gatewayURL: GATEWAY_URL,
  certificateId: CERTIFICATE_ID,
  tenantId: TENANT_ID,
  apiKey: ETHICALZEN_API_KEY,
  timeout: 30000,
});

console.log(`✅ EthicalZen Proxy Client initialized`);
console.log(`   Mode: ${TENANT_ID === 'demo' ? '🎮 DEMO (public playground)' : '🚀 PRODUCTION'}`);
console.log(`   Gateway: ${GATEWAY_URL}`);
console.log(`   Certificate: ${CERTIFICATE_ID}`);
console.log(`   Tenant: ${TENANT_ID}`);
console.log(`   LLM Provider: ${LLM_PROVIDER}`);

// =============================================================================
// Middleware
// =============================================================================

app.use(helmet());
app.use(cors());
app.use(express.json());
app.use(express.urlencoded({ extended: true }));

// Rate limiting
const limiter = rateLimit({
  windowMs: 1 * 60 * 1000, // 1 minute
  max: parseInt(process.env.MAX_REQUESTS_PER_MINUTE) || 60,
  message: 'Too many requests from this IP, please try again later.',
});
app.use('/chat', limiter);

// =============================================================================
// Routes
// =============================================================================

// Health check
app.get('/health', (req, res) => {
  res.json({
    status: 'healthy',
    timestamp: new Date().toISOString(),
    version: require('../package.json').version,
    ethicalzen: {
      mode: 'proxy-client-sdk',
      gateway: GATEWAY_URL,
      certificate: CERTIFICATE_ID,
      tenant: TENANT_ID,
    },
    llm: {
      provider: LLM_PROVIDER,
      model: LLM_MODEL,
    },
  });
});

// Main chat endpoint (protected by EthicalZen transparently via SDK)
app.post('/chat', async (req, res) => {
  try {
    const { message, user_id, session_id } = req.body;

    if (!message) {
      return res.status(400).json({
        error: 'BAD_REQUEST',
        message: 'Message is required',
      });
    }

    console.log(`[${new Date().toISOString()}] Chat request from user ${user_id || 'anonymous'}`);
    console.log(`  Input: "${message.substring(0, 100)}..."`);

    const startTime = Date.now();

    // Make a normal API call to the REAL LLM endpoint
    // SDK automatically:
    // 1. Packages certificate in headers
    // 2. Routes through EthicalZen gateway
    // 3. Gateway validates input (PCI compliance, financial advice, etc.)
    // 4. Gateway forwards to real LLM endpoint (with LLM API key)
    // 5. Gateway validates output (credit card numbers, account info, etc.)
    // 6. Gateway returns validated response
    const response = await ezClient.post(
      llmConfig.endpoint,
      {
        messages: [
          {
            role: 'system',
            content: `You are a helpful banking assistant. Provide general information about banking services, account types, and financial products. 

IMPORTANT SECURITY RULES:
- NEVER share or generate credit card numbers, CVV codes, or account numbers
- NEVER provide specific investment advice or stock recommendations
- NEVER help with money laundering or fraudulent activities
- Always recommend users contact their bank directly for account-specific issues
- Provide general educational content only`,
          },
          { role: 'user', content: message },
        ],
        model: LLM_MODEL,
        max_tokens: 500,
      },
      {
        headers: {
          // LLM API key - forwarded by gateway to the target LLM
          'Authorization': `Bearer ${llmConfig.apiKey}`,
        },
      }
    );

    const latency = Date.now() - startTime;

    // Extract content based on OpenAI-compatible format
    let responseContent = '';
    if (response.data.choices && response.data.choices.length > 0 && response.data.choices[0].message && response.data.choices[0].message.content) {
      responseContent = response.data.choices[0].message.content;
    } else if (response.data.response) {
      responseContent = response.data.response;
    } else {
      console.warn('Could not extract LLM response content from gateway response:', response.data);
      responseContent = 'An unexpected response format was received from the LLM.';
    }

    console.log(`  ✅ Response validated in ${latency}ms`);
    console.log(`  Guardrails: ${response.data.guardrails_checked || []}`);

    res.json({
      response: responseContent,
      ethicalzen: {
        status: response.data.status || 'APPROVED',
        guardrails_checked: response.data.guardrails_checked || [],
        validation_time_ms: response.data.validation_time_ms || latency,
        trace_id: response.data.trace_id,
        gateway_validated: true,
        certificate: CERTIFICATE_ID,
      },
      metadata: {
        latency_ms: latency,
        timestamp: new Date().toISOString(),
        model: LLM_MODEL,
        usage: response.data.usage,
      },
    });
  } catch (error) {
    console.error(`  ❌ Error: ${error.message}`);

    // EthicalZen blocked the request (403 from gateway)
    if (error.response && error.response.status === 403) {
      return res.status(403).json({
        error: 'BLOCKED_BY_ETHICALZEN',
        message: 'Request blocked by EthicalZen security policy',
        details: error.response.data,
        status: 'BLOCKED',
        blocked: true,
      });
    }

    // Other errors
    res.status(500).json({
      error: 'INTERNAL_ERROR',
      message: 'An error occurred processing your request',
      details: process.env.NODE_ENV === 'development' ? error.message : undefined,
    });
  }
});

// Demo endpoint (unprotected, for comparison - BYPASSES gateway)
app.post('/chat/unsafe', async (req, res) => {
  try {
    const { message } = req.body;

    if (!message) {
      return res.status(400).json({
        error: 'BAD_REQUEST',
        message: 'Message is required',
      });
    }

    console.log(`[${new Date().toISOString()}] UNSAFE chat request (NO GUARDRAILS)`);

    const axios = require('axios');
    const startTime = Date.now();

    // Direct call to LLM - NO PROTECTION
    const llmResponse = await axios.post(
      llmConfig.endpoint,
      {
        messages: [
          {
            role: 'system',
            content: 'You are a helpful banking assistant. Provide information about banking services. You are NOT protected by EthicalZen guardrails.',
          },
          { role: 'user', content: message },
        ],
        model: LLM_MODEL,
        max_tokens: 500,
      },
      {
        headers: {
          'Authorization': `Bearer ${llmConfig.apiKey}`,
          'Content-Type': 'application/json',
        },
      }
    );

    const responseContent = llmResponse.data.choices[0].message.content;
    const latency = Date.now() - startTime;

    console.log(`  ✅ Unsafe response received in ${latency}ms`);

    res.json({
      response: responseContent,
      warning: '⚠️ This endpoint bypasses EthicalZen security. DO NOT use in production!',
      metadata: {
        latency_ms: latency,
        timestamp: new Date().toISOString(),
        model: LLM_MODEL,
        usage: llmResponse.data.usage,
      },
    });
  } catch (error) {
    console.error(`  ❌ Unsafe chat error: ${error.message}`);
    res.status(500).json({
      error: 'INTERNAL_ERROR',
      message: 'An error occurred processing your unsafe request',
      details: process.env.NODE_ENV === 'development' ? error.message : undefined,
    });
  }
});

// =============================================================================
// Server Startup
// =============================================================================

async function startServer() {
  try {
    // Verify EthicalZen Gateway connection
    await ezClient.getGatewayURL();
    console.log('✅ EthicalZen Proxy Client SDK initialized and ready to connect to Gateway.');

    // Verify LLM API key
    if (!llmConfig.apiKey) {
      throw new Error(`LLM API key not configured for ${LLM_PROVIDER}. Please set ${LLM_PROVIDER.toUpperCase()}_API_KEY`);
    }
    console.log(`✅ LLM provider: ${LLM_PROVIDER}`);

    app.listen(PORT, () => {
      console.log('\n' + '='.repeat(60));
      console.log('🏦 Financial Banking Chatbot - EthicalZen Proxy Client SDK');
      console.log('='.repeat(60));
      console.log(`📍 Server: http://localhost:${PORT}`);
      console.log(`🛡️  Protected by EthicalZen (via Proxy Client SDK)`);
      console.log(`🔑 BYOK: ${LLM_PROVIDER}`);
      console.log(`📜 Certificate: ${CERTIFICATE_ID}`);
      console.log('='.repeat(60));
      console.log('\n📖 Endpoints:');
      console.log('  POST /chat          - Secure chat (EthicalZen protected)');
      console.log('  POST /chat/unsafe   - Demo endpoint (NO protection)');
      console.log('  GET  /health        - Health check');
      console.log('\n💡 Try it:');
      console.log('  curl -X POST http://localhost:3000/chat \\');
      console.log('    -H "Content-Type: application/json" \\');
      console.log('    -d \'{"message": "What types of savings accounts do you offer?"}\'');
      console.log('\n');
    });
  } catch (error) {
    console.error(`❌ Failed to start server: ${error.message}`);
    console.log('\n🔍 Troubleshooting:');
    console.log(`  1. Check your .env file has ETHICALZEN_API_KEY and ${LLM_PROVIDER.toUpperCase()}_API_KEY`);
    console.log(`  2. Verify EthicalZen Gateway is running at ${GATEWAY_URL}`);
    console.log(`  3. Verify certificate "${CERTIFICATE_ID}" is registered in the EthicalZen Portal`);
    process.exit(1);
  }
}

// Graceful shutdown
process.on('SIGTERM', () => {
  console.log('SIGTERM received, shutting down gracefully');
  process.exit(0);
});

process.on('SIGINT', () => {
  console.log('\nSIGINT received, shutting down gracefully');
  process.exit(0);
});

startServer();

module.exports = app; // For testing

