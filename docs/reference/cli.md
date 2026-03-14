# CLI Reference

This page summarizes the current CLI surface and the most important command and flag patterns.

For exact flags and examples, use built-in help (source of truth):

```bash
declarest --help
declarest <group> --help
declarest <group> <command> --help
```

## Command groups

### Basic commands

- `context` - manage contexts and validation
- `repository` - manage local repository state
- `resource` - save/get/list/diff/explain/apply/create/update/delete/edit/copy resources, inspect and mutate metadata, manage metadata-backed defaults, plus raw requests and template rendering
- `server` - inspect managed server connectivity and auth-derived values
- `secret` - initialize, detect, store, get, resolve, mask, normalize secrets

### Utility commands

- `completion` - generate shell completion scripts
- `version` - print CLI version/build info

## Global flags

Available on all commands:

- `-c, --context <name>` - choose context
- `-o, --output <auto|text|json|yaml>` - output format
- `-d, --debug` - debug output
- `-v, --verbose` - show complementary output for commands that suppress it by default
- `-n, --no-status` - hide status footer lines
- `--no-color` - disable ANSI color

## Path input conventions

Most path-aware commands accept either:

- positional `[path]`
- `--path`, `-p`

Paths are logical absolute paths (for example `/corporations/acme`).
A trailing `/` marks a collection path.

## `resource` command family (most-used workflows)

### Read and inspect

```bash
declarest resource get /corporations/acme
declarest resource get --source repository /corporations/acme
declarest resource get --source repository /corporations/acme --prune-defaults
declarest resource get /corporations/acme --show-metadata
declarest resource list /customers/
declarest resource list /customers/ --output text
declarest resource explain /corporations/acme
declarest resource diff /corporations/acme
declarest resource diff /corporations --recursive
declarest resource diff /corporations --recursive --list
```

`resource diff` defaults to normalized unified text output. For one resource, it prints one grouped section. For collection paths, it prints one section per changed resource, skips unchanged resources by default, and `--list` prints only the drifting logical paths. Add `--color always` to force ANSI coloring, or use `-o json|yaml` when you need structured `DiffEntry` output for automation.

### Import/save into repository

```bash
declarest resource save /corporations/acme
declarest resource save /corporations/acme --force
declarest resource save /corporations/acme --payload '{"id":"acme","name":"Acme"}' --force
declarest resource save /corporations/acme --payload '/id=acme,/name=Acme,/spec/tier=gold' --force --message ticket-123
declarest resource save /corporations/acme --secret-attributes
declarest resource save /corporations/acme --prune-defaults --force
declarest resource save /customers/ --mode auto
declarest resource save /customers/ --mode single
```

### Metadata-backed defaults

```bash
declarest resource defaults get /corporations/acme
declarest resource defaults edit /corporations/acme
declarest resource defaults config get /corporations/acme
declarest resource defaults config edit /corporations/acme
declarest resource defaults profile get /corporations/acme prod
declarest resource defaults profile edit /corporations/acme prod
declarest resource defaults profile delete /corporations/acme prod
declarest resource defaults infer /corporations/acme
declarest resource defaults infer /corporations/acme --save
declarest resource defaults infer /corporations/acme --check
declarest resource defaults infer /corporations/acme --managed-server --wait 2s --yes
declarest resource defaults infer /corporations/acme --managed-server --check --yes
```

Use this command family to keep shared values in `resource.defaults`, usually backed by deterministic selector-local files such as `defaults.yaml` and `defaults-<profile>.yaml`, while the rest of the CLI still works with the merged effective resource. `get` prints resolved defaults, `config get` prints the raw persisted metadata block, and `profile` manages named defaults profiles. Use `--save` to persist inferred baseline defaults, `--check` to validate the current resolved defaults without changing them, and do not combine those two flags. `resource defaults infer --managed-server` probes server-added defaults by creating temporary remote resources, so it requires `--yes`. Add `--wait <duration|seconds>` when the managed server needs extra time before the first probe readback; bare integers are treated as seconds.

### Mutate remote state

```bash
declarest resource apply /corporations/acme
declarest resource apply /corporations/acme --force
declarest resource create /corporations/acme
declarest resource update /corporations/acme
declarest resource delete /corporations/acme --yes
declarest resource delete /corporations/acme --yes --source repository --message "cleanup customer"
declarest resource edit /corporations/acme --editor "vi"
declarest resource copy /corporations/acme /corporations/acme-copy --override-attributes /name=acme-copy
```

### Raw HTTP and templates

```bash
declarest resource request get /corporations/acme
declarest resource request post /corporations --payload '{"id":"acme"}'
declarest resource template /corporations/acme --payload resource.json
```

Useful flags for mutation and payload-driven workflows:

