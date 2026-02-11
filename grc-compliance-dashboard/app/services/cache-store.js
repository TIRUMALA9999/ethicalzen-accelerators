/**
 * SQLite Cache Store for offline mode
 * Caches violations, evidence, exports, and metrics locally.
 * Supports both better-sqlite3 (native, preferred) and sql.js (pure JS fallback).
 */

const path = require('path');
const fs = require('fs');

let Database;
try {
  Database = require('better-sqlite3');
  // Verify native bindings actually work (require succeeds but native .node may be missing)
  new Database(':memory:').close();
} catch {
  Database = null;
}

// Fallback: try sql.js (pure JavaScript SQLite — no native build needed)
let initSqlJs;
if (!Database) {
  try {
    initSqlJs = require('sql.js');
  } catch {
    initSqlJs = null;
  }
}

/**
 * Wrapper around sql.js to provide a similar API to better-sqlite3
 */
class SqlJsWrapper {
  constructor(dbPath) {
    this.dbPath = dbPath;
    this.db = null;
    this._ready = false;
    this._inTransaction = false;
  }

  async init() {
    const SQL = await initSqlJs();
    // Load existing DB file if it exists
    if (fs.existsSync(this.dbPath)) {
      const buffer = fs.readFileSync(this.dbPath);
      this.db = new SQL.Database(buffer);
    } else {
      this.db = new SQL.Database();
    }
    this._ready = true;
  }

  _save() {
    if (!this._ready) return;
    try {
      const data = this.db.export();
      const buffer = Buffer.from(data);
      fs.writeFileSync(this.dbPath, buffer);
    } catch (e) {
      console.warn('[Cache] Failed to save DB:', e.message);
    }
  }

  pragma() { /* no-op for sql.js */ }

  exec(sql) {
    // sql.js db.run() only handles one statement; use exec for multiple
    const statements = sql.split(';').map(s => s.trim()).filter(s => s.length > 0);
    for (const stmt of statements) {
      this.db.run(stmt);
    }
    this._save();
  }

  prepare(sql) {
    const db = this.db;
    const wrapper = this;
    return {
      run(...params) {
        db.run(sql, params);
        if (!wrapper._inTransaction) wrapper._save();
      },
      all(...params) {
        const stmt = db.prepare(sql);
        stmt.bind(params);
        const rows = [];
        while (stmt.step()) {
          rows.push(stmt.getAsObject());
        }
        stmt.free();
        return rows;
      },
      get(...params) {
        const stmt = db.prepare(sql);
        stmt.bind(params);
        let row = null;
        if (stmt.step()) {
          row = stmt.getAsObject();
        }
        stmt.free();
        return row;
      }
    };
  }

  transaction(fn) {
    const wrapper = this;
    return function(...args) {
      // Batch mode: suppress per-statement saves, save once at the end
      wrapper._inTransaction = true;
      try {
        fn(...args);
      } finally {
        wrapper._inTransaction = false;
      }
      wrapper._save();
    };
  }
}

class CacheStore {
  constructor(cacheDir) {
    this.enabled = false;
    this.db = null;
    this._cacheDir = cacheDir;
    this._initSync(cacheDir);
  }

  _initSync(cacheDir) {
    // Try better-sqlite3 first (native, fast)
    if (Database) {
      try {
        const dbPath = path.join(cacheDir || process.env.CACHE_DIR || './data', 'grc-cache.db');
        this.db = new Database(dbPath);
        this.db.pragma('journal_mode = WAL');
        this.enabled = true;
        this._backend = 'better-sqlite3';
        this.initTables();
        return;
      } catch (e) {
        console.warn('[Cache] better-sqlite3 failed:', e.message);
      }
    }

    // Fallback: sql.js (async init)
    if (initSqlJs) {
      this._backend = 'sql.js';
      this._sqlJsReady = this._initSqlJs(cacheDir);
    } else {
      console.warn('[Cache] No SQLite library available, caching disabled');
    }
  }

  async _initSqlJs(cacheDir) {
    try {
      const dbPath = path.join(cacheDir || process.env.CACHE_DIR || './data', 'grc-cache.db');
      const wrapper = new SqlJsWrapper(dbPath);
      await wrapper.init();
      this.db = wrapper;
      this.enabled = true;
      this.initTables();
      console.log('  [Cache] Using sql.js (pure JavaScript SQLite fallback)');
      return true;
    } catch (e) {
      console.warn('[Cache] sql.js init failed:', e.message);
      return false;
    }
  }

  async ensureReady() {
    if (this._sqlJsReady) {
      await this._sqlJsReady;
    }
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
    if (!this.enabled) return { violations: 0, evidence: 0, error: 'Cache is disabled — no SQLite library available' };
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
