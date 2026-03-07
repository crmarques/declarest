# Kubernetes Operator and Reconcile Contracts

## Purpose
Define the Kubernetes operator contract for CRD validation, controller reconciliation, webhook-triggered refresh, and runtime context assembly used to execute DeclaREST sync workflows.

## In Scope
1. CRD spec/default/validation and status-condition contracts.
2. Controller reconcile responsibilities for `ResourceRepository`, `ManagedServer`, `SecretStore`, and `SyncPolicy`.
3. Sync planning and scheduling behavior (full vs incremental sync, prune, cron).
4. Repository webhook receiver contract and authentication rules.
5. Operator runtime context mapping into canonical DeclaREST interfaces.

## Out of Scope
1. CLI command semantics and UX output contracts.
2. E2E runner orchestration details (profile steps, runtime bootstrap scripts).
3. Kubernetes platform sizing/capacity tuning beyond operator behavior contracts.

## Normative Rules
1. The operator MUST register and reconcile `declarest.io/v1alpha1` resources: `ResourceRepository`, `ManagedServer`, `SecretStore`, and `SyncPolicy`.
2. Admission validation MUST apply resource defaults before `ValidateSpec()` checks on create/update, and MUST block deletes of dependency resources that are referenced by non-deleting `SyncPolicy` objects in the same namespace.
3. `SyncPolicy` admission and reconcile validation MUST reject overlapping logical source scopes regardless of dependency-reference equality.
4. Controllers MUST add finalizer `declarest.io/cleanup` and MUST remove it only after controller-owned cleanup is complete.
5. `ResourceRepository` reconcile MUST ensure configured storage availability, perform authenticated git sync against the configured branch, update `status.lastFetchedRevision` and `status.lastFetchedTime`, and set `Ready`/`Stalled` conditions deterministically.
6. `ManagedServer` reconcile MUST validate auth/proxy/throttling constraints, cache remote OpenAPI/metadata artifacts when configured, and persist cache paths in status without leaking secret values.
7. `SecretStore` reconcile MUST enforce provider one-of constraints (`vault` or `file`), ensure file-backed storage dependencies when required, and set `status.resolvedPath` only for file-backed stores.
8. `SyncPolicy` reconcile MUST validate referenced dependency resources, compute a secret-version hash from referenced Secret `resourceVersion` values, and trigger full sync when generation, secret hash, or full-resync schedule requires it.
9. Incremental sync planning MUST be deterministic, repository-diff based, and safety-biased; unknown/unsupported repository path changes MUST fall back to full sync.
10. `SyncPolicy` apply execution MUST invoke DeclaREST mutation workflows through `orchestrator.Orchestrator`, honor `spec.sync.force` and `spec.sync.prune`, and update status stats (`targeted`, `applied`, `pruned`, `failed`) from executed operations.
11. `SyncPolicy` scheduling MUST requeue by the earliest due trigger between `spec.syncInterval` and `spec.fullResyncCron` (when configured), with invalid cron expressions treated as spec-validation failures.
12. Repository webhook receiver MUST accept only authenticated provider events for configured repositories, enforce payload-size bounds, accept only push events for the configured branch, and patch webhook receipt annotations before returning success.
13. Webhook-triggered repository refresh MUST be event-driven via `ResourceRepository` annotation changes and MUST not wait for `pollInterval`.

## Data Contracts
1. Condition types: `Ready`, `Reconciling`, `Stalled` (from `api/v1alpha1`).
2. Finalizer: `declarest.io/cleanup`.
3. Repository webhook endpoint path: `/webhooks/repository/<namespace>/<repository>`; when `watch-namespace` is set, single-segment `<repository>` form MAY be accepted and resolves to that namespace.
4. Repository webhook annotation `declarest.io/webhook-last-received-at` stores the last accepted provider event timestamp (`RFC3339Nano`).
5. Repository webhook annotation `declarest.io/webhook-last-event-id` stores provider event identifiers when present.
6. `ResourceRepository` defaults: `spec.git.branch=main` when omitted; `spec.resourceFormat=json` when omitted, where `resourceFormat` defines the default payload type for new resources and allowed values are `json|yaml|xml|hcl|ini|properties|text|octet-stream`.
7. `ManagedServer` defaults: `spec.http.auth.oauth2.grantType=client_credentials` when omitted; `spec.pollInterval=10m` when omitted.
8. `SyncPolicy` defaults: `spec.source.recursive=true` and `spec.syncInterval=5m` when omitted.
9. `SyncPolicy` reconcile runtime MUST assemble a `config.Context` and bootstrap a session using `bootstrap.NewSessionFromResolvedContext`, yielding canonical interface implementations (`orchestrator.Orchestrator`, `repository.ResourceStore`, `metadata.MetadataService`, `secrets.SecretProvider`) for mutation workflows.
10. Sync execution plan modes: `full` or `incremental`; plan targets MUST be normalized and deduplicated.

## Failure Modes
1. Spec validation failure (one-of/auth/path/cron/poll interval invariants) marks resource `NotReady` with reason `SpecInvalid`.
2. Missing or invalid referenced dependency resources mark `SyncPolicy` `NotReady` with reason `DependencyInvalid`.
3. Repository unavailable, session bootstrap failure, or apply/prune errors mark `SyncPolicy` `NotReady` with reconcile-failure reasons.
4. Webhook authentication/signature/token mismatch returns authorization failure and MUST NOT mutate repository annotations.
5. Oversized webhook payloads or malformed target paths return request errors and MUST NOT enqueue repository refresh.

## Edge Cases
1. Secret rotation with unchanged repository revision MUST still trigger `SyncPolicy` reconcile via secret-version-hash change.
2. Metadata-only repository changes under collection metadata (`.../_/metadata.*`) SHOULD resolve to recursive apply targets for affected scope.
3. Branch-mismatched push webhook events MUST be acknowledged and ignored without status mutation.
4. `spec.fullResyncCron` with no previous full sync MUST schedule immediate full sync eligibility.
5. Non-overlapping source paths with shared dependency references MUST be accepted.

## Examples
1. Normal: `ResourceRepository` receives a valid authenticated push webhook for the tracked branch, webhook annotations are patched, and reconcile fetches a new revision before the next `pollInterval`.
2. Corner: `SyncPolicy` references valid dependencies and unchanged revision, but a referenced Secret `resourceVersion` changes; reconcile runs full mode and reapplies scoped resources even without repository diff changes.

## Verification Expectations
1. CRD defaulting/validation and webhook admission checks MUST be covered by `api/v1alpha1/*_types_test.go` and `api/v1alpha1/webhook_test.go`.
2. Webhook authentication, branch filtering, and annotation patch behavior MUST be covered by `internal/operator/controllers/repository_webhook_server_test.go`.
3. Incremental/full sync planning and safe full-fallback paths MUST be covered by `internal/operator/controllers/syncpolicy_plan_test.go`.
4. Full-resync schedule and requeue interval computation MUST be covered by `internal/operator/controllers/syncpolicy_schedule_test.go`.
5. Path-overlap validation and dependency-sharing behavior MUST be covered by `internal/operator/controllers/syncpolicy_controller_test.go`.
