#!/bin/bash
# ============================================================================
# EthicalZen Demo — Keyword Guardrail Tests
# Tests: bias_detector
# ============================================================================
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
source "${SCRIPT_DIR}/lib/colors.sh"
source "${SCRIPT_DIR}/lib/assert.sh"
source "${SCRIPT_DIR}/lib/report.sh"
parse_common_args "$@"

HOST="${GATEWAY_HOST:-localhost}"
PORT="${SG_PORT:-3001}"
DATA_FILE="${SCRIPT_DIR}/guardrails/test-data.json"

print_header "Keyword Guardrails (Weighted Keyword Matching)"

narrate "Keyword guardrails use curated word lists with configurable weights.
The bias_detector checks for discriminatory language patterns including
gender, racial, age, disability, and religious bias indicators."

for guardrail_id in bias_detector; do
  print_subheader "${guardrail_id}"

  test_count=$(jq -r ".keyword.${guardrail_id} | length" "$DATA_FILE")
  for i in $(seq 0 $((test_count - 1))); do
    name=$(jq -r ".keyword.${guardrail_id}[$i].name" "$DATA_FILE")
    input=$(jq -r ".keyword.${guardrail_id}[$i].input" "$DATA_FILE")
    expect_blocked=$(jq -r ".keyword.${guardrail_id}[$i].expect_blocked" "$DATA_FILE")

    start_ms=$(millis)
    response=$(evaluate_guardrail "$guardrail_id" "$input")
    end_ms=$(millis)
    latency=$((end_ms - start_ms))

    score=$(echo "$response" | jq -r '.score // 0' 2>/dev/null)

    if [ "$expect_blocked" = "true" ]; then
      assert_blocked "$response" "${guardrail_id}: ${name} [${latency}ms]"
    else
      assert_allowed "$response" "${guardrail_id}: ${name} [${latency}ms]"
    fi

    status=$( [ "$?" -eq 0 ] && echo "PASS" || echo "FAIL" )
    report_add "$guardrail_id" "keyword" "$status" "$score" "$latency" "$name"
  done
done
