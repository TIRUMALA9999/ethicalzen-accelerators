#!/bin/bash
# ============================================================================
# EthicalZen Demo — Health Check & Wait Functions
# ============================================================================

# Wait for a service to become healthy
# Usage: wait_for_health "http://localhost:8080/health" 120
wait_for_health() {
  local url="$1"
  local timeout_secs="${2:-120}"
  local start=$(date +%s)

  print_info "Waiting for ${url} to become healthy (timeout: ${timeout_secs}s)..."

  while true; do
    local elapsed=$(( $(date +%s) - start ))
    if [ "$elapsed" -ge "$timeout_secs" ]; then
      print_fail "Service did not become healthy within ${timeout_secs}s"
      return 1
    fi

    if curl -sf "$url" > /dev/null 2>&1; then
      print_pass "Service healthy after ${elapsed}s"
      return 0
    fi

    printf "  ${DIM}  Waiting... (%ds/%ds)${NC}\r" "$elapsed" "$timeout_secs"
    sleep 2
  done
}

# Check if gateway is healthy
check_gateway_health() {
  local host="${GATEWAY_HOST:-localhost}"
  local port="${GATEWAY_PORT:-8080}"
  local response=$(curl -sf "http://${host}:${port}/health" 2>&1)
  if echo "$response" | jq -e '.status == "healthy"' > /dev/null 2>&1; then
    return 0
  fi
  return 1
}

# Check if sidecar is healthy
check_sidecar_health() {
  local host="${GATEWAY_HOST:-localhost}"
  local port="${SG_PORT:-3001}"
  local response=$(curl -sf "http://${host}:${port}/health" 2>&1)
  if echo "$response" | jq -e '.status == "healthy"' > /dev/null 2>&1; then
    return 0
  fi
  return 1
}

# Check if jq is installed
require_jq() {
  if ! command -v jq &> /dev/null; then
    print_error "jq is required but not installed. Install with: brew install jq (macOS) or apt-get install jq (Linux)"
    exit 1
  fi
}

# Check if Docker is available
require_docker() {
  if ! command -v docker &> /dev/null; then
    print_error "Docker is required but not installed. See: https://docs.docker.com/get-docker/"
    exit 1
  fi
}
