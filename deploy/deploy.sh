#!/bin/bash
#set -x

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
# shellcheck source=common.sh
source "${SCRIPT_DIR}/common.sh"

usage() {
  cat <<EOF
${COLOR_BOLD}GoOne Deploy CLI${COLOR_RESET}

Usage:
  $0 help
  $0 env list
  $0 role list
  $0 run --env <name> --action <init|push|start|stop|restart> [--role <role> ...] [options] [-- <extra ansible args...>]

Options:
  --config <file>        dotenv config (default: ${SCRIPT_DIR}/.env if exists)
  -i, --inventory <file> ansible inventory file (default: hosts/host_<env>.txt else hosts/host_dev.txt)
  --limit <pattern>      ansible --limit
  --check                ansible --check
  --diff                 ansible --diff
  --dry-run              same as --check --diff
  --roles <a,b,c>        comma-separated roles (alternative to repeating --role)
  --no-hostkey-check     set ANSIBLE_HOST_KEY_CHECKING=False

Examples:
  $0 env list
  $0 role list
  $0 run --env dev1 --action restart --role websvr
  $0 run --env dev1 --action restart --roles websvr,mainsvr --dry-run
  $0 run --env dev1 --action push --limit 113.45.34.170 --role websvr -- -vv
EOF
}

list_envs() {
  (cd "${SCRIPT_DIR}/playbook_dev" && ls -1 *.yml 2>/dev/null | sed -e 's/\.yml$//') || true
}

list_roles() {
  (cd "${SCRIPT_DIR}/roles" && ls -1 2>/dev/null) || true
}

split_csv_roles() {
  local csv="$1"
  local old_ifs="$IFS"
  IFS=',' read -r -a _parts <<< "$csv"
  IFS="$old_ifs"
  local r
  for r in "${_parts[@]}"; do
    r="$(trim "$r")"
    [[ -n "$r" ]] && echo "$r"
  done
}

ACTIVE_DEFAULT_ROLES=(
  commconf
  gamedata
  connsvr
  mainsvr
  infosvr
  mysqlsvr
  roomcentersvr
  websvr
)

cmd="${1:-help}"
shift || true

case "$cmd" in
  help|-h|--help)
    usage
    exit 0
    ;;

  env)
    sub="${1:-}"
    [[ "$sub" == "list" ]] || die "Unknown: env $sub (use: env list)"
    list_envs
    exit 0
    ;;

  role)
    sub="${1:-}"
    [[ "$sub" == "list" ]] || die "Unknown: role $sub (use: role list)"
    list_roles
    exit 0
    ;;

  run)
    ;;

  *)
    die "Unknown command: $cmd (use: help)"
    ;;
esac

# ---- run command ----
DEFAULT_CONFIG="${SCRIPT_DIR}/.env"
CONFIG_FILE=""
ENV_NAME=""
ACTION=""
INVENTORY_FILE=""
ANSIBLE_LIMIT=""
ANSIBLE_CHECK=false
ANSIBLE_DIFF=false
NO_HOSTKEY_CHECK=false
ROLES=()
ANSIBLE_EXTRA_ARGS=()

while [[ $# -gt 0 ]]; do
  case "$1" in
    --config)
      CONFIG_FILE="${2:-}"
      [[ -n "$CONFIG_FILE" ]] || die "Missing value for --config"
      shift 2
      ;;
    --env)
      ENV_NAME="${2:-}"
      [[ -n "$ENV_NAME" ]] || die "Missing value for --env"
      shift 2
      ;;
    --action)
      ACTION="${2:-}"
      [[ -n "$ACTION" ]] || die "Missing value for --action"
      shift 2
      ;;
    -i|--inventory)
      INVENTORY_FILE="${2:-}"
      [[ -n "$INVENTORY_FILE" ]] || die "Missing value for $1"
      shift 2
      ;;
    --limit)
      ANSIBLE_LIMIT="${2:-}"
      [[ -n "$ANSIBLE_LIMIT" ]] || die "Missing value for --limit"
      shift 2
      ;;
    --check)
      ANSIBLE_CHECK=true
      shift 1
      ;;
    --diff)
      ANSIBLE_DIFF=true
      shift 1
      ;;
    --dry-run)
      ANSIBLE_CHECK=true
      ANSIBLE_DIFF=true
      shift 1
      ;;
    --no-hostkey-check)
      NO_HOSTKEY_CHECK=true
      shift 1
      ;;
    --role)
      r="${2:-}"
      [[ -n "$r" ]] || die "Missing value for --role"
      ROLES+=("$r")
      shift 2
      ;;
    --roles)
      csv="${2:-}"
      [[ -n "$csv" ]] || die "Missing value for --roles"
      while IFS= read -r rr; do
        ROLES+=("$rr")
      done < <(split_csv_roles "$csv")
      shift 2
      ;;
    --)
      shift 1
      ANSIBLE_EXTRA_ARGS+=("$@")
      break
      ;;
    *)
      die "Unknown argument: $1 (use: help)"
      ;;
  esac
