# Kubernetes Operator and Reconcile Contracts

## Purpose
Define the Kubernetes operator contract for CRD validation, controller reconciliation, webhook-triggered refresh, and runtime context assembly used to execute DeclaREST sync workflows.

## In Scope
1. CRD spec/default/validation and status-condition contracts.
2. Controller reconcile responsibilities for `ResourceRepository`, `ManagedService`, `SecretStore`, and `SyncPolicy`.
3. Sync planning and scheduling behavior (full vs incremental sync, prune, cron).
4. Repository webhook receiver contract and authentication rules.
5. Operator runtime context mapping into canonical DeclaREST interfaces.
6. OLM packaging artifacts (bundle image, file-based catalog image) that install and manage the operator through OLM.

## Out of Scope
1. CLI command semantics and UX output contracts.
2. E2E runner orchestration details (profile steps, runtime bootstrap scripts).
3. Kubernetes platform sizing/capacity tuning beyond operator behavior contracts.

## Normative Rules
1. The operator MUST register and reconcile `declarest.io/v1alpha1` resources: `ResourceRepository`, `ManagedService`, `SecretStore`, `SyncPolicy`, and `MetadataBundle`.
2. Admission validation MUST apply resource defaults before `ValidateSpec()` checks on create/update, and MUST block deletes of dependency resources that are referenced by non-deleting `SyncPolicy` objects in the same namespace.
3. Exact-match CR string values in the form `${ENV_VAR}` MUST resolve from the operator process environment before webhook validation, dependency-reference checks, overlap checks, and controller runtime use; the persisted CR spec MUST remain unchanged.
4. `SyncPolicy` admission and reconcile validation MUST reject overlapping logical source scopes regardless of dependency-reference equality.
5. Controllers MUST add finalizer `declarest.io/cleanup` and MUST remove it only after controller-owned cleanup is complete.
6. `ResourceRepository` reconcile MUST ensure configured storage availability, perform authenticated git sync against the configured branch, update `status.lastFetchedRevision` and `status.lastFetchedTime`, and set `Ready`/`Stalled` conditions deterministically.
7. `ManagedService` reconcile MUST validate auth/proxy/throttling constraints, cache remote OpenAPI/metadata artifacts when configured, merge process proxy environment with any configured proxy fields before artifact downloads, and persist cache paths in status without leaking secret values.
8. `SecretStore` reconcile MUST enforce provider one-of constraints (`vault` or `file`), ensure file-backed storage dependencies when required, and set `status.resolvedPath` only for file-backed stores.
9. `SyncPolicy` reconcile MUST validate referenced dependency resources, compute a secret-version hash from referenced Secret `resourceVersion` values, and trigger full sync when generation, secret hash, or full-resync schedule requires it.
10. Incremental sync planning MUST be deterministic, repository-diff based, and safety-biased; unknown/unsupported repository path changes MUST fall back to full sync, metadata-owned defaults artifacts such as `/_/defaults.<ext>`, `/_/defaults-<profile>.<ext>`, `<resource>/defaults.<ext>`, and `<resource>/defaults-<profile>.<ext>` MUST resolve to the owning metadata scope instead of a synthetic payload child path, and unsupported unknown files under the reserved `defaults` prefix MUST remain in the unknown/unsupported bucket rather than receiving incremental resource targeting.
11. `SyncPolicy` apply execution MUST invoke DeclaREST mutation workflows through `orchestrator.Orchestrator`, honor `spec.sync.force` and `spec.sync.prune`, and update status stats (`targeted`, `applied`, `pruned`, `failed`) from executed operations.
12. `SyncPolicy` scheduling MUST requeue by the earliest due trigger between `spec.syncInterval` and `spec.fullResyncCron` (when configured), with invalid cron expressions treated as spec-validation failures.
13. Repository webhook receiver MUST accept only authenticated provider events for configured repositories, enforce payload-size bounds, enforce bounded read/write/idle HTTP timeouts, accept only push events for the configured branch, and patch webhook receipt annotations before returning success.
14. Webhook-triggered repository refresh MUST be event-driven via `ResourceRepository` annotation changes and MUST not wait for `pollInterval`.
14.1 `MetadataBundle` spec MUST define exactly one of `spec.source.url`, `spec.source.pvc`, or `spec.source.configMap`. `spec.source.url` MUST accept any scheme supported by `bundlemetadata.ResolveBundle` (`oci://`, `https://`, `http://`, `file://`, or the legacy `<name>:<version>` shorthand) and expansion of `${VAR}` placeholders MUST run before admission validation. `spec.source.pullSecretRef` MUST only be honored when `spec.source.url` uses the `oci://` scheme and MUST reference a Secret of type `kubernetes.io/dockerconfigjson` in the same namespace; rotation of that Secret (detected via resourceVersion) MUST trigger a reconcile. `spec.source.configMap` MUST reference a ConfigMap in the same namespace whose `binaryData[key]` (or base64-encoded `data[key]`) carries the gzipped tarball; rotation of that ConfigMap MUST trigger a reconcile.
14.2 `MetadataBundle` reconcile MUST forward the resolved source and dependency inputs through `bootstrap.ResolveMetadataBundle`, MUST override the provider cache root when `DECLAREST_BUNDLE_CACHE_DIR` (or, absent that, `DECLAREST_OPERATOR_CACHE_BASE_DIR/bundles`) is set, and MUST publish `status.cachePath`, `status.openAPIPath`, and `status.manifest` without leaking Secret values into status or events.
15. The operator MUST ship an operator-sdk `registry+v1` bundle under `bundle/` whose `bundle/manifests/` tree contains the `ClusterServiceVersion`, all owned CRDs, operator `Deployment`, RBAC, `Service` objects, and `PodDisruptionBudget`, whose `bundle/metadata/annotations.yaml` declares `operators.operatorframework.io.bundle.package.v1=declarest-operator` with the `alpha` channel set as default, and whose `bundle/tests/scorecard/config.yaml` enables the default operator-sdk scorecard suite.
16. The bundle `ClusterServiceVersion` MUST declare `installModes` supporting `OwnNamespace`, `SingleNamespace`, and `AllNamespaces` (and MUST disable `MultiNamespace`), MUST list every `declarest.io/v1alpha1` owned CRD (`resourcerepositories`, `managedservices`, `secretstores`, `syncpolicies`, `repositorywebhooks`) with deterministic `alm-examples` sourced from `config/samples/`, MUST declare webhook definitions mirroring `config/manifests/webhooks.yaml` on container port 9443 with `failurePolicy=Fail` and `timeoutSeconds=10`, and MUST NOT embed an `icon` entry.
17. The bundle Deployment MUST replace the cluster-scoped `declarest-operator-state` `PersistentVolumeClaim` with an `emptyDir` volume (OLM `registry+v1` forbids bundled PVCs); kustomize-published release manifests (`install.yaml`, `install-admission-*.yaml`) MUST continue to ship the PVC for non-OLM installs.
18. `make bundle` MUST regenerate bundle manifests from `config/manifests/` via `operator-sdk generate bundle --manifests` (never `--overwrite`) so that the hand-authored `bundle.Dockerfile`, `bundle/metadata/annotations.yaml`, and `bundle/tests/scorecard/config.yaml` are preserved; `make bundle-validate` MUST run `operator-sdk bundle validate ./bundle` with the `operatorframework` optional validator suite and fail on any reported error.
19. The file-based catalog MUST live under `catalog/declarest-operator/catalog.yaml` with `olm.package`, `olm.channel` (`alpha`, pointing at `declarest-operator.v<VERSION>`), and an `olm.bundle` entry referencing `ghcr.io/crmarques/declarest-operator-bundle:<VERSION>`; `make catalog VERSION=<VERSION>` MUST regenerate that catalog deterministically, `catalog.Dockerfile` MUST build from `quay.io/operator-framework/opm:v1.65.0`, and `make catalog-validate` MUST run `opm validate ./catalog` successfully.
20. `make release-bundle VERSION=<VERSION>` MUST regenerate the OLM bundle and catalog for exactly that semver version, MUST stamp a deterministic CSV `createdAt`, MUST validate `config/olm/`, MUST run `operator-sdk bundle validate` and `opm validate`, and MUST fail when the CSV name/version, CSV manager image, CSV `containerImage` annotation, catalog bundle name/version, or catalog bundle image do not match `VERSION`.
21. Release tooling MUST publish the CLI artifacts, operator image, bundle image, catalog image, and GitHub release through one tag-triggered release workflow DAG; publishing the GitHub release MUST depend on successful operator, bundle, and catalog image publication for the same tag.
22. Release tooling MUST publish the operator image to `ghcr.io/crmarques/declarest-operator` with both `v<VERSION>` and `<VERSION>` tags plus `latest`, MUST publish bundle and catalog images to `ghcr.io/crmarques/declarest-operator-bundle:<VERSION>` and `ghcr.io/crmarques/declarest-operator-catalog:<VERSION>` plus `latest`, and MUST attach the bundle tarball, rendered CSV, and rendered catalog as release assets.
23. Standalone operator-image and bundle-image workflows MAY exist only as `workflow_dispatch` smoke builds; they MUST NOT publish images from tag pushes.
24. `config/olm/` MUST provide a reference install overlay (`Namespace`, `OperatorGroup`, `CatalogSource`, `Subscription`) targeting the published catalog image in namespace `olm`, and `make olm-install`/`make olm-uninstall` MUST apply/remove that overlay deterministically.

