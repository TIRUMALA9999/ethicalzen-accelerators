#!/bin/bash
# ============================================================================
# EthicalZen Demo — Master Guardrail Test Runner
# Runs all guardrail type tests and generates a unified report
#
# Usage:
#   ./test-guardrails.sh                    # All types
#   ./test-guardrails.sh --type regex       # Just regex
#   ./test-guardrails.sh --type smart       # Just SSG v3
#   ./test-guardrails.sh --type composite   # Just DAG
#   ./test-guardrails.sh --narrate          # Presenter mode
# ============================================================================
# Note: intentionally no set -e — we track failures via counters and want all tests to run
set -uo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
source "${SCRIPT_DIR}/lib/colors.sh"
source "${SCRIPT_DIR}/lib/assert.sh"
source "${SCRIPT_DIR}/lib/health.sh"
source "${SCRIPT_DIR}/lib/report.sh"
parse_common_args "$@"

# Parse --type filter
FILTER_TYPE=""
for i in "$@"; do
  case "$i" in
    --type)  shift; FILTER_TYPE="${1:-}"; shift ;;
    --type=*) FILTER_TYPE="${i#*=}" ;;
  esac
done

HOST="${GATEWAY_HOST:-localhost}"
PORT="${SG_PORT:-3001}"

# ────────────────────────────────────────────────────────────
print_header "EthicalZen — Guardrail Test Suite"

narrate "This test suite exercises every guardrail type supported by the Smart Guardrail Engine.

  Regex guardrails     — Pattern matching (PII, injection, toxicity, leakage, token cost)
  Smart guardrails     — SSG v3 embeddings + lexical (medical, legal, financial)
  Hybrid guardrails    — Combined regex + semantic (HIPAA, content mod, embedding attack)
  Keyword guardrails   — Keyword-list matching (bias detection)
  DLM Kernel           — Diffusion Language Model (graceful skip if uncalibrated)
  Composite DAG        — AND/OR/NOT logic trees combining multiple guardrails

Each guardrail is tested with safe AND unsafe inputs to verify both blocking and allowing."

# ────────────────────────────────────────────────────────────
# Pre-flight checks
# ────────────────────────────────────────────────────────────
require_jq

print_info "Checking sidecar health at ${HOST}:${PORT}..."
if ! check_sidecar_health "$HOST" "$PORT"; then
  print_error "Smart Guardrail Engine not reachable at ${HOST}:${PORT}"
  print_info  "Start it with: ./deploy-vpc.sh --local"
  exit 1
fi
print_pass "Sidecar healthy"
print_info "Note: Latencies include HTTP round-trip overhead (~90ms). Raw guardrail evaluation is <5ms (regex) / <50ms (smart)."

# Report init
report_init "guardrail-test-suite"

# ────────────────────────────────────────────────────────────
# Run test scripts based on filter
# ────────────────────────────────────────────────────────────
ARGS=""
[ "$NARRATE" = "true" ] && ARGS="--narrate"

run_type_test() {
  local type="$1"
  local script="${SCRIPT_DIR}/guardrails/test-${type}.sh"

  if [ -n "$FILTER_TYPE" ] && [ "$FILTER_TYPE" != "$type" ]; then
    return 0
  fi

  if [ ! -f "$script" ]; then
    print_warn "Script not found: ${script}"
    return 0
  fi

  # Run in same shell to share counters and report state
  source "$script"
}

run_type_test "regex"
run_type_test "smart"
run_type_test "hybrid"
run_type_test "keyword"
run_type_test "dlm-kernel"
run_type_test "composite-dag"

# ────────────────────────────────────────────────────────────
# Summary & Report
# ────────────────────────────────────────────────────────────
print_summary
report_finalize "${SCRIPT_DIR}/reports"

if [ "$FAILED_TESTS" -gt 0 ]; then
  exit 1
fi
