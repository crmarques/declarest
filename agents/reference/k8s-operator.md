# Kubernetes Operator and Reconcile Contracts

## Purpose
Define the operator contract for CRD validation, controller reconciliation, sync planning, webhook-triggered refresh, runtime context assembly, and OLM packaging used to execute DeclaREST sync workflows.

## Scope
Operator behavior contracts only. CLI semantics (cli.md), E2E orchestration (e2e.md), and cluster sizing are out of scope. `${ENV_VAR}` expansion semantics are defined in context-config.md; defaults-artifact layout in resource-repo.md; orchestrator flows in orchestrator.md; bundle resolution/`bundle.yaml` in metadata-bundle.md — this file enforces them at the operator boundary.

## Normative Rules

### CRD admission and validation
1. The operator MUST register and reconcile `declarest.io/v1alpha1` kinds: `ResourceRepository`, `ManagedService`, `SecretStore`, `SyncPolicy`, `MetadataBundle`.
2. Admission MUST apply resource defaults before `ValidateSpec()` on create/update, and MUST block deletes of dependency resources still referenced by non-deleting `SyncPolicy` objects in the same namespace.
3. Exact-match CR string values of form `${ENV_VAR}` MUST resolve from the operator process environment (per context-config.md) before webhook validation, dependency-reference checks, overlap checks, and controller runtime use; the persisted CR spec MUST remain unchanged.
4. `SyncPolicy` admission and reconcile MUST reject overlapping logical source scopes regardless of dependency-reference equality.
5. Invalid cron in `spec.fullResyncCron` MUST be treated as a spec-validation failure.

### Reconcile per CRD
6. Controllers MUST add finalizer `declarest.io/cleanup` and MUST remove it only after controller-owned cleanup completes.
7. `ResourceRepository` reconcile MUST ensure storage availability, perform authenticated git sync against the configured branch, update `status.lastFetchedRevision` and `status.lastFetchedTime`, and set `Ready`/`Stalled` deterministically.
8. `ManagedService` reconcile MUST validate auth/proxy/throttling constraints, cache configured remote OpenAPI/metadata artifacts, merge process proxy environment with configured proxy fields before downloads, and persist cache paths in status without leaking secret values.
9. `SecretStore` reconcile MUST enforce provider one-of (`vault` or `file`), ensure file-backed storage dependencies when required, and set `status.resolvedPath` only for file-backed stores.
10. `SyncPolicy` reconcile MUST validate referenced dependencies, compute a secret-version hash from referenced Secret `resourceVersion` values, and trigger full sync when generation, secret hash, or full-resync schedule requires it.
11. `SyncPolicy` apply execution MUST invoke DeclaREST mutation workflows through `orchestrator.Orchestrator` (orchestrator.md), honor `spec.sync.force` and `spec.sync.prune`, and update status stats (`targeted`, `applied`, `pruned`, `failed`) from executed operations.
12. `SyncPolicy` scheduling MUST requeue by the earliest due trigger between `spec.syncInterval` and `spec.fullResyncCron` (when configured).

### Sync planning
13. Incremental sync planning MUST be deterministic, repository-diff based, and safety-biased: unknown/unsupported repository path changes MUST fall back to full sync. Plan modes are `full` or `incremental`; plan targets MUST be normalized and deduplicated.
14. Metadata-owned defaults artifacts (`/_/defaults.<ext>`, `/_/defaults-<profile>.<ext>`, `<resource>/defaults.<ext>`, `<resource>/defaults-<profile>.<ext>`; layout per resource-repo.md) MUST resolve to the owning metadata scope, not a synthetic payload child path. Unsupported unknown files merely sharing the reserved `defaults` prefix MUST stay in the unknown/unsupported bucket and trigger full fallback.

### Webhook receiver
15. The repository webhook receiver MUST accept only authenticated provider events for configured repositories, enforce payload-size bounds and bounded read/write/idle HTTP timeouts, accept only push events for the configured branch, and patch webhook receipt annotations before returning success.
16. Webhook-triggered repository refresh MUST be event-driven via `ResourceRepository` annotation changes and MUST NOT wait for `pollInterval`.

### MetadataBundle
17. `MetadataBundle` spec MUST define exactly one of `spec.source.url`, `spec.source.pvc`, `spec.source.configMap`. `spec.source.url` MUST accept any scheme supported by `bundlemetadata.ResolveBundle` (metadata-bundle.md: `oci://`, `https://`, `http://`, `file://`, or legacy `<name>:<version>`); `${VAR}` expansion MUST run before admission. `spec.source.pullSecretRef` MUST be honored only when `url` uses `oci://` and MUST reference a same-namespace Secret of type `kubernetes.io/dockerconfigjson`. `spec.source.configMap` MUST reference a same-namespace ConfigMap whose `binaryData[key]` (or base64 `data[key]`) carries the gzipped tarball. Rotation (via `resourceVersion`) of a referenced pull Secret or ConfigMap MUST trigger reconcile.
18. `MetadataBundle` reconcile MUST forward resolved source and dependency inputs through `bootstrap.ResolveMetadataBundle`, MUST override the provider cache root when `DECLAREST_BUNDLE_CACHE_DIR` (else `DECLAREST_OPERATOR_CACHE_BASE_DIR/bundles`) is set, and MUST publish `status.cachePath`, `status.openAPIPath`, `status.manifest` without leaking Secret values into status or events.

