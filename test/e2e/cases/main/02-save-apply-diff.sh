#!/usr/bin/env bash

CASE_ID='save-apply-diff'
CASE_SCOPE='main'
CASE_REQUIRES='has-managed-server'

case_run() {
  local target_path
  local source_file
  local payload_file

  case_repo_template_apply_all_metadata
  target_path=$(case_repo_template_first_resource_path) || return 1
  source_file=$(case_repo_template_resource_file_for_path "${target_path}") || return 1
  payload_file="${source_file}"

  if [[ "${E2E_MANAGED_SERVER:-}" == 'rundeck' ]]; then
    local target_name="platform-save-apply-${RANDOM}${RANDOM}"
    target_path="/projects/${target_name}"
    payload_file="${E2E_CASE_TMP_DIR}/save-apply-diff-rundeck.json"
    jq --arg name "${target_name}" '
      if type == "object" and has("name") then
        .name = $name
      else
        .
      end
    ' "${source_file}" >"${payload_file}"
  fi

  case_run_declarest resource save "${target_path}" -f "${payload_file}" -i json
  case_expect_success

  case_run_declarest resource apply "${target_path}"
  case_expect_success

  case_run_declarest resource apply "${target_path}"
  case_expect_success

  case_run_declarest resource diff "${target_path}" -o json
  case_expect_success
  case_expect_output_contains '[]'

  case_run_declarest resource apply "${target_path}" --force
  case_expect_success

  case_run_declarest resource diff "${target_path}" -o json
  case_expect_success
  case_expect_output_contains '[]'
}
