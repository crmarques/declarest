#!/usr/bin/env bash

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
SCRIPTS_DIR="$(cd "$SCRIPT_DIR/.." && pwd)"
source "$SCRIPTS_DIR/lib/env.sh"
source "$SCRIPTS_DIR/lib/logging.sh"
source "$SCRIPTS_DIR/lib/text.sh"
source "$SCRIPTS_DIR/lib/cli.sh"

CONTEXT="$DECLAREST_CONTEXT_FILE"
export RUN_LOG="${RUN_LOG:-$DECLAREST_LOG_DIR/run-e2e_$(date -Iseconds | tr ':' '-').log}"

SECRET_CLIENT_PATH="/admin/realms/publico/clients/testB"
SECRET_LDAP_PATH="/admin/realms/publico/user-registry/ldap-test"
SECRET_CLIENT_KEY="secret"
SECRET_LDAP_KEY="config.bindCredential[0]"
SECRET_PLACEHOLDER="{{secret .}}"

RESOURCE_ADD_SOURCE="/admin/realms/publico"
RESOURCE_ADD_SUFFIX="${DECLAREST_RUN_ID:-$(date -u +%Y%m%dT%H%M%SZ)}"
RESOURCE_ADD_TARGET="/admin/realms/publico-copy-${RESOURCE_ADD_SUFFIX}"
RESOURCE_ADD_OVERRIDE_REALM="realm=publico-copy-${RESOURCE_ADD_SUFFIX}"
RESOURCE_ADD_OVERRIDE_JSON='{"displayName":"Publico Copy","attributes":{"copiedBy":"resource add command"}}'

secret_store_type="${DECLAREST_SECRET_STORE_TYPE:-file}"
secret_store_type="${secret_store_type,,}"
secrets_enabled=1
if [[ "$secret_store_type" == "none" ]]; then
    secrets_enabled=0
fi
e2e_profile="${DECLAREST_E2E_PROFILE:-complete}"
e2e_profile="${e2e_profile,,}"
run_extended=1
run_roundtrip=1
if [[ "$e2e_profile" != "complete" ]]; then
    run_extended=0
    run_roundtrip=0
fi
reduced_path_limit=2

require_jq() {
    if ! command -v jq >/dev/null 2>&1; then
        log_line "jq is required for secret tests"
        echo "jq is required for secret tests" >&2
        exit 1
    fi
}

test_resource_add_command() {
    local force_args=()
    local target_file
    target_file="$(resource_file_for_path "$RESOURCE_ADD_TARGET")"
    if [[ -f "$target_file" ]]; then
        force_args+=(--force)
    fi

    log_line "Testing resource add command by copying $RESOURCE_ADD_SOURCE to $RESOURCE_ADD_TARGET"
    run_cli "add resource copy" resource add --path "$RESOURCE_ADD_TARGET" --from-path "$RESOURCE_ADD_SOURCE" \
        --override "$RESOURCE_ADD_OVERRIDE_REALM" \
        --override "$RESOURCE_ADD_OVERRIDE_JSON" \
        "${force_args[@]}"
}

resource_file_for_path() {
    local path="$1"
    local trimmed="${path#/}"
    if [[ -z "$trimmed" ]]; then
        printf "%s/resource.json" "$DECLAREST_REPO_DIR"
    else
        printf "%s/%s/resource.json" "$DECLAREST_REPO_DIR" "$trimmed"
    fi
}

