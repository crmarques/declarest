# DeclaREST

<p align="center">
  <img src="assets/logo.png" alt="DeclaREST logo" width="200">
</p>

Managing API configurations usually means scripts, manual UI clicks, and changes nobody can review or reproduce. DeclaREST replaces that with a simple model: store what you want in Git files, and DeclaREST keeps your systems in sync.

## How it works

1. **Save** a resource from any REST API into a local file.
2. **Edit** the file to match what you want.
3. **Apply** the change back — DeclaREST handles the diff and the HTTP calls.

That’s it. Your configuration lives in Git, with full history, pull requests, and repeatable automation.

## What makes it different

- **Stable logical paths** — use `/corporations/acme` instead of raw endpoint URLs
- **Git as source of truth** — review changes before they hit the API
- **Metadata-driven adaptation** — works with APIs that aren’t clean REST
- **Secret placeholders** — no plaintext credentials in your repository
- **Two modes** — CLI for on-demand work, Kubernetes Operator for continuous sync

## Usage flow

<p align="center">
  <img src="assets/usage-flow.png" alt="Usage flow" width="650">
</p>

## Interacting with APIs through DeclaREST CLI

```bash
declarest resource save /corporations/acme # get a resource from managed server
declarest resource edit /corporations/acme # get a resource definition in the git repository
declarest resource diff /corporations/acme # see state difference from git repository and remote managed server
declarest resource apply /corporations/acme # apply new desired state to managed server
```

## Using Kubernetes Operator

Use the quickstart to install CRDs, create `ResourceRepository`, `ManagedServer`, `SecretStore`, and `SyncPolicy`, then verify reconciliation:

- [Quickstart](getting-started/quickstart-operator.md)

## Capabilities snapshot

- **Resource workflows:** read/list/save/diff/explain/apply/create/update/delete/edit/copy
- **Metadata workflows:** infer/render/set/resolve overrides for custom API shapes
- **Secret workflows:** detect/store/mask/resolve with save-time safeguards
- **Repository workflows:** status/tree/history/commit/refresh/push/reset/clean
- **Kubernetes operator workflows:** multi-CRD reconciliation (`ResourceRepository`, `ManagedServer`, `SecretStore`, `SyncPolicy`)

## When it fits

Use DeclaREST when you need:

- Git review and history for API configuration
- less standing administrative access without taking autonomy away from the teams who own the changes
- stronger auditability for who changed what, when, and why across API-backed configuration
- repeatable automation across environments
- one model for both local debugging and continuous sync
- support for APIs that need mapping logic

## When it may not fit

DeclaREST is usually not the best fit when:

- resource state changes are highly ephemeral and not worth storing in Git
- your workflow requires direct imperative-only changes with no desired-state source

## Start here

- [Quickstart (CLI)](getting-started/quickstart.md) — includes CLI installation and first-run setup
- [Quickstart (Operator - recommended)](getting-started/quickstart-operator.md)
- [Troubleshooting](getting-started/troubleshooting.md)

## For advanced API modeling

When your API uses inconsistent paths, mixed collections, or per-operation payload quirks:

- [Metadata overview](concepts/metadata.md)
- [Metadata Overrides](concepts/metadata-overrides.md)
- [Custom Paths](concepts/metadata-custom-paths.md)
- [Advanced Metadata Configuration](workflows/advanced-metadata-configuration.md)
