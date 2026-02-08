#!/bin/bash
# ============================================================================
# EthicalZen Demo — Setup Contract on Backend
# Registers guardrails and creates a Deterministic Contract via the backend API
#
# Usage:
#   ./e2e/setup-contract.sh                   # Uses defaults from .env
#   ./e2e/setup-contract.sh --tenant demo     # Custom tenant
# ============================================================================
# Note: no set -e — we track failures via counters and want all steps to run
set -uo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
source "${SCRIPT_DIR}/lib/colors.sh"
source "${SCRIPT_DIR}/lib/assert.sh"
source "${SCRIPT_DIR}/lib/health.sh"

# Load .env
[ -f "${SCRIPT_DIR}/.env" ] && source "${SCRIPT_DIR}/.env"

BACKEND_URL="${ETHICALZEN_BACKEND_URL:-https://api.ethicalzen.ai}"
API_KEY="${ETHICALZEN_API_KEY:-}"
TENANT_ID="${DEMO_TENANT_ID:-demo}"
RATE_DELAY="${RATE_DELAY:-7}"  # seconds between API calls (demo key: 10 req/min)

# Parse args
for arg in "$@"; do
  case "$arg" in
    --tenant)       shift; TENANT_ID="${1:-demo}" ;;
    --tenant=*)     TENANT_ID="${arg#*=}" ;;
    --delay=*)      RATE_DELAY="${arg#*=}" ;;
    --no-delay)     RATE_DELAY=0 ;;
  esac
done

if [ -z "$API_KEY" ]; then
  print_error "ETHICALZEN_API_KEY not set. Copy env.example to .env and fill in your key."
  exit 1
fi

print_header "Setting Up Demo Contract on Backend"

echo -e "  Backend:    ${BOLD}${BACKEND_URL}${NC}"
echo -e "  Tenant:     ${BOLD}${TENANT_ID}${NC}"
echo ""

require_jq

FAILURES=0

rate_wait() {
  if [ "$RATE_DELAY" -gt 0 ] 2>/dev/null; then
    sleep "$RATE_DELAY"
  fi
}

# Curl wrapper with rate-limit retry
api_call() {
  local method="$1"
  local url="$2"
  local data="${3:-}"
  local attempt=0
  local max_retries=2

  while [ $attempt -le $max_retries ]; do
    local response
    if [ -n "$data" ]; then
      response=$(curl -s -w "\n%{http_code}" -X "$method" "$url" \
        -H "Content-Type: application/json" \
        -H "X-API-Key: ${API_KEY}" \
        -H "X-Tenant-ID: ${TENANT_ID}" \
        -d "$data" 2>/dev/null)
    else
      response=$(curl -s -w "\n%{http_code}" -X "$method" "$url" \
        -H "Content-Type: application/json" \
        -H "X-API-Key: ${API_KEY}" \
        -H "X-Tenant-ID: ${TENANT_ID}" 2>/dev/null)
    fi

    local http_code
    http_code=$(echo "$response" | tail -1)
    local body
    body=$(echo "$response" | sed '$d')

    # Rate limited — wait and retry
    if [ "$http_code" = "429" ]; then
      attempt=$((attempt + 1))
      if [ $attempt -le $max_retries ]; then
        local retry_after
        retry_after=$(echo "$body" | jq -r '.retryAfter // 60' 2>/dev/null)
        print_info "Rate limited — waiting ${retry_after}s (attempt ${attempt}/${max_retries})"
        sleep "$retry_after"
        continue
      fi
    fi

    echo "$body"
    return 0
  done

  echo '{"error":"rate limit exceeded after retries"}'
}

# ── Step 1: Verify Backend ───────────────────────────────────
print_step 1 "Verifying backend connectivity"
health=$(curl -sf "${BACKEND_URL}/health" 2>/dev/null || echo "")
if [ -n "$health" ]; then
  print_pass "Backend reachable"
else
  print_error "Backend not reachable at ${BACKEND_URL}"
  exit 1
fi

# ── Step 2: Check Existing Guardrails ────────────────────────
print_step 2 "Checking guardrails for tenant ${TENANT_ID}"

REQUIRED_IDS=("pii_blocker" "hipaa_compliance" "prompt_injection_blocker" "toxicity_detector" "medical_advice_smart")

