# Architecture

> This page is for contributors and power users who want to understand DeclaREST internals. For daily usage, the [Guide](../guide/core-concepts.md) section is sufficient.

## High-level data flow

```text
                    ┌───────────────────────────┐
                    │   CLI  or  K8s Operator    │
                    └─────────────┬─────────────┘
                                  │
                    ┌─────────────▼─────────────┐
                    │    Context Resolution      │
                    │  (select repo, server,     │
                    │   metadata, secret store)  │
                    └─────────────┬─────────────┘
                                  │
          ┌───────────┬───────────┼───────────┬───────────┐
          ▼           ▼           ▼           ▼           ▼
    ┌──────────┐ ┌──────────┐ ┌──────────┐ ┌──────────┐ ┌──────────┐
    │Repository│ │ Metadata │ │ Managed  │ │ Secret   │ │Completion│
    │  Store   │ │ Service  │ │ Server   │ │ Provider │ │ Service  │
    └────┬─────┘ └────┬─────┘ └────┬─────┘ └────┬─────┘ └──────────┘
         │            │            │             │
         └────────────┴─────┬──────┴─────────────┘
                            ▼
                    ┌───────────────┐
                    │  Orchestrator │
                    │  (save, diff, │
                    │  apply, list) │
                    └───────────────┘
```

The orchestrator is the coordination boundary. It composes repository, metadata, managed-service, and secret operations into deterministic workflows. CLI commands and operator controllers both delegate to the orchestrator -- they never call providers directly.

## Layer model

```text
┌─────────────────────────────────────────────────┐
│  cmd/              Entry points (CLI, Operator)  │
├─────────────────────────────────────────────────┤
│  internal/cli/     CLI commands and UX logic     │
├─────────────────────────────────────────────────┤
│  internal/app/     Application workflows         │
├─────────────────────────────────────────────────┤
│  Domain packages   Public contracts and types    │
│  (orchestrator/, resource/, metadata/,           │
│   repository/, secrets/, managedservice/,          │
│   config/, faults/)                              │
├─────────────────────────────────────────────────┤
│  internal/providers/   Concrete implementations  │
│  (git, filesystem, vault, file, http, bundle)    │
├─────────────────────────────────────────────────┤
│  internal/bootstrap/   Composition root          │
│  (wires providers to interfaces at startup)      │
└─────────────────────────────────────────────────┘
```

**Dependency rules:**

- Domain packages define interfaces; providers implement them.
- CLI layer depends on domain packages but never on providers.
- App layer depends on domain packages but never on CLI or providers.
- Providers never import sibling providers.
- Bootstrap is the only place that knows about concrete implementations.

These rules are enforced by boundary tests that parse Go imports and fail on violations.

## Core runtime components

| Component | Package | Responsibility |
|-----------|---------|----------------|
| Context/config | `config/` | Selects repository, managed service, metadata, and secret store from a named context |
| Repository | `repository/` | Persists desired-state files and metadata overrides (filesystem or git backend) |
| Metadata | `metadata/` | Resolves logical-path-to-API mapping, layered overrides, template rendering |
| Managed service | `managedservice/` | Executes remote HTTP operations (CRUD/list with auth, TLS, throttling) |
| Secret provider | `secrets/` | Stores, masks, and resolves secret placeholders (file or Vault backend) |
| Orchestrator | `orchestrator/` | Coordinates multi-step workflows (save, diff, apply, list) with deterministic fallbacks |

## How the orchestrator coordinates workflows

A typical `resource apply` flow:

1. **Load local** -- read desired-state payload from the repository.
2. **Resolve metadata** -- merge metadata layers to determine operation specs.
3. **Resolve secrets** -- expand {% raw %}`{{secret .}}`{% endraw %} placeholders in the payload.
4. **Read remote** -- fetch current state from the managed service.
5. **Compare** -- diff desired vs actual using metadata compare transforms.
6. **Decide** -- skip if no drift (unless `--force`), create if remote returns `NotFound`, update otherwise.
7. **Mutate** -- send the create/update request with metadata-rendered headers, path, and body transforms.

Each step is bounded: no unbounded retries, no cascading searches. If a step fails, a typed error is returned immediately.

## Determinism principles

- **Normalized paths:** all logical paths are absolute and normalized before any I/O.
- **Metadata-first identity:** resource identity resolves through metadata `resource.id`/`resource.alias` templates before raw API response.
- **Bounded fallback:** read fallbacks (literal -> collection list/filter) are bounded by one level.
- **Stable ordering:** list and diff outputs use deterministic ordering for equivalent inputs.
- **Typed errors:** every error path maps to a specific category (`ValidationError`, `NotFoundError`, `ConflictError`, `AuthError`, `TransportError`, `InternalError`), which maps to a deterministic CLI exit code.

## Execution modes

Both CLI and Operator share the same orchestrator and provider implementations -- the only difference is the entry point and trigger model.

- **CLI**: explicit user or CI invocation. Each command maps to one orchestrator workflow.
- **Operator**: Kubernetes reconciliation loop. Four CRDs define desired configuration; the controller reconciles through the same save/diff/apply cycle.

## Extension points

- **Metadata overrides** for API-specific path, method, and payload adaptation without code changes.
- **Repository backend** choice (`filesystem` or `git`) based on workflow needs.
- **Secret backend** choice (`file` with local encryption or `vault` with HashiCorp Vault).
- **Metadata bundles** for distributing reusable metadata packages across teams.
- **Operator CRD composition** for multi-server, multi-repository continuous sync topologies.

## Architecture tradeoffs

- More metadata flexibility increases modeling power but also review complexity. Start with defaults and override only what you need.
- Operator mode improves convergence and reduces manual work, but adds cluster runtime dependencies.
- Git backend adds auditability and promotion flows, but requires network access and auth configuration.
- Bundle metadata simplifies distribution but adds a caching and versioning layer.
