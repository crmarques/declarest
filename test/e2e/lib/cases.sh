#!/usr/bin/env bash

E2E_CASE_FILES=()

e2e_case_declared_profiles() {
  local case_scope=$1
  local case_profiles=$2

  if [[ -n "${case_profiles}" ]]; then
    printf '%s\n' "${case_profiles}"
    return 0
  fi

  if [[ "${case_scope}" == 'operator-main' ]]; then
    printf 'operator\n'
    return 0
  fi

  printf 'cli\n'
}

e2e_case_validate_profiles() {
  local case_file=$1
  local case_scope=$2
  local case_profiles=$3
  local token
  local has_valid=0

  for token in ${case_profiles}; do
    case "${token}" in
      cli|operator)
        has_valid=1
        ;;
      *)
        e2e_die "case ${case_file} has invalid CASE_PROFILES entry: ${token} (allowed: cli, operator)"
        return 2
        ;;
    esac
  done

  if ((has_valid == 0)); then
    e2e_die "case ${case_file} must declare at least one CASE_PROFILES entry when set"
    return 2
  fi

  if [[ "${case_scope}" == 'operator-main' ]] && [[ " ${case_profiles} " != *' operator '* ]]; then
    e2e_die "case ${case_file} uses CASE_SCOPE=operator-main but does not include operator in CASE_PROFILES"
    return 2
  fi

  return 0
}

e2e_case_file_matches_current_profile() {
  local case_file=$1
  local expected_scope=$2
  local metadata
  local case_scope
  local case_profiles
  local declared_profiles
  local current_family

  metadata=$(
    awk '
      BEGIN {
        case_scope = ""
        case_profiles = ""
      }
      /^CASE_SCOPE=/ && case_scope == "" {
        value = $0
        sub(/^CASE_SCOPE=/, "", value)
        gsub(/^'\''|'\''$/, "", value)
        gsub(/^"|"$/, "", value)
        case_scope = value
      }
      /^CASE_PROFILES=/ && case_profiles == "" {
        value = $0
        sub(/^CASE_PROFILES=/, "", value)
        gsub(/^'\''|'\''$/, "", value)
        gsub(/^"|"$/, "", value)
        case_profiles = value
      }
      END {
        printf "%s\x1f%s\n", case_scope, case_profiles
      }
    ' "${case_file}"
  ) || {
    e2e_die "failed to read case metadata from ${case_file}"
    return 2
  }

  IFS=$'\x1f' read -r case_scope case_profiles <<<"${metadata}"
  if [[ -z "${case_scope}" ]]; then
    e2e_die "case ${case_file} must declare CASE_SCOPE"
    return 2
  fi

  if [[ "${case_scope}" != "${expected_scope}" ]]; then
    e2e_die "case ${case_file} declared CASE_SCOPE=${case_scope} but was discovered in scope ${expected_scope}"
    return 2
  fi

  declared_profiles=$(e2e_case_declared_profiles "${case_scope}" "${case_profiles}") || return 2
  e2e_case_validate_profiles "${case_file}" "${case_scope}" "${declared_profiles}" || return $?

  current_family=$(e2e_profile_family)
  [[ " ${declared_profiles} " == *" ${current_family} "* ]]
}

e2e_collect_scope_case_files() {
  local scope=$1
  local base_dir=$2
  local scope_dir="${base_dir}/${scope}"
  local file

  [[ -d "${scope_dir}" ]] || return 0

  while IFS= read -r file; do
    [[ -n "${file}" ]] || continue
    local match_status=0
    e2e_case_file_matches_current_profile "${file}" "${scope}" || match_status=$?
    if ((match_status == 0)); then
      printf '%s\n' "${file}"
      continue
    fi

    if ((match_status == 1)); then
      continue
    fi

    return 1
  done < <(find "${scope_dir}" -maxdepth 1 -type f -name '*.sh' | sort)
}

