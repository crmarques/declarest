#!/usr/bin/env bash

declare -Ag E2E_EXPLICIT

E2E_MANAGED_SERVER='simple-api-server'
E2E_MANAGED_SERVER_CONNECTION='local'
E2E_MANAGED_SERVER_AUTH_TYPE=''
E2E_MANAGED_SERVER_MTLS='false'
E2E_MANAGED_SERVER_PROXY='false'
E2E_MANAGED_SERVER_PROXY_HTTP_URL="${DECLAREST_E2E_MANAGED_SERVER_PROXY_HTTP_URL:-}"
E2E_MANAGED_SERVER_PROXY_HTTPS_URL="${DECLAREST_E2E_MANAGED_SERVER_PROXY_HTTPS_URL:-}"
E2E_MANAGED_SERVER_PROXY_NO_PROXY="${DECLAREST_E2E_MANAGED_SERVER_PROXY_NO_PROXY:-}"
E2E_MANAGED_SERVER_PROXY_AUTH_USERNAME="${DECLAREST_E2E_MANAGED_SERVER_PROXY_AUTH_USERNAME:-}"
E2E_MANAGED_SERVER_PROXY_AUTH_PASSWORD="${DECLAREST_E2E_MANAGED_SERVER_PROXY_AUTH_PASSWORD:-}"
E2E_METADATA='bundle'
E2E_REPO_TYPE='filesystem'
E2E_GIT_PROVIDER=''
E2E_GIT_PROVIDER_CONNECTION='local'
E2E_SECRET_PROVIDER='file'
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

e2e_parse_managed_server_auth_type_value() {
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
    *)
      e2e_die "invalid --managed-server-auth-type value: ${raw_value} (allowed: none, basic, oauth2, custom-header)"
      return 1
      ;;
  esac
}

e2e_parse_metadata_source_value() {
  local flag=$1
  local raw_value=$2

  case "${raw_value,,}" in
    bundle)
      printf 'bundle\n'
      return 0
      ;;
    dir)
      if [[ "${flag}" == '--metadata-source' ]]; then
        printf 'dir\n'
        return 0
      fi
      ;;
    base-dir)
      if [[ "${flag}" == '--metadata-type' ]]; then
        printf 'dir\n'
        return 0
      fi
      ;;
  esac

  case "${flag}" in
    --metadata-source)
      e2e_die "invalid --metadata-source value: ${raw_value} (allowed: bundle, dir)"
      ;;
    --metadata-type)
      e2e_die "invalid --metadata-type value: ${raw_value} (allowed: bundle, base-dir)"
      ;;
    *)
      e2e_die "invalid metadata source value: ${raw_value}"
      ;;
  esac

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
                     ResourceRepository/ManagedServer/SecretStore/SyncPolicy resources, then keep runtime for manual checks.
    operator-basic : Same operator environment as operator-manual, then run operator-focused "main" automated cases.
    operator-full  : Same operator environment as operator-basic, plus corner validations.

Platform selection:
  --platform <compose|kubernetes>                 default: kubernetes
    compose    : Start local containerized components with the selected compose engine (podman or docker).
    kubernetes : Start local containerized components in a run-scoped kind cluster.

