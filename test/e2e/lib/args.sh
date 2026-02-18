#!/usr/bin/env bash

declare -Ag E2E_EXPLICIT

E2E_RESOURCE_SERVER='simple-api-server'
E2E_RESOURCE_SERVER_CONNECTION='local'
E2E_RESOURCE_SERVER_BASIC_AUTH='false'
E2E_RESOURCE_SERVER_OAUTH2='true'
E2E_RESOURCE_SERVER_MTLS='false'
E2E_REPO_TYPE='filesystem'
E2E_GIT_PROVIDER=''
E2E_GIT_PROVIDER_CONNECTION='local'
E2E_SECRET_PROVIDER='file'
E2E_SECRET_PROVIDER_CONNECTION='local'
E2E_PROFILE='basic'
E2E_LIST_COMPONENTS=0
E2E_KEEP_RUNTIME=0
E2E_VERBOSE=0
E2E_CLEAN_RUN_ID=''
E2E_CLEAN_ALL=0

# shellcheck disable=SC2034
E2E_SELECTED_BY_PROFILE_DEFAULT=0

e2e_mark_explicit() {
  local key=$1
  E2E_EXPLICIT["${key}"]=1
}

e2e_is_explicit() {
  local key=$1
  [[ "${E2E_EXPLICIT[${key}]:-0}" == '1' ]]
}

e2e_has_help_flag() {
  local arg
  for arg in "$@"; do
    case "${arg}" in
      -h|--help)
        return 0
        ;;
    esac
  done
  return 1
}

e2e_profile_from_cli_args() {
  local profile='basic'

  while (($# > 0)); do
    case "$1" in
      --profile)
        [[ $# -ge 2 ]] || break
        profile=$2
        shift 2
        ;;
      *)
        shift
        ;;
    esac
  done

  printf '%s\n' "${profile}"
}

e2e_valid_component_name() {
  local value=$1
  [[ "${value}" =~ ^[A-Za-z0-9][A-Za-z0-9._-]*$ ]]
}

e2e_validate_component_arg() {
  local flag=$1
  local value=$2
  local allow_none=${3:-false}
  local allow_empty=${4:-false}

  if [[ -z "${value}" ]]; then
    if [[ "${allow_empty}" == 'true' ]]; then
      return 0
    fi
    e2e_die "invalid ${flag} value: empty"
    return 1
  fi

  if [[ "${allow_none}" == 'true' && "${value}" == 'none' ]]; then
    return 0
  fi

  if ! e2e_valid_component_name "${value}"; then
    if [[ "${allow_none}" == 'true' ]]; then
      e2e_die "invalid ${flag} value: ${value} (expected component name or none)"
    else
      e2e_die "invalid ${flag} value: ${value} (expected component name)"
    fi
    return 1
  fi

  return 0
}

e2e_parse_bool_value() {
  local flag=$1
  local raw_value=$2

  case "${raw_value,,}" in
    true|1|yes|on)
      printf 'true\n'
      ;;
    false|0|no|off)
      printf 'false\n'
      ;;
    *)
      e2e_die "invalid ${flag} value: ${raw_value} (allowed: true, false)"
      return 1
      ;;
  esac
}

