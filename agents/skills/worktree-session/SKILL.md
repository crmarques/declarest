---
name: worktree-session
description: Isolate a coding session in its own git branch + worktree so parallel sessions never collide, then rebase onto main, fast-forward main, and clean up. Use the moment you determine a task requires changing code.
---

# Worktree Session

Use the moment you determine a task requires changing code (not for read-only/spec/doc-only work). Each session works in its own branch and git worktree so concurrent sessions never touch the same working tree, then integrates back to `main` and removes its temporary branch and worktree.

This skill is the one sanctioned exception to the no-auto-commit default in `AGENTS.md`: inside a worktree session you commit and merge to `main` autonomously. Never `push` unless the user explicitly asks.

Run all `git` commands with `-C <repo-root>` (the git repo is the `declarest/` directory) so the main checkout's working directory is irrelevant. Let `REPO` be that root and `WT` be the workspace `.worktrees/` directory beside it.

## 1. Open the session
1. Confirm `git -C "$REPO" rev-parse --is-inside-work-tree` succeeds; if not, stop and report (offer `git init`).
2. Derive a unique slug from the task (kebab-case, ≤24 chars) and a branch `agent/<slug>-<shortid>`, where `<shortid>` is e.g. `$(date +%s | tail -c6)`.
3. Refresh the base: `git -C "$REPO" fetch --quiet` when a remote exists; base the branch on the freshest `main` (`origin/main` if present, else local `main`).
4. Create the isolated worktree:
   `git -C "$REPO" worktree add "$WT/<branch>" -b <branch> <base>`
   where `<base>` is `origin/main` (or `main`). Do all edits, builds, and tests under `$WT/<branch>`.

## 2. Implement and commit
1. Make the code changes inside the worktree only.
2. Commit in small, logical units. Subject line MUST follow `agents/reference/commit-instructions.md`: one line, `<type>(<scope>): <description>`, ≤72 chars. Append the `Co-Authored-By` trailer required by the environment; add no other body.
   `git -C "$WT/<branch>" add -A && git -C "$WT/<branch>" commit -m "<type>(<scope>): <description>"`
3. Keep secrets out of commits; scan the diff before committing.

## 3. Verify before integrating
1. Run the verification scope from `agents/skills/quality-gate/SKILL.md` inside the worktree.
2. When any `.go` file changed: `gofmt -w` the changed files, then `golangci-lint run`, fix every finding, then `go test -race ./...` (or the deepest feasible subset). A blocked or failing required gate is a blocker — stop and report instead of merging.

## 4. Rebase, fast-forward, clean up
1. Refresh main: `git -C "$REPO" fetch --quiet` (when remote) and ensure local `main` matches `origin/main`.
2. Rebase the session branch onto the current base, resolving conflicts:
   `git -C "$WT/<branch>" rebase <base>`
3. Fast-forward `main` to the rebased branch from the main checkout:
   `git -C "$REPO" merge --ff-only <branch>`
   (The rebase guarantees a fast-forward; if it is refused, main moved again — re-run step 2.)
4. Remove the worktree and delete the temporary branch:
   `git -C "$REPO" worktree remove "$WT/<branch>"`
   `git -C "$REPO" branch -d <branch>`
5. Confirm cleanup: `git -C "$REPO" worktree list` no longer shows the session path and `git -C "$REPO" branch` no longer lists the branch.

## Safety
1. Never `git push` unless the user explicitly asks.
2. If a rebase conflict cannot be resolved safely, stop and report; leave the worktree intact for inspection rather than force-merging.
3. If the verification gate cannot complete, do not merge — surface the blocker.
4. Leave `main` in a working state; never merge an unverified branch.
