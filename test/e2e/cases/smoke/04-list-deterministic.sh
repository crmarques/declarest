#!/usr/bin/env bash

CASE_ID='list-deterministic'
CASE_SCOPE='smoke'
CASE_PROFILES='cli operator'
CASE_REQUIRES='has-managed-server'

case_run() {
  local first_resource_path
  local target_collection_path
  local logical_path
  local -a created_paths=()

  case_repo_template_apply_all_metadata

  first_resource_path=$(case_repo_template_first_resource_path) || return 1
  target_collection_path=$(case_repo_template_collection_path_for_resource "${first_resource_path}") || return 1

  if [[ "${E2E_MANAGED_SERVER:-}" == 'rundeck' && "${target_collection_path}" == '/projects' ]]; then
    local source_file payload_file temp_name temp_path
    source_file=$(case_repo_template_resource_file_for_path "${first_resource_path}") || return 1
    temp_name="list-deterministic-${RANDOM}${RANDOM}"
    temp_path="/projects/${temp_name}"
    payload_file="${E2E_CASE_TMP_DIR}/rundeck-list-deterministic.json"

    jq --arg name "${temp_name}" '
      if type == "object" and has("name") then
        .name = $name
      else
        .
      end
    ' "${source_file}" >"${payload_file}"

    case_run_declarest resource create "${temp_path}" -f "${payload_file}" -i json
    case_expect_success
    created_paths+=("${temp_path}")
  else
    while IFS= read -r logical_path; do
      [[ -n "${logical_path}" ]] || continue
      case_repo_template_create_resource_path "${logical_path}"
      created_paths+=("${logical_path}")
    done < <(case_repo_template_collection_resource_paths "${target_collection_path}")
  fi

  case_run_declarest resource list "${target_collection_path}" --source managed-server -o json
  case_expect_success

  if ! jq -e 'map(.LogicalPath) as $paths | $paths == ($paths | sort)' <<<"${CASE_LAST_STDOUT}" >/dev/null; then
    printf 'expected deterministic sorted list order\n' >&2
    printf 'output: %s\n' "${CASE_LAST_OUTPUT}" >&2
    return 1
  fi

  local index
  for ((index = ${#created_paths[@]} - 1; index >= 0; index--)); do
    case_run_declarest resource delete "${created_paths[${index}]}" -y
    if ((CASE_LAST_STATUS != 0)) && ! grep -qiE 'not found|status 404' <<<"${CASE_LAST_OUTPUT}"; then
      printf 'failed to clean up created resource %s\n' "${created_paths[${index}]}" >&2
      printf 'output: %s\n' "${CASE_LAST_OUTPUT}" >&2
      return 1
    fi
  done
}
