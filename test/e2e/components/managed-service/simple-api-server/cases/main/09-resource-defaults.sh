#!/usr/bin/env bash

CASE_ID='simple-api-server-resource-defaults'
CASE_SCOPE='main'
CASE_REQUIRES='managed-service=simple-api-server'

case_run() {
  local repo_dir
  local project_path='/api/projects/defaults-sandbox'
  local project_dir
  local alpha_dir
  local beta_dir
  local alpha_path='/api/projects/defaults-sandbox/widgets/defaults-alpha'
  local collection_path='/api/projects/defaults-sandbox/widgets'
  local project_file
  local alpha_file

  repo_dir=$(case_context_repo_base_dir) || return 1
  project_dir="${repo_dir}/api/projects/defaults-sandbox"
  alpha_dir="${repo_dir}/api/projects/defaults-sandbox/widgets/defaults-alpha"
  beta_dir="${repo_dir}/api/projects/defaults-sandbox/widgets/defaults-beta"
  mkdir -p "${project_dir}" "${alpha_dir}" "${beta_dir}"

  cat >"${project_dir}/resource.json" <<'EOF'
{
  "id": "defaults-sandbox",
  "name": "defaults-sandbox",
  "displayName": "Defaults Sandbox",
  "owner": "defaults-team"
}
EOF

  cat >"${project_dir}/defaults.json" <<'EOF'
{
  "owner": "defaults-team"
}
EOF

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
  if [ -n "${CASE_LAST_STDOUT}" ]; then
    printf 'expected resource defaults infer --save to suppress stdout, got %s\n' "${CASE_LAST_STDOUT}" >&2
    return 1
  fi

  case_run_declarest resource defaults infer "${alpha_path}" --check -o json
  case_expect_success
  case_expect_output_contains '"enabled": true'

  case_run_declarest resource defaults get "${alpha_path}" -o json
  case_expect_success
  case_expect_output_contains '"project": "defaults-sandbox"'

  case_run_declarest resource get "${alpha_path}" --source repository -o json
  case_expect_success
  case_expect_output_contains '"project": "defaults-sandbox"'
  case_expect_output_contains '"enabled": true'

  case_run_declarest resource get "${alpha_path}" --source repository --prune-defaults -o json
  case_expect_success
  case_expect_output_contains '"version": 1'
  case_expect_output_not_contains '"project": "defaults-sandbox"'
  case_expect_output_not_contains '"enabled": true'

  project_file="${project_dir}/resource.json"
  alpha_file="${alpha_dir}/resource.json"
  case_run_declarest resource request put /api/projects/defaults-sandbox --payload "${project_file}" --content-type json
  case_expect_success

  case_run_declarest resource request put /api/projects/defaults-sandbox/widgets/defaults-alpha --payload "${alpha_file}" --content-type json
  case_expect_success

  case_run_declarest resource get "${project_path}" --source repository --prune-defaults -o json
  case_expect_success
  case_expect_output_contains '"displayName": "Defaults Sandbox"'
  case_expect_output_not_contains '"owner": "defaults-team"'

  case_run_declarest resource get "${project_path}" --source managed-service --prune-defaults -o json
  case_expect_success
  case_expect_output_contains '"displayName": "Defaults Sandbox"'
  case_expect_output_not_contains '"owner": "defaults-team"'

  case_run_declarest resource save "${project_path}" --prune-defaults --force
  case_expect_success
  if grep -Fq '"owner": "defaults-team"' "${project_file}"; then
    printf 'expected saved resource file to prune default owner value\n' >&2
    return 1
  fi
  if ! grep -Fq '"displayName": "Defaults Sandbox"' "${project_file}"; then
    printf 'expected saved resource file to keep explicit displayName\n' >&2
    return 1
  fi

  case_run_declarest resource defaults infer "${alpha_path}" --from managed-service
  case_expect_failure
  case_expect_output_contains '--yes'

  case_run_declarest resource defaults infer "${alpha_path}" --from managed-service --yes -o json
  case_expect_success
  case_expect_output_contains '"enabled": true'
  case_expect_output_contains '"project": "defaults-sandbox"'

  case_run_declarest resource list "${collection_path}" --source managed-service -o json
  case_expect_success
  case_expect_output_contains '"defaults-alpha"'

  case_run_declarest resource list /api/projects --source managed-service -o json
  case_expect_success
  case_expect_output_contains '"defaults-sandbox"'
  case_expect_output_not_contains 'declarest-defaults-'
}
