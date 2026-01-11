# DeclaREST Project Specification (AI-ready)

## 1) Purpose (What to build)
**DeclaREST** is a Go tool that keeps a Git repo (desired state) in sync with remote REST APIs (actual state).

- **Git is the source of truth** for *apply/update/delete* operations.
- Remote servers are the source of truth for *refresh/get/list* operations (unless explicitly stated otherwise).
- Everything is organized by **logical paths** that map to:
  - a directory in the repo
  - an API endpoint on the server
  - optional metadata overrides

### Core user value
1. I can declare resources as files in Git.
2. I can fetch resources from servers into Git deterministically.
3. I can diff desired vs actual.
4. I can reconcile safely and repeatably.

---

## 2) Glossary (Unambiguous)
- **Logical Path**: A normalized path-like identifier, e.g. `/fruits/apples/apple-01`.
- **Resource**: A single remote object stored at `<logical-path>/resource.json`.
- **Collection**: Any directory representing a group of resources, e.g. `/fruits/apples` (directory).
- **Generic Metadata**: Metadata applying to a collection subtree at `<collection>/_/metadata.json`.
- **Resource Metadata**: Resource-specific metadata at `<logical-path>/metadata.json`.
- **Desired State**: Repo content.
- **Actual State**: Server content.
- **Managers/Providers**: Stable interfaces that isolate implementations (see §6).

---

## 3) Repository Conventions (Filesystem = contract)

### 3.1 Normalization rules (MUST)
Logical paths are normalized before use:
- MUST start with `/`
- MUST use `/` as separator
- MUST NOT contain `..` segments
- MUST NOT contain empty segments (no `//`)
- MUST NOT escape repo root (enforced by repository manager)
- `_` is a **reserved directory name** used only for generic metadata directories.

### 3.2 On-disk layout (MUST)
For a resource with logical path `/fruits/apples/apple-01`:

```
/fruits/
  _/metadata.json              # generic metadata for /fruits/**
  apples/
    _/metadata.json            # generic metadata for /fruits/apples/**
    apple-01/
      resource.json            # desired (and/or refreshed) resource payload
      metadata.json            # resource-specific overrides for this resource only
```

### 3.3 Resource payload file (MUST)
- Filename: `resource.json`
- Stored as JSON (pretty formatting allowed, but must be stable/deterministic if tool rewrites it)
- No secrets should be printed to stdout (see §10)

---

## 4) Metadata (Behavior + algorithm)

### 4.1 Layering precedence (MUST)
Metadata is merged in this order (later wins):
1. **Conventions/defaults**
2. Each ancestor collection’s **generic metadata**: `/<a>/<b>/_/metadata.json` for all ancestors on the path
3. **Resource metadata**: `<logical-path>/metadata.json`

Metadata discovery details:
- At every depth, DeclaREST loads both **literal** and **wildcard** (`_`) metadata directories. For a path like `/a/b/c`, variants such as `/a/b/c/metadata.json`, `/a/b/_/metadata.json`, `/a/_/c/metadata.json`, `/a/_/_/metadata.json`, etc. are considered.
- Files are applied in deterministic order: shallower prefixes first; within the same depth, entries with more wildcards are applied before fewer; ties are lexicographic. Later files override earlier ones using the merge rules below.

Example for `/fruits/apples/apple-01`:
1. defaults
2. `/fruits/_/metadata.json`
3. `/fruits/apples/_/metadata.json`
4. `/fruits/apples/apple-01/metadata.json`

### 4.2 Metadata merge rules (MUST)
- JSON objects merge recursively (map keys)
- Scalars overwrite
- Arrays overwrite entirely (no deep merge)

### 4.3 `metadata.json` schema (v1) + defaults for a well-defined REST API (MUST)
Metadata is a JSON object. All fields are optional. Unknown fields MUST be ignored (forward-compatible).

#### 4.3.1 Default metadata for a conventional CRUD REST API
This is the baseline when no metadata file exists (or when fields are missing):