e2e_usage() {
  cat <<'USAGE'
Usage: ./run-e2e.sh [flags]

Objective:
  Run Declarest end-to-end workloads against a selectable component stack and profile to verify resource workflows.

Profiles (required, defaults to basic when omitted):
  --profile <basic|full|manual>                   default: basic
    basic   : For automated ci-style runs, execute cases tagged "main" against the default stack.
    full    : Also include "corner" cases that exercise less-common paths and additional components.
    manual  : Only boot the selected components, emit setup/reset shell scripts, and exit so you can drive Declarest commands interactively (requires all connections to stay local and the selected resource/secret components to support local execution).

Component selection (choose values for each flag; see notes below):
  --resource-server <simple-api-server|keycloak|rundeck|vault|none>    default: simple-api-server
    simple-api-server : Lightweight JSON API with optional basic-auth, OAuth2, and mTLS enforcement.
    keycloak          : Keycloak Admin REST API that enforces OAuth2 client-credentials tokens.
    rundeck           : Rundeck HTTP API surface.
    vault             : HashiCorp Vault HTTP API acting as the managed resource.
    none              : Skip provisioning any resource server; remote operations become no-ops or are validated against the repository only.
  --resource-server-connection <local|remote>           default: local
    local  : Start the chosen resource server locally via the E2E compose/runtime fixtures.
    remote : Assume the server already exists and is reachable via the configured connection details.
  --resource-server-basic-auth [<true|false>]         default: false
    true  : Enable HTTP basic authentication on the resource server (supported by components that publish basic-auth support, such as simple-api-server).
    false : Disable the feature, useful when the stack enforces other auth flows.
  --resource-server-oauth2 [<true|false>]             default: true
    true  : Enable OAuth2 token issuance for the server.
    false : Disable OAuth2 so the stack can rely on alternate auth paths.
  --resource-server-mtls [<true|false>]               default: false
    true  : Require client certificate authentication when the component advertises mTLS support.
    false : Run without client certificate validation.
  --repo-type <filesystem|git>                        default: filesystem
    filesystem : Use the local filesystem repository backend.
    git        : Use the git repository backend (requires a git provider).
  --git-provider <git|github|gitlab>                  default: git when --repo-type git (none otherwise)
    git    : Local file:// git remote provider (bundled in the repo fixture).
    github : Remote GitHub provider.
    gitlab : GitLab provider that can run locally or reach a remote instance.
  --git-provider-connection <local|remote>             default: local
    local  : Run the provider inside this environment (supports git and gitlab).
    remote : Reach an existing remote provider (required for github).
  --secret-provider <file|vault|none>                 default: file
    file  : Encrypted local file-based secret provider.
    vault : HashiCorp Vault secret provider (runs locally or targets remote Vault).
    none  : Skip secret provider integration and rely on plaintext placeholders.
  --secret-provider-connection <local|remote>          default: local
    local  : Launch the secret provider inside the E2E workspace.
    remote : Connect to an already running provider.

Runtime controls:
  --list-components            Enumerate every component defined under test/e2e/components and exit.
  --keep-runtime               Skip runtime teardown so containers/files remain for post-run inspection.
  --verbose                    Stream additional per-step logs for debugging.
  --clean <run-id>             Stop the referenced run process (run IDs live under test/e2e/.runs/<run-id>) and delete its containers/files; run-id must match [A-Za-z0-9._-]+.
  --clean-all                  Stop every recorded run process and remove all test/e2e/.runs executions.
                               (Clean commands cannot be combined with workload flags or with each other.)
  -h, --help                   Show this help text and exit immediately.

Environment overrides:
  DECLAREST_E2E_CONTAINER_ENGINE=<podman|docker>       default: podman
  DECLAREST_E2E_EXECUTION_LOG=<path>                   optional path where detailed execution logs are written

Examples:
  ./run-e2e.sh --profile basic --repo-type filesystem --resource-server simple-api-server --secret-provider file
  ./run-e2e.sh --profile full --repo-type git --git-provider gitlab --resource-server simple-api-server
  ./run-e2e.sh --resource-server keycloak --resource-server-oauth2 true --resource-server-basic-auth false
  ./run-e2e.sh --resource-server simple-api-server --resource-server-oauth2 false --resource-server-basic-auth true --resource-server-mtls true
  ./run-e2e.sh --profile manual --keep-runtime
  ./run-e2e.sh --clean 20260216-141148-216353
  ./run-e2e.sh --clean-all
USAGE
}

