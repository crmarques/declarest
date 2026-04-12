# Configuration Reference

This page documents the canonical `contexts.yaml` format used by declarest.

## Catalog

```yaml
currentContext: dev
# defaultEditor: vi

credentials:
  - name: shared
    username: api-user
    password: secret

contexts:
  - name: dev
    repository:
      filesystem:
        baseDir: /work/repo
    managedService:
      http:
        url: https://api.example.com
        auth:
          customHeaders:
            - header: Authorization
              prefix: Bearer
              value: change-me
```

Rules:

- `contexts` and `currentContext` are required.
- `credentials` is optional.
- Keys use camelCase only.
- Legacy aliases and unknown keys are rejected.

## Reusable credentials

Credentials are declared once and referenced anywhere basic username/password material is needed.

Literal values:

```yaml
credentials:
  - name: shared
    username: api-user
    password: secret
```

Prompt-backed values:

```yaml
credentials:
  - name: prompt-shared
    username:
      prompt: true
      persistInSession: true
    password:
      prompt: true
      persistInSession: true
```

Reference form:

```yaml
basic:
  credentialsRef:
    name: prompt-shared
```

`credentialsRef` works like a placeholder. At runtime, declarest injects the referenced credential content at that location and removes only the credential `name`.

To reuse `persistInSession: true` values across later `declarest` commands in one shell session, install the shell hook first:

```bash
eval "$(declarest context session-hook bash)"
# or
eval "$(declarest context session-hook zsh)"
```

Prompt-backed session cache files are written only under `XDG_RUNTIME_DIR/declarest/prompt-auth/` and the hook removes them on shell exit. If `XDG_RUNTIME_DIR` is unavailable, prompted values are reused only within the current `declarest` process.

## Repository

Choose exactly one of `git` or `filesystem`.

Filesystem:

```yaml
repository:
  filesystem:
    baseDir: /work/repo
```

Git:

```yaml
repository:
  git:
    local:
      baseDir: /work/repo
      autoInit: true
    remote:
      url: https://example.com/org/repo.git
      branch: main
      provider: github
      autoSync: true
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
```

`repository.git.remote.auth` accepts exactly one of:

- `basic`
- `ssh`
- `accessKey`

## Managed service

`managedService.http.url` is required when `managedService` is present.

Basic auth:

```yaml
managedService:
  http:
    url: https://api.example.com
    healthCheck: /health
    auth:
      basic:
        credentialsRef:
          name: shared
```

OAuth2:

```yaml
managedService:
  http:
    url: https://api.example.com
    auth:
      oauth2:
        tokenURL: https://sso.example.com/oauth/token
        grantType: client_credentials
        clientID: change-me
        clientSecret: change-me
```

Custom headers:

```yaml
managedService:
  http:
    url: https://api.example.com
    auth:
      customHeaders:
        - header: X-API-Key
          value: change-me
```

Proxy:

```yaml
managedService:
  http:
    url: https://api.example.com
    auth:
      customHeaders:
        - header: Authorization
          value: token
    proxy:
      http: http://proxy.example.com:3128
      https: http://proxy.example.com:3128
      noProxy: localhost,127.0.0.1
      auth:
        basic:
          credentialsRef:
            name: prompt-shared
```

Notes:

- `managedService.http.auth` accepts exactly one of `oauth2`, `basic`, or `customHeaders`.
- `managedService.http.healthCheck` is optional.
- When omitted, `server check` probes the normalized path from `managedService.http.url`.
- Relative health checks are resolved against `managedService.http.url`.

## Secret store

Choose exactly one of `file` or `vault`.

File:

```yaml
secretStore:
  file:
    path: /work/secrets.json
    passphrase: change-me
```

Vault userpass via reusable credentials:

```yaml
secretStore:
  vault:
    address: https://vault.example.com
    auth:
      password:
        credentialsRef:
          name: prompt-shared
        mount: userpass
```

Vault auth accepts exactly one of:

- `token`
- `password`
- `appRole`

## Metadata

Choose at most one of:

- `metadata.baseDir`
- `metadata.bundle`
- `metadata.bundleFile`

Example:

```yaml
metadata:
  bundle: keycloak-bundle:0.0.1
```

When all metadata sources are omitted, `metadata.baseDir` defaults to the repository base dir.

## Runtime overrides

Supported override keys:

- `repository.git.local.baseDir`
- `repository.filesystem.baseDir`
- `managedService.http.url`
- `managedService.http.healthCheck`
- `managedService.http.proxy.http`
- `managedService.http.proxy.https`
- `managedService.http.proxy.noProxy`
- `metadata.baseDir`
- `metadata.bundle`
- `metadata.bundleFile`

Example:

```bash
declarest context resolve \
  --set managedService.http.url=https://staging-api.example.com \
  --set metadata.bundle=keycloak-bundle:0.0.2
```
