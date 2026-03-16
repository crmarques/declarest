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
    managedServer:
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

## Managed server

`managedServer.http.url` is required when `managedServer` is present.

Basic auth:

```yaml
managedServer:
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
managedServer:
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
managedServer:
  http:
    url: https://api.example.com
    auth:
      customHeaders:
        - header: X-API-Key
          value: change-me
```

Proxy:

```yaml
managedServer:
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

- `managedServer.http.auth` accepts exactly one of `oauth2`, `basic`, or `customHeaders`.
- `managedServer.http.healthCheck` is optional.
- When omitted, `server check` probes the normalized path from `managedServer.http.url`.
- Relative health checks are resolved against `managedServer.http.url`.

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
- `managedServer.http.url`
- `managedServer.http.healthCheck`
- `managedServer.http.proxy.http`
- `managedServer.http.proxy.https`
- `managedServer.http.proxy.noProxy`
- `metadata.baseDir`
- `metadata.bundle`
- `metadata.bundleFile`

Example:

```bash
declarest context resolve \
  --set managedServer.http.url=https://staging-api.example.com \
  --set metadata.bundle=keycloak-bundle:0.0.2
```
