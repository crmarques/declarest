#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR=$(cd -- "$(dirname "${BASH_SOURCE[0]}")" && pwd)
# shellcheck disable=SC1091
source "${SCRIPT_DIR}/lib/common.sh"
# shellcheck disable=SC1091
source "${SCRIPT_DIR}/lib/args.sh"
# shellcheck disable=SC1091
source "${SCRIPT_DIR}/lib/profile.sh"
# shellcheck disable=SC1091
source "${SCRIPT_DIR}/lib/components.sh"
# shellcheck disable=SC1091
source "${SCRIPT_DIR}/lib/context.sh"
# shellcheck disable=SC1091
source "${SCRIPT_DIR}/lib/cases.sh"
# shellcheck disable=SC1091
source "${SCRIPT_DIR}/lib/ui.sh"

E2E_CLI_ARGS=()
E2E_SHORT_CIRCUIT=0
E2E_OVERALL_FAILED=0
E2E_FINALIZED=0
E2E_BOOTSTRAP_LOG_DIR=''

step_initialize() {
  e2e_parse_args "${E2E_CLI_ARGS[@]}" || return 1
  e2e_apply_profile_defaults || return 1

  e2e_discover_components || return 1

  if ((E2E_LIST_COMPONENTS == 1)); then
    e2e_list_components || return 1
    E2E_SHORT_CIRCUIT=1
    return 0
  fi

  e2e_validate_selection || return 1
  e2e_validate_profile_rules || return 1

  e2e_build_selected_components || return 1
  e2e_build_capabilities || return 1
  e2e_preflight_requirements || return 1

  e2e_info "profile=${E2E_PROFILE} repo-type=${E2E_REPO_TYPE} resource-server=${E2E_RESOURCE_SERVER} secret-provider=${E2E_SECRET_PROVIDER}"
  return 0
}

step_prepare_runtime() {
  E2E_RUN_ID=$(date +%Y%m%d-%H%M%S)-$$
  E2E_RUN_DIR="${E2E_RUNS_DIR}/${E2E_RUN_ID}"
  E2E_STATE_DIR="${E2E_RUN_DIR}/state"
  E2E_LOG_DIR="${E2E_RUN_DIR}/logs"
  E2E_CONTEXT_DIR="${E2E_RUN_DIR}/context"
  E2E_CONTEXT_FILE="${E2E_RUN_DIR}/contexts.yaml"
  E2E_BIN="${E2E_RUN_DIR}/bin/declarest"

  mkdir -p "${E2E_RUN_DIR}" "${E2E_STATE_DIR}" "${E2E_LOG_DIR}" "${E2E_CONTEXT_DIR}" "$(dirname -- "${E2E_BIN}")" || return 1

  if [[ -n "${E2E_BOOTSTRAP_LOG_DIR}" && -d "${E2E_BOOTSTRAP_LOG_DIR}" ]]; then
    cp -a "${E2E_BOOTSTRAP_LOG_DIR}/." "${E2E_LOG_DIR}/" 2>/dev/null || true
  fi

  e2e_run_cmd go build -o "${E2E_BIN}" ./cmd/declarest || return 1

  e2e_collect_case_files || return 1
  return 0
}

step_prepare_components() {
  e2e_components_run_hook_all 'init' || return 1
}

step_start_components() {
  e2e_components_start_local || return 1
  e2e_components_healthcheck_local || return 1
}

step_configure_access() {
  e2e_components_run_hook_all 'configure-auth' || return 1

  mkdir -p "${E2E_CONTEXT_DIR}" || return 1

  local component_key
  for component_key in "${E2E_SELECTED_COMPONENT_KEYS[@]}"; do
    local fragment_file
    fragment_file="${E2E_CONTEXT_DIR}/$(e2e_component_type "${component_key}")-$(e2e_component_name "${component_key}").yaml"
    export E2E_COMPONENT_CONTEXT_FRAGMENT="${fragment_file}"
    e2e_component_run_hook "${component_key}" 'context' "${fragment_file}" || return 1
  done

  e2e_context_build || return 1

  DECLAREST_CONTEXTS_FILE="${E2E_CONTEXT_FILE}" "${E2E_BIN}" --context "${E2E_CONTEXT_NAME}" config show >/dev/null || return 1
}

