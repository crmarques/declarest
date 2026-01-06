#!/usr/bin/env bash

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
export DECLAREST_TEST_DIR="$(cd "$SCRIPT_DIR/../.." && pwd)"

if [[ -z "${DECLAREST_RUN_ID:-}" ]]; then
    if [[ -n "${RUN_ID:-}" ]]; then
        export DECLAREST_RUN_ID="$RUN_ID"
    else
        export DECLAREST_RUN_ID="$(date +%Y%m%dT%H%M%S)"
    fi
fi

if [[ -z "${DECLAREST_WORK_DIR:-}" ]]; then
    export DECLAREST_WORK_BASE_DIR="${DECLAREST_WORK_BASE_DIR:-/tmp}"
    export DECLAREST_WORK_DIR="${DECLAREST_WORK_BASE_DIR%/}/declarest-rundeck-${DECLAREST_RUN_ID}"
fi

export DECLAREST_BIN_DIR="$DECLAREST_WORK_DIR/bin"
export DECLAREST_HOME_DIR="$DECLAREST_WORK_DIR/home"
export DECLAREST_REPO_DIR="$DECLAREST_WORK_DIR/repo"
export DECLAREST_CONTEXT_FILE="$DECLAREST_WORK_DIR/context.yaml"
export DECLAREST_LOG_DIR="$DECLAREST_WORK_DIR/logs"
export DECLAREST_RUNDECK_ENV_FILE="$DECLAREST_WORK_DIR/rundeck.env"
export DECLAREST_RESOURCE_FORMAT="${DECLAREST_RESOURCE_FORMAT:-json}"
export DECLAREST_OPENAPI_SPEC="${DECLAREST_OPENAPI_SPEC:-$DECLAREST_TEST_DIR/templates/openapi.yaml}"

export CONTAINER_RUNTIME="${CONTAINER_RUNTIME:-podman}"

export RUNDECK_IMAGE="${RUNDECK_IMAGE:-docker.io/rundeck/rundeck:4.14.0}"
export RUNDECK_CONTAINER_NAME="${RUNDECK_CONTAINER_NAME:-declarest-rundeck-${DECLAREST_RUN_ID}}"
export RUNDECK_USER="${RUNDECK_USER:-admin}"
export RUNDECK_PASSWORD="${RUNDECK_PASSWORD:-admin}"
export RUNDECK_AUTH_HEADER="${RUNDECK_AUTH_HEADER:-X-Rundeck-Auth-Token}"
export RUNDECK_TOKEN="${RUNDECK_TOKEN:-}"
export RUNDECK_API_VERSION="${RUNDECK_API_VERSION:-45}"

port_file=""
stored_port=""
if [[ -n "${DECLAREST_WORK_DIR:-}" ]]; then
    port_file="$DECLAREST_WORK_DIR/rundeck-port"
fi
if [[ -n "$port_file" && -f "$port_file" ]]; then
    stored_port="$(tr -d ' \t\r\n' < "$port_file")"
fi

if [[ -n "$stored_port" ]]; then
    export RUNDECK_HTTP_PORT="$stored_port"
elif [[ -n "${RUNDECK_HTTP_PORT:-}" ]]; then
    export RUNDECK_HTTP_PORT
elif [[ -n "${RUNDECK_PORT:-}" ]]; then
    export RUNDECK_HTTP_PORT="$RUNDECK_PORT"
else
    export RUNDECK_HTTP_PORT="4440"
fi

base_url="${RUNDECK_BASE_URL:-}"
if [[ -z "$base_url" || "$base_url" == http://localhost* || "$base_url" == http://127.0.0.1* || "$base_url" == http://0.0.0.0* ]]; then
    export RUNDECK_BASE_URL="http://localhost:${RUNDECK_HTTP_PORT}"
else
    export RUNDECK_BASE_URL="$base_url"
fi

export DECLAREST_PROJECT_NAME="${DECLAREST_PROJECT_NAME:-declarest-e2e-${DECLAREST_RUN_ID}}"
export DECLAREST_JOB_NAME="${DECLAREST_JOB_NAME:-hello-world}"