Component selection (choose values for each flag; see notes below):
  --managed-server <simple-api-server|keycloak|rundeck|vault>          default: simple-api-server
    simple-api-server : Lightweight JSON API with selectable auth modes (none/basic/oauth2) and optional mTLS.
    keycloak          : Keycloak Admin REST API that enforces OAuth2 client-credentials tokens.
    rundeck           : Rundeck HTTP API surface for job-centric operations.
    vault             : HashiCorp Vault HTTP API acting as the managed server.
    A managed-server selection is mandatory for e2e runs; `none` is not supported.
  --managed-server-connection <local|remote>            default: local
    local  : Start the chosen managed server via the provided fixtures and scripts.
    remote : Assume the server already exists and reach it via the configured connection details.
  --managed-server-auth-type <none|basic|oauth2|custom-header>
    Select the managed-server auth mode. When omitted, the selected component elects a default auth type
    (preference order: oauth2, custom-header, basic, none) subject to its capability contract.
  --managed-server-mtls [<true|false>]                  default: false
    true  : Require client certificates when the component advertises mTLS.
    false : Run without mTLS client validation even if the server can enforce it.
  --managed-server-proxy [<true|false>]                default: false
    true  : Inject managedServer.http.proxy into the generated context using DECLAREST_E2E_MANAGED_SERVER_PROXY_* values.
    false : Keep managed-server proxy unset in generated contexts.
  --metadata-source <bundle|dir>                    default: bundle
    bundle    : Use metadata.bundle shorthand from the selected managed-server contract and ignore component openapi.yaml.
    dir       : Use component-local metadata directory when provided and keep normal local OpenAPI wiring.
    legacy alias: --metadata-type <bundle|base-dir>
  --repo-type <filesystem|git>                        default: filesystem
    filesystem : Use the local filesystem repository backend.
    git        : Use the git repository backend (requires a git provider selection).
  --git-provider <git|github|gitlab|gitea>            default: git when --repo-type git (none otherwise)
    git    : Built-in file:// git provider supplied with the fixtures.
    github : Remote GitHub provider (requires --git-provider-connection remote).
    gitlab : GitLab provider that can run locally or remote, depending on the connection flag.
    gitea  : Gitea provider that can run locally or remote, depending on the connection flag.
    Selecting --repo-type git without an explicit --git-provider forces --git-provider=git.
  --git-provider-connection <local|remote>             default: local
    local  : Launch the git provider inside this workspace.
    remote : Reach an existing provider instance (required for github; optional for gitlab/gitea).
  --secret-provider <file|vault|none>                 default: file
    file  : Encrypted local file-based secret provider backed by fixtures.
    vault : HashiCorp Vault provider that can run locally or connect to a remote Vault.
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
  DECLAREST_E2E_MANAGED_SERVER_PROXY_HTTP_URL=<url>    optional managed-server proxy http-url
  DECLAREST_E2E_MANAGED_SERVER_PROXY_HTTPS_URL=<url>   optional managed-server proxy https-url
  DECLAREST_E2E_MANAGED_SERVER_PROXY_NO_PROXY=<list>   optional managed-server proxy no-proxy list
  DECLAREST_E2E_MANAGED_SERVER_PROXY_AUTH_USERNAME=<v> optional managed-server proxy auth username
  DECLAREST_E2E_MANAGED_SERVER_PROXY_AUTH_PASSWORD=<v> optional managed-server proxy auth password