done

# load config (explicit first; else default if exists)
if [[ -n "$CONFIG_FILE" ]]; then
  load_dotenv "$CONFIG_FILE"
elif [[ -f "$DEFAULT_CONFIG" ]]; then
  load_dotenv "$DEFAULT_CONFIG"
fi

# allow config defaults
ENV_NAME="${ENV_NAME:-${GOONE_ENV:-}}"
ACTION="${ACTION:-${GOONE_ACTION:-}}"
INVENTORY_FILE="${INVENTORY_FILE:-${GOONE_INVENTORY:-}}"
ANSIBLE_LIMIT="${ANSIBLE_LIMIT:-${GOONE_LIMIT:-}}"

[[ -n "$ENV_NAME" ]] || die "Missing --env (or set GOONE_ENV in deploy/.env)"
[[ -n "$ACTION" ]] || die "Missing --action (or set GOONE_ACTION in deploy/.env)"

VALID_ACTIONS=("init" "push" "start" "stop" "restart")
is_valid=false
for a in "${VALID_ACTIONS[@]}"; do
  if [[ "$ACTION" == "$a" ]]; then
    is_valid=true
    break
  fi
done
[[ "$is_valid" == true ]] || die "Invalid --action '$ACTION'. Supported: ${VALID_ACTIONS[*]}"

require_cmd ansible-playbook

PLAYBOOK_FILE="${SCRIPT_DIR}/playbook_dev/${ENV_NAME}.yml"
require_file "$PLAYBOOK_FILE"

if [[ -z "$INVENTORY_FILE" ]]; then
  if [[ -f "${SCRIPT_DIR}/hosts/host_${ENV_NAME}.txt" ]]; then
    INVENTORY_FILE="${SCRIPT_DIR}/hosts/host_${ENV_NAME}.txt"
  else
    INVENTORY_FILE="${SCRIPT_DIR}/hosts/host_dev.txt"
  fi
fi
require_file "$INVENTORY_FILE"

if [[ "$NO_HOSTKEY_CHECK" == true ]]; then
  export ANSIBLE_HOST_KEY_CHECKING=False
fi

# default roles: active role set only
if [[ ${#ROLES[@]} -eq 0 ]]; then
  for r in "${ACTIVE_DEFAULT_ROLES[@]}"; do
    ROLES+=("$r")
  done
fi

# validate role names (warn only)
for r in "${ROLES[@]}"; do
  if [[ ! -d "${SCRIPT_DIR}/roles/${r}" ]]; then
    log_warn "Unknown role '${r}' (no directory: roles/${r})"
  fi
done

# build tags: role_action,role_action...
TAGS=()
for r in "${ROLES[@]}"; do
  TAGS+=("${r}_${ACTION}")
done
TAG_STR="$(join_by "," "${TAGS[@]}")"

echo "${COLOR_BOLD}${COLOR_BLUE}========== GoOne Deploy ==========${COLOR_RESET}"
log_info "Env       : ${ENV_NAME}"
log_info "Action    : ${ACTION}"
log_info "Inventory : ${INVENTORY_FILE}"
log_info "Roles     : ${ROLES[*]}"
log_info "Tags      : ${TAG_STR}"
if [[ -n "$ANSIBLE_LIMIT" ]]; then
  log_info "Limit     : ${ANSIBLE_LIMIT}"
fi
if [[ "$ANSIBLE_CHECK" == true ]]; then
  log_warn "Mode      : --check"
fi
if [[ "$ANSIBLE_DIFF" == true ]]; then
  log_warn "Mode      : --diff"
fi

TMP=""
cleanup() {
  if [[ -n "${TMP:-}" && -f "$TMP" ]]; then
    rm -f "$TMP"
  fi
}
trap cleanup EXIT

TMP="$(mktemp_compat "${SCRIPT_DIR}/.tmpXXXXXX.myl")"
cp "$PLAYBOOK_FILE" "$TMP"

ANSIBLE_ARGS=(--tags "$TAG_STR")
if [[ -n "$ANSIBLE_LIMIT" ]]; then
  ANSIBLE_ARGS+=(--limit "$ANSIBLE_LIMIT")
fi
if [[ "$ANSIBLE_CHECK" == true ]]; then
  ANSIBLE_ARGS+=(--check)
fi
if [[ "$ANSIBLE_DIFF" == true ]]; then
  ANSIBLE_ARGS+=(--diff)
fi
if [[ ${#ANSIBLE_EXTRA_ARGS[@]} -gt 0 ]]; then
  ANSIBLE_ARGS+=("${ANSIBLE_EXTRA_ARGS[@]}")
fi

ansible-playbook -i "$INVENTORY_FILE" "$TMP" "${ANSIBLE_ARGS[@]}"
log_ok "Deploy completed."
