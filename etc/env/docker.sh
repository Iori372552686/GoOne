#!/bin/bash

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ROOT_DIR="$(cd "${SCRIPT_DIR}/.." && pwd)"

# Reuse deploy/common.sh for logging + dotenv support
# shellcheck source=../deploy/common.sh
source "${ROOT_DIR}/deploy/common.sh"

usage() {
  cat <<EOF
${COLOR_BOLD}GoOne Docker Env CLI${COLOR_RESET}

Usage:
  $0 help
  $0 <install|up|restart|down|status|logs> [options] [-- <extra ansible args...>]

Options:
  -e, --env <group>         target group in deploy/hosts/host_dev.txt (dev1/dev2/dev_local)
  -i, --inventory <file>    inventory file (default: deploy/hosts/host_dev.txt)
  -l, --limit <pattern>     ansible --limit (must match hosts in inventory)
  --compose <file>          compose yaml to upload (default: etc/env/env_docker.yaml)
  --remote-dir <dir>        remote directory to store docker-compose.yml (default: /data/GoOne/env)
  --project <name>          docker compose project name (default: goone-env)
  --check                   ansible --check
  --diff                    ansible --diff
  --dry-run                 same as --check --diff
  --no-hostkey-check        set ANSIBLE_HOST_KEY_CHECKING=False

Examples:
  # install docker + upload compose + up
  $0 install --env dev1

  # restart docker daemon + keep services up
  $0 restart --env dev1

  # show compose status
  $0 status --env dev1

  # run only on one host inside the group
  $0 install --env dev1 --limit 43.139.3.228

EOF
}

cmd="${1:-help}"
shift || true

case "$cmd" in
  help|-h|--help)
    usage
    exit 0
    ;;
  install|up|restart|down|status|logs)
    ;;
  *)
    die "Unknown command: $cmd (use: help)"
    ;;
esac

ENV_GROUP=""
INVENTORY_FILE="${ROOT_DIR}/deploy/hosts/host_dev.txt"
LIMIT=""
COMPOSE_FILE="${SCRIPT_DIR}/env_docker.yaml"
REMOTE_DIR="/data/GoOne/env"
PROJECT="goone-env"
ANSIBLE_CHECK=false
ANSIBLE_DIFF=false
NO_HOSTKEY_CHECK=false
ANSIBLE_EXTRA_ARGS=()

while [[ $# -gt 0 ]]; do
  case "$1" in
    -e|--env)
      ENV_GROUP="${2:-}"
      [[ -n "$ENV_GROUP" ]] || die "Missing value for $1"
      shift 2
      ;;
    -i|--inventory)
      INVENTORY_FILE="${2:-}"
      [[ -n "$INVENTORY_FILE" ]] || die "Missing value for $1"
      shift 2
      ;;
    -l|--limit)
      LIMIT="${2:-}"
      [[ -n "$LIMIT" ]] || die "Missing value for $1"
      shift 2
      ;;
    --compose)
      COMPOSE_FILE="${2:-}"
      [[ -n "$COMPOSE_FILE" ]] || die "Missing value for $1"
      shift 2
      ;;
    --remote-dir)
      REMOTE_DIR="${2:-}"
      [[ -n "$REMOTE_DIR" ]] || die "Missing value for $1"
      shift 2
      ;;
    --project)
      PROJECT="${2:-}"
      [[ -n "$PROJECT" ]] || die "Missing value for $1"
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

require_cmd ansible-playbook
require_file "$INVENTORY_FILE"
require_file "${SCRIPT_DIR}/docker-playbook.yml"
require_file "$COMPOSE_FILE"

if [[ "$NO_HOSTKEY_CHECK" == true ]]; then
  export ANSIBLE_HOST_KEY_CHECKING=False
fi

[[ -n "$ENV_GROUP" ]] || die "Missing --env <group> (e.g. --env dev1)"

echo "${COLOR_BOLD}${COLOR_BLUE}========== GoOne Docker Env ==========${COLOR_RESET}"
log_info "Action    : $cmd"
log_info "Env group : $ENV_GROUP"
log_info "Inventory : $INVENTORY_FILE"
log_info "Compose   : $COMPOSE_FILE"
log_info "Remote dir: $REMOTE_DIR"
log_info "Project   : $PROJECT"
if [[ -n "$LIMIT" ]]; then
  log_info "Limit     : $LIMIT"
fi
if [[ "$ANSIBLE_CHECK" == true ]]; then
  log_warn "Mode      : --check"
fi
if [[ "$ANSIBLE_DIFF" == true ]]; then
  log_warn "Mode      : --diff"
fi

ANSIBLE_ARGS=(-e "target_group=${ENV_GROUP}" -e "docker_action=${cmd}" -e "docker_compose_dir=${REMOTE_DIR}" -e "docker_project=${PROJECT}" -e "docker_compose_src=${COMPOSE_FILE}")
if [[ -n "$LIMIT" ]]; then
  ANSIBLE_ARGS+=(--limit "$LIMIT")
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

ansible-playbook -i "$INVENTORY_FILE" "${SCRIPT_DIR}/docker-playbook.yml" "${ANSIBLE_ARGS[@]}"
log_ok "Docker env action completed."


