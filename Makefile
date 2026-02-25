# ========================================
# DeltaCast — Developer Makefile
# ========================================

.PHONY: help build run test lint fmt tidy vet web-dev web-build web-lint docker-up docker-down docker-build clean clean-all

# Default target
help: ## Show available commands
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | \
		awk 'BEGIN {FS = ":.*?## "}; {printf "  \033[36m%-16s\033[0m %s\n", $$1, $$2}'

# ----------------------------------------
# Backend (Go)
# ----------------------------------------

build: ## Build the Go server binary
	cd server && go build -o bin/server ./cmd/

run: ## Run the Go server locally
	cd server && go run ./cmd/

test: ## Run all Go tests
	cd server && go test ./...

test-v: ## Run all Go tests (verbose)
	cd server && go test -v ./...

test-cover: ## Run Go tests with coverage report
	cd server && go test -coverprofile=coverage.out ./... && go tool cover -html=coverage.out -o coverage.html
	@echo "Coverage report: server/coverage.html"

lint: ## Run go vet
	cd server && go vet ./...

fmt: ## Format Go source files
	cd server && gofmt -w .
	cd server && goimports -w . 2>/dev/null || true

tidy: ## Tidy Go module dependencies
	cd server && go mod tidy

vet: lint ## Alias for lint

# ----------------------------------------
# Frontend (Next.js)
# ----------------------------------------

web-dev: ## Start Next.js dev server
	cd web && npm run dev

web-build: ## Build Next.js for production
	cd web && npm run build

web-lint: ## Lint frontend code
	cd web && npm run lint

# ----------------------------------------
# Docker
# ----------------------------------------

docker-up: ## Start all services via docker-compose
	docker-compose up -d

docker-down: ## Stop all services
	docker-compose down

docker-build: ## Rebuild and start all services
	docker-compose up -d --build

docker-logs: ## Tail logs from all services
	docker-compose logs -f

# ----------------------------------------
# Setup & Utilities
# ----------------------------------------

clean: ## Remove build artifacts
	rm -rf server/bin server/coverage.out server/coverage.html
	rm -rf web/.next web/out

clean-all: ## Remove build artifacts + Go caches
	rm -rf server/bin server/coverage.out server/coverage.html
	rm -rf web/.next web/out
	cd server && go clean -cache -testcache -modcache
