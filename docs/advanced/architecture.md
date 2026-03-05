# Architecture deep dive

This page describes how DeclaREST components fit together.

## High-level data flow

```text
CLI or Operator
  -> Context resolution
  -> Repository + Metadata + SecretStore + ManagedServer clients
  -> Orchestrator workflows (save/diff/apply/list/etc.)
```

The orchestrator is the coordination boundary that composes repository, metadata, managed-server, and secret operations.

## Core runtime components

- Context/config: selects repository, managed server, metadata, secret store.
- Repository layer: persists desired-state files and metadata overrides.
- Metadata layer: resolves logical-path mapping and request behavior.
- Managed-server client: executes remote API operations.
- Secret provider: stores/resolves placeholders.
- Orchestrator: applies workflow policy and deterministic fallbacks.

## Determinism principles

- normalized absolute logical paths
- metadata-first identity resolution
- bounded fallback behavior (no unbounded search)
- stable ordering for list/diff output where possible

## Execution modes

- CLI mode: explicit user/CI invocation.
- Operator mode: Kubernetes reconciliation loop around the same core workflows.

## Extension points

- metadata overrides for API-specific path/method/payload adaptation
- repository backend choice (`filesystem` or `git`)
- secret backend choice (`file` or `vault`)
- operator CRD composition for continuous sync

## Common architecture tradeoffs

- More metadata flexibility increases modeling power but also review complexity.
- Operator mode improves convergence but adds cluster/runtime operations.
- Git backend adds auditability and promotion flows, with extra auth/network dependencies.