```json
{
  "resourceInfo": {
    "idFromAttribute": "id",
    "aliasFromAttribute": "id",
    "collectionPath": "<one dir level above the the repo path this file is inside>"
  },
  "operationInfo": {
    "getResource": {
      "url": { "path": "./{{.id}}" },
      "httpMethod": "GET",
      "httpHeaders": []
    },
    "createResource": {
      "url": { "path": "." },
      "httpMethod": "POST",
      "httpHeaders": []
    },
    "updateResource": {
      "url": { "path": "./{{.id}}" },
      "httpMethod": "PUT",
      "httpHeaders": []
    },
    "deleteResource": {
      "url": { "path": "./{{.id}}" },
      "httpMethod": "DELETE",
      "httpHeaders": []
    },
    "listCollection": {
      "url": { "path": "." },
      "httpMethod": "GET",
      "httpHeaders": [],
      "jqFilter" : "<jq format>"
    },
    "compareResources": {
      "ignoreAttributes": []
    }
  }
}
```

Default interpretation rules:
- `resourceInfo.collectionPath`:
  - If empty or missing: default to the logical collection path (the resource logical path without the last segment).
  - Example: logical path `/fruits/apples/apple-01` → default collectionPath `/fruits/apples`.
- `operationInfo.<op>.url.path` is **relative** to `collectionPath`:
  - `"."` means the collection endpoint itself.
  - `"./{{.id}}"` means `collectionPath + "/" + <id>`.
- `httpHeaders` default to:
  - `Accept: application/json` for all operations.
  - `Content-Type: application/json` is added automatically for operations whose HTTP verb allows a body (POST/PUT/PATCH/DELETE) when not explicitly provided.
- `operationInfo.listCollection.jqFilter`:
  - Optional jq expression applied to the remote collection response before any alias/id matching or payload processing.
  - If the jq result is a list, each element is treated as an item; non-list values are wrapped into a list.

#### 4.3.2 `resourceInfo` fields (MUST)
```json
{
  "resourceInfo": {
    "idFromAttribute": "id",
    "aliasFromAttribute": "nameOrSlug",
    "collectionPath": "/api/v1/widgets",
    "secretInAttributes": ["secret", "config.bindCredential[0]"]
  }
}
```

- `idFromAttribute` (string):
  - Used when computing **remote resource paths**.
  - If present and the payload contains this attribute, then the value replaces the last path segment (directory name) when building the remote resource identifier.

  Example:
  - Repo path: `/fruits/apples/apple-01`
  - Effective metadata: `"idFromAttribute": "id"`
  - Desired payload (repo `resource.json`): `{ "id": "123", ... }`
  - Remote resource path suffix becomes `123`, so the final remote resource endpoint is:
    - `collectionPath` + `"/123"`
    - e.g. `/fruits/apples/123`

  Fallback:
  - If attribute is missing or empty in the payload, then DeclaREST MUST fall back to the directory name (`apple-01`) as the id.

- `aliasFromAttribute` (string):
  - Used when mapping **remote resources back to repo paths** (refresh/list).
  - If present and the remote payload contains this attribute, then its value is used as the directory name in the repo.

  Example:
  - Remote path: `/fruits/apples/123`
  - Remote payload: `{ "bla": "apple-01", ... }`
  - Effective metadata: `"aliasFromAttribute": "bla"`
  - Repo logical path becomes `/fruits/apples/apple-01`

  Fallback:
  - If attribute is missing or empty in the payload, DeclaREST MUST fall back to the remote id (the last segment, `123`).

- `collectionPath` (string):
  - Absolute collection endpoint on the server, e.g. `/admin/realms/publico/clients`.
  - If omitted/empty, it is derived from the logical collection path (see defaults above).
  - MAY include Go-template placeholders (`{{...}}`) that reference JSON paths on the resources that make up the current logical path. Resolution rules:

