# Sync Resources

This page shows practical save/diff/apply flows, from one resource to larger collections.

## Prerequisites

- `declarest` installed
- an active context (`declarest config current`)
- a reachable resource server (`declarest resource-server check`)

## 1. Confirm context and connectivity

```bash
declarest config current
declarest config check
declarest resource-server check
```

## 2. Save one remote resource into the repository

```bash
declarest resource save /corporations/acme
```

Useful variants:

```bash
# overwrite an existing local resource
declarest resource save /corporations/acme --overwrite

# import and safely handle plaintext secrets during save
declarest resource save /corporations/acme --handle-secrets

# save from explicit payload instead of remote read
cat payload.json | declarest resource save /corporations/acme --payload -
```

## 3. Review local state

```bash
declarest resource get --source repository /corporations/acme
declarest metadata get /corporations/acme
```

## 4. Diff local desired state against remote actual state

```bash
declarest resource diff /corporations/acme
```

Use `-o text` for concise line-oriented output or `-o json|yaml` for automation.

## 5. Apply back to the API

```bash
declarest resource apply /corporations/acme
```

Optional refresh after mutation (useful when the server adds defaults or generated fields):

```bash
declarest resource apply /corporations/acme --refresh-repository
```

## Collection workflows

### Save a collection as individual resources

```bash
declarest resource save /customers/
```

### Save a collection as one file (snapshot/opaque endpoints)

```bash
declarest resource save /customers/ --as-one-resource
```

### Apply/update/create a collection recursively

```bash
declarest resource apply /customers/ --recursive
declarest resource update /customers/ --recursive
declarest resource create /customers/ --recursive
```

## Wildcard save for bulk discovery/import

You can use `_` wildcard segments in `resource save` paths (without explicit payload input) to expand concrete targets through remote collection traversal.

Example:

```bash
declarest resource save /admin/realms/_/clients/_
```

This is useful for initial imports of complex APIs when you already know the logical selector structure.

## Debugging unexpected behavior

If save/apply/diff does not hit the endpoint you expect:

```bash
declarest metadata render /corporations/acme get
declarest metadata render /corporations/acme update
declarest resource explain /corporations/acme
```

See [Inspect and Debug Mappings](ad-hoc.md) and [Advanced Metadata Configuration](advanced-metadata-configuration.md).
