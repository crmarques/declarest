# Contexts and Configuration

## Purpose
Define the context catalog YAML schema, credential indirection, proxy model, `${ENV_VAR}` expansion, metadata-source selection, and catalog path/precedence. Config type field-lists are owned by `agents/reference/interfaces.md`; this file owns their wire schema, semantics, and validation.

## Normative Rules

### Catalog location and shape
1. Context catalogs MUST be stored at `~/.declarest/configs/contexts.yaml` by default; `DECLAREST_CONTEXTS_FILE` MAY override the path.
2. Persisted YAML keys MUST use camelCase. Readers MUST reject legacy aliases and any unknown YAML key.
3. `contexts` MUST be a list of full context objects with unique, non-empty `name` values.
4. `currentContext` MUST reference an existing context when `contexts` is non-empty, and MUST be empty when `contexts` is empty.
5. Each context MUST define at least one of `repository` or `managedService`, and MAY define `secretStore`, `metadata`, `preferences`. The catalog MAY define top-level `defaultEditor` and `credentials`.
6. Validation MUST fail fast when any one-of block (below) is missing or ambiguous.
7. Changes to the persisted context or catalog wire shape MUST update `schemas/context.schema.json` and `schemas/contexts.schema.json` in the same change.

### One-of constraints
8. `repository` MUST set exactly one of `git` or `filesystem`.
9. `repository.git.remote.auth`, when configured, MUST set exactly one of `basic`, `ssh`, `accessKey`.
10. `managedService` MUST define `http`, and `http.url` is required.
11. `managedService.http.auth` MUST set exactly one of `oauth2`, `basic`, `customHeaders`. Each `customHeaders[*]` MUST define `header` and `value`; `prefix` is optional.
12. `secretStore` MUST set exactly one of `file` or `vault`. `secretStore.file` MUST set exactly one of `key`, `keyFile`, `passphrase`, `passphraseFile`. `secretStore.vault.auth` MUST set exactly one of `token`, `password`, `appRole`.

### Credentials and credentialsRef
13. `credentials` MAY be omitted; when present, credential names MUST be unique, and each entry MUST define `name`, `username`, `password`.
14. Each credential attribute (`username`/`password`) MUST be either a non-empty string or an object `{prompt: true}`; `prompt: false` on such an object is invalid. `persistInSession` MAY be set only on the prompt object and defaults to `false`.
15. All persisted basic-auth (`repository.git.remote.auth.basic`, `managedService.http.auth.basic`, `secretStore.vault.auth.password`, any proxy `auth.basic`) MUST use `credentialsRef: {name: <catalog credential>}`; inline username/password pairs in those blocks are invalid.
16. `credentialsRef.name` MUST match a catalog credential, else validation fails.
17. When a component defines `credentialsRef`, runtime MUST inject the referenced credential object at that location, omitting the credential `name` field.
18. `secretStore.vault.auth.password.mount` is optional and defaults to `userpass` at runtime.

### Prompt-backed credentials and session cache
19. A referenced prompt-backed attribute MUST prompt only when the owning component first needs it at runtime.
20. Non-interactive execution MUST fail when a required prompt-backed attribute has no cached session value.
21. `persistInSession: true` MUST reuse prompted values across later `declarest` commands only when the shell exported `DECLAREST_PROMPT_AUTH_SESSION_ID` (e.g. via `declarest context session-hook bash|zsh`) and `XDG_RUNTIME_DIR` is available. When set, new cache files MUST be written only under `XDG_RUNTIME_DIR/declarest/prompt-auth/` and MUST NOT fall back to `~/.declarest/sessions`.
22. `context clean --credentials-in-session` MUST remove the detected `XDG_RUNTIME_DIR/declarest/prompt-auth/` cache file and any matching legacy `~/.declarest/sessions` cache file so later commands in that shell no longer reuse them.

### Proxy model
23. Proxy blocks (`managedService.http.proxy`, `repository.git.remote.proxy`, `secretStore.vault.proxy`, `metadata.proxy`) MAY define any subset of `http`, `https`, `noProxy`, `auth`. An empty `proxy: {}` MUST explicitly disable inherited and environment proxy for that component only.
24. Proxy `auth`, when present, MUST define `basic.credentialsRef`.
25. Proxy precedence MUST be: process environment (`HTTP_PROXY`, `HTTPS_PROXY`, `NO_PROXY`, plus lowercase aliases), then component-local proxy fields overriding only the fields they define, then shared-context inheritance where the first configured concrete proxy becomes the default for components that do not define their own proxy block.