- `secretInAttributes` (string[]):
  - List of JSON attribute paths to treat as secrets (dot paths with optional list indexes like `config.bindCredential[0]`).
  - When printing resources without `--with-secrets`, these paths are rendered as `{{secret .}}`.
  - If a secret value is stored using `{{secret 'key'}}`, that key overrides the default path-based key.
  - An explicit empty list clears inherited secret paths.
    1. Build a template context by loading ancestor resources (root → parent → target). Use the repo payload if present; if missing, fetch the remote resource first. Later entries overwrite earlier ones in the context.
    2. Each placeholder (`{{.foo}}`) looks up `.foo` as a dotted JSON path within the current context; if missing/empty, fall back to the literal directory segment.
    3. Render the template to produce the collection path; `idFromAttribute` still controls the final resource id segment, `aliasFromAttribute` still controls repo directory names.
  - Example (from `tests/sample/expected.md`):
    - Metadata: `/xxx/_/metadata.json` sets `idFromAttribute=bla`, `aliasFromAttribute=ble`; `/xxx/_/yyy/_/metadata.json` sets `idFromAttribute=bli`; `/xxx/_/yyy/_/zzz/_/metadata.json` sets `idFromAttribute=blu`, `aliasFromAttribute=blua`, `collectionPath="/xxx/{{.ble}}/yyy/{{.blu}}/zzz/"`.
    - Resources: `/xxx/xxx-01` has `{ "bla": "blaXXX", "ble": "xxx-01" }`; `/xxx/xxx-01/yyy/yyy-01` has `{ "bli": "bliYYY", "blo": "yyy-01" }`; `/xxx/xxx-01/yyy/yyy-01/zzz/zzz-01` has `{ "blu": "bluZZZ", "blua": "zzz-01" }`.
    - Expected remote paths:
      - `/xxx/xxx-01` → `/xxx/blaXXX`
      - `/xxx/xxx-01/yyy/yyy-01` → `/xxx/blaXXX/yyy/bliYYY`
      - `/xxx/xxx-01/yyy/yyy-01/zzz/zzz-01` → `/xxx/blaXXX/yyy/bluZZZ/zzz/zzz-01`

#### 4.3.3 `operationInfo` fields (MUST)
Each operation defines how to call the server.

```json
{
  "operationInfo": {
    "getResource": {
      "url": { "path": "./{{.id}}" },
      "httpMethod": "GET",
      "httpHeaders": [{ "name": "Accept", "value": "application/json" }]
    }
  }
}
```

- Supported operations (v1):
  - `getResource`, `createResource`, `updateResource`, `deleteResource`, `listCollection`, `compareResources`
- `url.path` (string):
  - A path relative to `collectionPath`.
  - Supports simple Go-template style substitutions using `{{.id}}` and `{{.alias}}`:
    - `.id` is the computed remote id (see `idFromAttribute`)
    - `.alias` is the computed repo directory name (see `aliasFromAttribute`)
  - `{{.id}}` and `{{.alias}}` are resolved at request time from the resource data (resource repository or remote), so alias fallback can still target the correct remote id.
- `url.queryStrings` (string[]):
  - Each entry is `key=value`. If no `=`, the value is empty.
  - Values are rendered as templates with the metadata context; repeated keys are preserved.
- `httpMethod`:
  - Must be a standard verb string (e.g., GET/POST/PUT/PATCH/DELETE).
- `httpHeaders`:
  - List of `"Name: value"` strings OR `{ "name": "...", "value": "..." }` entries.
  - Values MAY use the same substitutions as `url.path` (e.g., `"{{.id}}"`).
  - Header names are canonicalized when sent.
  - If `httpHeaders` omits `Accept`, DeclaREST defaults to `Accept: application/json`.
  - If `httpHeaders` omits `Content-Type` and the HTTP method allows a body (POST/PUT/PATCH/DELETE), DeclaREST defaults to `Content-Type: application/json`.
- `payload`:
  - `filterAttributes` (string[]): dot paths to keep; all other attributes are removed.
  - `suppressAttributes` (string[]): dot paths to remove.
  - `jqExpression` (string): jq transform applied after filter/suppress.
  - Applied to outbound payloads for create/update and to fetched payloads for get/list before saving/printing. For collections, it applies per item.
- `compareResources`:
  - `ignoreAttributes` (string[]): top-level attribute names removed from both payloads before compare/diff.
  - `suppressAttributes` (string[]): dot paths removed from both payloads.
  - `filterAttributes` (string[]): dot paths kept in both payloads.
  - `jqExpression` (string): jq transform applied after ignore/filter/suppress.

### 4.4 Metadata provider responsibilities (MUST)
Given a logical path, it must return the **effective metadata** after layering + merge.
- **Context assembly**: Walk ancestors from root → target, loading `resource.json` when present (resource repository or target file). Later resources overwrite earlier keys in the template context.
- **Default `id`/`alias`**: After assembling the context, `id` and `alias` default to the last path segment if not already set. If `resourceInfo.idFromAttribute` / `aliasFromAttribute` resolve in the context, they also populate `id` / `alias`.
- **Template rendering**: `resourceInfo.collectionPath`, `url.queryStrings`, `httpHeaders` values, and `jqFilter` are rendered as Go templates using the context above. `operationInfo.<op>.url.path` (and any `httpHeaders` values that still contain `{{.id}}`/`{{.alias}}`) keep those placeholders for request-time resolution.
- **Relative placeholders**: Placeholders like `{{../.foo}}` or `{{../../.id}}` move up the repo path (`../` per level), load that ancestor’s resource, and read the referenced attribute before rendering. Works anywhere templates are rendered (including collectionPath, query strings, httpHeaders, and jqFilter).
- **Collection filtering**: When `listCollection.jqFilter` is set, the jq expression is applied to the fetched collection before alias/id matching or payload transforms.

