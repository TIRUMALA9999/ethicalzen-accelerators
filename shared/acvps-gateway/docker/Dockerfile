# Multi-stage build for minimal image size
FROM golang:1.21-alpine AS builder

# Install build dependencies
RUN apk add --no-cache git make

# Set working directory
WORKDIR /build

# Copy go mod files
COPY go.mod go.sum ./
RUN go mod download

# Copy source code
COPY . .

# Build the binary
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build \
    -ldflags="-w -s -X main.Version=$(git describe --tags --always --dirty) -X main.BuildTime=$(date -u '+%Y-%m-%d_%H:%M:%S')" \
    -o acvps-gateway \
    cmd/gateway/main.go

# Final stage - minimal runtime image
FROM alpine:latest

# Install runtime dependencies
RUN apk add --no-cache ca-certificates tzdata

# Create non-root user
RUN addgroup -g 1000 acvps && \
    adduser -D -u 1000 -G acvps acvps

# Set working directory
WORKDIR /app

# Copy binary from builder
COPY --from=builder /build/acvps-gateway /app/acvps-gateway

# Copy example config (can be overridden with volume mount)
COPY config.example.yaml /app/config.yaml

# Create directories for logs and certs
RUN mkdir -p /app/logs /app/certs && \
    chown -R acvps:acvps /app

# Switch to non-root user
USER acvps

# Expose ports
EXPOSE 443 9090

# Health check
HEALTHCHECK --interval=30s --timeout=10s --start-period=5s --retries=3 \
    CMD wget --no-verbose --tries=1 --spider http://localhost:9090/health || exit 1

# Run the gateway
ENTRYPOINT ["/app/acvps-gateway"]
CMD ["--config", "/app/config.yaml"]

