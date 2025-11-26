#!/usr/bin/env node
/**
 * Database Initialization Script
 * Creates tables for metrics, requests, and violations
 */

require('dotenv').config();
const fs = require('fs');
const path = require('path');

const DB_TYPE = process.env.DB_TYPE || 'sqlite';

// SQLite Schema
const SQLITE_SCHEMA = `
-- Requests Table
CREATE TABLE IF NOT EXISTS requests (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    timestamp DATETIME DEFAULT CURRENT_TIMESTAMP,
    tenant_id TEXT NOT NULL,
    trace_id TEXT UNIQUE NOT NULL,
    contract_id TEXT NOT NULL,
    certificate_id TEXT,
    method TEXT NOT NULL,
    path TEXT NOT NULL,
    status_code INTEGER NOT NULL,
    response_time_ms INTEGER NOT NULL,
    request_size_bytes INTEGER DEFAULT 0,
    response_size_bytes INTEGER DEFAULT 0,
    user_agent TEXT,
    ip_address TEXT
);

CREATE INDEX IF NOT EXISTS idx_requests_timestamp ON requests(timestamp DESC);
CREATE INDEX IF NOT EXISTS idx_requests_tenant ON requests(tenant_id);
CREATE INDEX IF NOT EXISTS idx_requests_contract ON requests(contract_id);
CREATE INDEX IF NOT EXISTS idx_requests_status ON requests(status_code);

-- Violations Table
CREATE TABLE IF NOT EXISTS violations (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    timestamp DATETIME DEFAULT CURRENT_TIMESTAMP,
    tenant_id TEXT NOT NULL,
    trace_id TEXT NOT NULL,
    contract_id TEXT NOT NULL,
    certificate_id TEXT,
    violation_type TEXT NOT NULL,
    metric_name TEXT NOT NULL,
    metric_value REAL NOT NULL,
    threshold_min REAL,
    threshold_max REAL,
    severity TEXT DEFAULT 'medium',
    details TEXT
);

CREATE INDEX IF NOT EXISTS idx_violations_timestamp ON violations(timestamp DESC);
CREATE INDEX IF NOT EXISTS idx_violations_tenant ON violations(tenant_id);
CREATE INDEX IF NOT EXISTS idx_violations_type ON violations(violation_type);
CREATE INDEX IF NOT EXISTS idx_violations_contract ON violations(contract_id);

-- Aggregated Metrics Table (for faster dashboard queries)
CREATE TABLE IF NOT EXISTS metrics_hourly (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    hour DATETIME NOT NULL,
    tenant_id TEXT NOT NULL,
    contract_id TEXT,
    total_requests INTEGER DEFAULT 0,
    successful_requests INTEGER DEFAULT 0,
    blocked_requests INTEGER DEFAULT 0,
    avg_response_time_ms REAL DEFAULT 0,
    total_violations INTEGER DEFAULT 0,
    pii_violations INTEGER DEFAULT 0,
    grounding_violations INTEGER DEFAULT 0,
    hallucination_violations INTEGER DEFAULT 0,
    UNIQUE(hour, tenant_id, contract_id)
);

CREATE INDEX IF NOT EXISTS idx_metrics_hour ON metrics_hourly(hour DESC);
CREATE INDEX IF NOT EXISTS idx_metrics_tenant ON metrics_hourly(tenant_id);
`;

