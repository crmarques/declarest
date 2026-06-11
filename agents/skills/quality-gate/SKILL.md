---
name: quality-gate
description: Pick and run the smallest verification set that protects the changed contracts while keeping feedback fast.
---

# Quality Gate

## Workflow
1. Classify change impact: `doc`, `low`, `medium`, or `high`.
2. Run the fastest meaningful checks first; stop on first failure; expand scope only when contracts, orchestration, or security risk require it.
3. When any `.go` file changed, reserve final handoff for: `gofmt -w` on the changed files → `golangci-lint run` (fix every finding) → `go test -race ./...` (or the deepest feasible subset when blocked).
4. Record commands run and any intentional coverage gaps so blockers are reported accurately.
5. Keep a successful standard handoff minimal — no verification detail unless the user asks.

## Impact → scope
1. `doc`: spec/comment/instruction-only, no behavior change → tests optional unless a contract changed. (README, AGENTS.md wording, comment edits.)
2. `low`: pure transforms in one package, no I/O/auth/path-safety → targeted package tests `go test ./<pkg>/...`. (internal refactor, formatting bug.)
3. `medium`: CLI wiring, metadata/repository semantics, or provider-contract change → targeted tests + repository-wide `go test ./...`. (new flag, metadata directive, secret-attribute path, interface method.)
4. `high`: orchestration, auth/secrets, path safety, destructive ops, or E2E harness → repository-wide tests + relevant E2E. (apply/diff logic, auth flow, placeholder resolution, traversal guard, `resource delete`, operator reconcile, E2E contract.)

## Commands
1. Package scope first: `go test ./<package>/...`; regression gate: `go test ./...`.
2. `make check` when format/lint/tests all need reconfirmation.
3. Focused E2E before full profiles: `./test/e2e/run-e2e.sh --profile cli-basic ...` (or `make e2e E2E_FLAGS='...'`).
4. OLM packaging changes: `make verify-bundle`.

## Guardrails
1. Never claim coverage for checks not executed.
2. Security-sensitive and destructive workflows require negative-test evidence.
3. When `.go` files did not change, the Go-specific handoff commands MAY be skipped.
4. A blocked required check replaces the standard one-line handoff with the blocker.