# Query existing guardrails
rate_wait
existing_response=$(api_call "POST" "${BACKEND_URL}/api/guardrails/query" '{}')
existing_ids=$(echo "$existing_response" | jq -r '.guardrails[]?.id // empty' 2>/dev/null | sort -u)

REGISTERED=0
MISSING=()

for gid in "${REQUIRED_IDS[@]}"; do
  if echo "$existing_ids" | grep -qx "$gid"; then
    print_pass "${gid} — already registered"
    REGISTERED=$((REGISTERED + 1))
  else
    MISSING+=("$gid")
  fi
done

# Register missing guardrails (POST /api/guardrails/add requires: id, name)
if [ ${#MISSING[@]} -gt 0 ]; then
  print_info "Registering ${#MISSING[@]} missing guardrail(s)..."

  get_guardrail_meta() {
    case "$1" in
      pii_blocker)                echo "PII Blocker|regex|Detects and blocks PII including SSN, email, and credit card numbers" ;;
      hipaa_compliance)           echo "HIPAA Compliance|hybrid|Enforces HIPAA compliance by detecting PHI in patient data" ;;
      prompt_injection_blocker)   echo "Prompt Injection Blocker|regex|Prevents prompt injection and jailbreak attempts" ;;
      toxicity_detector)          echo "Toxicity Detector|regex|Detects toxic, harmful, or abusive content" ;;
      medical_advice_smart)       echo "Medical Advice Guard|smart_guardrail|Blocks unauthorized medical advice and prescriptions" ;;
      *)                          echo "Unknown|unknown|Unknown guardrail" ;;
    esac
  }

  for gid in "${MISSING[@]}"; do
    IFS='|' read -r gname gtype gdesc <<< "$(get_guardrail_meta "$gid")"
    rate_wait

    body=$(jq -n \
      --arg id "$gid" \
      --arg name "$gname" \
      --arg type "$gtype" \
      --arg desc "$gdesc" \
      '{id: $id, name: $name, type: $type, description: $desc, status: "active", isBlock: true}')

    response=$(api_call "POST" "${BACKEND_URL}/api/guardrails/add" "$body")

    if echo "$response" | jq -e '.success' &>/dev/null 2>&1; then
      print_pass "${gid} — registered"
      REGISTERED=$((REGISTERED + 1))
    else
      error_msg=$(echo "$response" | jq -r '.error // "unknown"' 2>/dev/null)
      if echo "$error_msg" | grep -qi "already exists\|duplicate"; then
        print_info "${gid} — already registered"
        REGISTERED=$((REGISTERED + 1))
      else
        print_warn "${gid} — ${error_msg}"
        FAILURES=$((FAILURES + 1))
      fi
    fi
  done
fi

echo -e "  ${DIM}${REGISTERED}/${#REQUIRED_IDS[@]} guardrails ready${NC}"
echo ""

# ── Step 3: Create Deterministic Contract ────────────────────
print_step 3 "Creating Deterministic Contract"

rate_wait

# ContractService requires: name, envelope.constraints, guardrails
CONTRACT_BODY=$(cat <<'CONTRACTJSON'
{
  "name": "MedFirst Healthcare Triage Contract",
  "use_case": "healthcare-triage",
  "industry": "HEALTHCARE",
  "guardrails": ["pii_blocker", "hipaa_compliance", "prompt_injection_blocker", "toxicity_detector", "medical_advice_smart"],
  "config": {
    "enforce_on_request": true,
    "enforce_on_response": true,
    "threshold": 0.85
  },
  "envelope": {
    "constraints": {
      "pii_risk": { "min": 0, "max": 0.2 },
      "toxicity": { "min": 0, "max": 0.3 },
      "injection_risk": { "min": 0, "max": 0.2 },
      "hipaa_compliance": { "min": 0.8, "max": 1.0 },
      "medical_accuracy": { "min": 0.7, "max": 1.0 }
    }
  }
}
CONTRACTJSON
)

contract_response=$(api_call "POST" "${BACKEND_URL}/api/dc/contracts" "$CONTRACT_BODY")

