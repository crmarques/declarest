# Metadata and API Modeling

> Read [Core Concepts](core-concepts.md) first for a high-level overview of metadata. This page is the full treatment.

Metadata is the translation layer between your logical path model and real API behavior. If your API is clean REST, you may only need minimal metadata. If it is inconsistent, nested, or RPC-ish, metadata becomes the core modeling tool.

## What metadata controls

| Section | Key fields | Purpose |
|---------|-----------|---------|
| **Identity** | `resource.id`, `resource.alias` | How a resource is uniquely identified. `id` is used in API URLs; `alias` names the repo directory. Default: `/id`. |
| **Path mapping** | `resource.remoteCollectionPath` | Maps a logical collection to the actual API endpoint. |
| **Selectors** | `selector.descendants` | Whether this metadata applies to nested subcollections. |
| **Operations** | `operations.get/create/update/delete/list/compare` | Per-operation control of `path`, `method`, `query`, `headers`, `body`. |
| **Transforms** | `operations.*.transforms` | Ordered mutation pipeline: `jqExpression`, `selectAttributes`, `excludeAttributes`. |
| **Compare** | `compare.transforms` | Suppress noisy fields (timestamps, versions) before diffing. |
| **Secrets** | `resource.secretAttributes` | JSON Pointer paths to sensitive fields. |
| **Externalized** | `resource.externalizedAttributes` | Long text fields stored as sidecar files. |
| **Defaults** | `resource.defaults` | Shared field values for compact resource files. |

Identity templates use canonical JSON Pointer placeholders like {% raw %}`{{/id}}`{% endraw %} and {% raw %}`{{/name}}`{% endraw %}. Single-level shorthand like {% raw %}`{{name}}`{% endraw %} also works for one-token lookups.

See [Metadata Schema reference](../reference/metadata-schema.md) for the complete field-by-field reference.

## Metadata files: where they live

Metadata is stored as minimal override files alongside resource payloads:

| Scope | File path | Applies to |
|-------|-----------|-----------|
| Collection subtree | `customers/_/metadata.json` | All resources under `/customers/` |
| Resource-only | `customers/acme/metadata.json` | Only `/customers/acme` |

You usually store only the overrides you need. DeclaREST merges them with built-in defaults at runtime.

## Override resolution order

Effective metadata is built deterministically, later layers winning:

1. **Engine defaults** -- built-in behavior
2. **Ancestor collection layers** -- from root downward
3. **Wildcard matches** at each depth (`_` segments)
4. **Literal matches** at each depth
5. **Resource-only metadata** (`<path>/metadata.json`)

Example tree:

```text
customers/
  _/metadata.json                     # defaults for all customers
  enterprise/
    _/metadata.json                   # overrides for enterprise subtree
    acme/
      metadata.json                   # resource-only overrides for acme
```

`/customers/enterprise/acme` resolves all three layers plus engine defaults, in stable order.

Wildcard metadata (`_`) is applied **before** literal metadata at the same depth, so a literal path can always refine a wildcard rule.

## Merge semantics

- **Objects** merge recursively -- deeper keys override specific fields.
- **Scalars** (strings, numbers, booleans) replace entirely.
- **Arrays replace** (important!) -- a deeper layer's array replaces the inherited array completely. This matters for `secretAttributes`, `excludeAttributes`, and `filterAttributes`.
- **Explicit empty** `[]` or `{}` clears inherited values. This is different from omitting the field.

## Custom path modeling

### `remoteCollectionPath`

Maps a logical collection to the real backend endpoint:

```json
{
  "resource": {
    "remoteCollectionPath": "{% raw %}/admin/realms/{{/realm}}/components{% endraw %}",
    "id": "{% raw %}{{/id}}{% endraw %}",
    "alias": "{% raw %}{{/name}}{% endraw %}"
  }
}
```

This lets `/admin/realms/prod/user-registry/ldap-main` map to Keycloak's `/components` endpoint.

### Relative operation paths

Operation `path` values can be relative to the effective collection path:

| Path | Meaning |
|------|---------|
| `.` | Collection endpoint itself |
| {% raw %}`./{{/id}}`{% endraw %} | Child resource under the collection |
| `./execution` | Nested sub-endpoint |

### Operation path defaults

When omitted, DeclaREST uses safe defaults:

- `create` and `list`: `.`
- `get`, `update`, `delete`, `compare`: {% raw %}`./{{/id}}`{% endraw %}

You only override paths for operations that truly differ.

### Path template context

Templates can resolve values from:

- Current resource payload fields
- Ancestor resource payload fields
- Logical path context (realm, aliases, IDs)

## Descendant-aware selectors

When a logical tree has arbitrary nesting depth under one metadata rule, enable `selector.descendants`:

```json
{
  "selector": { "descendants": true },
  "resource": {
    "id": "{% raw %}{{/name}}{% endraw %}",
    "alias": "{% raw %}{{/name}}{% endraw %}",
    "remoteCollectionPath": "{% raw %}/storage/keys/project/{{/project}}{{/descendantCollectionPath}}{% endraw %}"
  },
  "operations": {
    "list": { "path": "." },
    "get": { "path": "{% raw %}./{{/id}}{% endraw %}" }
  }
}
```

