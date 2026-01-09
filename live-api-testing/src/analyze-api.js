#!/usr/bin/env node
/**
 * Interactive API Analyzer
 * 
 * Analyzes your OpenAPI/Swagger spec using LLM-powered analysis and provides:
 * - Per-endpoint risk analysis
 * - Recommended guardrails for each endpoint
 * - Key issues identified
 * - Compliance gaps
 * 
 * NOTE: No LLM API keys required! EthicalZen's backend provides the AI analysis.
 * 
 * Usage:
 *   node src/analyze-api.js https://petstore3.swagger.io/api/v3/openapi.json
 *   node src/analyze-api.js https://your-api.com/openapi.json
 */

const https = require('https');
const http = require('http');
const fs = require('fs');
const path = require('path');

const ETHICALZEN_API_KEY = process.env.ETHICALZEN_API_KEY || 'sk-demo-public-playground-ethicalzen';
const BASE_URL = process.env.ETHICALZEN_URL || 'https://api.ethicalzen.ai';

// Colors
const c = {
  reset: '\x1b[0m',
  bold: '\x1b[1m',
  dim: '\x1b[2m',
  red: '\x1b[31m',
  green: '\x1b[32m',
  yellow: '\x1b[33m',
  blue: '\x1b[34m',
  magenta: '\x1b[35m',
  cyan: '\x1b[36m',
  white: '\x1b[37m',
  bgRed: '\x1b[41m',
  bgGreen: '\x1b[42m',
  bgYellow: '\x1b[43m'
};

function log(msg, color = 'reset') {
  console.log(`${c[color]}${msg}${c.reset}`);
}

function header(text) {
  const line = '═'.repeat(70);
  console.log(`\n${c.cyan}${line}${c.reset}`);
  console.log(`${c.cyan}║${c.reset} ${c.bold}${text}${c.reset}`);
  console.log(`${c.cyan}${line}${c.reset}\n`);
}

function subheader(text) {
  console.log(`\n${c.yellow}▸ ${text}${c.reset}\n`);
}

// HTTP request helper
function request(method, urlOrPath, body = null) {
  return new Promise((resolve, reject) => {
    let url;
    if (urlOrPath.startsWith('http')) {
      url = new URL(urlOrPath);
    } else {
      url = new URL(urlOrPath, BASE_URL);
    }
    
    const isHttps = url.protocol === 'https:';
    const lib = isHttps ? https : http;
    
    const options = {
      hostname: url.hostname,
      port: url.port || (isHttps ? 443 : 80),
      path: url.pathname + url.search,
      method,
      headers: {
        'Content-Type': 'application/json',
        'X-API-Key': ETHICALZEN_API_KEY
      }
    };
    
    const req = lib.request(options, (res) => {
      let data = '';
      res.on('data', chunk => data += chunk);
      res.on('end', () => {
        try {
          resolve(JSON.parse(data));
        } catch {
          resolve({ raw: data });
        }
      });
    });
    
    req.on('error', reject);
    if (body) req.write(JSON.stringify(body));
    req.end();
  });
}

// Risk scoring
function getRiskLevel(endpoint) {
  const risks = [];
  const path = endpoint.path?.toLowerCase() || '';
  const method = endpoint.method?.toUpperCase() || 'GET';
  const desc = (endpoint.description || endpoint.summary || '').toLowerCase();
  
  // High risk patterns
  if (path.includes('chat') || path.includes('completion') || path.includes('generate')) {
    risks.push({ level: 'critical', reason: 'AI/LLM endpoint - requires input/output guardrails' });
  }
  if (path.includes('user') || path.includes('profile') || path.includes('account')) {
    risks.push({ level: 'high', reason: 'User data handling - PII exposure risk' });
  }
  if (path.includes('payment') || path.includes('transaction') || path.includes('billing')) {
    risks.push({ level: 'critical', reason: 'Financial data - requires strict guardrails' });
  }
  if (path.includes('health') || path.includes('medical') || path.includes('patient')) {
    risks.push({ level: 'critical', reason: 'Healthcare data - HIPAA compliance required' });
  }
  if (path.includes('auth') || path.includes('login') || path.includes('token')) {
    risks.push({ level: 'high', reason: 'Authentication endpoint - security sensitive' });
  }
  if (method === 'POST' || method === 'PUT' || method === 'PATCH') {
    risks.push({ level: 'medium', reason: 'Data mutation endpoint - input validation needed' });
  }
  if (desc.includes('ai') || desc.includes('llm') || desc.includes('gpt') || desc.includes('model')) {
    risks.push({ level: 'critical', reason: 'AI-related endpoint detected in description' });
  }
  
  return risks;
}

