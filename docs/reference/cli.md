# CLI Reference

This page summarizes the current CLI surface and the most important command/flag patterns.

For exact flags and examples, use built-in help (source of truth):

```bash
declarest --help
declarest <group> --help
declarest <group> <command> --help
```

## Command groups

### Basic commands

- `config` - manage contexts and validation
- `metadata` - inspect, infer, render, set, and unset metadata
- `repo` - manage local repository state
- `resource` - save/get/list/diff/apply/create/update/delete resources
- `resource-server` - inspect connectivity and auth-derived values
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
declarest resource get /corporations/acme --show-metadata
declarest resource list /customers/
declarest resource explain /corporations/acme
declarest resource diff /corporations/acme
```

### Import/save into repository

```bash
declarest resource save /corporations/acme
declarest resource save /corporations/acme --overwrite
declarest resource save /corporations/acme --handle-secrets
declarest resource save /customers/ --as-one-resource
```

### Mutate remote state

```bash
declarest resource apply /corporations/acme
declarest resource create /corporations/acme
declarest resource update /corporations/acme
declarest resource delete /corporations/acme --confirm-delete
```

Useful mutation flags:

- `--payload <path|->` for explicit input payloads
- `--format <json|yaml>` for payload decoding
- `--recursive` for collection recursion on supported commands
- `--refresh-repository` (apply/create/update)
- `--http-method <METHOD>` override for remote calls

## `metadata` command family (advanced API modeling)

```bash
declarest metadata get /corporations/acme
declarest metadata get /corporations/acme --overrides-only
declarest metadata resolve /corporations/acme
declarest metadata render /corporations/acme update
declarest metadata infer /corporations/acme
declarest metadata infer /corporations/acme --apply
```

Write/remove metadata definitions:

```bash
declarest metadata set /customers/ --payload metadata.json
declarest metadata unset /customers/
```

## `config` command family (context management)

```bash
declarest config create
declarest config print-template
declarest config validate --payload contexts.yaml
declarest config add --file contexts.yaml --set-current
declarest config current
declarest config show
declarest config resolve
declarest config check
```

Useful for environment-specific testing without editing stored config:

```bash
declarest config resolve --set resource-server.http.base-url=https://staging-api.example.com
```

## `repo` command family (git/filesystem backends)

```bash
declarest repo status
declarest repo init
declarest repo refresh
declarest repo push
declarest repo reset
declarest repo check
```

Notes:

- `repo push` is only valid for `git` repository contexts.
- `repo reset` is destructive; review local changes before running it.

## `secret` command family

```bash
declarest secret init
declarest secret detect /customers/
declarest secret detect --fix /customers/
declarest secret store "/corporations/acme:apiToken" "secret-value"
declarest secret get /corporations/acme
declarest secret get /corporations/acme apiToken
```

Use `secret detect --fix` plus `resource save --handle-secrets` for the safest onboarding flow.

## `resource-server` command family

```bash
declarest resource-server check
declarest resource-server get base-url
declarest resource-server get token-url
declarest resource-server get access-token
```

These commands are useful when debugging auth or connectivity independently from resource reconciliation.

## Output and scripting tips

- Prefer `-o json` or `-o yaml` for automation.
- Some commands intentionally suppress payload output unless `--verbose` is used (especially state-changing commands).
- Status lines are printed to stderr by default; use `--no-status` when piping stdout.
- `resource get` redacts metadata-declared secret attributes by default; use `--show-secrets` only when necessary.

## Recommended debug sequence for advanced metadata issues

```bash
declarest metadata get /path
declarest metadata render /path get
declarest metadata render /path update
declarest resource explain /path
```

(Use a concrete path that exercises the selector/override you are validating.)
