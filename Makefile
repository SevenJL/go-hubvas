.PHONY: all build build-api build-ws run test lint clean dev docker-build docker-up docker-down

# Go parameters
GOCMD=go
GOBUILD=$(GOCMD) build
GOTEST=$(GOCMD) test
GOLINT=golangci-lint
BINARY_DIR=bin

# Binary names
API_BINARY=$(BINARY_DIR)/api-server
WS_BINARY=$(BINARY_DIR)/ws-server

all: build

## build: Build all binaries
build: build-api build-ws

build-api:
	$(GOBUILD) -o $(API_BINARY) ./cmd/api-server

build-ws:
	$(GOBUILD) -o $(WS_BINARY) ./cmd/ws-server

## run-api: Run the API server (dev)
run-api:
	$(GOCMD) run ./cmd/api-server

## run-ws: Run the WS server (dev)
run-ws:
	$(GOCMD) run ./cmd/ws-server

## test: Run all tests
test:
	$(GOTEST) ./... -v -count=1

## test-race: Run tests with race detector
test-race:
	$(GOTEST) ./... -v -race -count=1

## lint: Run linter
lint:
	$(GOLINT) run ./...

## clean: Remove build artifacts
clean:
	rm -rf $(BINARY_DIR)

## dev: Run both servers concurrently (requires 'make' -j)
dev:
	$(GOCMD) run ./cmd/api-server &
	$(GOCMD) run ./cmd/ws-server &
	wait

## docker-build: Build Docker images
docker-build:
	docker build -t hubvas-api:latest -f deployments/docker/Dockerfile.api .
	docker build -t hubvas-ws:latest -f deployments/docker/Dockerfile.ws .

## docker-up: Start services with Docker Compose (dev — build from source + port overrides)
docker-up:
	docker compose --env-file deployments/docker/.env.dev -f deployments/docker/docker-compose.yaml -f deployments/docker/docker-compose.override.yml up -d --build

## docker-down: Stop Docker Compose services
docker-down:
	docker compose --env-file deployments/docker/.env.dev -f deployments/docker/docker-compose.yaml -f deployments/docker/docker-compose.override.yml down

## tidy: Tidy Go modules
tidy:
	$(GOCMD) mod tidy

## help: Show this help
help:
	@echo "Hubvas — Collaborative Canvas Platform"
	@echo ""
	@echo "Usage:"
	@echo "  make build        Build all binaries"
	@echo "  make run-api      Run API server (dev)"
	@echo "  make run-ws       Run WS server (dev)"
	@echo "  make test         Run all tests"
	@echo "  make lint         Run linter"
	@echo "  make docker-build Build Docker images"
	@echo "  make tidy         Tidy Go modules"
