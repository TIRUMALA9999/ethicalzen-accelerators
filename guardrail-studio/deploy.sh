#!/bin/bash
# Deploy Guardrail Studio to Cloud Run
# Usage: ./deploy.sh [project-id]

set -e

PROJECT_ID="${1:-ethicalzen-platform}"
REGION="us-central1"
SERVICE_NAME="guardrail-studio"
IMAGE_NAME="gcr.io/${PROJECT_ID}/${SERVICE_NAME}"

echo "🚀 Deploying Guardrail Studio to Cloud Run..."
echo "   Project: ${PROJECT_ID}"
echo "   Region: ${REGION}"
echo "   Service: ${SERVICE_NAME}"

# Build the container
echo "📦 Building container image..."
docker build -t ${IMAGE_NAME} .

# Push to Container Registry
echo "⬆️  Pushing to Container Registry..."
docker push ${IMAGE_NAME}

# Deploy to Cloud Run
echo "🌐 Deploying to Cloud Run..."
gcloud run deploy ${SERVICE_NAME} \
  --image ${IMAGE_NAME} \
  --platform managed \
  --region ${REGION} \
  --project ${PROJECT_ID} \
  --allow-unauthenticated \
  --port 8080 \
  --memory 256Mi \
  --cpu 1 \
  --min-instances 0 \
  --max-instances 10 \
  --concurrency 80

# Get the service URL
SERVICE_URL=$(gcloud run services describe ${SERVICE_NAME} \
  --platform managed \
  --region ${REGION} \
  --project ${PROJECT_ID} \
  --format 'value(status.url)')

echo ""
echo "✅ Deployment complete!"
echo "   Service URL: ${SERVICE_URL}"
echo ""
echo "📋 Next steps:"
echo "   1. Map custom domain: studio.ethicalzen.ai"
echo "      gcloud run domain-mappings create --service ${SERVICE_NAME} --domain studio.ethicalzen.ai --region ${REGION}"
echo ""
echo "   2. Or use Load Balancer for custom domain with SSL"


