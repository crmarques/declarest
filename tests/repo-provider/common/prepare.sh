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

init_git_repo() {
    local repo_dir="$1"

    require_cmd git

    if git init -b main "$repo_dir" >/dev/null 2>&1; then
        return 0
    fi

    git init "$repo_dir" >/dev/null 2>&1
    git -C "$repo_dir" checkout -b main >/dev/null 2>&1
}

rm -rf "$DECLAREST_REPO_DIR"
mkdir -p "$DECLAREST_WORK_DIR"
repo_type="$(resolve_repo_type)"

case "$repo_type" in
    git-remote)
        rm -rf "$DECLAREST_REPO_DIR"
        mkdir -p "$DECLAREST_REPO_DIR"
        log_line "Remote repository configured; repo will be synced into $DECLAREST_REPO_DIR"
        ;;
    fs)
        if [[ ! -d "$DECLAREST_TEMPLATE_REPO_DIR" ]]; then
            printf "Template repository not found at %s\n" "$DECLAREST_TEMPLATE_REPO_DIR" >&2
            exit 1
        fi
        cp -R "$DECLAREST_TEMPLATE_REPO_DIR" "$DECLAREST_REPO_DIR"
        log_line "Filesystem repository configured; template repo copied to $DECLAREST_REPO_DIR"
        ;;
    git-local)
        if [[ ! -d "$DECLAREST_TEMPLATE_REPO_DIR" ]]; then
            printf "Template repository not found at %s\n" "$DECLAREST_TEMPLATE_REPO_DIR" >&2
            exit 1
        fi
        mkdir -p "$DECLAREST_REPO_DIR"
        init_git_repo "$DECLAREST_REPO_DIR"
        cp -R "$DECLAREST_TEMPLATE_REPO_DIR"/. "$DECLAREST_REPO_DIR"/
        git -C "$DECLAREST_REPO_DIR" config user.name "Declarest E2E"
        git -C "$DECLAREST_REPO_DIR" config user.email "declarest-e2e@example.com"
        git -C "$DECLAREST_REPO_DIR" add -A
        if ! git -C "$DECLAREST_REPO_DIR" diff --cached --quiet; then
            git -C "$DECLAREST_REPO_DIR" commit -m "Seed template repository" >/dev/null 2>&1
        fi
        log_line "Git local repository configured; template repo seeded into $DECLAREST_REPO_DIR"
        ;;
    *)
        printf "Unknown repo type: %s\n" "$repo_type" >&2
        exit 1
        ;;
esac
