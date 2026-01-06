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

require_cmd jq

source_dir=""
target_dir=""

case "$#" in
    1)
        target_dir="$1"
        ;;
    2)
        source_dir="$1"
        target_dir="$2"
        ;;
    *)
        die "Usage: strip-secrets.sh <repo-dir> [dest-dir]"
        ;;
esac

if [[ -n "$source_dir" ]]; then
    if [[ ! -d "$source_dir" ]]; then
        die "Source repository not found: $source_dir"
    fi
    rm -rf "$target_dir"
    mkdir -p "$target_dir"
    cp -R "$source_dir"/. "$target_dir"/
fi

if [[ -z "$target_dir" || ! -d "$target_dir" ]]; then
    die "Repository directory not found: ${target_dir:-}"
fi

placeholder="${DECLAREST_SECRET_PLACEHOLDER_VALUE:-declarest-no-secret}"
escaped_placeholder="$placeholder"
escaped_placeholder="${escaped_placeholder//\\/\\\\}"
escaped_placeholder="${escaped_placeholder//&/\\&}"
escaped_placeholder="${escaped_placeholder//\//\\/}"

removed=0
while IFS= read -r -d '' metadata_file; do
    if jq -e '.resourceInfo.secretInAttributes? // empty' "$metadata_file" >/dev/null 2>&1; then
        tmp_file="$(mktemp)"
        jq 'if .resourceInfo then del(.resourceInfo.secretInAttributes) else . end' "$metadata_file" > "$tmp_file"
        mv "$tmp_file" "$metadata_file"
        removed=$((removed + 1))
    fi
done < <(find "$target_dir" -name metadata.json -print0)

replaced=0
while IFS= read -r -d '' resource_file; do
    if grep -q '{{secret .}}' "$resource_file"; then
        sed -i "s/{{secret .}}/${escaped_placeholder}/g" "$resource_file"
        replaced=$((replaced + 1))
    fi
done < <(find "$target_dir" -name resource.json -print0)

log_line "Strip secrets: removed metadata entries in ${removed} files"
log_line "Strip secrets: replaced placeholders in ${replaced} files"

if command -v git >/dev/null 2>&1 && git -C "$target_dir" rev-parse --is-inside-work-tree >/dev/null 2>&1; then
    if ! git -C "$target_dir" diff --quiet; then
        git -C "$target_dir" config user.name "Declarest E2E"
        git -C "$target_dir" config user.email "declarest-e2e@example.com"
        git -C "$target_dir" add -A
        git -C "$target_dir" commit -m "Strip secret placeholders" >/dev/null 2>&1 || true
        log_line "Strip secrets: committed sanitized repository changes"
    fi
fi
