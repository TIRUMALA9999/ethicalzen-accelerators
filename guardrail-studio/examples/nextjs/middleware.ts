/**
 * Next.js Middleware Integration Example
 * 
 * This example shows how to integrate EthicalZen guardrails
 * as Next.js middleware for API route protection.
 * 
 * Place this file in your Next.js project root or src/ folder.
 * 
 * Environment variables needed:
 *   ETHICALZEN_API_KEY=sk-...
 */

import { NextRequest, NextResponse } from 'next/server';

const ETHICALZEN_API_URL = 'https://api.ethicalzen.ai/v1/evaluate';

interface GuardrailResult {
  decision: 'allow' | 'block' | 'review';
  score: number;
  reason: string;
}

async function evaluateGuardrail(
  guardrail: string,
  input: string,
  apiKey: string
): Promise<GuardrailResult> {
  const response = await fetch(ETHICALZEN_API_URL, {
    method: 'POST',
    headers: {
      'Authorization': `Bearer ${apiKey}`,
      'Content-Type': 'application/json',
    },
    body: JSON.stringify({ guardrail, input }),
  });
  
  if (!response.ok) {
    throw new Error(`Guardrail API error: ${response.status}`);
  }
  
  return response.json();
}

// Configuration for which routes to protect
const PROTECTED_ROUTES: Record<string, string[]> = {
  '/api/chat': ['prompt_injection_blocker', 'pii_blocker', 'medical_advice_smart'],
  '/api/completion': ['prompt_injection_blocker', 'pii_blocker'],
  '/api/assistant': ['prompt_injection_blocker', 'financial_advice_smart'],
};

export async function middleware(request: NextRequest) {
  const path = request.nextUrl.pathname;
  const guardrails = PROTECTED_ROUTES[path];
  
  // Skip if not a protected route
  if (!guardrails || request.method !== 'POST') {
    return NextResponse.next();
  }
  
  const apiKey = process.env.ETHICALZEN_API_KEY;
  if (!apiKey) {
    console.error('ETHICALZEN_API_KEY not configured');
    return NextResponse.next();
  }
  
  try {
    // Clone the request to read the body
    const body = await request.json();
    const userInput = body.message || body.prompt || body.content || '';
    
    if (!userInput) {
      return NextResponse.next();
    }
    
    // Check all guardrails
    for (const guardrail of guardrails) {
      const result = await evaluateGuardrail(guardrail, userInput, apiKey);
      
      if (result.decision === 'block') {
        return NextResponse.json(
          {
            error: 'Content blocked by safety guardrail',
            guardrail,
            reason: result.reason,
          },
          { status: 400 }
        );
      }
    }
    
    // All guardrails passed - continue to the API route
    return NextResponse.next();
    
  } catch (error) {
    console.error('Guardrail middleware error:', error);
    
    // Fail-closed: block on error (change to NextResponse.next() to fail-open)
    return NextResponse.json(
      { error: 'Safety check failed' },
      { status: 500 }
    );
  }
}

// Configure which paths this middleware applies to
export const config = {
  matcher: ['/api/chat', '/api/completion', '/api/assistant'],
};

