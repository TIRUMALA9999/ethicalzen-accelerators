# ACVPS Gateway Deployment Modes

## Overview

The ACVPS Gateway supports **three deployment modes**, each optimized for different use cases:

1. **Local Dev**: Standalone development on your laptop
2. **VPC/On-Premise**: Customer-deployed with cloud sync
3. **Cloud Multi-Tenant**: EthicalZen-hosted service

---

## 1️⃣ Local Dev Mode (Laptop)

### Architecture

```
┌─────────────────────────────────────────┐
│ Docker Compose (Local Laptop)           │
│                                         │
│  ┌────────────────┐  ┌───────────────┐ │
│  │ ACVPS Gateway  │──│ Metrics       │ │
│  │ (Port 8443)    │  │ Service       │ │
│  │ (Port 9090)    │  │ (Port 8090)   │ │
│  └────────────────┘  └───────────────┘ │
│         │                    │          │
│         └────────────────────┘          │
│           (Docker network)              │
└─────────────────────────────────────────┘
         │ Optional
         ▼ HTTPS (sync contracts)
  ┌─────────────────┐
  │ Cloud Backend   │
  └─────────────────┘
```

### Use Cases

- **Local development**: Test gateway features
- **Offline mode**: Work without internet
- **Contract testing**: Load contracts in-memory

### Setup

```bash
# 1. Configure for standalone
cp .env.local.template .env.local
# Leave GATEWAY_API_KEY blank for pure offline mode

# 2. Start gateway + metrics-service
docker-compose -f docker-compose-local.yaml up -d

# 3. Gateway ready
# - Gateway:  http://localhost:8443
# - Metrics:  http://localhost:9090/metrics
# - Telemetry: http://localhost:8090
```

### Configuration

```bash
GATEWAY_MODE=local
METRICS_ENABLED=true
METRICS_SERVICE_URL=http://metrics-service:8090

# Optional: Sync with cloud
# GATEWAY_API_KEY=gw_...
# CONTROL_PLANE_URL=https://...
```

---

## 2️⃣ VPC/On-Premise Mode (Customer Infrastructure)

### Architecture

```
┌──────────────────────────────────────────────────┐
│ Customer VPC/On-Premise                          │
│                                                  │
│  Kubernetes Pod / Docker Compose                │
│  ┌────────────────┐  ┌────────────────┐        │
│  │ ACVPS Gateway  │  │ Metrics        │        │
│  │ (Container 1)  │──│ Service        │        │
│  │                │  │ (Sidecar)      │        │
│  └────────────────┘  └────────────────┘        │
│         │                    │                   │
│         └────────────────────┘                   │
│           (Pod network)                          │
└─────────────┬────────────────────────────────────┘
              │
              ▼ HTTPS (secure tunnel)
┌──────────────────────────────────────────────────┐
│ EthicalZen Cloud Backend (Portal)                │
│                                                  │
│  ┌─────────────────┐  ┌─────────────────┐      │
│  │ Contract Sync   │  │ Evidence/Metrics│      │
│  │ (HTTP GET)      │  │ (HTTP POST)     │      │
│  └─────────────────┘  └─────────────────┘      │
│                                                  │
│  MySQL (contracts, policies, certificates)      │
└──────────────────────────────────────────────────┘
```

### Use Cases

- **Enterprise customers**: Deploy in their own VPC
- **Compliance requirements**: Keep data in customer environment
- **Air-gapped deployments**: Optional offline mode
- **High availability**: Customer controls scaling

### Setup (Kubernetes)

```yaml
# deployment.yaml
apiVersion: v1
kind: Pod
metadata:
  name: acvps-gateway
spec:
  containers:
    # Main gateway
    - name: gateway
      image: gcr.io/ethicalzen-public-04085/acvps-gateway:latest
      ports:
        - containerPort: 8443
          name: gateway
        - containerPort: 9090
          name: prometheus
      env:
        - name: GATEWAY_MODE
          value: "local"
        - name: GATEWAY_API_KEY
          valueFrom:
            secretKeyRef:
              name: gateway-credentials
              key: api-key
        - name: GATEWAY_TENANT_ID
          valueFrom:
            secretKeyRef:
              name: gateway-credentials
              key: tenant-id
        - name: CONTROL_PLANE_URL
          value: "https://ethicalzen-backend-400782183161.us-central1.run.app"
        - name: METRICS_ENABLED
          value: "true"
        - name: METRICS_SERVICE_URL
          value: "http://localhost:8090"
      
    # Metrics sidecar
    - name: metrics-service
      image: gcr.io/ethicalzen-public-04085/metrics-service:latest
      ports:
        - containerPort: 8090
          name: metrics
      env:
        - name: PORT
          value: "8090"
        - name: NODE_ENV
          value: "production"
      volumeMounts:
        - name: metrics-data
          mountPath: /app/data
  
  volumes:
    - name: metrics-data
      persistentVolumeClaim:
        claimName: metrics-pvc

---
apiVersion: v1
kind: Service
metadata:
  name: acvps-gateway
spec:
  selector:
    app: acvps-gateway
  ports:
    - name: gateway
      port: 8443
      targetPort: 8443
    - name: prometheus
      port: 9090
      targetPort: 9090
```

### Setup (Docker Compose)

