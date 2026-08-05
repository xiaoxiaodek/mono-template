FRONTEND_DIR := frontend
GOLANGCI_LINT ?= golangci-lint

.PHONY: help
help:
	@grep -E '^[a-zA-Z_-]+:.*?## ' $(MAKEFILE_LIST) | awk 'BEGIN {FS = ":.*?## "}; {printf "%-20s %s\n", $$1, $$2}'

.PHONY: go-test
go-test: ## Run backend tests
	go test ./...

.PHONY: go-build
go-build: ## Build backend binaries
	go build ./apps/operation-api/cmd/server

.PHONY: go-vet
go-vet: ## Run backend static analysis
	go vet ./...

.PHONY: go-fmt
go-fmt: ## Format backend code
	gofmt -w $$(find . -name '*.go' -not -path './.git/*')

.PHONY: run-api
run-api: ## Run the operation API from source
	go run ./apps/operation-api/cmd/server

.PHONY: go-lint
go-lint: ## Lint backend code
	$(GOLANGCI_LINT) run --config .golangci.yml ./...

.PHONY: swag-init
swag-init: ## Generate swagger docs from Go annotations
	swag init -g apps/operation-api/cmd/server/main.go -o apps/operation-api/docs --parseDependency --parseInternal

.PHONY: swag-check
swag-check: ## Fail when committed swagger docs are out of date
	@set -eu; \
	tmp=$$(mktemp -d); \
	trap 'rm -rf "$$tmp"' EXIT; \
	swag init -g apps/operation-api/cmd/server/main.go -o "$$tmp/docs" --parseDependency --parseInternal >/dev/null; \
	diff -r "$$tmp/docs" apps/operation-api/docs

.PHONY: web-install
web-install: ## Install frontend dependencies
	cd $(FRONTEND_DIR) && pnpm install

.PHONY: web-test
web-test: ## Run frontend tests
	cd $(FRONTEND_DIR) && pnpm test

.PHONY: web-build
web-build: ## Build frontend
	cd $(FRONTEND_DIR) && pnpm build

.PHONY: web-lint
web-lint: ## Type-check frontend packages
	cd $(FRONTEND_DIR) && pnpm lint

.PHONY: migrate-up
migrate-up: ## Apply operation API migrations with golang-migrate
	migrate -path apps/operation-api/migrations -database "$(DATABASE_URL)" up

.PHONY: migrate-down
migrate-down: ## Roll back one operation API migration with golang-migrate
	migrate -path apps/operation-api/migrations -database "$(DATABASE_URL)" down 1

.PHONY: docker-up
docker-up: ## Start local stack
	docker compose -f deployments/docker/docker-compose.yml up --build

.PHONY: docker-down
docker-down: ## Stop the local stack
	docker compose -f deployments/docker/docker-compose.yml down
