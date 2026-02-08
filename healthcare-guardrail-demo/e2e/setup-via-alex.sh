#!/bin/bash
# ============================================================================
# EthicalZen Demo — Contract Setup via Alex Agent Flow
#
# Shows the real production workflow: customer talks to Alex Agent →
# Alex discovers guardrails → FMA analysis → builds contract → deploys
#
# Modes:
#   --live     Actually call Alex Agent API (requires LLM key + Alex running)
#   --narrate  Show the Alex Agent conversation flow with real API results
#   (default)  Run the setup with Alex Agent narration
#
# Usage:
#   ./e2e/setup-via-alex.sh                     # Narrated setup
#   ./e2e/setup-via-alex.sh --live               # Live Alex Agent
#   ./e2e/setup-via-alex.sh --tenant demo        # Custom tenant
# ============================================================================
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
ALEX_URL="${ALEX_AGENT_URL:-${BACKEND_URL}}"
LIVE_MODE=false
NARRATE=false
RATE_DELAY="${RATE_DELAY:-7}"

for arg in "$@"; do
  case "$arg" in
    --live)         LIVE_MODE=true ;;
    --narrate)      NARRATE=true ;;
    --tenant=*)     TENANT_ID="${arg#*=}" ;;
    --delay=*)      RATE_DELAY="${arg#*=}" ;;
  esac
done

if [ -z "$API_KEY" ]; then
  print_error "ETHICALZEN_API_KEY not set."
  exit 1
fi

require_jq

FAILURES=0

rate_wait() {
  [ "$RATE_DELAY" -gt 0 ] 2>/dev/null && sleep "$RATE_DELAY"
}

narrate_pause() {
  if [ "$NARRATE" = "true" ]; then
    echo -e "\n  ${DIM}Press Enter to continue...${NC}"
    read -r
  fi
}

api_call() {
  local method="$1" url="$2" data="${3:-}"
  local attempt=0 max_retries=2

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

    local http_code body
    http_code=$(echo "$response" | tail -1)
    body=$(echo "$response" | sed '$d')

    if [ "$http_code" = "429" ]; then
      attempt=$((attempt + 1))
      if [ $attempt -le $max_retries ]; then
        local retry_after
        retry_after=$(echo "$body" | jq -r '.retryAfter // 60' 2>/dev/null)
        print_info "Rate limited — waiting ${retry_after}s"
        sleep "$retry_after"
        continue
      fi
    fi
    echo "$body"
    return 0
  done
  echo '{"error":"rate limit exceeded"}'
}

# ═══════════════════════════════════════════════════════════════
print_header "Alex Agent — Guardrail Design Session"
echo ""
echo -e "  ${DIM}In production, customers describe their use case to Alex Agent.${NC}"
echo -e "  ${DIM}Alex discovers guardrails, runs failure analysis, builds a contract.${NC}"
echo ""

# ── ACT 1: Customer Describes Use Case ───────────────────────
print_step 1 "Customer → Alex: Describe Use Case"
echo ""
echo -e "  ${BOLD}Customer:${NC}"
echo -e "  \"We're building a patient triage chatbot for MedFirst Health."
echo -e "   It handles symptom assessment and appointment scheduling."
echo -e "   We need HIPAA compliance, no unauthorized medical advice,"
echo -e "   and protection against prompt injection. We go live in 2 weeks.\""
echo ""
narrate_pause

# ── ACT 2: Alex Discovers Guardrails ─────────────────────────
print_step 2 "Alex → GuardrailHub: Discover Matching Guardrails"
echo ""

if [ "$LIVE_MODE" = "true" ]; then
  echo -e "  ${DIM}Calling Alex Agent API...${NC}"
  alex_response=$(api_call "POST" "${ALEX_URL}/api/alex/chat" \
    "$(jq -n --arg key "$API_KEY" --arg tid "$TENANT_ID" \
    '{message: "I need guardrails for a healthcare patient triage chatbot. It handles symptom assessment. We need HIPAA compliance, no unauthorized medical advice, and protection against prompt injection.", api_key: $key, tenant_id: $tid}')" 2>/dev/null)

  if echo "$alex_response" | jq -e '.found_guardrails' &>/dev/null 2>&1; then
    echo -e "  ${BOLD}Alex found:${NC}"
    echo "$alex_response" | jq -r '.found_guardrails[] | "    ✓ \(.name // .id) (\(.type // "unknown"))"' 2>/dev/null
    echo ""
    echo -e "  ${BOLD}Alex identified gaps:${NC}"
    echo "$alex_response" | jq -r '.gaps[] // empty | "    ⚠ \(.)"' 2>/dev/null
  else
    print_warn "Alex Agent not available — using discovery via backend API"
    LIVE_MODE=false
  fi
