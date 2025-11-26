# 🌐 ACVPS Gateway - Browser Access Guide

## ✅ Yes! You Can Use This in the Browser

The ACVPS Gateway exposes **HTTP endpoints** that you can access directly from any web browser.

---

## 🔗 Available Browser Endpoints

Once the gateway is running, you can access these URLs in your browser:

### 1. Health Check Dashboard
```
http://localhost:9090/health
```

**What you'll see:**
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
    "hit_rate": 0.99
  }
}
```

**Use case:** Quick visual check that the gateway is running

---

### 2. Prometheus Metrics
```
http://localhost:9090/metrics
```

**What you'll see:**
```
# HELP acvps_requests_total Total number of requests processed
# TYPE acvps_requests_total counter
acvps_requests_total 1234

# HELP acvps_cache_hit_ratio Cache hit ratio
# TYPE acvps_cache_hit_ratio gauge
acvps_cache_hit_ratio 0.99

# HELP acvps_validation_duration_seconds Validation duration
# TYPE acvps_validation_duration_seconds histogram
acvps_validation_duration_seconds_bucket{le="0.005"} 1200
...
```

**Use case:** See real-time metrics and performance stats

---

### 3. Proxied Backend Requests
```
http://localhost:8443/api/patient/records?patient_id=123456
```

**What happens:**
1. Your browser sends request to gateway (port 8443)
2. Gateway validates DC headers (if present)
3. Gateway forwards to backend service
4. Response comes back through gateway

**Use case:** Test the full proxy flow

---

## 🎨 Enhanced Browser Experience

Let me create a simple **Web Dashboard** for you:

### Dashboard Features:
- ✅ Gateway health status (live)
- ✅ Real-time metrics
- ✅ Cache hit rate visualization
- ✅ Request counter
- ✅ Blockchain connection status
- ✅ Test API calls

### Create Dashboard

Run this to create a web dashboard:

```bash
cd /Users/srinivasvaravooru/workspace/acvps-gateway

