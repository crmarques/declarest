#!/usr/bin/env bash

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
SCRIPTS_DIR="$(cd "$SCRIPT_DIR/.." && pwd)"
source "$SCRIPTS_DIR/lib/env.sh"
source "$SCRIPTS_DIR/lib/logging.sh"
source "$SCRIPTS_DIR/lib/shell.sh"

if [[ -z "${RUNDECK_TOKEN:-}" ]]; then
    die "RUNDECK_TOKEN is required to render the context file"
fi

yaml_quote() {
    local value="$1"
    value="${value//\\/\\\\}"
    value="${value//\"/\\\"}"
    printf "\"%s\"" "$value"
}

openapi_line=""
if [[ -n "${DECLAREST_OPENAPI_SPEC:-}" ]]; then
    openapi_line=$'    openapi: '"$(yaml_quote "$DECLAREST_OPENAPI_SPEC")"
fi

cat > "$DECLAREST_CONTEXT_FILE" <<EOF
repository:
  resource_format: "$DECLAREST_RESOURCE_FORMAT"
  filesystem:
    base_dir: "$DECLAREST_REPO_DIR"
managed_server:
  http:
    base_url: "$RUNDECK_BASE_URL"
${openapi_line}
    auth:
      custom_header:
        header: "$RUNDECK_AUTH_HEADER"
        token: "$RUNDECK_TOKEN"
EOF

log_line "Context file rendered to $DECLAREST_CONTEXT_FILE"
