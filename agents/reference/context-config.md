# Contexts and Configuration

## Purpose
Define the canonical context catalog schema, file location, validation rules, credential indirection model, and context-resolution behavior.

## In Scope
1. YAML context catalog structure.
2. Top-level reusable credentials and `credentialsRef` injection.
3. Validation and one-of constraints.
4. Catalog path resolution, runtime overrides, and environment placeholder expansion.

## Out of Scope
1. Operator CRD wire shapes.
2. Managed-service transport implementation details.
3. Secret payload values stored in repository resources.

## Normative Rules
1. Context catalogs MUST be stored at `~/.declarest/configs/contexts.yaml` by default.
2. Environment override `DECLAREST_CONTEXTS_FILE` MAY replace the default path.
3. Persisted YAML keys MUST use camelCase.
4. Catalog readers MUST reject legacy aliases and any other unknown YAML keys.
5. `contexts` MUST be a list of full context objects.
6. `currentContext` MUST reference an existing context when contexts are present, and MUST be empty when `contexts` is empty.
7. Context names MUST be unique and non-empty.
8. Every context MUST define at least one of `repository` or `managedService`.
9. Validation MUST fail fast when one-of blocks are missing or ambiguous.
10. Config precedence MUST be: runtime flags, environment placeholders, persisted context values, engine defaults.
11. Exact-match string values in the form `${ENV_VAR}` MUST resolve from the process environment before runtime validation, defaulting, and active-context resolution, while persisted YAML MUST keep the placeholder text unchanged.
12. `credentials` MAY be omitted; when present, credential names MUST be unique.
13. Each `credentials[*]` entry MUST define `name`, `username`, and `password`.
14. Each credential attribute (`username` or `password`) MUST be either a non-empty string or an object with `prompt: true`; `persistInSession` MAY be set only on the prompt object.
15. `persistInSession: true` MUST reuse prompted values across later `declarest` commands only when the shell session exported `DECLAREST_PROMPT_AUTH_SESSION_ID` (for example via `declarest context session-hook bash|zsh`) and `XDG_RUNTIME_DIR` is available.
16. When `persistInSession: true` is set, new prompt-auth cache files MUST be written only under `XDG_RUNTIME_DIR/declarest/prompt-auth/` and MUST NOT fall back to `~/.declarest/sessions`.
17. When a component defines `credentialsRef`, runtime MUST inject the referenced credential object into that location while omitting the credential `name` field.
18. Referenced prompt-backed credential attributes MUST prompt only when the owning component first needs that attribute at runtime.
19. Non-interactive execution MUST fail when a required prompt-backed credential attribute has no cached session value.
20. `context clean --credentials-in-session` MUST remove the detected prompt-auth session cache file under `XDG_RUNTIME_DIR/declarest/prompt-auth/` and any matching legacy `~/.declarest/sessions` cache file so later `declarest` commands in that shell session no longer reuse those cached values.
21. Persisted context YAML MUST use `basic.credentialsRef` for reusable basic-auth credentials; inline persisted username/password pairs in those context auth blocks are invalid.
22. Proxy blocks (`managedService.http.proxy`, `repository.git.remote.proxy`, `secretStore.vault.proxy`, `metadata.proxy`) MAY define any subset of `http`, `https`, `noProxy`, and `auth`; an empty `proxy: {}` block explicitly disables inherited or environment proxy settings for that component.
23. Proxy auth, when provided, MUST define `basic.credentialsRef`.
24. Proxy precedence MUST be: process environment (`HTTP_PROXY`, `HTTPS_PROXY`, `NO_PROXY`, plus lowercase aliases), then component-local proxy fields overriding only the fields they define, then shared-context inheritance where the first configured concrete proxy becomes the default for components that do not define their own proxy block.
25. `metadata` MUST define at most one source: `baseDir`, `bundle`, or `bundleFile`.
26. `metadata.baseDir` MUST default to the selected repository baseDir when all metadata sources are unset.
27. Persisted context YAML MUST omit `metadata.baseDir` when it equals repository baseDir.
28. When `managedService.http.openapi` is empty and `metadata.bundle` or `metadata.bundleFile` is configured, startup MUST resolve OpenAPI from bundle hints in order: `bundle.yaml declarest.openapi`, then peer `openapi.yaml` at the bundle root. The persisted `bundle.yaml` shape and its strict-decode plus compatibility-gate rules are owned by `agents/reference/metadata-bundle.md`.
29. `repository.git.remote.autoSync` MAY be omitted; when omitted, repository-mutation commands MUST treat it as enabled and only an explicit `false` disables automatic push behavior.
30. `managedService.http.healthCheck` MAY be configured as a relative path or an absolute `http|https` URL, and it MUST NOT include query parameters.
31. Changes to the canonical persisted context or catalog wire shape MUST update `schemas/context.schema.json` and `schemas/contexts.schema.json` in the same change.

## Data Contracts

### Catalog fields
1. `contexts`: list of context objects.
2. `currentContext`: active context name.
3. `defaultEditor`: optional editor command used by editor-opening CLI workflows.
4. `credentials`: optional list of reusable named credentials.

### Credential contract
1. `credentials[*].name` identifies the reusable credential.
2. `credentials[*].username` and `credentials[*].password` accept either a literal string or:
```yaml
prompt: true
persistInSession: true
```
3. `persistInSession` MAY be omitted and MUST default to `false`.
4. `persistInSession: true` SHOULD be paired with `eval "$(declarest context session-hook bash)"` or `eval "$(declarest context session-hook zsh)"` when cross-command reuse inside one shell session is desired.
5. `credentialsRef` is a placeholder object:
```yaml
credentialsRef:
  name: shared-name
```
6. Resolved components MUST behave as though the referenced credential content was inserted at that location, minus the credential `name`.

