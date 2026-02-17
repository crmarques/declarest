#!/usr/bin/env bash

e2e_apply_profile_defaults() {
  # Profiles share the same component defaults from args parsing.
  # Manual mode only changes workload behavior (handoff vs automated cases).
  return 0
}

e2e_validate_profile_rules() {
  if [[ "${E2E_PROFILE}" != 'manual' ]]; then
    return 0
  fi

  if [[ "${E2E_RESOURCE_SERVER_CONNECTION}" != 'local' && "${E2E_RESOURCE_SERVER}" != 'none' ]]; then
    e2e_die 'manual profile is local-instantiable only; resource-server connection must be local'
    return 1
  fi

  if [[ "${E2E_GIT_PROVIDER_CONNECTION}" != 'local' && -n "${E2E_GIT_PROVIDER}" ]]; then
    e2e_die 'manual profile is local-instantiable only; git-provider connection must be local'
    return 1
  fi

  if [[ "${E2E_SECRET_PROVIDER_CONNECTION}" != 'local' && "${E2E_SECRET_PROVIDER}" != 'none' ]]; then
    e2e_die 'manual profile is local-instantiable only; secret-provider connection must be local'
    return 1
  fi
}

e2e_profile_scopes() {
  case "${E2E_PROFILE}" in
    basic)
      printf 'main\n'
      ;;
    full)
      printf 'main\ncorner\n'
      ;;
    manual)
      ;;
  esac
}

e2e_manual_handoff_print() {
  local context_name=$1
  cat <<EOFH
Manual profile is ready.

Run ID:
  ${E2E_RUN_ID:-n/a}

Context file:
  ${E2E_CONTEXT_FILE}

To use it in another shell:
  export DECLAREST_E2E_RUN_DIR=${E2E_RUN_DIR@Q}
  export DECLAREST_CONTEXTS_FILE="\${DECLAREST_E2E_RUN_DIR}/contexts.yaml"
  export PATH="\${DECLAREST_E2E_RUN_DIR}/bin:\$PATH"
  export DECLAREST_E2E_CONTEXT=${context_name@Q}
  declarest --context "\${DECLAREST_E2E_CONTEXT}" config show
  declarest --context "\${DECLAREST_E2E_CONTEXT}" repo status -o json
  declarest --context "\${DECLAREST_E2E_CONTEXT}" resource list / --source local -o json

This execution finished and runtime resources were kept.
To stop and remove this execution:
  ./run-e2e.sh --clean ${E2E_RUN_ID:-<run-id>}
To stop and remove all executions:
  ./run-e2e.sh --clean-all
EOFH
}

e2e_profile_manual_handoff() {
  local context_name=$1
  e2e_manual_handoff_print "${context_name}"
}
