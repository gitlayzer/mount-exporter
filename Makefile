# Mount Exporter Makefile

.PHONY: build test clean run fmt lint vet coverage benchmark docker-build docker-run docker-build-registry docker-push docker-dev docker-clean

# Variables
BINARY_NAME=mount-exporter
BUILD_DIR=build
DOCKER_REGISTRY=ghcr.io/mount-exporter
VERSION=$(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
GIT_COMMIT=$(shell git rev-parse HEAD 2>/dev/null || echo "unknown")
BUILD_TIME=$(shell date -u +'%Y-%m-%dT%H:%M:%SZ')
LDFLAGS=-ldflags "-X main.version=$(VERSION) -X main.gitCommit=$(GIT_COMMIT) -X main.buildTime=$(BUILD_TIME)"

# Build targets
build:
	@echo "Building $(BINARY_NAME)..."
	@mkdir -p $(BUILD_DIR)
	go build $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY_NAME) .

clean:
	@echo "Cleaning up..."
	@rm -rf $(BUILD_DIR)
	go clean -cache

test:
	@echo "Running tests..."
	go test -v ./...

test-unit:
	@echo "Running unit tests..."
	go test -v ./config ./metrics ./system ./server

test-integration:
	@echo "Running integration tests..."
	go test -v -tags=integration ./test

test-coverage:
	@echo "Running tests with coverage..."
	go test -v -coverprofile=coverage.out ./...
	go tool cover -html=coverage.out -o coverage.html
	@echo "Coverage report generated: coverage.html"

benchmark:
	@echo "Running benchmarks..."
	go test -bench=. -benchmem ./...

fmt:
	@echo "Formatting code..."
	go fmt ./...

vet:
	@echo "Vetting code..."
	go vet ./...

lint:
	@echo "Running linter..."
	golangci-lint run

run:
	@echo "Running $(BINARY_NAME)..."
	go run .

# Docker targets
docker-build:
	@echo "Building Docker image..."
	docker build \
		--build-arg VERSION=$(VERSION) \
		--build-arg GIT_COMMIT=$(GIT_COMMIT) \
		--build-arg BUILD_TIME=$(BUILD_TIME) \
		-t $(BINARY_NAME):$(VERSION) \
		-t $(BINARY_NAME):latest .

docker-build-registry:
	@echo "Building Docker image for registry..."
	docker build \
		--build-arg VERSION=$(VERSION) \
		--build-arg GIT_COMMIT=$(GIT_COMMIT) \
		--build-arg BUILD_TIME=$(BUILD_TIME) \
		-t $(DOCKER_REGISTRY)/$(BINARY_NAME):$(VERSION) \
		-t $(DOCKER_REGISTRY)/$(BINARY_NAME):latest .

docker-push:
	@echo "Pushing Docker image to registry..."
	docker push $(DOCKER_REGISTRY)/$(BINARY_NAME):$(VERSION)
	docker push $(DOCKER_REGISTRY)/$(BINARY_NAME):latest

docker-run:
	@echo "Running Docker container..."
	docker run --rm -p 8080:8080 \
		-v $(PWD)/examples/config.yaml:/etc/mount-exporter/config.yaml:ro \
		-v /:/host:ro \
		$(BINARY_NAME):$(VERSION)

docker-run-local:
	@echo "Running Docker container with local config..."
	docker run --rm -p 8080:8080 \
		-v $(PWD)/examples:/config:ro \
		$(BINARY_NAME):$(VERSION)

docker-dev:
	@echo "Building and running in development mode..."
	docker build \
		--build-arg VERSION=$(VERSION)-dev \
		-t $(BINARY_NAME):dev .
	docker run --rm -it -p 8080:8080 \
		-v $(PWD)/examples:/config:ro \
		$(BINARY_NAME):dev

docker-clean:
	@echo "Cleaning up Docker resources..."
	-docker rmi $(BINARY_NAME):$(VERSION) 2>/dev/null || true
	-docker rmi $(BINARY_NAME):latest 2>/dev/null || true
	-docker rmi $(DOCKER_REGISTRY)/$(BINARY_NAME):$(VERSION) 2>/dev/null || true
	-docker rmi $(DOCKER_REGISTRY)/$(BINARY_NAME):latest 2>/dev/null || true

# Development targets
dev: fmt vet test

# CI target
ci: fmt vet test-unit test-integration test-coverage

# Release targets
release: clean test docker-build
	@echo "Release build completed for version $(VERSION)"

release-registry: clean test docker-build-registry docker-push
	@echo "Release pushed to registry for version $(VERSION)"

# Development targets
dev: fmt vet test

# CI target
ci: fmt vet test-unit test-integration test-coverage

# Help target
help:
	@echo "Available targets:"
	@echo ""
	@echo "Build & Test:"
	@echo "  build              - Build the binary"
	@echo "  test               - Run all tests"
	@echo "  test-unit          - Run unit tests only"
	@echo "  test-integration   - Run integration tests only"
	@echo "  test-coverage      - Run tests with coverage report"
	@echo "  benchmark          - Run benchmarks"
	@echo "  clean              - Clean build artifacts"
	@echo ""
	@echo "Code Quality:"
	@echo "  fmt                - Format code"
	@echo "  vet                - Run go vet"
	@echo "  lint               - Run golangci-lint"
	@echo ""
	@echo "Development:"
	@echo "  run                - Run the application"
	@echo "  dev                - Format, vet, and test"
	@echo "  ci                 - Run CI pipeline"
	@echo ""
	@echo "Docker:"
	@echo "  docker-build       - Build Docker image locally"
	@echo "  docker-build-registry - Build Docker image for registry"
	@echo "  docker-push        - Push Docker image to registry"
	@echo "  docker-run         - Run Docker container with host mounts"
	@echo "  docker-run-local   - Run Docker container with local config"
	@echo "  docker-dev         - Build and run in development mode"
	@echo "  docker-clean       - Clean up Docker resources"
	@echo ""
	@echo "Release:"
	@echo "  release            - Build release version locally"
	@echo "  release-registry   - Build and push release to registry"
	@echo ""
	@echo "Current version: $(VERSION)"
	@echo "Git commit: $(GIT_COMMIT)"
	@echo "Build time: $(BUILD_TIME)"
	@echo "Docker registry: $(DOCKER_REGISTRY)"