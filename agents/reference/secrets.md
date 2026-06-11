# Secret Management

## Purpose
Define the secret lifecycle: detection, masking, `{{secret .}}` key mapping, whole-resource vs attribute secrets, store contracts, and output-redaction defaults.

## Scope
Owns secret-handling behavior. The CLI flag/command grammar (`resource save --secret|--secret-attributes|--allow-plaintext|--force`, `resource get --show-secrets`, `secret set|get|list|delete`) is owned by cli.md; this file defines the behavior those flags enforce. Secret-provider method families and the error taxonomy are owned by interfaces.md. The `secretStore` config schema (file/vault, encryption keys, auth) is owned by context-config.md. Metadata declaration and validation of `resource.secret`/`resource.secretAttributes` (including format compatibility) are owned by metadata.md; this file maps those declarations to runtime detection, storage, and redaction.

## Normative Rules

### Safety invariants
1. Secret values MUST NOT be persisted in cleartext repository files when masking applies.
2. Secret values MUST NOT appear in logs, errors, diff output, or explain output.
3. `secret list` MUST return keys only and MUST NOT print plaintext values; plaintext output (`secret get`) MUST require explicit key selection — path-only discovery MUST route through `secret list`.

### Placeholder mapping (owned)
4. Placeholder syntax: `{{secret .}}` resolves an implicit key; `{{secret <custom-key>}}` overrides the key explicitly; the legacy quoted form `{{secret "<custom-key>"}}` remains valid.
5. When resolving resource payload placeholders: an attribute-scoped `{{secret .}}` MUST map to `<logical-path>:<attribute-json-pointer>`; an exact whole-resource `{{secret .}}` payload MUST map to `<logical-path>:.`; `{{secret <custom-key>}}` MUST map to `<logical-path>:<custom-key>`.
6. Placeholders MUST be resolved at request/apply execution time; repository content MUST remain masked.
7. Placeholder normalization MUST run before compare/diff so that equivalent placeholders with different key-naming conventions do not produce false drift.
8. Secret resolution MUST fail with a typed error (`NotFoundError` for a missing key, `AuthError`/store error for an unavailable store) per interfaces.md.

### Detection (owned)
9. Secret-candidate detection MUST ignore numeric-only and boolean-like plaintext values (avoids false positives on lifespans, policies, and feature toggles).
10. Metadata `resource.secretAttributes` entries MUST be treated as explicit secret candidates in detection and handling, regardless of whether the attribute name looks secret-like.
11. For collection/list saves, plaintext-candidate detection MUST be computed once from the full payload set and applied consistently across all items.

### Masking and storage at save time (behavior; flag grammar in cli.md)
12. `resource save` MUST automatically store and mask plaintext candidates declared by metadata `resource.secretAttributes` before repository persistence, even when no override flag is supplied.
13. `resource save` MUST fail with `ValidationError` when non-metadata-declared plaintext candidates are detected, unless `--allow-plaintext` (bypass all remaining candidates) or `--secret-attributes` (handle the listed/detected candidates) is set.
14. Attribute-scoped handling MUST store each plaintext value in the configured secret store under a deterministic path-scoped key `<logical-path>:<attribute-json-pointer>`, rewrite the payload value to `{{secret .}}` before persistence, and merge the handled JSON Pointer attributes into metadata `resource.secretAttributes` for the saved logical path.
15. When `--secret-attributes` handles only a subset of detected candidates, the command MUST fail with the plaintext-secret warning listing only the remaining non-metadata-declared candidates, unless `--allow-plaintext` is set.
16. For collection/group saves with an attribute list, each requested attribute MUST be applied only to resources where it is present; resources lacking the attribute MUST be skipped for that attribute without failing the command. A secret operation on a collection payload whose key scope is ambiguous (it cannot be resolved to a single logical resource) MUST be rejected with `ValidationError`.

### Whole-resource secrets (owned)
17. `resource save --secret`, and metadata `resource.secret: true`, MUST store the entire encoded resource payload in the secret store under key `<logical-path>:.`, persist only an exact root `{{secret .}}` placeholder in the repository (preserving the original payload descriptor and file suffix), and persist metadata `resource.secret: true`.
18. Metadata `resource.secret: true` MUST cause default single-resource saves to use whole-resource secret storage even when the user omits `--secret`. Mutual exclusivity with `resource.secretAttributes` is enforced by metadata.md.
19. UTF-8 whole-resource secret values SHOULD remain directly readable in the store; non-UTF-8 content MAY use a deterministic reversible encoding, provided decode behavior remains descriptor-aware.

