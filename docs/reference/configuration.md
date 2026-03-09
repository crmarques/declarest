# Configuration Reference

This page documents the user-facing context catalog schema used by DeclaREST.

Use `declarest context print-template` to generate the full commented template, and use `declarest context validate` before importing changes.

## Quick facts

- Default catalog path: `~/.declarest/configs/contexts.yaml`
- Override catalog path with `DECLAREST_CONTEXTS_FILE`
- YAML keys use `kebab-case`
- Unknown keys fail validation (strict decode)
- `managedServer` (managed server settings) is required in every context

## Top-level catalog shape

```yaml
contexts:
  - name: prod
    repository:
      filesystem:
        baseDir: /srv/declarest/prod
    managed-server:
      http:
        baseURL: https://api.example.com
        auth:
          customHeaders:
            - header: Authorization
              prefix: Bearer
              value: ${API_TOKEN}
currentContext: prod
```

Top-level fields:

- `contexts`: list of context objects
- `currentContext`: active context name

## Context object (high level)

Each context may include:

- `name` (required)
- `repository` (required)
- `managedServer` (required managed server settings)
- `secretStore` (optional)
- `metadata` (optional)
- `preferences` (optional free-form map)

## Repository configuration

### Repository backend (one-of)

Exactly one of these must be present:

- `repository.filesystem`
- `repository.git`

### Filesystem repository

```yaml
repository:
  filesystem:
    baseDir: /work/declarest/repo
```

### Git repository (local only)

```yaml
repository:
  git:
    local:
      baseDir: /work/declarest/repo
```

### Git repository with remote

```yaml
repository:
  git:
    local:
      baseDir: /work/declarest/repo
    remote:
      url: https://github.com/example/config-repo.git
      branch: main
      provider: github
      autoSync: true
      auth:
        access-key:
          token: ${GIT_TOKEN}
      tls:
        insecureSkipVerify: false
```

#### `repository.git.remote.auth` (one-of)

Exactly one auth method when `auth` is present:

- `basicAuth`
- `ssh`
- `access-key`

Examples:

```yaml
# basicAuth
auth:
  basicAuth:
    username: git-user
    password: ${GIT_PASSWORD}
```

```yaml
# ssh
auth:
  ssh:
    user: git
    privateKeyFile: /home/me/.ssh/id_rsa
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
  caCertFile: /path/to/ca.pem
  client-cert-file: /path/to/client.pem
  clientKeyFile: /path/to/client-key.pem
  insecureSkipVerify: false
```

#### Git remote proxy (optional)

```yaml
repository:
  git:
    remote:
      proxy:
        httpURL: http://proxy.example.com:3128
        httpsURL: http://proxy.example.com:3128
        no-proxy: localhost,127.0.0.1
        auth:
          username: proxy-user
          password: proxy-pass
```

`proxy` fields configure the HTTP/HTTPS proxy used for fetch/push operations. When unset, the component inherits the first configured proxy from `managedServer.http.proxy`, `secretStore.vault.proxy`, or `metadata.proxy`. Set `proxy:` with no values to explicitly skip the inherited proxy for this component.

Repository payload files are not configured in the context. DeclaREST persists `resource.<ext>` using the managed-server response media type or the explicit payload input media type.

## Managed server configuration (`managedServer`)

`managed-server.http` is required.

```yaml
managed-server:
  http:
    baseURL: https://api.example.com
    healthCheck: /health
    openapi: /path/to/openapi.yaml
    default-headers:
      X-Env: prod
    auth:
      oauth2:
        tokenURL: https://auth.example.com/oauth/token
        grantType: client_credentials
        clientID: declarest
        clientSecret: ${OAUTH_CLIENT_SECRET}
    tls:
      caCertFile: /path/to/ca.pem
      client-cert-file: /path/to/client.pem
      clientKeyFile: /path/to/client-key.pem
      insecureSkipVerify: false
```

Define any of the `managedServer.http.proxy`, `repository.git.remote.proxy`, `secretStore.vault.proxy`, or `metadata.proxy` blocks to set the default proxy shared across the other components. Add an empty `proxy:` block for a component if you want to explicitly disable the inherited proxy for that component.

### Health check (`managedServer.http.healthCheck`)

