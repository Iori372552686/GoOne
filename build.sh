#!/bin/bash

set -euo pipefail

project_root_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
target="${1:-all}"

usage() {
  cat <<EOF
Usage:
  ./build.sh
  ./build.sh all
  ./build.sh list
  ./build.sh help
  ./build.sh <target>

Targets:
  conn        -> cmd/connsvr       -> build/connsvr
  main        -> cmd/mainsvr       -> build/mainsvr
  info        -> cmd/infosvr       -> build/infosvr
  mysql       -> cmd/mysqlsvr      -> build/mysqlsvr
  roomcenter  -> cmd/roomcentersvr -> build/roomcentersvr
  web         -> cmd/web_svr       -> build/websvr

Aliases:
  connsvr, mainsvr, infosvr, mysqlsvr, room, roomcentersvr, websvr, web_svr
EOF
}

build_one() {
  local source_dir="$1"
  local output_name="$2"
  echo "building ${output_name} !"
  (cd "${project_root_dir}/${source_dir}" && go build -o "${project_root_dir}/build/${output_name}")
}

connsvr() { build_one "cmd/connsvr" "connsvr"; }
mainsvr() { build_one "cmd/mainsvr" "mainsvr"; }
infosvr() { build_one "cmd/infosvr" "infosvr"; }
mysqlsvr() { build_one "cmd/mysqlsvr" "mysqlsvr"; }
roomcentersvr() { build_one "cmd/roomcentersvr" "roomcentersvr"; }
websvr() { build_one "cmd/web_svr" "websvr"; }

run_all() {
  connsvr
  mainsvr
  mysqlsvr
  infosvr
  roomcentersvr
  websvr
}

case "${target}" in
  help|-h|--help)
    usage
    ;;
  list)
    printf '%s\n' conn main info mysql roomcenter web
    ;;
  all|"")
    run_all
    ;;
  conn|connsvr)
    connsvr
    ;;
  main|mainsvr)
    mainsvr
    ;;
  info|infosvr)
    infosvr
    ;;
  mysql|mysqlsvr)
    mysqlsvr
    ;;
  roomcenter|room|roomcentersvr)
    roomcentersvr
    ;;
  web|websvr|web_svr)
    websvr
    ;;
  *)
    echo "Unsupported build target: ${target}" >&2
    usage
    exit 1
    ;;
esac



