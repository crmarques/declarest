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

## Confirm
1. Summarize the changed files and intent; flag separable changes that could be multiple commits.
2. Ask "Do you want me to create a git commit for these changes?" and wait for an explicit yes/no. Do not stage or commit while waiting.

## On yes
1. Write a Conventional Commit message per `agents/reference/commit-instructions.md`.
2. Propose these in order, each executed only after the host approval button: `git status --porcelain` → `git diff --stat` → `git add -A` → `git commit -m "<message>"` → `git show --stat`.
3. Never `git push` unless the user explicitly asks.

## On no
Leave the working tree and index untouched; ask how they want to proceed.