## Data Contracts
1. Condition types: `Ready`, `Reconciling`, `Stalled` (from `api/v1alpha1`).
2. Finalizer: `declarest.io/cleanup`.
3. Repository webhook endpoint path: `/webhooks/repository/<namespace>/<repository>`; when `watch-namespace` is set, single-segment `<repository>` form MAY be accepted and resolves to that namespace.
4. Repository webhook annotation `declarest.io/webhook-last-received-at` stores the last accepted provider event timestamp (`RFC3339Nano`).
5. Repository webhook annotation `declarest.io/webhook-last-event-id` stores provider event identifiers when present.
6. `ResourceRepository` defaults: `spec.git.branch=main` when omitted; repository payload file extensions are determined at runtime from managed-service responses or explicit payload input, and `ResourceRepository` MUST NOT expose a payload-format default field.
7. `ManagedService` defaults: `spec.http.auth.oauth2.grantType=client_credentials` when omitted; `spec.pollInterval=10m` when omitted.
8. `SyncPolicy` defaults: `spec.source.recursive=true` and `spec.syncInterval=5m` when omitted.
9. `SyncPolicy` reconcile runtime MUST assemble a `config.Context` and bootstrap a session using `bootstrap.NewSessionFromResolvedContext`, yielding canonical interface implementations (`orchestrator.Orchestrator`, `repository.ResourceStore`, `metadata.MetadataService`, `secrets.SecretProvider`) for mutation workflows; managed-service and vault proxy blocks MAY override only selected fields from process proxy environment.
10. Sync execution plan modes: `full` or `incremental`; plan targets MUST be normalized and deduplicated.
11. OLM bundle layout: `bundle.Dockerfile` at repo root, `bundle/manifests/*.yaml` (CSV, CRDs, `Deployment`, RBAC, `Service` objects, `PodDisruptionBudget`), `bundle/metadata/annotations.yaml`, and `bundle/tests/scorecard/config.yaml`; bundle image tag equals the released operator `VERSION` (for example `0.0.1`).
12. File-based catalog layout: `catalog.Dockerfile`, `catalog/declarest-operator/catalog.yaml` with `olm.package`, `olm.channel` (`alpha` default), and one release-scoped `olm.bundle` entry referencing `ghcr.io/crmarques/declarest-operator-bundle:<VERSION>`.
13. OLM install overlay (`config/olm/`): `Namespace=olm`, `OperatorGroup=declarest-operators`, `CatalogSource=declarest-catalog` pointing at the published catalog image, and `Subscription=declarest-operator` on channel `alpha` with `installPlanApproval=Automatic`.
14. Scorecard suite: default operator-sdk basic and OLM tests enabled via `bundle/tests/scorecard/config.yaml` for release gating.

