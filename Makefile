# ========================================
# DeltaCast — Developer Makefile
# ========================================

.PHONY: help build run test lint fmt tidy vet web-dev web-build web-lint docker-up docker-down docker-build clean clean-all gcp-open gcp-open-public gcp-close gcp-status yt-status yt-open yt-close res-open res-open-public res-close res-status

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
	cd web && pnpm dev

web-build: ## Build Next.js for production
	cd web && pnpm build

web-lint: ## Lint frontend code
	cd web && pnpm lint

# ----------------------------------------
# Docker
# ----------------------------------------

docker-up: ## Start all services via docker-compose (local only)
	docker-compose -f docker-compose.local.yml up -d

docker-down: ## Stop all services
	docker-compose -f docker-compose.local.yml down

docker-build: ## Rebuild and start all services
	docker-compose -f docker-compose.local.yml up -d --build

docker-logs: ## Tail logs from all services
	docker-compose -f docker-compose.local.yml logs -f

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

# ----------------------------------------
# GCP Resource Control
# ----------------------------------------

gcp-status: ## Check GCP resource status (ready for test?)
	@chmod +x script/gcp-status.sh
	@./script/gcp-status.sh

gcp-open: ## Open CDN for testing: allowlist your IP + unlock GCS bucket
	@chmod +x script/gcp-cdn-armor.sh script/gcp-storage-secure.sh
	@echo "\n\033[36m── Step 1/2: CDN → allowlist your IP ──\033[0m"
	@./script/gcp-cdn-armor.sh --mode allowlist
	@echo "\n\033[36m── Step 2/2: GCS → unlock public read ──\033[0m"
	@./script/gcp-storage-secure.sh --mode unlock
	@echo "\n\033[32m✅  Resources open for testing. Run 'make gcp-status' to verify.\033[0m\n"

gcp-open-public: ## Open CDN to everyone (allow-all) + unlock GCS bucket
	@chmod +x script/gcp-cdn-armor.sh script/gcp-storage-secure.sh
	@echo "\n\033[36m── Step 1/2: CDN → allow all traffic ──\033[0m"
	@./script/gcp-cdn-armor.sh --mode allow-all
	@echo "\n\033[36m── Step 2/2: GCS → unlock public read ──\033[0m"
	@./script/gcp-storage-secure.sh --mode unlock
	@echo "\n\033[32m✅  Resources fully open. Run 'make gcp-status' to verify.\033[0m\n"

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

res-open: ## Open all resources for testing: allowlist IP + unlock GCS + unlock YT
	@echo "\n\033[36m════ res-open: GCP ════\033[0m"
	@$(MAKE) gcp-open
	@echo "\n\033[36m════ res-open: YouTube ════\033[0m"
	@$(MAKE) yt-open
	@echo "\n\033[32m✅  All resources open. Run 'make res-status' to verify.\033[0m\n"

res-open-public: ## Open all resources to everyone: CDN allow-all + unlock GCS + unlock YT
	@echo "\n\033[36m════ res-open-public: GCP ════\033[0m"
	@$(MAKE) gcp-open-public
	@echo "\n\033[36m════ res-open-public: YouTube ════\033[0m"
	@$(MAKE) yt-open
	@echo "\n\033[32m✅  All resources fully open. Run 'make res-status' to verify.\033[0m\n"

res-close: ## Close all resources after testing: deny CDN + lock GCS + lock YT
	@echo "\n\033[36m════ res-close: GCP ════\033[0m"
	@$(MAKE) gcp-close
	@echo "\n\033[36m════ res-close: YouTube ════\033[0m"
	@$(MAKE) yt-close
	@echo "\n\033[32m✅  All resources closed. Run 'make res-status' to verify.\033[0m\n"

res-status: ## Show status of all resources (GCP + YouTube)
	@echo "\n\033[36m════ GCP Status ════\033[0m"
	@$(MAKE) gcp-status
	@echo "\n\033[36m════ YouTube Status ════\033[0m"
	@$(MAKE) yt-status
