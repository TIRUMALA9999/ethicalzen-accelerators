#!/bin/bash
# ============================================================================
# EthicalZen Demo — Report Generator (JSON + Markdown)
# ============================================================================

# Use default-if-unset to preserve values when re-sourced by sub-scripts
if ! declare -p REPORT_RESULTS &>/dev/null; then
  REPORT_RESULTS=()
fi
: ${REPORT_NAME:=""}
: ${REPORT_START_TIME:=""}

report_init() {
  REPORT_NAME="$1"
  REPORT_START_TIME=$(date -u +"%Y-%m-%dT%H:%M:%SZ")
  REPORT_RESULTS=()
}

# Add a test result: report_add guardrail_id type status score latency_ms description
report_add() {
  local guardrail_id="$1"
  local type="$2"
  local status="$3"
  local score="$4"
  local latency_ms="$5"
  local description="$6"
  REPORT_RESULTS+=("{\"guardrail_id\":\"${guardrail_id}\",\"type\":\"${type}\",\"status\":\"${status}\",\"score\":${score:-0},\"latency_ms\":${latency_ms:-0},\"description\":\"${description}\"}")
}

report_finalize() {
  local output_dir="${1:-$(dirname "$0")/reports}"
  local timestamp=$(date +"%Y%m%d_%H%M%S")
  local json_file="${output_dir}/${REPORT_NAME}-${timestamp}.json"
  local md_file="${output_dir}/${REPORT_NAME}-${timestamp}.md"

  mkdir -p "$output_dir"

  # Build JSON array
  local results_json="["
  local first=true
  for r in "${REPORT_RESULTS[@]}"; do
    if [ "$first" = true ]; then
      first=false
    else
      results_json+=","
    fi
    results_json+="$r"
  done
  results_json+="]"

  # Write JSON report
  cat > "$json_file" << JSONEOF
{
  "report": "${REPORT_NAME}",
  "generated_at": "$(date -u +"%Y-%m-%dT%H:%M:%SZ")",
  "started_at": "${REPORT_START_TIME}",
  "total_tests": ${TOTAL_TESTS},
  "passed": ${PASSED_TESTS},
  "failed": ${FAILED_TESTS},
  "skipped": ${SKIPPED_TESTS},
  "results": ${results_json}
}
JSONEOF

  # Write Markdown report
  cat > "$md_file" << MDEOF
# ${REPORT_NAME} Report

**Generated:** $(date -u +"%Y-%m-%d %H:%M UTC")

## Summary

| Metric | Value |
|--------|-------|
| Total Tests | ${TOTAL_TESTS} |
| Passed | ${PASSED_TESTS} |
| Failed | ${FAILED_TESTS} |
| Skipped | ${SKIPPED_TESTS} |
| Pass Rate | $(( TOTAL_TESTS > 0 ? (PASSED_TESTS * 100 / TOTAL_TESTS) : 0 ))% |

## Results

| Guardrail | Type | Status | Score | Latency | Description |
|-----------|------|--------|-------|---------|-------------|
MDEOF

  echo "$results_json" | jq -r '.[] | "| \(.guardrail_id) | \(.type) | \(.status) | \(.score) | \(.latency_ms)ms | \(.description) |"' >> "$md_file" 2>/dev/null

  print_info "JSON report: ${json_file}"
  print_info "Markdown report: ${md_file}"
}
