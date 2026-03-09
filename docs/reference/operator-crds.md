# Operator CRDs

This page summarizes the four DeclaREST CRDs and how they connect.

## Relationship graph

```text
ResourceRepository + ManagedServer + SecretStore -> SyncPolicy -> reconcile
```

`SyncPolicy` is the execution unit. It references the other three resources.

## `ResourceRepository`

Purpose: define where desired state comes from (Git) and where it is stored in-cluster.

Key fields:

- `spec.type` (`git`)
- `spec.pollInterval`
- `spec.git.url`
- `spec.git.branch` (defaults to `main`)
- `spec.git.auth.tokenRef` or `spec.git.auth.sshSecretRef`
- `spec.storage` (`existingPVC` or `pvc`)

Repository payload files preserve the managed-server response media type or the explicit payload input media type. The CRD no longer exposes a repository payload-format default.

Minimal example:

```yaml
apiVersion: declarest.io/v1alpha1
kind: ResourceRepository
metadata:
  name: example-repository
spec:
  type: git
  pollInterval: 1m
  git:
    url: https://github.com/example/declarest-resources.git
    auth:
      tokenRef:
        name: repository-credentials
        key: token
  storage:
    pvc:
      accessModes: ["ReadWriteOnce"]
      requests:
        storage: 1Gi
```

## `ManagedServer`

Purpose: define target API connection/auth for reconciliation.

Key fields:

- `spec.http.baseURL`
- `spec.http.auth` (exactly one: `oauth2`, `basicAuth`, `customHeaders`)
- optional `spec.http.tls`, `spec.http.proxy`, `spec.http.requestThrottling`
- optional `spec.openapi.url`
- optional `spec.metadata.url` or `spec.metadata.bundle`

Minimal example:

```yaml
apiVersion: declarest.io/v1alpha1
kind: ManagedServer
metadata:
  name: example-managed-server
spec:
  http:
    baseURL: https://api.example.com
    auth:
      customHeaders:
        - header: Authorization
          prefix: Bearer
          valueRef:
            name: managed-server-auth
            key: token
```

## `SecretStore`

Purpose: define where DeclaREST resolves/stores secrets during workflows.

Key fields:

- `spec.file` or `spec.vault` (exactly one)
- `spec.file.path`, `spec.file.encryption`, `spec.file.storage`
- `spec.vault.address`, `spec.vault.auth.token.secretRef`
- optional `spec.vault.auth.userpass.*` or `spec.vault.auth.appRole.*`

Minimal file-provider example:

```yaml
apiVersion: declarest.io/v1alpha1
kind: SecretStore
metadata:
  name: example-secret-store
spec:
  file:
    path: /var/lib/declarest/secrets/secrets.json
    encryption:
      passphraseRef:
        name: secret-store-file
        key: passphrase
    storage:
      pvc:
        accessModes: ["ReadWriteOnce"]
        requests:
          storage: 1Gi
```

## `SyncPolicy`

Purpose: bind source + target + secret store and define sync behavior.

Key fields:

- `spec.resourceRepositoryRef.name`
- `spec.managedServerRef.name`
- `spec.secretStoreRef.name`
- `spec.source.path`
- `spec.source.recursive` (defaults to `true`)
- `spec.sync.force`, `spec.sync.prune`
- `spec.syncInterval` (defaults to `5m`)
- optional `spec.fullResyncCron`
- `spec.suspend`

Minimal example:

```yaml
apiVersion: declarest.io/v1alpha1
kind: SyncPolicy
metadata:
  name: example-sync
spec:
  resourceRepositoryRef:
    name: example-repository
  managedServerRef:
    name: example-managed-server
  secretStoreRef:
    name: example-secret-store
  source:
    path: /customers
    recursive: true
  sync:
    force: false
    prune: false
  suspend: false
```

## How they relate in practice

1. `ResourceRepository` fetches desired state from Git.
2. `ManagedServer` provides API connectivity.
3. `SecretStore` provides secret resolution/storage.
4. `SyncPolicy` reconciles one source scope from desired state to real state.

For runnable steps, see [Quickstart (Operator - recommended)](../getting-started/quickstart-operator.md).
