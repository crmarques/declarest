# Inspect and Debug Mappings

This page replaces ad-hoc request docs with the current workflow for understanding what DeclaREST is about to do before mutating anything.

## When to use this workflow

Use this when a command fails or hits the wrong endpoint and you need to inspect:

- resolved metadata
- rendered operation path/method/query/headers
- logical path identity mapping (id vs alias)
- repository vs remote payload differences

## 1. Confirm target path and source

```bash
declarest resource list --source repository /customers/
declarest resource list --source remote-server /customers/
declarest resource list --source remote-server /customers/ --output text
```

This catches simple path mistakes before deeper debugging.
Use `--output text` for a concise `alias (id)` view when metadata identity mapping is configured.

## 2. Inspect metadata (effective vs overrides)

```bash
# defaults + merged overrides
declarest metadata get /corporations/acme

# only authored overrides
declarest metadata get /corporations/acme --overrides-only
```

## 3. Render concrete operation specs

```bash
declarest metadata render /corporations/acme get
declarest metadata render /corporations/acme update
declarest metadata render /customers/ list
```

This is the fastest way to verify custom `remoteCollectionPath`, relative `path`, methods, headers, and query behavior.

## 4. Explain the planned resource operation

```bash
declarest resource explain /corporations/acme
```

Use this when you need a higher-level view of the reconciliation plan.

## 5. Inspect payload and metadata together

```bash
declarest resource get /corporations/acme --show-metadata
```

Useful for checking:

- which metadata layer produced the current operation rules
- whether secret attributes are being redacted as expected
- whether the path is being treated as a resource vs collection

## 6. Check managed server auth/connectivity directly

```bash
declarest server check
declarest server get base-url
declarest server get token-url
```

If OAuth2 is configured:

```bash
declarest server get access-token
```

## 7. Add debug output when needed

Global debug output can help surface repository/managed-server context around failures:

```bash
declarest -d resource get /corporations/acme
```

## Common diagnosis checklist

- Wrong endpoint: check `resource.remoteCollectionPath` and operation `path`
- Wrong ID/alias segment: check `resource.id` and `resource.alias`
- List returns extra objects: add `list.payload.jqExpression`
- Diff noise: tune `compare` suppress/filter rules
- Write payload rejected: inspect `create`/`update` payload transforms
