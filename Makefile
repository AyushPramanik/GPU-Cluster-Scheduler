# GPU Cluster Scheduler - developer Makefile
# Run `make help` for the target list.

SHELL := /bin/bash
.DEFAULT_GOAL := help

# ---- config ---------------------------------------------------------------
MODULE      := github.com/ayushpramanik/gpu-cluster-scheduler
SERVICES    := api-gateway scheduler node-agent metrics
BIN_DIR     := bin
REGISTRY    ?= gpu-cluster-scheduler
VERSION     ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
COMMIT      ?= $(shell git rev-parse --short HEAD 2>/dev/null || echo unknown)
LDFLAGS     := -s -w -X main.version=$(VERSION) -X main.commit=$(COMMIT)
COMPOSE     := docker compose

DATABASE_URL ?= postgres://gpuscheduler:gpuscheduler@localhost:5432/gpuscheduler?sslmode=disable
MIGRATIONS_DIR ?= migrations

.PHONY: help build build-images test test-integration run down proto migrate lint tidy dev load-test clean fmt

help: ## Show this help
	@grep -E '^[a-zA-Z0-9_-]+:.*?## .*$$' $(MAKEFILE_LIST) \
		| sort \
		| awk 'BEGIN {FS = ":.*?## "}; {printf "  \033[36m%-18s\033[0m %s\n", $$1, $$2}'

build: ## Compile all Go service binaries into ./bin
	@mkdir -p $(BIN_DIR)
	@for svc in $(SERVICES); do \
		echo ">> building $$svc"; \
		CGO_ENABLED=0 go build -trimpath -ldflags "$(LDFLAGS)" -o $(BIN_DIR)/$$svc ./cmd/$$svc || exit 1; \
	done

build-images: ## Build docker images for all services + frontend
	@for svc in $(SERVICES); do \
		echo ">> image $(REGISTRY)/$$svc:latest"; \
		docker build -f deploy/docker/Dockerfile \
			--build-arg SERVICE=$$svc \
			--build-arg VERSION=$(VERSION) \
			--build-arg COMMIT=$(COMMIT) \
			-t $(REGISTRY)/$$svc:latest . || exit 1; \
	done
	docker build -f deploy/docker/Dockerfile.frontend -t $(REGISTRY)/frontend:latest ./web

test: ## Run unit tests with race detector + coverage
	go test ./... -race -cover -covermode=atomic

test-integration: ## Run integration tests (needs Postgres + Redis; `make run` first)
	go test ./... -race -tags=integration -run Integration -count=1

run: ## Start the full stack via docker-compose
	$(COMPOSE) up -d --build
	@echo "API:        http://localhost:8080"
	@echo "Grafana:    http://localhost:3001  (admin/admin)"
	@echo "Prometheus: http://localhost:9099"
	@echo "Jaeger:     http://localhost:16686"
	@echo "Frontend:   http://localhost:3000"

down: ## Stop the stack and remove volumes
	$(COMPOSE) down -v --remove-orphans

proto: ## Generate Go code from protobuf definitions
	@command -v protoc >/dev/null 2>&1 || { echo "protoc not found; install protobuf-compiler"; exit 1; }
	protoc \
		--proto_path=proto \
		--go_out=. --go_opt=module=$(MODULE) \
		--go-grpc_out=. --go-grpc_opt=module=$(MODULE) \
		$$(find proto -name '*.proto')

migrate: ## Apply database migrations (runs migrations/*.sql in order via psql)
	@command -v psql >/dev/null 2>&1 || { echo "psql not found; install postgresql-client"; exit 1; }
	@for f in $$(ls $(MIGRATIONS_DIR)/*.sql | sort); do \
		echo ">> applying $$f"; \
		psql "$(DATABASE_URL)" -v ON_ERROR_STOP=1 -f "$$f" || exit 1; \
	done

lint: ## Run golangci-lint
	@command -v golangci-lint >/dev/null 2>&1 || { echo "golangci-lint not found: https://golangci-lint.run/usage/install/"; exit 1; }
	golangci-lint run --timeout=5m

fmt: ## Format Go code
	gofmt -s -w .
	go vet ./...

tidy: ## Tidy and verify go modules
	go mod tidy
	go mod verify

dev: ## Start datastores + observability only, run services locally with hot-reload
	$(COMPOSE) up -d postgres redis prometheus grafana otel-collector jaeger
	@echo "Datastores + observability up. Run services with: go run ./cmd/<service>"
	@command -v air >/dev/null 2>&1 && echo "Tip: 'air' detected - use it for hot reload" || true

load-test: ## Run a scheduling load test against the API (needs k6 or hey)
	@if command -v k6 >/dev/null 2>&1; then \
		k6 run scripts/loadtest.js; \
	elif command -v hey >/dev/null 2>&1; then \
		hey -z 60s -c 50 -m POST -H "Content-Type: application/json" \
			-d '{"gpus":1,"cpu":2,"memory":"4Gi"}' http://localhost:8080/api/v1/jobs; \
	else \
		echo "Install k6 (https://k6.io) or hey (github.com/rakyll/hey) to run load tests"; exit 1; \
	fi

clean: ## Remove build artifacts
	rm -rf $(BIN_DIR)
	go clean -cache -testcache
