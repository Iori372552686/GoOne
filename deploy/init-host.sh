#!/bin/bash

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
# shellcheck source=common.sh
source "${SCRIPT_DIR}/common.sh"

usage() {
  cat <<EOF
${COLOR_BOLD}GoOne Init-Host CLI${COLOR_RESET}

Usage:
  $0 help
  $0 [hosts...] [options] [-- <extra ansible args...>]
  $0 run [hosts...] [options] [-- <extra ansible args...>]

Options:
  --config <file>        dotenv config (default: ${SCRIPT_DIR}/.env if exists)
  -e, --env <dev>        use deploy/hosts/host_dev.txt and target group <dev> (dev1/dev2/dev_local)
  -i, --inventory <file> inventory file (default: inithost/host.txt)
  -p, --playbook <file>  playbook file (default: inithost/inithost.yml)
  --variant <ubuntu|centos> shortcut to select playbook (centos uses inithost/init_centos_bak.yaml)
  -l, --limit <pattern>  ansible --limit
  -H, --host <host>      shortcut for adding a host to limit (repeatable)
  --hosts <a,b,c>        shortcut for multiple hosts (comma-separated)
  --adhoc                generate a temporary inventory from provided hosts (no need to edit host.txt)
  -u, --user <user>      ssh user for adhoc inventory (default: root)
  -k, --key <path>       ssh private key for adhoc inventory (ansible_ssh_private_key_file)
  --password <pass>      ssh password for adhoc inventory (ansible_ssh_pass)
  --become-password <p>  sudo password for adhoc inventory (ansible_sudo_pass)
  -P, --port <port>      ssh port for adhoc inventory (ansible_port)
  --check                ansible --check
  --diff                 ansible --diff
  --dry-run              same as --check --diff
  --no-hostkey-check     set ANSIBLE_HOST_KEY_CHECKING=False

Examples:
  $0
  $0 192.168.50.250
  $0 host1 host2
  $0 --hosts 192.168.50.250,192.168.50.251
  $0 --env dev1
  $0 --env dev2
  $0 --adhoc -u root -k ~/.ssh/id_ed25519 43.139.3.228
  $0 --variant centos 192.168.50.250
  $0 --dry-run 192.168.50.250 -- -vv
EOF
}

split_csv_hosts() {
  local csv="$1"
  local old_ifs="$IFS"
  IFS=',' read -r -a _parts <<< "$csv"
  IFS="$old_ifs"
  local h
  for h in "${_parts[@]}"; do
    h="$(trim "$h")"
    [[ -n "$h" ]] && echo "$h"
  done
}

# Create a temporary inventory from given hosts with ssh auth settings.
make_adhoc_inventory() {
  local tmp_file="$1"
  shift
  local hosts=("$@")

  {
    echo "[dst]"
    for h in "${hosts[@]}"; do
      line="$h ansible_ssh_user=${ADHOC_USER}"
      if [[ -n "$ADHOC_PORT" ]]; then
        line="${line} ansible_port=${ADHOC_PORT}"
      fi
      if [[ -n "$ADHOC_KEY" ]]; then
        line="${line} ansible_ssh_private_key_file=${ADHOC_KEY}"
      fi
      if [[ -n "$ADHOC_PASSWORD" ]]; then
        line="${line} ansible_ssh_pass=${ADHOC_PASSWORD}"
      fi
      if [[ -n "$ADHOC_BECOME_PASSWORD" ]]; then
        line="${line} ansible_sudo_pass=${ADHOC_BECOME_PASSWORD}"
      fi
      echo "$line"
    done
  } > "$tmp_file"
}

cmd="${1:-run}"
shift || true

case "$cmd" in
  help|-h|--help)
    usage
    exit 0
    ;;
  run)
    ;;
  *)
    # If the first arg is not a known command, treat it as a host pattern for convenience:
    #   ./init-host.sh 192.168.50.250
    #   ./init-host.sh host1 host2
    set -- "$cmd" "$@"
    cmd="run"
    ;;
esac

DEFAULT_CONFIG="${SCRIPT_DIR}/.env"
CONFIG_FILE=""
ENV_NAME=""
INVENTORY_FILE=""
PLAYBOOK_FILE=""
VARIANT=""
ANSIBLE_LIMIT=""
HOSTS=()
ADHOC_INVENTORY=false
ADHOC_USER="root"
ADHOC_KEY=""
ADHOC_PASSWORD=""
ADHOC_BECOME_PASSWORD=""
ADHOC_PORT=""
ANSIBLE_CHECK=false
ANSIBLE_DIFF=false
NO_HOSTKEY_CHECK=false
ANSIBLE_EXTRA_ARGS=()

