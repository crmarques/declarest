# Concepts Overview

DeclaREST provides a consistent model for managing REST API resources as declarative files.

- You define desired state in repository files.
- DeclaREST maps logical paths to real API operations through metadata.
- You reconcile desired state with real state using the CLI or the Operator.

## Two ways to run DeclaREST

### CLI mode (on demand or CI)

Use CLI commands when you want explicit execution points:

- `resource save` to import remote state into the repository
- `resource diff` to compare desired vs real state
- `resource apply` to reconcile one path or collection

This works well for local workflows, CI pipelines, and controlled rollouts.

### Operator mode (continuous reconciliation)

Use the Kubernetes Operator when you want continuous GitOps sync:

- Git repository is the desired-state source of truth
- Operator reconciles `SyncPolicy` targets against Managed Servers
- Drift is corrected continuously based on declared state

This is the recommended runtime model for ongoing environments.

## Mental model

Think in this order:

1. Choose stable logical paths for humans and Git history.
2. Store desired payloads in repository files under those paths.
3. Use metadata to adapt logical paths to real API endpoints/methods/payloads.
4. Reconcile desired state to real state (CLI runs or Operator loop).

## Core building blocks

### Context

A named configuration that combines:

- repository backend (`filesystem` or `git`)
- managed server config (`managed-server`: `base-url`, auth, optional `health-check` and OpenAPI)
- optional secret store
- optional metadata source

### Resource

A logical object stored locally as:

- `resource.json`, or
- `resource.yaml` when `repository.resource-format: yaml`

### Collection

A logical grouping of resources, identified by a trailing slash:

- resource: `/corporations/acme`
- collection: `/customers/`

### Metadata

Directives that control identity mapping, endpoint mapping, HTTP behavior, transforms, compare normalization, and secret-marked attributes.

## Typical lifecycle (CLI)

```bash
# Pull remote state into repository
declarest resource save /corporations/acme

# Review drift
declarest resource diff /corporations/acme

# Reconcile desired state
declarest resource apply /corporations/acme
```

## Typical lifecycle (Operator)

1. Admin updates desired state in Git (often with CLI + pull request).
2. Commit is merged/pushed.
3. Operator reconciles `SyncPolicy` scope to the Managed Server.
4. Status/conditions report success or failure.

## Where advanced APIs fit

Many APIs are not clean REST: mixed collections, nonstandard paths, and operation-specific payload quirks. DeclaREST handles this through metadata without forcing users to adopt API-specific scripts.

Start with:

- [Git repository as source of truth](git-source-of-truth.md)
- [Operator model](operator.md)
- [CLI role in GitOps](cli-in-gitops.md)
- [Contexts](context.md)
- [Paths and Selectors](paths-and-selectors.md)
