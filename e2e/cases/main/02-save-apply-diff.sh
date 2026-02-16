#!/usr/bin/env bash

CASE_ID='save-apply-diff'
CASE_SCOPE='main'
CASE_REQUIRES='has-resource-server'

case_run() {
  local target_path

  case_repo_template_apply_all_metadata
  target_path=$(case_repo_template_first_resource_path) || return 1
  case_repo_template_save_resource_path "${target_path}"

  case_run_declarest resource apply "${target_path}"
  case_expect_success

  case_run_declarest resource diff "${target_path}" -o json
  case_expect_success
  case_expect_output_contains '[]'
}
