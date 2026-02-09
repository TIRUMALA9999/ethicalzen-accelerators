/**
 * EthicalZen Cloud API Client
 * Wraps all calls to api.ethicalzen.ai with authentication headers,
 * retry logic, and circuit breaker for graceful degradation.
 */

const axios = require('axios');

class ApiClient {
  constructor(config = {}) {
    this.baseUrl = config.apiUrl || process.env.ETHICALZEN_API_URL || 'https://api.ethicalzen.ai';
    this.apiKey = config.apiKey || process.env.ETHICALZEN_API_KEY || '';
    this.tenantId = config.tenantId || process.env.ETHICALZEN_TENANT_ID || 'demo';
    this.timeout = config.timeout || 10000;

    // Circuit breaker state
    this.failures = 0;
    this.circuitOpen = false;
    this.circuitOpenedAt = 0;
    this.circuitCooldown = 30000; // 30s

    this.client = axios.create({
      baseURL: this.baseUrl,
      timeout: this.timeout,
      headers: {
        'Content-Type': 'application/json',
        'X-API-Key': this.apiKey,
        'X-Tenant-ID': this.tenantId
      }
    });
  }

  isCircuitOpen() {
    if (!this.circuitOpen) return false;
    if (Date.now() - this.circuitOpenedAt > this.circuitCooldown) {
      this.circuitOpen = false;
      this.failures = 0;
      return false;
    }
    return true;
  }

  recordSuccess() {
    this.failures = 0;
    this.circuitOpen = false;
  }

  recordFailure() {
    this.failures++;
    if (this.failures >= 5) {
      this.circuitOpen = true;
      this.circuitOpenedAt = Date.now();
    }
  }

  async request(method, path, data = null, retries = 3) {
    if (this.isCircuitOpen()) {
      throw new Error('Circuit breaker open — cloud API unreachable');
    }

    let lastError;
    for (let attempt = 0; attempt < retries; attempt++) {
      try {
        const opts = { method, url: path };
        if (data && (method === 'post' || method === 'put')) {
          opts.data = data;
        } else if (data) {
          opts.params = data;
        }
        const response = await this.client(opts);
        this.recordSuccess();
        return response.data;
      } catch (err) {
        lastError = err;
        if (err.response && (err.response.status === 401 || err.response.status === 403)) {
          throw new Error(`Authentication failed: ${err.response.status}`);
        }
        if (attempt < retries - 1) {
          await new Promise(r => setTimeout(r, (attempt + 1) * 1000));
        }
      }
    }
    this.recordFailure();
    throw lastError;
  }

  // Convenience methods
  async getViolations(params = {}) {
    return this.request('get', '/api/dc/violations', params);
  }

  async getEvidence(params = {}) {
    return this.request('get', '/api/dc/evidence', params);
  }

  async getEvidenceById(traceId) {
    return this.request('get', `/api/dc/evidence/${traceId}`);
  }

  async getRequests(params = {}) {
    return this.request('get', '/api/dc/requests', params);
  }

  async getGuardrails() {
    return this.request('get', '/api/guardrails/list');
  }

  async getDriftStatus() {
    return this.request('get', '/api/guardrails/drift-status');
  }

  async exportOscal(body = {}) {
    return this.request('post', '/api/v2/grc/export/oscal', body);
  }

  async exportStix(body = {}) {
    return this.request('post', '/api/v2/grc/export/stix', body);
  }

  async taxiiDiscovery() {
    return this.request('get', '/taxii2/');
  }

  async taxiiCollections() {
    return this.request('get', '/taxii2/ethicalzen/collections/');
  }

  async taxiiObjects(collectionId, params = {}) {
    return this.request('get', `/taxii2/ethicalzen/collections/${collectionId}/objects/`, params);
  }

  async testConnection() {
    try {
      const start = Date.now();
      await this.request('get', '/api/health', null, 1);
      return { connected: true, latencyMs: Date.now() - start };
    } catch {
      return { connected: false, error: 'Cloud API unreachable' };
    }
  }

  getStatus() {
    return {
      baseUrl: this.baseUrl,
      tenantId: this.tenantId,
      hasApiKey: !!this.apiKey,
      circuitOpen: this.circuitOpen,
      consecutiveFailures: this.failures
    };
  }
}

module.exports = { ApiClient };
