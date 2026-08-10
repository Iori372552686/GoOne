## GoOne

GoOne 是一套基于 Go 实现的微服务分布式游戏服务器框架，核心思路是 **Reactor + CSP**（并发消息驱动）且继承了很多C++游戏架构的思想，并配套提供：服务治理、配置中心、消息总线、网络层、部署控制台等“工程化”能力，适用于中小型游戏、MMO 等游戏后端业务。

[Go Version](go.mod)
[License](LICENSE)
[Stars](https://github.com/Iori372552686/GoOne/stargazers)

---

## 1. 架构概览

image

- **网关服（connsvr）**：管理客户端连接，接入并转发消息
- **核心逻辑服（mainsvr）**：核心业务逻辑
- **信息服（infosvr）**：角色/信息等轻量查询服务
- **数据服务（mysqlsvr）**：存储、持久化、数据访问
- **房间中心（roomcentersvr）**：房间生命周期、房间 tick、房间编排
- **Web 管理（websvr）**：HTTP / gRPC 管理接口与对外 API

更详细的运行时与协议说明见：

- `[docs/ssrpc_idl.md](docs/ssrpc_idl.md)`（ssrpc / IDL 方案）
- `[docs/modernization_execution_plan_2026-07-v4.md](docs/modernization_execution_plan_2026-07-v4.md)`（V4 生产闭环现代化实施计划：当前主计划）
- `[docs/architecture_review_2026-07-v2.md](docs/architecture_review_2026-07-v2.md)`（架构复审 v2：生命周期、模块化与性能治理 · 历史归档，已被 v4 取代）
- `[docs/optimization_roadmap.md](docs/optimization_roadmap.md)`（核心现代化 P0/P1/P2 执行计划 · 历史归档，已被 v4 取代）
- `[docs/STYLE.md](docs/STYLE.md)`（代码风格规范）
- `[docs/observability/README.md](docs/observability/README.md)`（可观测性：指标 / Trace / 日志）
- `[docs/benchmarks/baseline.md](docs/benchmarks/baseline.md)`（性能基线与优化对比）

---

## 2. 优势

- **工程化**：提供统一的控制台入口 `main.sh`，把常用操作标准化（doctor/go/docker/build/deploy）
- **可扩展**：注册中心 / 配置中心 / 消息总线均采用接口 + 工厂抽象，便于替换实现
- **易落地**：提供 `deploy/`（Ansible）与 `etc/env/`（Docker 依赖环境）开箱即用
- **高性能导向**：网络层支持 tcp/kcp/ws（及 gnet 实现），多服务按 busId 路由通信

---

## 3. 功能模块

- **ssrpc 运行时**：`lib/service/ssrpc/*` -- IDL 驱动的统一 RPC runtime / codegen（当前 SSPacket/HTTP/WS 已接入，gRPC 支持 unary + server-streaming）
- **网络层**：`lib/net/*`（tcp/kcp/ws，及基于 gnet 的实现，含接入管理层 `net_mgr`）
- **服务注册发现**：`lib/contrib/registry/*`（默认 ZooKeeper，支持 etcd/consul/nacos/k8s 等）
- **配置中心**：`lib/contrib/config/*`（支持 apollo/etcd/consul/nacos/k8s 等，并提供本地 Manager 聚合/热更新）
- **消息总线（Bus）**：`lib/service/bus/*`（RabbitMQ / NSQ / NATS / Kafka / RocketMQ，统一 `IBus` 接口）
- **路由与服务编排**：`lib/service/router/*`、`lib/service/svrinstmgr/*`
- **部署与运维**：`deploy/*`（Ansible），`etc/env/*`（依赖服务 Docker compose）
- **工具链**：`tools/protoc-gen-goone`（IDL 代码生成器）、`tools/cmd/genproto`（主仓 proto 生成入口）、`tools/cmd/scaffold`（脚手架）、`tools/cfgtool/*`（配置工具）

---

## 3.1 ssrpc -- IDL 驱动的统一 RPC 运行时

GoOne 的 ssrpc 模块借鉴 CloudWeGo 的 IDL 驱动思路与 Kratos 的 middleware / transport 分层，嫁接到现有的 `TransactionMgr + SSPacket` 执行模型上。

更完整的现状说明见 `[docs/ssrpc_idl.md](docs/ssrpc_idl.md)`。


| 传输层      | Wrap 函数                                       | 挂载点                | Proto option            | 当前状态                                                |
| -------- | --------------------------------------------- | ------------------ | ----------------------- | --------------------------------------------------- |
| SSPacket | `WrapUnary`                                   | `TransactionMgr`   | `cmd/cmd_enum/cmd_name` | 主通路                                                 |
| HTTP     | `WrapHTTPGin`                                 | `gin.IRoutes`      | `http_path`             | 已在 `web_svr` 使用                                     |
| WS       | `WrapWS`                                      | `ssrpc.Dispatcher` | `ws: true`              | 已在 `connsvr` 登录前置使用                                 |
| gRPC     | `WrapGRPCUnary` / `WrapGRPCServerStreamTyped` | `grpc.Server`      | `grpc: true`            | runtime / codegen ready，支持 unary + server-streaming |


**关键组件：**

- `**ssrpc.Dispatcher`**：统一注册中心；生成器会产出 `Register<Service>ToDispatcher(...)`
- `**ssrpc.Context**`：统一请求上下文，middleware 不用关心底层 transport
- `**ssrpc.Middleware**`：`func(next Handler) Handler` 链式中间件
- `**New<Service>SServer(...)**`：统一默认中间件链入口
- `**protoc-gen-goone**`：从 proto service 生成 server 注册与 client stub
- `**ssrpc.CallByCmd` / `CallByCmdWithRouter` / `SendByCmdSimple**`：生成 client stub 依赖的 helper

**当前协议归属：**

- `common/`（submodule `g1_common`）：共享消息、枚举、`CMD`、主线业务 service proto（源在 `common/game_proto/`，生成在 `common/protocol/`），以及 cfgtool 工具链（`common/game_conf/`）与运行时配置数据（`common/game_data/`）
- `api/proto/goone/`**、`api/proto/game/**`、存在时的 `api/proto/web/**`：repo-owned proto 输入
- `api/gen/**`：统一生成产物

**当前推荐生成命令：**

```bash
go run ./tools/cmd/genproto
```

包装脚本：

```bash
./scripts/proto_goone.sh
# Windows / PowerShell
./scripts/proto_goone.ps1
```

校验生成物：

```bash
./main.sh check-genproto
./main.sh check-genproto --full
```

Windows / PowerShell：

- `.\scripts\check_genproto.ps1`
- `.\scripts\check_genproto.ps1 -Full`

**推荐接入方式：**

1. 用生成代码里的 `New<Service>SServer(...)` 创建服务端实现；
2. 创建 `ssrpc.Dispatcher`；
3. 调用 `Register<Service>ToDispatcher(...)` 完成注册；
4. 如果走 SSPacket 主通路，再调用 `d.RegisterToTransactionMgr(globals.TransMgr)` 挂到事务管理器上。

**当前边界：**

- HTTP-only method 可以不配置 cmd
- gRPC unary method 可以不配置 cmd
- `ws` 当前仍要求有 cmd 绑定
- gRPC server-streaming method 当前必须是 grpc-only
- generator 当前支持 gRPC unary 与 server-streaming 自动注册；client-streaming / bidi-streaming 尚未支持
- gRPC 通路（option 字段 `grpc` / `grpc_service` 定义于 `api/proto/goone/options/v1/options.proto`，runtime `WrapGRPCUnary` / `WrapGRPCServerStreamTyped` / `MountGRPC` 与 codegen 均就绪）当前**尚无业务 method 显式声明 `grpc: true`**；`common/game_proto/service/websvr.proto` 的 `Ping` / `MsgSecCheck` 目前均走 HTTP（`http_path`），未来按需 opt-in 即可
- `web_svr` 已接入可选的 `grpc_server` listener 配置（`enabled` / `port` / `reflection`），其他服务若要对外监听 gRPC，仍需各自在 app 层挂载
- `web_svr` 的 gRPC listener 已附带标准 health / reflection，便于本地调试

---

## 3.2 脚手架工具

快速创建新服务骨架：

```bash
go run tools/cmd/scaffold -name mysvr
```

生成 `src/mysvrsvr/` 目录（`main.go` / `app.go` / `globals` / `cmd_handler`），与 `infosvr` 结构一致。
当前脚手架仍会保留一个兼容用的 `cmd_handler/register.go` 骨架；如果新服务走 IDL-first，通常应在 `app.go` 里接生成的 `Register<Service>ToDispatcher(...)` / `Register<Service>ToTransactionMgr(...)`。

---

## 4. 快速开始（推荐：通过主控制台 main.sh）

> `main.sh` 是统一入口（类似 “框架控制台”）。在 Windows 上建议使用 **WSL2 或 Git-Bash** 来执行这些 bash 脚本。

### 4.1 自检（强烈推荐先跑）

```bash
./main.sh doctor
./main.sh help
```

### 4.2 安装/切换 Go 版本（可选，但推荐与 go.mod 对齐）

项目 `go.mod` 当前要求 Go `1.25.4`。

```bash
./main.sh go list
./main.sh go install 1.25.4
./main.sh go use 1.25.4
./main.sh go current
```

### 4.3 一键拉起依赖环境（MySQL / Redis / ZooKeeper / RabbitMQ）

**本地开发（推荐：直接 docker compose）**：

```bash
docker compose -f etc/env/env_docker.yaml up -d
```

**远程/统一管理（通过 main.sh + Ansible）**：

```bash
# 需要先在部署机安装 ansible（只需做一次）
./main.sh install ansible

./main.sh docker install --env dev_local
./main.sh docker status  --env dev_local
```

> Docker 配置来自：`[etc/env/env_docker.yaml](etc/env/env_docker.yaml)`

### 4.4 编译

`main.sh` 会调用仓库根目录的 `build.sh`；`build.sh` 现在只保留当前活跃服务的构建目标：`conn`、`main`、`info`、`mysql`、`roomcenter`、`web`（也兼容 `connsvr`、`roomcentersvr`、`web_svr` 这类别名）。

```bash
./main.sh build          # 默认编译全部当前活跃服务
./main.sh build web      # 单独编译 websvr（target 由 build.sh 定义）
./main.sh build roomcenter
./build.sh list          # 查看当前活跃 target

# 如果配置里使用了 etcd:// 作为服务注册地址，需要带 registry_etcd build tag
GO_BUILD_TAGS=registry_etcd ./main.sh build conn
```

Windows / PowerShell 可直接使用对应构建脚本：

```powershell
.\build.ps1
.\build.ps1 web
.\build.ps1 roomcenter

# 如果配置里使用了 etcd:// 作为服务注册地址，需要带 registry_etcd build tag
$env:GO_BUILD_TAGS = 'registry_etcd'; .\build.ps1 conn
```

### 4.5 本地运行（IDE/调试）

各服务通过统一 flag 读取配置（`-svr_conf`），示例配置见：

- `etc/config/server_conf_ide.yaml`

示例（按需启动）：

```bash
./build/connsvr  -svr_conf=./etc/config/server_conf_ide.yaml
./build/mainsvr  -svr_conf=./etc/config/server_conf_ide.yaml
./build/infosvr  -svr_conf=./etc/config/server_conf_ide.yaml
./build/mysqlsvr -svr_conf=./etc/config/server_conf_ide.yaml
./build/roomcentersvr -svr_conf=./etc/config/server_conf_ide.yaml
./build/websvr   -svr_conf=./etc/config/server_conf_ide.yaml
```

Windows 下如果使用 `build.ps1`，对应可执行文件通常在 `build\*.exe`，例如：

```powershell
.\build\connsvr.exe -svr_conf=.\etc\config\server_conf_ide.yaml
.\build\roomcentersvr.exe -svr_conf=.\etc\config\server_conf_ide.yaml
.\build\websvr.exe -svr_conf=.\etc\config\server_conf_ide.yaml
```

### 4.6 配置生成（xlsx → 配置数据 / proto / 查询代码）

策划在 `common/game_conf/xls/*.xlsx`（唯一数据源）填表后，通过 cfgtool 生成：

- 运行时配置数据 `.conf` → `common/game_data/`
- 配置表 proto → `common/game_proto/config/`
- Go 查询代码 → `module/gamedata/repository/`

**`main.sh xls` 默认生成 server 版本**（mode=server）：

```bash
./main.sh xls            # 默认 server
./main.sh xls client     # 仅客户端标记字段（输出目录见 common/gen_xls_client.bat）
./main.sh xls all        # 全部字段
```

> xlsx 第 4 行（标记行）控制字段归属：`key`（主键索引，任何模式都含）/ `all` / `client` / `server`，留空则仅 `all` 模式包含。

**Windows 一键脚本**（在 `common/` 下，环境配置集中在各 bat 顶部可改）：

```bat
.\common\gen_xls.bat           REM 全量（mode=all）
.\common\gen_xls_server.bat    REM 一键导出 server（默认）
.\common\gen_xls_client.bat    REM 一键导出 client（输出暂为 %TEMP%\g1_client_output\，后期改 bat 即可）
```

> 详细说明见 `[common/game_conf/readme.md](common/game_conf/readme.md)`；编译 cfgtool：`./main.sh build cfgtool`。

---

## 5. 部署（Ansible）

部署相关脚本在 `deploy/`，推荐仍通过 `main.sh` 调度：

### 5.1 安装 Ansible（部署机/控制机）

```bash
./main.sh install ansible
```

### 5.2 列出环境与角色

```bash
./main.sh env list
./main.sh role list
```

### 5.3 部署/重启服务

```bash
./main.sh deploy --env dev1 --action restart --role websvr
./main.sh deploy --env dev1 --action restart --roles websvr,mainsvr --dry-run
```

更多说明：

- 部署说明：`[deploy/README.md](deploy/README.md)`

---

## 6. 安装与环境（Linux / Windows）

- Linux / macOS / WSL：优先使用 `./main.sh doctor`、`./main.sh build`、`docker compose -f etc/env/env_docker.yaml up -d`
- Windows：本地构建可直接使用仓库根目录的 `build.ps1`；proto 校验/生成继续使用 `scripts/*.ps1`；如需运行 `main.sh`，优先使用 WSL 或 Git-Bash

> 提示：历史文档里部分 “手动安装 Go 1.13 / 手动装中间件” 已过时；当前更推荐使用 `main.sh go ...` 与 `main.sh docker ...` 统一管理。

---

## 7. 目录结构（Top Level）

```text
api/        IDL 定义 (api/proto/) 与生成代码 (api/gen/)
build/      编译后的可执行文件输出目录
common/     公共模块（配置/常量/工具/游戏数据等）
deploy/     Ansible 自动化部署脚本与 roles
docs/       文档（架构/环境搭建/IDL 设计等）
etc/        环境与本地调试配置（`etc/env/` docker compose；`etc/config/` 示例 `server_conf_ide.yaml`）
common/     协议子仓 g1_common（共享消息 / CMD / protocol-owned service proto / cfgtool / game_data）
lib/        框架核心库
  lib/service/ssrpc/    ssrpc 运行时（Context/Dispatcher/Middleware/Wrap*/Client）
  lib/service/bus/      消息总线（RabbitMQ/NATS/Kafka/...）
  lib/service/router/   路由与服务编排
  lib/net/              网络层（tcp/kcp/ws，含 gnet 实现与 net_mgr 接入层）
  lib/contrib/          注册中心 / 配置中心
src/        业务服务（connsvr/mainsvr/infosvr/web_svr/roomcentersvr/mysqlsvr）
tools/      工具链
  tools/protoc-gen-goone/   IDL 代码生成器（protoc 插件）
  tools/cmd/scaffold/       脚手架工具（生成新服务骨架）
  tools/cmd/genproto/       proto 编译脚本
  tools/cfgtool/            配置工具
main.sh     主控制台脚本（推荐入口）
```

---

## 8. 常见问题（FAQ）

- **在 Windows 上怎么运行 main.sh？**
  - 建议使用 **WSL2** 或 **Git-Bash**；如果只是本地构建当前活跃服务，可以直接运行 `./build.ps1`。
- **本地 go test 会因为 MQ/注册中心没启动而失败吗？**
  - 某些包是集成测试性质，建议先启动 `etc/env/env_docker.yaml` 中的依赖后再联调。
- **部署 inventory/账号密码在哪里配置？**
  - 在 `deploy/hosts/`* 与 `deploy/playbook_dev/*`；生产环境务必使用安全的凭据管理方式，避免明文提交。

---

## 9. 交流与贡献

- 联系 QQ：372552686
- QQ 群：767770895

---

## 10. License

GoOne 使用 MIT 协议发布，详见 `[LICENSE](LICENSE)`。