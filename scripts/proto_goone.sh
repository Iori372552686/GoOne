#!/usr/bin/env bash
set -euo pipefail

# Canonical proto wrapper:
# 1) generate protocol-owned message pb.go inside common/protocol (g1_common submodule)
# 2) generate GoOne api/gen pb.go + ssrpc wrappers from tools/cmd/genproto

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT_DIR"

MODULE="${MODULE:-github.com/Iori372552686/GoOne}"
OUT_DIR="${OUT_DIR:-.}"
PROTO_ROOT="${PROTO_ROOT:-api/proto}"

if [[ -f "$ROOT_DIR/common/game_proto/gen_code.sh" ]]; then
  echo "[proto_goone] step 1/2: generate common/protocol (g1_common)"
  (
    cd "$ROOT_DIR/common/game_proto"
    bash ./gen_code.sh
  )
fi

echo "[proto_goone] step 2/2: generate GoOne api/gen via tools/cmd/genproto"
go run ./tools/cmd/genproto -module "$MODULE" -out "$OUT_DIR" -proto_root "$PROTO_ROOT"

echo "[proto_goone] done"

