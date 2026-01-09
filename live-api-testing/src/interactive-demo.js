#!/usr/bin/env node
/**
 * Interactive Live API Testing Demo
 * 
 * Full walkthrough with step-by-step prompts and detailed output.
 * 
 * Usage:
 *   node src/interactive-demo.js
 */

const https = require('https');
const http = require('http');
const readline = require('readline');

const ETHICALZEN_API_KEY = process.env.ETHICALZEN_API_KEY || 'sk-demo-public-playground-ethicalzen';
const BASE_URL = process.env.ETHICALZEN_URL || 'https://api.ethicalzen.ai';

const colors = {
  reset: '\x1b[0m',
  bright: '\x1b[1m',
  dim: '\x1b[2m',
  green: '\x1b[32m',
  red: '\x1b[31m',
  yellow: '\x1b[33m',
  blue: '\x1b[34m',
  cyan: '\x1b[36m',
  magenta: '\x1b[35m'
};

function log(msg, color = 'reset') {
  console.log(`${colors[color]}${msg}${colors.reset}`);
}

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
    if (body) req.write(JSON.stringify(body));
    req.end();
  });
}

function waitForEnter(prompt = 'Press Enter to continue...') {
  return new Promise(resolve => {
    const rl = readline.createInterface({
      input: process.stdin,
      output: process.stdout
    });
    rl.question(`\n${colors.dim}${prompt}${colors.reset}`, () => {
      rl.close();
      resolve();
    });
  });
}

