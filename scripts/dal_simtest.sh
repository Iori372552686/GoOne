#!/usr/bin/env bash
# DAL 三层持久化模拟测试编排：
#   0) 中间件连通性预检（不可达则退出，等环境恢复后重跑）
#   1) 构建并启动 mysqlsvr → mainsvr → connsvr
#   2) tester phase=solo：20 个全链路用例
#   3) tester phase=write → dalprobe wipe-l2（模拟 L2 丢失）→ phase=verify（L3 权威恢复）
#   4) dalprobe check-l2：verify 后 L2 回填 + TTL 生效
# 用法：bash scripts/dal_simtest.sh   （在仓库根目录执行）
set -uo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT"

MW_HOST="${MW_HOST:-43.139.3.228}"
REDIS_PASS="${REDIS_PASS:-go123456}"
MYSQL_DSN="${MYSQL_DSN:-g1_game:go123456@tcp(${MW_HOST}:3306)/g1_game}"
CONF="$ROOT/etc/config/server_conf_ide.yaml"
OUT="$ROOT/build/dalsim"
UID_PROBE=100001

mkdir -p "$OUT"
PASS_CNT=0; FAIL_CNT=0
note() { echo "[dalsim $(date +%H:%M:%S)] $*"; }
fail() { echo "[dalsim $(date +%H:%M:%S)] FAIL: $*" | tee -a "$OUT/failures.log"; FAIL_CNT=$((FAIL_CNT+1)); }
pass() { PASS_CNT=$((PASS_CNT+1)); }

# ---------- 0. 中间件预检 ----------
# 端口对齐 server_conf：etcd 12379 / Redis 6379 / MySQL 3306 /
# RabbitMQ 15672（本环境 bus_mq_addr 的 AMQP 端口即 15672）。
note "probing middleware ${MW_HOST} ..."
for p in 6379 3306 12379 15672; do
  if ! (timeout 4 bash -c "echo > /dev/tcp/${MW_HOST}/${p}" 2>/dev/null); then
    note "port ${p} unreachable —— 中间件未就绪，稍后重跑本脚本"
    exit 2
  fi
done
note "middleware OK"

# ---------- 1. 构建与启动 ----------
note "building servers + tester + dalprobe ..."
go build -o "$OUT/mysqlsvr" ./cmd/mysqlsvr || exit 1
go build -o "$OUT/mainsvr" ./cmd/mainsvr || exit 1
go build -o "$OUT/connsvr" ./cmd/connsvr || exit 1
go build -o "$OUT/dalprobe"  ./tools/cmd/dalprobe   || exit 1

PIDS=()
cleanup() {
  for pid in "${PIDS[@]:-}"; do kill "$pid" 2>/dev/null; done
}
trap cleanup EXIT

start_svc() { # name
  note "starting $1"
  "$OUT/$1" -svr_conf "$CONF" > "$OUT/$1.log" 2>&1 &
  PIDS+=($!)
}

start_svc mysqlsvr; sleep 3
start_svc mainsvr;  sleep 3
start_svc connsvr

# 等 connsvr 监听（TCP 网关在 11001；11000 是 WS）
for i in $(seq 1 20); do
  if (timeout 2 bash -c "echo > /dev/tcp/127.0.0.1/11001" 2>/dev/null); then break; fi
  sleep 1
done
if ! (timeout 2 bash -c "echo > /dev/tcp/127.0.0.1/11001" 2>/dev/null); then
  fail "connsvr 未监听 11001（查看 $OUT/*.log）"
  note "=== connsvr.log tail ==="; tail -20 "$OUT/connsvr.log" || true
  note "=== mainsvr.log tail ===";  tail -20 "$OUT/mainsvr.log"  || true
  exit 1
fi
note "servers up (connsvr:11000)"

# tester 的 phase 由派生 toml 控制（testcfg 不支持模块参数环境变量覆盖）。
# 取 tester.toml 的 [run]/[server]/[player] 头部（截断于首个 [modules. 段），
# 端口保持原样：tcp_port=11001（connsvr 的 TCP 监听；11000 是 WS，勿改），
# 再追加本编排需要的模块。
gen_toml() { # phase outfile
  awk '/^\[modules\./{exit} {print}' "$ROOT/tools/tester/tester.toml" > "$2"
  cat >> "$2" <<EOF

[modules.login]
enabled = true

[modules.persist]
enabled = true
phase = "$1"
EOF
}
gen_toml solo   "$OUT/tester_solo.toml"
gen_toml write  "$OUT/tester_write.toml"
gen_toml verify "$OUT/tester_verify.toml"

run_tester() { # toml tag
  note "tester phase=$2 running ..."
  if go run ./tools/tester/cmd/tester -config "$1" > "$OUT/tester_$2.log" 2>&1; then
    note "tester phase=$2 PASSED"; pass
  else
    fail "tester phase=$2 失败（详见 $OUT/tester_$2.log）"
    tail -30 "$OUT/tester_$2.log" || true
  fi
}

# ---------- 2. solo 全链路 ----------
run_tester "$OUT/tester_solo.toml" solo

# ---------- 3. L3 权威验证（write → wipe → verify）----------
run_tester "$OUT/tester_write.toml" write

note "dalprobe check-l3（快照应已落库）"
if "$OUT/dalprobe" -op check-l3 -uid $UID_PROBE -mysql.dsn "$MYSQL_DSN" > "$OUT/probe_l3.log" 2>&1; then
  note "L3 snapshot OK"; cat "$OUT/probe_l3.log"; pass
else
  fail "L3 快照缺失"; cat "$OUT/probe_l3.log" || true
fi

note "dalprobe wipe-l2（模拟 TTL 过期/Redis 丢 key）"
if "$OUT/dalprobe" -op wipe-l2 -uid $UID_PROBE -redis.ip "$MW_HOST" -redis.pass "$REDIS_PASS" > "$OUT/probe_wipe.log" 2>&1; then
  cat "$OUT/probe_wipe.log"; pass
else
  fail "wipe-l2 失败"; cat "$OUT/probe_wipe.log" || true
fi

run_tester "$OUT/tester_verify.toml" verify

# ---------- 4. verify 后 L2 应回填且带 TTL ----------
note "dalprobe check-l2（L3 回源后 L2 应回填）"
if "$OUT/dalprobe" -op check-l2 -uid $UID_PROBE -redis.ip "$MW_HOST" -redis.pass "$REDIS_PASS" > "$OUT/probe_l2.log" 2>&1; then
  cat "$OUT/probe_l2.log"; pass
else
  fail "L2 回填异常"; cat "$OUT/probe_l2.log" || true
fi

# ---------- 汇总 ----------
note "=========== RESULT: pass=$PASS_CNT fail=$FAIL_CNT ==========="
[ "$FAIL_CNT" -eq 0 ] && echo "DAL SIMTEST ALL PASSED" || { echo "DAL SIMTEST HAS FAILURES"; exit 1; }