e2e_parse_cleanup_args() {
  local has_cleanup_flag=0
  local has_workload_flag=0

  E2E_CLEAN_RUN_ID=''
  E2E_CLEAN_ALL=0

  while (($# > 0)); do
    case "$1" in
      --clean)
        has_cleanup_flag=1
        [[ $# -ge 2 ]] || {
          e2e_die '--clean requires a run-id value'
          return 2
        }
        [[ "${2}" != -* ]] || {
          e2e_die '--clean requires a run-id value'
          return 2
        }
        [[ -z "${E2E_CLEAN_RUN_ID}" ]] || {
          e2e_die '--clean may only be provided once'
          return 2
        }
        E2E_CLEAN_RUN_ID=$2
        shift 2
        ;;
      --clean-all)
        has_cleanup_flag=1
        ((E2E_CLEAN_ALL == 0)) || {
          e2e_die '--clean-all may only be provided once'
          return 2
        }
        E2E_CLEAN_ALL=1
        shift
        ;;
      -h|--help)
        shift
        ;;
      --verbose)
        E2E_VERBOSE=1
        shift
        ;;
      --profile|--resource-server|--resource-server-connection|--repo-type|--git-provider|--git-provider-connection|--secret-provider|--secret-provider-connection)
        has_workload_flag=1
        shift
        [[ $# -gt 0 ]] && shift || true
        ;;
      --resource-server-basic-auth|--resource-server-oauth2|--resource-server-mtls)
        has_workload_flag=1
        shift
        if [[ $# -gt 0 && "${1}" != -* ]]; then
          shift
        fi
        ;;
      --list-components|--keep-runtime)
        has_workload_flag=1
        shift
        ;;
      *)
        has_workload_flag=1
        shift
        ;;
    esac
  done

  if ((has_cleanup_flag == 0)); then
    return 1
  fi

  if ((has_workload_flag == 1)); then
    e2e_die '--clean/--clean-all cannot be combined with workload flags'
    return 2
  fi

  if [[ -n "${E2E_CLEAN_RUN_ID}" && ${E2E_CLEAN_ALL} -eq 1 ]]; then
    e2e_die '--clean and --clean-all are mutually exclusive'
    return 2
  fi

  if [[ -z "${E2E_CLEAN_RUN_ID}" && ${E2E_CLEAN_ALL} -eq 0 ]]; then
    e2e_die 'cleanup mode requires --clean <run-id> or --clean-all'
    return 2
  fi

  return 0
}

