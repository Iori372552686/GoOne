#!/bin/bash
# add by Iori 2020.9.1
# Maintained/optimized for robustness & usability

set -euo pipefail

# Root dir = script dir (not current working dir)
ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
DEPLOY_DIR="${ROOT_DIR}/deploy"
ENV_DIR="${ROOT_DIR}/etc/env"
SCRIPT_DIR="${ROOT_DIR}/scripts"

# Reuse deploy/common.sh for consistent logging & config parsing
# shellcheck source=deploy/common.sh
source "${DEPLOY_DIR}/common.sh"

title() {
  echo "${COLOR_BOLD}${COLOR_BLUE}$*${COLOR_RESET}"
}

hr() {
  echo "${COLOR_BLUE}------------------------------------------${COLOR_RESET}"
}

kv() {
  # kv "Key" "Value"
  local k="$1"
  local v="$2"
  printf "%b %-14s%b %s\n" "${COLOR_BOLD}" "${k}:" "${COLOR_RESET}" "${v}"
}

print_header() {
  #echo
  title "========== GoOne Console =========="
  kv "Root" "$ROOT_DIR"
  kv "Deploy dir" "$DEPLOY_DIR"
  hr
}

usage() {
  cat <<EOF
${COLOR_BOLD}GoOne Console${COLOR_RESET}  ${COLOR_CYAN}(main entry)${COLOR_RESET}

${COLOR_BOLD}Usage${COLOR_RESET}
  ./main.sh ${COLOR_CYAN}help${COLOR_RESET}
  ./main.sh ${COLOR_CYAN}doctor${COLOR_RESET}
  ./main.sh ${COLOR_CYAN}check-genproto${COLOR_RESET} [--full]
  ./main.sh ${COLOR_CYAN}proto${COLOR_RESET} [game|full|help]
  ./main.sh ${COLOR_CYAN}install${COLOR_RESET} ansible [--venv <dir>]
  ./main.sh ${COLOR_CYAN}go${COLOR_RESET} <install|list|current|use|uninstall|check|help> [args...]
  ./main.sh ${COLOR_CYAN}docker${COLOR_RESET} <install|up|restart|down|status|logs> --env <dev> [options...]

  ./main.sh ${COLOR_CYAN}build${COLOR_RESET} [target]
  ./main.sh ${COLOR_CYAN}env${COLOR_RESET} list
  ./main.sh ${COLOR_CYAN}role${COLOR_RESET} list

  ./main.sh ${COLOR_CYAN}deploy${COLOR_RESET}      --env <env> --action <init|push|start|stop|restart> [--role <role> ...] [deploy options...]
  ./main.sh ${COLOR_CYAN}host${COLOR_RESET} init  run  [init-host options...]

${COLOR_BOLD}Examples${COLOR_RESET}
  # list
  ./main.sh env list
  ./main.sh role list

  # proto
  ./main.sh proto
  ./main.sh proto game
  ./main.sh proto full

  # build
  ./main.sh build
  ./main.sh build web
  ./main.sh build roomcenter

  # deploy
  ./main.sh deploy --env dev1 --action restart --role websvr
  ./main.sh deploy --env dev1 --action restart --roles websvr,mainsvr --dry-run
  ./main.sh deploy --env dev1 --action push --limit 43.139.3.228 --role websvr -- -vv

  # init hosts
  ./main.sh host init
  ./main.sh host init --limit 192.168.50.250
  ./main.sh host init --variant centos --dry-run

  # go version manager (delegates to scripts/go-manager.sh)
  ./main.sh go list
  ./main.sh go current
  ./main.sh go install 1.25.0
  ./main.sh go use 1.25.0

  # docker env (mysql/redis/zookeeper/rabbitmq)
  ./main.sh docker install --env dev1
  ./main.sh docker restart --env dev1
  ./main.sh docker status --env dev1

${COLOR_BOLD}Notes${COLOR_RESET}
  - Most deploy/init-host options are forwarded to ${COLOR_CYAN}deploy/deploy.sh${COLOR_RESET} / ${COLOR_CYAN}deploy/init-host.sh${COLOR_RESET}.
  - ${COLOR_CYAN}build.sh${COLOR_RESET} builds the active services under ${COLOR_CYAN}src/${COLOR_RESET}: connsvr, mainsvr, infosvr, mysqlsvr, roomcentersvr, web_svr.
  - On Windows, prefer ${COLOR_CYAN}.\build.ps1${COLOR_RESET} for local builds and PowerShell proto helpers under ${COLOR_CYAN}scripts/*.ps1${COLOR_RESET}; use WSL/Git-Bash for ${COLOR_CYAN}main.sh${COLOR_RESET}.
  - Set ${COLOR_CYAN}NO_COLOR=1${COLOR_RESET} to disable colored output.
EOF
}

check_genproto() {
  local mode="${1:-}"
  print_header
  title "check-genproto"
  require_cmd git
  require_cmd go

  local module
  module="$(grep -E '^module[[:space:]]+' "${ROOT_DIR}/go.mod" | head -n1 | awk '{print $2}')"
  [[ -n "${module}" ]] || die "Cannot read module path from go.mod"

  if [[ "${mode}" == "--full" || "${mode}" == "full" ]]; then
    require_file "${ROOT_DIR}/scripts/proto_goone.sh"
    log_info "Running full proto generation: ./scripts/proto_goone.sh"
    (cd "$ROOT_DIR" && ./scripts/proto_goone.sh)

    log_info "Checking working tree: api/gen and game_protocol/protocol must match generator output"
    if ! (cd "$ROOT_DIR" && git diff --quiet -- api/gen game_protocol/protocol); then
      die "api/gen or game_protocol/protocol is out of date. Run: ./scripts/proto_goone.sh, then commit."
    fi

    log_ok "api/gen and game_protocol/protocol match full proto generation."
    return
  fi

  log_info "Running: go run ./tools/cmd/genproto (module=${module})"
  (cd "$ROOT_DIR" && go run ./tools/cmd/genproto -module "${module}" -out . -proto_root api/proto)

  log_info "Checking working tree: api/gen must match generator output"
  if ! (cd "$ROOT_DIR" && git diff --quiet -- api/gen); then
    die "api/gen is out of date. Run: go run ./tools/cmd/genproto  (or ./main.sh check-genproto --full / ./scripts/proto_goone.sh for game_protocol + api/gen), then commit."
  fi

  log_ok "api/gen matches genproto."
  hr
  log_info "Note: shared message pb.go under game_protocol/protocol is not validated in default mode; use ./main.sh check-genproto --full when protocol messages change."
}

doctor() {
  print_header
  title "Doctor"
  log_info "Checking tools & scripts..."

  kv "bash" "$(command -v bash 2>/dev/null || echo "${COLOR_YELLOW}NOT FOUND${COLOR_RESET}")"
  kv "go" "$(command -v go 2>/dev/null || echo "${COLOR_YELLOW}NOT FOUND${COLOR_RESET}")"
  if [[ -f "${SCRIPT_DIR}/go-manager.sh" ]]; then
    kv "go-manager" "${COLOR_GREEN}OK${COLOR_RESET}  (${SCRIPT_DIR}/go-manager.sh)"
  else
    kv "go-manager" "${COLOR_YELLOW}NOT FOUND${COLOR_RESET}  (${SCRIPT_DIR}/go-manager.sh)"
  fi

  if [[ -f "${ENV_DIR}/docker.sh" ]]; then
    kv "etc/env/docker.sh" "${COLOR_GREEN}OK${COLOR_RESET}  (${ENV_DIR}/docker.sh)"
  else
    kv "etc/env/docker.sh" "${COLOR_YELLOW}NOT FOUND${COLOR_RESET}  (${ENV_DIR}/docker.sh)"
  fi

  if command -v ansible-playbook >/dev/null 2>&1; then
    kv "ansible-playbook" "$(command -v ansible-playbook)"
  else
    kv "ansible-playbook" "${COLOR_YELLOW}NOT FOUND${COLOR_RESET}  (run: ./main.sh install ansible)"
  fi

  require_file "${ROOT_DIR}/build.sh"
  require_file "${DEPLOY_DIR}/deploy.sh"
  require_file "${DEPLOY_DIR}/init-host.sh"
  require_file "${DEPLOY_DIR}/install.sh"
  require_file "${SCRIPT_DIR}/go-manager.sh"
  require_file "${ENV_DIR}/docker.sh"
  require_file "${ENV_DIR}/docker-playbook.yml"
  require_file "${ENV_DIR}/env_docker.yaml"
  log_ok "Scripts OK: build.sh, deploy/*, scripts/*, etc/env/*"

  hr
  title "Inventory (quick view)"
  if [[ -f "${DEPLOY_DIR}/.env" ]]; then
    log_info "Found config: deploy/.env (auto-loaded by deploy scripts)"
  else
    log_warn "No deploy/.env found (optional)."
  fi

  log_info "Envs:"
  (cd "$DEPLOY_DIR" && ./deploy.sh env list 2>/dev/null | sed 's/^/  - /') || log_warn "Cannot list envs (deploy/playbook_dev missing?)"
  log_info "Roles:"
  (cd "$DEPLOY_DIR" && ./deploy.sh role list 2>/dev/null | sed 's/^/  - /') || log_warn "Cannot list roles (deploy/roles missing?)"

  hr
  log_ok "Doctor finished."
}

run_build() {
  local target="${1:-}"
  require_file "${ROOT_DIR}/build.sh"
  print_header
  log_info "Building... (target='${target:-all}')"
  (cd "$ROOT_DIR" && ./build.sh ${target:+$target})
  log_ok "Build done."
}

run_proto() {
  local sub="${1:-game}"
  print_header

  case "$sub" in
    game|"")
      require_cmd go
      require_file "${ROOT_DIR}/game_protocol/gen_code.sh"
      log_info "Generating game_protocol/protocol..."
      (cd "${ROOT_DIR}/game_protocol" && bash ./gen_code.sh)
      log_ok "game_protocol/protocol generated."
      ;;

    full)
      require_file "${ROOT_DIR}/scripts/proto_goone.sh"
      log_info "Running full proto generation (game_protocol + api/gen)..."
      (cd "$ROOT_DIR" && ./scripts/proto_goone.sh)
      log_ok "Full proto generation done."
      ;;

    help|-h|--help)
      cat <<EOF
${COLOR_BOLD}proto${COLOR_RESET} - generate protocol code

${COLOR_BOLD}Usage${COLOR_RESET}
  ./main.sh ${COLOR_CYAN}proto${COLOR_RESET} [game|full]

${COLOR_BOLD}Subcommands${COLOR_RESET}
  game  Generate pb.go under ${COLOR_CYAN}game_protocol/protocol${COLOR_RESET} from proto files in ${COLOR_CYAN}game_protocol/proto${COLOR_RESET} (default).
  full  Run the full pipeline: ${COLOR_CYAN}game_protocol/protocol${COLOR_RESET} plus ${COLOR_CYAN}api/gen${COLOR_RESET} via ${COLOR_CYAN}scripts/proto_goone.sh${COLOR_RESET}.

${COLOR_BOLD}Examples${COLOR_RESET}
  ./main.sh proto
  ./main.sh proto game
  ./main.sh proto full
EOF
      ;;

    *)
      die "Unknown proto subcommand: $sub (supported: game, full, help)"
      ;;
  esac
}

cmd="${1:-help}"
shift || true

case "$cmd" in
  help|-h|--help)
    usage
    ;;

  doctor)
    doctor
    ;;

  check-genproto)
    check_genproto "$@"
    ;;

  install)
    sub="${1:-}"
    shift || true
    case "$sub" in
      ansible)
        (cd "$DEPLOY_DIR" && ./install.sh ansible "$@")
        ;;
      *)
        die "Unknown install target: $sub (supported: ansible)"
        ;;
    esac
    ;;

  go)
    sub="${1:-help}"
    shift || true
    # Delegate to scripts/go-manager.sh (handles install/list/use/current/uninstall/check/help)
    (cd "${SCRIPT_DIR}" && ./go-manager.sh "$sub" "$@")
    ;;

  docker)
    sub="${1:-help}"
    shift || true
    (cd "${ENV_DIR}" && ./docker.sh "$sub" "$@")
    ;;

  build)
    run_build "${1:-}"
    ;;

  proto)
    run_proto "${1:-game}"
    ;;

  env)
    sub="${1:-}"
    [[ "$sub" == "list" ]] || die "Unknown: env $sub (use: env list)"
    (cd "$DEPLOY_DIR" && ./deploy.sh env list)
    ;;

  role)
    sub="${1:-}"
    [[ "$sub" == "list" ]] || die "Unknown: role $sub (use: role list)"
    (cd "$DEPLOY_DIR" && ./deploy.sh role list)
    ;;

  deploy)
    # Main console hides the internal "run" subcommand for better UX.
    # Supported:
    #   ./main.sh deploy --env ... --action ...
    # Still compatible:
    #   ./main.sh deploy run --env ... --action ...
    if [[ $# -eq 0 ]]; then
      (cd "$DEPLOY_DIR" && ./deploy.sh help)
      exit 0
    fi

    if [[ "${1:-}" == "run" ]]; then
      shift || true
    fi
    (cd "$DEPLOY_DIR" && ./deploy.sh run "$@")
    ;;

  host)
    sub="${1:-}"
    shift || true
    case "$sub" in
      init)
        # forward to init-host.sh
        (cd "$DEPLOY_DIR" && ./init-host.sh "$@")
        ;;
      *)
        die "Unknown host subcommand: $sub (supported: init)"
        ;;
    esac
    ;;

  *)
    log_error "Unknown command: $cmd"
    usage
    exit 1
    ;;
esac
