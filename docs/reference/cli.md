# CLI reference

Run `declarest --help` for the full command list.
This page highlights the main commands and what they do.

## Global flags

- `--no-status`: suppress status messages and print only command output.
- `--debug[=groups]`: always print grouped debugging info (even on success) when this flag is supplied; groups are `network`, `repository`, `resource`, `all`. The `network` group reveals the managed server type/base URL/auth, default headers, and each captured HTTP interaction (method, URL, headers, payload, status, headers, body) so you can inspect OAuth/token exchanges or other requests; `resource` enables extra CLI output during apply/create/etc.

## config

Manage contexts and configuration files.

- `config add`: register a context file. Use `--force` to override an existing context; when run without a config path the command opens a guided terminal UI that prints section headings, hides secret input, requires a managed server block, surfaces numbered defaults, and lets you navigate list prompts with the arrow keys.
- `config update`: update an existing context.
- `config edit`: open a context configuration in your editor with defaults prefilled; when you save, the context is added or updated (`--editor` overrides `$VISUAL`/`$EDITOR`, otherwise `vi` is used).
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
- `resource explain`: describe the metadata/OpenAPI interpretation for a logical path so you can understand the collection path, id/alias attributes, headers, and matching HTTP operations. The output now groups metadata operations separately from the OpenAPI metadata section and prints the request schema structure for each matching method in a readable outline.
- `resource template`: print a sample collection payload generated from the OpenAPI schema for the supplied path (requires the configured OpenAPI spec).
- `resource list`: list repo or remote paths.
- `resource add`: add a local resource from a file, another resource path, or an OpenAPI schema (supports overrides and optional remote apply).
- `resource create`: create a remote resource from the repo.
- `resource update`: update a remote resource from the repo.
- `resource apply`: create or update a remote resource.
- `resource diff`: show differences between repo and remote.
- `resource delete`: delete resources from repo, remote, or both (when `--remote` is set without `--repo`, remote-only deletion is assumed).

Path arguments (the positional `<path>` argument or the `--path` flag) now provide context-aware shell completion: remote-focused commands (like `resource get`/`save`) query the managed server, repo-focused commands (such as `resource apply`/`create`/`update`/`diff`) use the repository contents, and every completion list also surfaces configured OpenAPI path templates when available. When your input matches a static OpenAPI segment, those templates are suggested first; when you end a collection path with `/`, DeclaREST lists the collection items from the chosen origin.

## ad-hoc

Send direct HTTP requests to the managed server while still honoring any metadata files that apply to the provided logical path (they can override URLs, headers, and templated placeholders).

- `ad-hoc get|post|put|patch|delete`: issue the named method. Pass the logical path either as `--path <path>` or as the positional argument; headers merge from metadata first, fall back to any OpenAPI header parameters, and then pick up explicit `--header` overrides.
- `--header`: repeat to add arbitrary headers (`Name: value` or `Name=value` format). Metadata-derived headers are merged first, then user headers override.
- `--default-headers`: re-apply the default `Accept: application/json` / `Content-Type: application/json` values even when metadata explicitly cleared them.
- `--payload`: supply an inline payload or prefix with `@` to load payload bytes from a file (useful for POST/PUT/PATCH/DELETE operations).

The command prints the response body to stdout (JSON responses are formatted like `resource get`) and a `[OK] METHOD PATH STATUS` summary to stderr unless `--no-status` is supplied.

## metadata

Manage metadata definitions.

- `metadata get`: render effective metadata.
- `metadata edit`: open the metadata in your editor with defaults prefilled; when you save, default values are stripped before writing the file (`--editor` overrides `$VISUAL`/`$EDITOR`, otherwise `vi` is used).
- `metadata set`: set an attribute.
- `metadata unset`: unset an attribute.
- `metadata add`: add metadata from a file.
- `metadata update-resources`: rewrite resources based on new metadata rules.
- `metadata infer`: infer resource metadata (id/alias attributes) from the OpenAPI spec (`--spec` overrides the configured spec, `--apply` writes the suggestions, `--id-from`/`--alias-from` force a value, and `--recursively` walks every collection defined under the supplied path). When a collection POST schema doesn't expose the identifying properties, inference also inspects the child resource path parameters (e.g., `/admin/realms/{realm}`) so `/admin/realms/` can still suggest `realm` for `idFromAttribute`/`aliasFromAttribute`, and it now also surfaces OpenAPI header parameters so `operationInfo.*.httpHeaders` can be suggested.

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

## completion

- `completion <shell>`: emit the completion script for `bash`, `zsh`, `fish`, or `powershell`. Pipe the output to `source`, redirect it into a shell-specific completion directory, or write it to your profile so Tab completion is active in every session.

### Completion alias labels

- When completing a collection path (input ends with `/` and the OpenAPI spec indicates a collection), entries are shown as `<collection>/<id> (alias)` using the collection's `resourceInfo.idFromAttribute` and `resourceInfo.aliasFromAttribute`. If those attributes are the same, the description is omitted.
- For other remote-oriented completions, the description still depends on which value is in the path: if the completion value uses the alias, the description is `(id)`; if it uses the id, the description is `alias`; otherwise the description is `alias (id)`. When alias and id match, no description is printed.
- Add metadata so the completion description surfaces friendly names. For example, `/admin/realms/` can use the realm name for both the remote ID and the alias:

```json
{
  "resourceInfo": {
    "idFromAttribute": "realm",
    "aliasFromAttribute": "realm"
  }
}
```

Save that JSON as `admin/realms/metadata.json`, or let the CLI write it for you:

```bash
declarest metadata set --path /admin/realms/ --attribute resourceInfo.aliasFromAttribute --value realm
```

After the metadata is in place, Tab completion for a collection such as `/admin/realms/master/clients/` will emit entries like `/admin/realms/master/clients/7ee92c11-d70b-44a1-a88d-148c01ba79bd  (clientA)` so you get the alias names (`clientA`, `clientB`, …) alongside the IDs. `declarest metadata infer --path /admin/realms/ --apply` now inspects the `{realm}` path parameter to suggest the friendly name for both the ID and alias; add `--alias-from realm` if you want to override that suggestion before applying.
