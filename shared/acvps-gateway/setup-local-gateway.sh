#!/bin/bash

set -e

# Colors
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

echo ""
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo "  🚀 EthicalZen ACVPS Gateway - Local/VPC Setup"
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo ""
echo "This script works for:"
echo "  • Local development (Docker on your laptop)"
echo "  • Customer VPC deployment (GCP/AWS/Azure)"
echo "  • On-premise deployment"
echo ""

# Detect if running in GCP
IS_GCP=false
if command -v gcloud &> /dev/null && gcloud config get-value project &> /dev/null; then
  IS_GCP=true
  PROJECT_ID=$(gcloud config get-value project)
  echo -e "${BLUE}ℹ️  Detected GCP environment: $PROJECT_ID${NC}"
  echo ""
fi

# Step 1: Check prerequisites
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo "  Step 1: Checking Prerequisites"
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo ""

# Check Docker
if ! command -v docker &> /dev/null; then
  echo -e "${RED}❌ Docker not found. Please install Docker first.${NC}"
  exit 1
fi
echo -e "${GREEN}✅ Docker installed:${NC} $(docker --version)"

# Check Docker Compose
if ! command -v docker-compose &> /dev/null && ! docker compose version &> /dev/null 2>&1; then
  echo -e "${RED}❌ Docker Compose not found. Please install Docker Compose first.${NC}"
  exit 1
fi
if command -v docker-compose &> /dev/null; then
  echo -e "${GREEN}✅ Docker Compose installed:${NC} $(docker-compose --version)"
  COMPOSE_CMD="docker-compose"
else
  echo -e "${GREEN}✅ Docker Compose installed:${NC} $(docker compose version)"
  COMPOSE_CMD="docker compose"
fi

# Check curl
if ! command -v curl &> /dev/null; then
  echo -e "${RED}❌ curl not found. Please install curl first.${NC}"
  exit 1
fi
echo -e "${GREEN}✅ curl installed${NC}"

echo ""

# Step 2: Choose deployment type
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo "  Step 2: Deployment Type"
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo ""
echo "Where are you deploying the gateway?"
echo "  1) Local development (Docker on this machine)"
echo "  2) Customer VPC (GCP Cloud Run)"
echo "  3) Customer VPC (Docker/Kubernetes)"
echo ""
read -p "Enter choice [1-3]: " DEPLOYMENT_TYPE

case $DEPLOYMENT_TYPE in
  1)
    DEPLOYMENT_MODE="local-docker"
    echo -e "${GREEN}✅ Local Docker deployment selected${NC}"
    ;;
  2)
    DEPLOYMENT_MODE="gcp-cloud-run"
    echo -e "${GREEN}✅ GCP Cloud Run deployment selected${NC}"
    if [ "$IS_GCP" = false ]; then
      echo -e "${RED}❌ gcloud not configured. Please run: gcloud auth login${NC}"
      exit 1
    fi
    ;;
  3)
    DEPLOYMENT_MODE="vpc-docker"
    echo -e "${GREEN}✅ VPC Docker/Kubernetes deployment selected${NC}"
    ;;
  *)
    echo -e "${RED}❌ Invalid choice${NC}"
    exit 1
    ;;
esac
echo ""

# Step 3: Authentication
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo "  Step 3: Authentication"
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo ""
echo "How do you want to authenticate?"
echo "  1) Email and password"
echo "  2) API key (from your portal dashboard)"
echo ""
read -p "Enter choice [1-2]: " AUTH_METHOD

CONTROL_PLANE_URL=${CONTROL_PLANE_URL:-"https://ethicalzen-backend-400782183161.us-central1.run.app"}

if [ "$AUTH_METHOD" = "1" ]; then
  # Login with email/password
  echo ""
  echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
  echo "  Login with Email/Password"
  echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
  echo ""
  
  read -p "Email: " USER_EMAIL
  read -sp "Password: " USER_PASSWORD
  echo ""
  
  echo ""
  echo -e "${BLUE}ℹ️  Authenticating...${NC}"
  
  # Login
  LOGIN_RESPONSE=$(curl -s -X POST "$CONTROL_PLANE_URL/api/auth/login" \
    -H "Content-Type: application/json" \
    -d "{\"email\": \"$USER_EMAIL\", \"password\": \"$USER_PASSWORD\"}")
  
  SUCCESS=$(echo "$LOGIN_RESPONSE" | grep -o '"success"[[:space:]]*:[[:space:]]*true' || echo "")
  
  if [ -z "$SUCCESS" ]; then
    echo -e "${RED}❌ Login failed${NC}"
    echo "$LOGIN_RESPONSE"
    echo ""
    echo "Don't have an account? Register at:"
    echo "https://ethicalzen.ai/enterprise-access.html"
    exit 1
  fi
  
  JWT_TOKEN=$(echo "$LOGIN_RESPONSE" | grep -o '"token"[[:space:]]*:[[:space:]]*"[^"]*"' | cut -d'"' -f4)
  echo -e "${GREEN}✅ Login successful!${NC}"
  
