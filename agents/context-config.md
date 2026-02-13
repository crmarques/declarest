# Contexts and Configuration

## Purpose
Define the canonical context catalog schema, file location, validation rules, and runtime loading behavior.

## In Scope
1. YAML context catalog structure.
2. Context lifecycle operations and current context selection.
3. Validation and one-of constraints.
4. Config file path resolution and overrides.

## Out of Scope
1. Secret payload values inside resource files.
2. HTTP transport adapter implementation details.
3. CLI completion internals.

## Normative Rules
1. Context catalogs MUST be stored at `~/declarest/config/contexts.yaml` by default.
2. Environment override `DECLAREST_CONTEXT_FILE` MAY replace the default path.
3. YAML keys MUST use kebab-case.
4. Unknown YAML keys MUST fail parsing.
5. `contexts` MUST be a list of full context objects.
6. `current-ctx` MUST reference an existing context.
7. Context names MUST be unique and non-empty.
8. Validation MUST fail fast when one-of blocks are missing or ambiguous.
9. Config precedence MUST be: runtime flags, environment overrides, persisted context values, engine defaults.
10. Unknown override keys MUST fail validation.

## Data Contracts
Top-level catalog fields:
1. `contexts`: list of context objects.
2. `current-ctx`: active context name.

Per-context fields:
1. `name`.
2. `repository`.
3. optional `managed-server`.
4. optional `secret-store`.
5. optional `metadata`.
6. optional `preferences`.

Repository one-of contract:
1. Exactly one of `repository.git` or `repository.filesystem` MUST be set.
2. `repository.resource-format` allowed values: `json` or `yaml`.

Managed server auth one-of contract:
1. Exactly one of `oauth2`, `basic-auth`, `bearer-token`, `custom-header` MUST be set under `managed-server.http.auth`.

Secret store one-of contracts:
1. Exactly one of `secret-store.file` or `secret-store.vault` MUST be set.
2. For `secret-store.file`, exactly one of `key`, `key-file`, `passphrase`, `passphrase-file` MUST be set.

Context manager operations:
1. `Create/Update/Delete/Rename/List`.
2. `SetCurrent/GetCurrent`.
3. `LoadResolvedConfig`.
4. `Validate`.

## Canonical YAML Template
```yaml
contexts:
  - name: xxx
    repository:
      # Resource file format: json (default) or yaml.
      resource-format: json
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

    managed-server:
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
          #   header: X-Example-Token
          #   token: change-me
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
```

## Failure Modes
1. `current-ctx` missing or not found in `contexts`.
2. Duplicate context names.
3. Unknown YAML key due to strict decode.
4. Repository backend one-of violation.
5. Managed server auth one-of violation.
6. Secret store one-of violation.
7. Secret file key source one-of violation.
8. Config path resolution failure for home expansion or file access.

## Edge Cases
1. Empty catalog with no contexts and no current context.
2. Context with optional `managed-server` omitted.
3. Context with optional `secret-store` omitted.
4. Runtime override targets a missing optional block.

## Examples
1. `LoadResolvedConfig("", nil)` loads the context named by `current-ctx`.
2. `SetCurrent("yyy")` updates `current-ctx` and preserves context list order.
3. `Validate` rejects a config that defines both `repository.git` and `repository.filesystem`.
