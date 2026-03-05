# Operator deep dive

This page expands the operator runtime model beyond quickstart usage.

## Reconcile responsibility by CRD

- `ResourceRepository`: fetch desired-state revision and maintain local checkout state.
- `ManagedServer`: validate/connectivity inputs and optional artifact cache paths.
- `SecretStore`: validate provider settings and resolve runtime storage path details.
- `SyncPolicy`: plan and execute full/incremental reconcile for one source scope.

## Planning model (full vs incremental)

`SyncPolicy` decides between:

- incremental sync: targeted updates based on repository changes
- full sync: safe fallback when changes are broad/unknown or policy requires it

Safety-first behavior prefers full sync when diff confidence is low.

## Trigger model

Reconcile is driven by:

- sync interval
- optional full-resync cron
- dependency generation changes
- referenced Secret version hash changes
- repository refresh events (poll/webhook)

## Status model

Track these for operations/debugging:

- `status.lastAttemptedRepoRevision`
- `status.lastAppliedRepoRevision`
- `status.lastSyncMode`
- `status.resourceStats.{targeted,applied,pruned,failed}`
- standard conditions (`Ready`, `Reconciling`, `Stalled`)

## Failure analysis sequence

1. `kubectl describe` the relevant CR.
2. Inspect condition reason/message.
3. Check controller logs for root cause.
4. Verify referenced Secrets and dependency refs.
5. Confirm repository branch/revision movement.

## Operational notes

- Keep source scopes non-overlapping across `SyncPolicy` objects.
- Use webhook-triggered refresh when low-latency response to Git push is needed.
- Keep sync scope narrow for faster, safer reconciles.
