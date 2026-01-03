# Resource paths

DeclaREST identifies everything using a **logical path** (sometimes called a “resource path” in the CLI).
The same path is used to:

- Address a resource in commands like `declarest resource get --path ...`
- Locate the corresponding files in the repository
- Render REST endpoints via metadata templates

## Resource vs collection

- **Resource path:** no trailing slash, for a single object (example: `/notes/n-1001`)
- **Collection path:** trailing slash, for a group of resources (example: `/notes/`)

Examples:

```bash
# Single resource
declarest resource get --path /notes/n-1001 --save

# List resources under a collection
declarest resource list --repo --path /notes/
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

- `/notes/n-1001` → `notes/n-1001/resource.json`
- `/notes/` collection metadata → `notes/_/metadata.json`

## Wildcards in metadata paths

Resource paths cannot contain `_`, but metadata paths can.
Using `_` as a path segment lets you apply the same metadata rules to many paths that differ only by IDs.

For example, to apply the same rules to every member under `/teams/platform/members/<id>/roles/`:

- Collection metadata file: `teams/platform/members/_/roles/_/metadata.json`

See [Metadata](metadata.md) for how DeclaREST merges and applies these files.
