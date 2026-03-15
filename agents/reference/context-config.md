# Contexts and Configuration

## Purpose
Define the canonical context catalog schema, file location, validation rules, and context resolution behavior.

## In Scope
1. YAML context catalog structure.
2. Context lifecycle operations and current context selection.
3. Validation and one-of constraints.
4. Config file path resolution and overrides.

## Out of Scope
1. Secret payload values inside resource files.
2. HTTP transport provider implementation details.
3. CLI completion internals.

## Normative Rules
1. Context catalogs MUST be stored at `~/.declarest/configs/contexts.yaml` by default.
2. Environment override `DECLAREST_CONTEXTS_FILE` MAY replace the default path.
3. Persisted YAML keys MUST use camelCase.
4. On-disk catalog readers MUST accept documented legacy aliases (for example `current-ctx`, `base-dir`, `managed-server`, and `secret-store`) and MUST normalize them before strict validation.
5. Unknown YAML keys MUST fail parsing after legacy-alias normalization.
6. `contexts` MUST be a list of full context objects.
7. `currentContext` MUST reference an existing context.
8. Context names MUST be unique and non-empty.
9. Validation MUST fail fast when one-of blocks are missing or ambiguous.
10. Config precedence MUST be: runtime flags, environment overrides, persisted context values, engine defaults.
11. Exact-match string values in the form `${ENV_VAR}` MUST resolve from the process environment before runtime validation, defaulting, and active-context resolution, while persisted YAML MUST keep the placeholder text unchanged.
12. Unknown override keys MUST fail validation.
13. Missing context catalog files MUST be treated as an empty catalog state.
14. `metadata` MUST define at most one source: `baseDir`, `bundle`, or `bundleFile`.
15. `metadata.baseDir` MUST default to the selected repository baseDir when all metadata sources are unset.
16. Persisted context YAML MUST omit `metadata.baseDir` when it equals repository baseDir.
17. When a repository baseDir exists and the resolved metadata source baseDir differs from that repository baseDir, runtime MUST merge the shared metadata source with repo-local metadata sidecars rooted at the repository baseDir; repo-local overlays remain highest priority for resolution, but the writable target MUST be the explicit `metadata.baseDir` source when configured and MUST be the repo-local tree when the shared source comes from `metadata.bundle` or `metadata.bundleFile`.
18. Every context MUST define at least one of `repository` or `managedServer`.
19. Catalog-level `defaultEditor` MAY be omitted and MUST default to `vi` when editor-opening CLI commands resolve no explicit `--editor` override.
20. Catalog edit workflows that replace the full YAML document (for example `context edit`) MUST validate strict YAML and context semantics before persisting any file changes.
21. When `managedServer.http.openapi` is empty and `metadata.bundle` or `metadata.bundleFile` is configured, startup MUST resolve OpenAPI from bundle hints in order: `bundle.yaml declarest.openapi`, then peer `openapi.yaml` at the bundle root.
22. Proxy blocks (`managedServer.http.proxy`, `repository.git.remote.proxy`, `secretStore.vault.proxy`, `metadata.proxy`) MAY define any subset of `httpURL`, `httpsURL`, `noProxy`, and `auth`; proxy auth (when provided) MUST define either both `username` and `password` or one `prompt` block, and an empty `proxy:` block explicitly disables inherited or environment proxy settings for that component.
23. Proxy precedence MUST be: process environment (`HTTP_PROXY`, `HTTPS_PROXY`, `NO_PROXY`, plus lowercase aliases), then component-local proxy fields overriding only the fields they define, then shared-context inheritance where the first configured concrete proxy becomes the default for components that do not define their own proxy block.
24. `managedServer.http.openapi` MAY reference either an OpenAPI 3.x (`openapi`) or Swagger 2.0 (`swagger`) document.
25. `managedServer.http.healthCheck` MAY be configured as a relative path or an absolute `http|https` URL, and it MUST NOT include query parameters.
26. `repository.git.remote.autoSync` MAY be omitted; when omitted, repository-mutation commands MUST treat it as enabled and only an explicit `false` disables automatic push behavior.
27. When `managedServer` is present, it MUST define `managedServer.http` with exactly one configured auth mode.
28. Prompt auth blocks (`repository.git.remote.auth.prompt`, `managedServer.http.auth.prompt`, `secretStore.vault.auth.prompt`, and `*.proxy.auth.prompt`) MUST collect credentials only when the owning component first needs them at runtime, MUST ask whether the first entered prompt credentials should be reused for other prompt-auth components in the same command, and MUST warn during entry when `keepCredentialsForSession=true`.
29. `keepCredentialsForSession` MAY be omitted and MUST default to `false`.
30. Changes to the canonical persisted context or catalog wire shape, one-of blocks, or schema-managed validation fields MUST update `schemas/context.schema.json` and `schemas/contexts.schema.json` in the same change; those schemas MUST describe canonical camelCase keys only and MUST NOT add legacy aliases.