case_selected_value_for_key() {
  local key=$1
  case "${key}" in
    profile) printf '%s\n' "${E2E_PROFILE}" ;;
    platform) printf '%s\n' "${E2E_PLATFORM}" ;;
    managed-server) printf '%s\n' "${E2E_MANAGED_SERVER}" ;;
    managed-server-connection) printf '%s\n' "${E2E_MANAGED_SERVER_CONNECTION}" ;;
    managed-server-auth-type) printf '%s\n' "${E2E_MANAGED_SERVER_AUTH_TYPE}" ;;
    managed-server-mtls) printf '%s\n' "${E2E_MANAGED_SERVER_MTLS}" ;;
    managed-server-proxy) printf '%s\n' "${E2E_MANAGED_SERVER_PROXY}" ;;
    managed-server-proxy-auth-type) printf '%s\n' "$(e2e_effective_proxy_auth_type)" ;;
    proxy-mode) printf '%s\n' "${E2E_PROXY_MODE:-none}" ;;
    proxy-auth-type) printf '%s\n' "$(e2e_effective_proxy_auth_type)" ;;
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
    has-managed-server)
      e2e_is_explicit 'managed-server' && [[ "${E2E_MANAGED_SERVER}" != 'none' ]]
      return
      ;;
    has-managed-server-mtls)
      e2e_is_explicit 'managed-server-mtls' && [[ "${E2E_MANAGED_SERVER_MTLS}" == 'true' ]]
      return
      ;;
    has-managed-server-proxy)
      (e2e_is_explicit 'managed-server-proxy' || e2e_is_explicit 'proxy-mode') && [[ "${E2E_MANAGED_SERVER_PROXY}" == 'true' ]]
      return
      ;;
    has-proxy)
      (e2e_is_explicit 'proxy-mode' || e2e_is_explicit 'managed-server-proxy') && [[ "${E2E_PROXY_MODE:-none}" != 'none' ]]
      return
      ;;
    remote-selection)
      if e2e_is_explicit 'managed-server-connection' && [[ "${E2E_MANAGED_SERVER_CONNECTION}" == 'remote' ]]; then
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
  local -A seen=()

  local scope
  while IFS= read -r scope; do
    [[ -n "${scope}" ]] || continue

    local file
    while IFS= read -r file; do
      [[ -n "${file}" ]] || continue
      if [[ -n "${seen[${file}]:-}" ]]; then
        continue
      fi
      seen["${file}"]=1
      E2E_CASE_FILES+=("${file}")
    done < <(e2e_collect_scope_case_files "${scope}" "${E2E_DIR}/cases")

    local component_key
    for component_key in "${E2E_SELECTED_COMPONENT_KEYS[@]}"; do
      local component_cases_root="${E2E_COMPONENT_PATH[${component_key}]}/cases"
      while IFS= read -r file; do
        [[ -n "${file}" ]] || continue
        if [[ -n "${seen[${file}]:-}" ]]; then
          continue
        fi
        seen["${file}"]=1
        E2E_CASE_FILES+=("${file}")
      done < <(e2e_collect_scope_case_files "${scope}" "${component_cases_root}")
    done
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

  {
    printf '[%s] CASE START id=%s file=%s\n' "$(e2e_now_utc)" "${case_id}" "${case_file}"
    printf '[%s] CASE requires=%s\n' "$(e2e_now_utc)" "${case_requires:-<none>}"
    printf '[%s] CASE stack profile=%s platform=%s repo-type=%s managed-server=%s(%s) managed-server-security=auth-type:%s mtls:%s proxy:%s git-provider=%s(%s) secret-provider=%s(%s)\n' \
      "$(e2e_now_utc)" \
      "${E2E_PROFILE}" \
      "${E2E_PLATFORM}" \
      "${E2E_REPO_TYPE}" \
      "${E2E_MANAGED_SERVER}" \
      "${E2E_MANAGED_SERVER_CONNECTION}" \
      "${E2E_MANAGED_SERVER_AUTH_TYPE:-auto}" \
      "${E2E_MANAGED_SERVER_MTLS:-false}" \
      "${E2E_PROXY_MODE:-none}/$(e2e_effective_proxy_auth_type)" \
      "${E2E_GIT_PROVIDER:-none}" \
      "${E2E_GIT_PROVIDER_CONNECTION}" \
      "${E2E_SECRET_PROVIDER}" \
      "${E2E_SECRET_PROVIDER_CONNECTION}"
    printf '[%s] CASE tmp-dir=%s\n' "$(e2e_now_utc)" "${case_dir}"
  } >"${case_log}"

  if [[ -n "${E2E_EXECUTION_LOG:-}" ]]; then
    printf '[%s] CASE START id=%s file=%s\n' "$(e2e_now_utc)" "${case_id}" "${case_file}" >>"${E2E_EXECUTION_LOG}"
  fi

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
  ) >>"${case_log}" 2>&1
  local rc=$?
  set -e

  {
    printf '[%s] CASE END id=%s rc=%d\n' "$(e2e_now_utc)" "${case_id}" "${rc}"
  } >>"${case_log}"
  if [[ -n "${E2E_EXECUTION_LOG:-}" ]]; then
    printf '[%s] CASE END id=%s rc=%d log=%s\n' "$(e2e_now_utc)" "${case_id}" "${rc}" "${case_log}" >>"${E2E_EXECUTION_LOG}"
  fi

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
