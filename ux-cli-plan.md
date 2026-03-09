# DeclaREST CLI & K8S Operator UX Improvement Plan

## Context

A critical UX audit of all user-facing surfaces in the DeclaREST project: CLI commands/flags, metadata file format (wire schema), context configuration files, and K8S Operator CRDs. The audit found naming inconsistencies across surfaces, verbose/confusing field names in metadata files, non-standard CLI flag patterns, and structural issues in CRD schemas. No backward compatibility is required — all changes target a clean break (CLI major version bump + CRD `v1alpha1`).

---

## Category 1: Naming Consistency (Cross-Cutting)

### 1.1 URL Field Casing [High]

CLI config uses `baseUrl`, `tokenUrl`, `httpUrl`, `httpsUrl` while K8S CRD correctly uses `baseURL`, `tokenURL`. Go convention: acronyms are all-caps.

| Surface | Before | After |
|---------|--------|-------|
| CLI config `HTTPServer` | `"baseUrl"` | `"baseURL"` |
| CLI config `OAuth2` | `"tokenUrl"` | `"tokenURL"` |
| CLI config `HTTPProxy` | `"httpUrl"`, `"httpsUrl"` | `"httpURL"`, `"httpsURL"` |

**Files:** `config/types.go` (lines 84, 103-104, 121), all resolve-override paths and error messages

### 1.2 ID Field Casing [High]

CLI config uses `clientId`, `roleId`, `secretId` while K8S CRD correctly uses uppercase ID.

| Before | After |
|--------|-------|
| `"clientId"` | `"clientID"` |
| `"roleId"` | `"roleID"` |
| `"secretId"` | `"secretID"` |

**Files:** `config/types.go` (lines 123, 197-198)

### 1.3 `secretInAttributes` [High]

Wire format uses `secretInAttributes`, Go struct uses `SecretsFromAttributes`. Both are confusing.

| Surface | Before | After |
|---------|--------|-------|
| Wire format (metadata YAML/JSON) | `"secretInAttributes"` | `"secretAttributes"` |
| Go struct + internal JSON tag | `SecretsFromAttributes` / `"secretsFromAttributes"` | `SecretAttributes` / `"secretAttributes"` |

**Files:** `metadata/schema_serialization.go` (line 23), `metadata/types.go` (line 25), `metadata/resource_metadata_helpers.go`, all bundle metadata files

---

## Category 2: Metadata Wire Format

These changes affect the JSON/YAML schema that bundle authors and users write — the most user-facing format.

### 2.1 Operation Field Names [Critical]

Inconsistent naming: `getResource`, `createResource`, `updateResource`, `deleteResource` (verb+Resource), but `listCollection` (verb+Collection) and `compareResources` (verb+Resources plural).

Since these live under `operations` (after 2.2), context is clear — simplify to just the verb.

| Before | After |
|--------|-------|
| `"getResource"` | `"get"` |
| `"createResource"` | `"create"` |
| `"updateResource"` | `"update"` |
| `"deleteResource"` | `"delete"` |
| `"listCollection"` | `"list"` |
| `"compareResources"` | `"compare"` |

**Files:** `metadata/schema_serialization.go` (lines 37-45), all conversion functions, all bundle metadata files

### 2.2 Drop `Info` Suffix from Wrapper Blocks [Critical]

| Before | After |
|--------|-------|
| `"resourceInfo"` | `"resource"` |
| `"operationsInfo"` | `"operations"` |

**Files:** `metadata/schema_serialization.go` (lines 12-15)

### 2.3 `httpMethod` to `method` [Medium]

HTTP is implied by context. The Go struct already uses `Method`.

**Files:** `metadata/schema_serialization.go` (line 74)

### 2.4 `httpHeaders` Array to `headers` Map [High]

Current wire format forces `[{"name":"X-Foo","value":"bar"}]`. Users should write `{"X-Foo":"bar"}` directly. The Go struct already uses `map[string]string`.

| Before | After |
|--------|-------|
| `"httpHeaders": [{"name": "X-Foo", "value": "bar"}]` | `"headers": [{"X-Foo": "bar"}]` |

should support one or a list of headers

**Files:** `metadata/schema_serialization.go` — remove `httpHeaderWire` struct (lines 68-71), `httpHeaderListPointer`, `httpHeaderListToMap`

### 2.5 `idFromAttribute` / `aliasFromAttribute` [Medium]

Drop the `From` preposition.

| Before | After |
|--------|-------|
| `"idFromAttribute"` | `"idAttribute"` |
| `"aliasFromAttribute"` | `"aliasAttribute"` |

**Files:** `metadata/types.go` (lines 20-21), `metadata/schema_serialization.go` (lines 18-19), `metadata/resource_metadata_helpers.go`

### 2.6 `payloadMutation` to `transforms` [Medium]

"Mutation" implies write operations (GraphQL convention). This feature transforms payloads.

| Before | After |
|--------|-------|
| `"payloadMutation"` | `"transforms"` |
| `PayloadMutationStep` | `TransformStep` |

**Files:** `metadata/types.go` (lines 28, 63, 67), `metadata/schema_serialization.go`, `metadata/resource_metadata_helpers.go`, all bundle metadata files

### 2.7 `suppressAttributes` to `excludeAttributes` [Low]

"Suppress" is non-standard. "Exclude" is universally understood.

**Files:** `metadata/types.go` (line 69), `metadata/schema_serialization.go`

---

## Category 3: CLI Flags

### 3.1 Unify `--overwrite` and `--force` [Medium]

`resource save --overwrite` and `resource apply --force` mean the same thing. Standardize on `--force`.

