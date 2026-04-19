# Installing with OLM

The DeclaREST Operator publishes an OLM-compatible bundle and file-based catalog for every tagged release. [Operator Lifecycle Manager](https://olm.operatorframework.io/) can then install, upgrade, and manage the operator on any cluster that runs OLM (for example OpenShift, or vanilla Kubernetes with OLM installed).

Use this path when your cluster already relies on OLM to manage operators. For clusters without OLM, keep using the kustomize manifests described in the [Operator Quickstart](../getting-started/quickstart-operator.md).

## Prerequisites

- A Kubernetes cluster with [OLM](https://olm.operatorframework.io/docs/getting-started/) already installed (provides the `olm` namespace and `operators.coreos.com` APIs).
- `kubectl` access to that cluster.
- Outbound pull access to `ghcr.io/crmarques/declarest-operator-bundle` and `ghcr.io/crmarques/declarest-operator-catalog`.

## 1. Apply the OLM install manifest

Each release ships `install-olm.yaml` alongside the other install bundles. It creates a `declarest-system` namespace plus a `CatalogSource` in the `olm` namespace and a matching `OperatorGroup`/`Subscription` in `declarest-system`.

```bash
VERSION={{ declarest_tag() }}
kubectl apply -f "https://github.com/crmarques/declarest/releases/download/${VERSION}/install-olm.yaml"
```

The manifest installs:

| Object | Name | Namespace |
|---|---|---|
| `Namespace` | `declarest-system` | - |
| `CatalogSource` | `declarest-operator-catalog` | `olm` |
| `OperatorGroup` | `declarest-operator` | `declarest-system` |
| `Subscription` | `declarest-operator` (channel `alpha`) | `declarest-system` |

The subscription uses `installPlanApproval: Automatic`. Edit the subscription before apply if you want to approve each upgrade explicitly.

## 2. Verify the install

```bash
kubectl -n olm get catalogsource declarest-operator-catalog
kubectl -n declarest-system get subscription declarest-operator
kubectl -n declarest-system get csv
```

Watch for `PHASE=Succeeded` on the `declarest-operator.v<VERSION>` CSV. Once the CSV reports success, the operator manager Deployment and admission webhooks are ready.

## 3. Install modes

The bundled CSV supports three install modes:

- `OwnNamespace` - operator reconciles CRs in the namespace it runs in.
- `SingleNamespace` - operator reconciles CRs in exactly one target namespace.
- `AllNamespaces` - operator reconciles CRs cluster-wide.

`MultiNamespace` is intentionally disabled. The reference manifest uses an empty `OperatorGroup.spec` (cluster-wide). To target a single namespace, edit the `OperatorGroup` to list `targetNamespaces` before you apply the overlay.

## 4. OLM vs kustomize install: what changes

| Aspect | kustomize (`install.yaml`) | OLM (`install-olm.yaml`) |
|---|---|---|
| Install channel | `kubectl apply -f install.yaml` | `Subscription` reconciled by OLM |
| Upgrades | Re-run `kubectl apply` | OLM pulls new CSVs from the catalog |
| Admission webhooks | Opt-in via `install-admission-*.yaml` | Always included; OLM injects serving certs |
| Operator state volume | PVC (`declarest-operator-state`) | `emptyDir` (OLM `registry+v1` forbids bundled PVCs) |
| CRD lifecycle | Applied as plain manifests | Owned by the CSV and cleaned up with the operator |

If you rely on persistent operator state (for example to preserve cached bundle artifacts across restarts), stay on the kustomize manifests. The OLM bundle deliberately uses `emptyDir` because OLM rejects PVCs inside bundle manifests.

## 5. Create CRs and prove reconcile

From this point the workflow matches the kustomize install: create the referenced Kubernetes Secrets, then apply your `ResourceRepository`, `ManagedService`, `SecretStore`, and `SyncPolicy` CRs. See the [Operator Quickstart](../getting-started/quickstart-operator.md#2-create-required-secrets) for ready-to-copy CR examples.

## 6. Uninstall

```bash
kubectl delete -f "https://github.com/crmarques/declarest/releases/download/${VERSION}/install-olm.yaml"
```

Deleting the `Subscription` stops upgrades. Deleting the CSV or the `declarest-system` namespace removes the operator Deployment, RBAC, and webhook configurations. OLM does not delete `declarest.io/v1alpha1` custom resources that you created; remove them separately if required.

## Troubleshooting

- **`CatalogSource` stays `TRANSIENT_FAILURE`**: confirm the cluster can pull `ghcr.io/crmarques/declarest-operator-catalog:<VERSION>`; check image pull secrets on the `olm` namespace service account.
- **`Subscription` reports `ResolutionFailed`**: ensure the `OperatorGroup` target namespaces match one of the supported install modes (`OwnNamespace`, `SingleNamespace`, `AllNamespaces`).
- **Webhook validation errors immediately after install**: OLM injects the webhook serving certificate. Give OLM up to a minute to finalize cert injection before applying CRs.
- **Operator pod crash-loops after an OLM upgrade**: check `kubectl -n declarest-system describe csv declarest-operator.v<VERSION>` for deployment errors, then inspect operator manager logs.

## Related

- [Operator Quickstart](../getting-started/quickstart-operator.md) - fastest path without OLM.
- [Running the Operator](running-the-operator.md) - full operational model once installed.
- [Operator CRDs reference](../reference/operator-crds.md) - CR field-level reference.
