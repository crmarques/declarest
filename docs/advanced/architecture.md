# Architecture deep dive

> This section is for users who want to understand DeclaREST internals or contribute to the project. For daily usage, the [Concepts](../concepts/overview.md) and [How-to](../workflows/sync.md) sections are sufficient.

This page describes how DeclaREST components fit together, how data flows through the system, and what design choices shape the implementation.

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

The orchestrator is the coordination boundary. It composes repository, metadata, managed-server, and secret operations into deterministic workflows. CLI commands and operator controllers both delegate to the orchestrator — they never call providers directly.

## Layer model

DeclaREST follows a strict layered architecture with explicit dependency rules:

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
│   repository/, secrets/, managedserver/,          │
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
- Bootstrap is the only place that knows about concrete implementations — it wires everything together at startup.

These rules are enforced by boundary tests that parse Go imports and fail on violations.

## Core runtime components

| Component | Package | Responsibility |
|-----------|---------|----------------|
| Context/config | `config/` | Selects repository, managed server, metadata, and secret store from a named context |
| Repository | `repository/` | Persists desired-state files and metadata overrides (filesystem or git backend) |
| Metadata | `metadata/` | Resolves logical-path-to-API mapping, layered overrides, template rendering |
| Managed server | `managedserver/` | Executes remote HTTP operations (CRUD/list with auth, TLS, throttling) |
| Secret provider | `secrets/` | Stores, masks, and resolves secret placeholders (file or Vault backend) |
| Orchestrator | `orchestrator/` | Coordinates multi-step workflows (save, diff, apply, list) with deterministic fallbacks |

## How the orchestrator coordinates workflows

A typical `resource apply` flow:

1. **Load local** — read desired-state payload from the repository.
2. **Resolve metadata** — merge metadata layers to determine operation specs.
3. **Resolve secrets** — expand `{{secret .}}` placeholders in the payload.
4. **Read remote** — fetch current state from the managed server.
5. **Compare** — diff desired vs actual using metadata compare transforms.
6. **Decide** — skip if no drift (unless `--force`), create if remote returns `NotFound`, update otherwise.
7. **Mutate** — send the create/update request with metadata-rendered headers, path, and body transforms.

Each step is bounded: no unbounded retries, no cascading searches. If a step fails, a typed error is returned immediately.

## Determinism principles

- **Normalized paths:** all logical paths are absolute and normalized before any I/O.
- **Metadata-first identity:** resource identity resolves through metadata `idAttribute`/`aliasAttribute` before raw API response.
- **Bounded fallback:** read fallbacks (literal -> collection list/filter) are always bounded by one level — no recursive search.
- **Stable ordering:** list and diff outputs use deterministic ordering for equivalent inputs.
- **Typed errors:** every error path maps to a specific category (`ValidationError`, `NotFoundError`, `ConflictError`, `AuthError`, `TransportError`, `InternalError`), which maps to a deterministic CLI exit code.

## Execution modes

### CLI mode

Explicit user or CI invocation. Each command maps to one orchestrator workflow. The CLI layer handles argument parsing, output formatting, and status reporting. Best for on-demand tasks, scripting, and CI pipelines.

### Operator mode

A Kubernetes reconciliation loop around the same orchestrator workflows. Four CRDs (`ResourceRepository`, `ManagedServer`, `SecretStore`, `SyncPolicy`) define the desired configuration. The operator controller watches for changes and reconciles through the standard save/diff/apply cycle.

Both modes share the same orchestrator and provider implementations — the only difference is the entry point and trigger model.

## Extension points

- **Metadata overrides** for API-specific path, method, and payload adaptation without code changes.
- **Repository backend** choice (`filesystem` or `git`) based on workflow needs.
- **Secret backend** choice (`file` with local encryption or `vault` with HashiCorp Vault).
- **Metadata bundles** for distributing reusable metadata packages across teams.
- **Operator CRD composition** for multi-server, multi-repository continuous sync topologies.

## Common architecture tradeoffs

- More metadata flexibility increases modeling power but also review complexity. Start with defaults and override only what you need.
- Operator mode improves convergence and reduces manual work, but adds cluster runtime dependencies.
- Git backend adds auditability and promotion flows (PRs, branches), but requires network access and auth configuration.
- Bundle metadata simplifies distribution but adds a caching and versioning layer.
