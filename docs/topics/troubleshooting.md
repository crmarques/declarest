# Troubleshooting

Common issues for both CLI and Operator mode.

## 1. No active context (CLI)

- Symptom: commands fail before any API call; messages mention missing current context.
- Fix: run `declarest context add` or `declarest context use <name>`, then `declarest context check`.

## 2. Path rejected as invalid

- Symptom: validation error for a path.
- Fix: use logical absolute paths (`/a/b`), and only use trailing `/` for collections.

## 3. `resource get` returns not found, but resource exists

- Symptom: direct read fails for alias-style paths.
- Fix: confirm metadata identity mapping (`resource.id` / `resource.alias`) and run `declarest resource metadata render <path> get`.

## 4. Diff shows unexpected drift every run

- Symptom: repeated updates even without intended changes.
- Fix: add compare suppression/filter rules in metadata for server-generated fields (timestamps, versions).

## 5. Plaintext secret warning on save

- Symptom: `resource save` fails due to detected plaintext secret candidates.
- Fix: use `--secret-attributes` (recommended), or declare `resource.secretAttributes` first. See [Managing Secrets](../guide/managing-secrets.md).

## 6. `repository push` fails

- Symptom: push command returns validation or auth errors.
- Fix: verify context uses `repository.git.remote` (not filesystem), then check branch/auth/proxy/TLS config.

## 7. Operator resources stay `NotReady`

- Symptom: CRs are created but conditions show `Stalled`/`NotReady`.
- Fix: inspect CR status and controller logs:
  - `kubectl -n declarest-system describe <kind> <name>`
  - `kubectl -n declarest-system logs deploy/declarest-operator -c manager`

## 8. `SyncPolicy` does not reconcile after Git push

- Symptom: no revision change after commit.
- Fix: verify branch and credentials in `ResourceRepository`; confirm webhook/poll settings and that push was to the configured branch.

## 9. Admission webhook blocks CR creation

- Symptom: `kubectl apply` fails with spec validation errors.
- Fix: correct one-of fields (for example auth/provider choices) and required refs; use [Operator CRDs reference](../reference/operator-crds.md).

## 10. Operator cannot authenticate to managed API

- Symptom: reconcile errors from auth failures.
- Fix: validate `ManagedServer.spec.http.auth` mode and referenced Kubernetes Secret keys; check token URL/credentials for OAuth2 mode.

## Quick debug checklist

```bash
# CLI side
declarest context current
declarest context check
declarest server check

# Operator side
kubectl -n declarest-system get resourcerepositories,managedservers,secretstores,syncpolicies
kubectl -n declarest-system logs deploy/declarest-operator -c manager --tail=200
```
