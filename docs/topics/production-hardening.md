# Production Hardening

This page covers security and performance guidance for production DeclaREST deployments.

## Security hardening

### Git and repository security

- Use protected branches and required reviews for desired-state repos.
- Use least-privilege Git credentials (token/SSH key scope limited to required repo).
- Prefer webhook signature/token validation when enabling repository webhooks.

### Secrets handling

- Keep plaintext secrets out of Git; use placeholders plus a secret store.
- Limit Kubernetes Secret access to the operator service account scope.
- Rotate credentials regularly and verify reconcile behavior after rotation.

### Network and transport

- Enforce TLS for managed API and Git endpoints.
- Configure explicit CA trust when using private PKI.
- Use mTLS where required by target API policy.
- Use proxy allowlists and `noProxy` carefully for internal endpoints.

### Operator runtime hardening

- Keep the default security context: `runAsNonRoot`, `readOnlyRootFilesystem`, dropped capabilities, `RuntimeDefault` seccomp.
- Restrict namespace and RBAC scope to the minimum needed.
- Limit who can create/update CRDs that drive reconciliation.

### Audit and change control

- Treat Git history as the primary audit trail for desired-state changes.
- Use PR templates/checks for risky paths.
- Monitor CR conditions and operator logs for repeated auth/spec failures.

### Recovery planning

- Define a rollback workflow: revert commit, reconcile, verify.
- Back up secret storage data for file-backed providers.
- Test failure paths (bad secret ref, bad auth, branch mismatch) in non-prod.

## Performance and scale

### Scale dimensions

Performance depends on:

- Number of resources per sync scope
- API latency and rate limits
- Metadata complexity (transforms and selectors)
- Reconcile frequency

### Practical tuning

- Keep `SyncPolicy.source.path` scopes focused; avoid one giant policy.
- Use reasonable `syncInterval` values for API capacity.
- Configure `ManagedService.http.requestThrottling` for bounded concurrency.
- Use incremental-friendly change patterns: small, scoped commits.

### Repository and policy design

- Split large domains into multiple non-overlapping `SyncPolicy` paths.
- Avoid frequent churn in identity fields and logical path structures.
- Keep metadata minimal and targeted to required overrides.

### API pressure control

- Start with conservative concurrency and queue settings.
- Measure failed/queued requests before increasing throughput.
- Use full-resync cadence carefully; frequent full syncs increase load.

### Capacity strategy

1. Baseline with one narrow scope.
2. Increase scope count gradually.
3. Add policy partitioning before raising concurrency aggressively.
4. Re-validate after metadata or API contract changes.
