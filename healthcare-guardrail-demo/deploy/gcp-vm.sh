#!/bin/bash
# ============================================================================
# EthicalZen Demo — Provision GCE VM + Deploy Gateway
# Creates a VM, opens firewall, deploys the Docker container
#
# Usage:
#   ./deploy/gcp-vm.sh                          # Default: us-central1-a
#   ./deploy/gcp-vm.sh --region us-east1         # Custom region
#   ./deploy/gcp-vm.sh --cleanup                 # Delete VM + firewall
# ============================================================================
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
source "${SCRIPT_DIR}/lib/colors.sh"
source "${SCRIPT_DIR}/lib/health.sh"

# Configuration
PROJECT_ID="${GCP_PROJECT_ID:-ethicalzen-public-04085}"
REGION="${GCP_REGION:-us-central1}"
ZONE="${GCP_ZONE:-us-central1-a}"
VM_NAME="ethicalzen-demo-gateway"
MACHINE_TYPE="e2-standard-2"
IMAGE="us-docker.pkg.dev/ethicalzen-public-04085/ethicalzen-public/ethicalzen-gateway:latest"
FIREWALL_RULE="allow-ethicalzen-demo"

# Parse arguments
CLEANUP=false
for arg in "$@"; do
  case "$arg" in
    --cleanup)  CLEANUP=true ;;
    --region)   shift; REGION="${1:-us-central1}"; ZONE="${REGION}-a" ;;
    --region=*) REGION="${arg#*=}"; ZONE="${REGION}-a" ;;
    --zone)     shift; ZONE="${1:-}" ;;
    --zone=*)   ZONE="${arg#*=}" ;;
  esac
done

# ── Cleanup Mode ─────────────────────────────────────────────
if [ "$CLEANUP" = true ]; then
  print_header "Cleaning Up GCE Demo Resources"
  print_info "Deleting VM: ${VM_NAME}..."
  gcloud compute instances delete "$VM_NAME" \
    --zone="$ZONE" --project="$PROJECT_ID" --quiet 2>/dev/null && \
    print_pass "VM deleted" || print_warn "VM not found or already deleted"

  print_info "Deleting firewall rule: ${FIREWALL_RULE}..."
  gcloud compute firewall-rules delete "$FIREWALL_RULE" \
    --project="$PROJECT_ID" --quiet 2>/dev/null && \
    print_pass "Firewall rule deleted" || print_warn "Firewall rule not found"

  print_info "Cleanup complete"
  exit 0
fi

# ── Pre-flight ───────────────────────────────────────────────
print_header "Deploying EthicalZen Gateway to GCE VM"

echo -e "  Project:      ${BOLD}${PROJECT_ID}${NC}"
echo -e "  Zone:         ${BOLD}${ZONE}${NC}"
echo -e "  Machine Type: ${BOLD}${MACHINE_TYPE}${NC}"
echo -e "  Image:        ${BOLD}${IMAGE}${NC}"
echo ""

if ! gcloud auth list --filter=status:ACTIVE --format="value(account)" 2>/dev/null | grep -q "@"; then
  print_error "Not authenticated. Run: gcloud auth login"
  exit 1
fi

gcloud config set project "$PROJECT_ID" --quiet

# ── Step 1: Create Firewall Rule ─────────────────────────────
print_step 1 "Creating firewall rule"
if gcloud compute firewall-rules describe "$FIREWALL_RULE" --project="$PROJECT_ID" &>/dev/null; then
  print_info "Firewall rule already exists"
else
  gcloud compute firewall-rules create "$FIREWALL_RULE" \
    --project="$PROJECT_ID" \
    --allow=tcp:8080,tcp:3001,tcp:9090 \
    --target-tags=ethicalzen-demo \
    --description="EthicalZen demo: gateway, sidecar, metrics"
  print_pass "Firewall rule created"
fi

# ── Step 2: Create VM ────────────────────────────────────────
print_step 2 "Provisioning VM"
if gcloud compute instances describe "$VM_NAME" --zone="$ZONE" --project="$PROJECT_ID" &>/dev/null; then
  print_info "VM already exists — reusing"
else
  gcloud compute instances create "$VM_NAME" \
    --project="$PROJECT_ID" \
    --zone="$ZONE" \
    --machine-type="$MACHINE_TYPE" \
    --image-family=cos-stable \
    --image-project=cos-cloud \
    --tags=ethicalzen-demo \
    --scopes=cloud-platform \
    --metadata=startup-script='#!/bin/bash
# Pull and run EthicalZen Gateway
docker-credential-gcr configure-docker
docker pull '"${IMAGE}"'
docker run -d --name ethicalzen-gateway \
  -p 8080:8080 -p 3001:3001 -p 9090:9090 \
  -e SMART_GUARDRAIL_URL=http://localhost:3001 \
  -e SG_ENGINE_URL=http://localhost:3001 \
  -e GUARDRAILS_PATH=/app/guardrails \
  -e PORTAL_BACKEND_URL='"${ETHICALZEN_BACKEND_URL:-https://api.ethicalzen.ai}"' \
  -e ETHICALZEN_API_KEY='"${ETHICALZEN_API_KEY:-}"' \
  -e NODE_ENV=production \
  '"${IMAGE}"'
'
  print_pass "VM created: ${VM_NAME}"
fi

# ── Step 3: Get External IP ──────────────────────────────────
print_step 3 "Getting external IP"
EXTERNAL_IP=$(gcloud compute instances describe "$VM_NAME" \
  --zone="$ZONE" --project="$PROJECT_ID" \
  --format="value(networkInterfaces[0].accessConfigs[0].natIP)")
print_pass "External IP: ${EXTERNAL_IP}"

# ── Step 4: Wait for Health ──────────────────────────────────
print_step 4 "Waiting for gateway to become healthy (up to 120s)"
if wait_for_health "http://${EXTERNAL_IP}:8080/health" 120; then
  print_pass "Gateway is healthy!"
else
  print_warn "Gateway not yet healthy. It may need more time for cold start."
  print_info "Check manually: curl http://${EXTERNAL_IP}:8080/health"
fi

# ── Step 5: Validate ─────────────────────────────────────────
print_step 5 "Running post-deployment validation"
"${SCRIPT_DIR}/deploy/validate-deployment.sh" "$EXTERNAL_IP" 8080 3001

# ── Summary ──────────────────────────────────────────────────
print_header "GCE Deployment Complete"
echo -e "  ${BOLD}Gateway:${NC}  http://${EXTERNAL_IP}:8080"
echo -e "  ${BOLD}Sidecar:${NC}  http://${EXTERNAL_IP}:3001"
echo -e "  ${BOLD}Metrics:${NC}  http://${EXTERNAL_IP}:9090/metrics"
echo ""
echo -e "  ${DIM}Export for test scripts:${NC}"
echo -e "    export GATEWAY_HOST=${EXTERNAL_IP}"
echo -e "    export SG_PORT=3001"
echo ""
echo -e "  ${DIM}Cleanup when done:${NC}"
echo -e "    ./deploy/gcp-vm.sh --cleanup"
echo ""