// PostgreSQL Schema
const POSTGRES_SCHEMA = `
-- Requests Table
CREATE TABLE IF NOT EXISTS requests (
    id SERIAL PRIMARY KEY,
    timestamp TIMESTAMPTZ DEFAULT NOW(),
    tenant_id TEXT NOT NULL,
    trace_id TEXT UNIQUE NOT NULL,
    contract_id TEXT NOT NULL,
    certificate_id TEXT,
    method TEXT NOT NULL,
    path TEXT NOT NULL,
    status_code INTEGER NOT NULL,
    response_time_ms INTEGER NOT NULL,
    request_size_bytes INTEGER DEFAULT 0,
    response_size_bytes INTEGER DEFAULT 0,
    user_agent TEXT,
    ip_address TEXT
);

CREATE INDEX IF NOT EXISTS idx_requests_timestamp ON requests(timestamp DESC);
CREATE INDEX IF NOT EXISTS idx_requests_tenant ON requests(tenant_id);
CREATE INDEX IF NOT EXISTS idx_requests_contract ON requests(contract_id);
CREATE INDEX IF NOT EXISTS idx_requests_status ON requests(status_code);

-- Violations Table
CREATE TABLE IF NOT EXISTS violations (
    id SERIAL PRIMARY KEY,
    timestamp TIMESTAMPTZ DEFAULT NOW(),
    tenant_id TEXT NOT NULL,
    trace_id TEXT NOT NULL,
    contract_id TEXT NOT NULL,
    certificate_id TEXT,
    violation_type TEXT NOT NULL,
    metric_name TEXT NOT NULL,
    metric_value REAL NOT NULL,
    threshold_min REAL,
    threshold_max REAL,
    severity TEXT DEFAULT 'medium',
    details TEXT
);

CREATE INDEX IF NOT EXISTS idx_violations_timestamp ON violations(timestamp DESC);
CREATE INDEX IF NOT EXISTS idx_violations_tenant ON violations(tenant_id);
CREATE INDEX IF NOT EXISTS idx_violations_type ON violations(violation_type);
CREATE INDEX IF NOT EXISTS idx_violations_contract ON violations(contract_id);

-- Aggregated Metrics Table
CREATE TABLE IF NOT EXISTS metrics_hourly (
    id SERIAL PRIMARY KEY,
    hour TIMESTAMPTZ NOT NULL,
    tenant_id TEXT NOT NULL,
    contract_id TEXT,
    total_requests INTEGER DEFAULT 0,
    successful_requests INTEGER DEFAULT 0,
    blocked_requests INTEGER DEFAULT 0,
    avg_response_time_ms REAL DEFAULT 0,
    total_violations INTEGER DEFAULT 0,
    pii_violations INTEGER DEFAULT 0,
    grounding_violations INTEGER DEFAULT 0,
    hallucination_violations INTEGER DEFAULT 0,
    UNIQUE(hour, tenant_id, contract_id)
);

CREATE INDEX IF NOT EXISTS idx_metrics_hour ON metrics_hourly(hour DESC);
CREATE INDEX IF NOT EXISTS idx_metrics_tenant ON metrics_hourly(tenant_id);
`;

async function initDatabase() {
    console.log('🗄️  Initializing Metrics Database...');
    console.log(`Database Type: ${DB_TYPE}`);

    if (DB_TYPE === 'sqlite') {
        const Database = require('better-sqlite3');
        const dbPath = process.env.SQLITE_PATH || './data/metrics.db';
        const dbDir = path.dirname(dbPath);

        // Ensure data directory exists
        if (!fs.existsSync(dbDir)) {
            fs.mkdirSync(dbDir, { recursive: true });
            console.log(`✅ Created directory: ${dbDir}`);
        }

        const db = new Database(dbPath);
        console.log(`✅ Connected to SQLite: ${dbPath}`);

        // Execute schema
        db.exec(SQLITE_SCHEMA);
        console.log('✅ SQLite schema initialized');

        db.close();
    } else if (DB_TYPE === 'postgres') {
        const { Client } = require('pg');
        const client = new Client({
            host: process.env.PG_HOST,
            port: process.env.PG_PORT,
            user: process.env.PG_USER,
            password: process.env.PG_PASSWORD,
            database: process.env.PG_DATABASE,
        });

        await client.connect();
        console.log('✅ Connected to PostgreSQL');

        await client.query(POSTGRES_SCHEMA);
        console.log('✅ PostgreSQL schema initialized');

        await client.end();
    } else {
        console.error(`❌ Unsupported DB_TYPE: ${DB_TYPE}`);
        process.exit(1);
    }

    console.log('✅ Database initialization complete!');
}

// Run if called directly
if (require.main === module) {
    initDatabase().catch(err => {
        console.error('❌ Database initialization failed:', err);
        process.exit(1);
    });
}

module.exports = { initDatabase };

