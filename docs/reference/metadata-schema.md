# Metadata schema reference

This page is a field map for the main metadata sections and what they control.

## Top-level sections

### `resourceInfo`

Controls identity and path mapping.

Common fields:

- `idFromAttribute`
- `aliasFromAttribute`
- `collectionPath`
- `secretInAttributes`

Use when path/identity on the API differs from your logical path model.

### `operationsInfo`

Controls operation-specific request behavior.

Common operation keys:

- `getResource`
- `createResource`
- `updateResource`
- `deleteResource`
- `listCollection`
- `compareResources`

Common operation fields:

- `path`
- `httpMethod`
- `query`
- `httpHeaders`
- `body`
- `payloadMutation`
- `validate.requiredAttributes`
- `validate.assertions`
- `validate.schemaRef`

Each `payloadMutation` entry must contain exactly one of:

- `selectAttributes`
- `suppressAttributes`
- `jqExpression`

DeclaREST applies `operationsInfo.defaults.payloadMutation` first and then the operation-specific pipeline.

### `operationsInfo.defaults`

Defines reusable defaults for transforms/compare behavior that operations can inherit.

## Quick field-to-impact map

- Identity problems: check `resourceInfo.idFromAttribute` and `aliasFromAttribute`.
- Wrong endpoint/method: check `operationsInfo.<op>.path` and `httpMethod`.
- Wrong payload shape: check the ordered `payloadMutation` pipeline.
- Noisy drift: check `compareResources.payloadMutation`.
- Secret handling gaps: check `resourceInfo.secretInAttributes`.

## Related docs

- [Metadata Overview](../concepts/metadata.md)
- [Metadata Overrides](../concepts/metadata-overrides.md)
- [Custom Paths](../concepts/metadata-custom-paths.md)
- [Advanced Metadata Configuration](../workflows/advanced-metadata-configuration.md)