## Data Contracts
Top-level catalog fields:
1. `contexts`: list of context objects.
2. `currentContext`: active context name.
3. optional `defaultEditor`: editor command used by CLI editor workflows when `--editor` is not provided.

Per-context fields:
1. `name`.
2. optional `repository`.
3. optional `managedServer`.
4. optional `secretStore`.
5. optional `metadata` (omit when equivalent to default repository baseDir behavior); `metadata.proxy` configures the HTTP proxy used for bundle downloads and participates in the shared proxy semantics.
6. optional `preferences`.
7. at least one of `repository` or `managedServer` is required.

Repository one-of contract:
1. Exactly one of `repository.git` or `repository.filesystem` MUST be set.
2. `repository.git.remote.proxy` MAY be used to configure HTTP/HTTPS proxies for git fetch/push flows; it inherits the shared proxy when unset and an empty block disables the inherited proxy for git operations.
3. `repository.git.remote.autoSync` defaults to `true` when omitted.
4. `repository.git.remote.auth` MUST define exactly one of `basicAuth`, `ssh`, `accessKey`, or `prompt` when configured.

Resource server auth one-of contract:
1. Exactly one of `oauth2`, `basicAuth`, `customHeaders`, or `prompt` MUST be set under `managedServer.http.auth`.
2. `managedServer.http.auth.customHeaders` MUST contain at least one entry.
3. Each `managedServer.http.auth.customHeaders[*]` entry MUST define `header` and `value`; it MAY define `prefix`, which is prepended as `<prefix> <value>`.
4. `managedServer.http.auth.prompt` resolves one runtime username/password pair and applies it as HTTP basic auth.

Resource server proxy contract:
1. `managedServer.http.proxy` MAY define `httpURL` and/or `httpsURL`.
2. `managedServer.http.proxy.noProxy` MAY define comma-separated bypass rules.
3. `managedServer.http.proxy.auth` MAY be configured; when set, it MUST define either both `username` and `password` or one `prompt` block.
4. The same `proxy` structure is available for `repository.git.remote.proxy`, `secretStore.vault.proxy`, and `metadata.proxy`; each block can override only selected fields from the process environment, and unset components inherit the first configured concrete proxy unless an empty `proxy:` block explicitly disables it for that component.

Resource server healthCheck contract:
1. `managedServer.http.healthCheck` MAY be omitted; when omitted, probe commands target `managedServer.http.baseURL` itself and reuse its normalized path.
2. Relative `managedServer.http.healthCheck` values MUST be normalized as managedServer request paths.
3. Absolute `managedServer.http.healthCheck` values MUST use `http` or `https` and MUST share scheme/host with `managedServer.http.baseURL`.
4. `managedServer.http.healthCheck` MUST NOT include query parameters.

Secret store one-of contracts:
1. Exactly one of `secretStore.file` or `secretStore.vault` MUST be set.
2. For `secretStore.file`, exactly one of `key`, `keyFile`, `passphrase`, `passphraseFile` MUST be set.
3. `secretStore.vault.auth` MUST define exactly one of `token`, `password`, `appRole`, or `prompt`.
4. `secretStore.vault.proxy` MAY configure HTTP/HTTPS proxies for Vault operations and follows the shared proxy inheritance rules; use an empty `proxy:` block to opt out.

Context manager operations:
1. `Create/Update/Delete/Rename/List`.
2. `SetCurrent/GetCurrent`.
3. `ResolveContext`.
4. `Validate`.

Runtime override keys:
1. `repository.git.local.baseDir`.
2. `repository.filesystem.baseDir`.
3. `managedServer.http.baseURL`.
4. `managedServer.http.healthCheck`.
5. `managedServer.http.proxy.httpURL`.
6. `managedServer.http.proxy.httpsURL`.
7. `managedServer.http.proxy.noProxy`.
8. `managedServer.http.proxy.auth.username`.
9. `managedServer.http.proxy.auth.password`.
10. `metadata.baseDir`.
11. `metadata.bundle`.
12. `metadata.bundleFile`.