### 4.5 Metadata resolution examples

| Repo path (logical) | Metadata files considered (layered) | Key metadata definitions | Computed remote path |
| --- | --- | --- | --- |
| `/fruits/apples/apple-01` | `/metadata.json` (defaults), `/fruits/_/metadata.json`, `/fruits/apples/_/metadata.json` | Default `collectionPath=/fruits/apples`, `idFromAttribute=id` with payload `{ "id": "123" }` | `/fruits/apples/123` |
| `/fruits/apples/apple-01` | same as above | Default `idFromAttribute=id`, payload missing `id` | `/fruits/apples/apple-01` |
| `/xxx/xxx-01` | `/xxx/_/metadata.json` sets `idFromAttribute=bla`, `aliasFromAttribute=ble` | Payload `{ "bla": "blaXXX", "ble": "xxx-01" }` | `/xxx/blaXXX` |
| `/xxx/xxx-01/yyy/yyy-01` | `/xxx/_/metadata.json`, `/xxx/_/yyy/_/metadata.json` sets `idFromAttribute=bli` | Payload `{ "bli": "bliYYY", "blo": "yyy-01" }` | `/xxx/blaXXX/yyy/bliYYY` |
| `/xxx/xxx-01/yyy/yyy-01/zzz/zzz-01` | `/xxx/_/metadata.json`, `/xxx/_/yyy/_/metadata.json`, `/xxx/_/yyy/_/zzz/_/metadata.json` sets `idFromAttribute=blu`, `aliasFromAttribute=blua`, `collectionPath="/xxx/{{.ble}}/yyy/{{.blu}}/zzz/"` | Payload `{ "blu": "bluZZZ", "blua": "zzz-01" }` | `/xxx/blaXXX/yyy/bluZZZ/zzz/zzz-01` |
| `/admin/realms/publico` | `/metadata.json`, `/admin/_/metadata.json` | `collectionPath` defaults to `/admin/realms`; `idFromAttribute=realm`; payload `{ "realm":"publico" }` | `/admin/realms/publico` |
| `/admin/realms/publico/clients/testA` | `/metadata.json`, `/admin/_/clients/_/metadata.json` | `collectionPath` defaults to `/admin/realms/publico/clients`; `idFromAttribute=id`; payload `{ "id":"c8d5", "clientId":"testA" }` | `/admin/realms/publico/clients/c8d5` |
| `/admin/realms/publico/components/ldap-test/mappers/email` | `/metadata.json`, `/admin/_/components/_/metadata.json`, `/admin/_/components/_/mappers/_/metadata.json` with `collectionPath="/admin/realms/{{.realm}}/components"` | Realm payload `{ "realm":"publico" }`, mapper payload `{ "id":"815e", "name":"email" }` | `/admin/realms/publico/components/815e` |
| `/admin/realms/publico/components/ldap-test/mappers/` (collection) | same as above plus jqFilter `.[] \| select(.providerId == "user-attribute-ldap-mapper")` | JQ filters collection before alias/id resolution | `/admin/realms/publico/components` (collection path) |
| `/foo/bar/baz` with wildcard metadata | `/foo/_/metadata.json` sets `collectionPath="/foo/{{.alias}}/api"`, `aliasFromAttribute=name` | Payload `{ "id":"123", "name":"baz" }` | `/foo/baz/api/123` |
| `/alpha/beta/gamma` with relative placeholder | `/alpha/_/beta/_/metadata.json` uses `collectionPath="/alpha/{{../.id}}/beta"` and ancestor `/alpha/resource.json` has `{ "id":"root-id" }` | Payload `{ "id":"g1" }` | `/alpha/root-id/beta/g1` |
| `/nested/a/b/c` with multiple wildcards | Metadata files: `/nested/_/metadata.json` (`idFromAttribute=id`), `/nested/_/_/metadata.json` (`aliasFromAttribute=alias`), `/nested/_/_/_/metadata.json` (`collectionPath="/api/{{.alias}}"`) | Payload `{ "id":"123", "alias":"c-folder" }` | `/api/c-folder/123` |
| `/collections/items/item-01` list fallback alias | `/collections/_/items/_/metadata.json` (`idFromAttribute=id`, `aliasFromAttribute=name`) | If remote missing literal `/collections/items/item-01`, tool lists `/collections/items`, finds element with `name="item-01"` and `id="abc"`, remote path `/collections/items/abc` | `/collections/items/abc` |
| `/tmpl/example` with relative placeholder | `/tmpl/_/metadata.json` sets `collectionPath="/root/{{../../.id}}/child"`; ancestor `/root/resource.json` has `{ "id":"root-01" }` | Payload `{ "id":"123" }` | `/root/root-01/child/123` |
| `/list/filter` using jqFilter | `/list/_/metadata.json` sets `listCollection.jqFilter="[.[] \| select(.kind==\"keep\")]"` | Server returns `[{id:1,kind:"drop"},{id:2,kind:"keep"}]`; filtered items only include `{id:2}` | `/list` (collection path), saved item alias from `idFromAttribute` |