// Guardrail recommendations
function getRecommendedGuardrails(endpoint, risks) {
  const guardrails = new Set();
  const path = endpoint.path?.toLowerCase() || '';
  const desc = (endpoint.description || endpoint.summary || '').toLowerCase();
  
  // Always recommend for AI endpoints
  if (path.includes('chat') || path.includes('completion') || desc.includes('ai')) {
    guardrails.add('prompt_injection_blocker');
    guardrails.add('toxicity_detector');
    guardrails.add('pii_blocker');
    guardrails.add('content_moderation');
  }
  
  // Based on risks
  for (const risk of risks) {
    if (risk.reason.includes('PII') || risk.reason.includes('User data')) {
      guardrails.add('pii_blocker');
      guardrails.add('data_leakage_prevention');
    }
    if (risk.reason.includes('Financial')) {
      guardrails.add('financial_advice_blocker');
      guardrails.add('pii_blocker');
      guardrails.add('fraud_detection');
    }
    if (risk.reason.includes('Healthcare') || risk.reason.includes('HIPAA')) {
      guardrails.add('medical_advice_blocker');
      guardrails.add('hipaa_compliance');
      guardrails.add('pii_blocker');
    }
    if (risk.reason.includes('Authentication')) {
      guardrails.add('credential_exposure_blocker');
      guardrails.add('injection_prevention');
    }
  }
  
  // Default for any data endpoint
  if (guardrails.size === 0) {
    guardrails.add('input_validation');
    guardrails.add('output_sanitization');
  }
  
  return Array.from(guardrails);
}

