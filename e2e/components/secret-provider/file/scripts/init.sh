#!/usr/bin/env bash
set -euo pipefail

# shellcheck disable=SC1091
source "${E2E_DIR}/lib/common.sh"

state_file=${E2E_COMPONENT_STATE_FILE}
: >"${state_file}"

secret_dir="${E2E_RUN_DIR}/secrets"
secret_file="${secret_dir}/secrets.enc.json"
secret_passphrase="pass-${RANDOM}${RANDOM}${RANDOM}"

mkdir -p "${secret_dir}"

e2e_write_state_value "${state_file}" SECRET_FILE_PATH "${secret_file}"
e2e_write_state_value "${state_file}" SECRET_FILE_PASSPHRASE "${secret_passphrase}"
