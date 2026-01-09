# 🔴 RED Team Security Audit Report

**Repository:** ethicalzen-accelerators  
**Audit Date:** January 9, 2026  
**Auditor:** Automated Security Scan  

---

## Executive Summary

| Severity | Count | Status |
|----------|-------|--------|
| 🔴 Critical | 1 | Requires Fix |
| 🟠 High | 2 | Requires Fix |
| 🟡 Medium | 3 | Should Fix |
| 🔵 Low | 2 | Consider |
| ✅ Pass | 6 | No Action |

---

## 🔴 CRITICAL FINDINGS

### 1. Internal GCP Project ID Exposed (59 occurrences)

**Location:** Multiple files across repository

**Issue:** Internal Cloud Run URLs expose GCP project ID `400782183161`

```
https://api.ethicalzen.ai
```

**Files Affected:**
- `live-api-testing/src/interactive-demo.js`
- `live-api-testing/src/ci-integration.js`
- `guardrails-sdk/examples/gdk-customer-journey.js`
- `self-deploy/env.example`
- `self-deploy/docker-compose.yml`
- `self-deploy/deploy.sh`
- All accelerator docker-compose files (healthcare, financial, legal, education, ecommerce)

**Risk:** 
- Exposes internal infrastructure details
- Could be used for targeted attacks
- Violates security best practice of hiding infrastructure

**Remediation:**
```bash
# Replace all instances with public API URL
find . -type f \( -name "*.js" -o -name "*.yml" -o -name "*.yaml" -o -name "*.sh" -o -name "*.md" \) \
  -exec sed -i '' 's|api.ethicalzen.ai|api.ethicalzen.ai|g' {} \;
```

---

## 🟠 HIGH FINDINGS

### 2. Default Passwords in Helm Templates

**Location:** `self-deploy/helm/ethicalzen-runtime/templates/mysql.yaml`

**Issue:** Default passwords hardcoded as fallback values

```yaml
mysql-root-password: {{ .Values.mysql.auth.rootPassword | default "ethicalzen-root-change-me" | quote }}
mysql-password: {{ .Values.mysql.auth.password | default "ethicalzen-change-me" | quote }}
```

**Risk:** Users may deploy with default passwords if not overridden

**Remediation:**
```yaml
# Remove defaults, require explicit values
mysql-root-password: {{ required "mysql.auth.rootPassword is required" .Values.mysql.auth.rootPassword | quote }}
mysql-password: {{ required "mysql.auth.password is required" .Values.mysql.auth.password | quote }}
```

---

### 3. Demo API Key Publicly Exposed

**Location:** Multiple files

**Issue:** Demo API key `sk-demo-public-playground-ethicalzen` hardcoded

```javascript
const ETHICALZEN_API_KEY = process.env.ETHICALZEN_API_KEY || 'sk-demo-public-playground-ethicalzen';
```

**Files:**
- `live-api-testing/src/demo.js`
- `live-api-testing/src/analyze-api.js`
- `live-api-testing/src/interactive-demo.js`
- `live-api-testing/LIVE_API_TESTING_GUIDE.md`
- `guardrails-sdk/examples/gdk-customer-journey.js`

**Risk Assessment:**
- ✅ If intentionally rate-limited demo key: **Acceptable**
- 🔴 If production key: **Critical vulnerability**

**Recommendation:** Confirm this is a rate-limited, read-only demo key with no write permissions.

---

## 🟡 MEDIUM FINDINGS

### 4. Internal Docker Registry Exposed

**Location:** Multiple docker-compose and helm files

**Issue:** Internal Artifact Registry URLs exposed

```yaml
image: us-central1-docker.pkg.dev/ethicalzen-public-04085/cloud-run-source-deploy/acvps-gateway:latest
```

**Files Affected:** 28 occurrences across:
- `self-deploy/values/*.yaml`
- `self-deploy/helm/ethicalzen-runtime/values.yaml`
- `self-deploy/docker-compose.yml`
- All accelerator `docker-compose.sdk.yml` files

**Risk:** Exposes GCP project name `ethicalzen-public-04085`

**Recommendation:** 
- If images are public: Document this is intentional
- If images are private: Use Docker Hub or create alias

---

### 5. Prometheus Admin Password in Values

**Location:** `self-deploy/helm/ethicalzen-runtime/values.yaml`

```yaml
adminPassword: "admin"
```

**Risk:** Default admin password for monitoring

**Remediation:**
```yaml
adminPassword: ""  # Must be set during installation
```

---

### 6. No .gitignore Enforcement for Secrets

**Issue:** No `.env` files found (good), but `.env.example` files contain placeholder patterns that could be copy-pasted incorrectly.

**Recommendation:** Add pre-commit hooks to prevent secret commits.

---

## 🔵 LOW FINDINGS

### 7. Localhost URLs in Documentation/Tests

**Location:** 154 occurrences

**Issue:** Many `localhost:3000`, `localhost:8080` references

**Assessment:** ✅ Acceptable - These are for local development/testing

---

### 8. No Code Injection Vulnerabilities

**Scan Result:** ✅ PASS

Checked for: `eval()`, `exec()`, `child_process`, `spawn()`, `execSync()`

**Result:** No dangerous code execution patterns found.

---

## ✅ PASSED CHECKS

| Check | Result |
|-------|--------|
| No hardcoded real API keys (sk-xxx 20+ chars) | ✅ Pass |
| No private keys (BEGIN PRIVATE) | ✅ Pass |
| No AWS credentials | ✅ Pass |
| No .env files committed | ✅ Pass |
| No code injection vulnerabilities | ✅ Pass |
| No Google credentials JSON | ✅ Pass |

---

## 📋 REMEDIATION PLAN

### Priority 1 (Do Now)

```bash
# 1. Replace internal backend URLs with public API
cd /Users/srinivasvaravooru/workspace/ethicalzen-accelerators

# Replace in all .js files
find . -name "*.js" -exec sed -i '' 's|https://api.ethicalzen.ai|https://api.ethicalzen.ai|g' {} \;

# Replace in all .yml/.yaml files  
find . -name "*.yml" -o -name "*.yaml" -exec sed -i '' 's|api.ethicalzen.ai|api.ethicalzen.ai|g' {} \;

# Replace in all .md files
find . -name "*.md" -exec sed -i '' 's|api.ethicalzen.ai|api.ethicalzen.ai|g' {} \;

# Replace in all .sh files
find . -name "*.sh" -exec sed -i '' 's|api.ethicalzen.ai|api.ethicalzen.ai|g' {} \;
```

### Priority 2 (This Week)

1. Remove default MySQL passwords from Helm templates
2. Add required() validation for sensitive values
3. Document that demo API key is intentionally public/rate-limited

### Priority 3 (This Month)

1. Set up pre-commit hooks for secret scanning
2. Consider migrating Docker images to Docker Hub for cleaner URLs
3. Add security.md with responsible disclosure info

---

## Summary

The repository is **generally secure** but has **infrastructure exposure issues** that should be fixed before public release:

1. 🔴 Replace all internal Cloud Run URLs with `api.ethicalzen.ai`
2. 🟠 Remove default passwords from Helm templates
3. 🟡 Document intentional public exposure of demo keys and registry

**Estimated fix time:** 30 minutes for Priority 1 items.

