# Metadata schema reference

This page is a field map for the main metadata sections and what they control.

## Top-level sections

### `selector`

Controls collection-selector behavior.

Common fields:

- `descendants`

Use `selector.descendants: true` when a non-root collection selector must continue applying to deeper descendant collections and resources.
When enabled, metadata templates can also use render-only helpers such as `{% raw %}{{/descendantCollectionPath}}{% endraw %}` and `{% raw %}{{/descendantPath}}{% endraw %}`.

### `resource`

Controls identity and path mapping.

Common fields:

- `id`
- `alias`
- `remoteCollectionPath`
- `secretAttributes`

Use when path/identity on the API differs from your logical path model.
`id` and `alias` accept full identity templates such as `{% raw %}{{/name}} - {{/version}}{% endraw %}` and raw JSON Pointer shorthand such as `/id`.
When omitted, effective metadata defaults both to `/id` for identity resolution.

### `operations`

Controls operation-specific request behavior.

Common operation keys:

- `get`
- `create`
- `update`
- `delete`
- `list`
- `compare`

Common operation fields:

- `path`
- `method`
- `query`
- `headers`
- `body`
- `transforms`
- `validate.requiredAttributes`
- `validate.assertions`
- `validate.schemaRef`

Each `transforms` entry must contain exactly one of:

- `selectAttributes`
- `excludeAttributes`
- `jqExpression`

DeclaREST applies `operations.defaults.transforms` first and then the operation-specific pipeline.

### `operations.defaults`

Defines reusable defaults for transforms/compare behavior that operations can inherit.

## Quick field-to-impact map

- Nested subpaths under one selector: check `selector.descendants` plus descendant helper usage.
- Identity problems: check `resource.id` and `resource.alias`.
- Wrong endpoint/method: check `operations.<op>.path` and `method`.
- Wrong payload shape: check the ordered `transforms` pipeline.
- Noisy drift: check `compare.transforms`.
- Secret handling gaps: check `resource.secretAttributes`.

## Related docs

- [Metadata and API Modeling](../guide/metadata-and-api-modeling.md)
