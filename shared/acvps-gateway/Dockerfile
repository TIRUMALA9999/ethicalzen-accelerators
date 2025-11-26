# ACVPS Gateway Dockerfile
# Multi-stage build for optimized image size

# Build stage
FROM golang:1.24-alpine AS builder

WORKDIR /build

# Install build dependencies
RUN apk add --no-cache git make

# Copy go mod files
COPY go.mod go.sum ./

# Download dependencies
RUN go mod download

# Copy source code
COPY . .

# Build the application
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build \
    -ldflags="-w -s" \
    -o gateway \
    ./cmd/gateway

# Runtime stage
FROM alpine:latest

WORKDIR /app

# Install runtime dependencies
RUN apk --no-cache add ca-certificates tzdata

# Copy binary from builder
COPY --from=builder /build/gateway .

# Copy config files (cloud config only - config.yaml is for local dev)
COPY config-cloud.yaml .

# ============================================================================
# BUNDLE PLATFORM BASELINE GUARDRAILS INTO IMAGE
# ============================================================================
# Platform guardrails (PII, PHI, HIPAA, etc.) are baked into the Docker image
# These provide the baseline security/compliance that ALL tenants get
#
# Tenant-specific custom guardrails are loaded from GitHub at runtime via GDK
# ============================================================================
COPY guardrail_repo /app/guardrail_repo

# Verify guardrails were copied
RUN ls -la /app/guardrail_repo/default/ && \
    echo "✅ Platform baseline guardrails bundled in image"

# ============================================================================
# BUNDLE MOCK CONTRACTS FOR LOCAL TESTING
# ============================================================================
# Mock contracts enable full validation testing in local mode without Redis
# These are only used when MODE=local
# ============================================================================
COPY mock-contracts /app/mock-contracts

# Verify mock contracts were copied
RUN ls -la /app/mock-contracts/ && \
    echo "✅ Mock contracts bundled for local testing"

# Expose ports
EXPOSE 8443 9090

# Health check
HEALTHCHECK --interval=30s --timeout=3s --start-period=10s --retries=3 \
  CMD wget --quiet --tries=1 --spider http://localhost:9090/metrics || exit 1

# Run as non-root user
RUN adduser -D -u 1000 gateway && \
    chown gateway:gateway /app/config-cloud.yaml
USER gateway

# Start gateway
# Replace Redis placeholders with actual values from env vars, then start gateway
CMD sh -c "sed -e \"s/REDIS_HOST_PLACEHOLDER/${REDIS_HOST:-localhost}/g\" -e \"s/REDIS_PORT_PLACEHOLDER/${REDIS_PORT:-6379}/g\" config-cloud.yaml > /tmp/config-runtime.yaml && exec ./gateway -config /tmp/config-runtime.yaml"