Optional probe target used by `declarest server check`.

- when omitted, `server check` probes `/` relative to `managedServer.http.baseURL`
- relative values (for example `/health`) are resolved against `managedServer.http.baseURL`
- absolute values (for example `https://api.example.com/health`) are allowed when scheme/host match the base URL
- query parameters are not supported in this field

### `managed-server.http.auth` (required one-of)

Exactly one auth method must be configured:

- `oauth2`
- `basicAuth`
- `customHeaders`

#### OAuth2 example

```yaml
auth:
  oauth2:
    tokenURL: https://auth.example.com/oauth/token
    grantType: client_credentials
    clientID: declarest
    clientSecret: ${OAUTH_CLIENT_SECRET}
    # optional depending on flow/provider:
    # username: alice
    # password: secret
    # scope: api.read api.write
    # audience: https://api.example.com
```

#### Basic auth example

```yaml
auth:
  basicAuth:
    username: alice
    password: ${API_PASSWORD}
```

#### Custom headers auth example

```yaml
auth:
  customHeaders:
    - header: Authorization
      prefix: Bearer
      value: ${API_TOKEN}
    - header: X-Tenant
      value: acme
```

Each entry requires `header` and `value`; `prefix` is optional. When set, DeclaREST sends `<prefix> <value>` in the configured header.

### OpenAPI (`managedServer.http.openapi`)

Optional but recommended.

When configured, DeclaREST can:

- improve default metadata inference (`metadata infer`)
- improve operation defaults for path/method/media behavior
- help path completion with API path templates

Explicit metadata overrides still win.

When omitted, and `metadata.bundle` or `metadata.bundleFile` is configured, DeclaREST attempts OpenAPI fallback from the bundle:

- `bundle.yaml` `declarest.openapi` (URL or relative path inside the bundle)
- bundled `openapi.yaml` at bundle root (peer of `bundle.yaml`)

Precedence is deterministic: context `managedServer.http.openapi` overrides bundle-provided OpenAPI sources.

## Secret store configuration (`secretStore`, optional)

Exactly one provider when `secretStore` is configured:

- `file`
- `vault`

### File secret store

```yaml
secretStore:
  file:
    path: /work/declarest/secrets.json
    passphrase: ${DECLAREST_SECRET_PASSPHRASE}
    # or exactly one of the following key sources instead of passphrase:
    # key: <base64-encoded-32-byte-key>
    # keyFile: /path/to/key.txt
    # passphraseFile: /path/to/passphrase.txt
    # kdf:
    #   time: 1
    #   memory: 65536
    #   threads: 4
```

`secretStore.file` key source is also a one-of:

- `key`
- `keyFile`
- `passphrase`
- `passphraseFile`

### Vault secret store (KV)

```yaml
secretStore:
  vault:
    address: https://vault.example.com
    mount: secret
    pathPrefix: declarest
    kvVersion: 2
    auth:
      token: ${VAULT_TOKEN}
      # or password / appRole blocks
    tls:
      caCertFile: /path/to/ca.pem
      client-cert-file: /path/to/client.pem
      clientKeyFile: /path/to/client-key.pem
      insecureSkipVerify: false
```

Vault auth under `secretStore.vault.auth` is also one-of:

- token (`token`)
- password (`password` block)
- appRole (`appRole` block)

### `secretStore.vault.proxy` (optional)

Configure Vault HTTP proxy settings when connecting to the secret store:

```yaml
secretStore:
  vault:
    proxy:
      httpURL: http://proxy.example.com:3128
      httpsURL: http://proxy.example.com:3128
      no-proxy: localhost
      auth:
        username: proxy-user
        password: proxy-pass
```

Proxy configuration uses the same propagation rules described above; an empty `proxy:` block disables any inherited proxy for Vault.

## Metadata configuration (`metadata`, optional)

Choose at most one metadata source:

```yaml
metadata:
  # local metadata tree
  # baseDir: /path/to/metadata
  #
  # or bundle reference (shorthand or URL)
  # bundle: keycloak-bundle:{{ declarest_version() }}
  # bundle: https://github.com/crmarques/declarest-bundle-keycloak/releases/download/{{ declarest_tag() }}/keycloak-bundle-{{ declarest_version() }}.tar.gz
  #
  # or local bundle archive
  # bundleFile: /path/to/keycloak-bundle-{{ declarest_version() }}.tar.gz
```

