/**
 * Financial Banking Chatbot - Secure AI Assistant
 * Protected by EthicalZen guardrails for PCI-DSS compliance
 */

require('dotenv').config();
const express = require('express');
const cors = require('cors');
const helmet = require('helmet');
const rateLimit = require('express-rate-limit');
const axios = require('axios');

const app = express();
const PORT = process.env.PORT || 3000;

// =============================================================================
// Middleware
// =============================================================================

app.use(helmet());
app.use(cors());
app.use(express.json());

// Rate limiting
const limiter = rateLimit({
  windowMs: 1 * 60 * 1000, // 1 minute
  max: parseInt(process.env.MAX_REQUESTS_PER_MINUTE) || 60,
  message: 'Too many requests from this IP, please try again later.'
});
app.use('/chat', limiter);

// =============================================================================
// EthicalZen Gateway Configuration
// =============================================================================

const GATEWAY_URL = process.env.ETHICALZEN_GATEWAY_URL || 'http://gateway:8080';
const ETHICALZEN_API_KEY = process.env.ETHICALZEN_API_KEY;
const CERTIFICATE_ID = process.env.ETHICALZEN_CERTIFICATE_ID;

function getLLMApiKey() {
  const provider = process.env.LLM_PROVIDER || 'openai';
  switch (provider.toLowerCase()) {
    case 'openai':
      return process.env.OPENAI_API_KEY;
    case 'anthropic':
      return process.env.ANTHROPIC_API_KEY;
    case 'groq':
      return process.env.GROQ_API_KEY;
    case 'azure':
      return process.env.AZURE_OPENAI_API_KEY;
    default:
      throw new Error(`Unsupported LLM provider: ${provider}`);
  }
}

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
      connected: !!process.env.ETHICALZEN_API_KEY,
      gateway: process.env.ETHICALZEN_GATEWAY_URL || 'https://gateway.ethicalzen.ai'
    },
    llm: {
      provider: process.env.LLM_PROVIDER || 'openai',
      model: process.env.LLM_MODEL || 'gpt-4'
    }
  });
});

// Main chat endpoint (protected by EthicalZen)
app.post('/chat', async (req, res) => {
  try {
    const { message, user_id, session_id } = req.body;

    if (!message) {
      return res.status(400).json({
        error: 'BAD_REQUEST',
        message: 'Message is required'
      });
    }

    console.log(`[${new Date().toISOString()}] Chat request from user ${user_id || 'anonymous'}`);
    console.log(`  Input: "${message.substring(0, 100)}..."`);

    const startTime = Date.now();

    // Call LLM through EthicalZen Gateway
    // This automatically:
    // 1. Validates INPUT for prompt injection, PHI leakage attempts
    // 2. Calls your LLM
    // 3. Validates OUTPUT for HIPAA compliance, medical advice
    // Determine LLM endpoint based on provider
    const llmProvider = process.env.LLM_PROVIDER || 'openai';
    let targetEndpoint;
    switch (llmProvider.toLowerCase()) {
      case 'openai':
        targetEndpoint = 'https://api.openai.com/v1/chat/completions';
        break;
      case 'groq':
        targetEndpoint = 'https://api.groq.com/openai/v1/chat/completions';
        break;
      case 'anthropic':
        targetEndpoint = 'https://api.anthropic.com/v1/messages';
        break;
      default:
        targetEndpoint = 'https://api.openai.com/v1/chat/completions';
    }

    const gatewayResponse = await axios.post(`${GATEWAY_URL}/api/proxy`, {
      messages: [
        {
          role: 'system',
          content: 'You are a helpful banking assistant. Provide general information about banking services, account types, and financial products. Never provide specific financial advice or share account numbers.'
        },
        {
          role: 'user',
          content: message
        }
      ],
      model: process.env.LLM_MODEL || 'gpt-4',
      max_tokens: 500
    }, {
      headers: {
        'x-api-key': ETHICALZEN_API_KEY,
        'x-contract-id': CERTIFICATE_ID,
        'x-target-endpoint': targetEndpoint,
        'Authorization': `Bearer ${getLLMApiKey()}`,
        'Content-Type': 'application/json'
      }
    });

    const result = gatewayResponse.data;
    const latency = Date.now() - startTime;

    // Extract LLM response from OpenAI-compatible format
    let llmResponse = '';
    if (result.choices && result.choices.length > 0) {
      llmResponse = result.choices[0].message.content;
    } else if (result.response) {
      llmResponse = result.response;
    } else if (result.content) {
      llmResponse = result.content;
    }

    console.log(`  ✅ Response validated in ${latency}ms`);
    console.log(`  Response preview: ${llmResponse.substring(0, 100)}...`);

    res.json({
      response: llmResponse,
      ethicalzen: {
        status: 'APPROVED',
        guardrails_checked: ['prompt_injection_detector', 'hipaa_compliance', 'medical_advice_blocker'],
        validation_time_ms: latency
      },
      metadata: {
        latency_ms: latency,
        timestamp: new Date().toISOString(),
        model: result.model || process.env.LLM_MODEL,
        usage: result.usage || {}
      }
    });

  } catch (error) {
    console.error(`  ❌ Error: ${error.message}`);

    // EthicalZen blocked the request
    if (error.response && error.response.status === 403) {
      return res.status(403).json({
        error: 'INPUT_BLOCKED',
        message: 'Request blocked by security policy',
        details: error.response.data
      });
    }

    // Other errors
    res.status(500).json({
      error: 'INTERNAL_ERROR',
      message: 'An error occurred processing your request',
      details: process.env.NODE_ENV === 'development' ? error.message : undefined
    });
  }
});

