/**
 * SQLite Cache Store for offline mode
 * Caches violations, evidence, exports, and metrics locally.
 */

const path = require('path');
let Database;
try {
  Database = require('better-sqlite3');
} catch {
  Database = null;
}

class CacheStore {
  constructor(cacheDir) {
    this.enabled = !!Database;
    if (!this.enabled) {
      console.warn('[Cache] better-sqlite3 not available, caching disabled');
      return;
    }

    const dbPath = path.join(cacheDir || process.env.CACHE_DIR || './data', 'grc-cache.db');
    this.db = new Database(dbPath);
    this.db.pragma('journal_mode = WAL');
    this.initTables();
  }

  initTables() {
    this.db.exec(`
      CREATE TABLE IF NOT EXISTS violations (
        id TEXT PRIMARY KEY,
        data TEXT NOT NULL,
        timestamp TEXT NOT NULL,
        cached_at TEXT DEFAULT (datetime('now'))
      );
      CREATE TABLE IF NOT EXISTS evidence (
        id TEXT PRIMARY KEY,
        data TEXT NOT NULL,
        timestamp TEXT NOT NULL,
        cached_at TEXT DEFAULT (datetime('now'))
      );
      CREATE TABLE IF NOT EXISTS exports (
        id INTEGER PRIMARY KEY AUTOINCREMENT,
        format TEXT NOT NULL,
        data TEXT NOT NULL,
        params TEXT,
        cached_at TEXT DEFAULT (datetime('now'))
      );
      CREATE TABLE IF NOT EXISTS kv (
        key TEXT PRIMARY KEY,
        value TEXT NOT NULL,
        cached_at TEXT DEFAULT (datetime('now'))
      );
      CREATE INDEX IF NOT EXISTS idx_violations_ts ON violations(timestamp);
      CREATE INDEX IF NOT EXISTS idx_evidence_ts ON evidence(timestamp);
    `);
  }

  cacheViolations(violations) {
    if (!this.enabled) return;
    const stmt = this.db.prepare(
      'INSERT OR REPLACE INTO violations (id, data, timestamp) VALUES (?, ?, ?)'
    );
    const tx = this.db.transaction((items) => {
      for (const v of items) {
        stmt.run(v.id || v.trace_id || `v_${Date.now()}`, JSON.stringify(v), v.timestamp || new Date().toISOString());
      }
    });
    tx(violations);
  }

  getViolations(limit = 100) {
    if (!this.enabled) return [];
    const rows = this.db.prepare(
      'SELECT data FROM violations ORDER BY timestamp DESC LIMIT ?'
    ).all(limit);
    return rows.map(r => JSON.parse(r.data));
  }

  cacheEvidence(records) {
    if (!this.enabled) return;
    const stmt = this.db.prepare(
      'INSERT OR REPLACE INTO evidence (id, data, timestamp) VALUES (?, ?, ?)'
    );
    const tx = this.db.transaction((items) => {
      for (const e of items) {
        stmt.run(e.id || e.trace_id || `e_${Date.now()}`, JSON.stringify(e), e.timestamp || new Date().toISOString());
      }
    });
    tx(records);
  }

  getEvidence(limit = 100) {
    if (!this.enabled) return [];
    const rows = this.db.prepare(
      'SELECT data FROM evidence ORDER BY timestamp DESC LIMIT ?'
    ).all(limit);
    return rows.map(r => JSON.parse(r.data));
  }

  cacheExport(format, data, params = {}) {
    if (!this.enabled) return;
    this.db.prepare(
      'INSERT INTO exports (format, data, params) VALUES (?, ?, ?)'
    ).run(format, JSON.stringify(data), JSON.stringify(params));
  }

  getExports(format, limit = 10) {
    if (!this.enabled) return [];
    const rows = this.db.prepare(
      'SELECT data, params, cached_at FROM exports WHERE format = ? ORDER BY cached_at DESC LIMIT ?'
    ).all(format, limit);
    return rows.map(r => ({ data: JSON.parse(r.data), params: JSON.parse(r.params || '{}'), cachedAt: r.cached_at }));
  }

  set(key, value) {
    if (!this.enabled) return;
    this.db.prepare(
      'INSERT OR REPLACE INTO kv (key, value) VALUES (?, ?)'
    ).run(key, JSON.stringify(value));
  }

  get(key) {
    if (!this.enabled) return null;
    const row = this.db.prepare('SELECT value FROM kv WHERE key = ?').get(key);
    return row ? JSON.parse(row.value) : null;
  }

  getStats() {
    if (!this.enabled) return { enabled: false };
    const violations = this.db.prepare('SELECT COUNT(*) as c FROM violations').get().c;
    const evidence = this.db.prepare('SELECT COUNT(*) as c FROM evidence').get().c;
    const exports = this.db.prepare('SELECT COUNT(*) as c FROM exports').get().c;
    return { enabled: true, violations, evidence, exports };
  }

  clear() {
    if (!this.enabled) return;
    this.db.exec('DELETE FROM violations; DELETE FROM evidence; DELETE FROM exports; DELETE FROM kv;');
  }

  seedDemoData() {
    if (!this.enabled) return;
    const now = new Date();
    const violations = [];
    const evidence = [];
    const types = ['pii_violation', 'prompt_injection', 'toxicity', 'hipaa_violation', 'bias_detected'];
    const statuses = ['blocked', 'blocked', 'blocked', 'allowed', 'blocked'];
    const severities = ['critical', 'high', 'high', 'moderate', 'moderate'];

    for (let i = 0; i < 50; i++) {
      const ts = new Date(now - i * 3600000).toISOString();
      const typeIdx = i % types.length;
      violations.push({
        id: `demo_v_${i}`,
        trace_id: `trace_demo_${i}`,
        contract_id: `dc_demo_contract_${i % 3}`,
        violation_type: types[typeIdx],
        severity: severities[typeIdx],
        status: statuses[typeIdx],
        risk_score: (0.3 + Math.random() * 0.7).toFixed(4),
        latency_ms: Math.floor(5 + Math.random() * 50),
        timestamp: ts
      });
    }

    for (let i = 0; i < 200; i++) {
      const ts = new Date(now - i * 1800000).toISOString();
      evidence.push({
        id: `demo_e_${i}`,
        trace_id: `trace_demo_e_${i}`,
        contract_id: `dc_demo_contract_${i % 5}`,
        request_type: 'guardrail_evaluation',
        status: i % 4 === 0 ? 'blocked' : 'allowed',
        risk_score: (Math.random() * 0.8).toFixed(4),
        latency_ms: Math.floor(2 + Math.random() * 30),
        timestamp: ts
      });
    }

    this.cacheViolations(violations);
    this.cacheEvidence(evidence);
    return { violations: violations.length, evidence: evidence.length };
  }
}

module.exports = { CacheStore };
