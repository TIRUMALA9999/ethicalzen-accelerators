# 🚀 ACVPS Gateway - Setup Instructions

## ✅ Current Status

**Build:** ✅ Complete (24MB binary ready)  
**Code:** ✅ 2,488 lines implemented  
**Ready to Test:** ✅ Yes (need dependencies)

---

## 📋 Prerequisites Checklist

### Required (Not Yet Installed)
- [ ] **Redis** - For contract caching
- [ ] **Docker** (Optional) - For easy dependency management
- [ ] **Hardhat Node** - For blockchain (or use existing from aipromptandseccheck)

### Already Installed ✅
- [x] **Go 1.25.2** - Build successful
- [x] **Homebrew** - Package manager
- [x] **Node.js** - For Hardhat

---

## 🔧 Installation Steps

### Option 1: Install Docker (Recommended - Easiest)

```bash
# Install Docker Desktop for Mac
brew install --cask docker

# Start Docker Desktop (from Applications)
# Then run:
docker run -d --name acvps-redis -p 6379:6379 redis:7-alpine
```

### Option 2: Install Redis Directly

```bash
# Install Redis via Homebrew
brew install redis

# Start Redis
brew services start redis

# Verify it's running
redis-cli ping  # Should return "PONG"
```

---

## 🏃 Quick Start (After Dependencies)

### Step 1: Start Redis

```bash
# If using Docker:
docker run -d --name acvps-redis -p 6379:6379 redis:7-alpine

# If using Homebrew:
brew services start redis
```

### Step 2: Start Blockchain (Use Existing)

```bash
# Use the existing Hardhat node from EthicalZen project
cd /Users/srinivasvaravooru/workspace/aipromptandseccheck/blockchain
npx hardhat node  # Keep this running

# In another terminal, deploy contract (if not already deployed)
npx hardhat run deploy.js --network localhost
# Note the deployed address!
```

### Step 3: Run ACVPS Gateway

```bash
cd /Users/srinivasvaravooru/workspace/acvps-gateway

# The config.yaml is already created for you
# Just update the contract address if needed:
# Edit config.yaml, line 10:
#   contract_address: "0x5FbDB2315678afecb367f032d93F642f64180aa3"

# Run the gateway
./acvps-gateway --config config.yaml
```

### Step 4: Test It!

```bash
# In another terminal:

# 1. Check health
curl http://localhost:9090/health

# Expected: {"status":"healthy","version":"dev"}

# 2. Test proxy (to backend on port 9001)
# First, start a backend service:
cd /Users/srinivasvaravooru/workspace/aipromptandseccheck/services
node patient-records-service.js &

# Then test through gateway:
curl http://localhost:8443/api/patient/records?patient_id=123456

# 3. Check metrics
curl http://localhost:9090/metrics | grep acvps
```

---

## 🐛 Troubleshooting

### Error: "Failed to initialize cache: connection refused"

**Problem:** Redis is not running  
**Solution:**
```bash
# Check if Redis is running
redis-cli ping

# If not, start it:
brew services start redis
# OR
docker start acvps-redis
```

### Error: "Failed to connect to Ethereum node"

**Problem:** Hardhat node is not running  
**Solution:**
```bash
cd /Users/srinivasvaravooru/workspace/aipromptandseccheck/blockchain
npx hardhat node
```

### Error: "Contract not found"

**Problem:** Wrong contract address in config  
**Solution:**
```bash
# Deploy the contract and get the address
cd /Users/srinivasvaravooru/workspace/aipromptandseccheck/blockchain
npx hardhat run deploy.js --network localhost

# Copy the address and update config.yaml
```

---

## 📊 What You'll See When It Works

### Terminal Output (Gateway)

```
{"level":"info","msg":"Starting ACVPS Gateway","version":"dev","time":"..."}
{"level":"info","msg":"✅ Cache initialized","time":"..."}
{"level":"info","msg":"✅ Blockchain client initialized","time":"..."}
{"level":"info","msg":"✅ Blockchain connection healthy","time":"..."}
{"level":"info","msg":"✅ Proxy handler initialized","time":"..."}
{"level":"info","msg":"✅ Metrics server configured","port":9090,"time":"..."}
{"level":"info","msg":"🚀 ACVPS Gateway started","port":8443,"tls":false,"time":"..."}
```

### Health Check Response

```json
{
  "status": "healthy",
  "version": "dev",
  "blockchain": {
    "connected": true,
    "block_number": 12345
  },
  "cache": {
    "connected": true,
    "hit_rate": 0.0
  }
}
```

### Metrics Output

```
acvps_requests_total 0
acvps_validation_duration_seconds_sum 0
acvps_cache_hit_ratio 0
```

---

## 🧪 Full End-to-End Test

Once the gateway is running, test the complete flow:

### 1. Register a Service Contract

```bash
curl -X POST http://localhost:8080/api/dc/services/register \
  -H "Content-Type: application/json" \
  -d '{
    "service_name": "test-service",
    "use_case": "Test ACVPS Gateway validation",
    "interface": {
      "endpoint": "/api/test",
      "method": "GET"
    },
    "industry": "healthcare",
    "jurisdiction": "us"
  }'

# Note the returned contract_id and policy_digest
```

### 2. Call Service Through Gateway

```bash
# With valid DC headers (should work)
curl http://localhost:8443/api/test \
  -H "X-DC-Id: test-service/healthcare/us/v1.0" \
  -H "X-DC-Digest: sha256-abc123..."

# Without DC headers (will pass through in 'observe' mode)
curl http://localhost:8443/api/test
```

### 3. Check Metrics

```bash
curl http://localhost:9090/metrics | grep acvps_requests_total
# Should show request count
```

---

## 🚀 Next Steps After Testing

### 1. Production Config

Update `config.yaml` for production:
- Set `validation.mode: "strict"` (enforce DC headers)
- Set `validation.require_dc_headers: true`
- Enable `mitigation.grounding.enabled: true`
- Set `logging.level: "warn"` (less verbose)
- Enable TLS with real certificates

### 2. Deploy to Production

```bash
# Build for production
GOOS=linux GOARCH=amd64 go build -o acvps-gateway-linux cmd/gateway/main.go

# Or use Docker
docker build -t ethicalzen/acvps-gateway:v1.0.0 -f docker/Dockerfile .
docker push ethicalzen/acvps-gateway:v1.0.0
```

### 3. Monitor in Production

- Grafana dashboard (import `grafana/dashboard.json`)
- Prometheus alerts
- Health check monitoring
- Log aggregation (ELK, Splunk, etc.)

---

## 📚 Documentation

- **README.md** - Complete overview
- **QUICKSTART.md** - 5-minute setup guide
- **MVP_COMPLETE.md** - What was built and why
- **ROADMAP.md** - Future plans

---

## ✅ Success Checklist

- [ ] Redis is running
- [ ] Hardhat node is running
- [ ] Gateway starts without errors
- [ ] Health check returns "healthy"
- [ ] Can proxy requests to backend
- [ ] Metrics are being collected
- [ ] Contract validation works (with DC headers)

---

**Once all dependencies are installed, the gateway is ready to run!**

**Estimated setup time: 10 minutes**

**Status:** ✅ Code complete, waiting for dependencies
