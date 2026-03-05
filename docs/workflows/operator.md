# Run the Operator

The Operator is the recommended mode for production GitOps. It continuously reconciles Managed Servers from repository desired state.

## Recommended workflow: Git is source of truth

Use this mental model:

1. Desired state lives in a Git repository.
2. Admins use the CLI to save/edit/diff desired state locally, then commit and push.
3. The Operator watches/polls the repository and reconciles `SyncPolicy` targets.
4. Managed Server real state converges to the latest desired state from Git.

```text
Git (desired state) -> Operator reconcile loop -> Managed Server (real state)
```

In this model, direct server edits are drift. Reconciliation corrects drift back to what is declared in Git.

## CRD model

The controller manager (`cmd/declarest-operator-manager`) reconciles four namespaced CRDs:

- `ResourceRepository`
- `ManagedServer`
- `SecretStore`
- `SyncPolicy`

How they work together:

- `ResourceRepository` polls Git and keeps the current revision available.
- `ManagedServer` defines endpoint/auth plus optional OpenAPI and metadata artifacts.
- `SecretStore` defines where secrets are resolved (`vault` or `file`).
- `SyncPolicy` references the other three and applies/prunes resources for a source path.

Important behavior:

- `SyncPolicy.spec.sync.force` can force updates even when compare output shows no drift.
- `SyncPolicy` requeues on `spec.syncInterval` (default `5m`).
- Reconcile is also triggered by relevant dependency changes and referenced Secret changes.

## Build and run locally

```bash
make operator-build
make operator-run
```

Container image:

```bash
make operator-image
```

Build a specific image reference:

```bash
make operator-image OPERATOR_IMAGE=ghcr.io/crmarques/declarest-operator OPERATOR_IMAGE_TAG=v0.2.2
```

## Publish operator images

Operator images are published to GHCR by `.github/workflows/operator-image.yml`.

- Trigger: push a semver tag (`vX.Y.Z`)
- Published tags: `ghcr.io/crmarques/declarest-operator:vX.Y.Z` and `ghcr.io/crmarques/declarest-operator:latest`

Manual fallback:

```bash
podman login ghcr.io
make operator-image OPERATOR_IMAGE=ghcr.io/crmarques/declarest-operator OPERATOR_IMAGE_TAG=v0.2.2
make operator-image-push OPERATOR_IMAGE=ghcr.io/crmarques/declarest-operator OPERATOR_IMAGE_TAG=v0.2.2
make operator-image-push OPERATOR_IMAGE=ghcr.io/crmarques/declarest-operator OPERATOR_IMAGE_TAG=latest
```

Image pull examples:

```bash
podman pull ghcr.io/crmarques/declarest-operator:v0.2.2
podman pull ghcr.io/crmarques/declarest-operator:latest
```

## Install on a cluster

```bash
kubectl create namespace declarest-system
kubectl apply -k config/default
```

`config/default` includes admission webhook resources and default manager deployment settings.

Apply sample resources:

```bash
kubectl apply -k config/samples
```

## Check reconcile status

```bash
kubectl get resourcerepositories,managedservers,secretstores,syncpolicies
kubectl describe syncpolicy <name>
kubectl logs -n declarest-system deploy/declarest-operator-controller-manager
```

Use these to confirm latest fetched revision, policy conditions, and apply/prune stats.

## Observability

The manager exposes:

- `/metrics` on `:8080`
- `/healthz` and `/readyz` on `:8081`

OTLP export can be enabled through standard `OTEL_EXPORTER_OTLP_*` environment variables.

## Security defaults

The manager deployment uses:

- `runAsNonRoot`
- `readOnlyRootFilesystem`
- dropped Linux capabilities
- `seccompProfile: RuntimeDefault`

Credentials are read from Kubernetes `Secret` references; sensitive values are not persisted in CR specs/status.

For SSH repository auth, host verification is required by default (`knownHostsRef`). It is only skipped when `spec.git.auth.sshSecretRef.insecureIgnoreHostKey: true` is explicitly set.
