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
3. YAML keys MUST use kebab-case.
4. Unknown YAML keys MUST fail parsing.
5. `contexts` MUST be a list of full context objects.
6. `current-ctx` MUST reference an existing context.
7. Context names MUST be unique and non-empty.
8. Validation MUST fail fast when one-of blocks are missing or ambiguous.
9. Config precedence MUST be: runtime flags, environment overrides, persisted context values, engine defaults.
10. Unknown override keys MUST fail validation.
11. Missing context catalog files MUST be treated as an empty catalog state.
12. `metadata` MUST define at most one source: `base-dir`, `bundle`, or `bundle-file`.
13. `metadata.base-dir` MUST default to the selected repository base-dir when all metadata sources are unset.
14. Persisted context YAML MUST omit `metadata.base-dir` when it equals repository base-dir.
15. Every context MUST define `managed-server.http` with one configured auth mode.
16. Catalog-level `default-editor` MAY be omitted and MUST default to `vi` when editor-opening CLI commands resolve no explicit `--editor` override.
17. Catalog edit workflows that replace the full YAML document (for example `config edit`) MUST validate strict YAML and context semantics before persisting any file changes.
18. When `managed-server.http.openapi` is empty and `metadata.bundle` or `metadata.bundle-file` is configured, startup MUST resolve OpenAPI from bundle hints in order: `bundle.yaml declarest.openapi`, then peer `openapi.yaml` at the bundle root.
19. When any proxy block (`managed-server.http.proxy`, `repository.git.remote.proxy`, `secret-store.vault.proxy`, `metadata.proxy`) is configured with values, it MUST define at least one of `http-url` or `https-url`; proxy auth (when provided) MUST include both `username` and `password`.
20. Proxy blocks across the managed server, repository, secret store, and metadata share the same default: the first configured concrete proxy becomes the inherited proxy for components that do not define their own, and defining an empty `proxy:` block in a component explicitly disables the inherited proxy for that component.
21. `managed-server.http.openapi` MAY reference either an OpenAPI 3.x (`openapi`) or Swagger 2.0 (`swagger`) document.
22. `managed-server.http.health-check` MAY be configured as a relative path or an absolute `http|https` URL, and it MUST NOT include query parameters.

## Data Contracts
Top-level catalog fields:
1. `contexts`: list of context objects.
2. `current-ctx`: active context name.
3. optional `default-editor`: editor command used by CLI editor workflows when `--editor` is not provided.

Per-context fields:
1. `name`.
2. `repository`.
3. required `managed-server`.
4. optional `secret-store`.
5. optional `metadata` (omit when equivalent to default repository base-dir behavior); `metadata.proxy` configures the HTTP proxy used for bundle downloads and participates in the shared proxy semantics.
6. optional `preferences`.

Repository one-of contract:
1. Exactly one of `repository.git` or `repository.filesystem` MUST be set.
2. `repository.resource-format` allowed values: `json`, `yaml`, `xml`, `hcl`, `ini`, `properties`, `text`, or `octet-stream`.
3. `repository.git.remote.proxy` MAY be used to configure HTTP/HTTPS proxies for git fetch/push flows; it inherits the shared proxy when unset and an empty block disables the inherited proxy for git operations.

Resource server auth one-of contract:
1. Exactly one of `oauth2`, `basic-auth`, or `custom-headers` MUST be set under `managed-server.http.auth`.
2. `managed-server.http.auth.custom-headers` MUST contain at least one entry.
3. Each `managed-server.http.auth.custom-headers[*]` entry MUST define `header` and `value`; it MAY define `prefix`, which is prepended as `<prefix> <value>`.

Resource server proxy contract:
1. `managed-server.http.proxy` MAY define `http-url` and/or `https-url`.
2. `managed-server.http.proxy.no-proxy` MAY define comma-separated bypass rules.
3. `managed-server.http.proxy.auth` MAY be configured; when set, it MUST define both `username` and `password`.
4. The same `proxy` structure is available for `repository.git.remote.proxy`, `secret-store.vault.proxy`, and `metadata.proxy`, and they inherit the shared proxy unless an empty `proxy:` block explicitly disables it for their component.

Resource server health-check contract:
1. `managed-server.http.health-check` MAY be omitted; when omitted, probe commands target the managed-server base path (`/` relative to `managed-server.http.base-url`).
2. Relative `managed-server.http.health-check` values MUST be normalized as managed-server request paths.
3. Absolute `managed-server.http.health-check` values MUST use `http` or `https` and MUST share scheme/host with `managed-server.http.base-url`.
4. `managed-server.http.health-check` MUST NOT include query parameters.

