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

- `resource get`: fetch from remote or repo.
- `resource list`: list repo or remote paths.
- `resource add`: add a local resource from a file, another resource path, or an OpenAPI schema (supports overrides and optional remote apply).
- `resource create`: create a remote resource from the repo.
- `resource update`: update a remote resource from the repo.
- `resource apply`: create or update a remote resource.
- `resource diff`: show differences between repo and remote.
- `resource delete`: delete resources from repo, remote, or both.

## metadata

Manage metadata definitions.

- `metadata get`: render effective metadata.
- `metadata set`: set an attribute.
- `metadata unset`: unset an attribute.
- `metadata add`: add metadata from a file.
- `metadata update-resources`: rewrite resources based on new metadata rules.
- `metadata infer`: infer resource metadata (id/alias attributes) from the OpenAPI spec (`--spec` overrides the configured spec, `--apply` writes the suggestions, `--id-from`/`--alias-from` force a value).

## secret

Manage secrets stored in the secret store.

- `secret init`: initialize the secret store.
- `secret add`: create or update a secret value.
- `secret get`: read a secret value.
- `secret delete`: remove a secret.
- `secret list`: list keys for a resource.
- `secret export`: write secrets under a path or all resources to CSV (`--path` or `--all`).
- `secret import`: load secrets from a CSV file (`--file`) and use `--force` to override existing values.
- `secret check`: scan for unmapped secrets.

## version

- `version`: print the CLI version.