## Canonical YAML Template
```yaml
contexts:
  - name: xxx
    repository:
      # Choose exactly one repository type: filesystem or git.
      git:
        local:
          baseDir: /path/to/repo
        # remote:
        #   url: https://example.com/org/repo.git
        #   branch: main
        #   provider: github
        #   autoSync: true
        #   auth:
        #     # Choose exactly one auth method: basicAuth, prompt, ssh, accessKey.
        #     basicAuth:
        #       username: change-me
        #       password: change-me
        #     prompt:
        #       keepCredentialsForSession: true
        #     ssh:
        #       user: git
        #       privateKeyFile: /path/to/id_rsa
        #       passphrase: change-me
        #       knownHostsFile: /path/to/known_hosts
        #       insecureIgnoreHostKey: false
        #     accessKey:
        #       token: change-me
        #   tls:
        #     insecureSkipVerify: false
        #   proxy:
        #     httpURL: http://proxy.example.com:3128
        #     httpsURL: http://proxy.example.com:3128
        #     noProxy: localhost,127.0.0.1
        #     auth:
        #       username: proxy-user
        #       password: proxy-pass
        #       # prompt:
        #       #   keepCredentialsForSession: true
      # filesystem:
      #   baseDir: /path/to/repo

    managedServer:
      http:
        baseURL: https://example.com/api
        # healthCheck: /health
        # openapi: /path/to/openapi-or-swagger.yaml
        # defaultHeaders:
        #   X-Example: value
        # proxy:
        #   httpURL: http://proxy.example.com:3128
        #   httpsURL: http://proxy.example.com:3128
        #   noProxy: localhost,127.0.0.1
        #   auth:
        #     username: proxy-user
        #     password: proxy-pass
        auth:
          # Choose exactly one auth method: oauth2, basicAuth, prompt, customHeaders.
          oauth2:
            tokenURL: https://example.com/oauth/token
            grantType: client_credentials
            clientID: change-me
            clientSecret: change-me
            # username: change-me
            # password: change-me
            # scope: api.read
            # audience: https://example.com/
          # basicAuth:
          #   username: change-me
          #   password: change-me
          # prompt:
          #   keepCredentialsForSession: true
          # customHeaders:
          #   - header: Authorization
          #     prefix: Bearer
          #     value: change-me
        # tls:
        #   insecureSkipVerify: false

    secretStore:
      # Choose exactly one: file or vault.
      file:
        path: /path/to/secrets.json
        # Choose exactly one: key, keyFile, passphrase, passphraseFile.
        passphrase: change-me
        # key: base64-encoded-key
        # keyFile: /path/to/key.txt
        # passphraseFile: /path/to/passphrase.txt
        # kdf:
        #   time: 1
        #   memory: 65536
        #   threads: 4
      # vault:
      #   address: https://vault.example.com
      #   mount: secret
      #   pathPrefix: declarest
      #   kvVersion: 2
      #   auth:
      #     token: s.xxxx
      #     # password:
      #     #   username: vault-user
      #     #   password: vault-pass
      #     #   mount: userpass
      #     # prompt:
      #     #   keepCredentialsForSession: true
      #     #   mount: userpass
      #     # appRole:
      #     #   roleID: roleID
      #     #   secretID: secretID
      #     #   mount: appRole
      #   tls:
      #     caCertFile: /path/to/ca.pem
      #     clientCertFile: /path/to/client.pem
      #     clientKeyFile: /path/to/client-key.pem
      #     insecureSkipVerify: false
      #   proxy:
      #     httpURL: http://proxy.example.com:3128
      #     httpsURL: http://proxy.example.com:3128
      #     noProxy: localhost,127.0.0.1
      #     auth:
      #       username: proxy-user
      #       password: proxy-pass
      #       # prompt:
      #       #   keepCredentialsForSession: true

    metadata:
      # Metadata source defaults to repository baseDir when both are unset.
      # Choose at most one metadata source.
      # baseDir: /path/to/metadata
      # bundle: keycloak-bundle:0.0.1
      # bundleFile: /path/to/keycloak-bundle-0.0.1.tar.gz
      # proxy:
      #   httpURL: http://proxy.example.com:3128
      #   httpsURL: http://proxy.example.com:3128
      #   noProxy: localhost,127.0.0.1
      #   auth:
      #     username: proxy-user
      #     password: proxy-pass

  - name: yyy
    repository:
      filesystem:
        baseDir: /other/repo

currentContext: xxx
# defaultEditor: vi
```

## Failure Modes
1. `currentContext` missing or not found in `contexts`.
2. Duplicate context names.
3. Unknown YAML key due to strict decode after legacy-alias normalization.
4. Repository backend one-of violation.
5. Both `repository` and `managedServer` missing.
6. Resource server auth one-of violation.
7. Secret store one-of violation.
8. Secret file key source one-of violation.
9. Metadata source one-of violation (multiple metadata sources set, for example `metadata.baseDir` with `metadata.bundle`).
10. Config path resolution failure for home expansion or file access.
11. Runtime override key not in the supported override-key list.
12. Composition root startup (`bootstrap.NewSession`) fails when neither `selection.name` nor `currentContext` resolves to a valid context.
13. a proxy block defines incomplete auth credentials, or a resolved proxy URL is invalid.
14. `managedServer.http.healthCheck` is configured with query parameters or an invalid URL form.
15. a runtime `${ENV_VAR}` placeholder resolves to an empty or invalid required value and active-context resolution fails validation.