- `--payload <path|->` for file/stdin payloads and inline JSON/YAML or JSON Pointer assignments (`/a=b,/c=d,/e/f/g=h`) on `resource apply|create|update|save`
- `--content-type <json|yaml|xml|hcl|ini|properties|text|txt|binary|mime>` for payload decoding overrides
- `--accept-type <mime|shortname>` on `resource request <method>` for explicit response media negotiation
- `--recursive` for collection recursion on supported commands
- `--force` on `resource apply` to execute update even when compare output has no drift
- `--mode <auto|items|single>` on `resource save` to choose between automatic list fan-out, forced item fan-out, or single-resource persistence
- `--prune-defaults` on `resource get|save` to remove fields already covered by resolved metadata defaults from printed or persisted payloads
- `--refresh` (apply/create/update)
- `--http-method <METHOD>` override for remote calls
- `--message <text>` overrides the default git commit message on `resource save`, `resource copy`, and repository-backed `resource delete`

## `resource metadata` command family (advanced API modeling)

```bash
declarest resource metadata get /corporations/acme
declarest resource metadata get /corporations/acme --overrides-only
declarest resource metadata resolve /corporations/acme
declarest resource metadata render /corporations/acme update
declarest resource metadata infer /corporations/acme
declarest resource metadata infer /corporations/acme --apply
```

Write/remove metadata definitions:

```bash
declarest resource metadata set /customers/ --payload metadata.json
declarest resource metadata unset /customers/
```

## `context` command family

```bash
declarest context add
declarest context add dev
declarest context edit
declarest context edit dev
declarest context edit dev --editor "vi"
declarest context print-template
declarest context validate --payload contexts.yaml
declarest context add --payload contexts.yaml --set-current
declarest context current
declarest context show
declarest context resolve
declarest context check
```

Useful for environment-specific testing without editing stored config:

```bash
declarest context resolve --set managedServer.http.baseURL=https://staging-api.example.com
```

## `repository` command family (git/filesystem backends)

```bash
declarest repository status
declarest repository tree
declarest repository clean
declarest repository commit --message "manual repository changes"
declarest repository history
declarest repository history --oneline --max-count 10 --author alice --grep fix --path customers
declarest repository init
declarest repository refresh
declarest repository push
declarest repository reset
declarest repository check
```

Notes:

- `repository push` is only valid for `git` repository contexts.
- `repository commit` and `repository history` are only supported for `git` repositories.
- `repository tree` prints local directory layout only (directories, deterministic order).
- `repository clean` discards local uncommitted changes (tracked and untracked) for `git` repositories and is a no-op for `filesystem` repositories.
- Git-backed repository operations auto-initialize the local `.git` repository on first use when the repository base dir exists but Git metadata is missing.
- `repository reset` is destructive; review local changes before running it.

## `secret` command family

```bash
declarest secret init
declarest secret detect /customers/
declarest secret detect --fix /customers/
declarest secret set /corporations/acme /apiToken secret-value
declarest secret list /corporations/acme
declarest secret list /projects --recursive
declarest secret get /corporations/acme /apiToken
```

Use `secret detect --fix` plus `resource save --secret-attributes` for the safest onboarding flow.

## `server` command family

```bash
declarest server check
declarest server get base-url
declarest server get token-url
declarest server get access-token
```

These commands are useful when debugging auth or connectivity independently from resource reconciliation.

## Output and scripting tips

- Prefer `-o json` or `-o yaml` for automation.
- `resource list --output text` prints a concise `alias (id)` summary per item using metadata identity mapping when available.
- `repository tree`, `secret get`, `server get`, `server check`, `completion`, and `context print-template` are text-only outputs.
- `secret list` behaves like other list commands: `--output text` prints one key per line, while `--output json|yaml` is safer for automation.
- `secret list <path> --recursive` expands discovery to descendant secret-bearing paths and renders matches as the full relative path from the selected root, for example `/test/secrets/private-key:.`.
- Explicit non-structured `--content-type` values such as `text`, `txt`, or `text/plain` keep inline payloads literal, so `--payload a=b --content-type txt` is saved as text, not parsed as JSON-style assignment shorthand.
- Some commands intentionally suppress payload output unless `--verbose` is used (especially state-changing commands).
- Status lines are printed to stderr by default; use `--no-status` when piping stdout.
- `resource get` redacts metadata-declared secret attributes by default; use `--show-secrets` only when necessary.
- `resource defaults get` prints the effective resolved defaults object; when no defaults are resolved yet, it returns an empty object.

## Recommended debug sequence for advanced metadata issues

```bash
declarest resource metadata get /path
declarest resource metadata render /path get
declarest resource metadata render /path update
declarest resource explain /path
```

(Use a concrete path that exercises the selector/override you are validating.)