cat > dashboard.html << 'EOFDASH'
<!DOCTYPE html>
<html lang="en">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>ACVPS Gateway Dashboard</title>
    <style>
        * {
            margin: 0;
            padding: 0;
            box-sizing: border-box;
        }
        
        body {
            font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, 'Helvetica Neue', Arial, sans-serif;
            background: linear-gradient(135deg, #667eea 0%, #764ba2 100%);
            color: #333;
            padding: 20px;
            min-height: 100vh;
        }
        
        .container {
            max-width: 1200px;
            margin: 0 auto;
        }
        
        h1 {
            color: white;
            text-align: center;
            margin-bottom: 30px;
            font-size: 2.5em;
            text-shadow: 2px 2px 4px rgba(0,0,0,0.2);
        }
        
        .cards {
            display: grid;
            grid-template-columns: repeat(auto-fit, minmax(300px, 1fr));
            gap: 20px;
            margin-bottom: 30px;
        }
        
        .card {
            background: white;
            border-radius: 12px;
            padding: 25px;
            box-shadow: 0 10px 30px rgba(0,0,0,0.2);
            transition: transform 0.3s;
        }
        
        .card:hover {
            transform: translateY(-5px);
        }
        
        .card h2 {
            font-size: 1.2em;
            color: #667eea;
            margin-bottom: 15px;
            display: flex;
            align-items: center;
            gap: 10px;
        }
        
        .status {
            width: 12px;
            height: 12px;
            border-radius: 50%;
            display: inline-block;
        }
        
        .status.healthy {
            background: #10b981;
            box-shadow: 0 0 10px #10b981;
        }
        
        .status.unhealthy {
            background: #ef4444;
            box-shadow: 0 0 10px #ef4444;
        }
        
        .metric {
            display: flex;
            justify-content: space-between;
            padding: 10px 0;
            border-bottom: 1px solid #f0f0f0;
        }
        
        .metric:last-child {
            border-bottom: none;
        }
        
        .metric-value {
            font-weight: bold;
            color: #667eea;
        }
        
        .big-number {
            font-size: 3em;
            font-weight: bold;
            color: #667eea;
            text-align: center;
            margin: 20px 0;
        }
        
        button {
            background: #667eea;
            color: white;
            border: none;
            padding: 12px 24px;
            border-radius: 8px;
            font-size: 1em;
            cursor: pointer;
            transition: background 0.3s;
            width: 100%;
            margin-top: 10px;
        }
        
        button:hover {
            background: #5568d3;
        }
        
        .test-result {
            margin-top: 15px;
            padding: 15px;
            border-radius: 8px;
            background: #f0f0f0;
            font-family: monospace;
            font-size: 0.9em;
            max-height: 200px;
            overflow-y: auto;
        }
        
        .loading {
            text-align: center;
            color: #999;
        }
    </style>
</head>
<body>
    <div class="container">
        <h1>🔒 ACVPS Gateway Dashboard</h1>
        
        <div class="cards">
            <!-- Health Status -->
            <div class="card">
                <h2>
                    <span class="status" id="status-indicator"></span>
                    Gateway Health
                </h2>
                <div class="metric">
                    <span>Status</span>
                    <span class="metric-value" id="health-status">Loading...</span>
                </div>
                <div class="metric">
                    <span>Version</span>
                    <span class="metric-value" id="version">-</span>
                </div>
                <div class="metric">
                    <span>Uptime</span>
                    <span class="metric-value" id="uptime">-</span>
                </div>
            </div>
            
            <!-- Blockchain -->
            <div class="card">
                <h2>⛓️ Blockchain</h2>
                <div class="metric">
                    <span>Connected</span>
                    <span class="metric-value" id="blockchain-connected">-</span>
                </div>
                <div class="metric">
                    <span>Block Number</span>
                    <span class="metric-value" id="block-number">-</span>
                </div>
                <div class="metric">
                    <span>Contract Address</span>
                    <span class="metric-value" style="font-size: 0.8em;">0x5FbDB...</span>
                </div>
            </div>
            
            <!-- Cache -->
            <div class="card">
                <h2>💾 Cache Performance</h2>
                <div class="big-number" id="cache-hit-rate">-</div>
                <div style="text-align: center; color: #999;">Hit Rate</div>
                <div class="metric">
                    <span>Total Hits</span>
                    <span class="metric-value" id="cache-hits">-</span>
                </div>
                <div class="metric">
                    <span>Total Misses</span>
                    <span class="metric-value" id="cache-misses">-</span>
                </div>
            </div>
            
            <!-- Requests -->
            <div class="card">
                <h2>📊 Request Stats</h2>
                <div class="big-number" id="total-requests">0</div>
                <div style="text-align: center; color: #999;">Total Requests</div>
                <div class="metric">
                    <span>Valid Contracts</span>
                    <span class="metric-value" id="valid-requests">-</span>
                </div>
                <div class="metric">
                    <span>Invalid Contracts</span>
                    <span class="metric-value" id="invalid-requests">-</span>
                </div>
            </div>
        </div>
        
        <!-- Test Section -->
        <div class="cards">
            <div class="card">
                <h2>🧪 Test Gateway</h2>
                <button onclick="testHealth()">Test Health Check</button>
                <button onclick="testMetrics()">Test Metrics</button>
                <button onclick="testProxy()">Test Proxy Request</button>
                <div class="test-result" id="test-result" style="display: none;"></div>
            </div>
            
            <div class="card">
                <h2>📖 Quick Links</h2>
                <button onclick="window.open('http://localhost:9090/health', '_blank')">
                    Open Health JSON
                </button>
                <button onclick="window.open('http://localhost:9090/metrics', '_blank')">
                    Open Metrics
                </button>
                <button onclick="window.open('http://localhost:8443/', '_blank')">
                    Open Gateway
                </button>
            </div>
        </div>
    </div>
    
    <script>
        let startTime = Date.now();
        
        async function fetchHealth() {
            try {
                const response = await fetch('http://localhost:9090/health');
                const data = await response.json();
                
                document.getElementById('health-status').textContent = data.status || 'Unknown';
                document.getElementById('version').textContent = data.version || 'dev';
                document.getElementById('status-indicator').className = 
                    'status ' + (data.status === 'healthy' ? 'healthy' : 'unhealthy');
                
                if (data.blockchain) {
                    document.getElementById('blockchain-connected').textContent = 
                        data.blockchain.connected ? '✅ Yes' : '❌ No';
                    document.getElementById('block-number').textContent = 
                        data.blockchain.block_number || '-';
                }
                
                if (data.cache) {
                    const hitRate = (data.cache.hit_rate * 100).toFixed(1);
                    document.getElementById('cache-hit-rate').textContent = hitRate + '%';
                }
            } catch (error) {
                document.getElementById('health-status').textContent = 'Offline';
                document.getElementById('status-indicator').className = 'status unhealthy';
                console.error('Health check failed:', error);
            }
        }
        
        async function fetchMetrics() {
            try {
                const response = await fetch('http://localhost:9090/metrics');
                const text = await response.text();
                
                // Parse Prometheus metrics
                const totalMatch = text.match(/acvps_requests_total (\d+)/);
                if (totalMatch) {
                    document.getElementById('total-requests').textContent = totalMatch[1];
                }
                
                // You can parse more metrics here
            } catch (error) {
                console.error('Metrics fetch failed:', error);
            }
        }
        
        function updateUptime() {
            const seconds = Math.floor((Date.now() - startTime) / 1000);
            const minutes = Math.floor(seconds / 60);
            const hours = Math.floor(minutes / 60);
            
            if (hours > 0) {
                document.getElementById('uptime').textContent = `${hours}h ${minutes % 60}m`;
            } else if (minutes > 0) {
                document.getElementById('uptime').textContent = `${minutes}m ${seconds % 60}s`;
            } else {
                document.getElementById('uptime').textContent = `${seconds}s`;
            }
        }
        
        async function testHealth() {
            const result = document.getElementById('test-result');
            result.style.display = 'block';
            result.textContent = 'Testing...';
            
            try {
                const response = await fetch('http://localhost:9090/health');
                const data = await response.json();
                result.textContent = JSON.stringify(data, null, 2);
            } catch (error) {
                result.textContent = 'Error: ' + error.message;
            }
        }
        
        async function testMetrics() {
            const result = document.getElementById('test-result');
            result.style.display = 'block';
            result.textContent = 'Testing...';
            
            try {
                const response = await fetch('http://localhost:9090/metrics');
                const text = await response.text();
                result.textContent = text.split('\n').slice(0, 20).join('\n') + '\n...';
            } catch (error) {
                result.textContent = 'Error: ' + error.message;
            }
        }
        
        async function testProxy() {
            const result = document.getElementById('test-result');
            result.style.display = 'block';
            result.textContent = 'Testing proxy request...';
            
            try {
                const response = await fetch('http://localhost:8443/api/health');
                const text = await response.text();
                result.textContent = text;
            } catch (error) {
                result.textContent = 'Error: ' + error.message + 
                    '\n\nNote: Backend service must be running on port 9001';
            }
        }
        
        // Auto-refresh every 2 seconds
        setInterval(() => {
            fetchHealth();
            fetchMetrics();
            updateUptime();
        }, 2000);
        
        // Initial load
        fetchHealth();
        fetchMetrics();
        updateUptime();
    </script>
</body>
</html>
EOFDASH

echo "✅ Dashboard created: dashboard.html"
```

---

## 🚀 How to Use the Dashboard

### Step 1: Start the Gateway

```bash
cd /Users/srinivasvaravooru/workspace/acvps-gateway

# Install Redis first (if not already)
brew install redis
brew services start redis

# Start the gateway
./acvps-gateway --config config.yaml
```

### Step 2: Open Dashboard in Browser

```bash
# Option A: Direct file access
open dashboard.html

# Option B: Via simple HTTP server
python3 -m http.server 8000
# Then open: http://localhost:8000/dashboard.html
```

### Step 3: Explore!

You'll see:
- ✅ Live health status
- ✅ Real-time metrics
- ✅ Cache performance
- ✅ Request counters
- ✅ Test buttons to try API calls
- ✅ Quick links to raw endpoints

---

## 📱 Browser Testing Scenarios

### Scenario 1: Monitor Gateway Health

1. Open `http://localhost:9090/health` in browser
2. See real-time health status
3. Refresh to see updates

### Scenario 2: View Performance Metrics

1. Open `http://localhost:9090/metrics` in browser
2. See Prometheus metrics (raw format)
3. Or use the dashboard for pretty visualization

### Scenario 3: Test Proxy Flow

1. Start a backend service (e.g., patient-records-service)
2. Open `http://localhost:8443/api/patient/records?patient_id=123456`
3. See request proxied through gateway

### Scenario 4: Monitor with Dashboard

1. Open `dashboard.html` in browser
2. Watch metrics update every 2 seconds
3. Click test buttons to try different endpoints
4. See cache hit rate, request counts, etc.

---

## 🔧 CORS Configuration

The gateway already has **CORS enabled** in the config:

```yaml
cors:
  enabled: true
  allowed_origins: ["*"]
  allowed_methods: ["GET", "POST", "PUT", "DELETE", "OPTIONS"]
  allowed_headers: ["*"]
```

This means **any web page can call the gateway APIs** from JavaScript!

---

## 💡 JavaScript Examples

### Fetch Health Status

```javascript
fetch('http://localhost:9090/health')
  .then(response => response.json())
  .then(data => {
    console.log('Gateway status:', data.status);
    console.log('Blockchain connected:', data.blockchain.connected);
    console.log('Cache hit rate:', data.cache.hit_rate);
  });
```

### Call Through Gateway (with DC headers)

```javascript
fetch('http://localhost:8443/api/patient/records?patient_id=123456', {
  headers: {
    'X-DC-Id': 'patient-service/healthcare/us/v1.0',
    'X-DC-Digest': 'sha256-abc123...',
  }
})
  .then(response => response.json())
  .then(data => console.log('Patient data:', data));
```

### Watch Metrics Live

```javascript
setInterval(async () => {
  const response = await fetch('http://localhost:9090/metrics');
  const text = await response.text();
  
  const requests = text.match(/acvps_requests_total (\d+)/);
  console.log('Total requests:', requests ? requests[1] : 0);
}, 2000);
```

---

## 🎯 Summary

**Yes, you can use ACVPS Gateway in the browser!**

**Available URLs:**
- `http://localhost:9090/health` - Health check
- `http://localhost:9090/metrics` - Prometheus metrics
- `http://localhost:8443/*` - Proxied requests

**Enhanced Experience:**
- Custom HTML dashboard with live updates
- Visual metrics and performance stats
- Test buttons for quick validation
- Auto-refresh every 2 seconds

**Just open `dashboard.html` in any browser and you're good to go!** 🚀
