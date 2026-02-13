# Contexts and Configuration

## Purpose
Define configuration modeling, context lifecycle, precedence rules, and validation behavior.

## In Scope
1. Context schema and lifecycle operations.
2. Override precedence from defaults, persisted config, environment, and runtime flags.
3. Config normalization and validation.
4. Context wiring into runtime managers.

## Out of Scope
1. Secret payload contents.
2. Transport adapter internals.
3. CLI completion internals.

## Normative Rules
1. Context selection MUST be explicit and deterministic.
2. Config precedence MUST be: runtime flags, environment overrides, persisted context values, engine defaults.
3. Unknown override keys MUST fail validation.
4. Required manager config for enabled capabilities MUST be validated before execution.
5. Optional managers MUST be absent by design, not by partial invalid configuration.
6. Configuration persistence SHOULD store normalized values and omit implicit defaults.

## Data Contracts
Context config fields:
1. `name`.
2. `repository` settings.
3. `metadata` settings.
4. optional `server` settings.
5. optional `secrets` settings.
6. optional CLI/editor preferences.

Context manager operations:
1. `Create/Update/Delete/Rename/List`.
2. `SetCurrent/GetCurrent`.
3. `LoadResolvedConfig`.
4. `Validate`.

## Failure Modes
1. Active context not found.
2. Environment override references unsupported key.
3. Required repository root missing.
4. Server enabled with incomplete auth or URL config.

## Edge Cases
1. Environment override clears optional manager config intentionally.
2. Context rename while currently active.
3. Multiple contexts share same repository root with different server configs.
4. Defaults imply behavior not explicitly persisted.

## Examples
1. Runtime `--context prod` overrides persisted `currentContext`.
2. `DECLAREST_CTX_SERVER_BASE_URL` overrides context server URL during execution only.
3. Validation rejects config with secrets enabled but missing store backend type.
