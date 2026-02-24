---
name: commit-workflow
description: Coordinate the pre-commit handoff so Conventional Commit guidance and all git commands run only after the user explicitly approves them through the host tool’s confirmation UI.
---

# Commit Workflow (Repo Standard)

## Purpose
Ensure the agent inspects the delta, validates it with the repository’s standard checks, summarizes the intent, and asks the user if a git commit should be created before proposing any commit-related commands that rely on the host tool’s approval buttons.

## Trigger
- This skill runs only when the agent’s work produced tracked or untracked changes; when the working tree is already clean, note that no commits are necessary and skip the workflow.
- Begin by running `git status` to confirm which files changed, and use `git diff` (and `git diff --cached` after any staging from earlier clarifications) to understand the delta before summarizing it for the user.

## Pre-commit verification
- The agent MUST identify the repository’s standard verification command(s). Consult `AGENTS.md`, the `Makefile`, or other canonical docs to discover the prescribed suite (for example, `make check`, a specific `make` target, or `go test ./...`); when no explicit command exists, the agent MUST default to `go test ./...`.
- Run the identified verification command(s), observe their outcomes, and if they fail the agent MUST either fix the issue or pause and ask the user how to proceed before making any commit proposals.
- Scan every diff for apparent secrets (API keys, tokens, private keys, `.env` values, etc.) and stop to ask the user before staging or committing anything suspicious.
- Confirm repository state is healthy (not in the middle of a rebase/merge, not on a detached HEAD). If the state is unusual, describe it to the user and request explicit permission before continuing.

## Summary and confirmation
- After verification succeeds, summarize the key files and the high-level intent of the changes. When multiple logical changes exist, describe them so the user can judge separability.
- Use the opportunity to flag a multi-commit possibility when the edits are clearly separable (for example, “These updates span docs versus tooling; should they be separate commits?”).
- Then ask the user, “Do you want me to create a git commit for these changes?” and wait for an explicit yes/no answer. Do not run any commit-related commands or stage files while awaiting that confirmation.

## Approval workflow when the user agrees
- When the user says yes, craft a Conventional Commit message in the format `<type>(optional-scope): <short summary>` (use `feat`, `fix`, `docs`, `chore`, `test`, etc.). Keep each logical change independent so a single message can describe it.
- Once the message is ready, rely on the host tool’s default approval/confirmation UI: propose the following commands in order and require approval before executing each.
  1. `git status --porcelain` (reconfirm the working tree is unchanged since the summary).
  2. `git diff --stat` (review the aggregate diff shape).
  3. `git add -A` (stage everything that belongs in the single commit).
  4. `git commit -m "<message>"` (use the planned Conventional Commit message).
  5. `git show --stat` (share the commit summary).
- Never run these commands automatically; treat each as a proposal that only executes after the user presses the host approval button.

## When the user declines
- If the user says no, do not stage or commit anything. Leave the working tree and the index untouched and ask how they would like to proceed or if they want additional verification.

## Safety rules
- The agent MUST never run `git push` unless the user explicitly asks for it.
- If the diff contains anything that resembles a secret, stop and escalate before staging or committing; suggest redacting or moving secrets to a proper store.
- If the repository is in an unusual state (detached HEAD, rebase/merge/in-flight patch), explain the situation, ask for user direction, and avoid staging/committing until they approve the next steps.

## Output required
- Report the verification commands executed and their results (pass/fail/blocked).
- Summarize the changed files, highlight any notable diffs, and mention any safety concerns the user should resolve before committing.
- Ask, “Should I commit the prepared changes now?” as the final question so the user can provide explicit consent before any commit action occurs.