## Failure Modes
1. Spec validation failure (one-of/auth/path/cron/poll interval invariants) marks resource `NotReady` with reason `SpecInvalid`.
2. Missing or invalid referenced dependency resources mark `SyncPolicy` `NotReady` with reason `DependencyInvalid`.
3. Repository unavailable, session bootstrap failure, or apply/prune errors mark `SyncPolicy` `NotReady` with reconcile-failure reasons.
4. Webhook authentication/signature/token mismatch returns authorization failure and MUST NOT mutate repository annotations.
5. Oversized webhook payloads or malformed target paths return request errors and MUST NOT enqueue repository refresh.
6. Bundle validation failure (`operator-sdk bundle validate --select-optional suite=operatorframework`) or catalog validation failure (`opm validate`) MUST block release-image publishing.
7. Bundle manifests containing OLM-incompatible kinds (for example `PersistentVolumeClaim`) MUST fail `operator-sdk bundle validate` before image build.

## Edge Cases
1. Secret rotation with unchanged repository revision MUST still trigger `SyncPolicy` reconcile via secret-version-hash change.
2. Metadata-only repository changes under collection metadata (`.../_/metadata.*`) SHOULD resolve to recursive apply targets for affected scope.
3. Branch-mismatched push webhook events MUST be acknowledged and ignored without status mutation.
4. `spec.fullResyncCron` with no previous full sync MUST schedule immediate full sync eligibility.
5. Non-overlapping source paths with shared dependency references MUST be accepted.
6. Slow or idle webhook clients MUST be disconnected by bounded HTTP server timeouts rather than holding connections indefinitely.
7. A change to `customers/_/defaults.yaml` SHOULD produce an incremental apply target for `/customers`, while a change to `customers/acme/defaults-prod.yaml` SHOULD target `/customers/acme`; unknown files under a resource directory that only share the `defaults` prefix still trigger safe full fallback.
8. OLM-installed pods MUST rely on the bundled `emptyDir` state volume and MUST continue to accept configuration that assumes no persistent state volume is mounted; persistent state on non-OLM clusters remains available through the kustomize manifests.
9. Installing the operator via OLM across `AllNamespaces`, `SingleNamespace`, and `OwnNamespace` install modes MUST preserve admission-webhook behavior because OLM injects the webhook-serving certificate automatically.

