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
12. `metadata.base-dir` MUST default to the selected repository base-dir when unset.
13. Persisted context YAML MUST omit `metadata.base-dir` when it equals repository base-dir.
14. Every context MUST define `resource-server.http` with one configured auth mode.
15. Catalog-level `default-editor` MAY be omitted and MUST default to `vi` when editor-opening CLI commands resolve no explicit `--editor` override.
16. Catalog edit workflows that replace the full YAML document (for example `config edit`) MUST validate strict YAML and context semantics before persisting any file changes.

## Data Contracts
Top-level catalog fields:
1. `contexts`: list of context objects.
2. `current-ctx`: active context name.
3. optional `default-editor`: editor command used by CLI editor workflows when `--editor` is not provided.

Per-context fields:
1. `name`.
2. `repository`.
3. required `resource-server`.
4. optional `secret-store`.
5. optional `metadata` (omit when equivalent to default repository base-dir behavior).
6. optional `preferences`.

Repository one-of contract:
1. Exactly one of `repository.git` or `repository.filesystem` MUST be set.
2. `repository.resource-format` allowed values: `json` or `yaml`.

Resource server auth one-of contract:
1. Exactly one of `oauth2`, `basic-auth`, `bearer-token`, `custom-header` MUST be set under `resource-server.http.auth`.
2. `resource-server.http.auth.custom-header` MUST define `header` and `value`; it MAY define `prefix`, which is prepended as `<prefix> <value>`.

Secret store one-of contracts:
1. Exactly one of `secret-store.file` or `secret-store.vault` MUST be set.
2. For `secret-store.file`, exactly one of `key`, `key-file`, `passphrase`, `passphrase-file` MUST be set.

Context manager operations:
1. `Create/Update/Delete/Rename/List`.
2. `SetCurrent/GetCurrent`.
3. `ResolveContext`.
4. `Validate`.

Runtime override keys:
1. `repository.resource-format`.
2. `repository.git.local.base-dir`.
3. `repository.filesystem.base-dir`.
4. `resource-server.http.base-url`.
5. `metadata.base-dir`.

## Canonical YAML Template
```yaml
contexts:
  - name: xxx
    repository:
      # Optional resource file format: json or yaml.
      # When omitted, declarest uses the remote resource format default.
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
      # filesystem:
      #   base-dir: /path/to/repo

    resource-server:
      http:
        base-url: https://example.com/api
        # openapi: /path/to/openapi.yaml
        # default-headers:
        #   X-Example: value
        auth:
          # Choose exactly one auth method: oauth2, basic-auth, bearer-token, custom-header.
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
          # bearer-token:
          #   token: change-me
          # custom-header:
          #   header: Authorization
          #   prefix: Bearer
          #   value: change-me
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

    metadata:
      # Metadata files default to repository base dir when unset.
      base-dir: /path/to/metadata

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
5. Missing required `resource-server`.
6. Resource server auth one-of violation.
7. Secret store one-of violation.
8. Secret file key source one-of violation.
9. Config path resolution failure for home expansion or file access.
10. Runtime override key not in the supported override-key list.
11. Composition root startup (`core.NewDeclarestContext`) fails when neither `selection.name` nor `current-ctx` resolves to a valid context.

## Edge Cases
1. Empty catalog with no contexts and no current context.
2. Context with optional `secret-store` omitted.
3. Context with required `resource-server` omitted fails validation.
4. Runtime override targets a missing optional block.
5. Catalog file absent on first run; list returns empty and current/resolve report `current context not set`.
6. `metadata.base-dir` omitted in YAML; resolve still returns repository base-dir as effective metadata base-dir.
7. `default-editor` omitted in YAML; editor-opening CLI commands still resolve `vi` by default.

## Examples
1. `ResolveContext({Name: "", Overrides: nil})` loads the context named by `current-ctx`.
2. `SetCurrent("yyy")` updates `current-ctx` and preserves context list order.
3. `Validate` rejects a config that defines both `repository.git` and `repository.filesystem`.
4. Corner case: `ResolveContext({Name: "dev", Overrides: {"unknown.key":"x"}})` fails with a validation error for unknown override keys.
5. `List()` on a missing catalog file returns `[]`; `GetCurrent()` returns `NotFoundError` with `current context not set`.
6. `core.NewDeclarestContext(..., ContextSelection{})` returns `NotFoundError` when `current-ctx` is not set.
7. `config edit prod` loads only context `prod` into a temporary document, validates the edited YAML, and replaces only that context in the persisted catalog when validation succeeds.
8. Corner case: `resource-server.http.auth.custom-header` with `header` + `value` and no `prefix` remains valid and sends the raw `value` in the configured header.
