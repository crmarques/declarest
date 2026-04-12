#!/usr/bin/env bash

declare -Ag E2E_EXPLICIT

E2E_MANAGED_SERVICE=''
E2E_MANAGED_SERVICE_CONNECTION='local'
E2E_MANAGED_SERVICE_AUTH_TYPE=''
E2E_MANAGED_SERVICE_MTLS='false'
E2E_PROXY_MODE='none'
E2E_PROXY_AUTH_TYPE=''
E2E_PROXY_HTTP_URL="${DECLAREST_E2E_PROXY_HTTP_URL:-}"
E2E_PROXY_HTTPS_URL="${DECLAREST_E2E_PROXY_HTTPS_URL:-}"
E2E_PROXY_NO_PROXY="${DECLAREST_E2E_PROXY_NO_PROXY:-}"
E2E_PROXY_AUTH_USERNAME="${DECLAREST_E2E_PROXY_AUTH_USERNAME:-}"
E2E_PROXY_AUTH_PASSWORD="${DECLAREST_E2E_PROXY_AUTH_PASSWORD:-}"
E2E_METADATA='bundle'
E2E_REPO_TYPE=''
E2E_GIT_PROVIDER=''
E2E_GIT_PROVIDER_CONNECTION='local'
E2E_SECRET_PROVIDER=''
E2E_SECRET_PROVIDER_CONNECTION='local'
E2E_PROFILE='cli-basic'
E2E_PLATFORM='kubernetes'
E2E_LIST_COMPONENTS=0
E2E_VALIDATE_COMPONENTS=0
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
  local profile='cli-basic'

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

e2e_args_ensure_component_catalog() {
  if ! declare -F e2e_component_catalog_ensure_discovered >/dev/null 2>&1; then
    # shellcheck disable=SC1091
    source "${SCRIPT_DIR}/lib/components.sh"
  fi

  e2e_component_catalog_ensure_discovered
}

e2e_args_apply_base_component_defaults() {
  [[ -n "${E2E_MANAGED_SERVICE:-}" ]] || E2E_MANAGED_SERVICE=$(e2e_component_default_name_for_type 'managed-service' 'base') || return 1
  [[ -n "${E2E_REPO_TYPE:-}" ]] || E2E_REPO_TYPE=$(e2e_component_default_name_for_type 'repo-type' 'base') || return 1
  [[ -n "${E2E_SECRET_PROVIDER:-}" ]] || E2E_SECRET_PROVIDER=$(e2e_component_default_name_for_type 'secret-provider' 'base') || return 1

  if [[ "${E2E_REPO_TYPE}" == 'git' && -z "${E2E_GIT_PROVIDER}" ]]; then
    E2E_GIT_PROVIDER=$(e2e_component_default_name_for_type 'git-provider' 'base') || return 1
  fi

  return 0
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

e2e_parse_managed_service_auth_type_value() {
  local raw_value=$1

  case "${raw_value,,}" in
    none)
      printf 'none\n'
      ;;
    basic)
      printf 'basic\n'
      ;;
    oauth2)
      printf 'oauth2\n'
      ;;
    custom-header)
      printf 'custom-header\n'
      ;;
    prompt)
      printf 'prompt\n'
      ;;
    *)
      e2e_die "invalid --managed-service-auth-type value: ${raw_value} (allowed: none, basic, oauth2, custom-header, prompt)"
      return 1
      ;;
  esac
}

e2e_parse_proxy_mode_value() {
  local raw_value=$1

  case "${raw_value,,}" in
    none)
      printf 'none\n'
      ;;
    local)
      printf 'local\n'
      ;;
    external)
      printf 'external\n'
      ;;
    *)
      e2e_die "invalid --proxy-mode value: ${raw_value} (allowed: none, local, external)"
      return 1
      ;;
  esac
}

e2e_parse_proxy_auth_type_value() {
  local raw_value=$1

  case "${raw_value,,}" in
    none)
      printf 'none\n'
      ;;
    basic)
      printf 'basic\n'
      ;;
    prompt)
      printf 'prompt\n'
      ;;
    *)
      e2e_die "invalid --proxy-auth-type value: ${raw_value} (allowed: none, basic, prompt)"
      return 1
      ;;
  esac
}

e2e_proxy_mode_is_explicit() {
  e2e_is_explicit 'proxy-mode'
}

e2e_proxy_auth_type_is_explicit() {
  e2e_is_explicit 'proxy-auth-type'
}

