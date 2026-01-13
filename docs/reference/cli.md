# CLI reference

Run `declarest --help` for the full command list. This page highlights the main commands and what they do.

## Command order

- The CLI groups commands as specified in `specs/04-cli.md`: `resource`, `metadata`, `repo`, `secret`, `config`. Utility commands such as `ad-hoc`, `version`, and `completion` appear afterward.
- Keeping this order consistent across help output makes it easier to find the command group you expect.

## Global flags

- `--no-status`: status messages (for example `[OK] created remote resource /teams/platform/users/alice`) are printed to stderr by default; this flag suppresses them and leaves only the command output on stdout.
- `--debug[=groups]`: print grouped debug information even when commands succeed. Groups are `network`, `repository`, `resource`, and `all`, which reveals HTTP interactions, repository metadata, and resource payloads as needed.

## resource

Operate on resource definitions.

- `resource get`: fetch from the remote server or the repository (`--repo`). Output is printed by default (`--print`), and secrets are masked unless you add `--with-secrets` (repo reads resolve placeholders using the secret store).
- `resource save`: fetch a remote resource and persist the result in the repository. Use `--force` to override existing definitions or to persist secrets via `--with-secrets`, and add `--as-one-resource` when you want a whole collection response in a single file (`--print` shows the payload).
- `resource add`: add definitions from a file, another resource path, or an OpenAPI schema (`--override`, `--from-openapi`, and optional `--apply` to push immediately).
- `resource explain`: describe metadata/OpenAPI mappings for a logical path, including metadata operations, resolved IDs/aliases, headers, and schema outlines.
- `resource template`: print an OpenAPI-based sample collection payload for the supplied path (requires an OpenAPI spec).
- `resource list`: list repository or remote paths. By default `--repo` is true; add `--remote` to enumerate the server. When `--remote` is used without `--path`, DeclaREST walks the repository collection metadata to drive remote listing.
- `resource create`, `resource update`, `resource apply`: reconcile repository payloads with the remote. `--all` operates on all repository paths, `--sync` re-fetches each remote resource after the operation, and `apply` creates/updates/no-ops in one idempotent command.
- `resource diff`: show repository vs. remote differences, applying metadata compare rules before printing a concise summary.
- `resource delete`: remove repository entries (`--repo`, true by default) and optionally remote resources (`--remote`). Remote-only deletion requires explicit `--repo=false`. On collection paths, add `--resource-list` to delete the collection entry in the repository and `--all-items` to delete every saved item under the collection, both only supported when `--repo` is true. `--yes` skips confirmations and `--all` deletes every resource path.

Path arguments (positional `<path>` or `--path`) now provide context-aware shell completion: remote-focused commands query the server, repo-focused commands use the repository contents, and every completion list also surfaces OpenAPI path templates when available. Static OpenAPI segments come first and collections ending with `/` list the child resources from the chosen origin.

## metadata

Manage metadata definitions for resources and collections.

- `metadata get`: render the effective metadata after layering and template rendering.
- `metadata edit`: open the merged metadata with defaults filled for every attribute (even ones you never defined) so the template shows the full shape, then strip those defaults before saving so the file stays clean (`--editor` overrides the default `vi`). Inline comments describe each attribute and a top note reminds you the CLI ignores those comments before parsing so only overrides remain.
- `metadata set`/`unset`: modify metadata attributes/value pairs (`--value` accepts JSON literals, `resourceInfo.secretInAttributes` accepts comma-separated entries).
- `metadata add`: write metadata from a JSON file (`--file` or positional argument).
- `metadata update-resources`: re-save repository resources using the latest metadata rules (alias moves are tracked in the result).
- `metadata infer`: inspect the OpenAPI spec to suggest `resourceInfo.idFromAttribute`/`aliasFromAttribute` (`--spec` overrides the configured descriptor, `--id-from`/`--alias-from` force values, `--recursively` walks matching collections, and `--apply` writes the suggestions).
- Use `--for-resource-only` on any subcommand to treat a path without a trailing `/` as a resource default.

