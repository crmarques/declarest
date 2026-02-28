# Configuration Reference

This page documents the user-facing context catalog schema used by DeclaREST.

Use `declarest config print-template` to generate the full commented template, and use `declarest config validate` before importing changes.

## Quick facts

- Default catalog path: `~/.declarest/configs/contexts.yaml`
- Override catalog path with `DECLAREST_CONTEXTS_FILE`
- YAML keys use `kebab-case`
- Unknown keys fail validation (strict decode)
- `resource-server` is required in every context

## Top-level catalog shape

```yaml
contexts:
  - name: prod
    repository:
      filesystem:
        base-dir: /srv/declarest/prod
    resource-server:
      http:
        base-url: https://api.example.com
        auth:
          bearer-token:
            token: ${API_TOKEN}
current-ctx: prod
```

Top-level fields:

- `contexts`: list of context objects
- `current-ctx`: active context name

## Context object (high level)

Each context may include:

- `name` (required)
- `repository` (required)
- `resource-server` (required)
- `secret-store` (optional)
- `metadata` (optional)
- `preferences` (optional free-form map)

## Repository configuration

### `repository.resource-format`

Optional. Controls local payload file format:

- `json`
- `yaml`

When omitted, DeclaREST uses the remote/default format behavior.

### Repository backend (one-of)

Exactly one of these must be present:

- `repository.filesystem`
- `repository.git`

### Filesystem repository

```yaml
repository:
  resource-format: yaml
  filesystem:
    base-dir: /work/declarest/repo
```

### Git repository (local only)

```yaml
repository:
  git:
    local:
      base-dir: /work/declarest/repo
```

### Git repository with remote

```yaml
repository:
  resource-format: json
  git:
    local:
      base-dir: /work/declarest/repo
    remote:
      url: https://github.com/example/config-repo.git
      branch: main
      provider: github
      auto-sync: false
      auth:
        access-key:
          token: ${GIT_TOKEN}
      tls:
        insecure-skip-verify: false
```

#### `repository.git.remote.auth` (one-of)

Exactly one auth method when `auth` is present:

- `basic-auth`
- `ssh`
- `access-key`

Examples:

```yaml
# basic-auth
auth:
  basic-auth:
    username: git-user
    password: ${GIT_PASSWORD}
```

```yaml
# ssh
auth:
  ssh:
    user: git
    private-key-file: /home/me/.ssh/id_rsa
    known-hosts-file: /home/me/.ssh/known_hosts
```

```yaml
# access-key
auth:
  access-key:
    token: ${GIT_TOKEN}
```

#### Git remote TLS (optional)

```yaml
tls:
  ca-cert-file: /path/to/ca.pem
  client-cert-file: /path/to/client.pem
  client-key-file: /path/to/client-key.pem
  insecure-skip-verify: false
```

## Resource-server configuration (`resource-server`)

`resource-server.http` is required.

```yaml
resource-server:
  http:
    base-url: https://api.example.com
    openapi: /path/to/openapi.yaml
    default-headers:
      X-Env: prod
    auth:
      oauth2:
        token-url: https://auth.example.com/oauth/token
        grant-type: client_credentials
        client-id: declarest
        client-secret: ${OAUTH_CLIENT_SECRET}
    tls:
      ca-cert-file: /path/to/ca.pem
      client-cert-file: /path/to/client.pem
      client-key-file: /path/to/client-key.pem
      insecure-skip-verify: false
```

### `resource-server.http.auth` (required one-of)

Exactly one auth method must be configured:

- `oauth2`
- `basic-auth`
- `bearer-token`
- `custom-header`

#### OAuth2 example

```yaml
auth:
  oauth2:
    token-url: https://auth.example.com/oauth/token
    grant-type: client_credentials
    client-id: declarest
    client-secret: ${OAUTH_CLIENT_SECRET}
    # optional depending on flow/provider:
    # username: alice
    # password: secret
    # scope: api.read api.write
    # audience: https://api.example.com
```

#### Basic auth example

```yaml
auth:
  basic-auth:
    username: alice
    password: ${API_PASSWORD}
```

#### Bearer token example

```yaml
auth:
  bearer-token:
    token: ${API_TOKEN}
```

#### Custom header auth example

```yaml
auth:
  custom-header:
    header: Authorization
    prefix: Bearer
    value: ${API_TOKEN}
```

`prefix` is optional. When set, DeclaREST sends `<prefix> <value>` in the configured header.

### OpenAPI (`resource-server.http.openapi`)

Optional but recommended.

When configured, DeclaREST can:

- improve default metadata inference (`metadata infer`)
- improve operation defaults for path/method/media behavior
- help path completion with API path templates

Explicit metadata overrides still win.

When omitted, and `metadata.bundle` is configured, DeclaREST attempts OpenAPI fallback from the bundle:

- `bundle.yaml` `declarest.openapi` (URL or relative path inside the bundle)
- bundled `openapi.yaml` at bundle root (peer of `bundle.yaml`)

Precedence is deterministic: context `resource-server.http.openapi` overrides bundle-provided OpenAPI sources.

## Secret store configuration (`secret-store`, optional)

Exactly one provider when `secret-store` is configured:

- `file`
- `vault`

### File secret store