```bash
# 1. Get credentials from portal
./setup-local-gateway.sh

# 2. Update .env.local
GATEWAY_MODE=local
GATEWAY_API_KEY=gw_xyz...
GATEWAY_TENANT_ID=tenant_abc...
CONTROL_PLANE_URL=https://ethicalzen-backend-400782183161.us-central1.run.app
METRICS_ENABLED=true
METRICS_SERVICE_URL=http://metrics-service:8090

# 3. Start services
docker-compose -f docker-compose-local.yaml up -d
```

### Data Flow

1. **Boot Time**: Gateway fetches tenant-scoped contracts from cloud
2. **Periodic Sync**: Re-sync every 60 seconds to get new contracts
3. **Request Processing**: Gateway validates requests using cached contracts
4. **Evidence Collection**: Metrics-service collects telemetry locally
5. **Evidence Push**: Metrics-service pushes batches to cloud backend (async)

### Benefits

- ✅ **Customer Control**: Deploy in customer VPC/on-premise
- ✅ **Low Latency**: Local validation, no cloud round-trip per request
- ✅ **Data Residency**: Customer data stays in customer environment
- ✅ **Observability**: Central portal shows metrics from all customer deployments
- ✅ **Auto-Update**: Contracts sync automatically from cloud

---

## 3️⃣ Cloud Multi-Tenant Mode (EthicalZen-Hosted)

### Architecture

```
┌──────────────────────────────────────────────────┐
│ GCP Cloud Run (EthicalZen Infrastructure)       │
│                                                  │
│  ┌─────────────────────────────────────────┐   │
│  │ Gateway Pool (Auto-scaling)             │   │
│  │  ┌──────┐  ┌──────┐  ┌──────┐          │   │
│  │  │ GW-1 │  │ GW-2 │  │ GW-N │          │   │
│  │  └───┬──┘  └───┬──┘  └───┬──┘          │   │
│  │      └──────────┴─────────┘             │   │
│  └─────────────┬───────────────────────────┘   │
│                │                                │
│        ┌───────┴────────┐                      │
│        │                │                       │
│   ┌────▼─────┐    ┌────▼──────┐               │
│   │  Redis   │    │  Backend  │               │
│   │ (Cache + │    │ (Portal   │               │
│   │  Pub/Sub)│    │  API)     │               │
│   └──────────┘    └─────┬─────┘               │
│                          │                      │
│                    ┌─────▼─────┐               │
│                    │  MySQL    │               │
│                    │ (Source   │               │
│                    │  of Truth)│               │
│                    └───────────┘               │
└──────────────────────────────────────────────────┘
```

### Use Cases

- **Multi-tenant SaaS**: All tenants share infrastructure
- **Rapid onboarding**: No customer deployment needed
- **Managed service**: EthicalZen handles scaling, updates
- **Global availability**: Multi-region deployment

### Contract Sync Mechanism

1. **MySQL** → Source of truth for all contracts
2. **Redis Cache** → Fast lookup, tenant-scoped
3. **Redis Pub/Sub** → Notify all gateway instances when contracts change
4. **Gateway Pool** → All instances auto-reload contracts on change

### Configuration

```bash
GATEWAY_MODE=cloud
REDIS_HOST=10.x.x.x  # Cloud Redis instance
REDIS_PORT=6379
BACKEND_URL=https://ethicalzen-backend-...
```

---

## Comparison Table

| Feature | Local Dev | VPC/On-Premise | Cloud Multi-Tenant |
|---------|-----------|----------------|-------------------|
| **Deployment** | Docker Compose | K8s/Docker | Cloud Run |
| **Contract Storage** | In-memory | In-memory + Cloud Sync | Redis + MySQL |
| **Metrics Collection** | Local SQLite | Sidecar → Cloud | Direct to Backend |
| **Scalability** | Single instance | Customer-controlled | Auto-scaling |
| **Data Residency** | Local | Customer VPC | EthicalZen Cloud |
| **Offline Mode** | ✅ Yes | ⚠️ Optional | ❌ No |
| **Setup Complexity** | Low | Medium | N/A (Managed) |
| **Cost** | Free | Customer infra | Usage-based |

---

## Choosing the Right Mode

### Use Local Dev When:
- 👨‍💻 Developing new features
- 🧪 Testing gateway locally
- 📚 Learning the platform

### Use VPC/On-Premise When:
- 🏢 Enterprise customer deployment
- 🔒 Data must stay in customer environment
- 📜 Compliance/regulatory requirements
- 🚀 High-throughput, low-latency needs

### Use Cloud Multi-Tenant When:
- ⚡ Quick onboarding
- 💰 Usage-based pricing
- 🌍 Global availability
- 🛠️ Managed infrastructure

---

## Migration Paths

### From Local → VPC
1. Configure `GATEWAY_API_KEY` and `CONTROL_PLANE_URL`
2. Enable metrics sidecar
3. Deploy to K8s/Docker

### From VPC → Cloud
1. Remove customer deployment
2. Use EthicalZen-hosted gateway
3. Update SDK to point to cloud gateway

### From Cloud → VPC
1. Register gateway via portal
2. Get API credentials
3. Deploy gateway + metrics-service
4. Update SDK to point to VPC gateway

---

## Next Steps

- **Local Dev**: See [QUICK_SETUP.md](./QUICK_SETUP.md)
- **VPC Deployment**: See [LOCAL_DEVELOPMENT_GUIDE.md](../docs/LOCAL_DEVELOPMENT_GUIDE.md)
- **Cloud Onboarding**: Contact EthicalZen team