Metadata commands reuse the shared logical-path completion helper, so `<path>` arguments (or `--path`) suggest the same repository/OpenAPI entries that other commands do. Setting `--attribute resourceInfo.secretInAttributes` also activates tab completion for `--value`, which lists the secret attribute paths detected for the provided logical path.

## repo

Manage the resource repository.

- `repo init`: initialize the local repository (Git repo created lazily) and optionally configure the remote.
- `repo refresh`: fetch and fast-forward the configured branch (fails on divergence or dirty tree).
- `repo push`: push the current/configured branch (`--force` requires confirmation unless `--yes`).
- `repo reset`: hard reset to the remote branch (`--yes` skips confirmation).
- `repo check`: verify repository connectivity and initialization state.

## secret

Manage secrets in the configured secret store.

- `secret init`: initialize the store.
- `secret add|get|delete`: read and write secrets at `<path>` and `<key>`.
- `secret list`: default groups keys by resource path; `--paths-only` shows only resource paths, `--show-secrets` includes values (flags are mutually exclusive).
- `secret export`: write secrets to CSV (`--path` or `--all`).
- `secret import`: load secrets from CSV (`--file` or positional file, use `--force` to override).
- `secret check`: scan resources for unmapped secrets and optionally `--fix` to map them and rewrite resources (requires an enabled secret store).

Secret subcommands reuse the common `<path>` completion helper, so their path suggestions match the repository/OpenAPI-aware entries shown elsewhere in the CLI.

## config

Manage DeclaREST contexts and configuration files.

- `config add`: register a context from a file or run the interactive setup when no file is provided (`--force` overrides existing contexts).
- `config update`: replace an existing context from a configuration file.
- `config edit`: edit a context’s configuration in your editor (`--editor` overrides the default `vi`); it preloads every attribute (filling unspecified values with the defaults) and, if you pass an existing config file, loads that file so you start with what is already defined. Inline comments explain each field and a heading reminds you defaults are stripped and comments are ignored before parsing. After you save, DeclaREST drops the defaults so only your overrides remain, and if you exit without making changes the command reports `[OK] no updates detected`.
- `config use`: set the default context.
- `config rename`: rename a context.
- `config list` / `config current`: show available contexts and the active one.
- `config delete`: remove a context (`--yes` skips confirmation).
- `config check`: validate configuration, repository, managed server, and secret store access.
- `config env`: show DECLAREST_* environment overrides.
- `config print-template`: print a full context configuration file.

## ad-hoc

Send direct HTTP requests to the managed server while still honoring any metadata overrides.

- `ad-hoc get|post|put|patch|delete`: issue the named method. Supply the logical path either as `--path <path>` or the positional argument; metadata headers merge first, OpenAPI headers next, and repeated `--header` values (`Name: value` or `Name=value`) override everything.
- `--default-headers`: reapply the JSON Accept/Content-Type defaults even when metadata clears them.
- `--payload`: inline payload or use `@file` to load from disk.
- Responses are printed to stdout (JSON is formatted like `resource get`), and a `[OK] METHOD PATH STATUS` summary is emitted on stderr unless `--no-status` is set.

## version

- `version`: print the CLI version.

## completion

- `completion <shell>`: emit completion scripts for `bash`, `zsh`, `fish`, or `powershell`; write them to your profile or a shell-specific completion directory to enable tab completion globally.

### Completion alias labels

- When completing a collection path (input ends with `/` and the OpenAPI spec marks it as a collection), entries are shown as `<collection>/<id> (alias)` using `resourceInfo.idFromAttribute` and `resourceInfo.aliasFromAttribute`. If those attributes match, the alias is omitted.
- For other completions, descriptions depend on whether the suggestion uses the alias (description `id`), the ID (`alias`), or both (`alias (id)`).
- Adding metadata for friendly names (e.g., `resourceInfo.aliasFromAttribute`) surfaces readable aliases: `/admin/realms/master/clients/7ee92c11-d70b-44a1-a88d-148c01ba79bd  (clientA)`.
