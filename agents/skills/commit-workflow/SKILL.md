---
name: commit-workflow
description: Run the pre-commit handoff so commit guidance and git commands execute only after the user explicitly approves, through the host tool's confirmation UI.
---

# Commit Workflow

Use ONLY when the user explicitly asks for commit help or commit creation after the request work is complete. Never run automatically just because the working tree changed. (Autonomous commits during isolated coding sessions are owned by `agents/skills/worktree-session/SKILL.md` instead.)

## Inspect
1. If the working tree is clean, say no commit is needed and stop.
2. `git status` and `git diff` (plus `git diff --cached` after any staging) to understand the delta.

## Verify
1. Run the repository's standard checks. When the request changed any `.go` file: `gofmt -w` the changed files → `golangci-lint run` (fix every finding) → `go test -race ./...` (or deepest feasible subset). Skip these when no `.go` files changed.
2. If checks fail, fix the issue or pause and ask the user how to proceed.
3. Scan the diff for secrets (keys, tokens, private keys, `.env` values); stop and escalate before staging anything suspicious.
4. If the repo is mid-rebase/merge or on a detached HEAD, describe it and ask before continuing.

## Commit
1. The user asking for commit help is the go-ahead — proceed autonomously, no separate yes/no gate. The git commands below are pre-authorized; run them directly rather than proposing them.
2. Write a single-line Conventional Commit message per `agents/reference/commit-instructions.md`.
3. Run in order: `git status --porcelain` → `git diff --stat` → `git add -A` → `git commit -m "<message>"` → `git show --stat`. Split separable concerns into multiple commits when the diff warrants it.
4. Stop and escalate instead of committing only when a safety check trips (secrets in the diff, mid-rebase/merge or detached HEAD, unexpected large/binary files) or a required verification gate is blocked.
5. Never `git push` unless the user explicitly asks; pushing reaches the remote and always needs a fresh go-ahead.
