# Resource

A **resource** is one remote entity (for example a team, user, or permission) represented locally as JSON (or YAML)
at `<logical-path>/resource.json` (or `resource.yaml` when `repository.resource_format` is `yaml`) and addressed in
the CLI by its **logical path**.

DeclaREST identifies everything using a **logical path** (sometimes called a “resource path” in the CLI).
The same path is used to:

- Address a resource in commands like `declarest resource get --path ...`
- Locate the corresponding files in the repository
- Render REST endpoints via metadata templates

## Resource vs collection

- **Resource path:** no trailing slash, for a single object (example: `/teams/platform/users/alice`)
- **Collection path:** trailing slash, for a group of resources (example: `/teams/platform/users/`)

Examples:

```bash
# Single resource
declarest resource get --path /teams/platform/users/alice --save

# List resources under a collection
declarest resource list --repo --path /teams/platform/users/
```

Some commands default to treating paths as collections (for example `declarest metadata ...`).
If you want to edit metadata for a resource directory (not the subtree), use `--for-resource-only`.

## Path rules

- Must start with `/`.
- Uses `/` as a separator.
- Cannot contain `..` or empty segments (`//`).
- The `_` segment name is reserved (used for metadata folders and wildcard matching).

## How paths map to repository files

Given a repository base directory `<base_repo_dir>`:

- Resource payload: `<base_repo_dir>/<logical-path>/resource.json` (or `resource.yaml` when configured)
- Resource metadata (overrides for that resource only): `<base_repo_dir>/<logical-path>/metadata.json`
- Collection subtree metadata (applies to everything under the collection): `<base_repo_dir>/<collection>/_/metadata.json`

Examples:

- `/teams/platform/users/alice` → `teams/platform/users/alice/resource.json`
- `/teams/platform/users/` collection metadata → `teams/platform/users/_/metadata.json`

## Resource payload includes

Resource payload files can inline other files that live alongside the resource definition by setting a value to the literal `{{include <file>}}` directive. The included path is resolved relative to the directory containing the current `resource.json`/`resource.yaml`. When the referenced file contains JSON or YAML content, DeclaREST merges that structured document in place; otherwise, the file is read as raw text so the resulting value remains a valid string (YAML renders it as a block scalar).

Includes can be nested and DeclaREST resolves them before validating the payload, so the final document is always valid JSON/YAML regardless of how many files you compose.

Example:

```yaml
service:
  config: "{{include config.json}}"
  script: "{{include deploy.sh}}"
```

The `config.json` data is merged as a map while `deploy.sh` is imported as a multi-line string.

## Wildcards in metadata paths

Resource paths cannot contain `_`, but metadata paths can.
Using `_` as a path segment lets you apply the same metadata rules to many paths that differ only by IDs.

For example, to apply the same rules to every permission assignment under `/teams/platform/users/<id>/permissions/`:

- Collection metadata file: `teams/platform/users/_/permissions/_/metadata.json`

See [Metadata](metadata.md) for how DeclaREST merges and applies these files.
