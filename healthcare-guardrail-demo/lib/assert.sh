#!/bin/bash
# ============================================================================
# EthicalZen Demo — Test Assertion Library
# Requires: colors.sh sourced first, jq installed
# ============================================================================

# Round score to 2 decimal places (avoids IEEE-754 floating-point display artifacts)
round_score() { python3 -c "import sys; print(f'{float(sys.argv[1]):.2f}')" "$1" 2>/dev/null || echo "$1"; }

# Assert that a guardrail evaluation returned allowed: false (blocked)
# Note: jq '//' treats false as falsy, so we use 'if .allowed then "true" else "false" end'
assert_blocked() {
  local response="$1"
  local test_name="$2"
  local allowed=$(echo "$response" | jq -r 'if .allowed == true then "true" elif .allowed == false then "false" elif .valid == true then "true" elif .valid == false then "false" else "unknown" end' 2>/dev/null)
  if [ "$allowed" = "false" ]; then
    local raw_score=$(echo "$response" | jq -r '.score // 0' 2>/dev/null)
    local score=$(round_score "$raw_score")
    print_pass "${test_name} (score: ${score})"
    return 0
  else
    print_fail "${test_name} — expected blocked, got: $(echo "$response" | head -c 200)"
    return 1
  fi
}

# Assert that a guardrail evaluation returned allowed: true (allowed)
assert_allowed() {
  local response="$1"
  local test_name="$2"
  local allowed=$(echo "$response" | jq -r 'if .allowed == true then "true" elif .allowed == false then "false" elif .valid == true then "true" elif .valid == false then "false" else "unknown" end' 2>/dev/null)
  if [ "$allowed" = "true" ]; then
    local raw_score=$(echo "$response" | jq -r '.score // 0' 2>/dev/null)
    local score=$(round_score "$raw_score")
    print_pass "${test_name} (score: ${score})"
    return 0
  else
    print_fail "${test_name} — expected allowed, got: $(echo "$response" | head -c 200)"
    return 1
  fi
}

# Assert HTTP status code
assert_http_status() {
  local actual="$1"
  local expected="$2"
  local test_name="$3"
  if [ "$actual" = "$expected" ]; then
    print_pass "${test_name} (HTTP ${actual})"
    return 0
  else
    print_fail "${test_name} — expected HTTP ${expected}, got HTTP ${actual}"
    return 1
  fi
}

# Assert a JSON field contains expected value
assert_json_contains() {
  local response="$1"
  local field="$2"
  local expected="$3"
  local test_name="$4"
  local actual=$(echo "$response" | jq -r ".${field} // empty" 2>/dev/null)
  if echo "$actual" | grep -qi "$expected"; then
    print_pass "${test_name}"
    return 0
  else
    print_fail "${test_name} — .${field} expected to contain '${expected}', got '${actual}'"
    return 1
  fi
}

# Evaluate a guardrail and return the JSON response
evaluate_guardrail() {
  local host="${GATEWAY_HOST:-localhost}"
  local port="${SG_PORT:-3001}"
  local guardrail_id="$1"
  local payload="$2"
  curl -sf -X POST "http://${host}:${port}/evaluate" \
    -H "Content-Type: application/json" \
    -d "{\"guardrail_id\":\"${guardrail_id}\",\"payload\":$(echo "$payload" | jq -Rs .)}" 2>&1
}

# Evaluate a composite DAG and return the JSON response
evaluate_composite() {
  local host="${GATEWAY_HOST:-localhost}"
  local port="${SG_PORT:-3001}"
  local dag_json="$1"
  local input="$2"
  curl -sf -X POST "http://${host}:${port}/evaluate-composite" \
    -H "Content-Type: application/json" \
    -d "{\"dag\":${dag_json},\"input\":$(echo "$input" | jq -Rs .)}" 2>&1
}
