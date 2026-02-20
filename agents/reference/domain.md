# Domain Vocabulary and Invariants

## Purpose
Define shared business language and non-negotiable invariants so behavior remains consistent across modules.

## In Scope
1. Domain terms and meanings.
2. Business invariants and conflict rules.
3. Identity and alias semantics.
4. Source-of-truth model.

## Out of Scope
1. CLI presentation details.
2. Adapter-specific protocol knobs.
3. Build and deployment concerns.

## Normative Rules
1. Local repository state is the source of desired state for apply workflows.
2. Remote server state is the source of observed state for refresh workflows.
3. Logical resource paths MUST be normalized absolute paths.
4. Resource identity MUST be derived by explicit metadata rules before fallback heuristics.
5. Alias resolution MUST be deterministic within a collection scope.
6. Metadata directives MUST be applied consistently for get/create/update/delete/list/compare operations.
7. The reserved segment `_` MUST be treated as metadata namespace and not a resource identifier.

## Data Contracts
Domain entities:
1. `Resource`: desired or observed payload.
2. `ResourceMetadata`: operation and transform directives.
3. `resource.Resource`: identity, paths, metadata, and payload bundle.
4. `orchestrator.DefaultOrchestrator`: active runtime managers and configuration identity.

Key terms:
1. Logical Path: canonical repository path for a resource.
2. Collection Path: parent path segment representing a collection.
3. Alias: human-friendly stable key used for local path selection.
4. Remote ID: server-facing identifier used in operation paths.
5. Template Context: data scope used to render metadata templates.

## Business Rules
1. Collection metadata applies by inheritance only where explicitly allowed.
2. Resource-level metadata overrides collection-level metadata.
3. Array fields in metadata are replace, not deep-merge.
4. Compare behavior MUST ignore fields declared by metadata suppression/filter rules.
5. Non-unique alias in the same collection is a conflict and MUST be surfaced.

## Failure Modes
1. Alias collision causing ambiguous target resolution.
2. Missing metadata fields required to build remote paths.
3. Divergent local and remote identity causing unintended updates.
4. Unsupported path segments violating normalization rules.

## Edge Cases
1. Resource exists remotely but local alias changed.
2. Metadata wildcard applies to nested descendants with partial overrides.
3. Resource payload contains fields with both secret and non-secret siblings.
4. Collection has zero items and metadata inference still required.

## Examples
1. Local path `/customers/acme` maps to collection `/customers`, alias `acme`, remote ID from `idFromAttribute` if configured.
2. Metadata on `/customers/_` sets default `operationInfo.getResource.path`; resource metadata on `/customers/acme` overrides only `operationInfo.updateResource.path`.
3. Diff operation suppresses `/updatedAt` and `/lastSeen` before comparison to avoid false drift.
