# EthicalZen GRC Compliance Dashboard

Enterprise-grade Governance, Risk & Compliance dashboard that connects to the EthicalZen cloud platform. Monitor AI guardrail violations in real-time, generate compliance reports in OSCAL 1.1.2, STIX 2.1, and browse TAXII 2.1 threat intelligence — across NIST AI RMF, ISO 42001, and NIST CSF 2.0 frameworks.

## Quick Start

### Docker (recommended)

```bash
cp .env.example .env
# Edit .env with your API key and tenant ID

docker compose up -d
# Dashboard at http://localhost:3000
```

### Node.js

```bash
npm install
cp .env.example .env
# Edit .env with your API key and tenant ID

npm start
# Dashboard at http://localhost:3000
```

## Configuration

| Variable | Description | Default |
|----------|-------------|---------|
| `ETHICALZEN_API_URL` | Cloud API endpoint (provided during onboarding) | — |
| `ETHICALZEN_API_KEY` | Your API key (never exposed to browser) | — |
| `ETHICALZEN_TENANT_ID` | Tenant identifier | — |
| `PORT` | Dashboard port | `3000` |
| `POLL_INTERVAL_MS` | Violation poll interval | `10000` |
| `CACHE_DIR` | SQLite cache directory | `./data` |

## Features

### 9 Dashboard Views

| View | Description |
|------|-------------|
| **Executive Dashboard** | KPI cards, violation timeline, risk gauge, quick actions |
| **Violations** | Real-time SSE feed with filters and severity breakdown |
| **Evidence Trail** | Searchable audit log of all guardrail evaluations |
| **Export Builder** | Generate OSCAL 1.1.2 and STIX 2.1 reports with preview |
| **Compliance Matrix** | Control coverage across NIST AI RMF, ISO 42001, NIST CSF |
| **Risk Overview** | Aggregated risk score (0-100) with per-framework breakdown |
| **TAXII Browser** | Browse TAXII 2.1 collections and STIX objects |
| **Drift Alerts** | Monitor guardrail performance drift and anomalies |
| **Settings** | API connection, cache management, theme |

### Key Capabilities

- **Real-time monitoring** — SSE-based live violation feed, auto-updating dashboard
- **Offline mode** — SQLite cache serves data when cloud is unreachable (shown as "Cached")
- **Zero framework** — Pure HTML/CSS/JS frontend, no build step required
- **Dark/light theme** — System-aware with manual toggle
- **Demo mode** — Seed sample data for offline demos without an API key
- **Enterprise formats** — OSCAL 1.1.2 Assessment Results, STIX 2.1 Bundles, TAXII 2.1

## Architecture

```
Browser (:3000)                  GRC Dashboard Server              EthicalZen Cloud
┌─────────────────┐            ┌─────────────────────┐          ┌──────────────────┐
│  SPA Frontend   │   fetch    │  Express Proxy       │  HTTPS   │ Your Cloud API   │
│  (HTML/CSS/JS)  │──────────> │  + Background Poller │────────> │                  │
│                 │   SSE      │  + SQLite Cache      │          │ Violations API   │
│  9 Views        │<────────── │  + Risk Aggregator   │          │ Evidence API     │
└─────────────────┘            └─────────────────────┘          │ GRC Export API   │
                                                                │ TAXII 2.1 Server │
                                                                └──────────────────┘
```

API keys are **never exposed to the browser** — all cloud requests are proxied through the local server with `X-API-Key` and `X-Tenant-ID` headers injected server-side.

## Compliance Formats

| Format | Version | Use Case |
|--------|---------|----------|
| **OSCAL** | 1.1.2 | NIST-standard assessment results for FedRAMP, auditors |
| **STIX** | 2.1 | Threat intelligence bundles for SOC/SIEM integration |
| **TAXII** | 2.1 | Automated threat intel sharing via collection API |

## Frameworks

| Framework | Coverage |
|-----------|----------|
| **NIST AI RMF** | Govern, Map, Measure, Manage functions |
| **ISO 42001** | AI management system controls (Annex A, B) |
| **NIST CSF 2.0** | Identify, Protect, Detect, Respond, Recover |

## API Proxy Routes

| Dashboard Route | Purpose |
|-----------------|---------|
| `GET /api/grc/violations` | Guardrail violation feed |
| `GET /api/grc/violations/stream` | SSE live stream (backed by poller) |
| `GET /api/grc/evidence` | Evidence audit trail |
| `POST /api/grc/export/oscal` | Generate OSCAL 1.1.2 report |
| `POST /api/grc/export/stix` | Generate STIX 2.1 bundle |
| `GET /api/grc/taxii/discovery` | TAXII 2.1 server discovery |
| `GET /api/grc/taxii/collections` | TAXII collection listing |
| `GET /api/grc/risk` | Aggregated risk score (local computation) |
| `GET /api/grc/health` | Connection + cache status |

## Development

```bash
# Install dependencies
npm install

# Start with auto-reload
npm run dev

# Seed demo data (for offline testing)
curl -X POST http://localhost:3000/api/grc/cache/seed-demo
```

## License

Proprietary — EthicalZen, Inc.
