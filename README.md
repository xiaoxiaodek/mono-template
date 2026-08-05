# Vort Ads Template

Full-stack ads-platform template with a Go/Gin control API, React/Ant Design admin shell, OpenAPI contracts, and deployment scaffolding.

## Layout

- `apps/`: Go backend module and deployable services.
- `frontend/`: pnpm workspace for React apps and shared frontend packages.
- `contracts/`: OpenAPI contracts.
- `deployments/`: Docker and Kubernetes templates.
- `docs/`: architecture and operations notes.
- `tests/`: cross-service tests and load-test scripts.
- `tools/`: local tooling docs and helpers.

## Quick Start

```bash
make go-test
make go-build
```

Frontend dependencies require `pnpm`:

```bash
make web-install
make web-test
make web-build
```
