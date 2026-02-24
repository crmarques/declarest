#!/usr/bin/env bash
set -euo pipefail

# shellcheck disable=SC1091
source "$(cd -- "$(dirname "${BASH_SOURCE[0]}")" && pwd)/testkit.sh"

load_profile_libs() {
  source_e2e_libs common profile
}

prepare_manual_env_scripts() {
  local tmp=$1

  export E2E_RUN_ID='manual-env-hook-test'
  export E2E_RUNS_DIR="${tmp}/runs"
  export E2E_RUN_DIR="${E2E_RUNS_DIR}/${E2E_RUN_ID}"
  export E2E_STATE_DIR="${E2E_RUN_DIR}/state"
  export E2E_CONTEXT_FILE="${E2E_RUN_DIR}/contexts.yaml"
  export E2E_BIN="${E2E_RUN_DIR}/bin/declarest"

  mkdir -p "${E2E_STATE_DIR}" "$(dirname -- "${E2E_BIN}")"
  printf '#!/usr/bin/env bash\nexit 0\n' >"${E2E_BIN}"
  chmod +x "${E2E_BIN}"

  SETUP_SCRIPT="${tmp}/setup.sh"
  RESET_SCRIPT="${tmp}/reset.sh"
  e2e_manual_write_env_setup_script "e2e-manual" "${SETUP_SCRIPT}" "${RESET_SCRIPT}" ""
  e2e_manual_write_env_reset_script "${RESET_SCRIPT}"
}

test_manual_env_scripts_install_and_restore_prompt_hook() {
  load_profile_libs

  local tmp
  tmp=$(new_temp_dir)
  trap 'rm -rf "${tmp}"' RETURN

  local SETUP_SCRIPT RESET_SCRIPT
  prepare_manual_env_scripts "${tmp}"

  assert_file_contains "${SETUP_SCRIPT}" "__declarest_e2e_prune_deleted_run_bins_from_path"
  assert_file_contains "${SETUP_SCRIPT}" "DECLAREST_E2E_ORIGINAL_PROMPT_COMMAND_SET"
  assert_file_contains "${RESET_SCRIPT}" "unset -f __declarest_e2e_prune_deleted_run_bins_from_path"
  assert_file_contains "${RESET_SCRIPT}" "DECLAREST_E2E_ORIGINAL_PROMPT_COMMAND"

  local output status
  set +e
  output=$(
    SETUP_SCRIPT="${SETUP_SCRIPT}" RESET_SCRIPT="${RESET_SCRIPT}" bash <<'EOF'
set -euo pipefail
PROMPT_COMMAND='printf pre >/dev/null'
export PROMPT_COMMAND
source "${SETUP_SCRIPT}"

case ";${PROMPT_COMMAND};" in
  *";__declarest_e2e_prune_deleted_run_bins_from_path; "*) ;;
  *)
    printf 'missing prompt hook: %s\n' "${PROMPT_COMMAND}" >&2
    exit 1
    ;;
esac

source "${RESET_SCRIPT}"
[[ "${PROMPT_COMMAND}" == 'printf pre >/dev/null' ]]
type __declarest_e2e_prune_deleted_run_bins_from_path >/dev/null 2>&1 && exit 1 || true
EOF
  )
  status=$?
  set -e
  assert_status "${status}" "0"
  [[ -z "${output}" ]] || true
}

test_manual_env_prompt_hook_prunes_deleted_run_bin_path_and_alias() {
  load_profile_libs

  local tmp
  tmp=$(new_temp_dir)
  trap 'rm -rf "${tmp}"' RETURN

  local SETUP_SCRIPT RESET_SCRIPT run_dir run_bin
  prepare_manual_env_scripts "${tmp}"
  run_dir="${E2E_RUN_DIR}"
  run_bin="${run_dir}/bin"

  local output status
  set +e
  output=$(
    SETUP_SCRIPT="${SETUP_SCRIPT}" RUN_DIR="${run_dir}" RUN_BIN="${run_bin}" bash <<'EOF'
set -euo pipefail
source "${SETUP_SCRIPT}"

case ":${PATH}:" in
  *":${RUN_BIN}:"*) ;;
  *)
    printf 'expected run bin in PATH before cleanup: %s\n' "${PATH}" >&2
    exit 1
    ;;
esac

alias declarest-e2e >/dev/null 2>&1
rm -rf "${RUN_DIR}"
__declarest_e2e_prune_deleted_run_bins_from_path

case ":${PATH}:" in
  *":${RUN_BIN}:"*)
    printf 'expected run bin to be pruned from PATH: %s\n' "${PATH}" >&2
    exit 1
    ;;
esac

if alias declarest-e2e >/dev/null 2>&1; then
  printf 'expected declarest-e2e alias to be removed after run cleanup\n' >&2
  exit 1
fi
EOF
  )
  status=$?
  set -e
  assert_status "${status}" "0"
  [[ -z "${output}" ]] || true
}

test_manual_env_scripts_install_and_restore_prompt_hook
test_manual_env_prompt_hook_prunes_deleted_run_bin_path_and_alias
