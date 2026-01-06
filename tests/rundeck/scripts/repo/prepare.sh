#!/usr/bin/env bash

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
SCRIPTS_DIR="$(cd "$SCRIPT_DIR/.." && pwd)"
# shellcheck source=../lib/env.sh
source "$SCRIPTS_DIR/lib/env.sh"
# shellcheck source=../lib/logging.sh
source "$SCRIPTS_DIR/lib/logging.sh"

rm -rf "$DECLAREST_REPO_DIR"
mkdir -p "$DECLAREST_REPO_DIR/projects/_" \
    "$DECLAREST_REPO_DIR/projects/_/jobs/_" \
    "$DECLAREST_REPO_DIR/projects/$DECLAREST_PROJECT_NAME/jobs/$DECLAREST_JOB_NAME"

cat > "$DECLAREST_REPO_DIR/projects/_/metadata.json" <<EOF
{
  "resourceInfo": {
    "idFromAttribute": "name",
    "aliasFromAttribute": "name",
    "collectionPath": "/api/${RUNDECK_API_VERSION}/projects"
  },
  "operationInfo": {
    "createResource": {
      "httpMethod": "POST"
    }
  }
}
EOF

cat > "$DECLAREST_REPO_DIR/projects/$DECLAREST_PROJECT_NAME/resource.json" <<EOF
{
  "name": "${DECLAREST_PROJECT_NAME}",
  "description": "Declarest E2E project"
}
EOF

cat > "$DECLAREST_REPO_DIR/projects/_/jobs/_/metadata.json" <<EOF
{
  "resourceInfo": {
    "idFromAttribute": "name",
    "aliasFromAttribute": "name",
    "collectionPath": "/api/${RUNDECK_API_VERSION}/job"
  },
  "operationInfo": {
    "createResource": {
      "httpMethod": "POST",
      "url": {
        "path": "./create"
      }
    }
  }
}
EOF

cat > "$DECLAREST_REPO_DIR/projects/$DECLAREST_PROJECT_NAME/jobs/$DECLAREST_JOB_NAME/resource.json" <<EOF
{
  "name": "${DECLAREST_JOB_NAME}",
  "project": "${DECLAREST_PROJECT_NAME}",
  "description": "Hello world job",
  "loglevel": "INFO",
  "scheduleEnabled": true,
  "executionEnabled": true,
  "sequence": {
    "strategy": "node-first",
    "keepgoing": false,
    "commands": [
      { "exec": "echo hello world" }
    ]
  }
}
EOF

log_line "Repository prepared at $DECLAREST_REPO_DIR"
