# Contexts

A context is the combination of repository backend, managed server, secrets, and metadata overrides that the CLI resolves before running any workflow. Context catalogs keep those definitions in one place so the same GitOps stack can target multiple APIs without changing source files or environment variables.

## Catalog layout and invariants

- Context catalogs MUST live at `~/.declarest/configs/contexts.yaml` by default. The `DECLAREST_CONTEXTS_FILE` environment variable MAY point to an alternate file, but the structure inside remains the same.
- The catalog YAML MUST contain `contexts` (a list of complete context objects) and `current-ctx` (the active context name). An optional `default-editor` value MAY be provided and defaults to `vi` when absent.
- Context names MUST be unique, non-empty, and appear exactly once in the `contexts` list. Duplicate names fail validation before any CLI operation mutates remote state.
- Each context object MUST include `repository` plus `managed-server`. Optional blocks are `secret-store`, `metadata`, and `preferences`.
- The repository block MUST set exactly one of `git` or `filesystem`. Repository payload filenames are derived at runtime from managed-server responses or explicit payload input.
- The managed server block MUST include an `http` section that in turn defines `base-url` and an `auth` section. Under `auth`, exactly one of `oauth2`, `basic-auth`, or `custom-headers` MUST be present, and custom headers entries MUST include both `header` and `value` (with an optional `prefix`). `managed-server.http.health-check` is optional and defines the probe target used by `managed-server check`.
- `managed-server.http.proxy` MAY be configured. If present, it MUST define at least one of `http-url` or `https-url`. When proxy auth is set, both `username` and `password` are REQUIRED.
- The optional `secret-store` block MUST define exactly one of `file` or `vault`. File-based stores require one of `key`, `key-file`, `passphrase`, or `passphrase-file`.
- The optional `metadata` block MAY point to `base-dir`, `bundle`, or `bundle-file`; at most one source is allowed. When all metadata sources are unset, merge logic defaults to the repository base dir and `metadata.base-dir` SHOULD be omitted in persisted YAML when it matches that default.
- When `metadata.bundle` or `metadata.bundle-file` is configured and `managed-server.http.openapi` is empty, startup MUST load OpenAPI from the bundle hints: first `bundle.yaml` `declarest.openapi`, then any `openapi.yaml`/`openapi.yml`/`openapi.json` in the bundle root. `metadata.base-dir` remains empty in this case so the bundle takes precedence.
- YAML decoding is strict: unknown keys anywhere in the catalog MUST fail parsing, and missing blocks or one-of violations (for example both `repository.git` and `repository.filesystem`) result in validation errors instead of silent defaults.

## Selecting the active context

- `current-ctx` points to the context that `ResolveContext` returns when no explicit name is provided. If the catalog is missing or empty, the resolver treats it as an empty catalog and `current-ctx` is effectively unset. `ResolveContext` therefore guarantees that `current-ctx` must reference an existing entry when any contexts exist.
- Runtime inputs follow this precedence: CLI flags (for example `--context`) override `DECLAREST_CONTEXTS_FILE` overrides, which override persisted catalog values, which override library defaults. That means an override key like `managed-server.http.base-url` or `managed-server.http.health-check` can be supplied at runtime and will shadow the catalog value without mutating the file.
- Context selection happens through the `config` subcommands. `declarest config current` prints the active context name; `declarest config use <name>` updates `current-ctx` after validating the target context; `declarest config resolve [<name>]` runs the same `ResolveContext` logic with optional override flags.
- `SetCurrent`, `GetCurrent`, and `ResolveContext` operations expose the same invariants the CLI enforces: missing contexts return `NotFoundError`, duplicate names are rejected, and override keys outside the supported list (for example `unknown.key`) raise `ValidationError` before CLI execution proceeds.

## CLI workflows for contexts

| Command | Role |
| --- | --- |
| `declarest config print-template` | Emits a commented template that documents every supported context field and one-of branch. |
| `declarest config add <name>` | Creates a new context roughly matching the template, enforcing repository, managed server, and optional block rules. |
| `declarest config edit <name>` | Loads the context into a temporary document, allows manual editing with the configured editor (`default-editor` or `vi`), validates the edited YAML, and replaces only that context when validation passes. |
| `declarest config list` | Shows every context and its effective server URL plus secret-store hints. |
| `declarest config current` | Shows the `current-ctx` name and the resolved server URL. |
| `declarest config use <name>` | Updates `current-ctx` so future commands resolve that context. |
| `declarest config validate` | Rejects unknown keys, one-of violations, or missing required blocks anywhere in the catalog. |
| `declarest config resolve [<name>]` | Resolves a named (or current) context, applies runtime overrides, and reports the resulting configuration without mutating state. |

Each workflow enforces strict decoding and failure-fast validation before proceeding. `config edit` never writes partial documents—if validation fails, the catalog stays untouched. `config add` and `config edit` MUST keep `contexts` ordered so `current-ctx` resolution remains predictable.

## Failure modes and corner cases

- `current-ctx` missing or referencing a non-existent context causes `ResolveContext` to return `NotFoundError` and forces CLI commands to fail fast before making remote calls.
- Duplicate context names or unknown override keys (`ContextSelection.Overrides`) cause `ValidationError` during `config edit`, `config add`, or override resolution.
- Omitting `managed-server` or its `http` block results in validation failure before the bootstrap session is created.
- Invalid `managed-server.http.health-check` values (unsupported URL form or query parameters) fail validation before command execution.
- Proxy configurations without `http-url` or `https-url`, or with incomplete auth, are rejected during validation.
- `metadata.bundle` or `metadata.bundle-file` without an accompanying OpenAPI hint can still provide metadata defaults, but you MUST keep `managed-server.http.openapi` empty so the bundle hints are used instead of conflicting files.

### Corner case example

```
# runtime overrides that cannot be applied
declarest config resolve dev --override "unknown.key=value"
```

The preceding command fails because `unknown.key` is not on the supported override list (`metadata.base-dir`, `repository.git.local.base-dir`, etc.). The CLI reports a validation error before it tries to mutate the catalog or call the API.

## Practical tip

When you edit contexts manually, drop `metadata.base-dir` when it equals the repository base dir so the persistence layer can rely on the implicit default. Keep bundle metadata sources (`metadata.bundle` or `metadata.bundle-file`) and `managed-server.http.openapi` mutually exclusive: only leave bundle hints populated when you intend to rely on extracted OpenAPI files instead of an explicit URL.