```yaml
secret-store:
  file:
    path: /work/declarest/secrets.json
    passphrase: ${DECLAREST_SECRET_PASSPHRASE}
    # or exactly one of the following key sources instead of passphrase:
    # key: <base64-encoded-32-byte-key>
    # key-file: /path/to/key.txt
    # passphrase-file: /path/to/passphrase.txt
    # kdf:
    #   time: 1
    #   memory: 65536
    #   threads: 4
```

`secret-store.file` key source is also a one-of:

- `key`
- `key-file`
- `passphrase`
- `passphrase-file`

### Vault secret store (KV)

```yaml
secret-store:
  vault:
    address: https://vault.example.com
    mount: secret
    path-prefix: declarest
    kv-version: 2
    auth:
      token: ${VAULT_TOKEN}
      # or password / approle blocks
    tls:
      ca-cert-file: /path/to/ca.pem
      client-cert-file: /path/to/client.pem
      client-key-file: /path/to/client-key.pem
      insecure-skip-verify: false
```

Vault auth under `secret-store.vault.auth` is also one-of:

- token (`token`)
- password (`password` block)
- approle (`approle` block)

## Metadata configuration (`metadata`, optional)

Choose at most one metadata source:

```yaml
metadata:
  # local metadata tree
  # base-dir: /path/to/metadata
  #
  # or bundle reference (shorthand, URL, or local tar.gz path)
  # bundle: keycloak-bundle:0.0.1
  # bundle: https://github.com/crmarques/declarest-bundle-keycloak/releases/download/v0.0.1/keycloak-bundle-0.0.1.tar.gz
  # bundle: /path/to/keycloak-bundle-0.0.1.tar.gz
```

When both metadata sources are omitted, `metadata.base-dir` defaults to the selected repository base dir.

Use `metadata.base-dir` when you want metadata files stored separately from resource payload files.
Use `metadata.bundle` when you want metadata definitions to be consumed from a bundle archive and cached under `~/.declarest/metadata-bundles/`.

Bundle OpenAPI hints (when present) can also supply `resource-server.http.openapi` automatically if context OpenAPI is not set.

## Preferences (optional)

`preferences` is an arbitrary key/value map for user or workflow hints.
DeclaREST may ignore unknown preference keys, but your team automation can still use them.

Example:

```yaml
preferences:
  env: prod
  owner: platform
```

## Complete example (advanced)

```yaml
contexts:
  - name: keycloak-prod
    repository:
      resource-format: yaml
      git:
        local:
          base-dir: /srv/declarest/keycloak-prod
        remote:
          url: git@github.com:acme/keycloak-config.git
          branch: main
          provider: github
          auto-sync: false
          auth:
            ssh:
              user: git
              private-key-file: /home/declarest/.ssh/id_ed25519
              known-hosts-file: /home/declarest/.ssh/known_hosts
    resource-server:
      http:
        base-url: https://sso.example.com/admin
        openapi: /srv/declarest/openapi/keycloak-admin.json
        auth:
          oauth2:
            token-url: https://sso.example.com/realms/master/protocol/openid-connect/token
            grant-type: client_credentials
            client-id: declarest
            client-secret: ${KEYCLOAK_CLIENT_SECRET}
        tls:
          ca-cert-file: /etc/ssl/certs/internal-ca.pem
    secret-store:
      vault:
        address: https://vault.example.com
        mount: secret
        path-prefix: declarest/keycloak-prod
        kv-version: 2
        auth:
          approle:
            role-id: ${VAULT_ROLE_ID}
            secret-id: ${VAULT_SECRET_ID}
            mount: approle
    metadata:
      base-dir: /srv/declarest/keycloak-metadata
    preferences:
      env: prod
      service: keycloak

current-ctx: keycloak-prod
```

## Validation and editing workflow

```bash
# generate a commented template
declarest config print-template > /tmp/contexts.yaml

# validate before import
declarest config validate --payload /tmp/contexts.yaml

# import and set current
declarest config add --file /tmp/contexts.yaml --set-current

# inspect resolved context
declarest config resolve
```

## Runtime override precedence

Effective config precedence is:

1. runtime flags/override inputs
2. environment overrides
3. persisted context values
4. engine defaults

### `config resolve --set` overrides (supported keys)

The canonical override keys are:

- `repository.resource-format`
- `repository.git.local.base-dir`
- `repository.filesystem.base-dir`
- `resource-server.http.base-url`
- `metadata.base-dir`
- `metadata.bundle`

Example:

```bash
declarest config resolve \
  --set resource-server.http.base-url=https://staging-api.example.com \
  --set metadata.bundle=keycloak-bundle:0.0.1
```

## Environment variables

### Context catalog location

- `DECLAREST_CONTEXTS_FILE`: overrides the path to the context catalog file

Example:

```bash
export DECLAREST_CONTEXTS_FILE=/work/declarest/configs/contexts.yaml
declarest config list
```

## Failure modes to expect (and validate early)

- both `repository.git` and `repository.filesystem` set
- missing `resource-server`
- missing/ambiguous `resource-server.http.auth` method
- unknown YAML key (strict decoding)
- `current-ctx` points to a non-existent context name
- invalid secret-store one-of configuration
- both `metadata.base-dir` and `metadata.bundle` set in the same context