while [[ $# -gt 0 ]]; do
  case "$1" in
    --config)
      CONFIG_FILE="${2:-}"
      [[ -n "$CONFIG_FILE" ]] || die "Missing value for --config"
      shift 2
      ;;
    -e|--env)
      ENV_NAME="${2:-}"
      [[ -n "$ENV_NAME" ]] || die "Missing value for $1"
      shift 2
      ;;
    -i|--inventory)
      INVENTORY_FILE="${2:-}"
      [[ -n "$INVENTORY_FILE" ]] || die "Missing value for $1"
      shift 2
      ;;
    -p|--playbook)
      PLAYBOOK_FILE="${2:-}"
      [[ -n "$PLAYBOOK_FILE" ]] || die "Missing value for $1"
      shift 2
      ;;
    --variant)
      VARIANT="${2:-}"
      [[ -n "$VARIANT" ]] || die "Missing value for --variant"
      shift 2
      ;;
    -l|--limit)
      ANSIBLE_LIMIT="${2:-}"
      [[ -n "$ANSIBLE_LIMIT" ]] || die "Missing value for $1"
      shift 2
      ;;
    -H|--host)
      h="${2:-}"
      [[ -n "$h" ]] || die "Missing value for $1"
      HOSTS+=("$h")
      shift 2
      ;;
    --hosts)
      csv="${2:-}"
      [[ -n "$csv" ]] || die "Missing value for $1"
      while IFS= read -r hh; do
        HOSTS+=("$hh")
      done < <(split_csv_hosts "$csv")
      shift 2
      ;;
    --adhoc)
      ADHOC_INVENTORY=true
      shift 1
      ;;
    -u|--user)
      ADHOC_USER="${2:-}"
      [[ -n "$ADHOC_USER" ]] || die "Missing value for $1"
      shift 2
      ;;
    -k|--key)
      ADHOC_KEY="${2:-}"
      [[ -n "$ADHOC_KEY" ]] || die "Missing value for $1"
      shift 2
      ;;
    --password)
      ADHOC_PASSWORD="${2:-}"
      [[ -n "$ADHOC_PASSWORD" ]] || die "Missing value for $1"
      shift 2
      ;;
    --become-password)
      ADHOC_BECOME_PASSWORD="${2:-}"
      [[ -n "$ADHOC_BECOME_PASSWORD" ]] || die "Missing value for $1"
      shift 2
      ;;
    -P|--port)
      ADHOC_PORT="${2:-}"
      [[ -n "$ADHOC_PORT" ]] || die "Missing value for $1"
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
      # Treat bare args as hosts for convenience, until '--' or options appear.
      HOSTS+=("$1")
      shift 1
      ;;
  esac
done

# load config
if [[ -n "$CONFIG_FILE" ]]; then
  load_dotenv "$CONFIG_FILE"
elif [[ -f "$DEFAULT_CONFIG" ]]; then
  load_dotenv "$DEFAULT_CONFIG"
fi

INVENTORY_FILE="${INVENTORY_FILE:-${GOONE_INIT_INVENTORY:-${SCRIPT_DIR}/inithost/host.txt}}"
PLAYBOOK_FILE="${PLAYBOOK_FILE:-${GOONE_INIT_PLAYBOOK:-${SCRIPT_DIR}/inithost/inithost.yml}}"

if [[ -n "$VARIANT" ]]; then
  case "$VARIANT" in
    ubuntu)
      PLAYBOOK_FILE="${SCRIPT_DIR}/inithost/inithost.yml"
      ;;
    centos)
      PLAYBOOK_FILE="${SCRIPT_DIR}/inithost/init_centos_bak.yaml"
      ;;
    *)
      die "Invalid --variant '$VARIANT' (supported: ubuntu|centos)"
      ;;
  esac
fi

if [[ "$NO_HOSTKEY_CHECK" == true ]]; then
  export ANSIBLE_HOST_KEY_CHECKING=False
fi

# If env is set, default to deploy/hosts/host_dev.txt and use that group as play target
if [[ -n "$ENV_NAME" ]]; then
  if [[ -z "${INVENTORY_FILE:-}" || "$INVENTORY_FILE" == "${SCRIPT_DIR}/inithost/host.txt" ]]; then
    INVENTORY_FILE="${SCRIPT_DIR}/hosts/host_dev.txt"
  fi
fi

# If user provided hosts and did not explicitly set --limit, turn hosts into an ansible --limit.
# Multiple hosts are joined by ':' (ansible union pattern).
if [[ -z "$ANSIBLE_LIMIT" && ${#HOSTS[@]} -gt 0 ]]; then
  ANSIBLE_LIMIT="$(join_by ":" "${HOSTS[@]}")"
fi

require_cmd ansible-playbook
require_file "$PLAYBOOK_FILE"

# Optional: create adhoc inventory so --limit can match and ssh key/password can be provided without editing host.txt
TMP_INVENTORY=""
cleanup_inventory() {
  if [[ -n "${TMP_INVENTORY:-}" && -f "$TMP_INVENTORY" ]]; then
    rm -f "$TMP_INVENTORY"
  fi
}
trap cleanup_inventory EXIT

if [[ "$ADHOC_INVENTORY" == true ]]; then
  [[ ${#HOSTS[@]} -gt 0 ]] || die "--adhoc requires hosts (e.g. ./init-host.sh --adhoc 1.2.3.4)"

  if [[ -n "$ADHOC_KEY" && ! -f "$ADHOC_KEY" ]]; then
    die "SSH key not found: $ADHOC_KEY"
  fi

  TMP_INVENTORY="$(mktemp_compat "${SCRIPT_DIR}/.tmp-inventory-XXXXXX.ini")"
  make_adhoc_inventory "$TMP_INVENTORY" "${HOSTS[@]}"
  INVENTORY_FILE="$TMP_INVENTORY"
fi

require_file "$INVENTORY_FILE"

echo "${COLOR_BOLD}${COLOR_BLUE}========== GoOne Init Host ==========${COLOR_RESET}"
log_info "Inventory: $INVENTORY_FILE"
log_info "Playbook : $PLAYBOOK_FILE"
if [[ -n "$ANSIBLE_LIMIT" ]]; then
  log_info "Limit    : $ANSIBLE_LIMIT"
fi
if [[ "$ANSIBLE_CHECK" == true ]]; then
  log_warn "Mode     : --check"
fi
if [[ "$ANSIBLE_DIFF" == true ]]; then
  log_warn "Mode     : --diff"
fi

ANSIBLE_ARGS=()
if [[ -n "$ENV_NAME" ]]; then
  ANSIBLE_ARGS+=(-e "target_group=${ENV_NAME}")
fi
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

ansible-playbook -i "$INVENTORY_FILE" "$PLAYBOOK_FILE" "${ANSIBLE_ARGS[@]}"
log_ok "Init host completed."


