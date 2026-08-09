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
  conn        -> cmd/connsvr          -> build/connsvr
  main        -> cmd/mainsvr          -> build/mainsvr
  info        -> cmd/infosvr          -> build/infosvr
  mysql       -> cmd/mysqlsvr         -> build/mysqlsvr
  roomcenter  -> cmd/roomcentersvr    -> build/roomcentersvr
  web         -> cmd/web_svr          -> build/websvr
  tester      -> tools/tester/cmd/tester -> build/tester
  stress      -> tools/tester/cmd/stress -> build/stress
  cfgtool     -> tools/cfgtool        -> common/game_conf/cfgtool

Environment:
  GO_BUILD_TAGS=tag1,tag2   pass extra build tags (e.g. config_etcd for cfgtool etcd upload)

Aliases:
  connsvr, mainsvr, infosvr, mysqlsvr, room, roomcentersvr, websvr, web_svr
EOF
}

build_one() {
  local source_dir="$1"
  local output_name="$2"
  echo "building ${output_name} !"
  (cd "${project_root_dir}/${source_dir}" && go build ${GO_BUILD_TAGS:+-tags "${GO_BUILD_TAGS}"} -o "${project_root_dir}/build/${output_name}")
}

connsvr() { build_one "cmd/connsvr" "connsvr"; }
mainsvr() { build_one "cmd/mainsvr" "mainsvr"; }
infosvr() { build_one "cmd/infosvr" "infosvr"; }
mysqlsvr() { build_one "cmd/mysqlsvr" "mysqlsvr"; }
roomcentersvr() { build_one "cmd/roomcentersvr" "roomcentersvr"; }
websvr() { build_one "cmd/web_svr" "websvr"; }
tester() { build_one "tools/tester/cmd/tester" "tester"; }
stress() { build_one "tools/tester/cmd/stress" "stress"; }

# cfgtool 输出到 common/game_conf/（跨平台带后缀），支持 GO_BUILD_TAGS=config_etcd
cfgtool() {
  local out_dir="${project_root_dir}/common/game_conf"
  local exe_name="cfgtool"
  if [[ "$(go env GOOS)" == "windows" ]]; then
    exe_name="cfgtool.exe"
  fi
  mkdir -p "${out_dir}"
  echo "building cfgtool -> ${out_dir}/${exe_name} ${GO_BUILD_TAGS:+(tags: ${GO_BUILD_TAGS})}!"
  (cd "${project_root_dir}" && go build ${GO_BUILD_TAGS:+-tags "${GO_BUILD_TAGS}"} -o "${out_dir}/${exe_name}" ./tools/cfgtool)
}

run_all() {
  connsvr
  mainsvr
  mysqlsvr
  infosvr
  roomcentersvr
  websvr
  tester
  stress
}

case "${target}" in
  help|-h|--help)
    usage
    ;;
  list)
    printf '%s\n' conn main info mysql roomcenter web tester stress cfgtool
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
  tester)
    tester
    ;;
  stress)
    stress
    ;;
  cfgtool)
    cfgtool
    ;;
  *)
    echo "Unsupported build target: ${target}" >&2
    usage
    exit 1
    ;;
esac