- {% raw %}`{{/descendantCollectionPath}}`{% endraw %} renders the collection suffix below the matched root.
- {% raw %}`{{/descendantPath}}`{% endraw %} renders the full resource suffix.

Example: `/projects/platform/secrets/path/to/db-password` renders against `/storage/keys/project/platform/path/to/db-password`.

## Externalized text attributes

Store long string fields (scripts, policies, certificates) as sidecar files instead of inline:

```json
{
  "resource": {
    "externalizedAttributes": [
      { "path": "/script", "file": "script.sh" },
      { "path": "/sequence/commands/*/script", "file": "script.sh" }
    ]
  }
}
```

On save, the string content is extracted to the sidecar file and replaced with {% raw %}`{{include script.sh}}`{% endraw %}. On apply/diff, the file content is loaded back into the effective payload.

For array-backed fields with `*`, filenames get index suffixes: `script-0.sh`, `script-1.sh`, etc.

## Transform pipelines

Operations support an ordered `transforms` array. Each step runs in sequence:

```json
{
  "operations": {
    "create": {
      "transforms": [
        { "jqExpression": ". | .provider = .providerId" },
        { "excludeAttributes": ["/providerId"] }
      ]
    }
  }
}
```

Available transform types:

- **`jqExpression`** -- arbitrary jq transformation
- **`selectAttributes`** -- keep only these JSON Pointer paths
- **`excludeAttributes`** -- remove these JSON Pointer paths

## Recipes for common API patterns

### Recipe 1: Friendly aliases with opaque IDs

```json
{ "resource": { "id": "{% raw %}{{/id}}{% endraw %}", "alias": "{% raw %}{{/name}}{% endraw %}" } }
```

Repo directory uses `name`; API calls use `id`.

### Recipe 2: Logical collection backed by a different endpoint

Use `remoteCollectionPath` to map `/user-registry/` to `/components`, then filter with list transforms.

### Recipe 3: Create and update use different endpoints

```json
{
  "operations": {
    "create": { "path": "./execution" },
    "update": { "path": "./" }
  }
}
```

### Recipe 4: Payload field names differ by operation

```json
{
  "operations": {
    "create": {
      "transforms": [
        { "jqExpression": ". | .provider = .providerId" },
        { "excludeAttributes": ["/providerId"] }
      ]
    }
  }
}
```

### Recipe 5: Filter mixed-type list responses

```json
{
  "operations": {
    "list": {
      "transforms": [
        { "jqExpression": "[ .[] | select(.type == \"desired\") ]" }
      ]
    }
  }
}
```

### Recipe 6: Suppress diff noise

Use `compare.transforms` to exclude server-generated fields like timestamps, versions, and computed status.

### Recipe 7: Nested resources from a flat backend

Map logical child collections to the backend endpoint and filter list results by parent ID. Use `resource("<logical-path>")` inside list jq to resolve parent data when needed.

## Working with metadata: the edit-verify loop

### Inspect

```bash
# effective metadata (defaults + overrides)
declarest resource metadata get /corporations/acme

# only authored overrides
declarest resource metadata get /corporations/acme --overrides-only
```

### Write

```bash
# set metadata from file
declarest resource metadata set /customers/ --payload customers-metadata.json

# set from stdin
cat metadata.json | declarest resource metadata set /customers/ --payload -

# remove metadata
declarest resource metadata unset /customers/
```

### Verify

Always render operations after every change:

```bash
declarest resource metadata render /corporations/acme get
declarest resource metadata render /corporations/acme create
declarest resource metadata render /corporations/acme update
declarest resource metadata render /customers/ list

declarest resource explain /corporations/acme
```

### Safe workflow

1. Start at the highest shared collection (`_/metadata.json`).
2. Add only the minimum overrides needed.
3. Render operations to verify.
4. Add deeper overrides only when a concrete exception appears.
5. Test with `resource save` / `resource apply` on one resource before scaling.

### Anti-patterns to avoid

- Duplicating full metadata blocks at many levels.
- Using resource-only metadata for subtree-wide behavior.
- Forgetting arrays replace (accidentally dropping inherited values).
- Editing metadata without checking rendered output.

## OpenAPI inference

Use OpenAPI specs as a starting point for metadata:

```bash
# preview inferred metadata
declarest resource metadata infer /customers/

# persist inferred metadata
declarest resource metadata infer /customers/ --apply
```

Inference is a baseline. Advanced APIs almost always need manual overrides afterward.

## Bundles

Bundles are reusable metadata packages for specific API products. Instead of writing metadata from scratch:

```yaml
metadata:
  bundle: keycloak-bundle-1.0.0.tar.gz
  # or
  bundleFile: /path/to/keycloak-bundle-1.0.0.tar.gz
```

A bundle contains pre-built metadata trees, and optionally an OpenAPI spec, that map logical paths to the product's API. Available bundles include Keycloak and Rundeck.

At most one metadata source can be active: `baseDir`, `bundle`, or `bundleFile`.
