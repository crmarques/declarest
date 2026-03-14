# Sync Resources

> This section covers real-world workflows. Make sure you understand [Contexts](../concepts/context.md) and [Resource Files](../concepts/resource.md) from the Concepts section first.

This page shows practical save/diff/apply flows, from one resource to larger collections.

## Prerequisites

- `declarest` installed
- an active context (`declarest context current`)
- a reachable managed server (`declarest server check`)

## 1. Confirm context and connectivity

```bash
declarest context current
declarest context check
declarest server check
```

## 2. Save one remote resource into the repository

```bash
declarest resource save /corporations/acme
```

Useful variants:

```bash
# overwrite an existing local resource
declarest resource save /corporations/acme --force

# import and safely handle plaintext secrets during save
declarest resource save /corporations/acme --secret-attributes

# save from explicit payload instead of remote read
cat payload.json | declarest resource save /corporations/acme --payload -
```

## 3. Review local state

```bash
declarest resource get --source repository /corporations/acme
declarest metadata get /corporations/acme
```

### Keep shared values in metadata-backed defaults

When many sibling resources share the same object fields, infer collection defaults and keep `resource.<ext>` focused on explicit overrides:

```bash
declarest resource defaults infer /corporations/acme
declarest resource defaults infer /corporations/acme --save
declarest resource defaults get /corporations/acme
declarest resource defaults edit /corporations/acme
declarest resource defaults config get /corporations/acme
```

By default, `edit` and `infer --save` store the baseline object in a selector-local `defaults.<ext>` file and wire metadata to it with `{{include defaults.<ext>}}`.

Use `declarest resource defaults infer /corporations/acme --check` in CI or local validation to confirm the current resolved defaults still match what DeclaREST would infer today.

If you want to probe server-added defaults instead of inferring from repository siblings, use managed-server probing explicitly:

```bash
declarest resource defaults infer /corporations/acme --managed-server --yes
declarest resource defaults infer /corporations/acme --managed-server --wait 2s --yes
declarest resource defaults infer /corporations/acme --managed-server --check --yes
```

`--managed-server` creates temporary remote resources and removes them before the command returns, so it intentionally requires `--yes`. Add `--wait <duration|seconds>` when the managed server needs extra time between probe creation and the first readback; bare integers are treated as seconds.

### Print or save only explicit overrides

After metadata defaults exist, you can compact merged payloads back to just the non-default values:

```bash
declarest resource get --source repository /corporations/acme --prune-defaults
declarest resource get /corporations/acme --prune-defaults
declarest resource save /corporations/acme --prune-defaults --force
```

This is useful when you want repository reads and refreshes to preserve the compact split between metadata-managed defaults and `resource.<ext>`.

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
declarest resource apply /corporations/acme --refresh
```

## Collection workflows

### Save a collection as individual resources

```bash
declarest resource save /customers/
```

### Save a collection as one file (snapshot/opaque endpoints)

```bash
declarest resource save /customers/ --mode single
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
