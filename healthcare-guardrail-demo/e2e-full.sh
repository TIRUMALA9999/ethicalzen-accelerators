#!/bin/bash
# ============================================================================
# EthicalZen Demo — Complete End-to-End Flow
#
# Runs the full demo: deploy → test guardrails → healthcare narrative →
# proxy (if OpenAI key) → generate compliance report
#
# Usage:
#   ./e2e-full.sh --mode offline    # Sidecar-only (no backend/OpenAI)
#   ./e2e-full.sh --mode full       # Backend + OpenAI proxy
#   ./e2e-full.sh --narrate         # Presenter mode with pauses
# ============================================================================
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
source "${SCRIPT_DIR}/lib/colors.sh"
source "${SCRIPT_DIR}/lib/health.sh"
parse_common_args "$@"

# Load .env
[ -f "${SCRIPT_DIR}/.env" ] && source "${SCRIPT_DIR}/.env"

# Parse mode
MODE="offline"
DEPLOY_LOCAL=false
for arg in "$@"; do
  case "$arg" in
    --mode)         shift; MODE="${1:-offline}" ;;
    --mode=*)       MODE="${arg#*=}" ;;
    --deploy-local) DEPLOY_LOCAL=true ;;
  esac
done

HOST="${GATEWAY_HOST:-localhost}"
SG_PORT="${SG_PORT:-3001}"
GW_PORT="${GATEWAY_PORT:-8080}"
ARGS=""
[ "$NARRATE" = "true" ] && ARGS="--narrate"

print_header "EthicalZen — Complete E2E Demo"

echo -e "  Mode:    ${BOLD}${MODE}${NC}"
echo -e "  Host:    ${BOLD}${HOST}${NC}"
echo -e "  Gateway: ${BOLD}:${GW_PORT}${NC}  Sidecar: ${BOLD}:${SG_PORT}${NC}"
echo ""

START_TIME=$(date +%s)

# ── Step 1: Environment Check ────────────────────────────────
print_step 1 "Environment Check"
require_jq

OPENAI_KEY="${OPENAI_API_KEY:-}"
BACKEND_KEY="${ETHICALZEN_API_KEY:-}"

print_info "jq: available"
if command -v docker &>/dev/null; then
  print_info "Docker: available"
else
  print_warn "Docker: not found (skip deploy)"
fi

if [ -n "$OPENAI_KEY" ]; then
  print_info "OpenAI API Key: set"
else
  print_info "OpenAI API Key: not set (proxy tests will mock)"
fi

if [ -n "$BACKEND_KEY" ]; then
  print_info "EthicalZen API Key: set"
else
  print_info "EthicalZen API Key: not set (contract setup will skip)"
fi

# ── Step 2: Deploy (if needed) ───────────────────────────────
print_step 2 "Deploy Gateway"

if check_gateway_health "$HOST" "$GW_PORT" 2>/dev/null && check_sidecar_health "$HOST" "$SG_PORT" 2>/dev/null; then
  print_pass "Gateway already running and healthy"
elif [ "$DEPLOY_LOCAL" = true ]; then
  print_info "Deploying locally..."
  "${SCRIPT_DIR}/deploy-vpc.sh" --local
else
  print_info "Gateway not running. Waiting up to 120s..."
  if wait_for_health "http://${HOST}:${GW_PORT}/health" 120; then
    print_pass "Gateway became healthy"
  else
    print_error "Gateway not available at ${HOST}:${GW_PORT}"
    print_info "Deploy first: ./deploy-vpc.sh --local"
    print_info "Or add --deploy-local flag to auto-deploy"
    exit 1
  fi
fi

# Verify sidecar
if ! check_sidecar_health "$HOST" "$SG_PORT"; then
  print_warn "Sidecar not ready. Waiting 30s for embedding model warmup..."
  wait_for_health "http://${HOST}:${SG_PORT}/health" 30 || true
fi

# ── Step 3: Guardrail Tests ──────────────────────────────────
print_step 3 "Running All Guardrail Tests"
"${SCRIPT_DIR}/test-guardrails.sh" $ARGS || true

# ── Step 4: Healthcare Narrative ─────────────────────────────
print_step 4 "Healthcare Triage Narrative"
"${SCRIPT_DIR}/narrative/healthcare-triage.sh" $ARGS || true

# ── Step 5: Contract Setup via Alex Agent (full mode) ────────
if [ "$MODE" = "full" ] && [ -n "$BACKEND_KEY" ]; then
  print_step 5 "Alex Agent — Guardrail Design & Contract Setup"
  source "${SCRIPT_DIR}/e2e/setup-via-alex.sh" || true
else
  print_step 5 "Contract Setup (skipped — offline mode or no API key)"
  print_info "Use --mode full with ETHICALZEN_API_KEY for contract setup"
fi

# ── Step 6: OpenAI Proxy Test ────────────────────────────────
if [ "$MODE" = "full" ] && [ -n "$OPENAI_KEY" ]; then
  print_step 6 "OpenAI Proxy Test (live)"
  "${SCRIPT_DIR}/e2e/proxy-openai.sh" $ARGS || true
else
  print_step 6 "OpenAI Proxy Test (mock mode)"
  "${SCRIPT_DIR}/e2e/proxy-openai.sh" --mock $ARGS || true
fi

# ── Step 7: Compliance Report ────────────────────────────────
print_step 7 "Generating Compliance Evidence Report"
"${SCRIPT_DIR}/e2e/generate-report.sh"

# ── Final Summary ────────────────────────────────────────────
END_TIME=$(date +%s)
DURATION=$((END_TIME - START_TIME))

print_header "E2E Demo Complete"

echo -e "  Duration:  ${BOLD}${DURATION}s${NC}"
echo -e "  Mode:      ${BOLD}${MODE}${NC}"
echo -e "  Gateway:   ${BOLD}http://${HOST}:${GW_PORT}${NC}"
echo -e "  Sidecar:   ${BOLD}http://${HOST}:${SG_PORT}${NC}"
echo ""
echo -e "  ${BOLD}Reports:${NC}"
ls -1 "${SCRIPT_DIR}/reports/"*.md 2>/dev/null | while read -r f; do
  echo -e "    ${DIM}$(basename "$f")${NC}"
done
echo ""
echo -e "  ${GREEN}${BOLD}Demo complete. Reports saved to demo/reports/${NC}"
echo ""