### OLM bundle
19. The operator MUST ship an operator-sdk `registry+v1` bundle: `bundle/manifests/` MUST contain the `ClusterServiceVersion`, all owned CRDs, operator `Deployment`, RBAC, `Service` objects, and `PodDisruptionBudget`; `bundle/metadata/annotations.yaml` MUST declare `operators.operatorframework.io.bundle.package.v1=declarest-operator` with `alpha` as default channel; `bundle/tests/scorecard/config.yaml` MUST enable the default operator-sdk scorecard suite (basic + OLM tests).
20. The CSV MUST declare `installModes` supporting `OwnNamespace`, `SingleNamespace`, `AllNamespaces` and disabling `MultiNamespace`; MUST list every owned CRD (`resourcerepositories`, `managedservices`, `secretstores`, `syncpolicies`, `repositorywebhooks`) with deterministic `alm-examples` sourced from `config/samples/`; MUST declare webhook definitions mirroring `config/manifests/webhooks.yaml` on container port 9443 with `failurePolicy=Fail` and `timeoutSeconds=10`; and MUST NOT embed an `icon`.
21. The bundle Deployment MUST replace the cluster-scoped `declarest-operator-state` PVC with an `emptyDir` volume (OLM `registry+v1` forbids bundled PVCs). Kustomize release manifests (`install.yaml`, `install-admission-*.yaml`) MUST continue to ship the PVC for non-OLM installs.

### Make targets and catalog
22. `make bundle` MUST regenerate manifests from `config/manifests/` via `operator-sdk generate bundle --manifests` (never `--overwrite`), preserving hand-authored `bundle.Dockerfile`, `bundle/metadata/annotations.yaml`, and `bundle/tests/scorecard/config.yaml`. `make bundle-validate` MUST run `operator-sdk bundle validate ./bundle` with the `operatorframework` optional validator suite and fail on any reported error.
23. The file-based catalog MUST live at `catalog/declarest-operator/catalog.yaml` with `olm.package`, `olm.channel` (`alpha`, pointing at `declarest-operator.v<VERSION>`), and an `olm.bundle` entry referencing `ghcr.io/crmarques/declarest-operator-bundle:<VERSION>`. `make catalog VERSION=<VERSION>` MUST regenerate it deterministically, `catalog.Dockerfile` MUST build from `quay.io/operator-framework/opm:v1.65.0`, and `make catalog-validate` MUST run `opm validate ./catalog` successfully.
24. `make release-bundle VERSION=<VERSION>` MUST regenerate bundle and catalog for exactly that semver, MUST stamp a deterministic CSV `createdAt`, MUST validate `config/olm/`, MUST run `operator-sdk bundle validate` and `opm validate`, and MUST fail when CSV name/version, CSV manager image, CSV `containerImage` annotation, catalog bundle name/version, or catalog bundle image mismatch `VERSION`.
25. `config/olm/` MUST provide a reference install overlay (`Namespace=olm`, `OperatorGroup=declarest-operators`, `CatalogSource=declarest-catalog` targeting the published catalog image, `Subscription=declarest-operator` on channel `alpha` with `installPlanApproval=Automatic`); `make olm-install`/`make olm-uninstall` MUST apply/remove it deterministically.

### Release publishing
26. Release tooling MUST publish CLI artifacts, operator image, bundle image, catalog image, and the GitHub release through one tag-triggered workflow DAG; publishing the GitHub release MUST depend on successful operator, bundle, and catalog image publication for the same tag.
27. Release tooling MUST publish the operator image to `ghcr.io/crmarques/declarest-operator` with `v<VERSION>`, `<VERSION>`, and `latest` tags; MUST publish bundle and catalog images to `ghcr.io/crmarques/declarest-operator-bundle:<VERSION>` and `ghcr.io/crmarques/declarest-operator-catalog:<VERSION>` plus `latest`; and MUST attach the bundle tarball, rendered CSV, and rendered catalog as release assets.
28. Standalone operator-image and bundle-image workflows MAY exist only as `workflow_dispatch` smoke builds and MUST NOT publish images from tag pushes.

