#!/usr/bin/env node
/**
 * Metrics Service - Telemetry Sidecar for ACVPS Gateway
 * 
 * Receives batched telemetry from ACVPS Gateway and serves metrics APIs for dashboard
 */

require('dotenv').config();
const express = require('express');
const cors = require('cors');
const db = require('./db');
const https = require('https');
const http = require('http');

const PORT = process.env.PORT || 8090;
const CORS_ORIGIN = process.env.CORS_ORIGIN || 'http://localhost:8080';
const INGESTION_API_KEY = process.env.INGESTION_API_KEY;

// Backend forwarding configuration (uses customer API key)
const BACKEND_FORWARD_ENABLED = process.env.BACKEND_FORWARD_ENABLED === 'true';
const BACKEND_URL = process.env.BACKEND_URL || process.env.CONTROL_PLANE_URL;
const CUSTOMER_API_KEY = process.env.ETHICALZEN_API_KEY; // Customer API key from portal

const app = express();

// Middleware
app.use(cors({ origin: CORS_ORIGIN }));
app.use(express.json({ limit: '10mb' })); // Large batches

// Initialize database
db.initDB();

// ============================================================================
// BACKEND FORWARDING (for VPC/Production deployments)
// ============================================================================

/**
 * Forward telemetry to cloud backend (async, non-blocking)
 * Authenticates using customer API key from portal
 */
async function forwardToBackend(requests, violations) {
    if (!BACKEND_FORWARD_ENABLED || !BACKEND_URL || !CUSTOMER_API_KEY) {
        return; // Forwarding disabled or not configured
    }

    try {
        const url = new URL('/api/evidence/batch', BACKEND_URL);
        const payload = JSON.stringify({ requests, violations });
        
        const options = {
            method: 'POST',
            headers: {
                'Content-Type': 'application/json',
                'Content-Length': Buffer.byteLength(payload),
                'Authorization': `Bearer ${CUSTOMER_API_KEY}`, // Customer API key
                'X-API-Key': CUSTOMER_API_KEY // Also support X-API-Key header
            }
        };

        const protocol = url.protocol === 'https:' ? https : http;
        
        const req = protocol.request(url, options, (res) => {
            if (res.statusCode === 200 || res.statusCode === 201) {
                console.log(`✅ Forwarded ${requests.length} requests, ${violations.length} violations to control plane`);
            } else {
                console.warn(`⚠️  Control plane returned status ${res.statusCode} for telemetry forwarding`);
                // Log response body for debugging
                let body = '';
                res.on('data', chunk => body += chunk);
                res.on('end', () => {
                    if (body) console.warn(`   Response: ${body}`);
                });
            }
        });

        req.on('error', (err) => {
            // Silent failure - local metrics still work even if backend is down
            console.warn(`⚠️  Failed to forward telemetry to control plane: ${err.message}`);
        });

        req.write(payload);
        req.end();

    } catch (err) {
        console.warn(`⚠️  Error forwarding telemetry: ${err.message}`);
    }
}

// ============================================================================
// INGESTION APIs (from ACVPS Gateway)
// ============================================================================

/**
 * Auth middleware for ingestion endpoints
 */
function authIngestion(req, res, next) {
    if (!INGESTION_API_KEY) {
        // No API key configured - allow (for local development)
        return next();
    }

    const providedKey = req.headers['x-api-key'];
    if (providedKey !== INGESTION_API_KEY) {
        return res.status(401).json({
            success: false,
            error: 'Unauthorized',
            message: 'Invalid or missing X-API-Key header'
        });
    }

    next();
}

/**
 * POST /ingest/batch
 * Batch insert for requests and violations
 */
app.post('/ingest/batch', authIngestion, async (req, res) => {
    try {
        const { requests = [], violations = [] } = req.body;

        let requestsInserted = 0;
        let violationsInserted = 0;

        // Insert requests
        if (requests.length > 0) {
            const insertSQL = `
                INSERT INTO requests (
                    timestamp, tenant_id, trace_id, contract_id, certificate_id,
                    method, path, status_code, response_time_ms,
                    request_size_bytes, response_size_bytes, user_agent, ip_address
                ) VALUES ${requests.map(() => '(?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)').join(', ')}
            `;

            const values = [];
            for (const r of requests) {
                values.push(
                    r.timestamp || new Date().toISOString(),
                    r.tenant_id,
                    r.trace_id,
                    r.contract_id,
                    r.certificate_id || null,
                    r.method,
                    r.path,
                    r.status_code,
                    r.response_time_ms,
                    r.request_size_bytes || 0,
                    r.response_size_bytes || 0,
                    r.user_agent || null,
                    r.ip_address || null
                );
            }

            const result = await db.query(insertSQL, values);
            requestsInserted = requests.length;
        }

        // Insert violations
        if (violations.length > 0) {
            const insertSQL = `
                INSERT INTO violations (
                    timestamp, tenant_id, trace_id, contract_id, certificate_id,
                    violation_type, metric_name, metric_value,
                    threshold_min, threshold_max, severity, details
                ) VALUES ${violations.map(() => '(?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)').join(', ')}
            `;

            const values = [];
            for (const v of violations) {
                values.push(
                    v.timestamp || new Date().toISOString(),
                    v.tenant_id,
                    v.trace_id,
                    v.contract_id,
                    v.certificate_id || null,
                    v.violation_type,
                    v.metric_name,
                    v.metric_value,
                    v.threshold_min || null,
                    v.threshold_max || null,
                    v.severity || 'medium',
                    v.details || null
                );
            }

            const result = await db.query(insertSQL, values);
            violationsInserted = violations.length;
        }

        // Forward to backend (async, non-blocking)
        if (BACKEND_FORWARD_ENABLED) {
            forwardToBackend(requests, violations).catch(err => {
                console.warn(`⚠️  Backend forwarding failed: ${err.message}`);
            });
        }

        res.json({
            success: true,
            inserted: {
                requests: requestsInserted,
                violations: violationsInserted
            },
            forwarded: BACKEND_FORWARD_ENABLED
        });
    } catch (error) {
        console.error('❌ Batch insert failed:', error);
        res.status(500).json({
            success: false,
            error: 'Internal server error',
            message: error.message
        });
    }
});