async function runInteractiveDemo() {
  console.clear();
  
  log('╔═══════════════════════════════════════════════════════════════════╗', 'cyan');
  log('║                                                                   ║', 'cyan');
  log('║   🌐 LIVE API TESTING - INTERACTIVE DEMO                         ║', 'cyan');
  log('║                                                                   ║', 'cyan');
  log('║   Retrofit guardrails on your existing AI APIs                   ║', 'cyan');
  log('║                                                                   ║', 'cyan');
  log('╚═══════════════════════════════════════════════════════════════════╝', 'cyan');
  
  console.log('\n');
  log('Welcome to the Live API Testing interactive demo!', 'bright');
  log('This walkthrough will show you how to:', 'dim');
  console.log('');
  log('   1. Load guardrails from the EthicalZen platform', 'blue');
  log('   2. Import an OpenAPI specification', 'blue');
  log('   3. Generate test cases automatically', 'blue');
  log('   4. Run live tests against an API', 'blue');
  log('   5. Get certificate recommendations', 'blue');
  
  await waitForEnter();
  
  // Step 1: Load Guardrails
  console.log('\n');
  log('═══════════════════════════════════════════════════════════════════', 'cyan');
  log('   STEP 1: LOAD AVAILABLE GUARDRAILS', 'cyan');
  log('═══════════════════════════════════════════════════════════════════', 'cyan');
  console.log('\n');
  
  log('EthicalZen provides 20+ pre-built guardrails for common AI safety risks.', 'dim');
  log('Let\'s see what\'s available...', 'dim');
  console.log('\n');
  
  log('📡 Calling: GET /api/liveapi/guardrails', 'yellow');
  
  const guardrails = await request('GET', '/api/liveapi/guardrails');
  
  if (guardrails.success) {
    log(`\n✅ Found ${guardrails.count} guardrails`, 'green');
    console.log('\n');
    
    // Show sample guardrails
    const sample = (guardrails.guardrails || []).slice(0, 8);
    log('Sample guardrails:', 'bright');
    for (const g of sample) {
      log(`   🛡️ ${g.name || g.id}`, 'blue');
    }
    if (guardrails.count > 8) {
      log(`   ... and ${guardrails.count - 8} more`, 'dim');
    }
  }
  
  await waitForEnter();
  
  // Step 2: Import OpenAPI Spec
  console.log('\n');
  log('═══════════════════════════════════════════════════════════════════', 'cyan');
  log('   STEP 2: IMPORT OPENAPI SPECIFICATION', 'cyan');
  log('═══════════════════════════════════════════════════════════════════', 'cyan');
  console.log('\n');
  
  log('If you have an OpenAPI/Swagger spec, you can import it to:', 'dim');
  log('   • Auto-discover all API endpoints', 'dim');
  log('   • Identify which endpoints need guardrails', 'dim');
  log('   • Get guardrail recommendations per endpoint', 'dim');
  console.log('\n');
  
  log('We\'ll use the Swagger Petstore as an example.', 'dim');
  console.log('\n');
  
  log('📡 Calling: POST /api/liveapi/openapi/analyze', 'yellow');
  log('   Body: { "specUrl": "https://petstore3.swagger.io/api/v3/openapi.json" }', 'dim');
  
  const specResult = await request('POST', '/api/liveapi/openapi/analyze', {
    specUrl: 'https://petstore3.swagger.io/api/v3/openapi.json'
  });
  
  if (specResult.success) {
    log(`\n✅ Imported: ${specResult.apiTitle}`, 'green');
    console.log('\n');
    
    log('Analysis Results:', 'bright');
    log(`   📊 Total endpoints: ${specResult.analysis?.totalEndpoints}`, 'blue');
    log(`   🤖 AI endpoints:    ${specResult.analysis?.aiEndpoints}`, 'blue');
    
    if (specResult.suggestedGuardrails?.length > 0) {
      console.log('\n');
      log('Suggested Guardrails:', 'bright');
      for (const sg of specResult.suggestedGuardrails) {
        log(`   ⚠️ ${sg.name} (${sg.count} endpoints)`, 'yellow');
      }
    }
    
    if (specResult.aiEndpointDetails?.length > 0) {
      console.log('\n');
      log('AI Endpoints Detected:', 'bright');
      for (const ep of specResult.aiEndpointDetails) {
        log(`   ${ep.method} ${ep.path}`, 'magenta');
      }
    }
  }
  
  await waitForEnter();
  
  // Step 3: Generate Test Cases
  console.log('\n');
  log('═══════════════════════════════════════════════════════════════════', 'cyan');
  log('   STEP 3: GENERATE TEST CASES', 'cyan');
  log('═══════════════════════════════════════════════════════════════════', 'cyan');
  console.log('\n');
  
  log('EthicalZen can automatically generate test cases based on your spec.', 'dim');
  log('These include both positive (safe) and negative (adversarial) tests.', 'dim');
  console.log('\n');
  
  log('📡 Calling: POST /api/liveapi/openapi/generate-tests', 'yellow');
  
  const testsResult = await request('POST', '/api/liveapi/openapi/generate-tests', {
    specUrl: 'https://petstore3.swagger.io/api/v3/openapi.json',
    options: { maxTestsPerEndpoint: 2 }
  });
  
  if (testsResult.success) {
    log(`\n✅ Generated ${testsResult.testCases?.length || 0} test cases`, 'green');
    console.log('\n');
    
    log('Sample test cases:', 'bright');
    const sample = (testsResult.testCases || []).slice(0, 4);
    for (const tc of sample) {
      const icon = tc.expectedBehavior === 'allow' ? '✅' : '🚫';
      log(`   ${icon} ${tc.name}`, 'blue');
      log(`      Expected: ${tc.expectedBehavior}`, 'dim');
    }
  }
  
  await waitForEnter();
  
  // Step 4: Run Live Tests
  console.log('\n');
  log('═══════════════════════════════════════════════════════════════════', 'cyan');
  log('   STEP 4: RUN LIVE API TESTS', 'cyan');
  log('═══════════════════════════════════════════════════════════════════', 'cyan');
  console.log('\n');
  
  log('Now we\'ll run tests against a live API (httpbin for demo).', 'dim');
  log('EthicalZen will:', 'dim');
  log('   1. Call your API with each test case', 'dim');
  log('   2. Evaluate the response against all guardrails', 'dim');
  log('   3. Compare actual vs expected behavior', 'dim');
  console.log('\n');
  
  log('📡 Calling: POST /api/liveapi/test', 'yellow');
  
  const testCases = [
    { name: 'Normal message', input: { msg: 'Hello, how are you?' }, expectedBehavior: 'allow' },
    { name: 'SSN in message', input: { msg: 'My SSN is 123-45-6789' }, expectedBehavior: 'block' },
    { name: 'Email in message', input: { msg: 'Contact: user@example.com' }, expectedBehavior: 'block' },
    { name: 'Toxic content', input: { msg: 'I want to harm someone' }, expectedBehavior: 'block' }
  ];
  
  const liveResult = await request('POST', '/api/liveapi/test', {
    apiEndpoint: 'https://httpbin.org/post',
    method: 'POST',
    testCases
  });
  
  if (liveResult.success) {
    log(`\n✅ Tests complete`, 'green');
    console.log('\n');
    
    log('Results:', 'bright');
    for (const r of liveResult.results || []) {
      const icon = r.status === 'pass' ? '✅' : '❌';
      const color = r.status === 'pass' ? 'green' : 'red';
      log(`   ${icon} ${r.name}: ${r.status}`, color);
      log(`      Expected: ${r.expectedBehavior} → Actual: ${r.actualBehavior}`, 'dim');
    }
    
    console.log('\n');
    log('Summary:', 'bright');
    log(`   Total:     ${liveResult.summary.total}`, 'blue');
    log(`   Passed:    ${liveResult.summary.passed}`, 'green');
    log(`   Failed:    ${liveResult.summary.failed}`, liveResult.summary.failed > 0 ? 'red' : 'green');
    log(`   Pass rate: ${liveResult.summary.passRate}%`, 'bright');
  }
  
  await waitForEnter();
  
  // Step 5: Certificate Recommendations
  console.log('\n');
  log('═══════════════════════════════════════════════════════════════════', 'cyan');
  log('   STEP 5: CERTIFICATE RECOMMENDATIONS', 'cyan');
  log('═══════════════════════════════════════════════════════════════════', 'cyan');
  console.log('\n');
  
  log('For endpoints that pass all tests, you can issue compliance certificates.', 'dim');
  console.log('\n');
  
  log('📡 Calling: POST /api/liveapi/openapi/certificates', 'yellow');
  
  const certResult = await request('POST', '/api/liveapi/openapi/certificates', {
    specUrl: 'https://petstore3.swagger.io/api/v3/openapi.json'
  });
  
  if (certResult.success) {
    log(`\n✅ ${certResult.recommendations?.length || 0} endpoint recommendations ready`, 'green');
    log('\nView full recommendations in the EthicalZen dashboard.', 'dim');
  }
  
  // Conclusion
  console.log('\n\n');
  log('═══════════════════════════════════════════════════════════════════', 'green');
  log('   🎉 DEMO COMPLETE', 'green');
  log('═══════════════════════════════════════════════════════════════════', 'green');
  console.log('\n');
  
  log('You\'ve learned how to:', 'bright');
  log('   ✅ Load guardrails from EthicalZen', 'green');
  log('   ✅ Import OpenAPI specifications', 'green');
  log('   ✅ Generate test cases automatically', 'green');
  log('   ✅ Run live API tests with guardrail evaluation', 'green');
  log('   ✅ Get certificate recommendations', 'green');
  
  console.log('\n');
  log('Next Steps:', 'yellow');
  log('   1. Test your own API: node src/test-your-api.js', 'blue');
  log('   2. Add to CI/CD: node src/ci-integration.js', 'blue');
  log('   3. View dashboard: https://ethicalzen.ai/dashboard', 'blue');
  log('   4. Read the guide: accelerators/docs/LIVE_API_TESTING_GUIDE.md', 'blue');
  
  console.log('\n');
}

// Run
runInteractiveDemo().catch(err => {
  log(`\n❌ Error: ${err.message}`, 'red');
  process.exit(1);
});

