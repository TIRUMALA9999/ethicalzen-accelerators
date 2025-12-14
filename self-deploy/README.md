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
│                         HYBRID SYNC (outbound only)                              │
│                               │                                                  │
│                               ▼                                                  │
│   ┌─────────────────────────────────────────────────────────────────────────┐   │
│   │                    ETHICALZEN CLOUD BACKEND                              │   │
│   │   • Certificate management                                              │   │
│   │   • Guardrail calibration data                                          │   │
│   │   • Dashboard & analytics (optional)                                    │   │
│   └─────────────────────────────────────────────────────────────────────────┘   │
└─────────────────────────────────────────────────────────────────────────────────┘
```

## 🚀 Quick Start

### Option 1: Docker Compose (Simplest)

```bash
cd accelerators/self-deploy

# Configure
cp env.example .env
vim .env  # Set your API key, passwords

# Deploy
docker compose up -d

# Verify
curl http://localhost:8080/health
```

### Option 2: Kubernetes (Helm)

```bash
cd accelerators/self-deploy

# Install Helm chart
helm upgrade --install ethicalzen-runtime ./helm/ethicalzen-runtime \
  -n ethicalzen-runtime --create-namespace \
  -f values/gcp.yaml \
  --set mysql.auth.password=YOUR_PASSWORD \
  --set mysql.auth.rootPassword=YOUR_ROOT_PASSWORD

# Verify
kubectl -n ethicalzen-runtime get pods
```

### Option 3: Cloud-Managed Kubernetes

```bash
cd accelerators/self-deploy

# GCP (GKE)
./bin/deploy.sh gcp

# AWS (EKS)
./bin/deploy.sh aws

# Azure (AKS)
./bin/deploy.sh azure
```

## 📦 Components

| Component | Port | Description |
|-----------|------|-------------|
| **Gateway** | 8080 | Main enforcement proxy |
| **Eval Engine** | 8091 | Smart Guardrail evaluation (embeddings) |
| **Metrics Service** | 8090 | Evidence logging, audit trail |
| **Redis** | 6379 | Caching, sessions |
| **MySQL** | 3306 | Persistent storage |
| **Prometheus** | 9090 | Metrics (optional) |
| **Grafana** | 3000 | Dashboards (optional) |

## 🔐 Configuration

### Environment Variables

| Variable | Required | Description |
|----------|----------|-------------|
| `ETHICALZEN_API_KEY` | Yes | Your API key from portal |
| `MYSQL_PASSWORD` | Yes | MySQL user password |
| `MYSQL_ROOT_PASSWORD` | Yes | MySQL root password |
| `ETHICALZEN_BACKEND_URL` | No | Cloud backend for sync (default: cloud) |

### Helm Values

See `values/gcp.yaml`, `values/aws.yaml`, `values/azure.yaml` for cloud-specific configurations.

## 🧪 Testing

After deployment, test the gateway:

```bash
# Health check
curl http://localhost:8080/health

# Evaluate a guardrail
curl -X POST http://localhost:8080/api/guardrails/evaluate \
  -H "Content-Type: application/json" \
  -H "X-API-Key: YOUR_API_KEY" \
  -d '{
    "guardrail_id": "medical_advice_smart",
    "input": "I have chest pain, what should I take?"
  }'

# Expected: {"decision": "block", "score": 0.77, ...}
```

## 📁 Directory Structure

```
self-deploy/
├── README.md              # This file
├── docker-compose.yml     # Docker Compose deployment
├── env.example            # Environment template
├── bin/
│   ├── deploy.sh          # One-click cloud deploy
│   └── destroy.sh         # Teardown
├── helm/
│   └── ethicalzen-runtime/
│       ├── Chart.yaml
│       ├── values.yaml
│       └── templates/
│           ├── gateway-*.yaml
│           ├── eval-engine.yaml
│           ├── metrics-service.yaml
│           ├── redis.yaml
│           └── mysql.yaml
├── terraform/
│   ├── gcp/               # GKE + VPC
│   ├── aws/               # EKS + VPC
│   └── azure/             # AKS + VNet
└── values/
    ├── gcp.yaml
    ├── aws.yaml
    └── azure.yaml
```

## 🔄 Hybrid Sync Mode

The gateway syncs certificates and calibration data from EthicalZen Cloud:

- **Certificates**: Pulled on startup and cached
- **Calibrations**: Updated every 5 minutes (configurable)
- **Guardrails**: Synced from cloud repository

All request/response data stays in your environment—only configuration is synced.

## 🛡️ Security

1. **Network Isolation**: Deploy in private subnet
2. **TLS**: Add ingress with SSL certificates
3. **Secrets**: Use Kubernetes secrets or Vault
4. **Audit**: All decisions logged to MySQL

## 📞 Support

- Documentation: https://docs.ethicalzen.ai
- Issues: https://github.com/ethicalzen/ethicalzen/issues
- Email: support@ethicalzen.ai