// ============================================================================
// QUERY APIs (for Dashboard)
// ============================================================================

/**
 * Tenant isolation middleware
 * Extracts tenant_id from query or JWT (future enhancement)
 */
function extractTenant(req, res, next) {
    // For now, use query param (in production, extract from JWT)
    const tenantId = req.query.tenant_id || req.headers['x-tenant-id'] || 'default';
    req.tenantId = tenantId;
    next();
}

/**
 * GET /metrics/summary
 * Aggregated metrics for dashboard cards
 */
app.get('/metrics/summary', extractTenant, async (req, res) => {
    try {
        const { period = 'today' } = req.query;
        const tenantId = req.tenantId;

        // Calculate time range
        const now = new Date();
        let startTime, previousStartTime;

        if (period === 'today') {
            startTime = new Date(now.getFullYear(), now.getMonth(), now.getDate());
            previousStartTime = new Date(startTime.getTime() - 24 * 60 * 60 * 1000);
        } else if (period === '7d') {
            startTime = new Date(now.getTime() - 7 * 24 * 60 * 60 * 1000);
            previousStartTime = new Date(startTime.getTime() - 7 * 24 * 60 * 60 * 1000);
        } else {
            startTime = new Date(now.getTime() - 24 * 60 * 60 * 1000);
            previousStartTime = new Date(startTime.getTime() - 24 * 60 * 60 * 1000);
        }

        // Requests today
        const requestsToday = await db.queryOne(`
            SELECT COUNT(*) as count
            FROM requests
            WHERE tenant_id = ? AND timestamp >= ?
        `, [tenantId, startTime.toISOString()]);

        const requestsYesterday = await db.queryOne(`
            SELECT COUNT(*) as count
            FROM requests
            WHERE tenant_id = ? AND timestamp >= ? AND timestamp < ?
        `, [tenantId, previousStartTime.toISOString(), startTime.toISOString()]);

        const requestsChange = requestsYesterday?.count > 0
            ? ((requestsToday?.count - requestsYesterday?.count) / requestsYesterday?.count * 100).toFixed(1)
            : 0;

        // Avg response time
        const avgResponseTime = await db.queryOne(`
            SELECT AVG(response_time_ms) as avg_ms
            FROM requests
            WHERE tenant_id = ? AND timestamp >= ?
        `, [tenantId, startTime.toISOString()]);

        const prevAvgResponseTime = await db.queryOne(`
            SELECT AVG(response_time_ms) as avg_ms
            FROM requests
            WHERE tenant_id = ? AND timestamp >= ? AND timestamp < ?
        `, [tenantId, previousStartTime.toISOString(), startTime.toISOString()]);

        const responseTimeChange = Math.round(
            (avgResponseTime?.avg_ms || 0) - (prevAvgResponseTime?.avg_ms || 0)
        );

        // Success rate
        const successRate = await db.queryOne(`
            SELECT 
                COUNT(*) as total,
                SUM(CASE WHEN status_code >= 200 AND status_code < 300 THEN 1 ELSE 0 END) as successful
            FROM requests
            WHERE tenant_id = ? AND timestamp >= ?
        `, [tenantId, startTime.toISOString()]);

        const successPct = successRate?.total > 0
            ? ((successRate?.successful / successRate?.total) * 100).toFixed(1)
            : 100;

        // Violations today
        const violationsToday = await db.queryOne(`
            SELECT COUNT(*) as count
            FROM violations
            WHERE tenant_id = ? AND timestamp >= ?
        `, [tenantId, startTime.toISOString()]);

        const violationsYesterday = await db.queryOne(`
            SELECT COUNT(*) as count
            FROM violations
            WHERE tenant_id = ? AND timestamp >= ? AND timestamp < ?
        `, [tenantId, previousStartTime.toISOString(), startTime.toISOString()]);

        const violationsChange = violationsYesterday?.count > 0
            ? ((violationsToday?.count - violationsYesterday?.count) / violationsYesterday?.count * 100).toFixed(1)
            : 0;

        // Violation breakdown
        const violationBreakdown = await db.query(`
            SELECT 
                violation_type,
                COUNT(*) as count
            FROM violations
            WHERE tenant_id = ? AND timestamp >= ?
            GROUP BY violation_type
        `, [tenantId, startTime.toISOString()]);

        const breakdown = {
            pii_violations: 0,
            grounding_violations: 0,
            hallucination_violations: 0
        };

        for (const row of violationBreakdown) {
            if (row.violation_type === 'pii_leakage') {
                breakdown.pii_violations = row.count;
            } else if (row.violation_type === 'low_grounding') {
                breakdown.grounding_violations = row.count;
            } else if (row.violation_type === 'hallucination') {
                breakdown.hallucination_violations = row.count;
            }
        }

        res.json({
            success: true,
            period,
            metrics: {
                requests_today: requestsToday?.count || 0,
                requests_change_pct: parseFloat(requestsChange),
                avg_response_time_ms: Math.round(avgResponseTime?.avg_ms || 0),
                response_time_change_ms: responseTimeChange,
                success_rate: parseFloat(successPct),
                violations_today: violationsToday?.count || 0,
                violations_change_pct: parseFloat(violationsChange),
                ...breakdown
            }
        });
    } catch (error) {
        console.error('❌ Summary query failed:', error);
        res.status(500).json({
            success: false,
            error: 'Internal server error',
            message: error.message
        });
    }
});

