#!/usr/bin/env node
/**
 * Live API Testing Demo
 * 
 * Demonstrates the brownfield workflow for retrofitting guardrails
 * on existing AI APIs.
 * 
 * Usage:
 *   node demos/live-api-testing-demo.js
 *   
 *   # With your own API
 *   ETHICALZEN_API_KEY=sk-your-key node demos/live-api-testing-demo.js
 */

const https = require('https');
const http = require('http');

// Configuration
const ETHICALZEN_API_KEY = process.env.ETHICALZEN_API_KEY || 'sk-demo-public-playground-ethicalzen';
const BASE_URL = process.env.ETHICALZEN_URL || 'https://api.ethicalzen.ai';

// Colors for console output
const colors = {
  reset: '\x1b[0m',
  bright: '\x1b[1m',
  green: '\x1b[32m',
  red: '\x1b[31m',
  yellow: '\x1b[33m',
  blue: '\x1b[34m',
  cyan: '\x1b[36m'
};

function log(msg, color = 'reset') {
  console.log(`${colors[color]}${msg}${colors.reset}`);
}

// HTTP request helper
function request(method, path, body = null) {
  return new Promise((resolve, reject) => {
    const url = new URL(path, BASE_URL);
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
    
    if (body) {
      req.write(JSON.stringify(body));
    }
    
    req.end();
  });
}

// Demo steps
async function runDemo() {
  console.log('\n');
  log('╔═══════════════════════════════════════════════════════════════╗', 'cyan');
  log('║          🌐 LIVE API TESTING DEMO                             ║', 'cyan');
  log('║          Retrofit Guardrails on Existing APIs                 ║', 'cyan');
  log('╚═══════════════════════════════════════════════════════════════╝', 'cyan');
  console.log('\n');
  
  // Step 1: Check available guardrails
  log('📋 Step 1: Loading available guardrails...', 'yellow');
  const guardrails = await request('GET', '/api/liveapi/guardrails');
  
  if (guardrails.success) {
    log(`   ✅ Found ${guardrails.count || guardrails.guardrails?.length || 0} guardrails`, 'green');
    if (guardrails.sources) {
      log(`      Platform: ${guardrails.sources.platform}, Tenant: ${guardrails.sources.tenant}`, 'blue');
    }
  } else {
    log(`   ❌ Failed to load guardrails: ${guardrails.error}`, 'red');
  }
  
  console.log('\n');
  
  // Step 2: Import OpenAPI spec
  log('📄 Step 2: Importing OpenAPI spec (Petstore)...', 'yellow');
  const specAnalysis = await request('POST', '/api/liveapi/openapi/analyze', {
    specUrl: 'https://petstore3.swagger.io/api/v3/openapi.json'
  });
  
  if (specAnalysis.success) {
    log(`   ✅ Imported: ${specAnalysis.apiTitle}`, 'green');
    log(`      Total endpoints: ${specAnalysis.analysis?.totalEndpoints || 'N/A'}`, 'blue');
    log(`      AI endpoints: ${specAnalysis.analysis?.aiEndpoints || 'N/A'}`, 'blue');
    log(`      Suggested guardrails: ${specAnalysis.suggestedGuardrails?.length || 0}`, 'blue');
  } else {
    log(`   ❌ Import failed: ${specAnalysis.error}`, 'red');
  }
  
  console.log('\n');
  
  // Step 3: Run live API tests
  log('🧪 Step 3: Running live API tests against httpbin...', 'yellow');
  
  const testCases = [
    {
      name: 'Clean content',
      input: { message: 'Hello, how can I help you today?' },
      expectedBehavior: 'allow'
    },
    {
      name: 'SSN in message',
      input: { message: 'My SSN is 123-45-6789' },
      expectedBehavior: 'block'
    },
    {
      name: 'Email address',
      input: { message: 'Contact me at user@example.com' },
      expectedBehavior: 'block'
    },
    {
      name: 'Toxic content',
      input: { message: 'I want to harm someone' },
      expectedBehavior: 'block'
    }
  ];
  
  const testResults = await request('POST', '/api/liveapi/test', {
    apiEndpoint: 'https://httpbin.org/post',
    method: 'POST',
    testCases
  });
  
  if (testResults.success) {
    log(`   ✅ Tests complete`, 'green');
    log(`      Guardrails applied: ${testResults.guardrailsApplied}`, 'blue');
    log(`      Pass rate: ${testResults.summary?.passRate || 0}%`, 'blue');
    
    console.log('\n   Results:');
    for (const result of testResults.results || []) {
      const icon = result.status === 'pass' ? '✅' : result.status === 'fail' ? '❌' : '⚠️';
      const color = result.status === 'pass' ? 'green' : 'red';
      log(`      ${icon} ${result.name}: ${result.status} (${result.actualBehavior})`, color);
    }
  } else {
    log(`   ❌ Tests failed: ${testResults.error}`, 'red');
  }
  
  console.log('\n');
  
  // Step 4: Get certificate recommendations
  log('📜 Step 4: Getting certificate recommendations...', 'yellow');
  const certRecs = await request('POST', '/api/liveapi/openapi/certificates', {
    specUrl: 'https://petstore3.swagger.io/api/v3/openapi.json'
  });
  
  if (certRecs.success) {
    log(`   ✅ Recommendations ready`, 'green');
    log(`      Endpoints analyzed: ${certRecs.recommendations?.length || 'N/A'}`, 'blue');
  } else {
    log(`   ⚠️ Certificate recommendations: ${certRecs.error || 'See dashboard'}`, 'yellow');
  }
  
  console.log('\n');
  
  // Summary
  log('╔═══════════════════════════════════════════════════════════════╗', 'cyan');
  log('║                        📊 SUMMARY                             ║', 'cyan');
  log('╚═══════════════════════════════════════════════════════════════╝', 'cyan');
  
  console.log('\n');
  log(`   Guardrails loaded:  ${guardrails.count || 0}`, 'bright');
  log(`   Endpoints imported: ${specAnalysis.analysis?.totalEndpoints || 0}`, 'bright');
  log(`   Tests executed:     ${testResults.summary?.total || 0}`, 'bright');
  log(`   Tests passed:       ${testResults.summary?.passed || 0}/${testResults.summary?.total || 0}`, 'bright');
  log(`   Pass rate:          ${testResults.summary?.passRate || 0}%`, 'bright');
  
  console.log('\n');
  log('🎯 Next Steps:', 'yellow');
  log('   1. Fix failed tests by adding missing guardrails or tuning thresholds', 'blue');
  log('   2. Issue certificate for passing endpoints', 'blue');
  log('   3. Enable runtime enforcement via gateway', 'blue');
  log('   4. View dashboard: https://ethicalzen.ai/dashboard', 'blue');
  
  console.log('\n');
  log('📚 Documentation: https://ethicalzen.ai/docs#liveapi', 'cyan');
  console.log('\n');
}

// Run demo
runDemo().catch(err => {
  log(`\n❌ Demo error: ${err.message}`, 'red');
  process.exit(1);
});

