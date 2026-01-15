#!/usr/bin/env bash

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
SCRIPTS_DIR="$(cd "$SCRIPT_DIR/.." && pwd)"
source "$SCRIPTS_DIR/lib/env.sh"
source "$SCRIPTS_DIR/lib/logging.sh"

resource_format="$(printf '%s' "${DECLAREST_RESOURCE_FORMAT:-json}" | tr '[:upper:]' '[:lower:]')"
resource_ext="json"
case "$resource_format" in
    json|"")
        resource_ext="json"
        ;;
    yaml|yml)
        resource_ext="yaml"
        ;;
    *)
        printf "Unsupported DECLAREST_RESOURCE_FORMAT: %s\n" "$resource_format" >&2
        exit 1
        ;;
esac

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

project_resource_dir="$DECLAREST_REPO_DIR/projects/$DECLAREST_PROJECT_NAME"
job_resource_dir="$DECLAREST_REPO_DIR/projects/$DECLAREST_PROJECT_NAME/jobs/$DECLAREST_JOB_NAME"

cat > "$project_resource_dir/resource.json" <<EOF
{
  "name": "${DECLAREST_PROJECT_NAME}",
  "description": "Declarest E2E project"
}
EOF

if [[ "$resource_ext" == "yaml" ]]; then
    cat > "$project_resource_dir/resource.yaml" <<EOF
name: "${DECLAREST_PROJECT_NAME}"
description: "Declarest E2E project"
EOF
    rm -f "$project_resource_dir/resource.json"
fi

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

cat > "$job_resource_dir/resource.json" <<EOF
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

if [[ "$resource_ext" == "yaml" ]]; then
    cat > "$job_resource_dir/resource.yaml" <<EOF
name: "${DECLAREST_JOB_NAME}"
project: "${DECLAREST_PROJECT_NAME}"
description: "Hello world job"
loglevel: "INFO"
scheduleEnabled: true
executionEnabled: true
sequence:
  strategy: node-first
  keepgoing: false
  commands:
    - exec: "echo hello world"
EOF
    rm -f "$job_resource_dir/resource.json"
fi

if [[ "$resource_ext" == "yaml" ]]; then
    if [[ ! -f "$project_resource_dir/resource.yaml" || -f "$project_resource_dir/resource.json" ]]; then
        printf "Expected YAML project resource file in %s\n" "$project_resource_dir" >&2
        exit 1
    fi
    if [[ ! -f "$job_resource_dir/resource.yaml" || -f "$job_resource_dir/resource.json" ]]; then
        printf "Expected YAML job resource file in %s\n" "$job_resource_dir" >&2
        exit 1
    fi
else
    if [[ ! -f "$project_resource_dir/resource.json" ]]; then
        printf "Expected JSON project resource file in %s\n" "$project_resource_dir" >&2
        exit 1
    fi
    if [[ ! -f "$job_resource_dir/resource.json" ]]; then
        printf "Expected JSON job resource file in %s\n" "$job_resource_dir" >&2
        exit 1
    fi
fi

log_line "Repository prepared at $DECLAREST_REPO_DIR"