Secret store one-of contracts:
1. Exactly one of `secret-store.file` or `secret-store.vault` MUST be set.
2. For `secret-store.file`, exactly one of `key`, `key-file`, `passphrase`, `passphrase-file` MUST be set.
3. `secret-store.vault.proxy` MAY configure HTTP/HTTPS proxies for Vault operations and follows the shared proxy inheritance rules; use an empty `proxy:` block to opt out.

Context manager operations:
1. `Create/Update/Delete/Rename/List`.
2. `SetCurrent/GetCurrent`.
3. `ResolveContext`.
4. `Validate`.

Runtime override keys:
1. `repository.resource-format`.
2. `repository.git.local.base-dir`.
3. `repository.filesystem.base-dir`.
4. `managed-server.http.base-url`.
5. `managed-server.http.health-check`.
6. `managed-server.http.proxy.http-url`.
7. `managed-server.http.proxy.https-url`.
8. `managed-server.http.proxy.no-proxy`.
9. `managed-server.http.proxy.auth.username`.
10. `managed-server.http.proxy.auth.password`.
11. `metadata.base-dir`.
12. `metadata.bundle`.
13. `metadata.bundle-file`.

## Canonical YAML Template
```yaml
contexts:
  - name: xxx
    repository:
      # Optional default payload type: json, yaml, xml, hcl, ini, properties, text, or octet-stream.
      # When omitted, declarest uses runtime inference before engine defaults.
      # resource-format: json
      # Choose exactly one repository type: filesystem or git.
      git:
        local:
          base-dir: /path/to/repo
        # remote:
        #   url: https://example.com/org/repo.git
        #   branch: main
        #   provider: github
        #   auto-sync: true
        #   auth:
        #     # Choose exactly one auth method: basic-auth, ssh, access-key.
        #     basic-auth:
        #       username: change-me
        #       password: change-me
        #     ssh:
        #       user: git
        #       private-key-file: /path/to/id_rsa
        #       passphrase: change-me
        #       known-hosts-file: /path/to/known_hosts
        #       insecure-ignore-host-key: false
        #     access-key:
        #       token: change-me
        #   tls:
        #     insecure-skip-verify: false
        #   proxy:
        #     http-url: http://proxy.example.com:3128
        #     https-url: http://proxy.example.com:3128
        #     no-proxy: localhost,127.0.0.1
        #     auth:
        #       username: proxy-user
        #       password: proxy-pass
      # filesystem:
      #   base-dir: /path/to/repo

    managed-server:
      http:
        base-url: https://example.com/api
        # health-check: /health
        # openapi: /path/to/openapi-or-swagger.yaml
        # default-headers:
        #   X-Example: value
        # proxy:
        #   http-url: http://proxy.example.com:3128
        #   https-url: http://proxy.example.com:3128
        #   no-proxy: localhost,127.0.0.1
        #   auth:
        #     username: proxy-user
        #     password: proxy-pass
        auth:
          # Choose exactly one auth method: oauth2, basic-auth, custom-headers.
          oauth2:
            token-url: https://example.com/oauth/token
            grant-type: client_credentials
            client-id: change-me
            client-secret: change-me
            # username: change-me
            # password: change-me
            # scope: api.read
            # audience: https://example.com/
          # basic-auth:
          #   username: change-me
          #   password: change-me
          # custom-headers:
          #   - header: Authorization
          #     prefix: Bearer
          #     value: change-me
        # tls:
        #   insecure-skip-verify: false

    secret-store:
      # Choose exactly one: file or vault.
      file:
        path: /path/to/secrets.json
        # Choose exactly one: key, key-file, passphrase, passphrase-file.
        passphrase: change-me
        # key: base64-encoded-key
        # key-file: /path/to/key.txt
        # passphrase-file: /path/to/passphrase.txt
        # kdf:
        #   time: 1
        #   memory: 65536
        #   threads: 4
      # vault:
      #   address: https://vault.example.com
      #   mount: secret
      #   path-prefix: declarest
      #   kv-version: 2
      #   auth:
      #     token: s.xxxx
      #     # password:
      #     #   username: vault-user
      #     #   password: vault-pass
      #     #   mount: userpass
      #     # approle:
      #     #   role-id: role-id
      #     #   secret-id: secret-id
      #     #   mount: approle
      #   tls:
      #     ca-cert-file: /path/to/ca.pem
      #     client-cert-file: /path/to/client.pem
      #     client-key-file: /path/to/client-key.pem
      #     insecure-skip-verify: false
      #   proxy:
      #     http-url: http://proxy.example.com:3128
      #     https-url: http://proxy.example.com:3128
      #     no-proxy: localhost,127.0.0.1
      #     auth:
      #       username: proxy-user
      #       password: proxy-pass

    metadata:
      # Metadata source defaults to repository base-dir when both are unset.
      # Choose at most one metadata source.
      # base-dir: /path/to/metadata
      # bundle: keycloak-bundle:0.0.1
      # bundle-file: /path/to/keycloak-bundle-0.0.1.tar.gz
      # proxy:
      #   http-url: http://proxy.example.com:3128
      #   https-url: http://proxy.example.com:3128
      #   no-proxy: localhost,127.0.0.1
      #   auth:
      #     username: proxy-user
      #     password: proxy-pass

  - name: yyy
    repository:
      resource-format: yaml
      filesystem:
        base-dir: /other/repo

current-ctx: xxx
# default-editor: vi
```

