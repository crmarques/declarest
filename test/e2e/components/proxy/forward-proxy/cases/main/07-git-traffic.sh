#!/usr/bin/env bash
set -euo pipefail

CASE_ID='proxy-git-traffic'
CASE_SCOPE='main'
CASE_REQUIRES='proxy-mode=local repo-type=git git-provider=gitea'

case_run() {
  local payload_file log_path
  payload_file="${E2E_CASE_TMP_DIR}/proxy-git-payload.json"
  log_path="${E2E_RUN_DIR}/proxy/access.log"

  cat >"${payload_file}" <<'EOF'
{"name":"proxy-git-check"}
EOF

  case_run_declarest resource save /proxy-git/check -f "${payload_file}" -i json --force
  case_expect_success || return 1

  case_run_declarest repository push
  case_expect_success || return 1

  case_wait_until 20 1 'proxy log contains git remote traffic' \
    grep -Fq -- '.git' "${log_path}"
}
