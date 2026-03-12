#!/usr/bin/env bash

CASE_ID='simple-api-server-resource-defaults'
CASE_SCOPE='main'
CASE_REQUIRES='managed-server=simple-api-server'

case_run() {
  local repo_dir
  local alpha_dir
  local beta_dir
  local alpha_path='/api/projects/defaults-sandbox/widgets/defaults-alpha'
  local collection_path='/api/projects/defaults-sandbox/widgets'
  local alpha_file

  repo_dir=$(case_context_repo_base_dir) || return 1
  alpha_dir="${repo_dir}/api/projects/defaults-sandbox/widgets/defaults-alpha"
  beta_dir="${repo_dir}/api/projects/defaults-sandbox/widgets/defaults-beta"
  mkdir -p "${alpha_dir}" "${beta_dir}"

  cat >"${alpha_dir}/resource.json" <<'EOF'
{
  "id": "defaults-alpha",
  "slug": "defaults-alpha",
  "project": "defaults-sandbox",
  "enabled": true,
  "version": 1
}
EOF

  cat >"${beta_dir}/resource.json" <<'EOF'
{
  "id": "defaults-beta",
  "slug": "defaults-beta",
  "project": "defaults-sandbox",
  "enabled": true,
  "version": 2
}
EOF

  case_repo_commit_setup_changes_if_git || return 1

  case_run_declarest resource defaults infer "${alpha_path}" -o json
  case_expect_success
  case_expect_output_contains '"enabled": true'
  case_expect_output_contains '"project": "defaults-sandbox"'

  case_run_declarest resource defaults infer "${alpha_path}" --save -o json
  case_expect_success
  case_expect_output_contains '"enabled": true'

  case_run_declarest resource defaults get "${alpha_path}" -o json
  case_expect_success
  case_expect_output_contains '"project": "defaults-sandbox"'

  alpha_file="${alpha_dir}/resource.json"
  case_run_declarest resource create "${alpha_path}" -f "${alpha_file}" -i json
  case_expect_success

  case_run_declarest resource defaults infer "${alpha_path}" --managed-server
  case_expect_failure
  case_expect_output_contains '--yes'

  case_run_declarest resource defaults infer "${alpha_path}" --managed-server --yes -o json
  case_expect_success
  case_expect_output_contains '{}'

  case_run_declarest resource list "${collection_path}" --source managed-server -o json
  case_expect_success
  case_expect_output_contains '"defaults-alpha"'
  case_expect_output_not_contains 'declarest-defaults-'
}