e2e_parse_args() {
  while (($# > 0)); do
    case "$1" in
      --profile)
        [[ $# -ge 2 ]] || {
          e2e_die '--profile requires a value'
          return 1
        }
        E2E_PROFILE=$2
        e2e_mark_explicit 'profile'
        shift 2
        ;;
      --resource-server)
        [[ $# -ge 2 ]] || {
          e2e_die '--resource-server requires a value'
          return 1
        }
        E2E_RESOURCE_SERVER=$2
        e2e_mark_explicit 'resource-server'
        shift 2
        ;;
      --resource-server-connection)
        [[ $# -ge 2 ]] || {
          e2e_die '--resource-server-connection requires a value'
          return 1
        }
        E2E_RESOURCE_SERVER_CONNECTION=$2
        e2e_mark_explicit 'resource-server-connection'
        shift 2
        ;;
      --resource-server-basic-auth)
        local basic_auth_value='true'
        if [[ $# -ge 2 && "${2}" != -* ]]; then
          basic_auth_value=$2
          shift 2
        else
          shift
        fi
        E2E_RESOURCE_SERVER_BASIC_AUTH=$(e2e_parse_bool_value '--resource-server-basic-auth' "${basic_auth_value}") || return 1
        e2e_mark_explicit 'resource-server-basic-auth'
        ;;
      --resource-server-oauth2)
        local oauth2_value='true'
        if [[ $# -ge 2 && "${2}" != -* ]]; then
          oauth2_value=$2
          shift 2
        else
          shift
        fi
        E2E_RESOURCE_SERVER_OAUTH2=$(e2e_parse_bool_value '--resource-server-oauth2' "${oauth2_value}") || return 1
        e2e_mark_explicit 'resource-server-oauth2'
        ;;
      --resource-server-mtls)
        local mtls_value='true'
        if [[ $# -ge 2 && "${2}" != -* ]]; then
          mtls_value=$2
          shift 2
        else
          shift
        fi
        E2E_RESOURCE_SERVER_MTLS=$(e2e_parse_bool_value '--resource-server-mtls' "${mtls_value}") || return 1
        e2e_mark_explicit 'resource-server-mtls'
        ;;
      --repo-type)
        [[ $# -ge 2 ]] || {
          e2e_die '--repo-type requires a value'
          return 1
        }
        E2E_REPO_TYPE=$2
        e2e_mark_explicit 'repo-type'
        shift 2
        ;;
      --git-provider)
        [[ $# -ge 2 ]] || {
          e2e_die '--git-provider requires a value'
          return 1
        }
        E2E_GIT_PROVIDER=$2
        e2e_mark_explicit 'git-provider'
        shift 2
        ;;
      --git-provider-connection)
        [[ $# -ge 2 ]] || {
          e2e_die '--git-provider-connection requires a value'
          return 1
        }
        E2E_GIT_PROVIDER_CONNECTION=$2
        e2e_mark_explicit 'git-provider-connection'
        shift 2
        ;;
      --secret-provider)
        [[ $# -ge 2 ]] || {
          e2e_die '--secret-provider requires a value'
          return 1
        }
        E2E_SECRET_PROVIDER=$2
        e2e_mark_explicit 'secret-provider'
        shift 2
        ;;
      --secret-provider-connection)
        [[ $# -ge 2 ]] || {
          e2e_die '--secret-provider-connection requires a value'
          return 1
        }
        E2E_SECRET_PROVIDER_CONNECTION=$2
        e2e_mark_explicit 'secret-provider-connection'
        shift 2
        ;;
      --list-components)
        E2E_LIST_COMPONENTS=1
        shift
        ;;
      --keep-runtime)
        E2E_KEEP_RUNTIME=1
        shift
        ;;
      --verbose)
        E2E_VERBOSE=1
        shift
        ;;
      -h|--help)
        # main handles help before runtime setup; keep parser behavior non-exiting.
        e2e_usage
        return 0
        ;;
      *)
        e2e_die "unknown argument: $1"
        return 1
        ;;
    esac
  done

  case "${E2E_PROFILE}" in
    basic|full|manual) ;;
    *)
      e2e_die "invalid profile: ${E2E_PROFILE} (allowed: basic, full, manual)"
      return 1
      ;;
  esac

  e2e_validate_component_arg '--resource-server' "${E2E_RESOURCE_SERVER}" 'true' || return 1

  case "${E2E_RESOURCE_SERVER_CONNECTION}" in
    local|remote) ;;
    *)
      e2e_die "invalid resource-server connection: ${E2E_RESOURCE_SERVER_CONNECTION}"
      return 1
      ;;
  esac

  E2E_RESOURCE_SERVER_BASIC_AUTH=$(e2e_parse_bool_value '--resource-server-basic-auth' "${E2E_RESOURCE_SERVER_BASIC_AUTH}") || return 1
  E2E_RESOURCE_SERVER_OAUTH2=$(e2e_parse_bool_value '--resource-server-oauth2' "${E2E_RESOURCE_SERVER_OAUTH2}") || return 1
  E2E_RESOURCE_SERVER_MTLS=$(e2e_parse_bool_value '--resource-server-mtls' "${E2E_RESOURCE_SERVER_MTLS}") || return 1

  e2e_validate_component_arg '--repo-type' "${E2E_REPO_TYPE}" || return 1

  if [[ "${E2E_REPO_TYPE}" == 'git' && -z "${E2E_GIT_PROVIDER}" ]]; then
    E2E_GIT_PROVIDER='git'
  fi

  e2e_validate_component_arg '--git-provider' "${E2E_GIT_PROVIDER}" 'false' 'true' || return 1

  case "${E2E_GIT_PROVIDER_CONNECTION}" in
    local|remote) ;;
    *)
      e2e_die "invalid git-provider connection: ${E2E_GIT_PROVIDER_CONNECTION}"
      return 1
      ;;
  esac

  e2e_validate_component_arg '--secret-provider' "${E2E_SECRET_PROVIDER}" 'true' || return 1

  case "${E2E_SECRET_PROVIDER_CONNECTION}" in
    local|remote) ;;
    *)
      e2e_die "invalid secret-provider connection: ${E2E_SECRET_PROVIDER_CONNECTION}"
      return 1
      ;;
  esac

  return 0
}
