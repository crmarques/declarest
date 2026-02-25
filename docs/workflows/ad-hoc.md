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

This is the fastest way to verify custom `collectionPath`, relative `path`, methods, headers, and query behavior.

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

## 6. Check resource-server auth/connectivity directly

```bash
declarest resource-server check
declarest resource-server get base-url
declarest resource-server get token-url
```

If OAuth2 is configured:

```bash
declarest resource-server get access-token
```

## 7. Add debug output when needed

Global debug output can help surface repository/resource-server context around failures:

```bash
declarest -d resource get /corporations/acme
```

## Common diagnosis checklist

- Wrong endpoint: check `resourceInfo.collectionPath` and operation `path`
- Wrong ID/alias segment: check `idFromAttribute` and `aliasFromAttribute`
- List returns extra objects: add `listCollection.payload.jqExpression`
- Diff noise: tune `compareResources` suppress/filter rules
- Write payload rejected: inspect `createResource`/`updateResource` payload transforms
