# Operator model

The DeclaREST Operator runs a reconciliation loop inside Kubernetes.

It uses four CRDs:

- `ResourceRepository`
- `ManagedServer`
- `SecretStore`
- `SyncPolicy`

## Reconciliation loop

At a high level:

1. Fetch latest desired state from the configured Git repository.
2. Resolve dependencies (`ManagedServer`, `SecretStore`).
3. Build a sync plan for the `SyncPolicy.source.path` scope.
4. Apply/prune toward desired state.
5. Update status conditions and resource stats.

## Drift handling

Drift means real state differs from desired state in Git.

- Default behavior: apply only when drift exists.
- Forced behavior: `spec.sync.force: true` triggers update calls even without detected drift.

## Intervals and triggers

`SyncPolicy` reconciles on:

- `spec.syncInterval` (default `5m`)
- dependency updates (repository/server/secret-store)
- referenced Kubernetes Secret version changes
- repository updates (polling and webhook-driven refresh when configured)

Optional `fullResyncCron` adds periodic full sync runs.

## Common failure modes

- Invalid spec (auth/one-of/required fields)
- Missing referenced dependency resources
- Git fetch/auth failures
- Managed API auth/transport failures
- apply/prune execution errors

All of these surface through CR status conditions and controller logs.

## Why Operator mode is recommended

It gives continuous convergence from Git desired state, instead of relying on manual CLI execution windows.
