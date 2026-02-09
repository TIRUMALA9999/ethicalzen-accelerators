// Frontend API client — talks to local proxy (never exposes API key to browser)

const API = {
  base: '/api/grc',

  async get(path, params = {}) {
    const url = new URL(this.base + path, window.location.origin);
    Object.entries(params).forEach(([k, v]) => {
      if (v !== undefined && v !== null) url.searchParams.set(k, v);
    });
    const controller = new AbortController();
    const tid = setTimeout(() => controller.abort(), 5000);
    try {
      const res = await fetch(url, { signal: controller.signal });
      clearTimeout(tid);
      if (!res.ok) throw new Error(`API ${res.status}: ${res.statusText}`);
      return res.json();
    } catch (err) {
      clearTimeout(tid);
      if (err.name === 'AbortError') throw new Error('Request timeout (5s)');
      throw err;
    }
  },

  async post(path, body = {}) {
    const controller = new AbortController();
    const tid = setTimeout(() => controller.abort(), 8000);
    try {
      const res = await fetch(this.base + path, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify(body),
        signal: controller.signal
      });
      clearTimeout(tid);
      if (!res.ok) throw new Error(`API ${res.status}: ${res.statusText}`);
      return res.json();
    } catch (err) {
      clearTimeout(tid);
      if (err.name === 'AbortError') throw new Error('Request timeout (8s)');
      throw err;
    }
  },

  // Specific endpoints
  health() { return this.get('/health'); },
  violations(params) { return this.get('/violations', params); },
  evidence(params) { return this.get('/evidence', params); },
  evidenceById(traceId) { return this.get(`/evidence/${traceId}`); },
  requests(params) { return this.get('/requests', params); },
  guardrails() { return this.get('/guardrails'); },
  driftStatus() { return this.get('/drift-status'); },
  risk() { return this.get('/risk'); },

  exportOscal(body) { return this.post('/export/oscal', body); },
  exportStix(body) { return this.post('/export/stix', body); },

  taxiiDiscovery() { return this.get('/taxii/discovery'); },
  taxiiCollections() { return this.get('/taxii/collections'); },
  taxiiObjects(id, params) { return this.get(`/taxii/collections/${id}/objects`, params); },

  cacheStatus() { return this.get('/cache/status'); },
  cacheClear() { return this.post('/cache/clear'); },
  cacheSeedDemo() { return this.post('/cache/seed-demo'); },

  updateSettings(body) { return this.post('/settings', body); },

  // SSE connection
  connectViolationStream(onEvent) {
    const source = new EventSource(this.base + '/violations/stream');
    source.addEventListener('violation', (e) => onEvent('violation', JSON.parse(e.data)));
    source.addEventListener('stats', (e) => onEvent('stats', JSON.parse(e.data)));
    source.addEventListener('heartbeat', (e) => onEvent('heartbeat', JSON.parse(e.data)));
    source.addEventListener('error', (e) => onEvent('error', e));
    source.addEventListener('connected', (e) => onEvent('connected', JSON.parse(e.data)));
    return source;
  }
};
