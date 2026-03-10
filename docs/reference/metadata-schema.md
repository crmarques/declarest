# Metadata schema reference

This page is a field map for the main metadata sections and what they control.

## Top-level sections

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

- Identity problems: check `resource.id` and `resource.alias`.
- Wrong endpoint/method: check `operations.<op>.path` and `method`.
- Wrong payload shape: check the ordered `transforms` pipeline.
- Noisy drift: check `compare.transforms`.
- Secret handling gaps: check `resource.secretAttributes`.

## Related docs

- [Metadata Overview](../concepts/metadata.md)
- [Metadata Overrides](../concepts/metadata-overrides.md)
- [Custom Paths](../concepts/metadata-custom-paths.md)
- [Advanced Metadata Configuration](../workflows/advanced-metadata-configuration.md)
