# Metadata

## Layering precedence (MUST)
Order (later wins):
1) defaults/conventions
2) each ancestor collection's generic metadata
3) resource metadata

Wildcard metadata:
- For a path like `/a/b/c`, variants such as `/a/_/c/metadata.json` are considered.
- Apply deterministic order: shallower prefixes first; within same depth, more wildcards before fewer; ties lexicographic.
- `resourceInfo.idFromAttribute` and `resourceInfo.aliasFromAttribute` do not inherit from ancestors unless set at the current depth or in resource metadata.

## Merge rules (MUST)
- Objects merge recursively.
- Scalars overwrite.
- Arrays overwrite entirely.

## Defaults (CRUD REST API)
- `resourceInfo.idFromAttribute` and `aliasFromAttribute` default to `id`.
- `resourceInfo.collectionPath` defaults to logical collection path.
- Operation defaults: `GET`/`POST`/`PUT`/`DELETE` endpoints derived from `collectionPath`.
- `Accept: application/json` default on all operations.
- `Content-Type: application/json` default for methods with bodies.
- `listCollection.jqFilter` optional; if it returns a list, each element is an item; non-list is wrapped.

## resourceInfo
- `idFromAttribute`: when present in payload, replaces the last path segment for remote id; fallback to directory name.
- `aliasFromAttribute`: when present in payload, used as repo directory name on refresh/list; fallback to remote id.
- `collectionPath`: absolute collection endpoint; may use Go-template placeholders referencing ancestor resources. Render using template context.
- `secretInAttributes`: dot paths to treat as secrets. Empty list clears inherited secrets.

Template context rules:
- Load ancestor resources root -> target; later keys overwrite earlier keys.
- Placeholders like `{{../.foo}}` walk up repo path and load that ancestor resource before rendering.

## operationInfo
Supported ops: `getResource`, `createResource`, `updateResource`, `deleteResource`, `listCollection`, `compareResources`.
- `url.path`: relative to `collectionPath`, supports `{{.id}}` and `{{.alias}}`.
- `url.queryStrings`: list of `key=value` entries; values are templates.
- `httpHeaders`: list of `Name: value` strings or `{name,value}`; values can be templates; header names are canonicalized.
- `payload`: filter/suppress/jqExpression applied to outbound and saved payloads; for list, apply per item.
- `compareResources`: ignore/suppress/filter/jqExpression applied before diff.

## Metadata provider responsibilities (MUST)
- Compute effective metadata with layering and merge.
- Assemble template context from ancestor resources.
- Default `id`/`alias` to last segment if not set; if `idFromAttribute`/`aliasFromAttribute` resolve in context, set them.
- Render templates in `collectionPath`, query strings, headers, and jq filters; keep `{{.id}}`/`{{.alias}}` placeholders for request-time resolution in `url.path`.
- Apply `listCollection.jqFilter` before alias/id matching or payload transforms.
