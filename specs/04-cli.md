# CLI Semantics

## Global rules
- Binary name: `declarest` (outputs refer to tool name as DeclaREST).
- Output: US English, concise, self-explanatory.
- Success messages include action + affected logical paths.
- Global flags: `-h`, `--help`, `--no-status`.
- If missing required args/flags: do not print an error; print help/usage for that command.
- Commands that accept `--path` also accept the path as the first positional argument.

## Command groups and order (MUST)
- `resource`: `get`, `create`, `update`, `apply`, `delete`, `diff`, `list`
- `metadata`: `get`, `set`, `unset`, `add`, `update-resources`
- `repo`: `init`, `refresh`, `push`, `reset`, `check`
- `secret`: `init`, `get`, `add`, `delete`, `list`, `check`
- `config`: add/update/delete/use/rename/list/current/check/print-template

## Resource commands
- `resource get --path <logical-path>`: fetch remote by default; `--repo` uses repo. `--print` prints payload; `--with-secrets` includes secrets (repo reads resolve placeholders via the secret store).
- `resource save --path <logical-path>`: fetch the remote resource and persist it in the repository; use `--force` to override existing definitions or to include secrets, `--as-one-resource` to store a collection as one file, and `--print` to display the payload.
- `resource list`: lists repo by default; `--remote` lists remote; do not allow both `--repo` and `--remote` true.
- `resource diff`: compare repo vs server; apply compare rules from metadata; print concise human-readable diff.
- `resource create/update/delete/apply`: apply repo payload; `apply` is idempotent create/update/no-op; `--all` applies to all repo resources; `--sync` re-fetches after remote ops; `delete` defaults to repo unless `--remote` (use `--repo=false` for remote-only deletes, `--resource-list` to drop the collection entry, and `--all-items` to remove saved items while `--repo` remains true).

## Metadata commands
- `metadata get/edit/set/unset/add/update-resources/infer`: render effective metadata, edit defaults, or mutate local metadata.
- Collection paths (trailing `/`) target `<collection>/_/metadata.json`.
- Metadata paths default to collections unless `--for-resource-only` is set.
- `infer` only writes metadata with `--apply`; print reasoning.

## Repo commands
- `init`: ensure repo root exists (Git repo created lazily).
- `refresh`: fetch + fast-forward; fails on divergence or dirty tree.
- `push`: push current/configured branch; `--force` requires confirmation unless `--yes`.
- `reset`: fetch + hard-reset; confirmation unless `--yes`.
- Filesystem repos return errors for refresh/push/reset.

## Secret commands
- `secret list`: default groups keys by resource path; `--paths-only` or `--show-secrets` (mutually exclusive).
- `secret check`: scan for secret-like fields; `--fix` adds paths + rewrites resources; fails with guidance if no secret store configured.
