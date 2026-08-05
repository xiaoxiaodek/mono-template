# Vort Ads Template

Full-stack Go/Gin operation API with React/Ant Design web frontend, Swagger docs,
and deployment scaffolding.

## Layout

- `internal/`: shared framework code (middleware, platform infrastructure)
- `pkg/`: reusable utility libraries (idgen, pagination, validator)
- `apps/operation-api/`: deployable API service
  - `cmd/server/`: entrypoint
  - `internal/bootstrap/`: dependency wiring
  - `internal/biz/`: business logic (Clean Architecture use cases)
  - `internal/data/`: data access (Postgres, Redis, in-memory adapters)
  - `internal/server/`: HTTP router and middleware composition
  - `internal/service/`: HTTP handlers and DTOs
  - `migrations/`: database migrations
  - `docs/`: Swagger 2.0 docs generated from Go annotations
- `configs/`: per-environment YAML configuration
- `deployments/`: Docker Compose and Kubernetes manifests
- `frontend/`: pnpm workspace (operation-web app, api-client package)
- `tests/`: cross-service and load-test scripts

## Quick Start

```bash
make go-test
make go-build
make run-api
```

Frontend dependencies require `pnpm`:

```bash
make web-install
make web-test
make web-build
```

Generate API documentation:

```bash
make swag-init
# dev/test: http://localhost:8080/docs/index.html
```
