/**
 * EthicalZen GRC Compliance Dashboard
 * Standalone web app that connects to the EthicalZen cloud backend
 * for real-time guardrail violation monitoring and compliance reporting.
 */

require('dotenv').config();
const express = require('express');
const cors = require('cors');
const helmet = require('helmet');
const compression = require('compression');
const path = require('path');
const fs = require('fs');

const { ApiClient } = require('./services/api-client');
const { ViolationPoller } = require('./services/poller');
const { CacheStore } = require('./services/cache-store');
const { RiskAggregator } = require('./services/risk-aggregator');

const app = express();
const PORT = process.env.PORT || 3000;

// =============================================================================
// Middleware
// =============================================================================

app.use(helmet({ contentSecurityPolicy: false }));
app.use(cors());
app.use(compression());
app.use(express.json());

// Serve static frontend
app.use(express.static(path.join(__dirname, 'public')));

// =============================================================================
// Initialize Services
// =============================================================================

const cacheDir = process.env.CACHE_DIR || path.join(__dirname, '..', 'data');
if (!fs.existsSync(cacheDir)) fs.mkdirSync(cacheDir, { recursive: true });

const apiClient = new ApiClient();
const cacheStore = new CacheStore(cacheDir);
const poller = new ViolationPoller(apiClient, cacheStore);
const riskAggregator = new RiskAggregator();

// =============================================================================
// Health & Status
// =============================================================================

app.get('/api/grc/health', async (req, res) => {
  const connection = await apiClient.testConnection();
  res.json({
    status: 'ok',
    service: 'grc-compliance-dashboard',
    cloud: connection,
    poller: poller.getStats(),
    cache: cacheStore.getStats(),
    apiStatus: apiClient.getStatus()
  });
});

// =============================================================================
// SSE — Live Violation Stream
// =============================================================================

app.get('/api/grc/violations/stream', (req, res) => {
  res.setHeader('Content-Type', 'text/event-stream');
  res.setHeader('Cache-Control', 'no-cache');
  res.setHeader('Connection', 'keep-alive');
  res.flushHeaders();

  // Send initial connection event
  res.write(`event: connected\ndata: ${JSON.stringify({ time: new Date().toISOString() })}\n\n`);

  poller.addClient(res);
});

// =============================================================================
// Proxy Routes — Violations
// =============================================================================

app.get('/api/grc/violations', async (req, res) => {
  try {
    const data = await apiClient.getViolations(req.query);
    if (Array.isArray(data) || data.violations) {
      const violations = Array.isArray(data) ? data : data.violations;
      cacheStore.cacheViolations(violations);
    }
    res.json(data);
  } catch (err) {
    // Serve from cache on failure
    const cached = cacheStore.getViolations(parseInt(req.query.limit) || 100);
    if (cached.length > 0) {
      res.json({ violations: cached, source: 'cache', stale: true });
    } else {
      res.status(502).json({ error: 'Cloud API unreachable', message: err.message });
    }
  }
});

// =============================================================================
// Proxy Routes — Evidence
// =============================================================================

app.get('/api/grc/evidence', async (req, res) => {
  try {
    const data = await apiClient.getEvidence(req.query);
    const records = Array.isArray(data) ? data : (data.evidence || data.data || []);
    cacheStore.cacheEvidence(records);
    res.json(data);
  } catch (err) {
    const cached = cacheStore.getEvidence(parseInt(req.query.limit) || 100);
    if (cached.length > 0) {
      res.json({ evidence: cached, source: 'cache', stale: true });
    } else {
      res.status(502).json({ error: 'Cloud API unreachable', message: err.message });
    }
  }
});

app.get('/api/grc/evidence/:traceId', async (req, res) => {
  try {
    const data = await apiClient.getEvidenceById(req.params.traceId);
    res.json(data);
  } catch (err) {
    res.status(502).json({ error: 'Cloud API unreachable', message: err.message });
  }
});

// =============================================================================
// Proxy Routes — Requests & Guardrails
// =============================================================================

app.get('/api/grc/requests', async (req, res) => {
  try {
    const data = await apiClient.getRequests(req.query);
    res.json(data);
  } catch (err) {
    res.status(502).json({ error: 'Cloud API unreachable', message: err.message });
  }
});

app.get('/api/grc/guardrails', async (req, res) => {
  try {
    const data = await apiClient.getGuardrails();
    cacheStore.set('guardrails', data);
    res.json(data);
  } catch (err) {
    const cached = cacheStore.get('guardrails');
    if (cached) {
      res.json({ ...cached, source: 'cache', stale: true });
    } else {
      res.status(502).json({ error: 'Cloud API unreachable', message: err.message });
    }
  }
});

// =============================================================================
// Proxy Routes — Drift
// =============================================================================

app.get('/api/grc/drift-status', async (req, res) => {
  try {
    const data = await apiClient.getDriftStatus();
    cacheStore.set('drift-status', data);
    res.json(data);
  } catch (err) {
    const cached = cacheStore.get('drift-status');
    if (cached) {
      res.json({ ...cached, source: 'cache', stale: true });
    } else {
      res.status(502).json({ error: 'Cloud API unreachable', message: err.message });
    }
  }
});

