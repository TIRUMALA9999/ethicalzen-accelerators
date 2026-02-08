#!/bin/bash
# ============================================================================
# EthicalZen Demo — Composite DAG Guardrail Tests
# Tests AND, OR, NOT operators and nested DAG trees
# ============================================================================
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
source "${SCRIPT_DIR}/lib/colors.sh"
source "${SCRIPT_DIR}/lib/assert.sh"
source "${SCRIPT_DIR}/lib/report.sh"
parse_common_args "$@"

HOST="${GATEWAY_HOST:-localhost}"
PORT="${SG_PORT:-3001}"
DATA_FILE="${SCRIPT_DIR}/guardrails/test-data.json"

print_header "Composite DAG Guardrails (AND / OR / NOT Logic Trees)"

narrate "Composite guardrails combine multiple guardrails into a Directed Acyclic Graph (DAG).

  AND: ALL child guardrails must allow (used for compliance: every check must pass)
  OR:  ANY child guardrail allowing is sufficient (used for redundancy)
  NOT: Inverts the child guardrail's decision (used for exclusion rules)

These can be nested to create complex policy trees, e.g.:
  AND(pii_blocker, hipaa_compliance, OR(injection_blocker, toxicity_detector))"

test_count=$(jq -r '.composite | length' "$DATA_FILE")
for i in $(seq 0 $((test_count - 1))); do
  name=$(jq -r ".composite[$i].name" "$DATA_FILE")
  dag=$(jq -c ".composite[$i].dag" "$DATA_FILE")
  input=$(jq -r ".composite[$i].input" "$DATA_FILE")
  expect_blocked=$(jq -r ".composite[$i].expect_blocked" "$DATA_FILE")
  reason=$(jq -r ".composite[$i].reason" "$DATA_FILE")

  print_subheader "${name}"
  echo "  DAG: $(echo "$dag" | jq -c .)"
  echo "  Input: \"$(echo "$input" | head -c 80)...\""
  echo "  Reason: ${reason}"
  echo ""

  start_ms=$(millis)
  response=$(evaluate_composite "$dag" "$input")
  end_ms=$(millis)
  latency=$((end_ms - start_ms))

  score=$(echo "$response" | jq -r '.score // 0' 2>/dev/null)
  guardrails_eval=$(echo "$response" | jq -r '.guardrails_evaluated // 0' 2>/dev/null)

  if [ "$expect_blocked" = "true" ]; then
    assert_blocked "$response" "Composite: ${name} [${latency}ms, ${guardrails_eval} guardrails evaluated]"
  else
    assert_allowed "$response" "Composite: ${name} [${latency}ms, ${guardrails_eval} guardrails evaluated]"
  fi

  status=$( [ "$?" -eq 0 ] && echo "PASS" || echo "FAIL" )
  report_add "composite_dag" "composite" "$status" "$score" "$latency" "$name"
done
