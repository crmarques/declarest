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
8. `resource save` MUST fail by default when potential plaintext secrets are detected and MUST require explicit `--ignore` override to proceed.
9. Metadata `secretsFromAttributes` entries MUST be treated as explicit secret candidates in save-time plaintext checks.
10. `secret detect` without payload input (`--file` or stdin) MUST scan local repository resources recursively under requested path, defaulting to `/` when no path is provided.
11. `secret detect --fix` MUST merge detected attributes into metadata `secretsFromAttributes` for detected resource paths in scope.
12. `secret detect --fix` with payload input MUST fail with `ValidationError` when no target path is provided.
13. `secret detect --secret-attribute` MUST restrict apply behavior to one detected attribute and MUST fail with `ValidationError` when the attribute is not detected in payload or repository scope.

## Data Contracts
Placeholder syntax:
1. `{{secret .}}` for current field key.
2. `{{secret "key-name"}}` for explicit key.

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
1. Save with masking enabled stores `apiToken` in secret store and writes `{{secret "apiToken"}}` in resource payload.
2. Apply operation resolves placeholders at execution time and keeps repository content masked.
3. Compare operation normalizes equivalent placeholders with different key naming conventions.
4. Save rejects plaintext at `credentials.authValue` when `secretsFromAttributes` includes that attribute and user omits `--ignore`.
5. `secret detect /customers/acme --fix` writes detected attributes into `/customers/acme` metadata `secretsFromAttributes`.
6. `secret detect` without path scans the whole local repository and returns detected attributes grouped by logical resource path.
