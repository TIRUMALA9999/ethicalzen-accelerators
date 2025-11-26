# ACVPS Gateway - Quick Setup

**One script for all deployments:** Local dev, Customer VPC, On-premise

---

## 🚀 Quick Start

```bash
cd acvps-gateway
./setup-local-gateway.sh
```

That's it! The script will:
1. ✅ Check prerequisites (Docker, curl)
2. ✅ Ask where you're deploying (local/VPC/on-prem)
3. ✅ Register a new gateway OR use existing credentials
4. ✅ Configure backend URL
5. ✅ Create config files
6. ✅ Start the gateway

---

## 📋 What You Need

### For All Deployments
- Docker & Docker Compose installed
- EthicalZen account (sign up at https://ethicalzen.ai)

### For Gateway Registration
- Your email and password (the script has a direct login flow)

### For Your Backend
- Backend service URL (e.g., `http://localhost:9001` for local dev)

---

## 🎯 Deployment Options

The script supports **3 deployment modes**:

### 1. Local Development (Docker on laptop)
```bash
./setup-local-gateway.sh
# Choose: 1) Local development
```

**Result:** Gateway runs on `localhost:8443`

---

### 2. Customer VPC (GCP Cloud Run)
```bash
# First: gcloud auth login
./setup-local-gateway.sh
# Choose: 2) Customer VPC (GCP Cloud Run)
```

**Result:** Gateway deployed to Cloud Run, auto-scaled

---

### 3. Customer VPC (Docker/Kubernetes)
```bash
./setup-local-gateway.sh
# Choose: 3) Customer VPC (Docker/Kubernetes)
```

**Result:** Config files created, manual deployment commands provided

---

## 🔑 Credentials Management

### Option 1: Register New Gateway (Recommended)
The script will:
1. Ask for your JWT token
2. Call `/api/gateway/register`
3. Save credentials to `.env.local`

### Option 2: Use Existing Credentials
If you already have gateway credentials:
1. Choose "Yes, I have gateway credentials"
2. Enter your `GATEWAY_API_KEY` and `GATEWAY_TENANT_ID`

---

## 📂 Files Created

After setup, you'll have:

```
acvps-gateway/
├── .env.local              # Your credentials (DO NOT commit!)
├── config-local.yaml       # Gateway configuration
├── docker-compose-local.yaml  # Docker Compose setup
└── setup-local-gateway.sh  # This setup script
```

---

## 🧪 Testing Your Setup

### Test 1: Health Check
```bash
curl http://localhost:9090/metrics
```

### Test 2: Contracts Loaded
```bash
curl http://localhost:8443/api/contracts | jq
```

### Test 3: End-to-End Validation
```bash
# Without contract headers (should be blocked)
curl -X POST http://localhost:8443/api/query \
  -H "Content-Type: application/json" \
  -d '{"query": "test"}'

# Expected: 409 Conflict (DC headers required)
```

---

## 🔄 Common Commands

### View Logs
```bash
docker-compose -f docker-compose-local.yaml logs -f gateway
```

### Restart Gateway
```bash
docker-compose -f docker-compose-local.yaml restart gateway
```

### Stop Gateway
```bash
docker-compose -f docker-compose-local.yaml down
```

### Re-run Setup (Update Config)
```bash
./setup-local-gateway.sh
```

---

## 🏢 Enterprise/VPC Deployment

For customer VPC deployments, the script creates configuration but doesn't auto-deploy.

### GCP Cloud Run (Manual)
```bash
# Build image
docker build -t gcr.io/YOUR_PROJECT/acvps-gateway:latest .

# Push to GCR
docker push gcr.io/YOUR_PROJECT/acvps-gateway:latest

# Deploy
gcloud run deploy acvps-gateway \
  --image gcr.io/YOUR_PROJECT/acvps-gateway:latest \
  --env-vars-file .env.local \
  --region us-central1 \
  --allow-unauthenticated
```

### Kubernetes
```bash
# Create ConfigMap
kubectl create configmap gateway-config --from-file=config-local.yaml

# Create Secret
kubectl create secret generic gateway-creds --from-env-file=.env.local

# Deploy
kubectl apply -f - <<EOF
apiVersion: apps/v1
kind: Deployment
metadata:
  name: acvps-gateway
spec:
  replicas: 3
  selector:
    matchLabels:
      app: acvps-gateway
  template:
    metadata:
      labels:
        app: acvps-gateway
    spec:
      containers:
      - name: gateway
        image: gcr.io/ethicalzen-public-04085/acvps-gateway:latest
        ports:
        - containerPort: 8443
        - containerPort: 9090
        envFrom:
        - secretRef:
            name: gateway-creds
        volumeMounts:
        - name: config
          mountPath: /app/config.yaml
          subPath: config-local.yaml
      volumes:
      - name: config
        configMap:
          name: gateway-config
EOF
```

---

## 🛠️ Troubleshooting

### Script Fails: "Docker not found"
```bash
# Install Docker
# macOS: https://docs.docker.com/desktop/install/mac-install/
# Linux: https://docs.docker.com/engine/install/
```

### Script Fails: "Failed to register gateway"
Check:
1. JWT token is valid (not expired)
2. Control plane URL is reachable
3. You have an active EthicalZen account

### Gateway Won't Start
Check logs:
```bash
docker-compose -f docker-compose-local.yaml logs gateway
```

Common issues:
- Port 8443 already in use
- Invalid API key
- Backend URL unreachable

### No Contracts Loaded
Check:
1. Gateway API key is correct
2. You have approved contracts in the portal
3. Tenant ID matches your account

---

## 📖 Next Steps

1. **Test the gateway:** See "Testing Your Setup" above
2. **Integrate your app:** See `sdk/QUICKSTART.md`
3. **Monitor evidence:** https://ethicalzen.ai/dashboard.html
4. **Read full guide:** `docs/LOCAL_DEVELOPMENT_GUIDE.md`

---

## 🤝 Support

- **Issues:** https://github.com/aiworksllc/ethicalzen-platform/issues
- **Docs:** https://ethicalzen.ai/docs
- **Email:** support@ethicalzen.ai

---

**Made with ❤️ by EthicalZen**

