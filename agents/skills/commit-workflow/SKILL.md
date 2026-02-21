---
name: commit-workflow
description: Guide agents through the repository's commit expectations and verification steps.
---

# Commit Workflow (Repo Standard)

## Purpose
Create clean, spec-aligned commits that pass hooks/tests and match repository conventions.

## Steps
1. Inspect changes
   - `git status` to confirm touched files.
   - `git diff` (working tree) to understand unstaged edits.
2. Split changes into logical commits
   - Use `git add -p` (or equivalent) to stage hunks per logical change.
   - Do not mix refactors with behavior changes unless clearly related.
3. Validate before each commit
   - Run the repository's standard test command(s) (or at least unit tests when the full suite is slow).
   - Scan diffs for secrets or unexpected large/binary files (`rg --hidden`/`git diff` as needed).
   - Review the staged diff via `git diff --staged` to ensure accuracy.
4. Write a Conventional Commit message
   - Format: `<type>(<scope>): <summary>` with an imperative summary (≤72 characters, no period).
   - Optional body may describe what/why and list breaking changes or follow-ups.
5. Commit and verify
   - Run `git commit -m "..."` once the above checks pass.
   - If a `commit-msg` hook fails, adjust the message and retry.

## Output required
- List commits created (`sha subject`).
- Document which tests/commands were run.
- Share any follow-up recommendations (for example, if the change should be split further).
