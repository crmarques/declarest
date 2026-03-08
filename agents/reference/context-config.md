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
3. YAML keys MUST use camelCase.
4. Unknown YAML keys MUST fail parsing.
5. `contexts` MUST be a list of full context objects.
6. `currentCtx` MUST reference an existing context.
7. Context names MUST be unique and non-empty.
8. Validation MUST fail fast when one-of blocks are missing or ambiguous.
9. Config precedence MUST be: runtime flags, environment overrides, persisted context values, engine defaults.
10. Unknown override keys MUST fail validation.
11. Missing context catalog files MUST be treated as an empty catalog state.
12. `metadata` MUST define at most one source: `baseDir`, `bundle`, or `bundleFile`.
13. `metadata.baseDir` MUST default to the selected repository baseDir when all metadata sources are unset.
14. Persisted context YAML MUST omit `metadata.baseDir` when it equals repository baseDir.
15. Every context MUST define `managedServer.http` with one configured auth mode.
16. Catalog-level `defaultEditor` MAY be omitted and MUST default to `vi` when editor-opening CLI commands resolve no explicit `--editor` override.
17. Catalog edit workflows that replace the full YAML document (for example `config edit`) MUST validate strict YAML and context semantics before persisting any file changes.
18. When `managedServer.http.openapi` is empty and `metadata.bundle` or `metadata.bundleFile` is configured, startup MUST resolve OpenAPI from bundle hints in order: `bundle.yaml declarest.openapi`, then peer `openapi.yaml` at the bundle root.
19. When any proxy block (`managedServer.http.proxy`, `repository.git.remote.proxy`, `secretStore.vault.proxy`, `metadata.proxy`) is configured with values, it MUST define at least one of `httpUrl` or `httpsUrl`; proxy auth (when provided) MUST include both `username` and `password`.
20. Proxy blocks across the managed server, repository, secret store, and metadata share the same default: the first configured concrete proxy becomes the inherited proxy for components that do not define their own, and defining an empty `proxy:` block in a component explicitly disables the inherited proxy for that component.
21. `managedServer.http.openapi` MAY reference either an OpenAPI 3.x (`openapi`) or Swagger 2.0 (`swagger`) document.
22. `managedServer.http.healthCheck` MAY be configured as a relative path or an absolute `http|https` URL, and it MUST NOT include query parameters.
23. `repository.git.remote.autoSync` MAY be omitted; when omitted, repository-mutation commands MUST treat it as enabled and only an explicit `false` disables automatic push behavior.

## Data Contracts
Top-level catalog fields:
1. `contexts`: list of context objects.
2. `currentCtx`: active context name.
3. optional `defaultEditor`: editor command used by CLI editor workflows when `--editor` is not provided.

Per-context fields:
1. `name`.
2. `repository`.
3. required `managedServer`.
4. optional `secretStore`.
5. optional `metadata` (omit when equivalent to default repository baseDir behavior); `metadata.proxy` configures the HTTP proxy used for bundle downloads and participates in the shared proxy semantics.
6. optional `preferences`.

Repository one-of contract:
1. Exactly one of `repository.git` or `repository.filesystem` MUST be set.
2. `repository.git.remote.proxy` MAY be used to configure HTTP/HTTPS proxies for git fetch/push flows; it inherits the shared proxy when unset and an empty block disables the inherited proxy for git operations.
3. `repository.git.remote.autoSync` defaults to `true` when omitted.

Resource server auth one-of contract:
1. Exactly one of `oauth2`, `basicAuth`, or `customHeaders` MUST be set under `managedServer.http.auth`.
2. `managedServer.http.auth.customHeaders` MUST contain at least one entry.
3. Each `managedServer.http.auth.customHeaders[*]` entry MUST define `header` and `value`; it MAY define `prefix`, which is prepended as `<prefix> <value>`.

Resource server proxy contract:
1. `managedServer.http.proxy` MAY define `httpUrl` and/or `httpsUrl`.
2. `managedServer.http.proxy.noProxy` MAY define comma-separated bypass rules.
3. `managedServer.http.proxy.auth` MAY be configured; when set, it MUST define both `username` and `password`.
4. The same `proxy` structure is available for `repository.git.remote.proxy`, `secretStore.vault.proxy`, and `metadata.proxy`, and they inherit the shared proxy unless an empty `proxy:` block explicitly disables it for their component.