/**
 * GET /requests/recent
 * Recent requests for table display
 */
app.get('/requests/recent', extractTenant, async (req, res) => {
    try {
        const { limit = 50 } = req.query;
        const tenantId = req.tenantId;

        const requests = await db.query(`
            SELECT 
                timestamp,
                trace_id,
                contract_id,
                method,
                path,
                status_code,
                response_time_ms,
                certificate_id
            FROM requests
            WHERE tenant_id = ?
            ORDER BY timestamp DESC
            LIMIT ?
        `, [tenantId, parseInt(limit)]);

        res.json({
            success: true,
            requests: requests.map(r => ({
                ...r,
                status_text: `${r.status_code} ${r.status_code < 300 ? 'OK' : 'Blocked'}`
            }))
        });
    } catch (error) {
        console.error('❌ Recent requests query failed:', error);
        res.status(500).json({
            success: false,
            error: 'Internal server error',
            message: error.message
        });
    }
});

/**
 * GET /violations/recent
 * Recent violations for table display
 */
app.get('/violations/recent', extractTenant, async (req, res) => {
    try {
        const { limit = 50 } = req.query;
        const tenantId = req.tenantId;

        const violations = await db.query(`
            SELECT 
                timestamp,
                trace_id,
                contract_id,
                violation_type,
                metric_name,
                metric_value,
                threshold_min,
                threshold_max,
                severity,
                details
            FROM violations
            WHERE tenant_id = ?
            ORDER BY timestamp DESC
            LIMIT ?
        `, [tenantId, parseInt(limit)]);

        res.json({
            success: true,
            violations
        });
    } catch (error) {
        console.error('❌ Recent violations query failed:', error);
        res.status(500).json({
            success: false,
            error: 'Internal server error',
            message: error.message
        });
    }
});

// ============================================================================
// HEALTH & STATUS
// ============================================================================

app.get('/health', (req, res) => {
    res.json({
        success: true,
        service: 'metrics-service',
        version: '1.0.0',
        uptime: process.uptime(),
        db_type: process.env.DB_TYPE || 'sqlite'
    });
});

// ============================================================================
// START SERVER
// ============================================================================

app.listen(PORT, () => {
    console.log('');
    console.log('📊 ═══════════════════════════════════════════════════════');
    console.log('📊  Metrics Service - ACVPS Telemetry Sidecar');
    console.log('📊 ═══════════════════════════════════════════════════════');
    console.log(`📊  Server: http://localhost:${PORT}`);
    console.log(`📊  Database: ${process.env.DB_TYPE || 'sqlite'}`);
    console.log(`📊  CORS Origin: ${CORS_ORIGIN}`);
    console.log(`📊  Ingestion Auth: ${INGESTION_API_KEY ? '✅ Enabled' : '⚠️  Disabled (local dev)'}`);
    console.log(`📊  Control Plane Forward: ${BACKEND_FORWARD_ENABLED && CUSTOMER_API_KEY ? '✅ Enabled' : '⚠️  Disabled'}`);
    console.log('📊 ═══════════════════════════════════════════════════════');
    console.log('');
    console.log('📡 Ingestion Endpoints:');
    console.log(`   POST http://localhost:${PORT}/ingest/batch`);
    console.log('');
    console.log('📈 Query Endpoints:');
    console.log(`   GET  http://localhost:${PORT}/metrics/summary?tenant_id=default`);
    console.log(`   GET  http://localhost:${PORT}/requests/recent?tenant_id=default`);
    console.log(`   GET  http://localhost:${PORT}/violations/recent?tenant_id=default`);
    console.log('');
    console.log('✅ Ready to receive telemetry from ACVPS Gateway');
    console.log('');
});

