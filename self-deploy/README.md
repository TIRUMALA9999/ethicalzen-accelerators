# EthicalZen Self-Deploy Accelerator

**One-click deployment** of EthicalZen runtime enforcement for your private cloud or on-premises environment.

## 🏗️ Architecture

```
┌─────────────────────────────────────────────────────────────────────────────────┐
│                    YOUR PRIVATE CLOUD / ON-PREMISES                              │
│                                                                                  │
│   ┌─────────────────────────────────────────────────────────────────────────┐   │
│   │                         CLIENT APPLICATION                               │   │
│   │   curl -X POST http://gateway/api/proxy \                               │   │
│   │     -H "X-API-Key: sk-your-key" \                                       │   │
│   │     -H "X-DC-Id: your-certificate-id" \                                 │   │
│   │     -H "X-Target-Endpoint: https://api.openai.com/v1/chat/completions"  │   │
│   └─────────────────────────────────────────────────────────────────────────┘   │
│                                      │                                           │
│                                      ▼                                           │
│   ┌─────────────────────────────────────────────────────────────────────────┐   │
│   │                     ETHICALZEN GATEWAY (:8080)                           │   │
│   │   • Authenticate tenant (X-API-Key)                                     │   │
│   │   • Load certificate guardrails                                         │   │
│   │   • Evaluate input against guardrails                                   │   │
│   │   • ALLOW → Forward to target API                                       │   │
│   │   • BLOCK → Return 403 with violation details                           │   │
│   └─────────────────────────────────────────────────────────────────────────┘   │
│            │                    │                    │                           │
│            ▼                    ▼                    ▼                           │
│   ┌─────────────┐    ┌─────────────────┐    ┌─────────────────┐                 │
│   │ Eval Engine │    │ Metrics Service │    │      Redis      │                 │
│   │   (:8091)   │    │    (:8090)      │    │     (:6379)     │                 │
│   │ Smart Guard │    │ Evidence Logs   │    │ Cache/Sessions  │                 │
│   └─────────────┘    └────────┬────────┘    └─────────────────┘                 │
│                               │                                                  │
│                               ▼                                                  │
│                      ┌─────────────────┐                                         │
│                      │      MySQL      │                                         │
│                      │     (:3306)     │                                         │
│                      │ Audit + Evidence│                                         │
│                      └─────────────────┘                                         │
│                                                                                  │
│   ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─   │
│                         HYBRID SYNC (bidirectional)                              │
│                               ↕                                                  │
│   ┌─────────────────────────────────────────────────────────────────────────┐   │
│   │                    ETHICALZEN CLOUD BACKEND                              │   │
│   │                                                                          │   │
│   │   INBOUND (Cloud → Gateway):                                            │   │
│   │   • Certificates & Contracts                                            │   │
│   │   • Guardrail calibrations                                              │   │
│   │   • Configuration updates                                               │   │
│   │                                                                          │   │
│   │   OUTBOUND (Gateway → Cloud) - Optional:                                │   │
│   │   • Evidence logs & audit trail                                         │   │
│   │   • Aggregated metrics                                                  │   │
│   │   • Dashboard analytics                                                 │   │
│   └─────────────────────────────────────────────────────────────────────────┘   │
└─────────────────────────────────────────────────────────────────────────────────┘
```

## 🔄 Data Flow

| Direction | Data | Purpose |
|-----------|------|---------|
| **↓ Inbound** | Certificates | Which guardrails apply to which use cases |
| **↓ Inbound** | Calibrations | Thresholds, embeddings for smart guardrails |
| **↓ Inbound** | Config updates | New guardrails, tuning changes |
| **↑ Outbound** | Evidence (optional) | Audit logs for compliance |
| **↑ Outbound** | Metrics (optional) | Usage analytics for dashboard |

> **Privacy Note**: No user prompts or responses are sent to the cloud. Only metadata (allow/block decisions, timestamps) is optionally synced.

## 🚀 Quick Start

### Option 1: Docker Compose (Simplest)

```bash
cd self-deploy

# Configure
cp env.example .env
vim .env  # Set your API key, passwords

# Deploy
docker compose up -d

# Verify
curl http://localhost:8080/health
```

### Option 2: Kubernetes (Helm)

See [HELM_DEPLOYMENT.md](./HELM_DEPLOYMENT.md) for detailed instructions.

```bash
helm upgrade --install ethicalzen-runtime ./helm/ethicalzen-runtime \
  -n ethicalzen-runtime --create-namespace \
  -f values/gcp.yaml \
  --set mysql.auth.password=YOUR_PASSWORD \
  --set mysql.auth.rootPassword=YOUR_ROOT_PASSWORD
```

### Option 3: Terraform + Helm (Full Infrastructure)

```bash
# GCP
cd terraform/gcp
terraform init
terraform apply

# Then deploy Helm chart
helm upgrade --install ethicalzen-runtime ../../helm/ethicalzen-runtime \
  -n ethicalzen-runtime --create-namespace \
  -f ../../values/gcp.yaml
```

## 📦 Components

| Component | Port | Description |
|-----------|------|-------------|
| **Gateway** | 8080 | Main proxy - validates requests against guardrails |
| **Eval Engine** | 8091 | Smart guardrail evaluation (embeddings, scoring) |
| **Metrics Service** | 8090 | Evidence logging, audit trail |
| **Redis** | 6379 | Caching, rate limiting |
| **MySQL** | 3306 | Persistent storage |
| **Prometheus** | 9090 | Metrics collection (optional) |

## 📚 Documentation

- [Helm Deployment Guide](./HELM_DEPLOYMENT.md)
- [Architecture Details](./RUNTIME_ENFORCEMENT_ARCHITECTURE.md)
- [EthicalZen Platform](https://ethicalzen.ai)
- [API Documentation](https://docs.ethicalzen.ai)

## 📄 License

MIT License - See individual directories for details.

---

## 🔒 Security & Privacy

### Data Flow Control

| Setting | Default | Effect |
|---------|---------|--------|
| `BACKEND_FORWARD_ENABLED` | `false` | Outbound sync disabled |
| `METRICS_ENABLED` | `true` | Local metrics only |

### For HIPAA/PCI/SOC2 Compliance

```yaml
# values/hipaa-compliant.yaml
gateway:
  env:
    BACKEND_FORWARD_ENABLED: "false"  # No outbound sync
    METRICS_ENABLED: "true"           # Local metrics only
    
metricsService:
  env:
    BACKEND_FORWARD_ENABLED: "false"  # Keep all data on-premises
```

### Air-Gapped Deployment

For fully disconnected environments, the gateway can operate with:
- Pre-loaded certificates (mounted as config)
- Local-only calibrations
- No internet connectivity required after initial setup

```yaml
gateway:
  env:
    OFFLINE_MODE: "true"
    CERTIFICATES_PATH: "/config/certificates.json"
```

> **Privacy Guarantee**: User prompts and LLM responses NEVER leave your infrastructure. Only metadata (allow/block decisions) can optionally be synced.
