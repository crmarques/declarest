# Use Cases

## Purpose
Capture cross-domain end-to-end scenarios that no single domain file's Examples section owns; per-feature scenarios live in their owner file.

## Scenario Template
Each scenario MUST state: Goal / Inputs / Execution / Expected outputs / Failure expectation. Execution steps MUST map to types in `agents/reference/interfaces.md`. Failure paths MUST name the expected error category. Expected outputs MUST be deterministic. Single-domain scenarios belong in the owner file's Examples; this file keeps only multi-owner flows.

## Normative Rules
1. A scenario MUST appear here only when it exercises behavior owned by two or more domain files; otherwise it MUST live in the single owner's Examples section.
2. Each scenario MUST cross-reference the owning domain files for the rules it exercises; it MUST NOT restate those rules. Identity-template, required-attribute, defaults-merge, and `format:any` invariants are defined in `agents/reference/metadata.md` and `agents/reference/domain.md`; this file only asserts their observable end-to-end effect.

## Scenarios

### Scenario 1: Apply with metadata layering and secret resolution
Owners exercised: `orchestrator.md`, `metadata.md`, `secrets.md`, `managed-service.md`.

Goal: apply one local resource whose payload contains secret placeholders and metadata-defined operation directives.

Inputs:
1. Path `/customers/acme`.
2. Local payload with `{{secret .}}`-mapped attributes (secret key mapping per `secrets.md`).
3. Resolved metadata defining `operations.update` path, compare transforms, and `resource.secretAttributes` (layering per `metadata.md`).

Execution:
1. `orchestrator.Orchestrator` loads the resource and resolves layered metadata.
2. `secrets.SecretProvider` resolves placeholders into request-time values.
3. `managedservice.ManagedServiceClient` builds and executes the update (auth + request construction per `managed-service.md`).
4. Orchestrator returns normalized remote mutation output with no implicit local persistence.

Expected outputs:
1. Remote update succeeds with resolved secret values in the request body only.
2. Local file remains masked; persisted repository content keeps placeholders.
3. Immediate diff reports no drift.

Failure expectation:
1. An unresolved secret key fails with `ValidationError` before any remote request.

### Scenario 2: Remote 404 -> metadata-aware alias/list fallback
Owners exercised: `orchestrator.md`, `managed-service.md`, `metadata.md`.

Goal: fetch a remote resource deterministically when the direct identity path is stale.

Inputs:
1. Path `/customers/acme`.
2. Resolved `operations.get.path` targeting a stale remote identifier.
3. Metadata identity templates `resource.id`/`resource.alias` and a `list-jq` resolver (per `metadata.md` and `managed-service.md`).

Execution:
1. Direct get returns HTTP 404.
2. Orchestrator runs the bounded alias/list fallback (strategy owned by `orchestrator.md`).
3. The matching list candidate updates `resource.Resource` identity fields via metadata identity rules.

Expected outputs:
1. Fetch succeeds and is identical on repeated runs.
2. Subsequent operations target the resolved remote identifier.

Failure expectation:
1. Multiple alias candidates fail with `ConflictError`.
2. No candidate after the bounded fallback fails with `NotFoundError`.

### Scenario 3: Authenticated git webhook -> immediate operator reconcile
Owners exercised: `k8s-operator.md`, `resource-repo.md`.

Goal: trigger an immediate repository refresh from a provider webhook before the poll interval deadline.

Inputs:
1. `ResourceRepository.spec.git.webhook` with provider `gitea` or `gitlab` and `secretRef`.
2. Operator webhook path `/webhooks/repository/<namespace>/<repository>`.
3. Push-event payload whose branch ref matches the repository branch.

Execution:
1. Provider sends a signed/tokenized push payload to the operator endpoint.
2. Operator validates auth headers (`X-Gitea-Signature` or `X-Gitlab-Token`) and event type (webhook receiver owned by `k8s-operator.md`).
3. Operator patches webhook-receipt annotations to enqueue reconcile; repository sync uses git lifecycle rules from `resource-repo.md`.

Expected outputs:
1. Repository reconcile starts before the next poll deadline.
2. `declarest.io/webhook-last-received-at` annotation updates deterministically.

Failure expectation:
1. Invalid signature/token returns an authentication failure with no annotation mutation and no reconcile.

### Scenario 4: E2E manual handoff with prompt-backed credentials
Owners exercised: `e2e.md`, `context-config.md`.