Examples:
  ./run-e2e.sh --platform kubernetes --profile cli-basic --repo-type filesystem --managed-server simple-api-server --secret-provider file
  ./run-e2e.sh --platform compose --profile cli-basic --repo-type filesystem --managed-server simple-api-server --secret-provider file
  ./run-e2e.sh --profile cli-basic --repo-type filesystem --managed-server simple-api-server --secret-provider file
  ./run-e2e.sh --profile cli-basic --managed-server simple-api-server --metadata-source dir
  ./run-e2e.sh --profile cli-full --repo-type git --git-provider gitlab --managed-server simple-api-server
  ./run-e2e.sh --profile cli-full --repo-type git --git-provider gitea --managed-server simple-api-server
  ./run-e2e.sh --profile operator-manual --managed-server simple-api-server --git-provider gitea --secret-provider file
  ./run-e2e.sh --profile operator-basic --managed-server simple-api-server --git-provider gitea --secret-provider file
  ./run-e2e.sh --profile operator-full --managed-server simple-api-server --git-provider gitea --secret-provider file
  ./run-e2e.sh --managed-server keycloak --managed-server-auth-type oauth2
  ./run-e2e.sh --managed-server simple-api-server --managed-server-auth-type basic --managed-server-mtls true
  DECLAREST_E2E_MANAGED_SERVER_PROXY_HTTP_URL=http://127.0.0.1:3128 ./run-e2e.sh --managed-server-proxy true
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
      --profile|--platform|--managed-server|--managed-server-connection|--managed-server-auth-type|--metadata-source|--metadata-type|--repo-type|--git-provider|--git-provider-connection|--secret-provider|--secret-provider-connection)
        has_workload_flag=1
        shift
        [[ $# -gt 0 ]] && shift || true
        ;;
      --managed-server-mtls|--managed-server-proxy)
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
      --managed-server)
        [[ $# -ge 2 ]] || {
          e2e_die '--managed-server requires a value'
          return 1
        }
        E2E_MANAGED_SERVER=$2
        e2e_mark_explicit 'managed-server'
        shift 2
        ;;
      --managed-server-connection)
        [[ $# -ge 2 ]] || {
          e2e_die '--managed-server-connection requires a value'
          return 1
        }
        E2E_MANAGED_SERVER_CONNECTION=$2
        e2e_mark_explicit 'managed-server-connection'
        shift 2
        ;;
      --managed-server-auth-type)
        [[ $# -ge 2 ]] || {
          e2e_die '--managed-server-auth-type requires a value'
          return 1
        }
        E2E_MANAGED_SERVER_AUTH_TYPE=$(e2e_parse_managed_server_auth_type_value "$2") || return 1
        e2e_mark_explicit 'managed-server-auth-type'
        shift 2
        ;;
      --managed-server-mtls)
        local mtls_value='true'
        if [[ $# -ge 2 && "${2}" != -* ]]; then
          mtls_value=$2
          shift 2
        else
          shift
        fi
        E2E_MANAGED_SERVER_MTLS=$(e2e_parse_bool_value '--managed-server-mtls' "${mtls_value}") || return 1
        e2e_mark_explicit 'managed-server-mtls'
        ;;
      --managed-server-proxy)
        local proxy_value='true'
        if [[ $# -ge 2 && "${2}" != -* ]]; then
          proxy_value=$2
          shift 2
        else
          shift
        fi
        E2E_MANAGED_SERVER_PROXY=$(e2e_parse_bool_value '--managed-server-proxy' "${proxy_value}") || return 1
        e2e_mark_explicit 'managed-server-proxy'
        ;;
      --metadata-source|--metadata-type)
        local metadata_flag=$1
        [[ $# -ge 2 ]] || {
          e2e_die "${metadata_flag} requires a value"
          return 1
        }
        E2E_METADATA=$(e2e_parse_metadata_source_value "${metadata_flag}" "$2") || return 1
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

  if [[ "${E2E_MANAGED_SERVER}" == 'none' ]]; then
    e2e_die '--managed-server none is not supported; select a managed-server component'
    return 1
  fi
  e2e_validate_component_arg '--managed-server' "${E2E_MANAGED_SERVER}" || return 1

  case "${E2E_MANAGED_SERVER_CONNECTION}" in
    local|remote) ;;
    *)
      e2e_die "invalid managed-server connection: ${E2E_MANAGED_SERVER_CONNECTION}"
      return 1
      ;;
  esac

  if [[ -n "${E2E_MANAGED_SERVER_AUTH_TYPE}" ]]; then
    E2E_MANAGED_SERVER_AUTH_TYPE=$(e2e_parse_managed_server_auth_type_value "${E2E_MANAGED_SERVER_AUTH_TYPE}") || return 1
  fi
  E2E_MANAGED_SERVER_MTLS=$(e2e_parse_bool_value '--managed-server-mtls' "${E2E_MANAGED_SERVER_MTLS}") || return 1
  E2E_MANAGED_SERVER_PROXY=$(e2e_parse_bool_value '--managed-server-proxy' "${E2E_MANAGED_SERVER_PROXY}") || return 1
  if [[ "${E2E_MANAGED_SERVER_PROXY}" == 'true' ]]; then
    if [[ -z "${E2E_MANAGED_SERVER_PROXY_HTTP_URL}" && -z "${E2E_MANAGED_SERVER_PROXY_HTTPS_URL}" ]]; then
      e2e_die "--managed-server-proxy requires DECLAREST_E2E_MANAGED_SERVER_PROXY_HTTP_URL and/or DECLAREST_E2E_MANAGED_SERVER_PROXY_HTTPS_URL"
      return 1
    fi
    if [[ -n "${E2E_MANAGED_SERVER_PROXY_AUTH_USERNAME}" || -n "${E2E_MANAGED_SERVER_PROXY_AUTH_PASSWORD}" ]]; then
      if [[ -z "${E2E_MANAGED_SERVER_PROXY_AUTH_USERNAME}" || -z "${E2E_MANAGED_SERVER_PROXY_AUTH_PASSWORD}" ]]; then
        e2e_die 'managed-server proxy auth requires both DECLAREST_E2E_MANAGED_SERVER_PROXY_AUTH_USERNAME and DECLAREST_E2E_MANAGED_SERVER_PROXY_AUTH_PASSWORD'
        return 1
      fi
    fi
  fi
  E2E_METADATA=$(e2e_parse_metadata_source_value '--metadata-source' "${E2E_METADATA}") || return 1

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