Notes:
- Wildcard `_` metadata applies to any segment at its position and is merged with literal metadata following precedence described in section 4.1.
- Relative placeholders `{{../.attr}}` walk up one level of the repo path, load that ancestor resource, and pull `attr` into the template context before rendering.
- `idFromAttribute` controls the remote id segment; `aliasFromAttribute` controls repo directory names and alias-based lookups for list fallbacks.
- `collectionPath` is rendered with the template context; `operationInfo.<op>.url.path` is relative to that rendered collection path.

#### 4.5.1 Operation metadata examples

##### Templated headers + query strings
Metadata:
```json
{
  "resourceInfo": {
    "aliasFromAttribute": "name"
  },
  "operationInfo": {
    "getResource": {
      "url": { "path": "./{{.id}}", "queryStrings": ["trace={{.alias}}"] },
      "httpHeaders": ["X-Alias: {{.alias}}"]
    }
  }
}
```
For `/items/foo` with payload `{ "id": "123", "name": "foo" }`, the request uses:
- path `/items/123`
- query `trace=foo`
- header `X-Alias: foo`

##### Payload transforms
Metadata:
```json
{
  "operationInfo": {
    "getResource": {
      "payload": {
        "filterAttributes": ["id", "nested.keep"],
        "suppressAttributes": ["secret"],
        "jqExpression": "."
      }
    }
  }
}
```
The fetched payload keeps only `id` and `nested.keep`, removes `secret`, then applies the jq expression.

##### Compare rules
Metadata:
```json
{
  "operationInfo": {
    "compareResources": {
      "ignoreAttributes": ["status"],
      "suppressAttributes": ["meta.updatedAt"],
      "filterAttributes": ["id", "name"],
      "jqExpression": "."
    }
  }
}
```
Diff ignores `status`, removes `meta.updatedAt`, keeps `id/name`, then applies the jq expression before comparison.

---

## 5) Sync semantics (What each CLI command means)
All operations MUST go through `Reconciler`. CLI never calls managers/providers directly.

### 5.1 `resource get --path <logical-path>`
- Fetch the resource from the remote server (default) or from the repo with `--repo`.
- `--print` writes the payload to stdout; `--save` persists the remote payload in the resource repository.
- When the logical path is a collection, `--save` writes each item as a separate resource; use `--save-as-one-resource` to save the collection payload as a single resource.
- Secrets are masked unless `--with-secrets`. When reading from the repo, `--with-secrets` resolves placeholders using the secret store. Saving plaintext secrets requires `--force`.

### 5.2 `resource list [--path <collection-logical-path>]`
- Lists logical paths within a collection from the repo (default) or remote with `--remote`.
- Defaults:
  - `--repo` is true by default.
  - `--remote` is false by default.
  - If `--remote` is set and `--repo` is not explicitly set, repo listing is disabled.
- If `--path` is omitted:
  - `--repo` lists all resource repository paths.
  - `--remote` lists resources by enumerating collection paths from the resource repository and unioning the remote results.
- If `--remote` is used without `--path` and the repo has no collection metadata, the result may be empty.
- Output is sorted and concise; explicitly setting both `--repo` and `--remote` is an error.

