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
8. `resource save` MUST fail by default when non-metadata-declared potential plaintext secrets are detected and MUST require explicit `--allow-plaintext` or `--secret-attributes` override to proceed.
9. `resource save --secret-attributes` MUST accept an optional comma-separated list of attributes, MUST require a structured payload (`json|yaml`), and MUST reject text or octet-stream payloads with guidance toward `--secret`.
10. `resource save --secret-attributes` MUST store handled plaintext values in the configured secret store with deterministic path-scoped keys and rewrite handled payload values to `{{secret .}}` placeholders before repository persistence.
11. `resource save --secret` MUST store the entire encoded resource payload in the configured secret store under key `<logical-path>:.`, MUST persist only an exact root `{{secret .}}` placeholder in the repository, and MUST persist metadata `resource.secret: true`.
12. When resolving resource payload placeholders, `{{secret .}}` MUST map to `<logical-path>:<attribute-path>` for attribute-scoped placeholders, an exact whole-resource `{{secret .}}` payload MUST map to `<logical-path>:.`, and `{{secret <custom-key>}}` MUST map to `<logical-path>:<custom-key>`.
13. When `resource save --secret-attributes` handles only a subset of detected candidates, the command MUST fail with plaintext-secret warning including only remaining unhandled candidates that are not declared in metadata, except when `--allow-plaintext` is set.
14. Metadata `resource.secretAttributes` entries MUST be treated as explicit secret candidates in detection and handling flows and MUST be automatically stored and masked during default `resource save` persistence workflows.
15. Metadata `resource.secret: true` MUST declare whole-resource secret handling for that logical scope, MUST be mutually exclusive with `resource.secretAttributes`, and MUST cause default single-resource saves to use whole-resource secret storage even when the user omits `--secret`.
16. `resource save --allow-plaintext` MUST bypass plaintext-secret save enforcement for all remaining candidates.
17. For collection/group saves with `resource save --secret-attributes=<attribute-list>`, each requested attribute MUST be applied only to resources where it is present; resources without the attribute MUST be skipped for that attribute without failing the command.
18. `resource get` MUST redact values for metadata `resource.secretAttributes` as `{{secret .}}` placeholders by default for repository and managed-server output modes, and MUST redact whole-resource secret scopes to the same placeholder when `resource.secret: true` applies.
19. `resource get --show-secrets` MUST disable metadata-driven output redaction and print plaintext values for both attribute-level and whole-resource secrets.
20. `secret detect` without payload input (`--payload <path|->` or stdin) MUST scan local repository resources recursively under requested path, defaulting to `/` when no path is provided.
21. `secret detect --fix` MUST merge detected attributes into metadata `resource.secretAttributes` for detected resource paths in scope.
22. `secret detect --fix` with payload input MUST fail with `ValidationError` when no target path is provided.
23. `secret detect --secret-attribute` MUST restrict apply behavior to one detected attribute and MUST fail with `ValidationError` when the attribute is not detected in payload or repository scope.
24. Secret-candidate detection MUST ignore numeric-only and boolean-like plaintext values to avoid policy/lifespan and feature-toggle false positives on non-secret fields.

## Data Contracts
Placeholder syntax:
1. `{{secret .}}` for implicit key resolution.
2. `{{secret <key-name>}}` for explicit key override.
3. Legacy quoted explicit keys (for example `{{secret "key-name"}}`) remain valid.

Whole-resource store contract:
1. `resource save --secret` or metadata-driven whole-resource secret persistence uses path-scoped key `<logical-path>:.`.
2. UTF-8 whole-resource secret values SHOULD remain directly readable in the secret store; non-UTF-8 content MAY use a deterministic reversible encoding as long as decode behavior remains descriptor-aware.

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
5. `resource save` detects metadata-declared plaintext secret candidates but no secret store provider is configured.
6. `resource save --secret` or metadata-driven whole-resource secret persistence is invoked without a configured secret store provider.
7. `resource save --secret` is invoked without a configured metadata service to persist `resource.secret: true`.

## Edge Cases
1. Field contains literal text matching placeholder pattern but is not a secret.
2. Secret key rotation with existing masked payloads.
3. Mixed masked and unmasked values in one payload.
4. Importing secret archive with duplicate keys.
5. Metadata-declared secret attributes use non-secret-like names and still require automatic save-time masking and storage.
6. `secret detect --fix` targets paths that have no metadata files yet; command creates metadata with `resource.secretAttributes`.

## Examples
1. `resource save --secret-attributes` at `/customers/acme` stores plaintext `apiToken` as key `/customers/acme:apiToken` and writes `{{secret .}}` in the resource payload.
2. Apply operation resolves placeholders at execution time and keeps repository content masked.
3. Compare operation normalizes equivalent placeholders with different key naming conventions.
4. `resource save --secret-attributes=password` handles `password` and then fails with warning when another non-metadata-declared detected candidate like `apiToken` remains unhandled unless `--allow-plaintext` is set.
5. Save auto-stores and masks plaintext at `credentials.authValue` when `resource.secretAttributes` includes that attribute, even when the user omits both `--allow-plaintext` and `--secret-attributes`.
6. `secret detect /customers/acme --fix` writes detected attributes into `/customers/acme` metadata `resource.secretAttributes`.
7. `secret detect` without path scans the whole local repository and returns detected attributes grouped by logical resource path.
8. `resource save /admin/realms/master/clients --secret-attributes=secret` writes handled secret attributes to collection metadata path `/admin/realms/_/clients`, skips resources without `secret`, and fails if other non-metadata-declared candidates remain unhandled.
9. `{{secret client-token}}` inside `/customers/acme` resolves secret key `/customers/acme:client-token`.
10. `actionTokenGeneratedByUserLifespan.reset-credentials: "43200"` is not treated as a secret candidate by default detection.
11. `access.token.claim: true` and `token.response.type.bearer.lower-case: false` are not treated as secret candidates by default detection.
12. `resource get /customers/acme` redacts `password` to `{{secret .}}` when metadata for `/customers/acme` includes `resource.secretAttributes: [password]`.
13. `resource get /customers/acme --show-secrets` prints plaintext `password` even when metadata declares that attribute as secret.
14. `resource save /customers/acme` fails with `ValidationError` when metadata declares `password` as secret and no secret store provider is configured.
15. `resource save /projects/platform/secrets/private-key --payload private.key --secret` stores the full key content under `/projects/platform/secrets/private-key:.`, persists `resource.secret: true`, leaves `resource.key` containing only `{{secret .}}`, and `resource apply /projects/platform/secrets/private-key` resolves the placeholder back to the original `.key` payload.
16. `resource.secret: true` on `/projects/platform/secrets/private-key` causes a later plain `resource save /projects/platform/secrets/private-key --payload private.key` to store the whole payload in the secret store without requiring `--secret` again.
