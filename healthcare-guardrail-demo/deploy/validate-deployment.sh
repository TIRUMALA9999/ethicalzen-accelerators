#!/bin/bash
# ============================================================================
# EthicalZen Demo — Post-Deployment Validation
# Runs smoke tests to verify gateway + sidecar are working
# ============================================================================
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
source "${SCRIPT_DIR}/lib/colors.sh"
source "${SCRIPT_DIR}/lib/assert.sh"
source "${SCRIPT_DIR}/lib/health.sh"

HOST="${1:-${GATEWAY_HOST:-localhost}}"
GATEWAY_PORT="${2:-${GATEWAY_PORT:-8080}}"
SIDECAR_PORT="${3:-${SG_PORT:-3001}}"

print_header "Post-Deployment Validation"

VALIDATION_PASSED=true

# ── Check 1: Gateway Health ──────────────────────────────────
print_subheader "Gateway Health"
if check_gateway_health "$HOST" "$GATEWAY_PORT"; then
  print_pass "Gateway healthy at ${HOST}:${GATEWAY_PORT}"
else
  print_fail "Gateway not healthy at ${HOST}:${GATEWAY_PORT}"
  VALIDATION_PASSED=false
fi

# ── Check 2: Sidecar Health ──────────────────────────────────
print_subheader "Sidecar Health"
if check_sidecar_health "$HOST" "$SIDECAR_PORT"; then
  print_pass "Sidecar healthy at ${HOST}:${SIDECAR_PORT}"
else
  print_fail "Sidecar not healthy at ${HOST}:${SIDECAR_PORT}"
  VALIDATION_PASSED=false
fi

# ── Check 3: Guardrails Loaded ───────────────────────────────
print_subheader "Guardrails Loaded"
guardrails_response=$(curl -sf "http://${HOST}:${SIDECAR_PORT}/guardrails" 2>/dev/null || echo "")
if [ -n "$guardrails_response" ]; then
  count=$(echo "$guardrails_response" | jq 'length' 2>/dev/null || echo "0")
  if [ "$count" -gt 0 ]; then
    print_pass "${count} guardrails loaded"
    echo "$guardrails_response" | jq -r '.[].guardrail_id' 2>/dev/null | while read -r gid; do
      echo -e "    ${DIM}• ${gid}${NC}"
    done
  else
    print_warn "No guardrails loaded (engine may still be initializing)"
  fi
else
  print_fail "Could not fetch guardrails list"
  VALIDATION_PASSED=false
fi

# ── Check 4: Smoke Guardrail Evaluation ──────────────────────
print_subheader "Smoke Test — PII Blocker"
smoke_response=$(curl -sf -X POST "http://${HOST}:${SIDECAR_PORT}/evaluate" \
  -H "Content-Type: application/json" \
  -d '{"guardrail_id":"pii_blocker","payload":"My SSN is 123-45-6789"}' 2>/dev/null || echo "")

if [ -n "$smoke_response" ]; then
  allowed=$(echo "$smoke_response" | jq -r '.allowed // .decision' 2>/dev/null)
  if [ "$allowed" = "false" ] || [ "$allowed" = "blocked" ]; then
    print_pass "PII blocker correctly blocked SSN input"
  else
    print_warn "PII blocker returned unexpected: ${allowed}"
  fi
else
  print_fail "Guardrail evaluation failed"
  VALIDATION_PASSED=false
fi

# ── Check 5: Metrics Endpoint ────────────────────────────────
print_subheader "Metrics Endpoint"
metrics_response=$(curl -sf "http://${HOST}:${SIDECAR_PORT}/metrics" 2>/dev/null || echo "")
if echo "$metrics_response" | grep -q "sg_" 2>/dev/null; then
  print_pass "Sidecar metrics available"
elif [ -n "$metrics_response" ]; then
  print_pass "Metrics endpoint responding"
else
  print_warn "Metrics endpoint not available (non-critical)"
fi

# ── Summary ──────────────────────────────────────────────────
echo ""
if [ "$VALIDATION_PASSED" = true ]; then
  echo -e "  ${GREEN}${BOLD}VALIDATION PASSED — Deployment is ready${NC}"
  echo ""
  echo -e "  ${DIM}Next steps:${NC}"
  echo -e "    ${DIM}• Run guardrail tests: ./test-guardrails.sh${NC}"
  echo -e "    ${DIM}• Run healthcare demo: ./narrative/healthcare-triage.sh${NC}"
  echo -e "    ${DIM}• Run full E2E:        ./e2e-full.sh --mode offline${NC}"
  exit 0
else
  echo -e "  ${RED}${BOLD}VALIDATION FAILED — Check logs above${NC}"
  echo ""
  echo -e "  ${DIM}Troubleshooting:${NC}"
  echo -e "    ${DIM}• Check container logs: docker logs ethicalzen-gateway-demo${NC}"
  echo -e "    ${DIM}• Wait for cold start (up to 90s for embedding model)${NC}"
  echo -e "    ${DIM}• Verify ports: netstat -tlnp | grep -E '8080|3001'${NC}"
  exit 1
fi
