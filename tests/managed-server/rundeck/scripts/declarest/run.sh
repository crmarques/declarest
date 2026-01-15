#!/usr/bin/env bash

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
SCRIPTS_DIR="$(cd "$SCRIPT_DIR/.." && pwd)"
source "$SCRIPTS_DIR/lib/env.sh"
source "$SCRIPTS_DIR/lib/logging.sh"
source "$SCRIPTS_DIR/lib/cli.sh"

run_cli "create project" resource create --path "/projects/$DECLAREST_PROJECT_NAME"
run_cli "create job" resource create --path "/projects/$DECLAREST_PROJECT_NAME/jobs/$DECLAREST_JOB_NAME"

include_job_name="included-job"
include_job_path="/projects/$DECLAREST_PROJECT_NAME/jobs/$include_job_name"
include_workdir="$DECLAREST_WORK_DIR/include-resource"
commands_dir="$include_workdir/commands"

rm -rf "$include_workdir"
mkdir -p "$commands_dir"

cat > "$commands_dir/command.sh" <<'EOF'
echo "included job command"
EOF

cat > "$commands_dir/commands.json" <<'EOF'
[
  { "exec": "{{include command.sh}}" }
]
EOF

cat > "$include_workdir/sequence.json" <<'EOF'
{
  "strategy": "node-first",
  "keepgoing": false,
  "commands": "{{include commands.json}}"
}
EOF

resource_format="${DECLAREST_RESOURCE_FORMAT,,}"
if [[ -z "$resource_format" ]]; then
  resource_format="json"
fi

case "$resource_format" in
  yaml|yml)
    include_resource_file="$include_workdir/resource.yaml"
    cat > "$include_resource_file" <<EOF
name: "$include_job_name"
project: "$DECLAREST_PROJECT_NAME"
description: "Job created via include"
loglevel: INFO
scheduleEnabled: true
executionEnabled: true
sequence: "{{include sequence.json}}"
EOF
    ;;
  *)
    include_resource_file="$include_workdir/resource.json"
    cat > "$include_resource_file" <<EOF
{
  "name": "$include_job_name",
  "project": "$DECLAREST_PROJECT_NAME",
  "description": "Job created via include",
  "loglevel": "INFO",
  "scheduleEnabled": true,
  "executionEnabled": true,
  "sequence": "{{include sequence.json}}"
}
EOF
    ;;
esac

run_cli "add job with includes" resource add --path "$include_job_path" --file "$include_resource_file"
run_cli "apply job with includes" resource apply --path "$include_job_path"

log_line "Rundeck E2E workflow completed"
