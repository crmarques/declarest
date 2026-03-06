# E2E Traceability Matrix

This file maps the primary E2E spec areas to the fast harness tests and runtime cases that currently enforce them.

| Spec area | Fast Bash contract tests | Runtime E2E cases |
|---|---|---|
| Profile scope selection and deterministic case discovery | `test/e2e/tests/cases_test.sh`, `test/e2e/tests/profile_operator_test.sh` | `test/e2e/cases/smoke/01-repo-status-baseline.sh`, `test/e2e/cases/smoke/04-list-deterministic.sh` |
| Component manifest and fixture validation | `test/e2e/tests/components_validate_test.sh` | `./test/e2e/run-e2e.sh --validate-components` |
| Hook semantic contract: state publication, context fragment output, repeated hook stability | `test/e2e/tests/components_contract_test.sh`, `test/e2e/tests/components_hooks_test.sh` | enforced before workload execution by harness hook contract checks |
| Repository baseline and sync-oriented smoke flows | N/A | `test/e2e/cases/smoke/01-repo-status-baseline.sh`, `test/e2e/cases/main/05-git-push-status.sh` |
| Save/apply/diff idempotency and force behavior | N/A | `test/e2e/cases/main/02-save-apply-diff.sh` |
| Secrets lifecycle and validation | N/A | `test/e2e/cases/main/03-secret-mask-resolve.sh`, `test/e2e/cases/main/07-secret-detect-normalize.sh`, `test/e2e/cases/corner/07-save-secret-plaintext-guard.sh`, `test/e2e/cases/corner/08-secret-detect-fix-validation.sh` |
| Deterministic list/read behavior | `test/e2e/tests/cases_test.sh` | `test/e2e/cases/smoke/04-list-deterministic.sh`, `test/e2e/cases/corner/05-alias-fallback-ambiguity.sh` |
| Metadata expansion and path safety | `test/e2e/tests/components_validate_test.sh` | `test/e2e/cases/corner/02-path-traversal-rejection.sh`, `test/e2e/cases/corner/06-metadata-expansion-multi-wildcard.sh` |
| Managed-server auth modes and mTLS | N/A | `test/e2e/components/managed-server/simple-api-server/cases/main/07-oauth2-auth-contract.sh`, `test/e2e/components/managed-server/simple-api-server/cases/main/08-basic-auth-contract.sh`, `test/e2e/components/managed-server/simple-api-server/cases/corner/07-mtls-trust-reload.sh` |
| Operator reconcile and webhook flows | `test/e2e/tests/operator_test.sh`, `test/e2e/tests/profile_operator_test.sh` | `test/e2e/cases/operator-main/01-operator-reconcile-create-update.sh`, `test/e2e/cases/operator-main/02-operator-reconcile-webhook.sh` |

Current intentional gaps:

- Parallel run isolation is still covered at the harness/helper level, not by a dedicated runtime E2E case.
- Timeout and retry UX is covered by harness tests, but not yet by a standalone runtime workload case.
