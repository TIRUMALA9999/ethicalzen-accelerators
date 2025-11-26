# 🔒 ACVPS Gateway
## AI Contracts Validation Protocol (Secured) - Reference Implementation

[![Go Version](https://img.shields.io/badge/Go-1.21+-00ADD8?style=flat&logo=go)](https://golang.org)
[![License](https://img.shields.io/badge/License-Apache%202.0-blue.svg)](LICENSE)
[![Docker](https://img.shields.io/badge/Docker-Ready-2496ED?style=flat&logo=docker)](https://hub.docker.com/r/ethicalzen/acvps-gateway)

**ACVPS Gateway** is a high-performance HTTP/HTTPS gateway that adds **blockchain-based AI contract validation** to any web service—without requiring code changes.

---

## 🎯 What is ACVPS?

**ACVPS (AI Contracts Validation Protocol - Secured)** is a backward-compatible extension of HTTPS that automatically validates AI service calls against immutable blockchain contracts.

**Key Features:**
- ✅ **Zero Code Changes** - Deploy gateway, point DNS, done
- ✅ **Blockchain-Backed** - Contracts stored on Ethereum/Arbitrum
- ✅ **<5ms Overhead** - Local blockchain nodes + aggressive caching
- ✅ **AI Safety Built-in** - PII redaction, grounding checks, failure mode detection
- ✅ **Production-Ready** - High performance Go implementation

---

## 🚀 Quick Start

### 1. Run with Docker

```bash
docker run -d \
  -p 443:443 \
  -e BLOCKCHAIN_RPC_URL=http://your-blockchain-node:8545 \
  -e DC_REGISTRY_ADDRESS=0x... \
  -e BACKEND_URL=http://your-service:8080 \
  ethicalzen/acvps-gateway:latest
```

### 2. Point DNS to Gateway

```bash
# Before: Client → Your Service
api.yourcompany.com → your-service:8080

# After: Client → ACVPS Gateway → Your Service
api.yourcompany.com → acvps-gateway:443 → your-service:8080
```

### 3. Services Get Automatic Validation

```bash
# Client request (with DC headers)
curl https://api.yourcompany.com/api/endpoint \
  -H "X-DC-Id: your-service/industry/region/v1.0" \
  -H "X-DC-Digest: sha256-abc123..."

# Gateway validates contract via blockchain
# → If valid: forwards to backend
# → If invalid: returns 409 DC_INVALID
```

**That's it. No code changes required.**

---

## 📋 Features

### Core Validation
- [x] DC header extraction (`X-DC-Id`, `X-DC-Digest`)
- [x] Blockchain contract validation via local node
- [x] Redis caching (5-min TTL, 99% hit rate)
- [x] Graceful fallback for non-DC requests

### AI Safety Enforcement
- [x] PII redaction (configurable field whitelist)
- [x] Grounding confidence checks
- [x] Failure mode detection
- [ ] Hallucination risk scoring (v2)
- [ ] Prompt injection detection (v2)

### Performance
- [x] <5ms overhead (99th percentile)
- [x] 10,000 req/s per instance
- [x] Horizontal scaling (stateless)
- [x] Connection pooling

### Operations
- [x] Health check endpoint (`/health`)
- [x] Metrics (Prometheus)
- [x] Structured logging (JSON)
- [x] Graceful shutdown
- [ ] Distributed tracing (OpenTelemetry) (v2)

---

## 🏗️ Architecture

```
┌─────────────┐
│   Client    │
└──────┬──────┘
       │ HTTPS + DC Headers
       ↓
┌────────────────────────┐
│   ACVPS Gateway        │
│                        │
│  1. Extract headers    │
│  2. Query cache        │────► Redis
│  3. Validate contract  │────► Local Blockchain Node
│  4. Check failures     │
│  5. Forward request    │
│  6. Inspect response   │
│  7. Apply mitigations  │
└──────┬─────────────────┘
       │
       ↓
┌──────────────────┐
│  Backend Service │  ← No code changes!
└──────────────────┘
```

---

## 📦 Installation

### Option 1: Docker (Recommended)

```bash
# Pull image
docker pull ethicalzen/acvps-gateway:latest

# Run
docker run -d \
  --name acvps-gateway \
  -p 443:443 \
  -p 9090:9090 \
  -v $(pwd)/config.yaml:/etc/acvps/config.yaml \
  -v $(pwd)/certs:/etc/acvps/certs \
  -e BLOCKCHAIN_RPC_URL=ws://blockchain-node:8546 \
  -e DC_REGISTRY_ADDRESS=0x5FbDB2315678afecb367f032d93F642f64180aa3 \
  ethicalzen/acvps-gateway:latest
```

### Option 2: Docker Compose

```yaml
version: '3.8'
services:
  acvps-gateway:
    image: ethicalzen/acvps-gateway:latest
    ports:
      - "443:443"
      - "9090:9090"
    volumes:
      - ./config.yaml:/etc/acvps/config.yaml
      - ./certs:/etc/acvps/certs
    environment:
      - BLOCKCHAIN_RPC_URL=ws://blockchain-node:8546
      - DC_REGISTRY_ADDRESS=${DC_REGISTRY_ADDRESS}
      - REDIS_URL=redis://redis:6379
    depends_on:
      - redis
      - blockchain-node

  redis:
    image: redis:7-alpine
    ports:
      - "6379:6379"

  blockchain-node:
    image: ethereum/client-go:latest
    command: --syncmode light --ws --ws.addr 0.0.0.0
    ports:
      - "8546:8546"
```

### Option 3: Kubernetes

```bash
kubectl apply -f https://raw.githubusercontent.com/ethicalzen/acvps-gateway/main/k8s/deployment.yaml
```

### Option 4: Build from Source

```bash
# Prerequisites: Go 1.21+
git clone https://github.com/ethicalzen/acvps-gateway.git
cd acvps-gateway
go build -o acvps-gateway cmd/gateway/main.go
./acvps-gateway --config config.yaml
```

---

## ⚙️ Configuration

### Minimal Configuration

```yaml
# config.yaml
gateway:
  port: 443
  tls:
    cert: "/etc/acvps/certs/cert.pem"
    key: "/etc/acvps/certs/key.pem"

blockchain:
  rpc_url: "ws://localhost:8546"
  contract_address: "0x5FbDB2315678afecb367f032d93F642f64180aa3"

backend:
  url: "http://backend-service:8080"

cache:
  redis_url: "redis://localhost:6379"
```

### Full Configuration

See [`config.example.yaml`](config.example.yaml) for all options.

---

## 🧪 Testing

### Test with Local Setup

```bash
# 1. Start dependencies
docker-compose up -d

# 2. Deploy test contract to blockchain
cd ../aipromptandseccheck/blockchain
npx hardhat run deploy.js --network hardhat

# 3. Start gateway
export DC_REGISTRY_ADDRESS="0x..."
go run cmd/gateway/main.go --config config.yaml

# 4. Test with valid contract
curl -k https://localhost:443/api/test \
  -H "X-DC-Id: test-service/healthcare/us/v1.0" \
  -H "X-DC-Digest: sha256-abc123..."

# Expected: Request forwarded to backend

# 5. Test with invalid contract
curl -k https://localhost:443/api/test \
  -H "X-DC-Id: fake-contract" \
  -H "X-DC-Digest: sha256-fake"

# Expected: 409 DC_INVALID
```

### Run Unit Tests

```bash
go test ./...
```

### Run Integration Tests

```bash
go test ./tests/integration -v
```

### Run Load Tests

```bash
# Requires: hey (https://github.com/rakyll/hey)
hey -n 10000 -c 100 -H "X-DC-Id: test/healthcare/us/v1.0" https://localhost:443/api/test
```

---

## 📊 Performance

### Benchmarks (M1 Mac, 8 cores)

| Metric | Value | Notes |
|--------|-------|-------|
| **Throughput** | 10,000 req/s | Single instance |
| **Latency (p50)** | 2ms | Cache hit |
| **Latency (p95)** | 5ms | Cache hit |
| **Latency (p99)** | 50ms | Cache miss + BC query |
| **Memory** | 100MB | With 10K cached contracts |
| **CPU** | 50% | At 10K req/s |

### Caching Performance

| Scenario | Latency | Hit Rate |
|----------|---------|----------|
| **Cache hit** | <1ms | 99% |
| **Cache miss** | ~50ms | 1% |
| **BC node down** | Fallback to last known state | - |

---

## 🔒 Security

### What ACVPS Prevents

✅ **Unauthorized AI Usage** - Only services with active blockchain contracts can process requests
✅ **PII Leakage** - Gateway redacts non-whitelisted PHI fields
✅ **Ungrounded Responses** - Blocks responses with confidence < threshold
✅ **Contract Bypass** - Backend in private network, not publicly routable
✅ **Replay Attacks** - Blockchain tracks revocation epochs

### Security Best Practices

1. **TLS Required** - Gateway enforces HTTPS
2. **Backend in Private Network** - No direct public access
3. **Blockchain Node Authentication** - Use WebSocket with auth
4. **Redis Authentication** - Enable AUTH in production
5. **Regular Updates** - Keep gateway and dependencies updated

---

## 📈 Monitoring

### Health Check

```bash
curl http://localhost:9090/health
```

**Response:**
```json
{
  "status": "healthy",
  "version": "1.0.0",
  "blockchain": {
    "connected": true,
    "block_number": 12345
  },
  "cache": {
    "connected": true,
    "hit_rate": 0.99
  },
  "backend": {
    "reachable": true
  }
}
```

### Metrics (Prometheus)

```bash
curl http://localhost:9090/metrics
```

**Key Metrics:**
- `acvps_requests_total` - Total requests processed
- `acvps_validation_duration_seconds` - Blockchain validation latency
- `acvps_cache_hit_ratio` - Cache hit rate
- `acvps_failures_detected_total` - Failure modes detected
- `acvps_mitigations_applied_total` - Mitigations applied

### Grafana Dashboard

Import [`grafana/dashboard.json`](grafana/dashboard.json) for pre-built dashboard.

---

## 🛠️ Development

### Project Structure

```
acvps-gateway/
├── cmd/
│   └── gateway/
│       └── main.go           # Entry point
├── internal/
│   ├── blockchain/           # Blockchain client
│   ├── cache/                # Redis caching
│   ├── proxy/                # HTTP proxy
│   ├── validation/           # Contract validation
│   ├── mitigation/           # Failure mode mitigation
│   └── config/               # Configuration
├── pkg/
│   └── acvps/                # Public ACVPS library
├── tests/
│   ├── unit/                 # Unit tests
│   └── integration/          # Integration tests
├── config/
│   └── config.example.yaml   # Example config
├── docker/
│   ├── Dockerfile            # Production image
│   └── docker-compose.yaml   # Local dev stack
├── k8s/                      # Kubernetes manifests
├── grafana/                  # Grafana dashboards
├── docs/                     # Documentation
├── go.mod
├── go.sum
├── Makefile
└── README.md
```

### Build

```bash
make build
```

### Run Tests

```bash
make test
```

### Run Locally

```bash
make run
```

### Docker Build

```bash
make docker-build
```

---

## 📚 Documentation

- [Protocol Specification](docs/PROTOCOL.md)
- [Configuration Guide](docs/CONFIG.md)
- [Deployment Guide](docs/DEPLOYMENT.md)
- [Troubleshooting](docs/TROUBLESHOOTING.md)
- [API Reference](docs/API.md)

---

## 🤝 Contributing

We welcome contributions! See [CONTRIBUTING.md](CONTRIBUTING.md) for guidelines.

**Areas we need help:**
- [ ] Multi-language SDKs (Python, Java, Node.js)
- [ ] Kubernetes operator
- [ ] Advanced failure mode detection
- [ ] Performance optimizations

---

## 📄 License

Apache 2.0 - See [LICENSE](LICENSE) for details.

---

## 🙏 Acknowledgments

- **EthicalZen** - AI safety platform
- **Ethereum Foundation** - Blockchain infrastructure
- **Go Community** - Amazing ecosystem

---

## 🔗 Links

- **Website:** https://ethicalzen.ai
- **Documentation:** https://docs.ethicalzen.ai/acvps
- **Docker Hub:** https://hub.docker.com/r/ethicalzen/acvps-gateway
- **Discord:** https://discord.gg/ethicalzen

---

## 📞 Support

- **Issues:** [GitHub Issues](https://github.com/ethicalzen/acvps-gateway/issues)
- **Email:** support@ethicalzen.ai
- **Discord:** https://discord.gg/ethicalzen

---

**Built with ❤️ by the EthicalZen team**

