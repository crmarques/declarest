# Contexts

A context is the resolved combination of repository, managed-server, secret-store, metadata, and reusable credential settings that the CLI uses for one run.

## Catalog shape

- Context catalogs live at `~/.declarest/configs/contexts.yaml` by default. `DECLAREST_CONTEXTS_FILE` may point to a different file.
- The catalog contains `contexts`, `currentContext`, optional `defaultEditor`, and optional top-level `credentials`.
- Context names must be unique and non-empty.
- Each context must define at least one of `repository` or `managedServer`.
- YAML decoding is strict. Legacy aliases and unknown keys are rejected.

## Reusable credentials

Reusable credentials are defined once at catalog scope and referenced from context auth blocks:

```yaml
credentials:
  - name: shared
    username: api-user
    password:
      prompt: true
      persistInSession: true
```

Components refer to that entry with `credentialsRef`:

```yaml
managedServer:
  http:
    url: https://api.example.com
    auth:
      basic:
        credentialsRef:
          name: shared
```

`credentialsRef` is a placeholder. At runtime, declarest injects the referenced credential content into that location and omits only the credential `name`. Prompt-backed attributes ask for input only when the owning component first needs them.

If you want `persistInSession: true` to reuse prompted values across later `declarest` commands in the same shell, enable the shell hook first:

```bash
eval "$(declarest context session-hook bash)"
# or
eval "$(declarest context session-hook zsh)"
```

When `XDG_RUNTIME_DIR` is available, declarest stores that prompt cache only under `XDG_RUNTIME_DIR/declarest/prompt-auth/` and removes it on shell exit through the hook. Without `XDG_RUNTIME_DIR`, prompted values are reused only within the current `declarest` command.

## Context rules

- `managedServer.http.url` is the canonical managed-server URL key.
- `managedServer.http.auth` must define exactly one of `oauth2`, `basic`, or `customHeaders`.
- `repository.git.remote.auth` must define exactly one of `basic`, `ssh`, or `accessKey`.
- `secretStore.vault.auth` must define exactly one of `token`, `password`, or `appRole`.
- Proxy blocks use `http`, `https`, `noProxy`, and optional `auth.basic.credentialsRef`.
- `metadata` may define at most one of `baseDir`, `bundle`, or `bundleFile`.
- When all metadata sources are omitted, `metadata.baseDir` defaults to the repository base dir and should usually be omitted from persisted YAML.

## Resolution and overrides

- `currentContext` is used when no explicit context name is provided.
- Exact `${ENV_VAR}` placeholders stay on disk unchanged but are expanded before runtime validation.
- Runtime overrides do not mutate the catalog. Supported keys are:
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

## Example

```yaml
currentContext: dev

credentials:
  - name: shared
    username: api-user
    password:
      prompt: true
      persistInSession: true

contexts:
  - name: dev
    repository:
      git:
        local:
          baseDir: /work/repo
        remote:
          url: https://example.com/org/repo.git
          auth:
            basic:
              credentialsRef:
                name: shared
    managedServer:
      http:
        url: https://api.example.com
        auth:
          basic:
            credentialsRef:
              name: shared
```