When all metadata sources are omitted, `metadata.baseDir` defaults to the selected repository base dir.

Use `metadata.baseDir` when you want metadata files stored separately from resource payload files.
Use `metadata.bundle` when you want metadata definitions to be consumed from a shorthand or URL bundle reference.
Use `metadata.bundleFile` when you want metadata definitions to be consumed from a local bundle archive file.

### `metadata.proxy` (optional)

```yaml
metadata:
  proxy:
    httpURL: http://proxy.example.com:3128
    httpsURL: http://proxy.example.com:3128
    no-proxy: localhost,127.0.0.1
    auth:
      username: proxy-user
      password: proxy-pass
```

`metadata.proxy` configures the HTTP proxy used when downloading metadata bundles. It follows the same inheritance rules as other proxy blocks, and an empty `proxy:` block disables any inherited proxy for metadata.

Bundle OpenAPI hints (when present) can also supply `managedServer.http.openapi` automatically if context OpenAPI is not set.

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
      git:
        local:
          baseDir: /srv/declarest/keycloak-prod
        remote:
          url: git@github.com:acme/keycloak-config.git
          branch: main
          provider: github
          autoSync: true
          auth:
            ssh:
              user: git
              privateKeyFile: /home/declarest/.ssh/id_ed25519
              known-hosts-file: /home/declarest/.ssh/known_hosts
    managed-server:
      http:
        baseURL: https://sso.example.com/admin
        openapi: /srv/declarest/openapi/keycloak-admin.json
        auth:
          oauth2:
            tokenURL: https://sso.example.com/realms/master/protocol/openid-connect/token
            grantType: client_credentials
            clientID: declarest
            clientSecret: ${KEYCLOAK_CLIENT_SECRET}
        tls:
          caCertFile: /etc/ssl/certs/internal-ca.pem
    secretStore:
      vault:
        address: https://vault.example.com
        mount: secret
        pathPrefix: declarest/keycloak-prod
        kvVersion: 2
        auth:
          appRole:
            roleID: ${VAULT_ROLE_ID}
            secretID: ${VAULT_SECRET_ID}
            mount: appRole
    metadata:
      baseDir: /srv/declarest/keycloak-metadata
    preferences:
      env: prod
      service: keycloak

currentContext: keycloak-prod
```

## Validation and editing workflow

```bash
# generate a commented template
declarest context print-template > /tmp/contexts.yaml

# validate before import
declarest context validate --payload /tmp/contexts.yaml

# import and set current
declarest context add --payload /tmp/contexts.yaml --set-current

# inspect resolved context
declarest context resolve
```

## Runtime override precedence

Effective config precedence is:

1. runtime flags/override inputs
2. environment overrides
3. persisted context values
4. engine defaults

### `context resolve --set` overrides (supported keys)

The canonical override keys are:

- `repository.git.local.baseDir`
- `repository.filesystem.baseDir`
- `managedServer.http.baseURL`
- `managedServer.http.healthCheck`
- `metadata.baseDir`
- `metadata.bundle`
- `metadata.bundleFile`

Example:

```bash
declarest context resolve \
  --set managedServer.http.baseURL=https://staging-api.example.com \
  --set metadata.bundle=keycloak-bundle:{{ declarest_version() }}
```

## Environment variables

### Context catalog location

- `DECLAREST_CONTEXTS_FILE`: overrides the path to the context catalog file

Example:

```bash
export DECLAREST_CONTEXTS_FILE=/work/declarest/configs/contexts.yaml
declarest context list
```

## Failure modes to expect (and validate early)

- both `repository.git` and `repository.filesystem` set
- missing `managedServer`
- missing/ambiguous `managed-server.http.auth` method
- invalid `managedServer.http.healthCheck` (query parameters or unsupported URL form)
- unknown YAML key (strict decoding)
- `currentContext` points to a non-existent context name
- invalid secretStore one-of configuration
- multiple metadata sources set in the same context (`metadata.baseDir`, `metadata.bundle`, `metadata.bundleFile`)
