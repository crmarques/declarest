#!/usr/bin/env bash

CASE_ID='rundeck-secret-storage-non-json-payload'
CASE_SCOPE='main'
CASE_REQUIRES='managed-server=rundeck has-secret-provider'

case_run() {
  local project_name="platform-secret-${RANDOM}${RANDOM}"
  local secret_name="tls-key-${RANDOM}${RANDOM}.pub"
  local project_path="/projects/${project_name}"
  local secret_path="${project_path}/secrets/${secret_name}"
  local project_source_file
  local project_payload_file="${E2E_CASE_TMP_DIR}/project.json"
  local private_key_file="${E2E_CASE_TMP_DIR}/private.key"
  local private_key_name="tls-private-${RANDOM}${RANDOM}.key"
  local private_key_path="${project_path}/secrets/${private_key_name}"
  local secret_metadata_file="${E2E_CASE_TMP_DIR}/secret-metadata.json"
  local secret_payload_file="${E2E_CASE_TMP_DIR}/secret.json"
  local armored_key_value
  local repository_secret_value

  case_run_declarest secret init
  case_expect_success

  project_source_file=$(case_repo_template_resource_file_for_path '/projects/platform' 'rundeck') || return 1
  jq \
    --arg name "${project_name}" \
    '
      .name = $name
      | .description = "Managed by declarest E2E non-json payload case"
      | .config["project.label"] = $name
      | .config["project.description"] = "Managed by declarest E2E non-json payload case"
    ' \
    "${project_source_file}" >"${project_payload_file}"

  case_run_declarest resource create "${project_path}" -f "${project_payload_file}" --content-type json
  case_expect_success

  case_write_json "${secret_metadata_file}" '{
    "resource": {
      "secretAttributes": ["/content"]
    }
  }'

  case_run_declarest metadata set "${secret_path}" -f "${secret_metadata_file}" --content-type json
  case_expect_success
  case_repo_commit_setup_changes_if_git

  armored_key_value=$(cat <<'EOF'
-----BEGIN PGP PUBLIC KEY BLOCK-----
Version: DeclaREST E2E

mQENBGX1K4UBCADQ8m3xY2G9Zb3vF1xq6XlYJ5w2q9dVvM2gP7nS4kL1tQ8uY5cM
3rV8nP6sF1yQ9cW4hT7mK2xB5vN8qD1gL4sR7wJ0zC3fV6aH9mP2tS5yV8bK1nQ4
gR7uK0xN3cF6vJ9sL2pD5hG8tK1mQ4wR7yU0aC3eF6hJ9kL2mN5pQ8tR1vW4yZ7
fA0cD3gF6jH9lK2nM5qP8tS1wV4yY7bA0dC3fG6jJ9mL2pN5rQ8tT1wV4zY7cA0
ABEBAAG0KURlY2xhUkVTVCBFMkUgUnVuZGVjayBUZXN0IEtleSA8ZTJlQGV4YW1w
bGUuY29tPokBTgQTAQoAOBYhBGrDk1CkG7V4hJ9qL2nM5qP8tS1wBQJl9SuFAhsD
BQsJCAcCBhUKCQgLAgQWAgMBAh4BAheAAAoJEC9pzOaj/LUtcX8H/0i6NQ2eV8u1
W4x7Z0c3F6i9L2o5R8u1X4a7D0g3J6m9P2s5V8y1B4e7H0k3N6q9T2w5Y8b1E4h7K
0n3Q6t9W2z5C8f1A4d7G0j3M6p9S2v5Y8b1E4h7K0n3Q6t9W2z5C8f1A4d7G0j3M
6p9S2v5Y8b1E4h7K0n3Q6t9W2z5C8f1A4d7G0j3M6p9S2v5Y8b1E4h7K0n3Q6t9W
=e2e1
-----END PGP PUBLIC KEY BLOCK-----
EOF
)

  jq \
    -n \
    --arg name "${secret_name}" \
    --arg content "${armored_key_value}" \
    '{
      name: $name,
      type: "public",
      contentType: "application/pgp-keys",
      content: $content
    }' >"${secret_payload_file}"

  case_run_declarest resource save "${secret_path}" -f "${secret_payload_file}" --content-type json
  case_expect_success

  case_run_declarest resource get "${secret_path}" --source repository -o json
  case_expect_success
  if ! jq -e \
    --arg name "${secret_name}" \
    '.name == $name and .type == "public" and .contentType == "application/pgp-keys" and .content == "{{secret .}}"' \
    <<<"${CASE_LAST_STDOUT}" >/dev/null; then
    printf 'expected repository payload to keep metadata fields and mask /content with a secret placeholder\n' >&2
    printf 'output: %s\n' "${CASE_LAST_OUTPUT}" >&2
    return 1
  fi

  case_run_declarest resource get "${secret_path}" --source repository --show-secrets -o json
  case_expect_success
  repository_secret_value=$(jq -r '.content' <<<"${CASE_LAST_STDOUT}")
  if [[ "${repository_secret_value}" != "${armored_key_value}" ]]; then
    printf 'expected --show-secrets to resolve armored key content from the secret store\n' >&2
    return 1
  fi

  case_run_declarest secret get "${secret_path}:/content"
  case_expect_success
  if [[ "${CASE_LAST_STDOUT}" != "${armored_key_value}" ]]; then
    printf 'expected file secret store to contain the original armored key content\n' >&2
    return 1
  fi

  case_run_declarest resource apply "${secret_path}"
  case_expect_success

  case_run_declarest resource diff "${secret_path}" -o json
  case_expect_success
  if ! jq -e '. == []' <<<"${CASE_LAST_STDOUT}" >/dev/null; then
    printf 'expected secret compare pipeline to stay idempotent after apply\n' >&2
    printf 'output: %s\n' "${CASE_LAST_OUTPUT}" >&2
    return 1
  fi

  case_run_declarest resource get "${secret_path}" --source managed-server -o json
  case_expect_success
  if ! jq -e \
    --arg name "${secret_name}" \
    '.name == $name and .type == "public" and .contentType == "application/pgp-keys"' \
    <<<"${CASE_LAST_STDOUT}" >/dev/null; then
    printf 'expected Rundeck managed-server metadata to describe the saved armored key\n' >&2
    printf 'output: %s\n' "${CASE_LAST_OUTPUT}" >&2
    return 1
  fi

  case_run_declarest resource list "${project_path}/secrets" --source managed-server -o json
  case_expect_success
  if ! jq -e \
    --arg name "${secret_name}" \
    'map(select(.name == $name and .type == "public" and .contentType == "application/pgp-keys")) | length == 1' \
    <<<"${CASE_LAST_STDOUT}" >/dev/null; then
    printf 'expected secret collection listing to include the saved armored key metadata\n' >&2
    printf 'output: %s\n' "${CASE_LAST_OUTPUT}" >&2
    return 1
  fi

  cat >"${private_key_file}" <<'EOF'
-----BEGIN PRIVATE KEY-----
MIIEvAIBADANBgkqhkiG9w0BAQEFAASCBKYwggSiAgEAAoIBAQDKRGXqvNcJwJwA
Hf9gVwB1g5B6Yq3N9mM2vByP0rG4Hk4F3m5k7lR8mWwJpB9Qxw7m0cS1d2e3f4g5
-----END PRIVATE KEY-----
EOF

  case_run_declarest resource save "${private_key_path}" --payload "${private_key_file}" --secret --force
  case_expect_success

  case_run_declarest resource apply "${private_key_path}"
  case_expect_success

  case_run_declarest resource get "${private_key_path}" --source managed-server -o json
  case_expect_success
  if ! jq -e '. == "{{secret .}}"' <<<"${CASE_LAST_STDOUT}" >/dev/null; then
    printf 'expected managed-server output to redact the whole-resource private key\n' >&2
    printf 'output: %s\n' "${CASE_LAST_OUTPUT}" >&2
    return 1
  fi
}