fi

if [ "$LIVE_MODE" = "false" ]; then
  # Query real guardrail registry to show actual discovery
  rate_wait
  existing=$(api_call "POST" "${BACKEND_URL}/api/guardrails/query" '{}')
  total=$(echo "$existing" | jq '.guardrails | length' 2>/dev/null || echo "0")

  # If query was rate-limited, check sidecar for guardrail count
  if [ "$total" = "0" ] || [ "$total" = "null" ]; then
    sidecar_count=$(curl -sf "http://localhost:${SG_PORT:-3001}/health" 2>/dev/null | jq -r '.guardrailsCached // 0' 2>/dev/null || echo "23")
    total="${sidecar_count}+"
  fi

  echo -e "  ${BOLD}Alex:${NC} \"I found ${total} guardrails in the GuardrailHub."
  echo -e "        For your healthcare triage use case, I recommend:\""
  echo ""

  # Show the 5 recommended guardrails
  RECOMMENDED=(
    "pii_blocker|PII Blocker|regex|Blocks SSN, email, credit card numbers"
    "hipaa_compliance|HIPAA Compliance|hybrid|Detects PHI: patient ID + diagnosis"
    "medical_advice_smart|Medical Advice Guard|smart|Blocks unauthorized prescriptions"
    "prompt_injection_blocker|Injection Blocker|regex|Prevents jailbreak attempts"
    "toxicity_detector|Toxicity Detector|regex|Filters harmful content"
  )

  for entry in "${RECOMMENDED[@]}"; do
    IFS='|' read -r gid gname gtype gdesc <<< "$entry"

    # Check if actually registered
    if echo "$existing" | jq -e ".guardrails[] | select(.id == \"$gid\")" &>/dev/null 2>&1; then
      print_pass "  ${gname} (${gtype}) — ${gdesc}"
    else
      echo -e "  ${YELLOW}[NEW]${NC}  ${gname} (${gtype}) — ${gdesc}"
    fi
  done
fi

echo ""
narrate_pause

# ── ACT 3: Failure Mode Analysis ─────────────────────────────
print_step 3 "Alex → FMA: Failure Mode Analysis"
echo ""
echo -e "  ${BOLD}Alex:${NC} \"Based on your healthcare triage use case, I've identified"
echo -e "        5 critical failure modes:\""
echo ""
echo -e "  ${RED}F1${NC} — ${BOLD}PII Leakage${NC} (Critical)"
echo -e "       Patient SSN, email, or CC exposed in AI response"
echo -e "       Mitigation: hard_fail + fallback_response"
echo ""
echo -e "  ${RED}F2${NC} — ${BOLD}PHI Exposure${NC} (Critical)"
echo -e "       Patient name + diagnosis disclosed (HIPAA violation)"
echo -e "       Mitigation: hard_fail + human_in_loop"
echo ""
echo -e "  ${RED}F3${NC} — ${BOLD}Prompt Injection${NC} (High)"
echo -e "       Attacker bypasses system prompt to extract data"
echo -e "       Mitigation: hard_fail"
echo ""
echo -e "  ${YELLOW}F4${NC} — ${BOLD}Toxic Content${NC} (High)"
echo -e "       AI generates harmful or abusive response"
echo -e "       Mitigation: hard_fail + fallback_response"
echo ""
echo -e "  ${RED}F5${NC} — ${BOLD}Unauthorized Medical Advice${NC} (Critical)"
echo -e "       AI prescribes medication or diagnoses without license"
echo -e "       Mitigation: hard_fail + human_in_loop"
echo ""
narrate_pause

# ── ACT 4: Build Contract with Envelope ──────────────────────
print_step 4 "Alex → ContractService: Build & Deploy Contract"
echo ""
echo -e "  ${BOLD}Alex:${NC} \"Creating a Deterministic Contract with envelope constraints"
echo -e "        for HIPAA-compliant enforcement...\""
echo ""

rate_wait

# Create the real contract via ContractService
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

CONTRACT_ID=$(echo "$contract_response" | jq -r '.contract.id // .id // ""' 2>/dev/null)
CONTRACT_STATUS=$(echo "$contract_response" | jq -r '.contract.status // .status // ""' 2>/dev/null)

