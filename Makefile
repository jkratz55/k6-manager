# Build parameters
BINARY_NAME=k6-manager
IMAGE_NAME=jkratz55/k6-manager
DOCKER_REGISTRY?=ghcr.io
VERSION?=$(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
PLATFORMS?=linux/amd64,linux/arm64

# Go parameters
GOCMD=go
GOBUILD=$(GOCMD) build
GOCLEAN=$(GOCMD) clean
GOTEST=$(GOCMD) test
GOMOD=$(GOCMD) mod
GOFMT=$(GOCMD) fmt
LDFLAGS=-ldflags="-s -w -X main.version=$(VERSION)"

# Frontend parameters
NPM=npm
FRONTEND_DIR=frontend

# Helm parameters
HELM=helm
CHART_DIR=chart

.DEFAULT_GOAL := help

.PHONY: help
help: ## Display this help message
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | sort | awk 'BEGIN {FS = ":.*?## "}; {printf "\033[36m%-20s\033[0m %s\n", $$1, $$2}'

.PHONY: all
all: clean build ## Clean and build the entire project

.PHONY: frontend-build
frontend-build: ## Build the frontend
	@echo "Building frontend..."
	@cd $(FRONTEND_DIR) && $(NPM) install && $(NPM) run build

.PHONY: build
build: ## Build the Go binary (includes frontend build)
	@if [ "$(SKIP_FRONTEND)" != "true" ]; then \
		$(MAKE) frontend-build; \
	fi
	@echo "Building $(BINARY_NAME)..."
	CGO_ENABLED=0 $(GOBUILD) $(LDFLAGS) -o $(BINARY_NAME) main.go

.PHONY: run
run: build ## Build and run the application locally
	./$(BINARY_NAME)

.PHONY: test
test: ## Run Go tests
	@echo "Running tests..."
	$(GOTEST) -v ./...

.PHONY: lint
lint: ## Run linters (golangci-lint and npm lint)
	@echo "Running Go linter..."
	@if command -v golangci-lint >/dev/null 2>&1; then \
		golangci-lint run ./...; \
	else \
		echo "golangci-lint not found. Run 'make install-lint' to install it."; \
	fi
	@echo "Running Frontend linter..."
	@cd $(FRONTEND_DIR) && $(NPM) run lint

.PHONY: tidy
tidy: ## Tidy Go modules
	$(GOMOD) tidy

.PHONY: fmt
fmt: ## Format code
	$(GOFMT) ./...
	@cd $(FRONTEND_DIR) && $(NPM) run lint -- --fix || true

.PHONY: clean
clean: ## Remove build artifacts
	@echo "Cleaning..."
	$(GOCLEAN)
	rm -f $(BINARY_NAME)
	rm -rf $(FRONTEND_DIR)/dist

.PHONY: docker-build
docker-build: ## Build local Docker image (for current architecture)
	@echo "Building Docker image $(IMAGE_NAME):$(VERSION)..."
	docker build -t $(IMAGE_NAME):$(VERSION) .

.PHONY: docker-release
docker-release: ## Build and push multi-arch Docker images using buildx
	@echo "Building and pushing multi-arch Docker images..."
	docker buildx build --platform $(PLATFORMS) \
		-t $(DOCKER_REGISTRY)/$(IMAGE_NAME):$(VERSION) \
		-t $(DOCKER_REGISTRY)/$(IMAGE_NAME):latest \
		--push .

.PHONY: docker-build-all
docker-build-all: ## Build multi-arch Docker images locally (no push)
	@echo "Building multi-arch Docker images..."
	docker buildx build --platform $(PLATFORMS) \
		-t $(IMAGE_NAME):$(VERSION) \
		.

.PHONY: helm-lint
helm-lint: ## Lint the Helm chart
	$(HELM) lint $(CHART_DIR)

.PHONY: helm-package
helm-package: ## Package the Helm chart
	$(HELM) package $(CHART_DIR)

.PHONY: install-lint
install-lint: ## Install golangci-lint
	curl -sSfL https://raw.githubusercontent.com/golangci/golangci-lint/master/install.sh | sh -s -- -b $(shell go env GOPATH)/bin v1.64.5