### Per-context fields
1. `name`.
2. optional `repository`.
3. optional `managedService`.
4. optional `secretStore`.
5. optional `metadata`; `metadata.proxy` participates in the shared proxy behavior.
6. optional `preferences`.

### Repository contract
1. Exactly one of `repository.git` or `repository.filesystem` MUST be set.
2. `repository.git.remote.auth` MUST define exactly one of `basic`, `ssh`, or `accessKey` when configured.
3. `repository.git.remote.auth.basic` MUST define `credentialsRef`.
4. `repository.git.remote.proxy` inherits the shared proxy when unset and an empty block disables the inherited proxy for git operations.

### Managed-service contract
1. `managedService` MUST define `http`.
2. `managedService.http.url` is required.
3. `managedService.http.auth` MUST define exactly one of `oauth2`, `basic`, or `customHeaders`.
4. `managedService.http.auth.basic` MUST define `credentialsRef`.
5. `managedService.http.auth.customHeaders[*]` entries MUST define `header` and `value`; `prefix` is optional.
6. `managedService.http.proxy` follows the shared proxy contract and uses `http` / `https` keys.
7. `managedService.http.healthCheck`, when omitted, defaults probe commands to the normalized `managedService.http.url` path.

### Secret-store contract
1. Exactly one of `secretStore.file` or `secretStore.vault` MUST be set.
2. `secretStore.file` MUST define exactly one of `key`, `keyFile`, `passphrase`, `passphraseFile`.
3. `secretStore.vault.auth` MUST define exactly one of `token`, `password`, or `appRole`.
4. `secretStore.vault.auth.password` MUST define `credentialsRef`; `mount` is optional and defaults to `userpass` at runtime.
5. `secretStore.vault.proxy` follows the shared proxy contract.

### Runtime override keys
1. `repository.git.local.baseDir`
2. `repository.filesystem.baseDir`
3. `managedService.http.url`
4. `managedService.http.healthCheck`
5. `managedService.http.proxy.http`
6. `managedService.http.proxy.https`
7. `managedService.http.proxy.noProxy`
8. `metadata.baseDir`
9. `metadata.bundle`
10. `metadata.bundleFile`

## Canonical YAML Template
```yaml
currentContext: dev

credentials:
  - name: shared
    username: change-me
    password: change-me
  - name: prompt-shared
    username:
      prompt: true
      persistInSession: true
    password:
      prompt: true
      persistInSession: true

contexts:
  - name: dev
    repository:
      git:
        local:
          baseDir: /path/to/repo
        remote:
          url: https://example.com/org/repo.git
          branch: main
          provider: github
          auth:
            basic:
              credentialsRef:
                name: shared
          proxy:
            http: http://proxy.example.com:3128
            https: http://proxy.example.com:3128
            noProxy: localhost,127.0.0.1
            auth:
              basic:
                credentialsRef:
                  name: prompt-shared

    managedService:
      http:
        url: https://example.com/api
        healthCheck: /health
        auth:
          basic:
            credentialsRef:
              name: shared

    secretStore:
      vault:
        address: https://vault.example.com
        auth:
          password:
            credentialsRef:
              name: prompt-shared
            mount: userpass

    metadata:
      bundle: keycloak-bundle:0.0.1
```

## Failure Modes
1. Duplicate context names.
2. Duplicate credential names.
3. Unknown YAML keys or legacy aliases.
4. Missing or ambiguous one-of blocks.
5. `credentialsRef.name` does not match any catalog credential.
6. A credential attribute uses a prompt object with `prompt: false`.
7. A required credential attribute resolves to empty.
8. `managedService.http.healthCheck` is invalid or includes query parameters.
9. Runtime override key is not in the supported override-key list.
10. A required `${ENV_VAR}` placeholder resolves to empty or invalid content.

## Edge Cases
1. Empty catalog with `contexts: []` and `currentContext: ""` is valid.
2. Repository-only contexts remain valid and still default `metadata.baseDir` from the repository.
3. A context may define only `managedService.http.proxy.noProxy` and inherit `http` / `https` from environment or the shared proxy.
4. `proxy: {}` disables inherited and environment proxy settings for that component only.
5. One credential may be reused by multiple components through different `credentialsRef` locations.
6. One credential may prompt for `username` while keeping a literal `password`, or the reverse.

## Examples
1. `ResolveContext({Name: "", Overrides: nil})` loads the context named by `currentContext`.
2. `ResolveContext({Name: "dev", Overrides: {"managedService.http.url":"https://staging.example.com"}})` applies the runtime override without mutating the catalog file.
3. A top-level credential named `prompt-shared` referenced by both `managedService.http.auth.basic` and `repository.git.remote.proxy.auth.basic` prompts each component only when first used.
4. `Validate` rejects a context where `managedService.http.auth.basic.credentialsRef.name` points to a missing catalog credential.
5. `declarest context clean --credentials-in-session` clears prompt-backed credential session cache files even when no current context is configured.
6. When `XDG_RUNTIME_DIR` is unavailable, `persistInSession: true` still allows reuse inside one running `declarest` process but later commands prompt again because no cross-command cache file is created.