Goal: hand a local stack to the user with credentials deferred to runtime prompts and reused across the shell session, never written as plaintext into the context file.

Inputs:
1. `run-e2e.sh --profile cli-manual`.
2. Catalog credential `shared-login` with prompt-backed `username`/`password` and `persistInSession: true` (catalog schema + `credentialsRef` owned by `context-config.md`).
3. Context components referencing `shared-login` via `credentialsRef`.
4. User evaluated `declarest context session-hook <bash|zsh>` in the current shell.

Execution:
1. Runner validates the stack is local-instantiable, starts components, and emits a temporary context catalog (handoff owned by `e2e.md`).
2. Runner copies the managed-service `repo-template` into the context repository directory and generates setup/reset scripts.
3. Startup resolves the context without prompting because prompt-backed attributes are deferred.
4. The first component needing credentials prompts once and warns about session reuse; values persist only under `XDG_RUNTIME_DIR/declarest/prompt-auth/`.

Expected outputs:
1. Persisted catalog keeps only the prompt-backed definition plus `credentialsRef` placeholders; no plaintext credentials in context blocks.
2. One hooked shell session prompts at most once per prompt-backed attribute; later commands reuse cached values.
3. Sourcing the setup script exports runtime env vars, defines alias `declarest-e2e`, and enables session reuse; the reset script restores prior state and removes the alias.

Failure expectation:
1. A non-interactive command with uncached prompt-backed attributes fails with `ValidationError`.
2. When `XDG_RUNTIME_DIR` is unavailable, no cross-command cache is created and a new session prompts again.

### Scenario 5: Bundle shared metadata + repo-local overlay precedence
Owners exercised: `metadata.md`, `metadata-bundle.md`, `context-config.md`.

Goal: let repository-local metadata refine bundle metadata without mutating the extracted bundle source.

Inputs:
1. Bundle-provided shared metadata `/customers/_/metadata.yaml` with `resource.id: "{{/id}}"` and `resource.format: yaml` (bundle ref/resolution per `metadata-bundle.md`; bundle source selected via context per `context-config.md`).
2. Repository-local overlay `/customers/acme/metadata.yaml` with `resource.alias: "{{/name}}"` and `resource.format: json`.

Execution:
1. Runtime resolves metadata for `/customers/acme` (layering/precedence owned by `metadata.md`).
2. User edits metadata via `resource metadata set`/`edit`.

Expected outputs:
1. Resolved metadata keeps `resource.id` from the shared source and overrides `resource.alias` and `resource.format` from the repo-local overlay.
2. Metadata mutation writes only the repo-local sidecar; the extracted bundle source is unchanged.

Failure expectation:
1. Resolution ignoring the overlay, or mutation rewriting the extracted bundle source, breaches the contract.

### Scenario 6: Descendant-scoped nested secret paths
Owners exercised: `metadata.md`, `managed-service.md`.

Goal: support nested secret folders under one collection selector without slashful resource IDs.

Inputs:
1. Collection metadata `/projects/_/secrets/_/metadata.yaml` with `selector.descendants: true` (descendant selectors owned by `metadata.md`).
2. `resource.remoteCollectionPath: /storage/keys/project/{{/project}}{{/descendantCollectionPath}}`.
3. Relative operation paths `operations.get.path: ./{{/id}}` and `operations.list.path: .` (request building owned by `managed-service.md`).
4. Logical targets `/projects/platform/secrets/db-password`, `/projects/platform/secrets/path/to/db-password`, `/projects/platform/secrets/path/to`.

Execution:
1. Metadata resolution matches the `/projects/_/secrets/_` selector at concrete root `/projects/platform/secrets`.
2. Render scope derives `project`, `descendantPath`, and `descendantCollectionPath` from the matched root, not the full nested suffix.
3. Request building renders relative operation paths against the descendant-aware remote collection path.

Expected outputs:
1. `/projects/platform/secrets/db-password` renders `/storage/keys/project/platform/db-password`.
2. `/projects/platform/secrets/path/to/db-password` renders `/storage/keys/project/platform/path/to/db-password`.
3. `resource list /projects/platform/secrets/path/to` renders `/storage/keys/project/platform/path/to`.
4. `resource.id` stays segment-safe (`db-password`), never `path/to/db-password`.

Failure expectation:
1. Inheriting nested paths without `selector.descendants: true`, deriving bogus fields (e.g. `secret=path`), or requiring slashful `resource.id` breaches the contract.