### Output redaction (owned)
20. `resource get` MUST redact metadata `resource.secretAttributes` values to `{{secret .}}` by default for both repository and managed-service output, and MUST redact the whole-resource scope to the same placeholder when `resource.secret: true` applies.
21. `resource get --show-secrets` MUST disable redaction and print plaintext for both attribute-level and whole-resource secrets.

### `secret detect` behavior
22. `secret detect` without payload input MUST scan local repository resources recursively under the requested path, defaulting to `/`, and MUST group results by logical resource path.
23. `secret detect --fix` MUST merge detected attributes into metadata `resource.secretAttributes` for each detected path in scope, creating metadata where none exists yet.
24. `secret detect --fix` with payload input MUST fail with `ValidationError` when no target path is provided.
25. `secret detect --secret-attribute <pointer>` MUST restrict apply to that one attribute and MUST fail with `ValidationError` when it is not detected in the payload or repository scope.

### Store contracts
26. Supported stores: a file store with encrypted payloads, and an external vault store with authenticated access. Store initialization MUST validate required credentials and encryption configuration (schema in context-config.md) and fail with a typed error otherwise.

## Failure Modes
1. Placeholder references a non-existent key, or the store is unavailable/unauthorized -> typed error (rules 8).
2. `resource save` selects plaintext candidates for handling (metadata-declared or `--secret-attributes`) but no secret provider is configured -> `ValidationError`.
3. `resource save --secret` / metadata-driven whole-resource persistence runs without a configured secret provider, or without a metadata service to persist `resource.secret: true` -> `ValidationError`.
4. Masking attempted on an unsupported (non-structured) payload for attribute scope -> `ValidationError` with guidance toward `--secret` (validation owned by metadata.md/cli.md).

## Edge Cases
1. A field contains literal text matching the placeholder pattern but is not a secret; treat as plaintext, not a stored secret.
2. Secret key rotation: existing masked payloads keep resolving by key, so rotation updates the store value only.
3. A payload mixes masked placeholders and plaintext secret-like values; detection still flags the plaintext ones.
4. Metadata declares a non-secret-looking attribute name as a secret attribute; save-time masking/storage still applies (rule 10).

## Examples
1. `resource save --secret-attributes` at `/customers/acme` stores `apiToken` as key `/customers/acme:/apiToken` and writes `{{secret .}}` into the payload.
2. `resource save /customers/acme --secret-attributes=/password` handles `/password`, then fails warning that non-declared `/apiToken` remains unhandled, unless `--allow-plaintext` is set.
3. `resource save /admin/realms/master/clients --secret-attributes=/secret` writes handled attributes to collection metadata path `/admin/realms/_/clients`, skips resources without `/secret`, and fails on other non-declared candidates.
4. Save auto-stores and masks `/credentials/authValue` because metadata `resource.secretAttributes` includes it, with no flag supplied; it fails with `ValidationError` if no secret provider is configured.
5. `resource save /projects/platform/secrets/private-key --payload private.key --secret` stores the full content under `/projects/platform/secrets/private-key:.`, persists `resource.secret: true`, leaves `resource.key` as `{{secret .}}`; a later plain save of the same path stores the whole payload without re-passing `--secret`, and `resource apply` resolves the placeholder back to the original `.key` payload.
6. `resource save /projects/test/secrets/pass-word --payload a=b --content-type txt --secret` stores literal `a=b` under `/projects/test/secrets/pass-word:.` and preserves the `.txt` repository suffix.
7. `{{secret client-token}}` inside `/customers/acme` resolves key `/customers/acme:client-token`.
8. `resource get /customers/acme` redacts `password` to `{{secret .}}` when metadata declares `resource.secretAttributes: [/password]`; `--show-secrets` prints the plaintext.
9. `secret detect /customers/acme --fix` writes detected attributes into `/customers/acme` metadata; `secret list /projects --recursive` prints descendant entries such as `/test/secrets/private-key:.`, while `secret get /customers/acme` (no key) fails with `ValidationError` directing the user to `secret list`.
10. Non-secret values like `actionTokenGeneratedByUserLifespan.reset-credentials: "43200"`, `access.token.claim: true`, and `token.response.type.bearer.lower-case: false` are not treated as secret candidates by default detection.
