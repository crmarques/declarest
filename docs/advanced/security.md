# Security hardening

This page lists practical hardening controls for DeclaREST CLI and Operator deployments.

## Git and repository security

- Use protected branches and required reviews for desired-state repos.
- Use least-privilege Git credentials (token/SSH key scope limited to required repo).
- Prefer webhook signature/token validation when enabling repository webhooks.

## Secrets handling

- Keep plaintext secrets out of Git; use placeholders plus secret store.
- Limit Kubernetes Secret access to operator service account scope.
- Rotate credentials regularly and verify reconcile behavior after rotation.

## Network and transport

- Enforce TLS for managed API and Git endpoints.
- Configure explicit CA trust when using private PKI.
- Use mTLS where required by target API policy.
- Use proxy allowlists and `noProxy` carefully for internal endpoints.

## Operator runtime hardening

- Keep `runAsNonRoot`, `readOnlyRootFilesystem`, dropped capabilities, and `RuntimeDefault` seccomp.
- Restrict namespace and RBAC scope to the minimum needed.
- Limit who can create/update CRDs that drive reconciliation.

## Audit and change control

- Treat Git history as the primary audit trail for desired-state changes.
- Use PR templates/checks for risky paths.
- Monitor CR conditions and operator logs for repeated auth/spec failures.

## Recovery planning

- Define rollback workflow (revert commit, reconcile, verify).
- Back up secret storage data for file-backed providers.
- Test failure paths (bad secret ref, bad auth, branch mismatch) in non-prod.