# Extract contract ID from response
CONTRACT_ID=$(echo "$contract_response" | jq -r '.contract.id // .id // .contractId // ""' 2>/dev/null)
CONTRACT_STATUS=$(echo "$contract_response" | jq -r '.contract.status // .status // ""' 2>/dev/null)

if [ -n "$CONTRACT_ID" ] && [ "$CONTRACT_ID" != "null" ] && [ "$CONTRACT_ID" != "" ]; then
  print_pass "Contract created: ${CONTRACT_ID} (status: ${CONTRACT_STATUS})"

  # Show envelope constraints
  echo -e "  ${DIM}Envelope constraints:${NC}"
  echo "$contract_response" | jq -r '.contract.envelope.constraints // {} | to_entries[] | "    \(.key): [\(.value.min) – \(.value.max)]"' 2>/dev/null || true
elif echo "$contract_response" | jq -e '.error' &>/dev/null 2>&1; then
  error=$(echo "$contract_response" | jq -r '.error // "unknown"' 2>/dev/null)
  message=$(echo "$contract_response" | jq -r '.message // ""' 2>/dev/null)

  if echo "$error $message" | grep -qi "already exists\|duplicate\|CONTRACT_EXISTS"; then
    print_info "Contract already exists"
    rate_wait
    list_response=$(api_call "GET" "${BACKEND_URL}/api/dc/contracts")
    CONTRACT_ID=$(echo "$list_response" | jq -r '(.contracts // .)[-1].id // "unknown"' 2>/dev/null || echo "unknown")
    print_info "Using existing contract: ${CONTRACT_ID}"
  else
    print_warn "Contract creation: ${message:-$error}"
    CONTRACT_ID="demo-fallback"
    FAILURES=$((FAILURES + 1))
  fi
else
  print_warn "Unexpected contract response"
  CONTRACT_ID="demo-fallback"
  FAILURES=$((FAILURES + 1))
fi

# ── Step 4: Verify Contract ─────────────────────────────────
if [ "$CONTRACT_ID" != "demo-fallback" ]; then
  print_step 4 "Verifying contract"
  rate_wait

  encoded_id=$(python3 -c "import urllib.parse; print(urllib.parse.quote('${CONTRACT_ID}', safe=''))" 2>/dev/null || echo "$CONTRACT_ID")
  verify_response=$(api_call "GET" "${BACKEND_URL}/api/dc/contracts/${encoded_id}")

  if echo "$verify_response" | jq -e '.id // .contract.id' &>/dev/null 2>&1; then
    verified_id=$(echo "$verify_response" | jq -r '.id // .contract.id' 2>/dev/null)
    verified_status=$(echo "$verify_response" | jq -r '.status // "unknown"' 2>/dev/null)
    guardrail_count=$(echo "$verify_response" | jq -r '.guardrails | if type == "string" then (. | fromjson | length) elif type == "array" then length else 0 end' 2>/dev/null || echo "?")
    print_pass "Contract verified — ${verified_id} (${verified_status}, ${guardrail_count} guardrails)"
  else
    print_info "Contract stored — verification via GET not available"
  fi
else
  print_step 4 "Skipping verification (contract creation failed)"
fi

# ── Summary ──────────────────────────────────────────────────
echo ""
print_header "Contract Setup Complete"
echo -e "  ${BOLD}Contract ID:${NC}  ${CONTRACT_ID}"
echo -e "  ${BOLD}Tenant:${NC}       ${TENANT_ID}"
echo -e "  ${BOLD}Guardrails:${NC}   ${REGISTERED}/${#REQUIRED_IDS[@]} registered"
echo -e "  ${BOLD}Industry:${NC}     HEALTHCARE"
echo -e "  ${BOLD}Failures:${NC}     ${FAILURES}"
echo ""

if [ "$FAILURES" -eq 0 ]; then
  echo -e "  ${GREEN}All setup steps succeeded${NC}"
else
  echo -e "  ${YELLOW}${FAILURES} step(s) had warnings — demo may still work${NC}"
fi

echo ""
echo -e "  ${DIM}Use this contract ID for proxy tests:${NC}"
echo -e "    export CONTRACT_ID=${CONTRACT_ID}"
echo ""

# Export for downstream scripts
export CONTRACT_ID
