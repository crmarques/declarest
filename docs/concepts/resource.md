# Resource

A **resource** is one remote entity (for example a team, user, or permission) represented locally as JSON at
`<logical-path>/resource.json` and addressed in the CLI by its **logical path**.

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

- Resource payload: `<base_repo_dir>/<logical-path>/resource.json`
- Resource metadata (overrides for that resource only): `<base_repo_dir>/<logical-path>/metadata.json`
- Collection subtree metadata (applies to everything under the collection): `<base_repo_dir>/<collection>/_/metadata.json`

Examples:

- `/teams/platform/users/alice` → `teams/platform/users/alice/resource.json`
- `/teams/platform/users/` collection metadata → `teams/platform/users/_/metadata.json`

## Wildcards in metadata paths

Resource paths cannot contain `_`, but metadata paths can.
Using `_` as a path segment lets you apply the same metadata rules to many paths that differ only by IDs.

For example, to apply the same rules to every permission assignment under `/teams/platform/users/<id>/permissions/`:

- Collection metadata file: `teams/platform/users/_/permissions/_/metadata.json`

See [Metadata](metadata.md) for how DeclaREST merges and applies these files.
