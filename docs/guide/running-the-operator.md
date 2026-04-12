# Running the Operator

> Completed the [Operator Quickstart](../getting-started/quickstart-operator.md)? This page covers the full operational model.

The Operator is the recommended runtime for production. It continuously reconciles Managed Services from Git desired state.

```text
Git (desired state) -> Operator reconcile loop -> Managed Service (real state)
```

Direct server edits are drift. Reconciliation corrects drift back to what is declared in Git.

## CRD model

The controller manager reconciles four namespaced CRDs:

| CRD | Responsibility |
|-----|---------------|
| **ResourceRepository** | Polls Git and maintains the current revision |
| **ManagedService** | Defines endpoint, auth, optional OpenAPI and metadata artifacts |
| **SecretStore** | Resolves secrets from `file` or `vault` provider |
| **SyncPolicy** | References the other three; plans and executes apply/prune for a source path |

`SyncPolicy` is the execution unit. It references one of each dependency and reconciles resources within its `source.path` scope.

## Reconciliation: incremental vs full

`SyncPolicy` decides between two modes:

- **Incremental**: targeted updates based on repository changes. Faster and lower-impact.
- **Full**: processes every resource in scope. Used when changes are broad, confidence is low, or the policy requires it.

Safety-first behavior prefers full sync when diff confidence is low.

## Triggers

Reconcile is driven by:

- **Sync interval**: `spec.syncInterval` (default `5m`)
- **Full-resync cron**: optional `fullResyncCron` for periodic full syncs
- **Dependency changes**: ResourceRepository, ManagedService, or SecretStore generation updates
- **Secret changes**: referenced Kubernetes Secret version hash changes
- **Repository refresh**: poll or webhook-driven

## Drift handling

- **Default**: apply only when drift is detected between desired and real state.
- **Forced**: `spec.sync.force: true` triggers update calls even when compare shows no drift.

## Status model

Track these fields for operations and debugging:

```bash
kubectl describe syncpolicy <name>
```

Key status fields:

- `lastAttemptedRepoRevision` / `lastAppliedRepoRevision`
- `lastSyncMode` (incremental or full)
- `resourceStats.{targeted, applied, pruned, failed}`
- Conditions: `Ready`, `Reconciling`, `Stalled`

### Failure analysis sequence

1. `kubectl describe` the relevant CR.
2. Inspect condition reason/message.
3. Check controller logs for root cause.
4. Verify referenced Secrets and dependency refs.
5. Confirm repository branch/revision movement.

```bash
kubectl get resourcerepositories,managedservices,secretstores,syncpolicies
kubectl logs -n declarest-system deploy/declarest-operator-controller-manager
```

### Common failure modes

- Invalid spec (auth one-of, required fields)
- Missing referenced dependency resources
- Git fetch/auth failures
- Managed API auth/transport failures
- Apply/prune execution errors

All surface through CR status conditions and controller logs.

## Building and deploying

### Local build and run

```bash
make operator-build
make operator-run
```

### Container image

```bash
make operator-image
make operator-image OPERATOR_IMAGE=ghcr.io/crmarques/declarest-operator OPERATOR_IMAGE_TAG=v0.2.2
```

Images are published to GHCR by `.github/workflows/operator-image.yml` on semver tag push.

Manual push:

```bash
podman login ghcr.io
make operator-image-push OPERATOR_IMAGE=ghcr.io/crmarques/declarest-operator OPERATOR_IMAGE_TAG=v0.2.2
make operator-image-push OPERATOR_IMAGE=ghcr.io/crmarques/declarest-operator OPERATOR_IMAGE_TAG=latest
```

### Install on a cluster

Released install manifests are the recommended install path:

| Manifest | Includes | Depends on | Recommended use |
|---|---|---|---|
| `install.yaml` | CRDs, RBAC, manager deployment | None | Simplest install, evaluation environments, or clusters where you do not want admission webhook dependencies |
| `install-admission-certmanager.yaml` | Base install plus validating admission webhooks | `cert-manager` | Recommended default for production Kubernetes clusters |
| `install-admission-openshift.yaml` | Base install plus validating admission webhooks | OpenShift serving cert integration | Recommended for OpenShift |

```bash
VERSION={{ declarest_tag() }}
kubectl create namespace declarest-system
kubectl apply -f "https://github.com/crmarques/declarest/releases/download/${VERSION}/install-admission-certmanager.yaml"
```

Released install manifests watch all namespaces by default. They ship cluster-scope RBAC for DeclaREST resources, Secrets, and PVCs, plus a namespace-scoped lease role for leader election in `declarest-system`.

Why the variants exist:

- `install.yaml` keeps the footprint smallest. It does not enable the validating admission webhook, so CR validation happens later in the controller/runtime path instead of being enforced at the Kubernetes admission layer.
- `install-admission-certmanager.yaml` enables the validating admission webhook and relies on `cert-manager` to provision the webhook TLS certificate and CA injection.
- `install-admission-openshift.yaml` also enables the validating admission webhook, but gets the serving certificate from OpenShift service annotations instead of `cert-manager`.

Use the cert-manager variant unless one of these is true:

- You are on OpenShift: use `install-admission-openshift.yaml`.
- You want the fewest cluster dependencies or a quick evaluation install: use `install.yaml`.

For local development from a source checkout, the repository kustomize bases are still useful:

```bash
kubectl create namespace declarest-system
kubectl apply -k config/default
```

Developer overlays map to the release assets like this:

- `config/release/core` -> `install.yaml`
- `config/release/admission-certmanager` -> `install-admission-certmanager.yaml`
- `config/release/admission-openshift` -> `install-admission-openshift.yaml`

Treat `config/samples/*.yaml` as reference templates, not a ready-to-apply first-success bundle. Edit the URLs, Secret references, and names for your environment before applying them.

## Observability

The manager exposes:

- `/metrics` on `:8080`
- `/healthz` and `/readyz` on `:8081`

OTLP export is enabled through standard `OTEL_EXPORTER_OTLP_*` environment variables.

### Signals to watch

- Sync duration trends
- Reconcile error rates
- `resourceStats.failed` and repeated retries
- Managed API latency and throttling responses

## Security defaults

The manager deployment uses:

- `runAsNonRoot`
- `readOnlyRootFilesystem`
- Dropped Linux capabilities
- `seccompProfile: RuntimeDefault`

Credentials come from Kubernetes Secret references; sensitive values are not in CR specs/status.

For SSH repository auth, host verification is required by default (`knownHostsRef`). It is only skipped when `spec.git.auth.sshSecretRef.insecureIgnoreHostKey: true` is explicitly set.

## CLI + Operator together

Most teams use both:

- **CLI** for authoring: save, edit, diff, validate, commit, push.
- **Operator** for runtime: continuous reconciliation after merge.

Typical admin flow:

1. `repository refresh` and `resource save` to import current state.
2. Edit payloads and metadata locally.
3. `resource diff` and `resource explain` to validate.
4. Commit and push for review/merge.
5. Operator reconciles after merge.

### Operational guidelines

- Keep source scopes non-overlapping across `SyncPolicy` objects.
- Use webhook-triggered refresh for low-latency response to Git push.
- Keep sync scope narrow for faster, safer reconciles.
- Split large domains into multiple `SyncPolicy` paths.