select_representative_paths() {
    local -n in_paths="$1"
    local -n out_paths="$2"
    local -a preferred=("$SECRET_CLIENT_PATH" "$SECRET_LDAP_PATH" "/admin/realms/publico")
    local path candidate
    out_paths=()
    for candidate in "${preferred[@]}"; do
        for path in "${in_paths[@]}"; do
            if [[ "$path" == "$candidate" ]]; then
                out_paths+=("$path")
                break
            fi
        done
    done
    if [[ ${#out_paths[@]} -eq 0 ]]; then
        out_paths=("${in_paths[@]:0:$reduced_path_limit}")
        return 0
    fi
    local -a unique=()
    local -A seen=()
    for path in "${out_paths[@]}"; do
        if [[ -n "$path" && -z "${seen[$path]+x}" ]]; then
            unique+=("$path")
            seen["$path"]=1
        fi
    done
    out_paths=("${unique[@]}")
    if [[ "$reduced_path_limit" -gt 0 && ${#out_paths[@]} -gt "$reduced_path_limit" ]]; then
        out_paths=("${out_paths[@]:0:$reduced_path_limit}")
    fi
}

seed_secrets_via_cli() {
    run_cli "secret init" secret init

    run_cli "secret add client secret" secret add --path "$SECRET_CLIENT_PATH" --key "$SECRET_CLIENT_KEY" --value "$DECLAREST_TEST_CLIENT_SECRET"
    run_cli "secret add ldap bind credential" secret add --path "$SECRET_LDAP_PATH" --key "$SECRET_LDAP_KEY" --value "$DECLAREST_TEST_LDAP_BIND_CREDENTIAL"

    local client_secret
    client_secret=$(capture_cli "secret get client secret" --no-status secret get --path "$SECRET_CLIENT_PATH" --key "$SECRET_CLIENT_KEY")
    if [[ "$client_secret" != "$DECLAREST_TEST_CLIENT_SECRET" ]]; then
        log_line "Secret check failed: client secret mismatch"
        echo "Expected client secret from secret get" >&2
        exit 1
    fi

    local ldap_secret
    ldap_secret=$(capture_cli "secret get ldap bind credential" --no-status secret get --path "$SECRET_LDAP_PATH" --key "$SECRET_LDAP_KEY")
    if [[ "$ldap_secret" != "$DECLAREST_TEST_LDAP_BIND_CREDENTIAL" ]]; then
        log_line "Secret check failed: ldap bind credential mismatch"
        echo "Expected bind credential from secret get" >&2
        exit 1
    fi

    local list_output
    list_output=$(capture_cli "secret list resources" --no-status secret list --paths-only)
    local -a list_entries
    split_lines_nonempty list_entries "$list_output"
    local found_client=0
    local found_ldap=0
    for entry in "${list_entries[@]}"; do
        entry="$(trim_whitespace "$entry")"
        entry="${entry%:}"
        [[ "$entry" == "$SECRET_CLIENT_PATH" ]] && found_client=1
        [[ "$entry" == "$SECRET_LDAP_PATH" ]] && found_ldap=1
    done
    if [[ $found_client -ne 1 || $found_ldap -ne 1 ]]; then
        log_line "Secret check failed: secret list missing resources"
        echo "Expected resources in secret list output" >&2
        exit 1
    fi

    local keys_output
    keys_output=$(capture_cli "secret list keys" --no-status secret list --path "$SECRET_LDAP_PATH")
    local -a key_entries
    split_lines_nonempty key_entries "$keys_output"
    local found_key=0
    for entry in "${key_entries[@]}"; do
        entry="$(trim_whitespace "$entry")"
        [[ -z "$entry" ]] && continue
        [[ "$entry" == *: ]] && continue
        [[ "$entry" == "$SECRET_LDAP_KEY" ]] && found_key=1
    done
    if [[ $found_key -ne 1 ]]; then
        log_line "Secret check failed: secret list keys missing $SECRET_LDAP_KEY"
        echo "Expected $SECRET_LDAP_KEY in secret list keys output" >&2
        exit 1
    fi

    run_cli "secret delete ldap bind credential" secret delete --path "$SECRET_LDAP_PATH" --key "$SECRET_LDAP_KEY" --yes

    set +e
    deleted_output=$(capture_cli_all "secret get deleted ldap (expected fail)" --no-status secret get --path "$SECRET_LDAP_PATH" --key "$SECRET_LDAP_KEY")
    deleted_status=$?
    set -e
    if [[ $deleted_status -eq 0 ]]; then
        log_line "Secret check failed: secret get after delete succeeded"
        echo "Expected secret get to fail after delete" >&2
        exit 1
    fi
    if [[ -z "$deleted_output" ]]; then
        log_line "Secret check failed: missing error output after delete"
        echo "Expected error output after secret delete" >&2
        exit 1
    fi

    run_cli "secret add ldap bind credential (restore)" secret add --path "$SECRET_LDAP_PATH" --key "$SECRET_LDAP_KEY" --value "$DECLAREST_TEST_LDAP_BIND_CREDENTIAL"
}

test_ad_hoc_command() {
    local path="/admin/realms/publico"
    log_line "Testing ad-hoc command for $path"
    local attempts="${KEYCLOAK_RETRY_ATTEMPTS:-10}"
    local delay="${KEYCLOAK_RETRY_DELAY:-2}"
    if ! run_cli_retry_transient "ad-hoc get realm" "$attempts" "$delay" --no-status ad-hoc get --path "$path"; then
        log_line "ad-hoc command failed after retries"
        echo "Expected ad-hoc get to succeed" >&2
        exit 1
    fi
    local output="$CLI_LAST_OUTPUT"
    if ! grep -Eq '"realm"[[:space:]]*:[[:space:]]*"publico"' <<<"$output"; then
        log_line "ad-hoc command failed: unexpected output"
        echo "Expected ad-hoc get to include the realm name" >&2
        exit 1
    fi
}

test_completion_metadata_fragment() {
    log_line "Testing resource path completion includes metadata fragments"
    local output
    output=$(capture_cli "resource completion metadata" --no-status __complete resource get /admin/realms/publico/ "")
    if ! grep -F "/admin/realms/publico/user-store/" <<<"$output"; then
        log_line "Completion output missing /admin/realms/publico/user-store/"
        echo "Expected metadata-defined fragment /admin/realms/publico/user-store/ in completion results" >&2
        exit 1
    fi
}

test_secret_export_import() {
    local export_file="$DECLAREST_WORK_DIR/secret-export.csv"
    mkdir -p "$(dirname "$export_file")"

    local csv_output
    csv_output=$(capture_cli "secret export ldap" --no-status secret export "$SECRET_LDAP_PATH")
    printf "%s" "$csv_output" > "$export_file"

    run_cli "secret delete ldap for import test" secret delete --path "$SECRET_LDAP_PATH" --key "$SECRET_LDAP_KEY" --yes

    run_cli "secret import ldap from CSV" secret import "$export_file"

    local imported
    imported=$(capture_cli "secret get ldap after import" --no-status secret get --path "$SECRET_LDAP_PATH" --key "$SECRET_LDAP_KEY")
    if [[ "$imported" != "$DECLAREST_TEST_LDAP_BIND_CREDENTIAL" ]]; then
        log_line "Secret export/import failed: unexpected LDAP secret after import"
        echo "Expected LDAP secret value to match exported value" >&2
        exit 1
    fi

    set +e
    local conflict_output
    conflict_output=$(capture_cli_all "secret import conflict" secret import --file "$export_file")
    local conflict_status=$?
    set -e
    if [[ $conflict_status -eq 0 ]]; then
        log_line "Secret import conflict test failed: import unexpectedly succeeded"
        echo "Expected import to fail without --force" >&2
        exit 1
    fi
    if ! grep -q "Import would overwrite existing secrets" <<<"$conflict_output"; then
        log_line "Secret import conflict test failed: missing conflict message"
        echo "Expected conflict message when importing existing secrets" >&2
        exit 1
    fi

    local override_value="declarest-ldap-override"
    cat <<EOF > "$export_file"
path,key,value
$SECRET_LDAP_PATH,$SECRET_LDAP_KEY,$override_value
EOF

    run_cli "secret import force override" secret import --file "$export_file" --force

    local forced
    forced=$(capture_cli "secret get ldap override" --no-status secret get --path "$SECRET_LDAP_PATH" --key "$SECRET_LDAP_KEY")
    if [[ "$forced" != "$override_value" ]]; then
        log_line "Secret import force test failed: unexpected LDAP secret value"
        echo "Expected LDAP secret value to update after --force import" >&2
        exit 1
    fi

    run_cli "secret add ldap bind credential (restore)" secret add --path "$SECRET_LDAP_PATH" --key "$SECRET_LDAP_KEY" --value "$DECLAREST_TEST_LDAP_BIND_CREDENTIAL"
}

phase() {
    log_line "PHASE $1"
}

wait_for_keycloak() {
    local url="http://localhost:${KEYCLOAK_HTTP_PORT}/realms/master"
    local attempts=${KEYCLOAK_WAIT_ATTEMPTS:-60}
    local delay=${KEYCLOAK_WAIT_DELAY:-2}

    log_line "Waiting for Keycloak readiness at $url (${attempts} attempts, ${delay}s delay)"
    for ((i=1; i<=attempts; i++)); do
        if curl -sk --fail "$url" >/dev/null 2>&1; then
            log_line "Keycloak is ready after attempt ${i}"
            return 0
        fi
        if (( i % 10 == 0 )); then
            log_line "Still waiting for Keycloak (${i}/${attempts})"
        fi
        sleep "$delay"
    done

    log_line "Keycloak did not become ready in time"
    return 1
}

cli_retry_on_failure() {
    wait_for_keycloak || true
}

create_with_retry() {
    local path="$1"
    local attempts="${KEYCLOAK_RETRY_ATTEMPTS:-10}"
    local delay="${KEYCLOAK_RETRY_DELAY:-2}"

    if run_cli_retry_transient "create $path" "$attempts" "$delay" resource create --path "$path" --sync; then
        return 0
    fi

    if grep -Eq "409 Conflict" <<<"${CLI_LAST_OUTPUT:-}"; then
        log_line "Create returned 409 Conflict for $path; falling back to apply"
        run_cli_retry_transient "apply $path (after create conflict)" "$attempts" "$delay" resource apply --path "$path" --sync
        return $?
    fi

    return "${CLI_LAST_STATUS:-1}"
}

update_with_retry() {
    local path="$1"
    local attempts="${KEYCLOAK_RETRY_ATTEMPTS:-10}"
    local delay="${KEYCLOAK_RETRY_DELAY:-2}"

    if run_cli_retry_transient "update $path" "$attempts" "$delay" resource update --path "$path" --sync; then
        return 0
    fi

    if grep -Eq "404 Not Found" <<<"${CLI_LAST_OUTPUT:-}"; then
        log_line "Update returned 404 Not Found for $path; falling back to apply"
        run_cli_retry_transient "apply $path (after update 404)" "$attempts" "$delay" resource apply --path "$path" --sync
        return $?
    fi

    return "${CLI_LAST_STATUS:-1}"
}

refresh_master_if_needed() {
    local path="$1"
    if [[ "$path" == "/admin/realms/master" ]]; then
        run_cli "refresh $path" resource save --path "$path" --force || true
    fi
}

diff_all() {
    local tag="$1"
    for local in "${local_paths[@]}"; do
        if ! run_cli_retry_transient "diff $local [$tag]" "${KEYCLOAK_RETRY_ATTEMPTS:-10}" "${KEYCLOAK_RETRY_DELAY:-2}" resource diff --path "$local" --fail; then
            capture_cli "diff patch $local [$tag]" resource diff --path "$local" >/dev/null || true
            log_line "Diff failed for $local during $tag"
            return 1
        fi
    done
}

log_line "Declarest workflow starting (context: $CONTEXT)"
resource_add_enabled="$run_extended"

if [[ $secrets_enabled -eq 1 ]]; then
    phase "Seeding secrets via CLI"
    seed_secrets_via_cli

    if [[ $run_extended -eq 1 ]]; then
        phase "Secret export/import checks"
        test_secret_export_import
    fi
else
    log_line "Secret store disabled; skipping secret seed"
fi

if [[ $run_extended -eq 1 ]]; then
    phase "Testing resource add command"
    test_resource_add_command
fi

phase "Discovering repository resources"
local_output=$(capture_cli "list repository resources" resource list --repo)
split_lines_nonempty local_paths "$local_output"
found_resource_add=0
if [[ "$resource_add_enabled" -eq 1 ]]; then
    for entry in "${local_paths[@]}"; do
        if [[ "$entry" == "$RESOURCE_ADD_TARGET" ]]; then
            found_resource_add=1
            break
        fi
    done
    if [[ $found_resource_add -ne 1 ]]; then
        log_line "Resource add command failed to add $RESOURCE_ADD_TARGET"
        echo "Expected repository to contain $RESOURCE_ADD_TARGET after resource add" >&2
        exit 1
    fi
fi
if [[ ${#local_paths[@]} -eq 0 ]]; then
    log_line "No repository resources found; aborting."
    exit 1
fi

local_paths_total=${#local_paths[@]}
if [[ $resource_add_enabled -eq 1 && $found_resource_add -eq 1 ]]; then
    filtered_paths=()
    for entry in "${local_paths[@]}"; do
        if [[ "$entry" == "$RESOURCE_ADD_TARGET" ]]; then
            continue
        fi
        filtered_paths+=("$entry")
    done
    local_paths=("${filtered_paths[@]}")
    log_line "Found ${#local_paths[@]} repository resources (excluded $RESOURCE_ADD_TARGET from downstream sync; ${local_paths_total} total)"
else
    log_line "Found ${#local_paths[@]} repository resources"
fi

if [[ "$e2e_profile" == "reduced" ]]; then
    reduced_paths=()
    select_representative_paths local_paths reduced_paths
    if [[ ${#reduced_paths[@]} -gt 0 ]]; then
        log_line "Reduced profile selected ${#reduced_paths[@]} of ${#local_paths[@]} repository resources"
        local_paths=("${reduced_paths[@]}")
    fi
fi

if [[ ${#local_paths[@]} -eq 0 ]]; then
    log_line "No repository resources selected; aborting."
    exit 1
fi

sort_paths_by_depth local_paths local_paths_parent_first asc
sort_paths_by_depth local_paths local_paths_child_first desc

if [[ $run_extended -eq 1 ]]; then
    phase "Testing path completion fragments"
    test_completion_metadata_fragment
fi

phase "Creating remote resources"
for local in "${local_paths_parent_first[@]}"; do
    if ! create_with_retry "$local"; then
        log_line "Create failed for $local"
        exit 1
    fi
    refresh_master_if_needed "$local"
done
log_line "All resources created remotely"

phase "Updating resources in Keycloak"
for local in "${local_paths_parent_first[@]}"; do
    if ! update_with_retry "$local"; then
        log_line "Update failed for $local"
        exit 1
    fi
    refresh_master_if_needed "$local"
done
log_line "Resources updated in Keycloak"

phase "Applying resources in Keycloak"
for local in "${local_paths_parent_first[@]}"; do
    if ! run_cli_retry_transient "apply $local" "${KEYCLOAK_RETRY_ATTEMPTS:-10}" "${KEYCLOAK_RETRY_DELAY:-2}" resource apply --path "$local" --sync; then
        log_line "Apply failed for $local"
        exit 1
    fi
done
log_line "Resources applied in Keycloak"

if [[ $run_roundtrip -eq 1 ]]; then
    phase "Deleting remote resources"
    for path in "${local_paths_child_first[@]}"; do
        if ! run_cli_retry_transient "delete remote $path" "${KEYCLOAK_RETRY_ATTEMPTS:-10}" "${KEYCLOAK_RETRY_DELAY:-2}" resource delete --path "$path" --remote --yes --repo=false; then
            log_line "Remote delete failed for $path"
            exit 1
        fi
    done
    log_line "Remote resources deleted"

    phase "Waiting for Keycloak after deletes"
    wait_for_keycloak || true

    phase "Re-creating resources in Keycloak"
    for local in "${local_paths_parent_first[@]}"; do
        if ! create_with_retry "$local"; then
            log_line "Recreate failed for $local"
            exit 1
        fi
        refresh_master_if_needed "$local"
    done
    log_line "Remote resources recreated"

    phase "Deleting repository resources"
    for local in "${local_paths_child_first[@]}"; do
        if ! run_cli "delete repository $local" resource delete --path "$local" --repo --remote=false --yes; then
            log_line "Repository delete failed for $local"
            exit 1
        fi
    done
    log_line "Repository resources deleted"

    if [[ $found_resource_add -eq 1 ]]; then
        phase "Cleaning resource add artifact"
        if ! run_cli "delete added resource" resource delete --path "$RESOURCE_ADD_TARGET" --repo --remote=false --yes; then
            log_line "Resource add artifact delete failed for $RESOURCE_ADD_TARGET"
            exit 1
        fi
        log_line "Resource add artifact $RESOURCE_ADD_TARGET removed from repository"
    fi

    phase "Retrieving resources from Keycloak"
    for path in "${local_paths_parent_first[@]}"; do
        if ! run_cli_retry_transient "get $path" "${KEYCLOAK_RETRY_ATTEMPTS:-10}" "${KEYCLOAK_RETRY_DELAY:-2}" --no-status resource save --path "$path" --force; then
            log_line "Get failed for $path"
            exit 1
        fi
    done
    log_line "Resources re-downloaded"

    phase "Fetching collections"
    collections=(
        "/admin/realms/publico/user-registry/ldap-test/mappers/"
    )
    for coll in "${collections[@]}"; do
        if ! run_cli_retry_transient "get collection $coll" "${KEYCLOAK_RETRY_ATTEMPTS:-10}" "${KEYCLOAK_RETRY_DELAY:-2}" resource save --path "$coll"; then
            log_line "Collection get failed for $coll"
            exit 1
        fi
    done
    log_line "Collections saved"

    if [[ $run_extended -eq 1 ]]; then
        phase "Testing ad-hoc command"
        test_ad_hoc_command
    fi

    if [[ $secrets_enabled -eq 1 ]]; then
        phase "Restoring secrets for diff"
        for secret_path in "$SECRET_CLIENT_PATH" "$SECRET_LDAP_PATH"; do
            if ! run_cli_retry_transient "restore secret $secret_path" "${KEYCLOAK_RETRY_ATTEMPTS:-10}" "${KEYCLOAK_RETRY_DELAY:-2}" resource save --path "$secret_path" --with-secrets --force; then
                log_line "Secret restore failed for $secret_path"
                exit 1
            fi
        done
    fi

    phase "Final diff of all resources"
    diff_all "final"
    log_line "Resources are synced after final diff"

    if [[ $secrets_enabled -eq 1 ]]; then
        phase "Secret management checks"
        require_jq

        client_with_secrets=$(capture_cli "get client secret with secrets" --no-status resource get --path "$SECRET_CLIENT_PATH" --with-secrets)
        client_secret_value=$(jq -r '.secret // empty' <<<"$client_with_secrets")
        if [[ -z "$client_secret_value" || "$client_secret_value" == "$SECRET_PLACEHOLDER" ]]; then
            log_line "Secret check failed: missing client secret"
            echo "Expected client secret in remote payload" >&2
            exit 1
        fi

        if [[ "${DECLAREST_SECRET_STORE_TYPE:-file}" == "file" && ! -s "$DECLAREST_SECRETS_FILE" ]]; then
            log_line "Secret check failed: secrets file missing at $DECLAREST_SECRETS_FILE"
            echo "Expected secrets file at $DECLAREST_SECRETS_FILE" >&2
            exit 1
        fi

        run_cli "save client secret (no with-secrets)" resource save --path "$SECRET_CLIENT_PATH" --force
        run_cli "save ldap bind credential (no with-secrets)" resource save --path "$SECRET_LDAP_PATH" --force

        if [[ $run_extended -eq 1 ]]; then
            client_with_secrets_again=$(capture_cli "get client secret after save" --no-status resource get --path "$SECRET_CLIENT_PATH" --with-secrets)
            client_secret_again=$(jq -r '.secret // empty' <<<"$client_with_secrets_again")
            if [[ -z "$client_secret_again" || "$client_secret_again" == "$SECRET_PLACEHOLDER" ]]; then
                log_line "Secret check failed: with-secrets missing client secret"
                echo "Expected client secret in with-secrets output" >&2
                exit 1
            fi

            set +e
            save_with_secrets_output=$(capture_cli_all "save with-secrets (expected fail)" --no-status resource save --path "$SECRET_CLIENT_PATH" --with-secrets)
            save_with_secrets_status=$?
            set -e
            if [[ $save_with_secrets_status -eq 0 ]]; then
                log_line "Secret check failed: save with-secrets unexpectedly succeeded"
                echo "Expected save with-secrets to fail without --force" >&2
                exit 1
            fi
            if ! grep -q "refusing to save plaintext secrets" <<<"$save_with_secrets_output"; then
                log_line "Secret check failed: missing refusal message"
                echo "Expected refusal message when saving with --with-secrets" >&2
                exit 1
            fi

            if ! run_cli_retry_transient "apply client with secrets" "${KEYCLOAK_RETRY_ATTEMPTS:-10}" "${KEYCLOAK_RETRY_DELAY:-2}" resource apply --path "$SECRET_CLIENT_PATH" --sync; then
                log_line "Apply failed for $SECRET_CLIENT_PATH after secret masking"
                exit 1
            fi

            client_after_apply=$(capture_cli "get client secret after apply" --no-status resource get --path "$SECRET_CLIENT_PATH" --with-secrets)
            client_secret_after_apply=$(jq -r '.secret // empty' <<<"$client_after_apply")
            if [[ -z "$client_secret_after_apply" || "$client_secret_after_apply" == "$SECRET_PLACEHOLDER" ]]; then
                log_line "Secret check failed: with-secrets missing client secret after apply"
                echo "Expected client secret after apply" >&2
                exit 1
            fi
            if [[ "$client_secret_after_apply" != "$client_secret_again" ]]; then
                log_line "Secret check failed: client secret changed after apply"
                echo "Client secret changed after apply" >&2
                exit 1
            fi
        fi

        client_file="$(resource_file_for_path "$SECRET_CLIENT_PATH")"
        client_local_secret=$(jq -r '.secret // empty' "$client_file")
        if [[ "$client_local_secret" != "$SECRET_PLACEHOLDER" ]]; then
            log_line "Secret check failed: client secret not masked in repo"
            echo "Expected placeholder in $client_file" >&2
            exit 1
        fi

        ldap_file="$(resource_file_for_path "$SECRET_LDAP_PATH")"
        ldap_local_secret=$(jq -r '.config.bindCredential[0] // empty' "$ldap_file")
        if [[ "$ldap_local_secret" != "$SECRET_PLACEHOLDER" ]]; then
            log_line "Secret check failed: bindCredential not masked in repo"
            echo "Expected placeholder in $ldap_file" >&2
            exit 1
        fi

        client_masked_output=$(capture_cli "get client secret masked" --no-status resource get --path "$SECRET_CLIENT_PATH")
        client_masked_value=$(jq -r '.secret // empty' <<<"$client_masked_output")
        if [[ "$client_masked_value" != "$SECRET_PLACEHOLDER" ]]; then
            log_line "Secret check failed: masked get returned unexpected value"
            echo "Expected placeholder in masked output" >&2
            exit 1
        fi

        log_line "Secret management checks completed"
    fi
fi
log_line "Declarest resource run completed successfully"
