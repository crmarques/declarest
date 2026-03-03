# Kubernetes Operator (Multi-CRD)

DeclaREST includes an Operator SDK based controller manager (`cmd/declarest-operator-manager`) that reconciles repository desired state to managed servers using four namespaced CRDs:

- `ResourceRepository`
- `ManagedServer`
- `SecretStore`
- `SyncPolicy`

## Resource model

- `ResourceRepository` polls Git and keeps the latest checked-out revision available to dependent policies.
- `ManagedServer` defines endpoint/auth plus optional OpenAPI and metadata artifact URLs.
- `SecretStore` defines the secret backend (`vault` or `file`).
- `SyncPolicy` references the other three resources and performs repo-to-managed apply (and optional prune).
- `SyncPolicy` requeues on `spec.syncInterval` (defaults to 5m), and reconciles when referenced dependency CRDs or referenced Kubernetes Secrets change.

## Build and run

```bash
make operator-build
make operator-run
```

Container build:

```bash
make operator-image
```

## Install on cluster

Apply the generated manifests:

```bash
kubectl create namespace declarest-system
kubectl apply -k config/default
```

Notes:

- `config/default` now includes a validating admission webhook for all four CRDs.
- The webhook TLS certificate is provisioned via `cert-manager` resources in `config/certmanager`.
- The manager state volume is PVC-backed (`declarest-operator-state`) rather than `emptyDir`.

Apply sample resources:

```bash
kubectl apply -k config/samples
```

## Observability

The manager exposes:

- `/metrics` on `:8080`
- `/healthz` and `/readyz` on `:8081`

OTLP export is enabled through standard `OTEL_EXPORTER_OTLP_*` environment variables.

## Security defaults

The manager deployment uses:

- `runAsNonRoot`
- `readOnlyRootFilesystem`
- dropped Linux capabilities
- `seccompProfile: RuntimeDefault`

All credentials are read from Kubernetes `Secret` references; sensitive values are not persisted in CR spec/status.

SSH repository auth requires `knownHostsRef` by default. Host-key verification can only be skipped when `spec.git.auth.sshSecretRef.insecureIgnoreHostKey: true` is explicitly set.
