#!/bin/bash
# ============================================================================
# EthicalZen Demo — Regex Guardrail Tests
# Tests: pii_blocker, prompt_injection_blocker, toxicity_detector,
#        system_prompt_leakage_detector, token_cost_limiter
# ============================================================================
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
source "${SCRIPT_DIR}/lib/colors.sh"
source "${SCRIPT_DIR}/lib/assert.sh"
source "${SCRIPT_DIR}/lib/report.sh"
parse_common_args "$@"

HOST="${GATEWAY_HOST:-localhost}"
PORT="${SG_PORT:-3001}"
DATA_FILE="${SCRIPT_DIR}/guardrails/test-data.json"

print_header "Regex Guardrails (Pattern-Based, <5ms Latency)"

narrate "Regex guardrails use compiled patterns to detect PII, prompt injections,
toxic content, system prompt leakage, and token cost attacks.
They are the fastest guardrail type — typically under 5ms per evaluation."

print_info "Latencies shown include HTTP round-trip. Raw evaluation: <5ms for regex, <50ms for smart guardrails."

# Test each regex guardrail from test-data.json
for guardrail_id in pii_blocker prompt_injection_blocker toxicity_detector system_prompt_leakage_detector token_cost_limiter; do
  print_subheader "${guardrail_id}"

  test_count=$(jq -r ".regex.${guardrail_id} | length" "$DATA_FILE")
  for i in $(seq 0 $((test_count - 1))); do
    name=$(jq -r ".regex.${guardrail_id}[$i].name" "$DATA_FILE")
    input=$(jq -r ".regex.${guardrail_id}[$i].input" "$DATA_FILE")
    expect_blocked=$(jq -r ".regex.${guardrail_id}[$i].expect_blocked" "$DATA_FILE")

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
    report_add "$guardrail_id" "regex" "$status" "$score" "$latency" "$name"
  done
done
