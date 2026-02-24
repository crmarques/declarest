# Paths and Selectors

This page is foundational for advanced metadata.

## Logical paths (what users type)

DeclaREST commands use **logical absolute paths**.

Examples:

- `/corporations/acme`
- `/users/user-001`
- `/admin/config-set/config`

Rules:

- must start with `/`
- `/` is the segment separator
- trailing `/` marks a collection path
- `_` is reserved (special meaning in metadata selectors and wildcard saves)

## Resource path vs collection path

- **Resource path**: no trailing slash, targets one object (`/corporations/acme`)
- **Collection path**: trailing slash, targets a list/group (`/customers/`)

This distinction matters for:

- `resource list`
- `metadata render` default operation selection
- metadata file placement under `_/metadata.json`

## Selector paths (metadata-only patterns)

Metadata can target patterns, not just concrete resources.
Selector paths use `_` segments as wildcards in the metadata tree.

Example selector path:

- `/customers/_/addresses/_/`

This means "any address under any customer".

## Metadata file locations

Given a logical path, metadata may exist at two scopes.

### Collection subtree metadata

Applies to the whole subtree under a collection:

- `<collection>/_/metadata.json`

Example:

- `/customers/` collection metadata -> `customers/_/metadata.json`

### Resource-only metadata

Applies only to one resource directory:

- `<logical-path>/metadata.json`

Example:

- `/corporations/acme` resource metadata -> `customers/acme/metadata.json`

## Wildcards in metadata vs wildcards in commands

These are related but different:

- **Metadata selector wildcard (`_`)**: used in metadata file paths to define reusable rules.
- **`resource save` wildcard (`_`)**: can be used in the CLI path to expand remote collections when saving, for example `/customers/_/addresses/_`.

The metadata wildcard defines behavior; the command wildcard expands concrete targets.

## Why selectors matter for non-REST APIs

Selectors let you keep a clean logical hierarchy even when the API endpoint is different.
Later, metadata can remap selector-based logical paths to the real HTTP endpoint using `resourceInfo.collectionPath` and operation overrides.

See [Custom Paths](metadata-custom-paths.md).