elif [ "$AUTH_METHOD" = "2" ]; then
  # Use user's API key from portal
  echo ""
  echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
  echo "  Authenticate with API Key"
  echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
  echo ""
  echo "Find your API key at: https://ethicalzen.ai/dashboard.html"
  echo "  → Go to 'API Keys' tab"
  echo "  → Copy your API key"
  echo ""
  
  read -p "API Key: " USER_API_KEY
  
  if [ -z "$USER_API_KEY" ]; then
    echo -e "${RED}❌ API key is required${NC}"
    exit 1
  fi
  
  echo ""
  echo -e "${BLUE}ℹ️  Authenticating with API key...${NC}"
  
  # Authenticate with API key (using a simple endpoint to validate)
  AUTH_RESPONSE=$(curl -s -X GET "$CONTROL_PLANE_URL/api/tenants/me" \
    -H "X-API-Key: $USER_API_KEY")
  
  SUCCESS=$(echo "$AUTH_RESPONSE" | grep -o '"success"[[:space:]]*:[[:space:]]*true' || echo "")
  
  if [ -z "$SUCCESS" ]; then
    echo -e "${RED}❌ Invalid API key${NC}"
    echo "$AUTH_RESPONSE"
    exit 1
  fi
  
  # API key is valid, use it as Bearer token for gateway registration
  JWT_TOKEN=$USER_API_KEY
  echo -e "${GREEN}✅ API key validated!${NC}"
  
else
  echo -e "${RED}❌ Invalid choice${NC}"
  exit 1
fi

# Register gateway (same for both auth methods)
echo ""
read -p "Enter gateway name (e.g., my-dev-gateway): " GATEWAY_NAME
GATEWAY_NAME=${GATEWAY_NAME:-"local-gateway-$(date +%s)"}

echo ""
echo -e "${BLUE}ℹ️  Registering gateway: $GATEWAY_NAME${NC}"

RESPONSE=$(curl -s -X POST "$CONTROL_PLANE_URL/api/gateway/register" \
  -H "Authorization: Bearer $JWT_TOKEN" \
  -H "Content-Type: application/json" \
  -d "{\"gateway_name\": \"$GATEWAY_NAME\"}")

SUCCESS=$(echo "$RESPONSE" | grep -o '"success"[[:space:]]*:[[:space:]]*true' || echo "")

if [ -z "$SUCCESS" ]; then
  echo -e "${RED}❌ Failed to register gateway${NC}"
  echo "$RESPONSE"
  exit 1
fi

GATEWAY_API_KEY=$(echo "$RESPONSE" | grep -o '"api_key"[[:space:]]*:[[:space:]]*"[^"]*"' | cut -d'"' -f4)
GATEWAY_TENANT_ID=$(echo "$RESPONSE" | grep -o '"tenant_id"[[:space:]]*:[[:space:]]*"[^"]*"' | cut -d'"' -f4)