Resource server healthCheck contract:
1. `managedServer.http.healthCheck` MAY be omitted; when omitted, probe commands target `managedServer.http.baseUrl` itself and reuse its normalized path.
2. Relative `managedServer.http.healthCheck` values MUST be normalized as managedServer request paths.
3. Absolute `managedServer.http.healthCheck` values MUST use `http` or `https` and MUST share scheme/host with `managedServer.http.baseUrl`.
4. `managedServer.http.healthCheck` MUST NOT include query parameters.

Secret store one-of contracts:
1. Exactly one of `secretStore.file` or `secretStore.vault` MUST be set.
2. For `secretStore.file`, exactly one of `key`, `keyFile`, `passphrase`, `passphraseFile` MUST be set.
3. `secretStore.vault.proxy` MAY configure HTTP/HTTPS proxies for Vault operations and follows the shared proxy inheritance rules; use an empty `proxy:` block to opt out.

Context manager operations:
1. `Create/Update/Delete/Rename/List`.
2. `SetCurrent/GetCurrent`.
3. `ResolveContext`.
4. `Validate`.

Runtime override keys:
1. `repository.git.local.baseDir`.
2. `repository.filesystem.baseDir`.
3. `managedServer.http.baseUrl`.
4. `managedServer.http.healthCheck`.
5. `managedServer.http.proxy.httpUrl`.
6. `managedServer.http.proxy.httpsUrl`.
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
        #     # Choose exactly one auth method: basicAuth, ssh, accessKey.
        #     basicAuth:
        #       username: change-me
        #       password: change-me
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
        #     httpUrl: http://proxy.example.com:3128
        #     httpsUrl: http://proxy.example.com:3128
        #     noProxy: localhost,127.0.0.1
        #     auth:
        #       username: proxy-user
        #       password: proxy-pass
      # filesystem:
      #   baseDir: /path/to/repo

    managedServer:
      http:
        baseUrl: https://example.com/api
        # healthCheck: /health
        # openapi: /path/to/openapi-or-swagger.yaml
        # defaultHeaders:
        #   X-Example: value
        # proxy:
        #   httpUrl: http://proxy.example.com:3128
        #   httpsUrl: http://proxy.example.com:3128
        #   noProxy: localhost,127.0.0.1
        #   auth:
        #     username: proxy-user
        #     password: proxy-pass
        auth:
          # Choose exactly one auth method: oauth2, basicAuth, customHeaders.
          oauth2:
            tokenUrl: https://example.com/oauth/token
            grantType: client_credentials
            clientId: change-me
            clientSecret: change-me
            # username: change-me
            # password: change-me
            # scope: api.read
            # audience: https://example.com/
          # basicAuth:
          #   username: change-me
          #   password: change-me
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
      #     # appRole:
      #     #   roleId: roleId
      #     #   secretId: secretId
      #     #   mount: appRole
      #   tls:
      #     caCertFile: /path/to/ca.pem
      #     clientCertFile: /path/to/client.pem
      #     clientKeyFile: /path/to/client-key.pem
      #     insecureSkipVerify: false
      #   proxy:
      #     httpUrl: http://proxy.example.com:3128
      #     httpsUrl: http://proxy.example.com:3128
      #     noProxy: localhost,127.0.0.1
      #     auth:
      #       username: proxy-user
      #       password: proxy-pass

    metadata:
      # Metadata source defaults to repository baseDir when both are unset.
      # Choose at most one metadata source.
      # baseDir: /path/to/metadata
      # bundle: keycloak-bundle:0.0.1
      # bundleFile: /path/to/keycloak-bundle-0.0.1.tar.gz
      # proxy:
      #   httpUrl: http://proxy.example.com:3128
      #   httpsUrl: http://proxy.example.com:3128
      #   noProxy: localhost,127.0.0.1
      #   auth:
      #     username: proxy-user
      #     password: proxy-pass

  - name: yyy
    repository:
      filesystem:
        baseDir: /other/repo