### 5.3 `resource diff --path <logical-path>`
- Compare desired (`resource.json` in repo) vs actual (server).
- Apply metadata compare rules before diff (ignore/suppress/filter/jq).
- Diff uses JSON Patch internally but prints a concise human-readable summary (one line per operation).

### 5.4 `resource create/update/delete/apply`
- `create`: create remote resource from repo payload (fail if exists unless API says otherwise)
- `update`: update remote resource from repo payload (fail if missing unless API says otherwise)
- `delete`: delete resource repository entries by default; add `--remote` to delete remote resources too (confirmation required unless `--yes` is set).
- `apply`: idempotent reconcile:
  - if missing → create
  - if present and differs → update
  - if equal → no-op with a clear message
- `--all` applies the operation to every resource path in the resource repository; `--all` cannot be combined with `--path`.
- `--sync` (create/update/apply): after each remote operation, fetch the remote resource and save it in the resource repository.
- `delete` defaults to repository entries; set `--remote` for remote deletes. Use `--repo=false` if you want remote-only deletion.
- `--yes` skips confirmation prompts (useful for non-interactive scripts).
- `--all` deletes repository entries by default, and deletes remote resources when `--remote` is set.

> If the server API supports PUT vs PATCH nuances, the effective metadata decides which verb/pattern to use.

### 5.5 `metadata get/edit/set/unset/add/update-resources`
- `get`: render the effective metadata after layering and template rendering; output JSON to stdout.
- `edit`: open metadata in the configured editor with defaults prefilled, then strip default values before saving.
- `set`: update a metadata attribute in the local metadata file for the logical path.
- `unset`: remove a metadata attribute/value from the local metadata file for the logical path (when `--value` is provided, remove only that value; otherwise delete the attribute).
- `add`: write metadata from a JSON file to the local metadata file for the logical path.
- `update-resources`: re-save resources for the logical path using current metadata (including alias moves) based on local repository data only.
- `infer`: examine the OpenAPI spec for the target resource/collection, propose `resourceInfo.idFromAttribute` and `resourceInfo.aliasFromAttribute`, and optionally write the suggested attributes with `--apply` (`--spec` overrides the configured OpenAPI spec and `--id-from`/`--alias-from` force a specific value).
- Collection paths (trailing `/`) target generic metadata under `<collection>/_/metadata.json`.
- Metadata paths default to collections; omit the trailing `/` and DeclaREST still treats the path as a collection unless `--for-resource-only` is set.
- Metadata paths may include `_` segments to define wildcard collections (for example, `/admin/realms/_/clients/`).

Inference does not write metadata unless `--apply` is set; the command prints structured reasoning so operators can review the proposed attribute names before persisting them.

### 5.6 `repo init/refresh/push/reset`
- `init`: ensure the repository root exists (Git repo is created lazily on Git operations).
- `refresh`: fetch and fast-forward the current/configured branch; fails on divergence or working tree changes.
- `push`: push the current/configured branch to the remote. `--force` requires confirmation unless `--yes` is set.
- `reset`: fetch and hard-reset the current/configured branch to the remote; requires confirmation unless `--yes` is set.
- Filesystem repositories return errors for refresh/push/reset.

### 5.7 `secret list [--path <logical-path>]`
- Default output groups keys under each resource path:
  ```
  /path:
    key1
    key2
  ```
- `--paths-only` lists only resource paths (one per line).
- `--show-secrets` includes secret values as `key:value`.
- `--paths-only` and `--show-secrets` are mutually exclusive.

### 5.8 `secret check [--path <logical-path>] [--fix]`
- Scans local resources for likely secret fields that are not mapped in `resourceInfo.secretInAttributes`.
- When `--fix` is set, DeclaREST adds missing secret paths to metadata and rewrites resources with `{{secret .}}` placeholders, storing values in the configured secret store.
- If no secret store is configured, `--fix` aborts with a guidance message.

---

## 6) Architecture & Ownership (Stable contracts)
**Do not change interface shape without explicit design approval.**

### Layers (MUST remain)
| Package | Responsibility |
| --- | --- |
| `cli/cmd` | Cobra commands. Parses args/flags, prints output. Calls only `Reconciler`. |
| `internal/reconciler` | Orchestrates compare/diff/sync using metadata + managers. |
| `internal/managedserver` | Resource server contracts + HTTP implementation. |
| `internal/repository` | Repo contracts + Git/FS implementation. |
| `internal/metadata` | Metadata loading, layering, access. |
| `internal/context` | Context discovery/config wiring (ContextManager). |
| `internal/resource` | Pure helpers for JSON handling, patching, diffing. |

