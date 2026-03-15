#!/usr/bin/env bash
set -euo pipefail

CASE_ID='proxy-vault-traffic'
CASE_SCOPE='main'
CASE_REQUIRES='proxy-mode=local secret-provider=vault'

case_run() {
  local log_path="${E2E_RUN_DIR}/proxy/access.log"

  case_run_declarest secret set proxy-vault-check super-secret
  case_expect_success || return 1

  case_run_declarest secret get proxy-vault-check
  case_expect_success || return 1

  case_wait_until 20 1 'proxy log contains vault API traffic' \
    grep -Fq -- '/v1/' "${log_path}"
}