e2e_set_proxy_mode() {
  local value=$1

  if e2e_proxy_mode_is_explicit && [[ "${E2E_PROXY_MODE:-none}" != "${value}" ]]; then
    e2e_die "conflicting proxy mode selection: ${E2E_PROXY_MODE} vs ${value}"
    return 1
  fi

  E2E_PROXY_MODE="${value}"
  e2e_mark_explicit 'proxy-mode'
}

e2e_set_proxy_auth_type() {
  local value=$1

  if e2e_proxy_auth_type_is_explicit && [[ "${E2E_PROXY_AUTH_TYPE:-}" != "${value}" ]]; then
    e2e_die "conflicting proxy auth-type selection: ${E2E_PROXY_AUTH_TYPE} vs ${value}"
    return 1
  fi

  E2E_PROXY_AUTH_TYPE="${value}"
  e2e_mark_explicit 'proxy-auth-type'
}

e2e_has_proxy_basic_auth_values() {
  [[ -n "${E2E_PROXY_AUTH_USERNAME:-}" || -n "${E2E_PROXY_AUTH_PASSWORD:-}" ]]
}

e2e_effective_proxy_auth_type() {
  if [[ "${E2E_PROXY_MODE:-none}" == 'none' ]]; then
    printf 'none\n'
    return 0
  fi

  if [[ -n "${E2E_PROXY_AUTH_TYPE:-}" ]]; then
    printf '%s\n' "${E2E_PROXY_AUTH_TYPE}"
    return 0
  fi

  if e2e_has_proxy_basic_auth_values; then
    printf 'basic\n'
    return 0
  fi

  if [[ "${E2E_PROXY_MODE:-none}" == 'local' ]]; then
    printf 'basic\n'
    return 0
  fi

  printf 'none\n'
}

e2e_parse_metadata_source_value() {
  local raw_value=$1

  case "${raw_value,,}" in
    bundle)
      printf 'bundle\n'
      return 0
      ;;
    dir)
      printf 'dir\n'
      return 0
      ;;
  esac

  e2e_die "invalid --metadata-source value: ${raw_value} (allowed: bundle, dir)"
  return 1
}

e2e_parse_platform_value() {
  local raw_value=$1

  case "${raw_value,,}" in
    compose)
      printf 'compose\n'
      ;;
    kubernetes)
      printf 'kubernetes\n'
      ;;
    *)
      e2e_die "invalid --platform value: ${raw_value} (allowed: compose, kubernetes)"
      return 1
      ;;
  esac
}

