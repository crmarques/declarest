#!/usr/bin/env bash

CASE_ID='list-deterministic'
CASE_SCOPE='main'
CASE_REQUIRES='has-resource-server'

case_run() {
  local first_resource_path
  local target_collection_path
  local logical_path

  case_repo_template_apply_all_metadata

  first_resource_path=$(case_repo_template_first_resource_path) || return 1
  target_collection_path=$(case_repo_template_collection_path_for_resource "${first_resource_path}") || return 1

  while IFS= read -r logical_path; do
    [[ -n "${logical_path}" ]] || continue
    case_repo_template_create_resource_path "${logical_path}"
  done < <(case_repo_template_collection_resource_paths "${target_collection_path}")

  case_run_declarest resource list "${target_collection_path}" --remote-server -o json
  case_expect_success

  if ! jq -e 'map(.LogicalPath) as $paths | $paths == ($paths | sort)' <<<"${CASE_LAST_OUTPUT}" >/dev/null; then
    printf 'expected deterministic sorted list order\n' >&2
    printf 'output: %s\n' "${CASE_LAST_OUTPUT}" >&2
    return 1
  fi
}
