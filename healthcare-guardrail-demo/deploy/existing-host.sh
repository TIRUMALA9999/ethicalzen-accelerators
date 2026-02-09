#!/bin/bash
# ============================================================================
# EthicalZen Demo — Deploy to Existing VM via SSH
# Deploys the gateway container to a pre-existing VM
#
# Usage:
#   ./deploy/existing-host.sh                    # Uses VM_HOST from .env
#   ./deploy/existing-host.sh --host 34.56.96.14 # Explicit host
# ============================================================================
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
source "${SCRIPT_DIR}/lib/colors.sh"
source "${SCRIPT_DIR}/lib/health.sh"

# Load .env if present
[ -f "${SCRIPT_DIR}/.env" ] && source "${SCRIPT_DIR}/.env"

# Configuration
TARGET_HOST="${VM_HOST:-}"
SSH_USER="${SSH_USER:-$(whoami)}"
SSH_KEY="${SSH_KEY_PATH:-~/.ssh/id_rsa}"
IMAGE="us-docker.pkg.dev/ethicalzen-public-04085/ethicalzen-public/ethicalzen-gateway:latest"
CONTAINER_NAME="ethicalzen-gateway"

# Parse arguments
for arg in "$@"; do
  case "$arg" in
    --host)     shift; TARGET_HOST="${1:-}" ;;
    --host=*)   TARGET_HOST="${arg#*=}" ;;
    --user)     shift; SSH_USER="${1:-}" ;;
    --user=*)   SSH_USER="${arg#*=}" ;;
    --key)      shift; SSH_KEY="${1:-}" ;;
    --key=*)    SSH_KEY="${arg#*=}" ;;
  esac
done

if [ -z "$TARGET_HOST" ]; then
  print_error "No host specified. Use --host IP or set VM_HOST in .env"
  exit 1
fi

print_header "Deploying to Existing Host: ${TARGET_HOST}"

echo -e "  Host:      ${BOLD}${TARGET_HOST}${NC}"
echo -e "  SSH User:  ${BOLD}${SSH_USER}${NC}"
echo -e "  SSH Key:   ${BOLD}${SSH_KEY}${NC}"
echo -e "  Image:     ${BOLD}${IMAGE}${NC}"
echo ""

SSH_CMD="ssh -o StrictHostKeyChecking=no -o ConnectTimeout=10 -i ${SSH_KEY} ${SSH_USER}@${TARGET_HOST}"

# ── Step 1: Verify SSH Access ────────────────────────────────
print_step 1 "Verifying SSH access"
if $SSH_CMD "echo ok" &>/dev/null; then
  print_pass "SSH connection successful"
else
  print_error "Cannot SSH to ${SSH_USER}@${TARGET_HOST}"
  print_info "Verify: ssh -i ${SSH_KEY} ${SSH_USER}@${TARGET_HOST}"
  exit 1
fi

# ── Step 2: Check Docker ─────────────────────────────────────
print_step 2 "Checking Docker on remote host"
if $SSH_CMD "docker --version" &>/dev/null; then
  print_pass "Docker available"
else
  print_error "Docker not installed on ${TARGET_HOST}"
  print_info "Install: curl -fsSL https://get.docker.com | sh"
  exit 1
fi

# ── Step 3: Pull Image ───────────────────────────────────────
print_step 3 "Pulling latest gateway image"
$SSH_CMD "docker pull ${IMAGE}" 2>&1 | tail -1
print_pass "Image pulled"

# ── Step 4: Stop Existing Container ──────────────────────────
print_step 4 "Stopping existing container (if any)"
$SSH_CMD "docker stop ${CONTAINER_NAME} 2>/dev/null; docker rm ${CONTAINER_NAME} 2>/dev/null" || true
print_info "Cleared existing container"

# ── Step 5: Start Container ──────────────────────────────────
print_step 5 "Starting gateway container"
$SSH_CMD "docker run -d --name ${CONTAINER_NAME} \
  -p 8080:8080 -p 3001:3001 -p 9090:9090 \
  -e SMART_GUARDRAIL_URL=http://localhost:3001 \
  -e SG_ENGINE_URL=http://localhost:3001 \
  -e GUARDRAILS_PATH=/app/guardrails \
  -e PORTAL_BACKEND_URL=${ETHICALZEN_BACKEND_URL:-https://api.ethicalzen.ai} \
  -e ETHICALZEN_API_KEY=${ETHICALZEN_API_KEY:-} \
  -e NODE_ENV=production \
  ${IMAGE}"
print_pass "Container started"

# ── Step 6: Wait for Health ──────────────────────────────────
print_step 6 "Waiting for gateway health (up to 120s)"
if wait_for_health "http://${TARGET_HOST}:8080/health" 120; then
  print_pass "Gateway is healthy!"
else
  print_warn "Gateway not yet healthy. Cold start may take longer."
  print_info "Check: curl http://${TARGET_HOST}:8080/health"
fi

# ── Step 7: Validate ─────────────────────────────────────────
print_step 7 "Running validation"
"${SCRIPT_DIR}/deploy/validate-deployment.sh" "$TARGET_HOST" 8080 3001

# ── Summary ──────────────────────────────────────────────────
print_header "Deployment Complete"
echo -e "  ${BOLD}Gateway:${NC}  http://${TARGET_HOST}:8080"
echo -e "  ${BOLD}Sidecar:${NC}  http://${TARGET_HOST}:3001"
echo -e "  ${BOLD}Metrics:${NC}  http://${TARGET_HOST}:9090/metrics"
echo ""
echo -e "  ${DIM}Export for test scripts:${NC}"
echo -e "    export GATEWAY_HOST=${TARGET_HOST}"
echo ""
