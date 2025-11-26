# Metrics Service Architecture

## Overview
Lightweight telemetry sidecar for ACVPS Gateway that collects, stores, and serves metrics/observability data.

## Data Flow

```
┌─────────────────────────────────────────────────────────────────┐
│                      ACVPS Gateway                              │
│                                                                 │
│  ┌──────────────┐     ┌─────────────────┐                     │
│  │  Request     │────▶│ In-Memory       │                     │
│  │  Handler     │     │ Telemetry       │                     │
│  │              │     │ Buffer          │                     │
│  │ (Fast Path)  │     │ (100 events or  │                     │
│  └──────────────┘     │  5 seconds)     │                     │
│                       └────────┬────────┘                     │
│                                │                               │
│                      ┌─────────▼─────────┐                    │
│                      │ Background        │                    │
│                      │ Batcher           │                    │
│                      │ (goroutine)       │                    │
│                      └─────────┬─────────┘                    │
└────────────────────────────────┼──────────────────────────────┘
                                 │ HTTP POST /ingest/batch
                                 │ (async, non-blocking)
                                 ▼
┌─────────────────────────────────────────────────────────────────┐
│                    Metrics Service                              │
│                                                                 │
│  ┌──────────────┐     ┌──────────────┐     ┌──────────────┐  │
│  │ Ingestion    │────▶│   SQLite/    │◀────│  Query API   │  │
│  │ API          │     │  PostgreSQL  │     │              │  │
│  │ /ingest/*    │     │              │     │ /metrics/*   │  │
│  └──────────────┘     └──────────────┘     └──────┬───────┘  │
│                                                     │          │
└─────────────────────────────────────────────────────┼──────────┘
                                                      │
                                                      ▼
                                          ┌──────────────────────┐
                                          │   Dashboard UI       │
                                          │                      │
                                          │  - Requests tab      │
                                          │  - Violations tab    │
                                          │  - Real-time stats   │
                                          └──────────────────────┘
```

## Design Principles

### 1. **Non-Blocking Telemetry**
- Telemetry collection NEVER blocks request processing
- Gateway buffers events in memory (~1μs overhead)
- Async batch sends every 5s or 100 events
- If metrics service is down: drop telemetry (fail open)

### 2. **Batching for Efficiency**
- Batch size: 100 events (configurable)
- Batch interval: 5 seconds (configurable)
- Batch insert into DB (single transaction)
- Reduces network calls by 100x

### 3. **Backpressure Protection**
- If metrics service is slow/down, gateway doesn't care
- Buffer has max size (1000 events), then drops oldest
- Telemetry is **nice-to-have**, not critical path

### 4. **Multi-Tenant Isolation**
- All tables have `tenant_id` column
- API queries filter by tenant (from JWT or API key)
- No cross-tenant data leakage

### 5. **Data Retention**
- Raw requests: 30 days (configurable)
- Raw violations: 90 days
- Hourly aggregates: 1 year
- Auto-cleanup via cron job

## API Specification

### Ingestion APIs (from ACVPS Gateway)

#### `POST /ingest/batch`
Batch insert for requests and violations.

```json
{
  "requests": [
    {
      "timestamp": "2025-11-06T10:45:23Z",
      "tenant_id": "tenant-001",
      "trace_id": "trace-abc123",
      "contract_id": "patient-intake/healthcare/us/v1.0",
      "certificate_id": "cert-xyz789",
      "method": "POST",
      "path": "/api/intake",
      "status_code": 200,
      "response_time_ms": 118,
      "request_size_bytes": 1024,
      "response_size_bytes": 512,
      "ip_address": "192.168.1.1"
    }
  ],
  "violations": [
    {
      "timestamp": "2025-11-06T10:45:12Z",
      "tenant_id": "tenant-001",
      "trace_id": "trace-xyz456",
      "contract_id": "client-profile/finance/us/v1.0",
      "certificate_id": "cert-abc123",
      "violation_type": "pii_leakage",
      "metric_name": "pii_risk",
      "metric_value": 0.89,
      "threshold_min": 0.0,
      "threshold_max": 0.05,
      "severity": "high",
      "details": "SSN detected in response"
    }
  ]
}
```

**Response:**
```json
{
  "success": true,
  "inserted": {
    "requests": 1,
    "violations": 1
  }
}
```

### Query APIs (for Dashboard)

#### `GET /metrics/summary?tenant_id=X&period=today`
Returns aggregated metrics for dashboard cards.

```json
{
  "requests_today": 1234,
  "requests_change_pct": 8.0,
  "avg_response_time_ms": 124,
  "response_time_change_ms": -12,
  "success_rate": 98.4,
  "violations_today": 42,
  "violations_change_pct": -15.0,
  "pii_violations": 18,
  "grounding_violations": 12,
  "hallucination_violations": 24
}
```

#### `GET /requests/recent?tenant_id=X&limit=50`
Returns recent requests for table display.