## Edge Cases
1. Empty catalog with no contexts and no current context.
2. Context with optional `secretStore` omitted.
3. Repository-only context with `managedServer` omitted remains valid for local-only workflows.
4. Runtime override targets a missing optional block.
5. Catalog file absent on first run; list returns empty and current/resolve report `current context not set`.
6. `metadata.baseDir` omitted in YAML; resolve still returns repository baseDir as effective metadata baseDir.
7. `metadata.bundle` configured; resolve keeps `metadata.baseDir` empty and startup resolves metadata from the bundle cache.
8. `metadata.bundle` provides `declarest.openapi` or peer `openapi.yaml`; startup wires that OpenAPI source only when context `managedServer.http.openapi` is unset.
9. `metadata.bundleFile` configured; resolve keeps `metadata.baseDir` empty and startup resolves metadata from the local bundle archive.
10. `defaultEditor` omitted in YAML; editor-opening CLI commands still resolve `vi` by default.
11. `managedServer.http.proxy.noProxy` can be set without proxy auth and still remains valid.
12. a context can define only `managedServer.http.proxy.noProxy` and inherit `httpURL` / `httpsURL` from environment variables or the shared proxy default.
13. `managedServer.http.healthCheck` can be absolute and still resolves to a managedServer-relative probe when scheme/host match `managedServer.http.baseURL`.
14. one command can resolve prompt auth for one component, reject reuse for the others, and still prompt those other components independently later in the same command.

## Examples
1. `ResolveContext({Name: "", Overrides: nil})` loads the context named by `currentContext`.
2. `SetCurrent("yyy")` updates `currentContext` and preserves context list order.
3. `Validate` rejects a config that defines both `repository.git` and `repository.filesystem`.
4. Corner case: `ResolveContext({Name: "dev", Overrides: {"unknown.key":"x"}})` fails with a validation error for unknown override keys.
5. Corner case: `ResolveContext({Name: "dev", Overrides: {"metadata.bundle":"keycloak-bundle:0.0.1"}})` resolves bundle metadata source and clears `metadata.baseDir`.
6. Corner case: `ResolveContext({Name: "dev", Overrides: {"metadata.bundleFile":"/tmp/keycloak-bundle-0.0.1.tar.gz"}})` resolves local bundle metadata source and clears both `metadata.baseDir` and `metadata.bundle`.
7. `List()` on a missing catalog file returns `[]`; `GetCurrent()` returns `NotFoundError` with `current context not set`.
8. `bootstrap.NewSession(..., ContextSelection{})` returns `NotFoundError` when `currentContext` is not set.
9. `context edit prod` loads only context `prod` into a temporary document, validates the edited YAML, and replaces only that context in the persisted catalog when validation succeeds.
10. Corner case: `managedServer.http.auth.customHeaders` with one entry that defines `header` + `value` and no `prefix` remains valid and sends the raw `value` in the configured header.
11. Corner case: `ResolveContext({Name: "dev", Overrides: nil})` with empty `managedServer.http.openapi` and bundle metadata source (`metadata.bundle` or `metadata.bundleFile`) that includes `openapi.yaml` keeps context config unchanged while startup wiring resolves OpenAPI from the extracted bundle.
12. Corner case: `ResolveContext({Name: "dev", Overrides: {"managedServer.http.proxy.httpURL":"http://proxy.example.com:3128"}})` applies the proxy override and keeps other proxy fields untouched when unset.
13. Corner case: `ResolveContext({Name: "dev", Overrides: nil})` with `HTTPS_PROXY=https://proxy.example.com:3128` in the environment and only `managedServer.http.proxy.noProxy=localhost` in the context resolves a managed-server proxy with inherited `httpsURL` plus overridden `noProxy`.
14. Corner case: a legacy catalog entry with `current-ctx` and `filesystem.base-dir` resolves as `currentContext` and `repository.filesystem.baseDir`, and persists back without the legacy keys.
15. Corner case: `ResolveContext({Name: "local-only", Overrides: nil})` with repository-only config and no `managedServer` remains valid, defaults `metadata.baseDir` from the repository, and bootstraps local-only commands without a remote client.
16. Corner case: `managedServer.http.auth.prompt` plus `repository.git.remote.auth.prompt` prompts once for the first used component, asks whether to reuse those credentials for the other prompt-auth component in the same command, and only stores component credentials for later commands when that component sets `keepCredentialsForSession: true`.
17. Corner case: adding a canonical field under `managedServer.http` or `metadata` requires the same change to update `schemas/context.schema.json` and `schemas/contexts.schema.json`, while documented legacy aliases remain reader-only and stay out of the schema files.