currentCtx: xxx
# defaultEditor: vi
```

## Failure Modes
1. `currentCtx` missing or not found in `contexts`.
2. Duplicate context names.
3. Unknown YAML key due to strict decode.
4. Repository backend one-of violation.
5. Missing required `managedServer`.
6. Resource server auth one-of violation.
7. Secret store one-of violation.
8. Secret file key source one-of violation.
9. Metadata source one-of violation (multiple metadata sources set, for example `metadata.baseDir` with `metadata.bundle`).
10. Config path resolution failure for home expansion or file access.
11. Runtime override key not in the supported override-key list.
12. Composition root startup (`bootstrap.NewSession`) fails when neither `selection.name` nor `currentCtx` resolves to a valid context.
13. `managedServer.http.proxy` is configured without at least one proxy URL, or with incomplete auth credentials.
14. `managedServer.http.healthCheck` is configured with query parameters or an invalid URL form.

## Edge Cases
1. Empty catalog with no contexts and no current context.
2. Context with optional `secretStore` omitted.
3. Context with required `managedServer` omitted fails validation.
4. Runtime override targets a missing optional block.
5. Catalog file absent on first run; list returns empty and current/resolve report `current context not set`.
6. `metadata.baseDir` omitted in YAML; resolve still returns repository baseDir as effective metadata baseDir.
7. `metadata.bundle` configured; resolve keeps `metadata.baseDir` empty and startup resolves metadata from the bundle cache.
8. `metadata.bundle` provides `declarest.openapi` or peer `openapi.yaml`; startup wires that OpenAPI source only when context `managedServer.http.openapi` is unset.
9. `metadata.bundleFile` configured; resolve keeps `metadata.baseDir` empty and startup resolves metadata from the local bundle archive.
10. `defaultEditor` omitted in YAML; editor-opening CLI commands still resolve `vi` by default.
11. `managedServer.http.proxy.noProxy` can be set without proxy auth and still remains valid.
12. `managedServer.http.healthCheck` can be absolute and still resolves to a managedServer-relative probe when scheme/host match `managedServer.http.baseUrl`.

## Examples
1. `ResolveContext({Name: "", Overrides: nil})` loads the context named by `currentCtx`.
2. `SetCurrent("yyy")` updates `currentCtx` and preserves context list order.
3. `Validate` rejects a config that defines both `repository.git` and `repository.filesystem`.
4. Corner case: `ResolveContext({Name: "dev", Overrides: {"unknown.key":"x"}})` fails with a validation error for unknown override keys.
5. Corner case: `ResolveContext({Name: "dev", Overrides: {"metadata.bundle":"keycloak-bundle:0.0.1"}})` resolves bundle metadata source and clears `metadata.baseDir`.
6. Corner case: `ResolveContext({Name: "dev", Overrides: {"metadata.bundleFile":"/tmp/keycloak-bundle-0.0.1.tar.gz"}})` resolves local bundle metadata source and clears both `metadata.baseDir` and `metadata.bundle`.
7. `List()` on a missing catalog file returns `[]`; `GetCurrent()` returns `NotFoundError` with `current context not set`.
8. `bootstrap.NewSession(..., ContextSelection{})` returns `NotFoundError` when `currentCtx` is not set.
9. `config edit prod` loads only context `prod` into a temporary document, validates the edited YAML, and replaces only that context in the persisted catalog when validation succeeds.
10. Corner case: `managedServer.http.auth.customHeaders` with one entry that defines `header` + `value` and no `prefix` remains valid and sends the raw `value` in the configured header.
11. Corner case: `ResolveContext({Name: "dev", Overrides: nil})` with empty `managedServer.http.openapi` and bundle metadata source (`metadata.bundle` or `metadata.bundleFile`) that includes `openapi.yaml` keeps context config unchanged while startup wiring resolves OpenAPI from the extracted bundle.
12. Corner case: `ResolveContext({Name: "dev", Overrides: {"managedServer.http.proxy.httpUrl":"http://proxy.example.com:3128"}})` applies proxy override and keeps other proxy fields untouched when unset.
