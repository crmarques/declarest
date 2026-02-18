# Secret Management

## Purpose
Define secret lifecycle behavior for detection, masking, storage, resolution, and safety.

## In Scope
1. Secret placeholders and payload transformations.
2. Secret store abstractions and initialization.
3. Secret import/export and auditing semantics.
4. Safety and non-disclosure rules.

## Out of Scope
1. Remote API token issuance workflows.
2. CLI rendering details beyond safety guarantees.
3. Metadata inference internals.

## Normative Rules
1. Secret values MUST never be stored in cleartext resource files when masking is enabled.
2. Secret values MUST never be printed in logs, errors, diff output, or explain output.
3. Placeholder syntax MUST be deterministic and reversible.
4. Secret resolution MUST fail with typed errors for missing keys.
5. Secret operations on collection payloads MUST be rejected when key scope is ambiguous.
6. Secret normalization MUST occur before compare/diff to avoid false drift.
7. Secret store initialization MUST validate required credentials and encryption configuration.
8. `resource save` MUST fail by default when potential plaintext secrets are detected and MUST require explicit `--ignore` or `--handle-secrets` override to proceed.
9. `resource save --handle-secrets` MUST accept an optional comma-separated list of attributes; when omitted, all detected candidates MUST be handled.
10. `resource save --handle-secrets` MUST store handled plaintext values in the configured secret store with deterministic path-scoped keys and rewrite handled payload values to `{{secret .}}` placeholders before repository persistence.
11. When resolving resource payload placeholders, `{{secret .}}` MUST map to `<logical-path>:<attribute-path>` and `{{secret <custom-key>}}` MUST map to `<logical-path>:<custom-key>`.
12. Resource placeholder resolution MUST continue to accept legacy absolute key placeholders (for example `{{secret "/customers/acme:apiToken"}}`) without rewriting failures.
13. When `resource save --handle-secrets` handles only a subset of detected candidates, the command MUST fail with plaintext-secret warning including only remaining unhandled candidates unless `--ignore` is set.
14. Metadata `secretsFromAttributes` entries MUST be treated as explicit secret candidates in save-time plaintext checks.
15. `secret detect` without payload input (`--file` or stdin) MUST scan local repository resources recursively under requested path, defaulting to `/` when no path is provided.
16. `secret detect --fix` MUST merge detected attributes into metadata `secretsFromAttributes` for detected resource paths in scope.
17. `secret detect --fix` with payload input MUST fail with `ValidationError` when no target path is provided.
18. `secret detect --secret-attribute` MUST restrict apply behavior to one detected attribute and MUST fail with `ValidationError` when the attribute is not detected in payload or repository scope.

## Data Contracts
Placeholder syntax:
1. `{{secret .}}` for implicit key resolution.
2. `{{secret <key-name>}}` for explicit key override.
3. Legacy quoted explicit keys (for example `{{secret "key-name"}}`) remain valid.

Secret manager method families:
1. Lifecycle: `Init`.
2. CRUD: `Store/Get/Delete/List`.
3. Transform: `MaskPayload/ResolvePayload/NormalizeSecretPlaceholders`.
4. Discovery: `DetectSecretCandidates`.

Store contracts:
1. File store with encrypted payloads.
2. External vault store with authenticated access.

## Failure Modes
1. Placeholder references non-existent key.
2. Secret store unavailable or unauthorized.
3. Payload masking attempts on unsupported structures.
4. Encryption key or passphrase misconfiguration.

## Edge Cases
1. Field contains literal text matching placeholder pattern but is not a secret.
2. Secret key rotation with existing masked payloads.
3. Mixed masked and unmasked values in one payload.
4. Importing secret archive with duplicate keys.
5. Metadata-declared secret attributes use non-secret-like names and still require plaintext enforcement.
6. `secret detect --fix` targets paths that have no metadata files yet; command creates metadata with `secretsFromAttributes`.

## Examples
1. `resource save --handle-secrets` at `/customers/acme` stores plaintext `apiToken` as key `/customers/acme:apiToken` and writes `{{secret .}}` in the resource payload.
2. Apply operation resolves placeholders at execution time and keeps repository content masked.
3. Compare operation normalizes equivalent placeholders with different key naming conventions.
4. `resource save --handle-secrets=password` handles `password` and then fails with warning when another detected candidate like `apiToken` remains unhandled unless `--ignore` is set.
5. Save rejects plaintext at `credentials.authValue` when `secretsFromAttributes` includes that attribute and user omits both `--ignore` and `--handle-secrets`.
6. `secret detect /customers/acme --fix` writes detected attributes into `/customers/acme` metadata `secretsFromAttributes`.
7. `secret detect` without path scans the whole local repository and returns detected attributes grouped by logical resource path.
8. `resource save /admin/realms/master/clients --handle-secrets=secret` writes handled secret attributes to collection metadata path `/admin/realms/_/clients` and fails if other detected candidates remain unhandled.
9. `{{secret client-token}}` inside `/customers/acme` resolves secret key `/customers/acme:client-token`.
