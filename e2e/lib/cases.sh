#!/usr/bin/env bash

E2E_CASE_FILES=()

case_selected_value_for_key() {
  local key=$1
  case "${key}" in
    profile) printf '%s\n' "${E2E_PROFILE}" ;;
    resource-server) printf '%s\n' "${E2E_RESOURCE_SERVER}" ;;
    resource-server-connection) printf '%s\n' "${E2E_RESOURCE_SERVER_CONNECTION}" ;;
    repo-type) printf '%s\n' "${E2E_REPO_TYPE}" ;;
    git-provider) printf '%s\n' "${E2E_GIT_PROVIDER}" ;;
    git-provider-connection) printf '%s\n' "${E2E_GIT_PROVIDER_CONNECTION}" ;;
    secret-provider) printf '%s\n' "${E2E_SECRET_PROVIDER}" ;;
    secret-provider-connection) printf '%s\n' "${E2E_SECRET_PROVIDER_CONNECTION}" ;;
    *) printf '' ;;
  esac
}

case_requirement_matches() {
  local requirement=$1

  if [[ "${requirement}" == *=* ]]; then
    local key=${requirement%%=*}
    local expected=${requirement#*=}
    local actual
    actual=$(case_selected_value_for_key "${key}")
    [[ -n "${actual}" && "${actual}" == "${expected}" ]]
    return
  fi

  e2e_has_capability "${requirement}"
}

case_requirement_requested_explicitly() {
  local requirement=$1

  if [[ "${requirement}" == *=* ]]; then
    local key=${requirement%%=*}
    local expected=${requirement#*=}
    local actual
    actual=$(case_selected_value_for_key "${key}")

    if e2e_is_explicit "${key}" && [[ "${actual}" == "${expected}" ]]; then
      return 0
    fi

    return 1
  fi

  case "${requirement}" in
    has-secret-provider)
      e2e_is_explicit 'secret-provider' && [[ "${E2E_SECRET_PROVIDER}" != 'none' ]]
      return
      ;;
    has-resource-server)
      e2e_is_explicit 'resource-server' && [[ "${E2E_RESOURCE_SERVER}" != 'none' ]]
      return
      ;;
    remote-selection)
      if e2e_is_explicit 'resource-server-connection' && [[ "${E2E_RESOURCE_SERVER_CONNECTION}" == 'remote' ]]; then
        return 0
      fi
      if e2e_is_explicit 'git-provider-connection' && [[ "${E2E_GIT_PROVIDER_CONNECTION}" == 'remote' ]]; then
        return 0
      fi
      if e2e_is_explicit 'secret-provider-connection' && [[ "${E2E_SECRET_PROVIDER_CONNECTION}" == 'remote' ]]; then
        return 0
      fi
      return 1
      ;;
    *)
      return 1
      ;;
  esac
}

e2e_collect_case_files() {
  E2E_CASE_FILES=()

  local scope
  while IFS= read -r scope; do
    [[ -n "${scope}" ]] || continue

    local file
    while IFS= read -r file; do
      [[ -n "${file}" ]] || continue
      E2E_CASE_FILES+=("${file}")
    done < <(find "${E2E_DIR}/cases/${scope}" -maxdepth 1 -type f -name '*.sh' | sort)
  done < <(e2e_profile_scopes)
}

e2e_run_cases() {
  local case_file
  local failed=0

  if ((${#E2E_CASE_FILES[@]} == 0)); then
    e2e_warn 'no case files found for selected profile'
    return 0
  fi

  for case_file in "${E2E_CASE_FILES[@]}"; do
    if ! e2e_run_single_case "${case_file}"; then
      failed=1
    fi
  done

  if ((failed == 1)); then
    return 1
  fi

  return 0
}

e2e_run_single_case() {
  local case_file=$1

  unset CASE_ID CASE_SCOPE CASE_REQUIRES
  # shellcheck disable=SC1090
  source "${case_file}"

  local case_id=${CASE_ID:-$(basename "${case_file}" .sh)}
  local case_requires=${CASE_REQUIRES:-}

  local requirement
  local missing=()
  local missing_mandatory=()

  for requirement in ${case_requires}; do
    if ! case_requirement_matches "${requirement}"; then
      missing+=("${requirement}")
      if case_requirement_requested_explicitly "${requirement}"; then
        missing_mandatory+=("${requirement}")
      fi
    fi
  done

  if ((${#missing_mandatory[@]} > 0)); then
    ui_case_result "${case_id}" 'FAIL' "missing mandatory requirements: ${missing_mandatory[*]}"
    return 1
  fi

  if ((${#missing[@]} > 0)); then
    ui_case_result "${case_id}" 'SKIP' "missing requirements: ${missing[*]}"
    return 0
  fi

  local case_dir="${E2E_RUN_DIR}/cases/${case_id}"
  local case_log="${E2E_LOG_DIR}/case-${case_id}.log"
  mkdir -p "${case_dir}"

  set +e
  (
    set -euo pipefail
    export E2E_CASE_ID="${case_id}"
    export E2E_CASE_TMP_DIR="${case_dir}"

    # shellcheck disable=SC1091
    source "${E2E_DIR}/lib/assert.sh"
    # shellcheck disable=SC1090
    source "${case_file}"

    case_run
  ) >"${case_log}" 2>&1
  local rc=$?
  set -e

  if ((rc == 0)); then
    ui_case_result "${case_id}" 'PASS'
    if ((E2E_VERBOSE == 1)); then
      printf '    log: %s\n' "${case_log}"
    fi
    return 0
  fi

  ui_case_result "${case_id}" 'FAIL' "log=${case_log}"
  tail -n 30 "${case_log}" | sed 's/^/    | /'
  return 1
}
