# 贡献指南 / Contributing

感谢参与 GoOne！在提交代码前请阅读本指南。

## 开发环境

- Go 版本：以 `go.mod` 为准（`./main.sh go use <version>` 可切换）。
- 本地中间件（MySQL/Redis/ZooKeeper/RabbitMQ）：`docker compose -f etc/env/env_docker.yaml up -d`。
- 环境自检：`./main.sh doctor`（Windows 建议 WSL2/Git-Bash；本地构建可直接 `./build.ps1`）。

## 提交前检查清单

```bash
go build ./...
go vet -composites=false ./...
go test ./lib/... ./src/... -count=1
./main.sh check-genproto        # 若改动了 proto 或生成器
golangci-lint run --new-from-rev=origin/dev
```

- 代码风格遵循 `[docs/STYLE.md](docs/STYLE.md)`（新代码强制，CI lint 只检查增量）。
- 性能相关改动必须附 benchmark 前后对比，流程见 `[docs/benchmarks/baseline.md](docs/benchmarks/baseline.md)`。
- 修 bug 请先补一个能复现的测试。
- 集成测试必须带可达性预检查并 `t.Skip`，不得依赖外部环境才能通过 CI。

## 生成代码

以下目录禁止手改，修改源头后重新生成：


| 生成物                                      | 源头                                     | 生成命令                          |
| ---------------------------------------- | -------------------------------------- | ----------------------------- |
| `api/gen/**`                             | `api/proto/**`、`common/game_proto/*.proto` | `go run ./tools/cmd/genproto` |
| `common/protocol/*.pb.go`                | `common/game_proto/**`               | `./main.sh proto game`         |
| `module/gamedata/repository/**/*.gen.go` | 策划 xlsx（cfgtool）                       | `./main.sh xls`               |


## 提交信息

格式：`<type>: <summary>`，type ∈ `feat` / `fix` / `perf` / `refactor` / `test` / `docs` / `chore`。

正文说明"为什么"，关联 issue 用 `#<id>`。

## PR 要求

- 一个 PR 只做一件事；破坏性调整（目录移动、接口变更）单独提 PR。
- 描述中包含：动机、方案、测试方式、（性能项）benchmark 数据。
- CI 必须全绿（build / vet / test / check-genproto / lint 增量）。

## 讨论

- Issue / PR 均欢迎中英文。
- 联系方式见 `[readme.md](readme.md)` 第 9 节。

