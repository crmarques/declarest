#!/usr/bin/env bash

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
SCRIPTS_DIR="$(cd "$SCRIPT_DIR/.." && pwd)"
# shellcheck source=../lib/env.sh
source "$SCRIPTS_DIR/lib/env.sh"
# shellcheck source=../lib/logging.sh
source "$SCRIPTS_DIR/lib/logging.sh"
# shellcheck source=../lib/repo.sh
source "$SCRIPTS_DIR/lib/repo.sh"
# shellcheck source=../lib/cli.sh
source "$SCRIPTS_DIR/lib/cli.sh"

context_name="${DECLAREST_CONTEXT_NAME:-keycloak-e2e}"
populate_repo="${DECLAREST_POPULATE_REPO:-1}"

log_line "Registering declarest context (${context_name})"
if ! run_cli "cli add-context" config add-context --name "$context_name" --config "$DECLAREST_CONTEXT_FILE"; then
    run_cli "cli set-context" config set-context --name "$context_name" --config "$DECLAREST_CONTEXT_FILE"
fi
run_cli "cli set-current-context" config set-current-context --name "$context_name"
repo_type="$(resolve_repo_type)"

if [[ "$repo_type" == "git-remote" ]]; then
    log_line "Refreshing repository from remote"
    if ! run_cli "cli repo refresh" repo refresh; then
        if [[ "$populate_repo" == "1" ]]; then
            log_line "Repo refresh failed; falling back to template repository"
            if [[ -n "${DECLAREST_REPO_DIR:-}" ]]; then
                rm -rf "$DECLAREST_REPO_DIR"
            fi
            if [[ -d "${DECLAREST_TEMPLATE_REPO_DIR:-}" ]]; then
                mkdir -p "$DECLAREST_WORK_DIR"
                cp -R "$DECLAREST_TEMPLATE_REPO_DIR" "$DECLAREST_REPO_DIR"
                log_line "Template repo copied to $DECLAREST_REPO_DIR"
            else
                log_line "Template repo missing; leaving repository empty"
            fi
        else
            log_line "Repo refresh failed; leaving repository empty (use --populate-repo to seed templates)"
        fi
    fi
fi
log_line "Declarest context ready"
