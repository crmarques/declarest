# CLI reference

Run `declarest --help` for the full command list.
This page highlights the main commands and what they do.

## Global flags

- `--no-status`: suppress status messages and print only command output.
- `--debug[=groups]`: always print grouped debugging info (even on success) when this flag is supplied; groups are `network`, `repository`, `resource`, `all`. The `network` group reveals the managed server type/base URL/auth, default headers, and each captured HTTP interaction (method, URL, headers, payload, status, headers, body) so you can inspect OAuth/token exchanges or other requests; `resource` enables extra CLI output during apply/create/etc.

## config

Manage contexts and configuration files.

- `config add`: register a context file. Use `--force` to override an existing context; when run without a config path the command switches into the interactive flow that prints section headings, hides secret input, requires a managed server block, and surfaces numbered defaults so less typing is needed.
- `config update`: update an existing context.
- `config use`: set the default context.
- `config list`: list all contexts.
- `config current`: show the current context.
- `config rename`: rename a context.
- `config delete`: remove a context.
- `config check`: validate configuration and connectivity (does not validate authentication).
- `config env`: display DECLAREST_* environment overrides and the resolved context store locations they control.
- `config print-template`: print a full config file.

## repo

Manage the resource repository.

- `repo init`: initialize local and (optionally) remote repositories.
- `repo refresh`: fast-forward local from remote.
- `repo push`: push local changes to remote.
- `repo reset`: hard reset local to remote.
- `repo check`: validate repository connectivity.

## resource

Operate on resource definitions.

- `resource get`: fetch from remote or repo without touching the repository (use `--repo` to read stored data and `--with-secrets` to include secret placeholders).
- `resource save`: fetch a remote resource and persist it in the repository; use `--force` to override saved definitions or to include secrets with `--with-secrets`, and add `--as-one-resource` when you want to store a fetched collection as a single repository entry (collections default to saving each item separately). `--print` can be added to show the payload in addition to writing it.
- `resource explain`: describe the metadata/OpenAPI interpretation for a logical path so you can understand the collection path, id/alias attributes, headers, and matching HTTP operations.
- `resource list`: list repo or remote paths.
- `resource add`: add a local resource from a file, another resource path, or an OpenAPI schema (supports overrides and optional remote apply).
- `resource create`: create a remote resource from the repo.
- `resource update`: update a remote resource from the repo.
- `resource apply`: create or update a remote resource.
- `resource diff`: show differences between repo and remote.
- `resource delete`: delete resources from repo, remote, or both (when `--remote` is set without `--repo`, remote-only deletion is assumed).

Path arguments (the positional `<path>` argument or the `--path` flag) now provide context-aware shell completion: remote-focused commands (like `resource get`/`save`) query the managed server, repo-focused commands (such as `resource apply`/`create`/`update`/`diff`) use the repository contents, and every completion list also surfaces configured OpenAPI path templates when available.

## ad-hoc

Send direct HTTP requests to the managed server while still honoring any metadata files that apply to the provided logical path (they can override URLs, headers, and templated placeholders).

- `ad-hoc get|post|put|patch|delete`: issue the named method. Pass the logical path either as `--path <path>` or as the positional argument.
- `--header`: repeat to add arbitrary headers (`Name: value` or `Name=value` format). Metadata-derived headers are merged first, then user headers override.
- `--default-headers`: re-apply the default `Accept: application/json` / `Content-Type: application/json` values even when metadata explicitly cleared them.
- `--payload`: supply an inline payload or prefix with `@` to load payload bytes from a file (useful for POST/PUT/PATCH/DELETE operations).

The command prints the raw response body to stdout and a `[OK] METHOD PATH STATUS` summary to stderr unless `--no-status` is supplied.

## metadata

Manage metadata definitions.

- `metadata get`: render effective metadata.
- `metadata set`: set an attribute.
- `metadata unset`: unset an attribute.
- `metadata add`: add metadata from a file.
- `metadata update-resources`: rewrite resources based on new metadata rules.
- `metadata infer`: infer resource metadata (id/alias attributes) from the OpenAPI spec (`--spec` overrides the configured spec, `--apply` writes the suggestions, `--id-from`/`--alias-from` force a value, and `--recursively` walks every collection defined under the supplied path).

Use `--for-resource-only` on any metadata subcommand to treat a path without a trailing slash as a resource instead of a collection default.

When `--recursively` is provided, the command prints a JSON payload whose `results` array contains the inferred `resourceInfo` plus `reasons` for each collection path. Add `--apply` to write the suggestions into the matching metadata files.

## secret

Manage secrets stored in the secret store.

- `secret init`: initialize the secret store.
- `secret add`: create or update a secret value.
- `secret get`: read a secret value.
- `secret delete`: remove a secret.
- `secret list`: list keys for a resource.
- `secret export`: write secrets under a path or all resources to CSV (`--path` or positional path, or `--all`).
- `secret import`: load secrets from a CSV file (`--file` or positional file) and use `--force` to override existing values.
- `secret check`: scan for unmapped secrets.

## version

- `version`: print the CLI version.