### Hard boundaries (MUST)
- Repository I/O only via `ResourceRepositoryManager`
- Server I/O only via `ResourceServerManager`
- Metadata lookups only via `MetadataProvider`
- Context lifecycle only via `ContextManager`
- All orchestration only via `Reconciler`

---

## 7) Contexts (Configuration)
A “context” defines how to connect managers/providers (repo root, server base URL, auth, etc.).

Requirements:
- Commands obtain a reconciler via `loadDefaultReconciler` (or equivalent).
- Context commands manage contexts:
  - `config add` (runs the interactive flow when no config path is provided)
  - `config update`
  - `config delete`
  - `config use`
  - `config rename`
  - `config list`
  - `config current`
  - `config check`
  - `config print-template`

Repository configuration highlights:
- `repository.git.local.base_dir` sets the repo root.
- `repository.git.remote.url` and `repository.git.remote.branch` override the remote endpoint and branch used for repo operations.
- `repository.git.remote.provider` may be `github` or `gitlab` to tune token auth defaults.
- `repository.git.remote.auto_sync` controls automatic pushes after local repository changes (defaults to true).
- `repository.git.remote.auth` supports `basic_auth` (username/password), `access_key` (token; uses provider-specific basic auth when provider is set, otherwise bearer token), or `ssh` (user, private_key_file, passphrase, known_hosts_file, insecure_ignore_host_key).
- `repository.git.remote.tls.insecure_skip_verify` controls TLS verification for Git over HTTPS.
- `repository.filesystem.base_dir` uses a non-Git filesystem repository.

If the repository already has a context format, **do not invent a new one**. Prefer minimal additions.

---

## 8) CLI rules (AI should follow exactly)
- Binary name: `declarest` (outputs refer to tool name as **DeclaREST**)
- Output: US English, concise, self-explanatory
- Success messages include action + affected logical paths
- Global flags: `-h`, `--help`, `--no-status`
- `--no-status` suppresses status messages and prints only command output
- If user runs a command with missing required args/flags:
  - **Do not print an error**; print help/usage for that command
- Positional args:
  - Commands that accept `--path` also accept the path as the first positional argument.
  - `resource add <path> <file>` and `metadata add <path> <file>` accept positional file paths.
  - `secret get|add|delete` accept `<path> <key> [value]`, and `secret list [path]` accepts an optional path.
- Command groups and order (MUST):
  - `resource`: `get`, `create`, `update`, `apply`, `delete`, `diff`, `list`
  - `metadata`: `get`, `set`, `unset`, `add`, `update-resources`
  - `repo`: `init`, `refresh`, `push`, `reset`, `check`
  - `secret`: `init`, `get`, `add`, `delete`, `list`, `check`
  - `config`: as listed in §7

---

## 9) Error handling & quality gates
- Wrap root causes with `%w`
- User-facing errors must explain what failed (path, operation, server/repo side)
- `go test ./...` must pass
- New logic must include tests OR (if impossible) a short documented validation note in the PR description (prefer tests)
- Always run `gofmt -w .` after editing/creating any Go file

---

## 10) Security invariants (MUST)
- Never allow path traversal outside configured repo root
- Never print credentials/tokens/secrets unless explicitly requested (e.g. `--with-secrets`, `secret get`, or `secret list --show-secrets`)
- Prefer safe defaults:
  - destructive git actions require explicit user intent (flags/confirmation already defined by current CLI behavior)
- HTTP client must respect TLS settings defined by context (do not silently disable TLS verification)
- External dependencies must be widely used, well-maintained, and reputable; avoid obscure or unmaintained packages.

---

## 11) Implementation checklist (What AI should do when modifying code)
1. Find existing interfaces for: `Reconciler`, `ResourceRepositoryManager`, `ResourceServerManager`, `MetadataProvider`, `ContextManager`
2. Do **not** change interface signatures
3. Ensure metadata layering (§4.1) is honored wherever endpoints/IDs/compare rules are used
4. Ensure CLI talks only to `Reconciler`
5. Add/adjust tests to keep `go test ./...` green
6. Remove unused vars/params; reuse duplicate logic; keep code simple
7. Keep formatting and whitespace conventions consistent (one blank line between logical blocks)