## Failure Modes
1. `current-ctx` missing or not found in `contexts`.
2. Duplicate context names.
3. Unknown YAML key due to strict decode.
4. Repository backend one-of violation.
5. Missing required `managed-server`.
6. Resource server auth one-of violation.
7. Secret store one-of violation.
8. Secret file key source one-of violation.
9. Metadata source one-of violation (multiple metadata sources set, for example `metadata.base-dir` with `metadata.bundle`).
10. Config path resolution failure for home expansion or file access.
11. Runtime override key not in the supported override-key list.
12. Composition root startup (`bootstrap.NewSession`) fails when neither `selection.name` nor `current-ctx` resolves to a valid context.
13. `managed-server.http.proxy` is configured without at least one proxy URL, or with incomplete auth credentials.
14. `managed-server.http.health-check` is configured with query parameters or an invalid URL form.

## Edge Cases
1. Empty catalog with no contexts and no current context.
2. Context with optional `secret-store` omitted.
3. Context with required `managed-server` omitted fails validation.
4. Runtime override targets a missing optional block.
5. Catalog file absent on first run; list returns empty and current/resolve report `current context not set`.
6. `metadata.base-dir` omitted in YAML; resolve still returns repository base-dir as effective metadata base-dir.
7. `metadata.bundle` configured; resolve keeps `metadata.base-dir` empty and startup resolves metadata from the bundle cache.
8. `metadata.bundle` provides `declarest.openapi` or peer `openapi.yaml`; startup wires that OpenAPI source only when context `managed-server.http.openapi` is unset.
9. `metadata.bundle-file` configured; resolve keeps `metadata.base-dir` empty and startup resolves metadata from the local bundle archive.
10. `default-editor` omitted in YAML; editor-opening CLI commands still resolve `vi` by default.
11. `managed-server.http.proxy.no-proxy` can be set without proxy auth and still remains valid.
12. `managed-server.http.health-check` can be absolute and still resolves to a managed-server-relative probe when scheme/host match `managed-server.http.base-url`.

## Examples
1. `ResolveContext({Name: "", Overrides: nil})` loads the context named by `current-ctx`.
2. `SetCurrent("yyy")` updates `current-ctx` and preserves context list order.
3. `Validate` rejects a config that defines both `repository.git` and `repository.filesystem`.
4. Corner case: `ResolveContext({Name: "dev", Overrides: {"unknown.key":"x"}})` fails with a validation error for unknown override keys.
5. Corner case: `ResolveContext({Name: "dev", Overrides: {"metadata.bundle":"keycloak-bundle:0.0.1"}})` resolves bundle metadata source and clears `metadata.base-dir`.
6. Corner case: `ResolveContext({Name: "dev", Overrides: {"metadata.bundle-file":"/tmp/keycloak-bundle-0.0.1.tar.gz"}})` resolves local bundle metadata source and clears both `metadata.base-dir` and `metadata.bundle`.
7. `List()` on a missing catalog file returns `[]`; `GetCurrent()` returns `NotFoundError` with `current context not set`.
8. `bootstrap.NewSession(..., ContextSelection{})` returns `NotFoundError` when `current-ctx` is not set.
9. `config edit prod` loads only context `prod` into a temporary document, validates the edited YAML, and replaces only that context in the persisted catalog when validation succeeds.
10. Corner case: `managed-server.http.auth.custom-headers` with one entry that defines `header` + `value` and no `prefix` remains valid and sends the raw `value` in the configured header.
11. Corner case: `ResolveContext({Name: "dev", Overrides: nil})` with empty `managed-server.http.openapi` and bundle metadata source (`metadata.bundle` or `metadata.bundle-file`) that includes `openapi.yaml` keeps context config unchanged while startup wiring resolves OpenAPI from the extracted bundle.
12. Corner case: `ResolveContext({Name: "dev", Overrides: {"managed-server.http.proxy.http-url":"http://proxy.example.com:3128"}})` applies proxy override and keeps other proxy fields untouched when unset.
