# Metadata Overview

Metadata is the translation layer between your logical path model and the real API behavior.

If your API is perfect REST, you may only need a little metadata.
If your API is inconsistent, nested, RPC-ish, or mixed-type, metadata becomes the core modeling tool.

## What metadata can control

### Resource identity and path mapping

- `resourceInfo.idFromAttribute`
- `resourceInfo.aliasFromAttribute`
- `resourceInfo.collectionPath`
- `resourceInfo.secretInAttributes`

### Operation behavior (`operationInfo.*`)

For each operation (`getResource`, `createResource`, `updateResource`, `deleteResource`, `listCollection`, `compareResources`) you can control:

- `path`
- `httpMethod`
- `query`
- `httpHeaders`
- `body`
- payload transforms (`payload.filterAttributes`, `payload.suppressAttributes`, `payload.jqExpression`)

### Compare/diff normalization

Use `compareResources` transforms to suppress noisy fields before diffing (timestamps, server-generated versions, etc.).

## Metadata files are overrides, not full schema dumps

You usually store only the overrides you need.
DeclaREST applies defaults and merges matching metadata layers at runtime.

Helpful commands:

```bash
# Effective metadata (merged with defaults)
declarest metadata get /corporations/acme

# Minimal override view
declarest metadata get /corporations/acme --overrides-only

# Render one operation to inspect the final HTTP request mapping
declarest metadata render /corporations/acme update
```

## OpenAPI helps, but metadata is the final authority

If an OpenAPI spec is configured, DeclaREST can infer or improve defaults.
But explicit metadata overrides win.

```bash
declarest metadata infer /corporations/acme
declarest metadata infer /corporations/acme --apply
```

Use inference as a starting point, then refine manually for edge cases.

## Advanced topics

- [Metadata Overrides](metadata-overrides.md) for precedence and merge behavior
- [Custom Paths](metadata-custom-paths.md) for non-REST endpoints and logical-path indirection
- [Advanced Metadata Configuration workflow](../workflows/advanced-metadata-configuration.md) for real Keycloak examples
