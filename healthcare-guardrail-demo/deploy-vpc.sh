#!/bin/bash
# ============================================================================
# EthicalZen Demo — One-Click VPC Deployment
#
# Deploys the EthicalZen Gateway + Smart Guardrail Engine to a customer VPC.
# Single Docker container with Go gateway + Node.js sidecar via supervisord.
#
# Usage:
#   ./deploy-vpc.sh --local                          # Docker on localhost
#   ./deploy-vpc.sh --host 34.56.96.14               # Existing VM via SSH
#   ./deploy-vpc.sh --cloud gcp                      # Provision new GCE VM
#   ./deploy-vpc.sh --cloud gcp --region us-east1    # Custom region
#   ./deploy-vpc.sh --cleanup                        # Delete GCE resources
#
# Prerequisites:
#   --local:  Docker installed
#   --host:   SSH access to target VM with Docker
#   --cloud:  gcloud CLI authenticated
# ============================================================================
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
source "${SCRIPT_DIR}/lib/colors.sh"
source "${SCRIPT_DIR}/lib/health.sh"

# Load .env if present
[ -f "${SCRIPT_DIR}/.env" ] && source "${SCRIPT_DIR}/.env"

# Defaults
MODE=""
CLEANUP=false
EXTRA_ARGS=()

# Parse arguments
while [[ $# -gt 0 ]]; do
  case "$1" in
    --local)    MODE="local"; shift ;;
    --host)     MODE="host"; shift; EXTRA_ARGS+=("--host" "${1:-}"); shift ;;
    --host=*)   MODE="host"; EXTRA_ARGS+=("--host" "${1#*=}"); shift ;;
    --cloud)    MODE="cloud"; shift; EXTRA_ARGS+=("${1:-gcp}"); shift ;;
    --cleanup)  CLEANUP=true; shift ;;
    *)          EXTRA_ARGS+=("$1"); shift ;;
  esac
done

# ── Cleanup ──────────────────────────────────────────────────
if [ "$CLEANUP" = true ]; then
  print_header "Cleaning Up Demo Deployment"

  # Stop local Docker
  if docker ps --format '{{.Names}}' 2>/dev/null | grep -q "ethicalzen-gateway-demo"; then
    print_info "Stopping local container..."
    docker compose -f "${SCRIPT_DIR}/deploy/docker-compose.demo.yml" down 2>/dev/null || \
    docker stop ethicalzen-gateway-demo 2>/dev/null || true
    docker rm ethicalzen-gateway-demo 2>/dev/null || true
    print_pass "Local container stopped"
  fi

  # GCE cleanup
  if command -v gcloud &>/dev/null; then
    "${SCRIPT_DIR}/deploy/gcp-vm.sh" --cleanup 2>/dev/null || true
  fi

  print_pass "Cleanup complete"
  exit 0
fi

# ── Mode Selection ───────────────────────────────────────────
if [ -z "$MODE" ]; then
  echo ""
  echo -e "${BOLD}EthicalZen — VPC Deployment${NC}"
  echo ""
  echo "  Select deployment mode:"
  echo ""
  echo "    --local              Docker on this machine"
  echo "    --host <IP>          Deploy to existing VM via SSH"
  echo "    --cloud gcp          Provision new GCE VM"
  echo "    --cleanup            Remove demo resources"
  echo ""
  echo "  Example: ./deploy-vpc.sh --local"
  echo ""
  exit 1
fi

# ── Deploy ───────────────────────────────────────────────────
print_header "EthicalZen — Customer VPC Deployment"
echo -e "  Mode: ${BOLD}${MODE}${NC}"
echo ""

case "$MODE" in
  local)
    print_step 1 "Deploying with Docker Compose (local)"
    require_docker

    # Pull latest
    print_info "Pulling latest gateway image..."
    docker compose -f "${SCRIPT_DIR}/deploy/docker-compose.demo.yml" pull 2>/dev/null || \
      docker pull ethicalzen/acvps-gateway:latest

    # Start
    print_info "Starting gateway..."
    docker compose -f "${SCRIPT_DIR}/deploy/docker-compose.demo.yml" up -d

    print_step 2 "Waiting for health (up to 120s)"
    HOST="${GATEWAY_HOST:-localhost}"
    if wait_for_health "http://${HOST}:${GATEWAY_PORT:-8080}/health" 120; then
      print_pass "Gateway is healthy!"
    else
      print_warn "Gateway not yet healthy. Checking container..."
      docker logs ethicalzen-gateway-demo --tail 20 2>/dev/null || true
    fi

    print_step 3 "Validating deployment"
    "${SCRIPT_DIR}/deploy/validate-deployment.sh" "${HOST}" "${GATEWAY_PORT:-8080}" "${SG_PORT:-3001}"

    print_header "Local Deployment Complete"
    echo -e "  ${BOLD}Gateway:${NC}  http://${HOST}:${GATEWAY_PORT:-8080}"
    echo -e "  ${BOLD}Sidecar:${NC}  http://${HOST}:${SG_PORT:-3001}"
    echo -e "  ${BOLD}Metrics:${NC}  http://${HOST}:${METRICS_PORT:-9090}/metrics"
    echo ""
    echo -e "  ${DIM}Run tests:    ./test-guardrails.sh${NC}"
    echo -e "  ${DIM}Run demo:     ./narrative/healthcare-triage.sh${NC}"
    echo -e "  ${DIM}Cleanup:      ./deploy-vpc.sh --cleanup${NC}"
    ;;

  host)
    "${SCRIPT_DIR}/deploy/existing-host.sh" "${EXTRA_ARGS[@]}"
    ;;

  cloud)
    "${SCRIPT_DIR}/deploy/gcp-vm.sh" "${EXTRA_ARGS[@]}"
    ;;

  *)
    print_error "Unknown mode: ${MODE}"
    exit 1
    ;;
esac