**Files:** `internal/cli/resource/command_save.go` (line 136)

### 3.2 `--as-items` + `--as-one-resource` to `--mode` [Medium]

Replace two mutually exclusive booleans with `--mode items|single|auto`.

**Files:** `internal/cli/resource/command_save.go` (lines 131-132)

### 3.4 Simplify Commit Message Flags [Medium]

| Before | After |
|--------|-------|
| `-m/--message` (appends to default) + `--commit-message` (overrides) | `-m/--message` (overrides, like `git commit -m`) |

**Files:** `internal/cli/resource/local_edit_helpers.go`, `command_save.go`, `command_delete.go`, `command_copy.go`

### 3.5 `--skip-items` to `--exclude` [Low]

Shorter, standard term. Also make repeatable in addition to comma-separated.

**Files:** `internal/cli/resource/command.go`

### 3.6 `--refresh-repository` to `--refresh` [Low]

Context makes it clear what is being refreshed.

**Files:** `internal/cli/resource/command_apply.go`, `command_create.go`, `command_update.go`

### 3.7 `--content-type` Short Forms Only [Low]

Accept only `json`, `yaml`, `xml`, `hcl`, `ini`, `properties`, `text`, `binary` — auto-map to MIME types internally.

**Files:** `internal/cli/cliutil/flags.go`, `internal/cli/cliutil/input.go`

---

## Category 4: CLI Command Structure

### 4.1 `config` to `context` [High]

The command manages contexts (add, delete, rename, use, show, list, current). Its name should reflect that.

| Before | After |
|--------|-------|
| `declarest config list` | `declarest context list` |
| `declarest config use prod` | `declarest context use prod` |
| `declarest config init` | `declarest context init` |
| `declarest config check` | `declarest context check` |

**Files:** `internal/cli/config/command.go` (line 29), `internal/cli/root.go`

### 4.2 `managed-server` to `server` [High]

Verbose hyphenated name. "Server" is sufficient in context.

| Before | After |
|--------|-------|
| `declarest managed-server check` | `declarest server check` |
| `declarest managed-server get base-url` | `declarest server get base-url` |

**Files:** `internal/cli/managedserver/command.go`

---

## Category 5: K8S CRD Schema

### 5.1 SecretStore shortName `ss` to `sst` [Medium]

`ss` conflicts with the Linux `ss` (socket statistics) command.

**Files:** `api/v1alpha1/secretstore_types.go` (line 63)

### 5.2 Remove `provider` Discriminator from SecretStore [Medium]

`spec.provider: vault` is redundant when `spec.vault: {...}` is set. CLI config already infers provider from populated block.

| Before | After |
|--------|-------|
| `spec.provider: vault` + `spec.vault: {...}` | `spec.vault: {...}` (infer from populated block) |

**Files:** `api/v1alpha1/secretstore_types.go` (lines 11-16, 49-53, 81-170)

### 5.3 Vault Auth: Flatten to Nested [High]

CRD flattens all auth methods at the same level. CLI config properly nests them. CRD should match.

| Before (flat) | After (nested) |
|---------------|----------------|
| `auth.tokenRef` | `auth.token.secretRef` |
| `auth.usernameRef` + `auth.passwordRef` + `auth.userpassMount` | `auth.userpass.usernameRef` + `.passwordRef` + `.mount` |
| `auth.appRoleRoleIDRef` + `auth.appRoleSecretIDRef` + `auth.appRoleMount` | `auth.appRole.roleIDRef` + `.secretIDRef` + `.mount` |

**Files:** `api/v1alpha1/secretstore_types.go` (lines 18-26)

### 5.4 `tokenSecretRef` to `tokenRef` [Medium]

ResourceRepository uses `tokenSecretRef`, SecretStore uses `tokenRef`. Standardize.

**Files:** `api/v1alpha1/resourcerepository_types.go` (line 19)

---

## Category 6: Configuration Files

### 6.1 `currentCtx` to `currentContext` [Medium]

Don't abbreviate in config files.

**Files:** `config/types.go` (line 17), `config/context_service.go`

### 6.2 Sync State Casing [Low]

`in_sync` uses snake_case while everything else is camelCase.

| Before | After |
|--------|-------|
| `"in_sync"` | `"inSync"` |

**Files:** `repository/types.go`

---

## Category 7: Output & Errors

### 7.1 `auto` Output Format TTY Detection [Medium]

When `--output auto` is active and stdout is not a TTY, default to JSON for scriptability.

**Files:** `internal/cli/cliutil/output.go`

### 7.2 Error Message Consistency [Low]

Standardize on Go convention: lowercase-start for all error strings. Audit all `fmt.Errorf` and `ValidationError` calls.

---

## Implementation Sequencing

| Phase | Scope | Rationale |
|-------|-------|-----------|
| **1** | Cross-cutting naming (1.1-1.3, 6.1) | Foundation — other changes depend on stable naming |
| **2** | Metadata wire format (2.1-2.8) | Highest user-facing impact for bundle authors |
| **3** | CLI commands + flags (3.1-3.9, 4.1-4.3) | User-facing CLI surface improvements |
| **4** | K8S CRD schema (5.1-5.4) | CRD version bump to `v1alpha1` |
| **5** | Config + output polish (6.2-6.3, 7.1-7.2) | Final polish |

---

## Verification

- Run full test suite after each phase: `go test ./...`
- Update all bundle metadata files (`declarest-bundle-keycloak`, `declarest-bundle-rundeck`) after Phase 2
- Validate CRD generation after Phase 4: `make generate manifests`
- E2E tests with updated CLI flags and config format
- Documentation rebuild: `mkdocs build` to verify all docs reflect new naming