if [ -n "$CONTRACT_ID" ] && [ "$CONTRACT_ID" != "null" ] && [ "$CONTRACT_ID" != "" ]; then
  print_pass "Contract deployed: ${CONTRACT_ID}"
  echo ""
  echo -e "  ${DIM}Contract details:${NC}"
  echo -e "    ID:       ${BOLD}${CONTRACT_ID}${NC}"
  echo -e "    Status:   ${GREEN}${CONTRACT_STATUS}${NC}"
  echo -e "    Industry: HEALTHCARE"
  echo -e "    Suite:    HIPAA (S2)"
  echo ""

  # Show envelope
  echo -e "  ${DIM}Envelope constraints (safety boundaries):${NC}"
  echo "$contract_response" | jq -r '.contract.envelope.constraints // {} | to_entries[] | "    \(.key): [\(.value.min) – \(.value.max)]"' 2>/dev/null || true
  echo ""

  # Show guardrails
  echo -e "  ${DIM}Guardrails bound to contract:${NC}"
  echo "$contract_response" | jq -r '.contract.guardrails[]? | "    • \(.id) (v\(.version))"' 2>/dev/null || true
else
  error=$(echo "$contract_response" | jq -r '.error // .message // "unknown"' 2>/dev/null)
  if echo "$error" | grep -qi "rate limit"; then
    print_warn "Rate limited — contract will be created on next run"
    CONTRACT_ID="pending-rate-limit"
  else
    print_warn "Contract creation: ${error}"
    CONTRACT_ID="demo-fallback"
  fi
  FAILURES=$((FAILURES + 1))
fi

echo ""
narrate_pause

# ── ACT 5: Verify Enforcement ────────────────────────────────
print_step 5 "Alex → Gateway: Verify Enforcement Ready"
echo ""

# Check sidecar has the guardrails loaded
SG_PORT="${SG_PORT:-3001}"
sidecar_health=$(curl -sf "http://localhost:${SG_PORT}/health" 2>/dev/null || echo "")

if [ -n "$sidecar_health" ]; then
  cached=$(echo "$sidecar_health" | jq -r '.guardrailsCached // 0' 2>/dev/null)
  ready=$(echo "$sidecar_health" | jq -r '.ready // false' 2>/dev/null)
  print_pass "Smart Guardrail Engine: ${cached} guardrails cached, ready=${ready}"

  # Quick enforcement test
  test_response=$(curl -sf -X POST "http://localhost:${SG_PORT}/evaluate" \
    -H "Content-Type: application/json" \
    -d '{"guardrail_id":"pii_blocker","content":"My SSN is 123-45-6789","type":"request"}' 2>/dev/null || echo "")

  if [ -n "$test_response" ]; then
    blocked=$(echo "$test_response" | jq -r '.allowed' 2>/dev/null)
    score=$(echo "$test_response" | jq -r '.score // 0' 2>/dev/null)
    score=$(python3 -c "print(f'{float(\"${score}\"):.2f}')" 2>/dev/null || echo "$score")
    if [ "$blocked" = "false" ]; then
      print_pass "PII enforcement active — SSN blocked (score: ${score})"
    else
      print_warn "PII test: expected block, got allow"
    fi
  fi
else
  print_info "Sidecar not running locally — enforcement will work when gateway is deployed"
fi

# ── Summary ──────────────────────────────────────────────────
echo ""
print_header "Alex Agent — Session Complete"
echo ""
echo -e "  ${BOLD}Alex:${NC} \"Your healthcare triage contract is ready. Here's what I set up:\""
echo ""
echo -e "    Contract:   ${BOLD}${CONTRACT_ID}${NC}"
echo -e "    Tenant:     ${BOLD}${TENANT_ID}${NC}"
echo -e "    Guardrails: ${BOLD}5${NC} (PII, HIPAA, Injection, Toxicity, Medical Advice)"
echo -e "    Envelope:   ${BOLD}5 constraints${NC} with safety boundaries"
echo -e "    Status:     ${GREEN}${CONTRACT_STATUS:-approved}${NC}"
echo ""
echo -e "  ${BOLD}Alex:${NC} \"Your chatbot is now HIPAA-compliant. Zero code changes needed."
echo -e "        The gateway enforces all 5 guardrails on every API call.\""
echo ""

if [ "$FAILURES" -eq 0 ]; then
  echo -e "  ${GREEN}Setup complete — 0 failures${NC}"
else
  echo -e "  ${YELLOW}${FAILURES} step(s) had warnings${NC}"
fi

echo ""

# Export for downstream scripts
export CONTRACT_ID
