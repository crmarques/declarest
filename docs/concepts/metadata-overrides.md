# Metadata Overrides

This page explains how DeclaREST decides which metadata wins when multiple files match a path.

## Resolution order (high level)

DeclaREST resolves metadata deterministically.
The effective metadata for a path is built in this order (later wins):

1. engine defaults
2. ancestor collection metadata layers
3. wildcard matches at each depth
4. literal matches at each depth
5. resource-only metadata (`<logical-path>/metadata.json`)

The result is then validated and used to render operation specs.

## Why this matters

Advanced APIs usually need shared rules plus exceptions:

- a whole subtree shares auth headers or list filters
- one nested collection needs a different delete endpoint
- one specific resource needs a compare suppression tweak

Overrides let you keep shared behavior centralized while making narrow exceptions explicit.

## Example tree

```text
customers/
  _/metadata.json                     # defaults for all customers
  enterprise/
    _/metadata.json                   # overrides for enterprise subtree
    acme/
      metadata.json                   # resource-only overrides for /customers/enterprise/acme
```

Logical path `/customers/enterprise/acme` will resolve with all three layers (plus defaults), in a stable order.

## Wildcard vs literal at the same depth

Wildcard metadata (`_`) is applied before literal metadata at the same depth.
That means a literal path can always refine/override a generic wildcard rule.

Example intent:

- `users/_/roles/_/metadata.json` defines shared role mapping behavior
- `users/admin/roles/_/metadata.json` overrides one rule for the `admin` subtree

## Merge semantics

### Objects merge recursively

Nested objects combine, and deeper keys can override only the specific fields they need.

### Scalars replace

Strings, numbers, booleans, and null-like values replace earlier values.

### Arrays replace (important)

Arrays do **not** deep-merge.
If you set an array in a deeper layer, it replaces the previous array entirely.

This is especially important for:

- `secretInAttributes`
- `payload.suppressAttributes`
- `payload.filterAttributes`

## Explicit empty arrays/maps are meaningful

An empty array (`[]`) or object (`{}`) can intentionally replace inherited values.
That is different from omitting the field.

Use this when you need to clear inherited transforms in a deeper subtree.

## `metadata get` vs `--overrides-only`

Use both views while designing overrides:

```bash
# Full effective behavior (defaults + merged overrides)
declarest metadata get /corporations/acme

# Only what your metadata files currently override
declarest metadata get /corporations/acme --overrides-only
```

Practical use:

- `metadata get` answers: "What will DeclaREST actually do?"
- `--overrides-only` answers: "What custom behavior have I authored?"

## Safe override workflow

1. Start at the highest shared collection (`.../_/metadata.json`).
2. Add only the minimum overrides needed.
3. Render the specific operation to verify behavior.
4. Introduce deeper wildcard/literal/resource overrides only when a concrete exception appears.

```bash
declarest metadata render /corporations/acme update
declarest resource explain /corporations/acme
```

## Common anti-patterns

- Duplicating full metadata blocks at many levels (hard to maintain)
- Using resource-only metadata for behavior that is really subtree-wide
- Forgetting arrays replace, then accidentally dropping inherited values
- Editing metadata without checking rendered operation output
