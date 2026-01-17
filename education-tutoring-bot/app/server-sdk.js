/**
 * Education Tutoring Bot - Secure AI Assistant
 * Protected by EthicalZen guardrails for educational compliance
 */

const { EthicalZenProxyClient } = require('@ethicalzen/sdk');
const express = require('express');
const cors = require('cors');
const dotenv = require('dotenv');
const helmet = require('helmet');

dotenv.config();

const app = express();
const PORT = process.env.PORT || 3000;

const GATEWAY_URL = process.env.ETHICALZEN_GATEWAY_URL || 'http://localhost:8080';
const CERTIFICATE_ID = process.env.ETHICALZEN_CERTIFICATE_ID || 'Education Tutoring Bot/education/us/v1.0';
const TENANT_ID = process.env.ETHICALZEN_TENANT_ID || 'demo';
const ETHICALZEN_API_KEY = process.env.ETHICALZEN_API_KEY || 'sk-demo-public-playground-ethicalzen';
const LLM_PROVIDER = process.env.LLM_PROVIDER || 'groq';
const LLM_MODEL = process.env.LLM_MODEL || 'llama-3.3-70b-versatile';

function getLLMConfig() {
  switch (LLM_PROVIDER.toLowerCase()) {
    case 'groq':
      return { endpoint: 'https://api.groq.com/openai/v1/chat/completions', apiKey: process.env.GROQ_API_KEY };
    case 'openai':
      return { endpoint: 'https://api.openai.com/v1/chat/completions', apiKey: process.env.OPENAI_API_KEY };
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

console.log(`✅ EthicalZen Proxy Client initialized - ${LLM_PROVIDER}`);

app.use(helmet());
app.use(cors());
app.use(express.json());

app.get('/health', (req, res) => {
  res.json({ status: 'healthy', timestamp: new Date().toISOString(), llm: { provider: LLM_PROVIDER, model: LLM_MODEL } });
});

app.post('/chat', async (req, res) => {
  try {
    const { message } = req.body;
    if (!message) return res.status(400).json({ error: 'Message is required' });

    console.log(`[${new Date().toISOString()}] Chat: "${message.substring(0, 50)}..."`);
    const startTime = Date.now();

    const response = await ezClient.post(
      llmConfig.endpoint,
      {
        messages: [
          {
            role: 'system',
            content: `You are a helpful educational tutor. Help students learn and understand concepts.

IMPORTANT RULES:
- NEVER complete homework assignments for students
- Guide students to find answers themselves through questions
- Explain concepts clearly with examples
- Encourage critical thinking
- Do not provide direct answers to test/exam questions`,
          },
          { role: 'user', content: message },
        ],
        model: LLM_MODEL,
        max_tokens: 500,
      },
      { headers: { 'Authorization': `Bearer ${llmConfig.apiKey}` } }
    );

    const latency = Date.now() - startTime;
    const responseContent = response.data.choices?.[0]?.message?.content || response.data.response || '';

    res.json({
      response: responseContent,
      ethicalzen: { status: 'APPROVED', gateway_validated: true, certificate: CERTIFICATE_ID },
      metadata: { latency_ms: latency, model: LLM_MODEL, usage: response.data.usage },
    });
  } catch (error) {
    console.error(`❌ Error: ${error.message}`);
    if (error.response?.status === 403) {
      return res.status(403).json({ error: 'BLOCKED_BY_ETHICALZEN', message: 'Request blocked', blocked: true });
    }
    res.status(500).json({ error: 'INTERNAL_ERROR', message: 'An error occurred' });
  }
});

app.listen(PORT, () => {
  console.log(`\n📚 Education Tutoring Bot running on port ${PORT}`);
  console.log(`🛡️  Protected by EthicalZen\n`);
});

module.exports = app;