## Data Contracts
1. Condition types `Ready`, `Reconciling`, `Stalled`; finalizer `declarest.io/cleanup` (from `api/v1alpha1`).
2. Webhook endpoint path `/webhooks/repository/<namespace>/<repository>`; when `watch-namespace` is set, the single-segment `<repository>` form MAY be accepted and resolves to that namespace.
3. Webhook annotations: `declarest.io/webhook-last-received-at` (last accepted event timestamp, `RFC3339Nano`), `declarest.io/webhook-last-event-id` (provider event id when present).
4. Defaults: `ResourceRepository` `spec.git.branch=main` (payload file extensions are runtime-determined from managed-service responses or explicit input — no payload-format default field); `ManagedService` `spec.http.auth.oauth2.grantType=client_credentials`, `spec.pollInterval=10m`; `SyncPolicy` `spec.source.recursive=true`, `spec.syncInterval=5m`.
5. `SyncPolicy` reconcile runtime MUST assemble a `config.Context` and bootstrap a session via `bootstrap.NewSessionFromResolvedContext`, yielding canonical implementations (`orchestrator.Orchestrator`, `repository.ResourceStore`, `metadata.MetadataService`, `secrets.SecretProvider`); managed-service and vault proxy blocks MAY override only selected fields from process proxy environment.
6. OLM layout: `bundle.Dockerfile` and `catalog.Dockerfile` at repo root; bundle image tag equals released operator `VERSION` (e.g. `0.0.1`). Remaining bundle/catalog/overlay file shapes are fixed by Rules 19, 23, 25.

## Failure Modes
1. Spec validation failure (one-of/auth/path/cron/poll-interval invariants) -> resource `NotReady`, reason `SpecInvalid`.
2. Missing/invalid referenced dependency -> `SyncPolicy` `NotReady`, reason `DependencyInvalid`.
3. Repository unavailable, session bootstrap failure, or apply/prune error -> `SyncPolicy` `NotReady` with reconcile-failure reason.
4. Webhook auth/signature/token mismatch -> authorization failure; MUST NOT mutate repository annotations.
5. Oversized payload or malformed target path -> request error; MUST NOT enqueue refresh.
6. Bundle validate (`--select-optional suite=operatorframework`) or `opm validate` failure -> MUST block release-image publishing; OLM-incompatible kinds (e.g. PVC) MUST fail bundle validate before image build.

## Edge Cases
1. Secret rotation with unchanged repository revision MUST still trigger `SyncPolicy` reconcile (full mode) via secret-version-hash change.
2. Metadata-only changes under collection metadata (`.../_/metadata.*`) SHOULD resolve to recursive apply targets for the affected scope.
3. Branch-mismatched push events MUST be acknowledged and ignored without status mutation.
4. `spec.fullResyncCron` with no previous full sync MUST schedule immediate full-sync eligibility.
5. Non-overlapping source paths sharing dependency references MUST be accepted.
6. Slow/idle webhook clients MUST be disconnected by bounded HTTP timeouts.
7. OLM-installed pods MUST rely on the bundled `emptyDir` state volume and accept configuration assuming no persistent state volume; persistent state on non-OLM clusters stays available via kustomize manifests. OLM install across `AllNamespaces`/`SingleNamespace`/`OwnNamespace` MUST preserve admission-webhook behavior (OLM injects the serving cert automatically).

## Examples
1. Authenticated push for the tracked branch: webhook annotations are patched and reconcile fetches a new revision before the next `pollInterval`.
2. Valid dependencies, unchanged revision, but a referenced Secret `resourceVersion` changes: reconcile runs full mode and reapplies scoped resources without any repository diff.
3. Path classification: `customers/_/defaults.yaml` change -> incremental target `/customers`; `customers/acme/defaults-prod.yaml` change -> target `/customers/acme`; an unknown file under a resource dir merely sharing the `defaults` prefix -> safe full fallback.

## Verification Expectations
1. CRD defaulting/validation and webhook admission (incl. runtime `${ENV_VAR}` expansion for repository/server/secret-store/sync-policy fields) -> `api/v1alpha1/*_types_test.go`, `api/v1alpha1/webhook_test.go`, and controller-level tests such as `internal/operator/controllers/util_test.go`.
2. Webhook auth, branch filtering, annotation patch, timeout config -> `internal/operator/controllers/repository_webhook_server_test.go`.
3. Incremental/full planning, metadata defaults-artifact path classification, safe full-fallback -> `internal/operator/controllers/syncpolicy_plan_test.go`.
4. Full-resync schedule and requeue interval computation -> `internal/operator/controllers/syncpolicy_schedule_test.go`.
5. Path-overlap validation and dependency-sharing -> `internal/operator/controllers/syncpolicy_controller_test.go`.
6. OLM packaging changes -> `make verify-bundle` (kustomize build of `config/manifests` and `config/olm`, `operator-sdk bundle validate` with the `operatorframework` optional suite, `opm validate ./catalog`, version/image consistency checks, before/after artifact drift check); any release-workflow change touching bundle/catalog images MUST keep tag-triggered CI validation green before publish.
