# ========================================
# DeltaCast — Developer Makefile
# ========================================

.PHONY: help build run test lint fmt tidy web-install web-dev web-build web-lint web-type-check web-test web-preview docker-up docker-down docker-build clean gcp-open gcp-close gcp-status gcp-livestream-cleanup yt-status yt-open yt-close res-open res-close res-status

SERVER_DIR := server
WEB_DIR := web


# Default target
help: ## Show available commands
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | \
		awk 'BEGIN {FS = ":.*?## "}; {printf "  \033[36m%-16s\033[0m %s\n", $$1, $$2}'

# ----------------------------------------
# Backend (Go)
# ----------------------------------------

build: ## Build the Go server binary
	@cd $(SERVER_DIR) && go build -o bin/server ./cmd/

run: ## Run the Go server locally (loads server/.env.local if present)
	@cd $(SERVER_DIR) && set -a && [ -f .env.local ] && . .env.local; set +a && go run ./cmd/

test: ## Run all Go tests
	@cd $(SERVER_DIR) && go test -race ./...

test-cover: ## Run Go tests with coverage report
	@cd $(SERVER_DIR) && go test -coverprofile=coverage.out ./... && go tool cover -html=coverage.out -o coverage.html
	echo "Coverage report: $(SERVER_DIR)/coverage.html"

lint: ## Run go vet
	cd $(SERVER_DIR) && go vet ./...

fmt: ## Format Go source files
	@cd $(SERVER_DIR) && gofmt -w .
	@cd $(SERVER_DIR) && goimports -w . 2>/dev/null || true

# Download dependencies
deps:
	@cd $(SERVER_DIR) && go mod download

# Tidy dependencies
tidy:
	@cd $(SERVER_DIR) && go mod tidy

# ----------------------------------------
# Frontend (React / Vite)
# ----------------------------------------

web-install: ## Install frontend dependencies (pnpm)
	@cd $(WEB_DIR) && pnpm install

web-dev: ## Start Vite dev server (localhost:5173)
	@cd $(WEB_DIR) && pnpm dev

web-build: ## Build frontend for production (output: web/dist/)
	@cd $(WEB_DIR) && pnpm build

web-lint: ## Lint frontend code
	@cd $(WEB_DIR) && pnpm lint

web-type-check: ## Type check frontend code
	@cd $(WEB_DIR) && pnpm type-check

web-test: ## Run frontend tests
	@cd $(WEB_DIR) && pnpm test

web-preview: ## Preview production build locally
	@cd $(WEB_DIR) && pnpm preview

# ----------------------------------------
# Docker（僅 backend）
# ----------------------------------------

docker-up: ## Start backend service via docker-compose (local only)
	@docker-compose -f docker-compose.local.yml up -d

docker-down: ## Stop backend service
	@docker-compose -f docker-compose.local.yml down

docker-build: ## Rebuild and start backend service
	@docker-compose -f docker-compose.local.yml up -d --build

docker-logs: ## Tail logs from backend service
	@docker-compose -f docker-compose.local.yml logs -f

# ----------------------------------------
# Setup & Utilities
# ----------------------------------------

clean: ## Remove build artifacts + Go caches
	@rm -rf $(SERVER_DIR)/bin $(SERVER_DIR)/coverage.out $(SERVER_DIR)/coverage.html
	@rm -rf $(WEB_DIR)/dist
	@cd $(SERVER_DIR) && go clean -cache -testcache -modcache

# ----------------------------------------
# GCP Resource Control
# ----------------------------------------

gcp-status: ## Check GCP resource status (ready for test?)
	@chmod +x script/gcp-status.sh
	@./script/gcp-status.sh

gcp-livestream-cleanup: ## Stop and delete all active (billable) Live Stream channels and inputs
	@chmod +x script/gcp-livestream-cleanup.sh
	@./script/gcp-livestream-cleanup.sh

gcp-open: ## Open CDN for testing: allow-all traffic + unlock GCS bucket
	@chmod +x script/gcp-cdn-armor.sh script/gcp-storage-secure.sh
	@echo "\n\033[36m── Step 1/2: CDN → allow all traffic (via Cloudflare) ──\033[0m"
	@./script/gcp-cdn-armor.sh --mode allow-all
	@echo "\n\033[36m── Step 2/2: GCS → unlock public read ──\033[0m"
	@./script/gcp-storage-secure.sh --mode unlock
	@echo "\n\033[32m✅  Resources open for testing. Run 'make gcp-status' to verify.\033[0m\n"

gcp-close: ## Close CDN after testing: deny-all + lock GCS bucket
	@chmod +x script/gcp-cdn-armor.sh script/gcp-storage-secure.sh
	@echo "\n\033[36m── Step 1/2: CDN → deny all traffic ──\033[0m"
	@./script/gcp-cdn-armor.sh --mode deny-all
	@echo "\n\033[36m── Step 2/2: GCS → lock direct access ──\033[0m"
	@./script/gcp-storage-secure.sh --mode lock
	@echo "\n\033[32m✅  Resources closed. Run 'make gcp-status' to verify.\033[0m\n"

# ----------------------------------------
# YouTube Resource Control
# ----------------------------------------

yt-status: ## Show YouTube API status + all Broadcasts privacy state
	@chmod +x script/youtube-secure.sh
	@./script/youtube-secure.sh --mode status

yt-open: ## Unlock YouTube for testing: set Broadcasts to unlisted
	@chmod +x script/youtube-secure.sh
	@./script/youtube-secure.sh --mode unlock

yt-close: ## Lock YouTube after testing: set Broadcasts to private
	@chmod +x script/youtube-secure.sh
	@./script/youtube-secure.sh --mode lock

# ----------------------------------------
# Test Session Control (GCP + YouTube)
# ----------------------------------------

res-open: ## Open all resources for testing: allow-all CDN + unlock GCS + unlock YT
	@echo "\n\033[36m════ res-open: GCP ════\033[0m"
	@$(MAKE) gcp-open
	@echo "\n\033[36m════ res-open: YouTube ════\033[0m"
	@$(MAKE) yt-open
	@echo "\n\033[32m✅  All resources open. Run 'make res-status' to verify.\033[0m\n"

res-close: ## Close all resources after testing: deny CDN + lock GCS + lock YT + cleanup Live Stream
	@echo "\n\033[36m════ res-close: GCP ════\033[0m"
	@$(MAKE) gcp-close
	@echo "\n\033[36m════ res-close: Live Stream cleanup ════\033[0m"
	@$(MAKE) gcp-livestream-cleanup
	@echo "\n\033[36m════ res-close: YouTube ════\033[0m"
	@$(MAKE) yt-close
	@echo "\n\033[32m✅  All resources closed. Run 'make res-status' to verify.\033[0m\n"

res-status: ## Show status of all resources (GCP + YouTube)
	@echo "\n\033[36m════ GCP Status ════\033[0m"
	@$(MAKE) gcp-status
	@echo "\n\033[36m════ YouTube Status ════\033[0m"
	@$(MAKE) yt-status