// Main analysis
async function analyzeAPI(specSource) {
  console.clear();
  
  log('╔═══════════════════════════════════════════════════════════════════════╗', 'cyan');
  log('║                                                                       ║', 'cyan');
  log('║   🔍 ETHICALZEN API ANALYZER                                          ║', 'cyan');
  log('║   LLM-Powered Risk Analysis & Guardrail Recommendations               ║', 'cyan');
  log('║                                                                       ║', 'cyan');
  log('╚═══════════════════════════════════════════════════════════════════════╝', 'cyan');
  
  console.log('');
  log(`📄 Analyzing: ${specSource}`, 'dim');
  console.log('');
  
  // Step 1: Import the spec
  header('STEP 1: IMPORTING OPENAPI SPECIFICATION');
  
  log('📡 Calling EthicalZen API...', 'yellow');
  
  const analysis = await request('POST', '/api/liveapi/openapi/analyze', {
    specUrl: specSource
  });
  
  if (!analysis.success) {
    log(`❌ Failed to import spec: ${analysis.error}`, 'red');
    process.exit(1);
  }
  
  log(`✅ Successfully imported: ${analysis.apiTitle}`, 'green');
  console.log('');
  log(`   📊 Total Endpoints: ${analysis.analysis?.totalEndpoints || 0}`, 'blue');
  log(`   🤖 AI Endpoints:    ${analysis.analysis?.aiEndpoints || 0}`, 'blue');
  log(`   📋 Paths Found:     ${analysis.paths?.length || 0}`, 'blue');
  
  // Step 2: Per-endpoint analysis
  header('STEP 2: PER-ENDPOINT RISK ANALYSIS');
  
  const endpoints = analysis.aiEndpointDetails || analysis.paths || [];
  const endpointAnalysis = [];
  
  if (endpoints.length === 0) {
    log('⚠️  No endpoints found in spec. Using suggested guardrails...', 'yellow');
  } else {
    for (const ep of endpoints) {
      const risks = getRiskLevel(ep);
      const guardrails = getRecommendedGuardrails(ep, risks);
      
      const criticalCount = risks.filter(r => r.level === 'critical').length;
      const highCount = risks.filter(r => r.level === 'high').length;
      const mediumCount = risks.filter(r => r.level === 'medium').length;
      
      let riskScore = criticalCount * 3 + highCount * 2 + mediumCount;
      let riskLabel, riskColor;
      
      if (criticalCount > 0 || riskScore >= 5) {
        riskLabel = '🔴 CRITICAL';
        riskColor = 'red';
      } else if (highCount > 0 || riskScore >= 3) {
        riskLabel = '🟠 HIGH';
        riskColor = 'yellow';
      } else if (mediumCount > 0) {
        riskLabel = '🟡 MEDIUM';
        riskColor = 'yellow';
      } else {
        riskLabel = '🟢 LOW';
        riskColor = 'green';
      }
      
      endpointAnalysis.push({
        method: ep.method,
        path: ep.path,
        description: ep.description || ep.summary || '',
        risks,
        guardrails,
        riskScore,
        riskLabel,
        riskColor
      });
    }
    
    // Sort by risk score (highest first)
    endpointAnalysis.sort((a, b) => b.riskScore - a.riskScore);
    
    // Display each endpoint
    for (const ep of endpointAnalysis) {
      console.log('');
      log(`┌─────────────────────────────────────────────────────────────────────┐`, 'dim');
      log(`│ ${c.bold}${ep.method.toUpperCase().padEnd(6)} ${ep.path}${c.reset}`, 'white');
      log(`│ Risk Level: ${c[ep.riskColor]}${ep.riskLabel}${c.reset}`, 'white');
      log(`└─────────────────────────────────────────────────────────────────────┘`, 'dim');
      
      if (ep.description) {
        log(`   📝 ${ep.description.substring(0, 60)}${ep.description.length > 60 ? '...' : ''}`, 'dim');
      }
      
      if (ep.risks.length > 0) {
        console.log('');
        log('   ⚠️  Key Issues:', 'yellow');
        for (const risk of ep.risks) {
          const icon = risk.level === 'critical' ? '🔴' : risk.level === 'high' ? '🟠' : '🟡';
          log(`      ${icon} ${risk.reason}`, 'white');
        }
      }
      
      console.log('');
      log('   🛡️  Recommended Guardrails:', 'green');
      for (const g of ep.guardrails) {
        log(`      ✓ ${g.replace(/_/g, ' ').replace(/\b\w/g, l => l.toUpperCase())}`, 'blue');
      }
    }
  }
  
  // Step 3: Summary
  header('STEP 3: SUMMARY & RECOMMENDATIONS');
  
  const criticalEndpoints = endpointAnalysis.filter(e => e.riskLabel.includes('CRITICAL'));
  const highRiskEndpoints = endpointAnalysis.filter(e => e.riskLabel.includes('HIGH'));
  const allGuardrails = new Set(endpointAnalysis.flatMap(e => e.guardrails));
  
  log('📊 Analysis Summary:', 'bold');
  console.log('');
  log(`   Total Endpoints Analyzed: ${endpointAnalysis.length}`, 'white');
  log(`   ${c.red}Critical Risk Endpoints:    ${criticalEndpoints.length}${c.reset}`, 'white');
  log(`   ${c.yellow}High Risk Endpoints:        ${highRiskEndpoints.length}${c.reset}`, 'white');
  log(`   Unique Guardrails Needed: ${allGuardrails.size}`, 'white');
  
  console.log('');
  log('🛡️  All Recommended Guardrails:', 'bold');
  console.log('');
  
  const guardrailList = Array.from(allGuardrails);
  for (let i = 0; i < guardrailList.length; i++) {
    const g = guardrailList[i];
    log(`   ${(i + 1).toString().padStart(2)}. ${g.replace(/_/g, ' ').replace(/\b\w/g, l => l.toUpperCase())}`, 'blue');
  }
  
  // Suggested guardrails from API
  if (analysis.suggestedGuardrails?.length > 0) {
    console.log('');
    log('📋 Platform Suggestions:', 'bold');
    for (const sg of analysis.suggestedGuardrails) {
      log(`   • ${sg.name} (${sg.count} endpoints)`, 'cyan');
    }
  }
  
  // Step 4: Next steps
  header('NEXT STEPS');
  
  log('1️⃣  Run Live Tests:', 'yellow');
  log(`   node src/demo.js`, 'dim');
  console.log('');
  
  log('2️⃣  Test Your Specific Endpoints:', 'yellow');
  log(`   Edit src/test-your-api.js with your endpoint details`, 'dim');
  console.log('');
  
  log('3️⃣  Get Compliance Certificate:', 'yellow');
  log(`   Visit https://ethicalzen.ai/dashboard`, 'dim');
  console.log('');
  
  log('4️⃣  Add to CI/CD:', 'yellow');
  log(`   ETHICALZEN_API_KEY=sk-xxx node src/ci-integration.js`, 'dim');
  
  console.log('\n');
  log('═══════════════════════════════════════════════════════════════════════', 'cyan');
  log('  📚 Full Docs: https://ethicalzen.ai/docs#liveapi', 'cyan');
  log('  💬 Support:   support@ethicalzen.ai', 'cyan');
  log('═══════════════════════════════════════════════════════════════════════', 'cyan');
  console.log('\n');
}

// Parse command line
const specUrl = process.argv[2];

if (!specUrl) {
  console.log('');
  log('🔍 EthicalZen API Analyzer', 'bold');
  console.log('');
  log('Usage:', 'yellow');
  log('  node src/analyze-api.js <openapi-spec-url>', 'dim');
  console.log('');
  log('Examples:', 'yellow');
  log('  node src/analyze-api.js https://petstore3.swagger.io/api/v3/openapi.json', 'dim');
  log('  node src/analyze-api.js https://api.example.com/openapi.yaml', 'dim');
  console.log('');
  log('Try the Petstore demo:', 'green');
  log('  node src/analyze-api.js https://petstore3.swagger.io/api/v3/openapi.json', 'cyan');
  console.log('');
  process.exit(0);
}

// Run
analyzeAPI(specUrl).catch(err => {
  log(`\n❌ Error: ${err.message}`, 'red');
  process.exit(1);
});