echo ""
echo -e "${GREEN}✅ Gateway registered successfully!${NC}"
echo ""
echo "Gateway ID: $(echo "$RESPONSE" | grep -o '"gateway_id"[[:space:]]*:[[:space:]]*"[^"]*"' | cut -d'"' -f4)"
echo "Gateway API Key: $GATEWAY_API_KEY"
echo "Tenant ID: $GATEWAY_TENANT_ID"
echo ""
echo -e "${YELLOW}⚠️  Save the Gateway API Key - it will not be shown again!${NC}"
echo ""

# Default control plane URL
CONTROL_PLANE_URL=${CONTROL_PLANE_URL:-"https://ethicalzen-backend-400782183161.us-central1.run.app"}

echo ""

# Step 4: Gateway Configuration Complete
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo "  Step 4: Gateway Configuration"
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo ""
echo -e "${CYAN}ℹ️  The gateway routes to backends based on contracts${NC}"
echo -e "${CYAN}   Each contract specifies its own service_endpoint${NC}"
echo ""
echo -e "${GREEN}✅ No hardcoded backend URL needed!${NC}"
echo ""

# Step 5: Create configuration files
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo "  Step 5: Creating Configuration Files"
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo ""

# Create .env.local
cat > .env.local <<EOF
# EthicalZen Gateway Configuration
# Generated: $(date)
# Deployment: $DEPLOYMENT_MODE

# Gateway Authentication (REQUIRED)
GATEWAY_API_KEY=$GATEWAY_API_KEY
GATEWAY_TENANT_ID=$GATEWAY_TENANT_ID

# Control Plane (Backend API)
CONTROL_PLANE_URL=$CONTROL_PLANE_URL

# Logging
LOG_LEVEL=debug
LOG_FORMAT=text

# Note: The gateway routes to multiple backends based on contracts
# Each contract specifies its own service_endpoint
EOF

echo -e "${GREEN}✅ Created .env.local${NC}"

# Create config-local.yaml if it doesn't exist
if [ ! -f "config-local.yaml" ]; then
  if [ -f "config-local.yaml.template" ]; then
    cp config-local.yaml.template config-local.yaml
    echo -e "${GREEN}✅ Created config-local.yaml from template${NC}"
  else
    echo -e "${YELLOW}⚠️  config-local.yaml.template not found, skipping${NC}"
  fi
fi

echo ""

# Step 6: Deploy
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo "  Step 6: Deployment"
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo ""

if [ "$DEPLOYMENT_MODE" = "local-docker" ]; then
  echo -e "${BLUE}ℹ️  Starting gateway with Docker Compose...${NC}"
  echo ""
  
  $COMPOSE_CMD -f docker-compose-local.yaml --env-file .env.local up -d
  
  echo ""
  echo -e "${GREEN}✅ Gateway started!${NC}"
  echo ""
  echo "Gateway is running at:"
  echo "  • Gateway: http://localhost:8443"
  echo "  • Metrics: http://localhost:9090/metrics"
  echo ""
  echo "View logs:"
  echo "  $COMPOSE_CMD -f docker-compose-local.yaml logs -f gateway"
  echo ""
  echo "Stop gateway:"
  echo "  $COMPOSE_CMD -f docker-compose-local.yaml down"
  echo ""
  
  # Test connection
  echo -e "${BLUE}ℹ️  Testing gateway...${NC}"
  sleep 3
  
  if curl -s http://localhost:9090/metrics > /dev/null; then
    echo -e "${GREEN}✅ Gateway is healthy!${NC}"
  else
    echo -e "${YELLOW}⚠️  Gateway may still be starting...${NC}"
  fi
  
elif [ "$DEPLOYMENT_MODE" = "gcp-cloud-run" ]; then
  echo -e "${BLUE}ℹ️  Deploying to GCP Cloud Run...${NC}"
  echo ""
  
  # Build and push image
  IMAGE_NAME="gcr.io/$PROJECT_ID/acvps-gateway-local:latest"
  
  echo "Building Docker image..."
  docker build -t "$IMAGE_NAME" .
  
  echo "Pushing to GCR..."
  docker push "$IMAGE_NAME"
  
  # Deploy to Cloud Run
  SERVICE_NAME="acvps-gateway-local-${GATEWAY_TENANT_ID##*_}"
  
  echo "Deploying to Cloud Run..."
  gcloud run deploy "$SERVICE_NAME" \
    --image "$IMAGE_NAME" \
    --platform managed \
    --region us-central1 \
    --allow-unauthenticated \
    --set-env-vars "GATEWAY_MODE=local,GATEWAY_API_KEY=$GATEWAY_API_KEY,GATEWAY_TENANT_ID=$GATEWAY_TENANT_ID,CONTROL_PLANE_URL=$CONTROL_PLANE_URL,LOG_LEVEL=debug" \
    --port 8443 \
    --memory 512Mi \
    --cpu 1 \
    --min-instances 1 \
    --max-instances 10
  
  GATEWAY_URL=$(gcloud run services describe "$SERVICE_NAME" --region us-central1 --format='value(status.url)')
  
  echo ""
  echo -e "${GREEN}✅ Gateway deployed to Cloud Run!${NC}"
  echo ""
  echo "Gateway URL: $GATEWAY_URL"
  echo ""
  
else
  echo -e "${BLUE}ℹ️  VPC Docker/Kubernetes deployment${NC}"
  echo ""
  echo "Configuration files created. Next steps:"
  echo ""
  echo "For Docker:"
  echo "  docker run -d \\"
  echo "    -p 8443:8443 \\"
  echo "    -p 9090:9090 \\"
  echo "    --env-file .env.local \\"
  echo "    -v \$(pwd)/config-local.yaml:/app/config.yaml:ro \\"
  echo "    gcr.io/ethicalzen-public-04085/acvps-gateway:latest"
  echo ""
  echo "For Kubernetes:"
  echo "  1. Create ConfigMap: kubectl create configmap gateway-config --from-file=config-local.yaml"
  echo "  2. Create Secret: kubectl create secret generic gateway-creds --from-env-file=.env.local"
  echo "  3. Deploy: kubectl apply -f k8s-deployment.yaml"
  echo ""
fi

echo ""
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo "  ✅ Setup Complete!"
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo ""
echo "Next steps:"
echo "  1. Test the gateway (see above)"
echo "  2. Configure your app to use the gateway"
echo "  3. Monitor evidence in the portal: https://ethicalzen.ai/dashboard.html"
echo ""
echo "Documentation:"
echo "  • Local Dev Guide: docs/LOCAL_DEVELOPMENT_GUIDE.md"
echo "  • SDK Integration: sdk/QUICKSTART.md"
echo ""

