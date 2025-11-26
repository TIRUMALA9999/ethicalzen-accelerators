# Metrics Service

**Telemetry Sidecar for ACVPS Gateway**

Collects, stores, and serves observability data (requests, violations, response times) for the EthicalZen platform dashboard.

## Architecture

```
ACVPS Gateway         Metrics Service          Portal Dashboard
     │                      │                        │
     │  POST /ingest/batch  │                        │
     ├─────────────────────>│                        │
     │  (async, batched)    │                        │
     │                      │                        │
     │                      │  GET /metrics/summary  │
     │                      │<───────────────────────┤
     │                      │                        │
     │                      │  GET /requests/recent  │
     │                      │<───────────────────────┤
```

**Key Design Principles:**
- **Non-blocking**: ACVPS Gateway buffers events in memory and sends batches asynchronously
- **Fail-open**: If metrics service is down, gateway continues processing requests
- **Batching**: 100 events or 5 seconds (configurable)
- **Multi-tenant**: All data isolated by `tenant_id`

## Quick Start

### Local Development (SQLite)

```bash
# Install dependencies
npm install

# Initialize database
npm run init-db

# Start server
npm start
```

The service will start on `http://localhost:8090`.

### Production (PostgreSQL)

1. Update `.env`:
```bash
DB_TYPE=postgres
PG_HOST=localhost
PG_PORT=5432
PG_USER=ethicalzen
PG_PASSWORD=your_password
PG_DATABASE=ethicalzen_metrics
```

2. Run:
```bash
npm run init-db
npm start
```

## API Endpoints

### Ingestion APIs (from ACVPS Gateway)

#### `POST /ingest/batch`
Batch insert for requests and violations.

**Headers:**
- `X-API-Key`: Optional API key for authentication

**Body:**
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
      "response_time_ms": 118
    }
  ],
  "violations": [
    {
      "timestamp": "2025-11-06T10:45:12Z",
      "tenant_id": "tenant-001",
      "trace_id": "trace-xyz456",
      "contract_id": "client-profile/finance/us/v1.0",
      "violation_type": "pii_leakage",
      "metric_name": "pii_risk",
      "metric_value": 0.89,
      "threshold_max": 0.05,
      "severity": "high",
      "details": "SSN detected in response"
    }
  ]
}
```

### Query APIs (for Dashboard)

#### `GET /metrics/summary?tenant_id=X&period=today`
Returns aggregated metrics.

**Response:**
```json
{
  "success": true,
  "metrics": {
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
}
```

#### `GET /requests/recent?tenant_id=X&limit=50`
Returns recent requests.

#### `GET /violations/recent?tenant_id=X&limit=50`
Returns recent violations.

#### `GET /health`
Health check endpoint.

## Configuration

### Environment Variables

| Variable | Default | Description |
|----------|---------|-------------|
| `PORT` | 8090 | Server port |
| `DB_TYPE` | sqlite | Database type (sqlite or postgres) |
| `SQLITE_PATH` | ./data/metrics.db | SQLite database path |
| `METRICS_RETENTION_DAYS` | 90 | Days to keep metrics |
| `REQUESTS_RETENTION_DAYS` | 30 | Days to keep raw requests |
| `VIOLATIONS_RETENTION_DAYS` | 90 | Days to keep violations |
| `CORS_ORIGIN` | http://localhost:8080 | CORS origin |
| `INGESTION_API_KEY` | - | Optional API key for ingestion endpoints |

### ACVPS Gateway Configuration

The ACVPS Gateway needs these environment variables to send telemetry:

```bash
METRICS_SERVICE_URL=http://localhost:8090
METRICS_ENABLED=true
METRICS_BATCH_SIZE=100          # Events per batch
METRICS_BATCH_INTERVAL=5s       # Max time to hold batch
METRICS_BUFFER_SIZE=1000        # Max in-memory buffer
METRICS_API_KEY=your-key        # Optional
```

## Database Schema

### `requests` Table
Stores all requests processed by ACVPS Gateway.

| Column | Type | Description |
|--------|------|-------------|
| id | INTEGER/SERIAL | Primary key |
| timestamp | DATETIME | Request time |
| tenant_id | TEXT | Multi-tenant isolation |
| trace_id | TEXT | Unique trace ID |
| contract_id | TEXT | Which contract was enforced |
| certificate_id | TEXT | Certificate used (if any) |
| status_code | INTEGER | HTTP status code |
| response_time_ms | INTEGER | Request duration |

### `violations` Table
Stores all contract violations.

| Column | Type | Description |
|--------|------|-------------|
| id | INTEGER/SERIAL | Primary key |
| timestamp | DATETIME | Violation time |
| tenant_id | TEXT | Multi-tenant isolation |
| trace_id | TEXT | Links to request |
| contract_id | TEXT | Contract violated |
| violation_type | TEXT | pii_leakage, hallucination, etc. |
| metric_name | TEXT | Which metric violated |
| metric_value | REAL | Actual value |
| threshold_min/max | REAL | Expected bounds |
| severity | TEXT | high/medium/low |

### `metrics_hourly` Table
Pre-computed hourly rollups for fast dashboard queries.

## Performance

- **Ingestion Rate**: 10,000+ events/second (batched inserts)
- **Query Latency**: <50ms for dashboard queries
- **Storage**: ~1KB per request, ~500B per violation
- **Scalability**: Single instance handles 1M requests/day easily

## Monitoring

Check service health:
```bash
curl http://localhost:8090/health
```

View logs:
```bash
tail -f /tmp/metrics-service.log
```

Check database:
```bash
# SQLite
sqlite3 ./data/metrics.db "SELECT COUNT(*) FROM requests;"

# PostgreSQL
psql -d ethicalzen_metrics -c "SELECT COUNT(*) FROM requests;"
```

## Deployment

### Standalone
```bash
./start.sh
```

### With Full Platform
```bash
cd /path/to/ethicalzen
./start-all-with-metrics.sh
```

### Docker (Future)
```bash
docker-compose up metrics-service
```

## Troubleshooting

**Q: Dashboard shows "Unable to load metrics"**
- Ensure metrics service is running: `curl http://localhost:8090/health`
- Check CORS settings in `.env`
- Check browser console for errors

**Q: No data appearing in dashboard**
- Ensure ACVPS Gateway has `METRICS_ENABLED=true`
- Check Gateway is sending telemetry: `tail -f /tmp/acvps-gateway.log | grep Telemetry`
- Check metrics service logs: `tail -f /tmp/metrics-service.log`

**Q: Database errors**
- Re-run `npm run init-db`
- Check database permissions
- For PostgreSQL, ensure database exists and user has correct permissions

## License

MIT

