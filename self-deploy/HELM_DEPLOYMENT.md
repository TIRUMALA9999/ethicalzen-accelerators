# EthicalZen Runtime Enforcement - Helm Deployment Guide

Deploy AI guardrail enforcement in your private Kubernetes cluster with a single command.

---

## 📋 Prerequisites

| Requirement | Version | Notes |
|-------------|---------|-------|
| Kubernetes | 1.25+ | GKE, EKS, AKS, or on-premises |
| Helm | 3.10+ | [Install Helm](https://helm.sh/docs/intro/install/) |
| kubectl | 1.25+ | Configured for your cluster |
| Storage | 15GB+ | For MySQL and Redis persistence |

---

## 🚀 Quick Start (5 minutes)

### Step 1: Clone the Repository

```bash
git clone https://github.com/aiworksllc/ethicalzen-accelerators.git
cd ethicalzen-accelerators/self-deploy
```

### Step 2: Deploy

```bash
# Create namespace
kubectl create namespace ethicalzen-runtime

# Deploy with Helm
helm upgrade --install ethicalzen-runtime ./helm/ethicalzen-runtime \
  -n ethicalzen-runtime \
  -f values/gcp.yaml \
  --set mysql.auth.password="$(openssl rand -base64 16)" \
  --set mysql.auth.rootPassword="$(openssl rand -base64 16)"
```

### Step 3: Verify

```bash
# Check pods (all should be Running)
kubectl get pods -n ethicalzen-runtime

# Expected output:
# NAME                                      READY   STATUS    RESTARTS   AGE
# ethicalzen-eval-engine-xxx                1/1     Running   0          2m
# ethicalzen-gateway-xxx                    1/1     Running   0          2m
# ethicalzen-gateway-xxx                    1/1     Running   0          2m
# ethicalzen-mysql-0                        1/1     Running   0          2m
# ethicalzen-redis-xxx                      1/1     Running   0          2m

# Get Gateway external IP
kubectl get svc ethicalzen-gateway -n ethicalzen-runtime
```

### Step 4: Test

```bash
# Get Gateway IP
GATEWAY_IP=$(kubectl get svc ethicalzen-gateway -n ethicalzen-runtime -o jsonpath='{.status.loadBalancer.ingress[0].ip}')

# Health check
curl http://$GATEWAY_IP/health
# {"service":"acvps-gateway","status":"healthy",...}

# Test guardrail enforcement (should block PII)
curl -X POST "http://$GATEWAY_IP/api/proxy" \
  -H "Content-Type: application/json" \
  -H "X-DC-Id: dc_demo_public" \
  -H "X-Target-Endpoint: https://api.openai.com/v1/chat/completions" \
  -H "Authorization: Bearer YOUR_OPENAI_KEY" \
  -d '{
    "model": "gpt-4",
    "messages": [{"role": "user", "content": "My SSN is 123-45-6789"}]
  }'
# Response: 403 Forbidden (PII blocked)
```

---

## 🏗️ Architecture

```
┌─────────────────────────────────────────────────────────────────────┐
│                    YOUR KUBERNETES CLUSTER                           │
│                                                                      │
│   ┌─────────────────┐     ┌─────────────────┐     ┌────────────┐   │
│   │    Gateway      │────▶│   Eval Engine   │     │   Redis    │   │
│   │   (2 replicas)  │     │   (1 replica)   │     │  (cache)   │   │
│   │   Port: 80      │     │   Port: 8091    │     │ Port: 6379 │   │
│   └────────┬────────┘     └─────────────────┘     └────────────┘   │
│            │                                                        │
│            │ LoadBalancer                                           │
│            ▼                                                        │
│   ┌─────────────────┐                                               │
│   │  External IP    │◀──── Your Applications                        │
│   │  34.x.x.x:80    │                                               │
│   └─────────────────┘                                               │
│                                                                      │
│            │ Hybrid Sync (outbound)                                 │
│            ▼                                                        │
│   ┌─────────────────────────────────────────────────────────────┐  │
│   │              EthicalZen Cloud (api.ethicalzen.ai)            │  │
│   │   • Certificates & Contracts                                 │  │
│   │   • Guardrail Calibrations                                   │  │
│   │   • Dashboard (optional)                                     │  │
│   └─────────────────────────────────────────────────────────────┘  │
└─────────────────────────────────────────────────────────────────────┘
```

---

## ⚙️ Configuration Options

### Core Settings

| Parameter | Description | Default |
|-----------|-------------|---------|
| `gateway.replicas` | Number of gateway pods | `2` |
| `gateway.env.BACKEND_URL` | EthicalZen Cloud URL | `https://api.ethicalzen.ai` |
| `evalEngine.enabled` | Enable smart guardrails | `true` |
| `evalEngine.replicas` | Eval engine pods | `1` |
| `redis.enabled` | Enable Redis caching | `true` |
| `mysql.enabled` | Enable MySQL storage | `true` |
| `prometheus.enabled` | Enable metrics | `true` |

### Security Settings

| Parameter | Description | Required |
|-----------|-------------|----------|
| `mysql.auth.password` | MySQL user password | Yes |
| `mysql.auth.rootPassword` | MySQL root password | Yes |
| `gateway.env.ETHICALZEN_API_KEY` | API key for cloud sync | Optional |

### Example: Production Deployment

```bash
helm upgrade --install ethicalzen-runtime ./helm/ethicalzen-runtime \
  -n ethicalzen-runtime \
  -f values/gcp.yaml \
  --set gateway.replicas=3 \
  --set evalEngine.replicas=2 \
  --set mysql.auth.password="$(kubectl get secret mysql-secret -o jsonpath='{.data.password}' | base64 -d)" \
  --set mysql.auth.rootPassword="$(kubectl get secret mysql-secret -o jsonpath='{.data.root-password}' | base64 -d)" \
  --set prometheus.enabled=true \
  --set grafana.enabled=true
```

---

## 🔧 Cloud-Specific Instructions

### Google Cloud (GKE)

```bash
# Create GKE cluster (if needed)
gcloud container clusters create ethicalzen-cluster \
  --region us-central1 \
  --num-nodes 3 \
  --machine-type e2-standard-2

# Get credentials
gcloud container clusters get-credentials ethicalzen-cluster --region us-central1

# Deploy
helm upgrade --install ethicalzen-runtime ./helm/ethicalzen-runtime \
  -n ethicalzen-runtime --create-namespace \
  -f values/gcp.yaml \
  --set mysql.auth.password="secure-password-here" \
  --set mysql.auth.rootPassword="secure-root-password"
```

### Amazon Web Services (EKS)

```bash
# Create EKS cluster (if needed)
eksctl create cluster --name ethicalzen-cluster --region us-east-1

# Deploy
helm upgrade --install ethicalzen-runtime ./helm/ethicalzen-runtime \
  -n ethicalzen-runtime --create-namespace \
  -f values/aws.yaml \
  --set mysql.auth.password="secure-password-here" \
  --set mysql.auth.rootPassword="secure-root-password"
```

### Microsoft Azure (AKS)

```bash
# Create AKS cluster (if needed)
az aks create --resource-group myResourceGroup --name ethicalzen-cluster --node-count 3

# Get credentials
az aks get-credentials --resource-group myResourceGroup --name ethicalzen-cluster

# Deploy
helm upgrade --install ethicalzen-runtime ./helm/ethicalzen-runtime \
  -n ethicalzen-runtime --create-namespace \
  -f values/azure.yaml \
  --set mysql.auth.password="secure-password-here" \
  --set mysql.auth.rootPassword="secure-root-password"
```

---

## 📊 Monitoring

### Access Prometheus

```bash
# Get Prometheus URL
kubectl get svc prometheus -n ethicalzen-runtime
# Access: http://<PROMETHEUS_IP>:9090
```

### Key Metrics

| Metric | Description |
|--------|-------------|
| `gateway_requests_total` | Total API requests |
| `gateway_violations_total` | Blocked requests |
| `guardrail_evaluation_duration_seconds` | Eval latency |
| `cache_hit_ratio` | Redis cache efficiency |

---

## 🔐 Using with Your Applications

### SDK Integration (Recommended)

```javascript
const { EthicalZenProxyClient } = require('@ethicalzen/sdk');

const client = new EthicalZenProxyClient({
  gatewayURL: 'http://34.60.61.15',  // Your Gateway IP
  certificateId: 'dc_your_certificate_id'
});

// All requests automatically protected
const response = await client.post(
  'https://api.openai.com/v1/chat/completions',
  {
    model: 'gpt-4',
    messages: [{ role: 'user', content: 'Hello!' }]
  },
  { headers: { 'Authorization': `Bearer ${OPENAI_KEY}` } }
);
```

### Direct API Integration

```bash
curl -X POST "http://GATEWAY_IP/api/proxy" \
  -H "Content-Type: application/json" \
  -H "X-DC-Id: your-certificate-id" \
  -H "X-Target-Endpoint: https://api.openai.com/v1/chat/completions" \
  -H "Authorization: Bearer YOUR_OPENAI_KEY" \
  -d '{
    "model": "gpt-4",
    "messages": [{"role": "user", "content": "Your prompt here"}]
  }'
```

---

## 🛠️ Troubleshooting

### Pods Not Starting

```bash
# Check pod status
kubectl describe pod <pod-name> -n ethicalzen-runtime

# Check logs
kubectl logs <pod-name> -n ethicalzen-runtime
```

### Eval Engine CrashLoopBackOff

Ensure `evalEngine.image` is set to the same image as the gateway:

```yaml
evalEngine:
  image: "us-central1-docker.pkg.dev/ethicalzen-public-04085/cloud-run-source-deploy/acvps-gateway:latest"
```

### Connection Refused on Health Check

Ensure the eval engine uses `SG_PORT` (not `PORT`):

```yaml
# In templates/eval-engine.yaml
env:
  - name: SG_PORT
    value: "8091"
```

### LoadBalancer Pending

If `EXTERNAL-IP` shows `<pending>`:
- GKE: Ensure you have sufficient quota for external IPs
- EKS: Ensure AWS Load Balancer Controller is installed
- On-premises: Use `NodePort` instead of `LoadBalancer`

---

## 📞 Support

- **Documentation**: https://docs.ethicalzen.ai
- **Dashboard**: https://ethicalzen.ai/dashboard
- **Email**: support@ethicalzen.ai

---

## 📝 License

This deployment accelerator is provided under the EthicalZen Enterprise License.
See [LICENSE](LICENSE) for details.

