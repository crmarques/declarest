# E2E Component Standard

This document defines the component contract used by `test/e2e/run-e2e.sh`.

## Scope

Use this standard for all component groups:

- `resource-server`
- `git-provider`
- `secret-provider`
- `repo-type`

## Directory Layout

Each component lives at:

```text
test/e2e/components/<type>/<name>/
```

Required files:

```text
component.env
scripts/init.sh
scripts/configure-auth.sh
scripts/context.sh
```

Components MAY also ship `openapi.yaml`. When provided, the runner copies it into the run directory, exposes the run-scoped path via `E2E_COMPONENT_OPENAPI_SPEC`, and lets each component (for example resource servers) surface it inside their generated context so context-aware commands can infer the API surface.

Additional runtime files:

- `compose` runtime components MUST include:
  - `compose.yaml`
  - `scripts/health.sh`
- `native` runtime components MAY omit `compose.yaml` and `scripts/health.sh`.
- Optional hook: `scripts/manual-info.sh`.

## `component.env` Contract

All fields are required unless marked optional.

```bash
COMPONENT_TYPE=resource-server
COMPONENT_NAME=keycloak
COMPONENT_CONTRACT_VERSION=1
SUPPORTED_CONNECTIONS="local remote"
DEFAULT_CONNECTION=local
REQUIRES_DOCKER=true
COMPONENT_RUNTIME_KIND=compose
COMPONENT_DEPENDS_ON=""
DESCRIPTION="Human-readable summary"
# Required only for COMPONENT_TYPE=resource-server:
SUPPORTED_SECURITY_FEATURES="oauth2 mtls"
# Optional only for COMPONENT_TYPE=resource-server:
REQUIRED_SECURITY_FEATURES="oauth2"
```

Field rules:

- `COMPONENT_TYPE`: group key (`resource-server`, `git-provider`, `secret-provider`, `repo-type`).
- `COMPONENT_NAME`: stable selector name used by CLI flags.
- `SUPPORTED_CONNECTIONS`: whitespace-separated values from `local remote`.
- `DEFAULT_CONNECTION`: one value from `SUPPORTED_CONNECTIONS`.
- `REQUIRES_DOCKER`: `true|false`; MUST align with runtime kind:
  - `compose` -> `true`
  - `native` -> `false`
- `COMPONENT_CONTRACT_VERSION`: current supported value is `1`; runner validation rejects missing or unsupported versions.
- `COMPONENT_RUNTIME_KIND`: `native|compose`.
- `COMPONENT_DEPENDS_ON`: whitespace-separated dependency selectors:
  - exact: `<type>:<name>` (for example `repo-type:git`)
  - wildcard by type: `<type>:*` (for example `git-provider:*`)
  - use empty string when no dependencies (`COMPONENT_DEPENDS_ON=""`).
- `DESCRIPTION`: short operator-facing description.
- `SUPPORTED_SECURITY_FEATURES` (`resource-server` only): whitespace-separated subset of `none basic-auth oauth2 custom-header mtls`; MUST include at least one auth-type capability (`none|basic-auth|oauth2|custom-header`).
- `REQUIRED_SECURITY_FEATURES` (`resource-server` optional): whitespace-separated subset of `SUPPORTED_SECURITY_FEATURES`; MAY include at most one auth-type capability because resource-server auth selection is one-of.
- Runner selection uses `--resource-server-auth-type <none|basic|oauth2|custom-header>` for auth-mode selection and `--resource-server-mtls` independently for mTLS.
- Resource-server fixture metadata files (`*/_/metadata.json`) MUST include non-empty `resourceInfo.idFromAttribute` and `resourceInfo.aliasFromAttribute`.

## Hook Contract

Runner-managed hook sequence:

1. `init`
2. `start` (compose runtime only; built-in unless overridden by `scripts/start.sh`)
3. `health` (compose runtime only)
4. `configure-auth`
5. `context`
6. `stop` (built-in unless overridden by `scripts/stop.sh`)

Hook behavior:

- Hooks MUST be executable by `bash` and ShellCheck-friendly.
- Hooks MUST write generated values to `${E2E_COMPONENT_STATE_FILE}`.
- `context` SHOULD write to `${E2E_COMPONENT_CONTEXT_FRAGMENT}`.
- `context` MAY accept output path as `$1`; runner also exports `E2E_COMPONENT_CONTEXT_FRAGMENT`.
- Hooks MUST be idempotent for repeated runs in the same run directory.

## Dependency and Parallelism Model

- The runner builds batches per hook using `COMPONENT_DEPENDS_ON`.
- Components in the same dependency-ready batch run in parallel.
- Components with dependencies run after required components complete for that hook.
- Cycles fail fast with explicit dependency-cycle errors.

## Runtime Environment Exposed to Hooks

Common exported variables:

- `E2E_COMPONENT_KEY`, `E2E_COMPONENT_TYPE`, `E2E_COMPONENT_NAME`
- `E2E_COMPONENT_DIR`, `E2E_COMPONENT_HOOK`
- `E2E_COMPONENT_CONNECTION`
- `E2E_COMPONENT_RUNTIME_KIND`, `E2E_COMPONENT_DEPENDS_ON`
- `E2E_RESOURCE_SERVER_AUTH_TYPE`, `E2E_RESOURCE_SERVER_MTLS`
- `E2E_COMPONENT_STATE_FILE`
- `E2E_COMPONENT_PROJECT_NAME` (compose project when applicable)
- `E2E_COMPONENT_CONTEXT_FRAGMENT`
- `E2E_RUN_DIR`, `E2E_STATE_DIR`, `E2E_LOG_DIR`, `E2E_CONTEXT_DIR`, `E2E_CONTEXT_FILE`

## Onboarding Checklist

1. Copy an existing component from the same group as baseline.
2. Implement required scripts (`init`, `configure-auth`, `context`).
3. Set `COMPONENT_RUNTIME_KIND` and `COMPONENT_DEPENDS_ON` explicitly.
4. Add `compose.yaml` and `health.sh` for `compose` runtime.
5. Add component-specific `cases/main` and `cases/corner` when behavior differs.
6. Run:
  - `bash -n test/e2e/run-e2e.sh test/e2e/lib/*.sh`
  - `./test/e2e/tests/run.sh`
  - `./test/e2e/run-e2e.sh --validate-components`
  - `./test/e2e/run-e2e.sh --list-components`
7. Validate one local and one remote path when both connections are supported.
