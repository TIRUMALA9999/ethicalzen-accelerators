# ACVPS Gateway Deployment Comparison

## Quick Reference: Local Dev vs VPC vs Cloud

| Feature | Local Dev | VPC Deployment | Cloud (Multi-Tenant) |
|---------|-----------|----------------|---------------------|
| **Gateway Mode** | `local` | `local` | `cloud` |
| **Gateway Location** | Docker container (laptop) | VPC service (K8s/Cloud Run) | Cloud Run pool (GCP) |
| **Client Location** | Host machine | Inside VPC | Internet |
| **Backend Location** | Host machine | Inside VPC | Internet/VPC |
| **Networking** | Docker bridge + localhost | Private VPC networking | Public internet or VPC |
| **Localhost Translation** | ✅ Required | ❌ Not needed | ❌ Not needed |
| **Backend URL Examples** | `localhost:4500` | `http://service:8080`<br>`http://10.0.x.x` | `https://api.service.com` |
| **Contract Sync** | HTTP API (optional) | HTTP API | Redis + Pub/Sub |
| **Metrics Storage** | SQLite (local) | SQLite + Forward to portal | Redis |
| **Authentication** | Gateway API key | Gateway API key | Gateway API key + User API keys |
| **Scaling** | Single instance | Auto-scaling (3-100) | Auto-scaling (10-1000) |
| **Use Case** | Development, testing | Customer VPC, on-premise | SaaS, shared infrastructure |

---

## Configuration Comparison

### Local Development

```yaml
# docker-compose-local.yaml
environment:
  - GATEWAY_MODE=local
  - GATEWAY_API_KEY=gw_xxx
  - GATEWAY_TENANT_ID=tenant-xxx
  - CONTROL_PLANE_URL=https://ethicalzen-backend.run.app
  - METRICS_ENABLED=true
  - METRICS_SERVICE_URL=http://metrics-service:8090
  - BACKEND_FORWARD_ENABLED=false  # Optional

extra_hosts:
  - "host.docker.internal:host-gateway"  # For localhost translation
```

**Client Configuration:**
```javascript
const client = createProxyClient({
  gatewayUrl: 'http://localhost:8443',
  contractId: 'healthcare-app/healthcare/us/v1.0',
  tenantId: 'tenant-123'
});

// Backends on host machine
await client.post('http://localhost:4500/api/query', data);
```

---

### VPC Deployment (Kubernetes)

```yaml
# k8s/gateway-deployment.yaml
env:
  - name: GATEWAY_MODE
    value: "local"
  - name: GATEWAY_API_KEY
    valueFrom:
      secretKeyRef:
        name: gateway-credentials
        key: api-key
  - name: GATEWAY_TENANT_ID
    value: "tenant-123"
  - name: CONTROL_PLANE_URL
    value: "https://ethicalzen-backend.run.app"
  - name: METRICS_ENABLED
    value: "true"
  - name: METRICS_SERVICE_URL
    value: "http://metrics-service:8090"
  - name: BACKEND_FORWARD_ENABLED
    value: "true"  # Forward to portal
```

**Client Configuration:**
```javascript
const client = createProxyClient({
  gatewayUrl: 'http://acvps-gateway.default.svc.cluster.local:8443',
  contractId: 'healthcare-app/healthcare/us/v1.0',
  tenantId: 'tenant-123'
});

// Backends inside VPC (K8s service DNS)
await client.post('http://healthcare-service:8080/api/query', data);
```

---

### Cloud Deployment (Multi-Tenant SaaS)

```yaml
# Cloud Run deployment
env:
  - GATEWAY_MODE=cloud
  - REDIS_HOST=10.0.0.5
  - REDIS_PORT=6379
  - BACKEND_URL=https://unified-mock-backend.run.app  # Fallback only
  - METRICS_ENABLED=true
  - METRICS_SERVICE_URL=http://metrics-service:8090
```

**Client Configuration:**
```javascript
const client = createProxyClient({
  gatewayUrl: 'https://gateway.ethicalzen.ai:8443',
  contractId: 'healthcare-app/healthcare/us/v1.0',
  tenantId: 'tenant-123'
});

// Backends on public internet or VPC
await client.post('https://api.healthcare-company.com/api/query', data);
```

---

## Localhost Translation Matrix

| Client URL | Local Dev (Docker) | VPC Deployment | Cloud Deployment |
|------------|-------------------|----------------|------------------|
| `http://localhost:4500` | `→ host.docker.internal:4500` ✅ | ❌ Should not use | ❌ Should not use |
| `http://service-name:8080` | ❌ Not available | ✅ K8s DNS | ✅ If in same VPC |
| `http://10.0.3.10:8080` | ❌ Not routable | ✅ Private IP | ✅ If in same VPC |
| `https://api.example.com` | ✅ Public internet | ✅ Public internet | ✅ Public internet |

**Key Rule:** Localhost translation ONLY activates when URL contains `://localhost:` or `://localhost/`

