#!/usr/bin/env bash

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
SCRIPTS_DIR="$(cd "$SCRIPT_DIR/.." && pwd)"
# shellcheck source=../lib/logging.sh
source "$SCRIPTS_DIR/lib/logging.sh"
# shellcheck source=../lib/shell.sh
source "$SCRIPTS_DIR/lib/shell.sh"
# shellcheck source=../lib/shell.sh
source "$SCRIPTS_DIR/lib/shell.sh"
# shellcheck source=../lib/repo.sh
source "$SCRIPTS_DIR/lib/repo.sh"
# shellcheck source=../lib/cli.sh
source "$SCRIPTS_DIR/lib/cli.sh"
# shellcheck source=../lib/git-auth.sh
source "$SCRIPTS_DIR/lib/git-auth.sh"

repo_type="$(resolve_repo_type)"
if [[ "$repo_type" != "git-remote" ]]; then
    log_line "Repo verification skipped (repo type: ${repo_type:-unknown})"
    exit 0
fi

if [[ -z "${DECLAREST_REPO_REMOTE_URL:-}" ]]; then
    die "Remote repository URL is not configured"
fi
if [[ -z "${DECLAREST_REPO_DIR:-}" || ! -d "$DECLAREST_REPO_DIR" ]]; then
    die "Repository directory not found: ${DECLAREST_REPO_DIR:-}"
fi

require_cmd git

set +e
repo_check_output="$(capture_cli "repo check (pre-push)" repo check)"
repo_check_status=$?
set -e
if [[ $repo_check_status -ne 0 ]]; then
    if grep -q "\[ERROR\] Remote repository sync" <<<"$repo_check_output" && \
        ! grep -q "\[ERROR\] Remote repository access" <<<"$repo_check_output" && \
        ! grep -q "\[ERROR\] Repository access" <<<"$repo_check_output"; then
        log_line "Repo check reported sync mismatch before push (expected)"
    else
        die "Repo check failed unexpectedly; see logs for details"
    fi
fi

run_cli "repo push" repo push
run_cli "repo check (post-push)" repo check

branch="$(git -C "$DECLAREST_REPO_DIR" rev-parse --abbrev-ref HEAD)"
local_head="$(git -C "$DECLAREST_REPO_DIR" rev-parse HEAD)"
remote_head="$(git_ls_remote_head "$branch")"

if [[ -z "$remote_head" ]]; then
    die "Failed to resolve remote head for ${branch}"
fi
if [[ "$remote_head" != "$local_head" ]]; then
    die "Remote head ${remote_head} does not match local ${local_head}"
fi

run_cli "repo refresh" repo refresh
run_cli "repo reset" repo reset --yes

log_block "Repository log" "$(git -C "$DECLAREST_REPO_DIR" log -n 10 --oneline)"
log_line "Remote head: ${remote_head}"
log_line "Remote head verified for ${branch}"

clone_dir="$(resolve_clone_dir)"
if [[ -z "$clone_dir" ]]; then
    log_line "Output git clone dir not configured; skipping clone"
    exit 0
fi
mkdir -p "$(dirname "$clone_dir")"
if ! clone_remote_repo "$clone_dir"; then
    die "Failed to clone remote repository into ${clone_dir}"
fi
log_line "Remote repository cloned into ${clone_dir}"
