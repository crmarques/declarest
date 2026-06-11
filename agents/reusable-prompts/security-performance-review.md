# Security & Performance Review

Re-runnable defensive audit of DeclaREST for (1) unsafe/incorrect behavior, (2) vulnerabilities, (3) performance/efficiency risks. Be skeptical, assume hostile input, look for failure modes. Review and plan only — do not implement unless explicitly asked. Tie every finding to a concrete file + symbol.

`agents/reference/quality.md` and `secrets.md` define the security invariants; flag any code that violates them and any gap they miss.

## Model the system first
Entrypoints (CLI, operator manager, repository-webhook server, library API), data flows (untrusted input → parse/validate → workflow → external call → storage → output), trust boundaries (network, filesystem, env vars, secret stores, git remotes, managed-service APIs, OCI registries, OpenAPI/bundle downloads), and privileged operations (file writes, git, OS exec, admin/webhook endpoints, auth flows).

## Security checklist
1. **AuthN/AuthZ** — missing/bypassable auth, confused-deputy, over-broad permissions, insecure defaults (debug, permissive CORS, exposed admin/webhook paths); webhook signature/token verification and event/branch filtering.
2. **Input & injection** — path traversal / unsafe joins / symlink following (repository + metadata + bundle extraction zip-slip), command injection, SSRF (OpenAPI/bundle/proxy URLs, cross-origin fetches), header injection, open redirect, template injection.
3. **Secrets** — secrets in code/config/logs/errors/diff/explain output; safe loading and redaction (`Authorization` + configured custom auth headers); world-readable cache/session files; prompt-auth cache only under `XDG_RUNTIME_DIR`.
4. **Crypto/TLS/tokens** — weak/custom crypto, insecure randomness, JWT/OAuth2 validation, TLS/mTLS verification (no silent skip-verify), plain-HTTP credential transmission warnings.
5. **DoS / resource exhaustion** — missing timeouts, unbounded concurrency/body sizes/uploads, regex/parse bombs, request-throttling correctness, goroutine/connection leaks, missing cancellation/context propagation.
6. **Parsing/deserialization** — strict YAML/JSON decode (unknown-key rejection), schema validation, untrusted archives/URLs.
7. **Supply chain** — dependency risk; CI workflows pin actions to full SHAs, least `GITHUB_TOKEN`, provenance + SBOM attestations for released artifacts/images.

## Performance checklist
1. **Hot paths** — O(n^2) patterns, excess allocations, repeated parse/serialize/render, heavy regex, redundant work across layers.
2. **I/O & network** — needless disk/network round-trips, missing caching (metadata/bundle/OpenAPI/token), N+1 remote calls, loading whole bodies into memory.
3. **Concurrency** — data races, lock contention, long critical sections, unbounded pools, blocking calls in handlers, missing backpressure.
4. **Defaults** — HTTP server/client timeouts, rate limiting / circuit breaking for external dependencies.

## Deliverables
- **A. Executive summary** — top 5 security + top 5 performance risks; overall risk (Low/Med/High) and main attack surfaces.
- **B. Findings (P0/P1/P2)** — category, severity, evidence (file+symbol), impact (exploit/failure scenario), root cause, fix direction, validation (test/benchmark/fuzz/race detector/negative test).
- **C. Plan** — small PR-sized milestones with scope, benefit, risk/mitigation, acceptance criteria.

Be evidence-driven; prefer fixes that shrink attack surface; use standard libraries for any crypto change; avoid heavy new dependencies unless justified.