## Examples
1. Normal: `ResourceRepository` receives a valid authenticated push webhook for the tracked branch, webhook annotations are patched, and reconcile fetches a new revision before the next `pollInterval`.
2. Corner: `SyncPolicy` references valid dependencies and unchanged revision, but a referenced Secret `resourceVersion` changes; reconcile runs full mode and reapplies scoped resources even without repository diff changes.

## Verification Expectations
1. CRD defaulting/validation and webhook admission checks MUST be covered by `api/v1alpha1/*_types_test.go` and `api/v1alpha1/webhook_test.go`.
2. Webhook authentication, branch filtering, annotation patch behavior, and timeout configuration MUST be covered by `internal/operator/controllers/repository_webhook_server_test.go`.
3. Incremental/full sync planning, metadata defaults-artifact path classification, and safe full-fallback paths MUST be covered by `internal/operator/controllers/syncpolicy_plan_test.go`.
4. Full-resync schedule and requeue interval computation MUST be covered by `internal/operator/controllers/syncpolicy_schedule_test.go`.
5. Path-overlap validation and dependency-sharing behavior MUST be covered by `internal/operator/controllers/syncpolicy_controller_test.go`.
6. Runtime `${ENV_VAR}` expansion for CR-backed repository/server/secret-store/sync-policy fields MUST be covered by `api/v1alpha1/webhook_test.go` plus controller-level tests such as `internal/operator/controllers/util_test.go`.
7. OLM packaging changes MUST be covered by `make verify-bundle` (kustomize build of `config/manifests` and `config/olm`, `operator-sdk bundle validate` with the `operatorframework` optional validator suite, `opm validate ./catalog`, generated version/image consistency checks, and a before/after artifact drift check), and any release-workflow change touching the bundle/catalog images MUST keep the tag-triggered CI validation steps green before publish.
