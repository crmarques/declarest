# Quickstart (Operator - recommended)

This is the shortest path to first reconcile success with the Kubernetes Operator.

## Prerequisites

- A Kubernetes cluster with `kubectl` access
- A namespace for the Operator (examples use `declarest-system`)
- A Git repository that stores DeclaREST resource files (desired state)
- Managed Service API endpoint + auth credentials
- Dynamic PVC provisioning, or existing PVCs

## 1. Install CRDs and Operator

Choose one released install manifest:

| Manifest | What it installs | Depends on | Use it when |
|---|---|---|---|
| `install.yaml` | CRDs, RBAC, operator manager | Nothing beyond Kubernetes | You want the fastest path to first reconcile success |
| `install-admission-certmanager.yaml` | `install.yaml` plus validating admission webhooks | `cert-manager` | Recommended for most non-OpenShift production clusters |
| `install-admission-openshift.yaml` | `install.yaml` plus validating admission webhooks | OpenShift serving cert support | Recommended on OpenShift |
| `install-olm.yaml` | `OperatorGroup`, `CatalogSource`, `Subscription` for OLM-managed install | [OLM](https://olm.operatorframework.io/) installed on the cluster | Your cluster manages operators through OLM (see [Installing with OLM](../guide/installing-with-olm.md)) |

Admission-enabled manifests reject invalid CRs earlier, at create/update time. The base `install.yaml` keeps cluster dependencies minimal and is the shortest quickstart path. The OLM manifest delegates lifecycle management (install, upgrade, uninstall) to OLM and always enables admission webhooks.

```bash
VERSION={{ declarest_tag() }}
kubectl create namespace declarest-system
kubectl apply -f "https://github.com/crmarques/declarest/releases/download/${VERSION}/install.yaml"
```

That installs the CRDs and deploys the Operator manager from the tagged release.

If you want admission validation from the start, swap the filename in the URL:

- `install-admission-certmanager.yaml` on standard Kubernetes clusters with `cert-manager`
- `install-admission-openshift.yaml` on OpenShift

## 2. Create required Secrets

Create secrets referenced by CRs (adjust names/keys/values for your environment):

```bash
kubectl -n declarest-system create secret generic repository-credentials \
  --from-literal=token='<git-token>' \
  --from-literal=webhook-secret='<webhook-secret>'

kubectl -n declarest-system create secret generic managed-service-auth \
  --from-literal=token='<api-token>'

kubectl -n declarest-system create secret generic secret-store-file \
  --from-literal=passphrase='<strong-passphrase>'
```

## 3. Apply minimal CRs

For first reconcile success, use a manifest like this. The files under `config/samples/` are reference templates and still require environment-specific edits before you apply them.

```yaml
apiVersion: declarest.io/v1alpha1
kind: ResourceRepository
metadata:
  name: demo-repository
  namespace: declarest-system
spec:
  type: git
  pollInterval: 1m
  git:
    url: https://github.com/example/declarest-resources.git
    branch: main
    auth:
      tokenRef:
        name: repository-credentials
        key: token
  storage:
    pvc:
      accessModes: ["ReadWriteOnce"]
      requests:
        storage: 1Gi
---
apiVersion: declarest.io/v1alpha1
kind: ManagedService
metadata:
  name: demo-managed-service
  namespace: declarest-system
spec:
  http:
    baseURL: https://api.example.com
    auth:
      customHeaders:
        - header: Authorization
          prefix: Bearer
          valueRef:
            name: managed-service-auth
            key: token
---
apiVersion: declarest.io/v1alpha1
kind: SecretStore
metadata:
  name: demo-secret-store
  namespace: declarest-system
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
---
apiVersion: declarest.io/v1alpha1
kind: SyncPolicy
metadata:
  name: demo-sync
  namespace: declarest-system
spec:
  resourceRepositoryRef:
    name: demo-repository
  managedServiceRef:
    name: demo-managed-service
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

`spec.storage.pvc.accessModes` is required when you ask DeclaREST to create a PVC. The operator does not default it because valid access modes depend on your cluster storage class.

## 4. Confirm health and reconcile

```bash
kubectl -n declarest-system get resourcerepositories,managedservices,secretstores,syncpolicies
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

- [Core Concepts](../guide/core-concepts.md) — understand all the building blocks
- [Running the Operator](../guide/running-the-operator.md) — full operational model
- [Operator CRDs reference](../reference/operator-crds.md)