step_run_workload() {
  if [[ "${E2E_PROFILE}" == 'manual' ]]; then
    e2e_profile_manual_handoff "${E2E_CONTEXT_NAME}" || return 1
    return 0
  fi

  e2e_run_cases || return 1
}

step_skip_not_requested() {
  return "${E2E_STEP_SKIP}"
}

step_finalize() {
  if ((E2E_FINALIZED == 1)); then
    return 0
  fi

  E2E_FINALIZED=1

  if ((E2E_KEEP_RUNTIME == 1)); then
    e2e_info 'keeping runtime resources because --keep-runtime was set'
  else
    e2e_components_stop_started || true
  fi

  if [[ -n "${E2E_BOOTSTRAP_LOG_DIR}" && -d "${E2E_BOOTSTRAP_LOG_DIR}" && "${E2E_LOG_DIR}" != "${E2E_BOOTSTRAP_LOG_DIR}" ]]; then
    rm -rf "${E2E_BOOTSTRAP_LOG_DIR}" || true
  fi

  e2e_cleanup_temp_files
  return 0
}

main() {
  E2E_CLI_ARGS=("$@")
  E2E_START_EPOCH=$(e2e_epoch_now)

  E2E_BOOTSTRAP_LOG_DIR=$(mktemp -d /tmp/declarest-e2e-bootstrap.XXXXXX)
  E2E_LOG_DIR="${E2E_BOOTSTRAP_LOG_DIR}"

  ui_init
  E2E_STEPS_TOTAL=7

  if ! ui_run_step 1 "${E2E_STEPS_TOTAL}" 'Initializing' step_initialize; then
    E2E_OVERALL_FAILED=1
  fi

  if ((E2E_OVERALL_FAILED == 0)); then
    if ((E2E_SHORT_CIRCUIT == 1)); then
      cat "${E2E_LOG_DIR}/step-1.log"
      ui_run_step 2 "${E2E_STEPS_TOTAL}" 'Preparing Runtime' step_skip_not_requested || true
      ui_run_step 3 "${E2E_STEPS_TOTAL}" 'Preparing Components' step_skip_not_requested || true
      ui_run_step 4 "${E2E_STEPS_TOTAL}" 'Starting Components' step_skip_not_requested || true
      ui_run_step 5 "${E2E_STEPS_TOTAL}" 'Configuring Access' step_skip_not_requested || true
      ui_run_step 6 "${E2E_STEPS_TOTAL}" 'Running Workload' step_skip_not_requested || true
    else
      ui_run_step 2 "${E2E_STEPS_TOTAL}" 'Preparing Runtime' step_prepare_runtime || E2E_OVERALL_FAILED=1
      if ((E2E_OVERALL_FAILED == 0)); then
        ui_run_step 3 "${E2E_STEPS_TOTAL}" 'Preparing Components' step_prepare_components || E2E_OVERALL_FAILED=1
      fi
      if ((E2E_OVERALL_FAILED == 0)); then
        ui_run_step 4 "${E2E_STEPS_TOTAL}" 'Starting Components' step_start_components || E2E_OVERALL_FAILED=1
      fi
      if ((E2E_OVERALL_FAILED == 0)); then
        ui_run_step 5 "${E2E_STEPS_TOTAL}" 'Configuring Access' step_configure_access || E2E_OVERALL_FAILED=1
      fi
      if ((E2E_OVERALL_FAILED == 0)); then
        ui_run_step 6 "${E2E_STEPS_TOTAL}" 'Running Workload' step_run_workload || E2E_OVERALL_FAILED=1
      fi
    fi
  fi

  ui_run_step 7 "${E2E_STEPS_TOTAL}" 'Finalizing' step_finalize || true
  ui_print_summary

  if ((E2E_OVERALL_FAILED == 1 || E2E_CASE_FAILED > 0)); then
    exit 1
  fi

  exit 0
}

main "$@"