```json
{
  "requests": [
    {
      "timestamp": "2025-11-06T10:45:23Z",
      "trace_id": "trace-abc123",
      "contract_id": "patient-intake/healthcare/us/v1.0",
      "response_time_ms": 118,
      "status_code": 200,
      "status_text": "200 OK"
    }
  ]
}
```

#### `GET /violations/recent?tenant_id=X&limit=50`
Returns recent violations for table display.

```json
{
  "violations": [
    {
      "timestamp": "2025-11-06T10:45:12Z",
      "violation_type": "pii_leakage",
      "contract_id": "client-profile/finance/us/v1.0",
      "metric_name": "pii_risk",
      "metric_value": 0.89,
      "threshold_max": 0.05,
      "details": "SSN detected in response"
    }
  ]
}
```

#### `GET /metrics/timeseries?tenant_id=X&metric=requests&period=24h`
Returns time-series data for charts (future enhancement).

## Database Schema

### `requests` Table
- `id`: Primary key
- `timestamp`: Request time (indexed)
- `tenant_id`: Multi-tenant isolation (indexed)
- `trace_id`: Unique trace ID (indexed, for correlating violations)
- `contract_id`: Which contract was enforced (indexed)
- `certificate_id`: Certificate used
- `method`, `path`: HTTP details
- `status_code`: Response status (indexed for filtering)
- `response_time_ms`: Duration
- `request_size_bytes`, `response_size_bytes`: Size metrics
- `ip_address`, `user_agent`: Client info

### `violations` Table
- `id`: Primary key
- `timestamp`: Violation time (indexed)
- `tenant_id`: Multi-tenant isolation (indexed)
- `trace_id`: Links to request (foreign key)
- `contract_id`: Contract violated (indexed)
- `certificate_id`: Certificate used
- `violation_type`: Category (pii_leakage, hallucination, etc.) (indexed)
- `metric_name`: Which metric violated
- `metric_value`, `threshold_min`, `threshold_max`: Numeric details
- `severity`: high/medium/low
- `details`: Human-readable explanation

### `metrics_hourly` Table (Aggregates)
- Pre-computed hourly rollups for fast dashboard queries
- Updated via background job every hour
- Dramatically speeds up queries like "requests today"

## Performance Characteristics

### Gateway Overhead
- **Per-request overhead**: ~1-2μs (in-memory buffer append)
- **Network overhead**: 0 (async background thread)
- **Blocking**: None (telemetry never blocks request handling)

### Metrics Service
- **Ingestion rate**: 10,000+ events/second (batched inserts)
- **Query latency**: <50ms for dashboard queries (using indexes + aggregates)
- **Storage**: ~1KB per request, ~500B per violation

### Scalability
- **Single instance**: Handles 1M requests/day easily
- **Horizontal scaling**: Add read replicas for query load
- **Vertical scaling**: SQLite → PostgreSQL for 100M+ requests/day

## Configuration

### Gateway Environment Variables
```bash
METRICS_SERVICE_URL=http://localhost:8090
METRICS_BATCH_SIZE=100          # Events per batch
METRICS_BATCH_INTERVAL=5s       # Max time to hold batch
METRICS_BUFFER_SIZE=1000        # Max in-memory buffer
METRICS_ENABLED=true            # Feature flag
```

### Metrics Service Environment Variables
```bash
PORT=8090
DB_TYPE=sqlite                  # or postgres
SQLITE_PATH=./data/metrics.db
METRICS_RETENTION_DAYS=90
REQUESTS_RETENTION_DAYS=30
CORS_ORIGIN=http://localhost:8080
INGESTION_API_KEY=secret-key    # Optional auth for /ingest/*
```

## Deployment Topologies

### Local Development
```
docker-compose:
  - acvps-gateway (port 8443)
  - metrics-service (port 8090, SQLite)
  - portal-backend (port 8080)
```

### Production (Cloud)
```
K8s:
  - acvps-gateway (DaemonSet, one per node)
  - metrics-service (Deployment, 2-3 replicas, PostgreSQL)
  - postgresql (StatefulSet or managed RDS)
```

### Production (Edge)
```
Each edge location:
  - acvps-gateway → local metrics-service (SQLite)
  - Periodic sync to central PostgreSQL (optional)
```

## Alternatives Considered

### ❌ Pull Model (Polling)
- Metrics service polls gateway every N seconds
- **Rejected**: Adds load to gateway, stale data, need to implement endpoint

### ❌ Synchronous Logging
- Gateway makes HTTP call per request
- **Rejected**: Adds latency, blocks request, single point of failure

### ✅ Push with Batching (CHOSEN)
- Gateway buffers and sends batches asynchronously
- **Why**: Near-zero overhead, real-time, fail-open

### 🔶 Message Queue (Future)
- Gateway → Kafka → Metrics Service
- **When**: If handling 100M+ requests/day across multiple gateways

## Future Enhancements

1. **Distributed Tracing**: Full trace correlation
2. **Alerting**: Webhooks when violation threshold exceeded
3. **Charts**: Time-series visualization in dashboard
4. **Export**: Prometheus metrics endpoint
5. **Anomaly Detection**: ML-based outlier detection
6. **Cost Tracking**: Token usage and billing by tenant