---

## Decision Tree: Which Deployment Mode?

```
┌─────────────────────────────────────────────────────────────────────┐
│ Are you developing locally on your laptop?                         │
└─────────────────────────────────────────────────────────────────────┘
         │
         │ YES → LOCAL DEVELOPMENT
         │       └─ Use docker-compose-local.yaml
         │       └─ Backend URLs: http://localhost:XXXX
         │       └─ Localhost translation: ENABLED
         │
         │ NO ↓
         │
┌─────────────────────────────────────────────────────────────────────┐
│ Is the gateway deployed INSIDE a customer's VPC or on-premise?     │
└─────────────────────────────────────────────────────────────────────┘
         │
         │ YES → VPC DEPLOYMENT
         │       └─ Use K8s/Cloud Run/ECS with mode=local
         │       └─ Backend URLs: http://service-name:8080 or http://10.0.x.x
         │       └─ Localhost translation: DISABLED (automatic)
         │       └─ Metrics: Forward to control plane
         │
         │ NO ↓
         │
┌─────────────────────────────────────────────────────────────────────┐
│ Is this a multi-tenant SaaS gateway serving many customers?        │
└─────────────────────────────────────────────────────────────────────┘
         │
         │ YES → CLOUD DEPLOYMENT
               └─ Use Cloud Run with mode=cloud
               └─ Backend URLs: https://api.customer.com
               └─ Localhost translation: DISABLED (automatic)
               └─ Contracts: Redis + Pub/Sub for all tenants
               └─ Metrics: Redis
```

---

## Performance Comparison

| Metric | Local Dev | VPC | Cloud |
|--------|-----------|-----|-------|
| **Request Latency** | 5-15ms | 5-20ms | 50-200ms |
| **Throughput** | 1K req/s | 50K+ req/s | 100K+ req/s |
| **Scaling** | Manual | Auto (3-100) | Auto (10-1000) |
| **Cost per 1M req** | $0 | $5-20 | $20-50 |

**Why VPC is faster than Cloud:**
- VPC uses private networking (no internet hops)
- Lower latency (< 5ms between services)
- Higher bandwidth (no egress limits)

**When to use each:**
- **Local**: Development, testing, debugging
- **VPC**: Customer deployments, high-security, low-latency
- **Cloud**: SaaS, shared infrastructure, easy scaling

---

## Security Comparison

| Feature | Local Dev | VPC | Cloud |
|---------|-----------|-----|-------|
| **Network Isolation** | ❌ Shared host | ✅ Private VPC | ⚠️ Public internet |
| **TLS/mTLS** | Optional | ✅ Required | ✅ Required |
| **Firewall** | Host firewall | ✅ Security groups | ✅ Cloud Armor |
| **Data Residency** | N/A | ✅ Customer VPC | ⚠️ Multi-region |
| **Compliance** | N/A | ✅ HIPAA, PCI-DSS | ✅ SOC 2 |

---

## Cost Comparison (Monthly)

**Assumptions:**
- 10M requests/month
- 3 gateway instances
- 1GB metrics storage

| Component | Local Dev | VPC (GCP) | Cloud (Shared) |
|-----------|-----------|-----------|----------------|
| Gateway compute | $0 | $100 | $30 (shared) |
| Metrics storage | $0 | $20 | $10 (shared) |
| Networking | $0 | $0 (internal) | $50 (egress) |
| Load balancer | $0 | $20 | $0 (included) |
| **Total** | **$0** | **$140** | **$90** |

**VPC is more expensive BUT:**
- ✅ Private networking (no internet exposure)
- ✅ Customer data never leaves VPC
- ✅ Lower latency, higher throughput
- ✅ Compliance requirements (HIPAA, PCI-DSS)

---

## Summary

### Use Local Development When:
✅ Developing features locally
✅ Testing gateway functionality
✅ Debugging contracts
✅ Quick iteration cycles

### Use VPC Deployment When:
✅ Customer requires on-premise/VPC hosting
✅ Low-latency requirements (< 10ms)
✅ High-security/compliance requirements
✅ Data residency requirements
✅ Customer controls infrastructure

### Use Cloud Deployment When:
✅ Building SaaS product
✅ Serving many tenants
✅ Easy scaling and ops
✅ Cost-effective for high volume
✅ Flexible backend targets

---

## Further Reading

- **Local Development**: [LOCAL_DEVELOPMENT_GUIDE.md](../docs/LOCAL_DEVELOPMENT_GUIDE.md)
- **VPC Deployment**: [VPC_DEPLOYMENT_GUIDE.md](../docs/VPC_DEPLOYMENT_GUIDE.md)
- **Gateway Patterns**: [GATEWAY_DEPLOYMENT_PATTERNS.md](../docs/GATEWAY_DEPLOYMENT_PATTERNS.md)
- **Architecture**: [ARCHITECTURE_ROUTING.md](ARCHITECTURE_ROUTING.md)

