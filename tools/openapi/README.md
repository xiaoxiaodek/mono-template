# OpenAPI Tooling

`contracts/openapi/control-api.yaml` is the editable source contract. Keep it aligned with the backend routes and transport DTOs, and run all commands from the repository root.

## Lint

Run the Redocly CLI pinned by `frontend/package.json` and
`frontend/pnpm-lock.yaml` through the Make target:

```sh
make openapi-lint
```

The equivalent default command is:

```sh
cd frontend && env CI=true pnpm exec redocly lint ../contracts/openapi/control-api.yaml
```

It parses the YAML, resolves references, and exits non-zero for contract violations. CI should run this target for every OpenAPI change. To use an already-installed or alternative compatible linter, override the command, for example:

```sh
make openapi-lint OPENAPI_LINT_CMD='redocly lint'
```

## Generate TypeScript types

Run the `openapi-typescript` version pinned by the frontend workspace:

```sh
make openapi-generate
```

The equivalent default command is:

```sh
cd frontend && env CI=true pnpm exec openapi-typescript ../contracts/openapi/control-api.yaml --output packages/api-client/src/generated/control-api.ts
```

The generated output is `frontend/packages/api-client/src/generated/control-api.ts`. Override either the tool command or output path when needed:

```sh
make openapi-generate OPENAPI_GENERATE_CMD='openapi-typescript' OPENAPI_GENERATED_FILE=/tmp/control-api.ts
```

`OPENAPI_FILE` and `OPENAPI_GENERATED_FILE` accept repository-relative or
absolute paths, including paths that contain spaces. The Make targets resolve
those paths before changing into the frontend workspace.

## Contract and generated-file policy

- Edit `contracts/openapi/control-api.yaml` when the API contract changes.
- Update backend routes, transport DTOs, the YAML contract, and the frontend client together.
- Commit `frontend/packages/api-client/src/generated/control-api.ts` only after the client package formally adopts that generated artifact. Until then, generate to a temporary path for type validation; the default TypeScript output is not required to be committed.
- Never hand-edit generated TypeScript; change the YAML contract and regenerate.
- Contract changes must run OpenAPI linting, then frontend lint, tests, and build.
- CI drift checks should run generation and fail when `git diff --exit-code -- frontend/packages/api-client/src/generated/control-api.ts` is non-zero.

## Recommended checks

Run an OpenAPI-related change in this order:

```sh
make openapi-lint
make openapi-generate OPENAPI_GENERATED_FILE=/private/tmp/control-api.ts
cd frontend && pnpm --filter @vort-ads/api-client lint
cd frontend && pnpm --filter @vort-ads/api-client test
```

After the client adopts a tracked generated file, CI can additionally run `git diff --exit-code -- frontend/packages/api-client/src/generated/control-api.ts`. Install the locked frontend workspace dependencies before running these targets. Offline environments can then execute the tools from the pnpm store without
resolving a fresh dependency graph. The command variables remain overridable for
compatible preinstalled binaries.