### Metadata source
26. `metadata` MUST define at most one of `baseDir`, `bundle`, `bundleFile`.
27. `metadata.bundle` accepts four reference forms, in priority order: `oci://<registry>/<repository>:<tag|@digest>` (default for published bundles), `<name>:<version>` (GitHub-release shorthand), `http`/`https` URL to a `.tar.gz`, or an absolute path to a local `.tar.gz`.
28. `metadata.baseDir` MUST default to the selected repository baseDir when all metadata sources are unset, and MUST be omitted from persisted YAML when it equals the repository baseDir.
29. When `managedService.http.openapi` is empty and `metadata.bundle`/`metadata.bundleFile` is set, startup MUST resolve OpenAPI from bundle hints in order: `bundle.yaml declarest.openapi`, then peer `openapi.yaml` at the bundle root. `bundle.yaml` shape, strict decode, and compatibility gates are owned by `agents/reference/metadata-bundle.md`.

### Other component rules
30. `repository.git.remote.autoSync` MAY be omitted and MUST be treated as enabled; only an explicit `false` disables automatic push.
31. `managedService.http.healthCheck` MAY be a relative path or an absolute `http|https` URL and MUST NOT include query parameters; when omitted it defaults to the normalized `managedService.http.url` path.

### Resolution and precedence
32. Config precedence MUST be: runtime flags, environment placeholders, persisted context values, engine defaults.
33. Exact-match string values of the form `${ENV_VAR}` MUST resolve from the process environment before validation, defaulting, and active-context resolution; persisted YAML MUST keep the placeholder text unchanged. A required placeholder resolving to empty/invalid content MUST fail.
34. Runtime overrides MUST be limited to these keys; any other override key MUST fail: `repository.git.local.baseDir`, `repository.filesystem.baseDir`, `managedService.http.url`, `managedService.http.healthCheck`, `managedService.http.proxy.http`, `managedService.http.proxy.https`, `managedService.http.proxy.noProxy`, `metadata.baseDir`, `metadata.bundle`, `metadata.bundleFile`. Overrides MUST NOT mutate the catalog file.

## Canonical YAML Template
```yaml
currentContext: dev

credentials:
  - name: shared
    username: change-me
    password: change-me
  - name: prompt-shared            # username and/or password may prompt independently
    username: { prompt: true, persistInSession: true }
    password: { prompt: true }

contexts:
  - name: dev
    repository:
      git:
        local: { baseDir: /path/to/repo }
        remote:
          url: https://example.com/org/repo.git
          branch: main
          provider: github
          auth: { basic: { credentialsRef: { name: shared } } }
          proxy:                   # empty `proxy: {}` disables inherited/env proxy
            http: http://proxy.example.com:3128
            https: http://proxy.example.com:3128
            noProxy: localhost,127.0.0.1
            auth: { basic: { credentialsRef: { name: prompt-shared } } }
    managedService:
      http:
        url: https://example.com/api
        healthCheck: /health
        auth: { basic: { credentialsRef: { name: shared } } }
    secretStore:
      vault:
        address: https://vault.example.com
        auth:
          password:
            credentialsRef: { name: prompt-shared }
            mount: userpass
    metadata:
      bundle: keycloak-bundle:0.0.1
```

## Failure Modes
1. Duplicate context names; duplicate credential names; unknown key or legacy alias.
2. Missing/ambiguous one-of block; `credentialsRef.name` matches no catalog credential; prompt object with `prompt: false`.
3. Required credential attribute or required `${ENV_VAR}` resolves to empty.
4. `healthCheck` invalid or carries query parameters; runtime override key not in the supported list.

## Examples
1. `ResolveContext({Name:"", Overrides:nil})` loads the context named by `currentContext`. Empty catalog (`contexts: []`, `currentContext: ""`) is valid.
2. `ResolveContext({Name:"dev", Overrides:{"managedService.http.url":"https://staging.example.com"}})` applies the override without mutating the catalog.
3. A credential `prompt-shared` referenced by both `managedService.http.auth.basic` and `repository.git.remote.proxy.auth.basic` prompts each component only when first used; one credential may prompt `username` while keeping a literal `password`.
4. A context defining only `proxy.noProxy` inherits `http`/`https` from env or the shared proxy; `proxy: {}` disables both for that component only.
5. A repository-only context still defaults `metadata.baseDir` from the repository.
6. `context clean --credentials-in-session` clears prompt cache files even with no current context. When `XDG_RUNTIME_DIR` is unavailable, `persistInSession: true` reuses within one running process but later commands re-prompt (no cross-command cache file is created).
