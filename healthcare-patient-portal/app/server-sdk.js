/**
 * Healthcare Patient Portal - Using EthicalZen Proxy Client SDK
 * 
 * This version demonstrates the simplest integration pattern:
 * 1. Initialize EthicalZenProxyClient with gateway URL and certificate
 * 2. Make normal API calls to real URLs (e.g., https://api.openai.com/...)
 * 3. SDK automatically packages certificate and routes through gateway
 * 4. Gateway intercepts, validates input, forwards to target
 * 5. Gateway intercepts response, validates output, returns to client
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

// ============================================================================
// EthicalZen Configuration
// ============================================================================
// Demo Mode (default): Uses public demo tenant for easy testing
// Production Mode: Set your own ETHICALZEN_TENANT_ID and ETHICALZEN_API_KEY
// 
// To get your own tenant keys:
//   1. Visit https://ethicalzen.ai/portal
//   2. Register your organization
//   3. Create a new use case → get your certificate ID
//   4. Generate API keys in the dashboard
// ============================================================================

const GATEWAY_URL = process.env.ETHICALZEN_GATEWAY_URL || 'http://localhost:8080';
const CERTIFICATE_ID = process.env.ETHICALZEN_CERTIFICATE_ID || 'healthcare-sdk-test/healthcare/us/v1.0';

// LLM Configuration (BYOK - Bring Your Own Key)
const LLM_PROVIDER = process.env.LLM_PROVIDER || 'groq';
const LLM_MODEL = process.env.LLM_MODEL || 'llama-3.3-70b-versatile';

// Helper to get LLM API endpoint and key
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

// Initialize EthicalZen Proxy Client once
const llmConfig = getLLMConfig();

// Demo tenant keys (work out of the box for demos)
// For production: set ETHICALZEN_API_KEY and ETHICALZEN_TENANT_ID
const ETHICALZEN_API_KEY = process.env.ETHICALZEN_API_KEY || 'sk-demo-public-playground-ethicalzen';
const TENANT_ID = process.env.ETHICALZEN_TENANT_ID || 'demo';

const ezClient = new EthicalZenProxyClient({
  gatewayURL: GATEWAY_URL,
  certificateId: CERTIFICATE_ID,
  tenantId: TENANT_ID,
  apiKey: ETHICALZEN_API_KEY,
  timeout: 30000,
});

const isDemo = TENANT_ID === 'demo';
console.log(`✅ EthicalZen Proxy Client initialized`);
console.log(`   Mode: ${isDemo ? '🎮 DEMO (public playground)' : '🔐 PRODUCTION'}`);
console.log(`   Gateway: ${GATEWAY_URL}`);
console.log(`   Certificate: ${CERTIFICATE_ID}`);
console.log(`   Tenant: ${TENANT_ID}`);
if (isDemo) {
  console.log(`   💡 Using demo keys - register at https://ethicalzen.ai for production keys`);
}
console.log(`   LLM Provider: ${LLM_PROVIDER}`);

// Middleware
app.use(cors());
app.use(helmet());
app.use(express.json());
app.use(express.urlencoded({ extended: true }));

// Rate limiting
const limiter = rateLimit({
  windowMs: 15 * 60 * 1000,
  max: 100,
  message: 'Too many requests from this IP, please try again after 15 minutes',
});
app.use(limiter);

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
    // 3. Gateway validates input (prompt injection, PHI, etc.)
    // 4. Gateway forwards to real LLM endpoint (with LLM API key)
    // 5. Gateway validates output (diagnosis risk, HIPAA compliance)
    // 6. Gateway returns validated response
    const response = await ezClient.post(
      llmConfig.endpoint,
      {
        messages: [
          {
            role: 'system',
            content: 'You are a helpful healthcare assistant. Provide information about general health topics.',
          },
          { role: 'user', content: message },
        ],
        model: LLM_MODEL,
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
    if (response.data && response.data.choices && response.data.choices.length > 0) {
      responseContent = response.data.choices[0].message.content;
    } else if (response.data && response.data.response) {
      responseContent = response.data.response;
    } else {
      console.warn('Could not extract LLM response content:', response.data);
      responseContent = 'An unexpected response format was received.';
    }

    console.log(`  ✅ Response validated in ${latency}ms`);

    res.json({
      response: responseContent,
      ethicalzen: {
        status: 'APPROVED',
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

    // EthicalZen blocked the request (403 or 409 from gateway)
    if (error.response && (error.response.status === 403 || error.response.status === 409)) {
      const blockDetails = error.response.data;
      console.log(`  🛡️  BLOCKED by EthicalZen: ${JSON.stringify(blockDetails)}`);
      
      return res.status(403).json({
        error: 'BLOCKED_BY_ETHICALZEN',
        message: 'Request blocked by EthicalZen security policy',
        ethicalzen: {
          status: 'BLOCKED',
          violation: blockDetails.violation || blockDetails.error,
          feature: blockDetails.feature,
          bounds: blockDetails.bounds,
          certificate: CERTIFICATE_ID,
        },
        details: blockDetails,
      });
    }

    // Check for validation errors (422)
    if (error.response && error.response.status === 422) {
      return res.status(422).json({
        error: 'VALIDATION_FAILED',
        message: 'EthicalZen guardrail validation failed',
        details: error.response.data,
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
            content: 'You are a helpful healthcare assistant. Provide information about general health topics.',
          },
          { role: 'user', content: message },
        ],
        model: LLM_MODEL,
      },
      {
        headers: {
          Authorization: `Bearer ${llmConfig.apiKey}`,
          'Content-Type': 'application/json',
        },
      }
    );

    const latency = Date.now() - startTime;
    const responseContent = llmResponse.data.choices[0].message.content;

    console.log(`  ⚠️  UNPROTECTED response in ${latency}ms`);

    res.json({
      response: responseContent,
      ethicalzen: {
        status: 'BYPASSED',
        warning: 'This endpoint has NO guardrails - for demo purposes only',
      },
      metadata: {
        latency_ms: latency,
        timestamp: new Date().toISOString(),
        model: LLM_MODEL,
      },
    });
  } catch (error) {
    console.error(`  ❌ Error: ${error.message}`);
    res.status(500).json({
      error: 'INTERNAL_ERROR',
      message: 'An error occurred processing your request',
    });
  }
});

// Start server
async function startServer() {
  try {
    // Verify LLM API key
    if (!llmConfig.apiKey) {
      throw new Error(`LLM API key not configured for ${LLM_PROVIDER}`);
    }

    app.listen(PORT, () => {
      console.log('\n' + '='.repeat(70));
      console.log('🏥 Healthcare Patient Portal - EthicalZen Proxy Client SDK');
      console.log('='.repeat(70));
      console.log(`📍 Server: http://localhost:${PORT}`);
      console.log(`🛡️  Protected by EthicalZen (via Proxy Client SDK)`);
      console.log(`🔑 BYOK: ${LLM_PROVIDER}`);
      console.log(`📜 Certificate: ${CERTIFICATE_ID}`);
      console.log('='.repeat(70));
      console.log('\n📖 Endpoints:');
      console.log('  POST /chat          - Secure chat (EthicalZen protected)');
      console.log('  POST /chat/unsafe   - Demo endpoint (NO protection)');
      console.log('  GET  /health        - Health check');
      console.log('\n💡 Try it:');
      console.log('  curl -X POST http://localhost:3000/chat \\');
      console.log('    -H "Content-Type: application/json" \\');
      console.log('    -d \'{"message": "What are symptoms of a cold?"}\'');
      console.log('\n');
    });
  } catch (error) {
    console.error(`❌ Failed to start server: ${error.message}`);
    console.log('\n🔍 Troubleshooting:');
    console.log(`  1. Check GROQ_API_KEY (or other LLM key) is set`);
    console.log(`  2. Verify EthicalZen Gateway is running at ${GATEWAY_URL}`);
    console.log(`  3. Verify certificate "${CERTIFICATE_ID}" is registered`);
    process.exit(1);
  }
}

startServer();
