#!/usr/bin/env bash
set -euo pipefail

CASE_ID='openapi-spec-version-compatibility'
CASE_SCOPE='corner'
CASE_REQUIRES='has-managed-server'

case_run_declarest_with_context() {
  local context_file=$1
  shift

  local output

  set +e
  output=$(DECLAREST_CONTEXTS_FILE="${context_file}" "${E2E_BIN}" --context "${E2E_CONTEXT_NAME}" "$@" 2>&1)
  CASE_LAST_STATUS=$?
  set -e

  CASE_LAST_OUTPUT="${output}"
  return 0
}

case_write_context_with_openapi() {
  local source_context=$1
  local destination_context=$2
  local openapi_spec_path=$3

  cp "${source_context}" "${destination_context}"

  if grep -Eq '^[[:space:]]*openapi:' "${destination_context}"; then
    sed -i -E "s|^[[:space:]]*openapi:.*$|        openapi: '${openapi_spec_path}'|" "${destination_context}"
    return 0
  fi

  if ! grep -Eq '^[[:space:]]*base-url:' "${destination_context}"; then
    printf 'failed to patch context openapi path: base-url field not found in %s\n' "${destination_context}" >&2
    return 1
  fi

  awk -v openapi_spec_path="${openapi_spec_path}" '
    {
      print $0
      if (!inserted && $0 ~ /^[[:space:]]*base-url:/) {
        print "        openapi: \047" openapi_spec_path "\047"
        inserted = 1
      }
    }
  ' "${destination_context}" >"${destination_context}.tmp"
  mv "${destination_context}.tmp" "${destination_context}"
}

case_write_openapi3_spec() {
  local spec_file=$1
  cat >"${spec_file}" <<'EOF'
openapi: 3.0.3
paths:
  /e2e/openapi-v3-check:
    post:
      requestBody:
        required: true
        content:
          application/json:
            schema:
              type: object
              required:
                - name
              properties:
                name:
                  type: string
      responses:
        "200":
          description: ok
          content:
            application/json:
              schema:
                type: object
EOF
}

case_write_swagger2_spec() {
  local spec_file=$1
  cat >"${spec_file}" <<'EOF'
swagger: "2.0"
consumes:
  - application/json
produces:
  - application/json
paths:
  /e2e/swagger-v2-check:
    post:
      parameters:
        - in: body
          name: body
          required: true
          schema:
            type: object
            required:
              - name
            properties:
              name:
                type: string
      responses:
        "200":
          description: ok
          schema:
            type: object
EOF
}

case_write_validation_metadata() {
  local metadata_file=$1
  local operation_path=$2

  cat >"${metadata_file}" <<EOF
{
  "operations": {
    "create": {
      "path": "${operation_path}",
      "validate": {
        "schemaRef": "openapi:request-body"
      }
    }
  }
}
EOF
}

case_assert_schema_ref_validation() {
  local label=$1
  local openapi_spec_path=$2
  local logical_path=$3

  local temp_context="${E2E_CASE_TMP_DIR}/contexts-${label}.yaml"
  local metadata_file="${E2E_CASE_TMP_DIR}/metadata-${label}.json"
  local payload_file="${E2E_CASE_TMP_DIR}/payload-${label}.json"

  case_write_context_with_openapi "${E2E_CONTEXT_FILE}" "${temp_context}" "${openapi_spec_path}"
  case_write_validation_metadata "${metadata_file}" "${logical_path}"
  case_write_json "${payload_file}" '{}'

  case_run_declarest_with_context "${temp_context}" metadata set "${logical_path}/_" --payload "${metadata_file}"
  case_expect_success

  case_run_declarest_with_context "${temp_context}" resource request post "${logical_path}" --payload "${payload_file}"
  case_expect_failure
  case_expect_output_contains 'openapi:request-body'
  case_expect_output_contains 'missing required property'

  case_run_declarest_with_context "${temp_context}" metadata unset "${logical_path}/_"
  case_expect_success
}

case_run() {
  local openapi3_spec_file="${E2E_CASE_TMP_DIR}/openapi3.yaml"
  local swagger2_spec_file="${E2E_CASE_TMP_DIR}/swagger2.yaml"

  case_write_openapi3_spec "${openapi3_spec_file}"
  case_write_swagger2_spec "${swagger2_spec_file}"

  case_assert_schema_ref_validation "openapi3" "${openapi3_spec_file}" "/e2e/openapi-v3-check"
  case_assert_schema_ref_validation "swagger2" "${swagger2_spec_file}" "/e2e/swagger-v2-check"
}