e2e_usage() {
  cat <<'USAGE'
Usage: ./run-e2e.sh [flags]

Objective:
  Validate Declarest end-to-end workflows by orchestrating the selected component stack,
  matching the chosen profile requirements, and exercising CLI cases that verify repository,
  metadata, secret, and security behavior across deterministic steps.

Profiles (required, defaults to cli-basic when omitted):
  --profile <cli-basic|cli-full|cli-manual|operator-manual|operator-basic|operator-full>   default: cli-basic
    cli-basic      : Run "main" cases against the default stack in an automated CLI workflow.
    cli-full       : Execute "main" plus "corner" CLI cases to cover less-common paths and components.
    cli-manual     : Start only local-instantiable components, emit setup/reset shell scripts, and exit so you can run
                     Declarest commands interactively. Requires every selected connection to stay local.
    operator-manual: Provision a kubernetes-only local stack, deploy the operator manager in-cluster, apply generated
                     ResourceRepository/ManagedService/SecretStore/SyncPolicy resources, then keep runtime for manual checks.
    operator-basic : Same operator environment as operator-manual, then run operator-focused "main" automated cases.
    operator-full  : Same operator environment as operator-basic, plus corner validations.

Platform selection:
  --platform <compose|kubernetes>                 default: kubernetes
    compose    : Start local containerized components with the selected compose engine (podman or docker).
    kubernetes : Start local containerized components in a run-scoped kind cluster.

Component selection (choose values for each flag; see notes below):
  --managed-service <name>                              default: component contract default
    Use --list-components to inspect available managed-service components and their descriptions.
    A managed-service selection is mandatory for e2e runs; `none` is not supported.
  --managed-service-connection <local|remote>            default: local
    local  : Start the chosen managed service via the provided fixtures and scripts.
    remote : Assume the server already exists and reach it via the configured connection details.
  --managed-service-auth-type <none|basic|oauth2|custom-header|prompt>
    Select the managed-service auth mode. When omitted, the selected component elects a default auth type
    (preference order: oauth2, custom-header, basic, none) subject to its capability contract.
    prompt    : Emit managedService.http.auth.prompt for basic-auth-capable managed-service components.
  --managed-service-mtls [<true|false>]                  default: false
    true  : Require client certificates when the component advertises mTLS.
    false : Run without mTLS client validation even if the server can enforce it.
  --proxy-mode <none|local|external>                default: none
    none     : Keep proxy settings unset in generated contexts.
    local    : Start the bundled forward proxy component and inject explicit proxy blocks for selected CLI components.
    external : Inject explicit proxy blocks using DECLAREST_E2E_PROXY_* values without starting a local proxy component.
  --proxy-auth-type <none|basic|prompt>
    none   : Inject no proxy auth block.
    basic  : Inject proxy auth username/password; local mode defaults here and auto-generates creds when none are supplied.
    prompt : Inject *.proxy.auth.prompt and defer proxy credentials to runtime prompts (cli-manual only).
  --metadata-source <bundle|dir>                    default: bundle
    bundle    : Use metadata.bundle shorthand from the selected managed-service contract and ignore component openapi.yaml.
    dir       : Use component-local metadata directory when provided and keep normal local OpenAPI wiring.
  --repo-type <name>                                  default: component contract default
    Use --list-components to inspect available repository backends and their descriptions.
  --git-provider <name>                               default: git-provider component contract default when --repo-type git
    Use --list-components to inspect available git-provider components and their descriptions.
  --git-provider-connection <local|remote>             default: local
    local  : Launch the git provider inside this workspace.
    remote : Reach an existing provider instance using the selected component contract.
  --secret-provider <name|none>                        default: component contract default
    Use --list-components to inspect available secret-provider components and their descriptions.
    none  : Skip secret provider integration so placeholders remain plaintext.
  --secret-provider-connection <local|remote>          default: local
    local  : Start the secret provider from the workspace fixtures.
    remote : Connect to a running remote provider endpoint.

Runtime controls:
  --list-components            Enumerate every component defined under test/e2e/components and exit.
  --validate-components        Validate every discovered component contract and fixture tree, then exit.
  --keep-runtime               Skip runtime teardown so containers and files remain available for inspection.
  --verbose                    Stream supplemental per-step logs for troubleshooting.

Cleanup controls:
  --clean <run-id>             Stop the referenced run (test/e2e/.runs/<run-id>) and delete its runtime artifacts; run-id must match [A-Za-z0-9._-]+.
  --clean-all                  Stop every recorded run process and remove all test/e2e/.runs executions.
                               (--clean/--clean-all cannot be combined with each other or with workload flags.)

Global flags:
  -h, --help                   Show this help text and exit immediately.

Environment overrides:
  DECLAREST_E2E_CONTAINER_ENGINE=<podman|docker>       default: podman
  DECLAREST_E2E_K8S_COMPONENT_READY_TIMEOUT_SECONDS=<seconds>
                                                       default: 600 (kubernetes pod readiness wait per component)
  DECLAREST_E2E_OPERATOR_READY_TIMEOUT_SECONDS=<seconds>
                                                       default: 120 (operator CR readiness wait; must be <= 600)
  DECLAREST_E2E_EXECUTION_LOG=<path>                   optional path where detailed execution logs are written
  DECLAREST_E2E_PROXY_HTTP_URL=<url>                   optional proxy http-url (external mode)
  DECLAREST_E2E_PROXY_HTTPS_URL=<url>                  optional proxy https-url (external mode)
  DECLAREST_E2E_PROXY_NO_PROXY=<list>                  optional proxy no-proxy list
  DECLAREST_E2E_PROXY_AUTH_USERNAME=<v>                optional proxy auth username
  DECLAREST_E2E_PROXY_AUTH_PASSWORD=<v>                optional proxy auth password

Examples:
  ./run-e2e.sh --platform kubernetes --profile cli-basic
  ./run-e2e.sh --platform compose --profile cli-basic
  ./run-e2e.sh --profile cli-basic --metadata-source dir
  ./run-e2e.sh --profile cli-full --repo-type git --git-provider <git-provider-name>
  ./run-e2e.sh --profile operator-manual
  ./run-e2e.sh --profile operator-basic
  ./run-e2e.sh --profile operator-full
  ./run-e2e.sh --managed-service <managed-service-name> --managed-service-auth-type oauth2
  ./run-e2e.sh --profile cli-manual --managed-service-auth-type prompt
  ./run-e2e.sh --managed-service-auth-type basic --managed-service-mtls true
  ./run-e2e.sh --proxy-mode local
  ./run-e2e.sh --profile cli-manual --proxy-mode local --proxy-auth-type prompt
  DECLAREST_E2E_PROXY_HTTP_URL=http://127.0.0.1:3128 ./run-e2e.sh --proxy-mode external
  DECLAREST_E2E_PROXY_HTTP_URL=http://127.0.0.1:3128 ./run-e2e.sh --proxy-mode external --proxy-auth-type prompt
  ./run-e2e.sh --profile cli-manual --keep-runtime
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
      --profile|--platform|--managed-service|--managed-service-connection|--managed-service-auth-type|--proxy-mode|--proxy-auth-type|--metadata-source|--repo-type|--git-provider|--git-provider-connection|--secret-provider|--secret-provider-connection)
        has_workload_flag=1
        shift
        [[ $# -gt 0 ]] && shift || true
        ;;
      --managed-service-mtls)
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
      --validate-components)
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
      --platform)
        [[ $# -ge 2 ]] || {
          e2e_die '--platform requires a value'
          return 1
        }
        E2E_PLATFORM=$(e2e_parse_platform_value "$2") || return 1
        e2e_mark_explicit 'platform'
        shift 2
        ;;
      --managed-service)
        [[ $# -ge 2 ]] || {
          e2e_die '--managed-service requires a value'
          return 1
        }
        E2E_MANAGED_SERVICE=$2
        e2e_mark_explicit 'managed-service'
        shift 2
        ;;
      --managed-service-connection)
        [[ $# -ge 2 ]] || {
          e2e_die '--managed-service-connection requires a value'
          return 1
        }
        E2E_MANAGED_SERVICE_CONNECTION=$2
        e2e_mark_explicit 'managed-service-connection'
        shift 2
        ;;
      --managed-service-auth-type)
        [[ $# -ge 2 ]] || {
          e2e_die '--managed-service-auth-type requires a value'
          return 1
        }
        E2E_MANAGED_SERVICE_AUTH_TYPE=$(e2e_parse_managed_service_auth_type_value "$2") || return 1
        e2e_mark_explicit 'managed-service-auth-type'
        shift 2
        ;;
      --proxy-mode)
        [[ $# -ge 2 ]] || {
          e2e_die '--proxy-mode requires a value'
          return 1
        }
        e2e_set_proxy_mode "$(e2e_parse_proxy_mode_value "$2")" || return 1
        shift 2
        ;;
      --proxy-auth-type)
        [[ $# -ge 2 ]] || {
          e2e_die '--proxy-auth-type requires a value'
          return 1
        }
        e2e_set_proxy_auth_type "$(e2e_parse_proxy_auth_type_value "$2")" || return 1
        shift 2
        ;;
      --managed-service-mtls)
        local mtls_value='true'
        if [[ $# -ge 2 && "${2}" != -* ]]; then
          mtls_value=$2
          shift 2
        else
          shift
        fi
        E2E_MANAGED_SERVICE_MTLS=$(e2e_parse_bool_value '--managed-service-mtls' "${mtls_value}") || return 1
        e2e_mark_explicit 'managed-service-mtls'
        ;;
      --metadata-source)
        [[ $# -ge 2 ]] || {
          e2e_die '--metadata-source requires a value'
          return 1
        }
        E2E_METADATA=$(e2e_parse_metadata_source_value "$2") || return 1
        e2e_mark_explicit 'metadata'
        shift 2
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
      --validate-components)
        E2E_VALIDATE_COMPONENTS=1
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
    cli-basic|cli-full|cli-manual|operator-manual|operator-basic|operator-full) ;;
    *)
      e2e_die "invalid profile: ${E2E_PROFILE} (allowed: cli-basic, cli-full, cli-manual, operator-manual, operator-basic, operator-full)"
      return 1
      ;;
  esac

  E2E_PLATFORM=$(e2e_parse_platform_value "${E2E_PLATFORM}") || return 1

  e2e_args_ensure_component_catalog || return 1
  e2e_args_apply_base_component_defaults || return 1

  if [[ "${E2E_MANAGED_SERVICE}" == 'none' ]]; then
    e2e_die '--managed-service none is not supported; select a managed-service component'
    return 1
  fi
  e2e_validate_component_arg '--managed-service' "${E2E_MANAGED_SERVICE}" || return 1

  case "${E2E_MANAGED_SERVICE_CONNECTION}" in
    local|remote) ;;
    *)
      e2e_die "invalid managed-service connection: ${E2E_MANAGED_SERVICE_CONNECTION}"
      return 1
      ;;
  esac

  if [[ -n "${E2E_MANAGED_SERVICE_AUTH_TYPE}" ]]; then
    E2E_MANAGED_SERVICE_AUTH_TYPE=$(e2e_parse_managed_service_auth_type_value "${E2E_MANAGED_SERVICE_AUTH_TYPE}") || return 1
  fi
  if [[ -n "${E2E_PROXY_AUTH_TYPE}" ]]; then
    E2E_PROXY_AUTH_TYPE=$(e2e_parse_proxy_auth_type_value "${E2E_PROXY_AUTH_TYPE}") || return 1
  fi
  E2E_MANAGED_SERVICE_MTLS=$(e2e_parse_bool_value '--managed-service-mtls' "${E2E_MANAGED_SERVICE_MTLS}") || return 1
  E2E_PROXY_MODE=$(e2e_parse_proxy_mode_value "${E2E_PROXY_MODE}") || return 1
  case "${E2E_PROXY_MODE}" in
    none)
      if [[ -n "${E2E_PROXY_AUTH_TYPE}" ]]; then
        e2e_die '--proxy-auth-type requires --proxy-mode local or external'
        return 1
      fi
      if e2e_has_proxy_basic_auth_values; then
        if [[ -z "${E2E_PROXY_AUTH_USERNAME}" || -z "${E2E_PROXY_AUTH_PASSWORD}" ]]; then
          e2e_die 'proxy auth requires both DECLAREST_E2E_PROXY_AUTH_USERNAME and DECLAREST_E2E_PROXY_AUTH_PASSWORD'
        else
          e2e_die '--proxy-mode none cannot be combined with proxy auth credentials'
        fi
        return 1
      fi
      ;;
    external)
      if [[ -z "${E2E_PROXY_HTTP_URL}" && -z "${E2E_PROXY_HTTPS_URL}" ]]; then
        e2e_die '--proxy-mode external requires DECLAREST_E2E_PROXY_HTTP_URL and/or DECLAREST_E2E_PROXY_HTTPS_URL'
        return 1
      fi
      case "$(e2e_effective_proxy_auth_type)" in
        none)
          ;;
        basic)
          if [[ -z "${E2E_PROXY_AUTH_USERNAME}" || -z "${E2E_PROXY_AUTH_PASSWORD}" ]]; then
            if [[ -n "${E2E_PROXY_AUTH_TYPE}" ]]; then
              e2e_die 'proxy auth-type basic requires DECLAREST_E2E_PROXY_AUTH_USERNAME and DECLAREST_E2E_PROXY_AUTH_PASSWORD'
            else
              e2e_die 'proxy auth requires both DECLAREST_E2E_PROXY_AUTH_USERNAME and DECLAREST_E2E_PROXY_AUTH_PASSWORD'
            fi
            return 1
          fi
          ;;
        prompt)
          if e2e_has_proxy_basic_auth_values; then
            e2e_die 'proxy auth-type prompt cannot be combined with DECLAREST_E2E_PROXY_AUTH_USERNAME or DECLAREST_E2E_PROXY_AUTH_PASSWORD'
            return 1
          fi
          ;;
        *)
          e2e_die "invalid proxy auth-type: $(e2e_effective_proxy_auth_type)"
          return 1
          ;;
      esac
      ;;
    local)
      case "$(e2e_effective_proxy_auth_type)" in
        none)
          if e2e_has_proxy_basic_auth_values; then
            e2e_die '--proxy-auth-type none cannot be combined with proxy auth credentials'
            return 1
          fi
          ;;
        basic)
          if e2e_has_proxy_basic_auth_values && [[ -z "${E2E_PROXY_AUTH_USERNAME}" || -z "${E2E_PROXY_AUTH_PASSWORD}" ]]; then
            e2e_die 'proxy auth requires both DECLAREST_E2E_PROXY_AUTH_USERNAME and DECLAREST_E2E_PROXY_AUTH_PASSWORD'
            return 1
          fi
          ;;
        prompt)
          if e2e_has_proxy_basic_auth_values; then
            e2e_die 'proxy auth-type prompt cannot be combined with DECLAREST_E2E_PROXY_AUTH_USERNAME or DECLAREST_E2E_PROXY_AUTH_PASSWORD'
            return 1
          fi
          ;;
        *)
          e2e_die "invalid proxy auth-type: $(e2e_effective_proxy_auth_type)"
          return 1
          ;;
      esac
      ;;
  esac
  E2E_METADATA=$(e2e_parse_metadata_source_value "${E2E_METADATA}") || return 1

  e2e_validate_component_arg '--repo-type' "${E2E_REPO_TYPE}" || return 1

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