// =============================================================================
// Proxy Routes — GRC Exports (OSCAL, STIX)
// =============================================================================

app.post('/api/grc/export/oscal', async (req, res) => {
  try {
    const data = await apiClient.exportOscal(req.body);
    cacheStore.cacheExport('oscal', data, req.body);
    res.json(data);
  } catch (err) {
    const cached = cacheStore.getExports('oscal', 1);
    if (cached.length > 0) {
      res.json({ ...cached[0].data, source: 'cache', stale: true });
    } else {
      res.status(502).json({ error: 'OSCAL export failed', message: err.message });
    }
  }
});

app.post('/api/grc/export/stix', async (req, res) => {
  try {
    const data = await apiClient.exportStix(req.body);
    cacheStore.cacheExport('stix', data, req.body);
    res.json(data);
  } catch (err) {
    const cached = cacheStore.getExports('stix', 1);
    if (cached.length > 0) {
      res.json({ ...cached[0].data, source: 'cache', stale: true });
    } else {
      res.status(502).json({ error: 'STIX export failed', message: err.message });
    }
  }
});

// =============================================================================
// Proxy Routes — TAXII
// =============================================================================

app.get('/api/grc/taxii/discovery', async (req, res) => {
  try {
    const data = await apiClient.taxiiDiscovery();
    res.json(data);
  } catch (err) {
    res.status(502).json({ error: 'TAXII discovery failed', message: err.message });
  }
});

app.get('/api/grc/taxii/collections', async (req, res) => {
  try {
    const data = await apiClient.taxiiCollections();
    res.json(data);
  } catch (err) {
    res.status(502).json({ error: 'TAXII collections failed', message: err.message });
  }
});

app.get('/api/grc/taxii/collections/:id/objects', async (req, res) => {
  try {
    const data = await apiClient.taxiiObjects(req.params.id, req.query);
    res.json(data);
  } catch (err) {
    res.status(502).json({ error: 'TAXII objects failed', message: err.message });
  }
});

// =============================================================================
// Risk Aggregation
// =============================================================================

app.get('/api/grc/risk', async (req, res) => {
  try {
    let violations, drift;
    try {
      violations = await apiClient.getViolations({ limit: 500 });
      violations = Array.isArray(violations) ? violations : (violations.violations || []);
    } catch {
      violations = cacheStore.getViolations(500);
    }
    try {
      drift = await apiClient.getDriftStatus();
      drift = Array.isArray(drift) ? drift : (drift.alerts || []);
    } catch {
      drift = [];
    }
    const risk = riskAggregator.computeRisk(violations, [], drift);
    res.json(risk);
  } catch (err) {
    res.status(500).json({ error: 'Risk computation failed', message: err.message });
  }
});

// =============================================================================
// Cache Management
// =============================================================================

app.get('/api/grc/cache/status', (req, res) => {
  res.json(cacheStore.getStats());
});

app.post('/api/grc/cache/clear', (req, res) => {
  cacheStore.clear();
  res.json({ success: true, message: 'Cache cleared' });
});

app.post('/api/grc/cache/seed-demo', (req, res) => {
  const result = cacheStore.seedDemoData();
  res.json({ success: true, message: 'Demo data seeded', ...result });
});

// =============================================================================
// Settings — Runtime configuration update
// =============================================================================

app.post('/api/grc/settings', (req, res) => {
  const { apiUrl, apiKey, tenantId } = req.body;
  if (apiUrl) apiClient.baseUrl = apiUrl;
  if (apiKey) {
    apiClient.apiKey = apiKey;
    apiClient.client.defaults.headers['X-API-Key'] = apiKey;
  }
  if (tenantId) {
    apiClient.tenantId = tenantId;
    apiClient.client.defaults.headers['X-Tenant-ID'] = tenantId;
  }
  res.json({ success: true, status: apiClient.getStatus() });
});

// =============================================================================
// SPA Fallback
// =============================================================================

app.get('*', (req, res) => {
  res.sendFile(path.join(__dirname, 'public', 'index.html'));
});

// =============================================================================
// Start Server
// =============================================================================

const server = app.listen(PORT, () => {
  console.log(`\n  EthicalZen GRC Compliance Dashboard`);
  console.log(`  Running on http://localhost:${PORT}`);
  console.log(`  Cloud API: ${apiClient.baseUrl}`);
  console.log(`  Tenant: ${apiClient.tenantId}`);
  console.log(`  API Key: ${apiClient.apiKey ? '***configured***' : 'NOT SET'}\n`);

  // Start background violation poller
  if (apiClient.apiKey) {
    poller.start();
  } else {
    console.log('  [Poller] Skipped — no API key configured. Set ETHICALZEN_API_KEY in .env\n');
  }
});

// Graceful shutdown
process.on('SIGTERM', () => {
  console.log('\n[Shutdown] Stopping...');
  poller.stop();
  server.close(() => process.exit(0));
});
