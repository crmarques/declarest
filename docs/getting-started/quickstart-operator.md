# Quickstart (Operator - recommended)

This is the shortest path to first reconcile success with the Kubernetes Operator.

## Prerequisites

- A Kubernetes cluster with `kubectl` access
- A namespace for the Operator (examples use `declarest-system`)
- A Git repository that stores DeclaREST resource files (desired state)
- Managed Server API endpoint + auth credentials
- Dynamic PVC provisioning, or existing PVCs

## 1. Install CRDs and Operator

```bash
kubectl create namespace declarest-system
kubectl apply -k config/default
```

This installs the CRDs and deploys the Operator manager.

## 2. Create required Secrets

Create secrets referenced by CRs (adjust names/keys/values for your environment):

```bash
kubectl -n declarest-system create secret generic repository-credentials \
  --from-literal=token='<git-token>' \
  --from-literal=webhook-secret='<webhook-secret>'

kubectl -n declarest-system create secret generic managed-server-auth \
  --from-literal=token='<api-token>'

kubectl -n declarest-system create secret generic secret-store-file \
  --from-literal=passphrase='<strong-passphrase>'
```

## 3. Apply minimal CRs

You can start from `config/samples/*.yaml` or apply this minimal set:

```yaml
apiVersion: declarest.io/v1alpha1
kind: ResourceRepository
metadata:
  name: demo-repository
  namespace: declarest-system
spec:
  type: git
  pollInterval: 1m
  resourceFormat: json
  git:
    url: https://github.com/example/declarest-resources.git
    branch: main
    auth:
      tokenSecretRef:
        name: repository-credentials
        key: token
  storage:
    pvc:
      accessModes: ["ReadWriteOnce"]
      requests:
        storage: 1Gi
---
apiVersion: declarest.io/v1alpha1
kind: ManagedServer
metadata:
  name: demo-managed-server
  namespace: declarest-system
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
---
apiVersion: declarest.io/v1alpha1
kind: SecretStore
metadata:
  name: demo-secret-store
  namespace: declarest-system
spec:
  provider: file
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
---
apiVersion: declarest.io/v1alpha1
kind: SyncPolicy
metadata:
  name: demo-sync
  namespace: declarest-system
spec:
  resourceRepositoryRef:
    name: demo-repository
  managedServerRef:
    name: demo-managed-server
  secretStoreRef:
    name: demo-secret-store
  source:
    path: /customers
    recursive: true
  sync:
    force: false
    prune: false
  suspend: false
```

Apply it:

```bash
kubectl apply -f quickstart-operator.yaml
```

## 4. Confirm health and reconcile

```bash
kubectl -n declarest-system get resourcerepositories,managedservers,secretstores,syncpolicies
kubectl -n declarest-system describe syncpolicy demo-sync
kubectl -n declarest-system logs deploy/declarest-operator -c manager --tail=200
```

Look for `Ready=True` conditions and non-empty `lastFetchedRevision` / `lastAppliedRepoRevision` in status.

## 5. Prove Git commit -> state change

1. Change one resource file in your Git repo under `/customers/...`.
2. Commit and push.
3. Wait one reconcile interval (or trigger a reconcile by changing `SyncPolicy`).
4. Check status again:

```bash
kubectl -n declarest-system get syncpolicy demo-sync -o yaml
```

You should see the new attempted/applied revision and updated sync timestamps.

## Next steps

- [Operator model](../concepts/operator.md)
- [Run the Operator](../workflows/operator.md)
- [Operator CRDs reference](../reference/operator-crds.md)