// Demo endpoint (unprotected, for comparison)
app.post('/chat/unsafe', async (req, res) => {
  try {
    const { message } = req.body;

    console.log(`[UNSAFE] Processing: "${message}"`);

    // Call LLM directly WITHOUT EthicalZen protection
    // ⚠️ DO NOT USE IN PRODUCTION - This is for demo purposes only
    const llmResponse = await callLLMDirectly(message);

    res.json({
      response: llmResponse,
      warning: '⚠️ This endpoint bypasses EthicalZen security. DO NOT use in production!'
    });

  } catch (error) {
    res.status(500).json({ error: error.message });
  }
});

// =============================================================================
// Helper Functions
// =============================================================================

async function callLLMDirectly(message) {
  const provider = process.env.LLM_PROVIDER || 'openai';
  
  if (provider === 'openai') {
    const response = await fetch('https://api.openai.com/v1/chat/completions', {
      method: 'POST',
      headers: {
        'Content-Type': 'application/json',
        'Authorization': `Bearer ${process.env.OPENAI_API_KEY}`
      },
      body: JSON.stringify({
        model: process.env.LLM_MODEL || 'gpt-4',
        messages: [
          {
            role: 'system',
            content: 'You are a helpful banking assistant. Provide general information about banking services, account types, and financial products. Never provide specific financial advice or share account numbers.'
          },
          { role: 'user', content: message }
        ]
      })
    });

    const data = await response.json();
    return data.choices[0].message.content;
  }

  throw new Error(`Direct LLM calling not implemented for provider: ${provider}`);
}

// =============================================================================
// Server Startup
// =============================================================================

async function startServer() {
  try {
    // Verify EthicalZen connection
    try {
      await axios.get(`${GATEWAY_URL}/health`);
      console.log('✅ EthicalZen Gateway connection verified');
    } catch (err) {
      console.warn('⚠️  Gateway not responding yet, will retry on first request');
    }

    // Verify LLM API key
    if (!getLLMApiKey()) {
      throw new Error('LLM API key not configured. Please set OPENAI_API_KEY, ANTHROPIC_API_KEY, or GROQ_API_KEY');
    }
    console.log(`✅ LLM provider: ${process.env.LLM_PROVIDER || 'openai'}`);

    // Start HTTP server
    app.listen(PORT, () => {
      console.log('\n' + '='.repeat(60));
      console.log('🏥 Healthcare Patient Portal - READY');
      console.log('='.repeat(60));
      console.log(`📍 Server: http://localhost:${PORT}`);
      console.log(`🛡️  Protected by EthicalZen guardrails`);
      console.log(`🔑 BYOK: ${process.env.LLM_PROVIDER || 'openai'}`);
      console.log('='.repeat(60));
      console.log('\n📖 Endpoints:');
      console.log(`  POST /chat          - Secure chat (EthicalZen protected)`);
      console.log(`  POST /chat/unsafe   - Demo endpoint (NO protection)`);
      console.log(`  GET  /health        - Health check`);
      console.log('\n💡 Try it:');
      console.log(`  curl -X POST http://localhost:${PORT}/chat \\`);
      console.log(`    -H "Content-Type: application/json" \\`);
      console.log(`    -d '{"message": "What are symptoms of a cold?"}'`);
      console.log('\n');
    });

  } catch (error) {
    console.error('❌ Failed to start server:', error.message);
    console.error('\n🔍 Troubleshooting:');
    console.error('  1. Check your .env file has ETHICALZEN_API_KEY');
    console.error('  2. Check your .env file has OPENAI_API_KEY (or other LLM key)');
    console.error('  3. Verify keys are valid at https://ethicalzen.ai/dashboard');
    console.error('\n');
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

