GO_MODULE_DIR := apps
FRONTEND_DIR := frontend
GOLANGCI_LINT ?= golangci-lint
OPENAPI_FILE ?= contracts/openapi/control-api.yaml
OPENAPI_GENERATED_FILE ?= frontend/packages/api-client/src/generated/control-api.ts
OPENAPI_LINT_CMD ?= env CI=true pnpm exec redocly lint
OPENAPI_GENERATE_CMD ?= env CI=true pnpm exec openapi-typescript

.PHONY: help
help:
	@grep -E '^[a-zA-Z_-]+:.*?## ' $(MAKEFILE_LIST) | awk 'BEGIN {FS = ":.*?## "}; {printf "%-20s %s\n", $$1, $$2}'

.PHONY: go-test
go-test: ## Run backend tests
	cd $(GO_MODULE_DIR) && go test ./...

.PHONY: go-build
go-build: ## Build backend binaries
	cd $(GO_MODULE_DIR) && go build ./control-api/cmd/server ./worker/cmd/worker

.PHONY: go-vet
go-vet: ## Run backend static analysis
	cd $(GO_MODULE_DIR) && go vet ./...

.PHONY: go-fmt
go-fmt: ## Format backend code
	cd $(GO_MODULE_DIR) && gofmt -w $$(find . -name '*.go' -not -path './vendor/*')

.PHONY: run-api
run-api: ## Run the control API from source
	cd $(GO_MODULE_DIR) && go run ./control-api/cmd/server

.PHONY: run-worker
run-worker: ## Run the worker scaffold from source
	cd $(GO_MODULE_DIR) && go run ./worker/cmd/worker

.PHONY: go-lint
go-lint: ## Lint backend code
	cd $(GO_MODULE_DIR) && $(GOLANGCI_LINT) run --config ../.golangci.yml ./...

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

.PHONY: openapi-lint
openapi-lint: ## Lint the OpenAPI source contract
	@set -eu; \
	openapi_file="$(OPENAPI_FILE)"; \
	case "$$openapi_file" in /*) ;; *) openapi_file="$$PWD/$$openapi_file" ;; esac; \
	openapi_file_dir=$$(dirname "$$openapi_file"); \
	openapi_file_name=$$(basename "$$openapi_file"); \
	openapi_file="$$(cd "$$openapi_file_dir" && pwd -P)/$$openapi_file_name"; \
	cd "$(FRONTEND_DIR)" && $(OPENAPI_LINT_CMD) "$$openapi_file"

.PHONY: openapi-generate
openapi-generate: ## Generate TypeScript types from the OpenAPI contract
	@set -eu; \
	openapi_file="$(OPENAPI_FILE)"; \
	case "$$openapi_file" in /*) ;; *) openapi_file="$$PWD/$$openapi_file" ;; esac; \
	openapi_file_dir=$$(dirname "$$openapi_file"); \
	openapi_file_name=$$(basename "$$openapi_file"); \
	openapi_file="$$(cd "$$openapi_file_dir" && pwd -P)/$$openapi_file_name"; \
	generated_file="$(OPENAPI_GENERATED_FILE)"; \
	case "$$generated_file" in /*) ;; *) generated_file="$$PWD/$$generated_file" ;; esac; \
	generated_file_dir=$$(dirname "$$generated_file"); \
	generated_file_name=$$(basename "$$generated_file"); \
	mkdir -p "$$generated_file_dir"; \
	generated_file="$$(cd "$$generated_file_dir" && pwd -P)/$$generated_file_name"; \
	cd "$(FRONTEND_DIR)" && $(OPENAPI_GENERATE_CMD) "$$openapi_file" --output "$$generated_file"

.PHONY: migrate-up
migrate-up: ## Apply control API migrations with golang-migrate
	migrate -path apps/control-api/migrations -database "$(DATABASE_URL)" up

.PHONY: migrate-down
migrate-down: ## Roll back one control API migration with golang-migrate
	migrate -path apps/control-api/migrations -database "$(DATABASE_URL)" down 1

.PHONY: docker-up
docker-up: ## Start local stack
	docker compose -f deployments/docker/docker-compose.yml up --build

.PHONY: docker-down
docker-down: ## Stop the local stack
	docker compose -f deployments/docker/docker-compose.yml down
