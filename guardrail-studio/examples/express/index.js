/**
 * Express.js Integration Example
 * 
 * This example shows how to integrate EthicalZen guardrails
 * into an Express.js API that wraps an LLM.
 * 
 * Run:
 *   npm install express @ethicalzen/sdk
 *   ETHICALZEN_API_KEY=sk-... node index.js
 */

const express = require('express');

// Mock EthicalZen SDK (replace with actual import in production)
// import { EthicalZen } from '@ethicalzen/sdk';
const EthicalZen = {
  evaluate: async ({ guardrail, input }) => {
    // This is a mock - replace with actual SDK call
    const response = await fetch('https://api.ethicalzen.ai/v1/evaluate', {
      method: 'POST',
      headers: {
        'Authorization': `Bearer ${process.env.ETHICALZEN_API_KEY}`,
        'Content-Type': 'application/json'
      },
      body: JSON.stringify({ guardrail, input })
    });
    return response.json();
  }
};

const app = express();
app.use(express.json());

// Middleware: Guardrail protection for all /chat routes
const guardrailMiddleware = (guardrails) => {
  return async (req, res, next) => {
    const userInput = req.body.message || req.body.prompt || '';
    
    if (!userInput) {
      return next();
    }
    
    try {
      // Evaluate against all specified guardrails
      for (const guardrail of guardrails) {
        const result = await EthicalZen.evaluate({
          guardrail,
          input: userInput
        });
        
        if (result.decision === 'block') {
          return res.status(400).json({
            error: 'Content blocked by safety guardrail',
            guardrail: guardrail,
            reason: result.reason
          });
        }
      }
      
      // All guardrails passed
      next();
    } catch (error) {
      console.error('Guardrail evaluation failed:', error);
      // Fail-open or fail-closed based on your policy
      // Here we fail-closed (block on error)
      return res.status(500).json({
        error: 'Safety check failed',
        message: 'Unable to verify content safety'
      });
    }
  };
};

// Apply guardrails to the chat endpoint
app.post('/chat', 
  guardrailMiddleware([
    'prompt_injection_blocker',
    'pii_blocker',
    'medical_advice_smart'
  ]),
  async (req, res) => {
    const { message } = req.body;
    
    // Your LLM call here (OpenAI, Anthropic, etc.)
    const llmResponse = await callYourLLM(message);
    
    // Optionally: Check the response too
    const outputCheck = await EthicalZen.evaluate({
      guardrail: 'medical_advice_smart',
      input: llmResponse
    });
    
    if (outputCheck.decision === 'block') {
      return res.json({
        response: "I'm not able to provide that specific advice. Please consult a professional."
      });
    }
    
    res.json({ response: llmResponse });
  }
);

// Mock LLM function
async function callYourLLM(message) {
  // Replace with actual LLM call
  return `This is a response to: ${message}`;
}

const PORT = process.env.PORT || 3000;
app.listen(PORT, () => {
  console.log(`Server running on http://localhost:${PORT}`);
  console.log('POST /chat with { "message": "your message" }');
});

