#!/bin/bash
# ============================================================================
# EthicalZen Demo — Smart Guardrail Tests (SSG v3: Embedding + Lexical)
# Tests: medical_advice_smart, legal_advice_smart, financial_advice_smart
# These use all-MiniLM-L6-v2 (384-dim) embeddings + lexical pattern matching
# ============================================================================
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
source "${SCRIPT_DIR}/lib/colors.sh"
source "${SCRIPT_DIR}/lib/assert.sh"
source "${SCRIPT_DIR}/lib/report.sh"
parse_common_args "$@"

HOST="${GATEWAY_HOST:-localhost}"
PORT="${SG_PORT:-3001}"
DATA_FILE="${SCRIPT_DIR}/guardrails/test-data.json"

print_header "Smart Guardrails (SSG v3: Semantic Similarity + Lexical)"

narrate "Smart Guardrails use the SSG v3 algorithm:
  1. Embed the input using all-MiniLM-L6-v2 (384 dimensions)
  2. Compare cosine similarity to safe/unsafe centroids
  3. Extract lexical features (keyword patterns with weights)
  4. Fuse embedding score + lexical score with configurable weights
  5. Apply 3-zone decision: ALLOW (< tAllow) | REVIEW | BLOCK (> tBlock)

First evaluation triggers model loading (~10-30s warmup)."

# Warm up the embedding model
print_info "Warming up embedding model (first call loads MiniLM-L6-v2)..."
warmup=$(curl -sf -X POST "http://${HOST}:${PORT}/evaluate" \
  -H "Content-Type: application/json" \
  -d '{"guardrail_id":"medical_advice_smart","payload":"hello"}' 2>&1)
if echo "$warmup" | jq -e '.score' > /dev/null 2>&1; then
  print_info "Embedding model loaded successfully"
else
  print_warn "Model warmup returned: $(echo "$warmup" | head -c 200)"
  print_info "Smart guardrails may use lexical-only fallback"
fi

# Test each smart guardrail
for guardrail_id in medical_advice_smart legal_advice_smart financial_advice_smart; do
  print_subheader "${guardrail_id}"

  test_count=$(jq -r ".smart.${guardrail_id} | length" "$DATA_FILE")
  for i in $(seq 0 $((test_count - 1))); do
    name=$(jq -r ".smart.${guardrail_id}[$i].name" "$DATA_FILE")
    input=$(jq -r ".smart.${guardrail_id}[$i].input" "$DATA_FILE")
    expect_blocked=$(jq -r ".smart.${guardrail_id}[$i].expect_blocked" "$DATA_FILE")

    start_ms=$(millis)
    response=$(evaluate_guardrail "$guardrail_id" "$input")
    end_ms=$(millis)
    latency=$((end_ms - start_ms))

    raw_score=$(echo "$response" | jq -r '.score // 0' 2>/dev/null)
    score=$(round_score "$raw_score")
    raw_eval_type=$(echo "$response" | jq -r '.evaluation_type // "unknown"' 2>/dev/null)
    eval_type="${raw_eval_type/code/smart}"
    zone=$(echo "$response" | jq -r '.zone // .decision // "unknown"' 2>/dev/null)

    if [ "$expect_blocked" = "true" ]; then
      assert_blocked "$response" "${guardrail_id}: ${name} [${latency}ms, ${eval_type}, zone=${zone}]"
    else
      assert_allowed "$response" "${guardrail_id}: ${name} [${latency}ms, ${eval_type}, zone=${zone}]"
    fi

    status=$( [ "$?" -eq 0 ] && echo "PASS" || echo "FAIL" )
    report_add "$guardrail_id" "smart_guardrail" "$status" "$score" "$latency" "$name"
  done
done
