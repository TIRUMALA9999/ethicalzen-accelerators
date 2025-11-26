.PHONY: build test run docker-build docker-run clean

# Variables
BINARY_NAME=acvps-gateway
DOCKER_IMAGE=ethicalzen/acvps-gateway
VERSION=$(shell git describe --tags --always --dirty)
BUILD_TIME=$(shell date -u '+%Y-%m-%d_%H:%M:%S')

# Build flags
LDFLAGS=-ldflags "-X main.Version=$(VERSION) -X main.BuildTime=$(BUILD_TIME)"

# Build the gateway
build:
	@echo "Building $(BINARY_NAME)..."
	go build $(LDFLAGS) -o $(BINARY_NAME) cmd/gateway/main.go

# Run tests
test:
	@echo "Running tests..."
	go test -v -race -cover ./...

# Run integration tests
test-integration:
	@echo "Running integration tests..."
	go test -v ./tests/integration/...

# Run the gateway locally
run:
	@echo "Running $(BINARY_NAME)..."
	go run cmd/gateway/main.go --config config.yaml

# Build Docker image
docker-build:
	@echo "Building Docker image..."
	docker build -t $(DOCKER_IMAGE):$(VERSION) -f docker/Dockerfile .
	docker tag $(DOCKER_IMAGE):$(VERSION) $(DOCKER_IMAGE):latest

# Run Docker image
docker-run:
	@echo "Running Docker image..."
	docker-compose -f docker/docker-compose.yaml up

# Push Docker image
docker-push:
	@echo "Pushing Docker image..."
	docker push $(DOCKER_IMAGE):$(VERSION)
	docker push $(DOCKER_IMAGE):latest

# Clean build artifacts
clean:
	@echo "Cleaning..."
	rm -f $(BINARY_NAME)
	go clean

# Install dependencies
deps:
	@echo "Installing dependencies..."
	go mod download
	go mod tidy

# Run linter
lint:
	@echo "Running linter..."
	golangci-lint run

# Format code
fmt:
	@echo "Formatting code..."
	go fmt ./...

# Generate mocks
mocks:
	@echo "Generating mocks..."
	mockgen -source=internal/blockchain/client.go -destination=internal/blockchain/mocks/client_mock.go

# Run load test
load-test:
	@echo "Running load test..."
	hey -n 10000 -c 100 -H "X-DC-Id: test/healthcare/us/v1.0" -H "X-DC-Digest: sha256-test" http://localhost:443/health

# Help
help:
	@echo "Available targets:"
	@echo "  build            - Build the gateway binary"
	@echo "  test             - Run unit tests"
	@echo "  test-integration - Run integration tests"
	@echo "  run              - Run the gateway locally"
	@echo "  docker-build     - Build Docker image"
	@echo "  docker-run       - Run Docker compose stack"
	@echo "  docker-push      - Push Docker image to registry"
	@echo "  clean            - Clean build artifacts"
	@echo "  deps             - Install dependencies"
	@echo "  lint             - Run linter"
	@echo "  fmt              - Format code"
	@echo "  mocks            - Generate mocks"
	@echo "  load-test        - Run load test"

