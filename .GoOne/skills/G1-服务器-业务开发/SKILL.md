---
name: "G1-服务器-业务开发"
description: "基于技术分析设计文档进行业务功能开发和迭代。当技术方案确认后需要落地编码、进行 proto 文件设计生成、配置表注册、ssrpc 协议注册、业务代码开发、自测优化及输出开发报告时调用。"
---

# G1-服务器-业务开发

## 概述

本技能在**技术分析方案（技术分析文档 + 技术设计文档）确认并通过后**，基于已评审的技术方案，进行完整的业务功能编码落地。覆盖从 proto 文件设计、代码生成、ssrpc 注册、业务代码开发、自测优化到最终输出开发报告的全流程。

> **定位边界**：
> - ✅ 基于已确认的技术分析方案进行编码落地
> - ✅ 设计和迭代 proto 文件（配置表 proto、通讯协议 proto、ssrpc service proto）
> - ✅ 运行代码生成工具，注册配置表和 ssrpc handler
> - ✅ 按照各服务的开发规范编写业务代码
> - ✅ 完成自测：存储分析、协议性能推导、配置表反向校验
> - ✅ 迭代优化直到代码闭环
> - ✅ 输出开发报告
> - ❌ 不涉及技术方案设计（那是"G1-服务器-技术分析设计"技能的职责）
> - ❌ 不审查 PRD 逻辑（那是"G1-服务器-需求分析"技能的职责）
> - ❌ 不修改 `lib/` 框架层代码
> - ❌ 不上线部署、不做运维操作

## 适用场景

- 技术分析方案评审通过后，启动编码开发
- 需要为某个需求新增配置表、通讯协议、数据库结构
- 需要基于已有服务扩展新功能模块
- 需要新建一个服务来承载新的业务需求
- 完成编码后需要进行自测和优化迭代
- 需要输出开发报告供团队归档

---

## 前置条件

本技能依赖技术分析方案已输出并确认。开始前必须确认：

1. 知识库中 `{analysis_output_dir}/{需求名称}/技术分析方案.md` 已存在且状态为"已确认"
2. 知识库中 `{analysis_output_dir}/{需求名称}/需求分析报告.md` 已存在且审查通过
3. 需求的致命和严重问题已全部闭环
4. 如果技术分析方案不存在或未确认，提示用户先使用"G1-服务器-技术分析设计"技能完成技术方案设计

---

## 规范遵循（强制）

> 在进行配置表 Proto、通讯协议、数据库存储等数据结构的设计或优化时，**先去 `.GoOne/spec/` 目录下自行寻找并读取相关规范文档**（01~06），严格遵循其中的规则和约定。技能不固化具体规范文件名，以目录下实际文档为准。文档缺失时应提示用户补充。

## 项目技术要素速查

在进行业务开发前，必须先了解 GoOne 的核心技术要素：

| 要素 | 说明 | 涉及路径/模块 |
|------|------|-------------|
| **配置表 proto** | xlsx → `common/game_proto/config/` → `.conf` → `module/gamedata` | `common/game_conf/xls/`、`common/game_data/`、`module/gamedata/repository/` |
| **通讯协议 proto（core）** | 共享消息、枚举、CMD | `common/game_proto/core/` |
| **service proto（ssrpc IDL）** | 各服务的 rpc 定义 + ssrpc option | `common/game_proto/service/` |
| **存储 proto** | KV 抽象（dbsvr） | `common/game_proto/storage/` |
| **生成代码** | ssrpc wrapper + .pb.go（禁止手改） | `api/gen/`、`common/protocol/` |
| **配置加载** | `module/conf`（不可变快照 + Get/Unmarshal） | `module/conf/`、`module/gconf/` |
| **路由** | `ServerType_*` + `ServerRouteRules` + cmd 编码 | `module/misc/` |
| **框架核心** | runtime/bussvc/ssrpc/transaction/router | `lib/service/` |
| **DB** | Redis（`lib/db/redis`）、MySQL（`lib/db/xorm`） | `lib/db/` |

---

## 工作流程

### 第一步：获取项目配置

调用 `G1-项目配置` 技能获取：
- `vault_path` / `prd_dir` / `analysis_output_dir`

### 第二步：读取技术方案

读取 `{analysis_output_dir}/{需求名称}/技术分析方案.md`，提取：
- 涉及的服务（connsvr/mainsvr/infosvr/mysqlsvr/roomcentersvr/web_svr）
- 需要新增/修改的 proto（core/service/config/storage）
- 需要新增的 cmd、ssrpc method
- 涉及的配置表、数据库表/Redis key
- 组件设计、交互流程

### 第三步：proto 设计与生成

#### 3.1 通讯协议 proto（service + ssrpc option）

在 `common/game_proto/service/<svc>.proto` 中定义/修改 rpc method，配 ssrpc option：

```proto
rpc NewMethod(g1.protocol.NewReq) returns (g1.protocol.NewRsp) {
  option (goone.options.v1.ssrpc) = {
    cmd_name: "CMD_MAIN_NEW_REQ"   // 引用 common/protocol.CMD 常量
    auth: true                      // 是否需要鉴权
    comment: "新增方法"
  };
}
```

如需新 cmd，先在 `common/game_proto/core/cmd.proto` 的 `CMD` 枚举里加（遵循 ServerType 号段，见 [02-通讯协议规范文档.md](../../spec/02-通讯协议规范文档.md)）。

#### 3.2 代码生成

```bash
# 主仓 proto 生成
go run ./tools/cmd/genproto

# 子模块（g1_common）proto 生成（如改了 common/game_proto/）
./main.sh proto game

# 校验（生成后必须无 diff）
./main.sh check-genproto
./main.sh check-genproto --full    # 连 common/protocol 一起校验

# 如新增了 cmd，重新生成 cmd 镜像
go run ./tools/cmd/gencmdproto
```

> ⚠️ 禁止手改 `api/gen/**` 与 `common/protocol/*.pb.go`。改 proto 后必须跑生成 + 校验。

#### 3.3 配置表 proto（如需）

见 [01-游戏配置规范文档.md](../../spec/01-游戏配置规范文档.md)。用 cfgtool（一键入口 `./main.sh xls` 或 `common/game_conf/run_me.sh`）从 xlsx 生成 proto + `.conf`（落到 `common/game_data/`）+ 查询代码。禁止手改 `module/gamedata/repository/**/*.gen.go`。

### 第四步：ssrpc handler 注册

在目标服务 `src/<svc>svr/app.go` 的 `NewApp()` 中，经 `ssrpc.NewRegistryComponent` 注册：

```go
registerHandlers := ssrpc.NewRegistryComponent(
    "ssrpc_registry",
    func(r *ssrpc.Registry) error {
        srv := <svc>v1.New<Svc>SServer(&service.Impl{}, ssrpc.DefaultMWOptions{})
        return <svc>v1.Register<Svc>ToRegistry(r, srv)
    },
    ssrpc.WithTransactionManager(globals.TransMgr),
)
app.MustRegister(registerHandlers, ...)
```

> ⚠️ CI 静态门禁禁止生产代码用 `RegisterXxxToDispatcher` / `RegisterXxxToTransactionMgr` / `TransMgr.RegisterCmd`。

### 第五步：业务代码开发

按目标服务的开发规范编写业务代码（参考 `specs/` 下对应服务的开发规范）：

- **connsvr**：网关层（登录接入、Session、KickOut/Broadcast、AdmissionController）
- **mainsvr**：核心业务（`RoleMgr`、玩家业务、房间操作）
- **infosvr**：缓存/查询（BriefInfo/IconDesc）
- **mysqlsvr**：持久化（ORM 表、异步写库）
- **roomcentersvr**：房间生命周期/tick（德州玩法）
- **web_svr**：对外 HTTP(Gin)/gRPC 接口

#### 代码规范（`docs/STYLE.md` 硬性要求）

- 命名：包名全小写；服务目录 `<name>svr` 连写；文件名 snake_case
- 注释统一中文；导出符号注释以符号名开头
- 错误处理：统一 `lib/api/gerr`（或 `ssrpc.E`/`Wrap`）；禁裸 `errors.New` 穿越 RPC
- 并发：串行化优先复用 TransactionMgr serial key；channel 入队必须有界
- 热路径：Debug 日志加 `logger.DebugEnabled()` 前置；禁每消息 `make`/goroutine
- 配置：新配置只加 grouped 结构（`base_cfg.*`）；启动不可变
- 生命周期（P0）：每个资源归属唯一 Component；每个 goroutine 能被 Stop/Drain 取消并 join

### 第六步：自测

#### 6.1 编译与单测

```bash
go build ./...                                    # 编译
go test -count=1 -timeout 600s ./...              # 单测（不依赖外部中间件）
go test -race ./src/<svc>svr/...                  # 竞态检测
```

#### 6.2 集成测试

```bash
GOONE_INTEGRATION=1 go test ./...                 # 需真实 mysql/redis/zk/rabbitmq
```

#### 6.3 客户端联调（`tools/tester/`）

```bash
# 新业务测试组件放 tools/tester/app/component/<name>/，component.Register(...) 注册
go run ./tools/tester/cmd/tester -config ./tools/tester/tester.toml    # regression
go run ./tools/tester/cmd/stress  -config ./tools/tester/stress.toml   # stress
```

#### 6.4 自测维度

- **协议**：proto 字段编号无冲突；cmd 号段正确；生成代码无 diff
- **存储**：Redis key / MySQL 表设计合理；落盘/加载流程完整
- **配置表**：xlsx → .conf → repository 链路通；查询 API 正确
- **边界**：参数校验、并发行为（serial key）、异常路径

### 第七步：生成代码校验

```bash
./main.sh check-genproto --full     # 生成代码与 proto 一致
go run ./tools/cmd/checkdocs ./docs # docs 断链检查
go vet ./...                        # 静态检查
```

### 第八步：输出开发报告

在 `{analysis_output_dir}/{需求名称}/开发报告.md` 输出：

```markdown
# {需求名称} — 开发报告
> 基于 {服务名} 业务开发规范

## 一、变更清单
| 类型 | 路径 | 说明 |
|------|------|------|
| proto | common/game_proto/service/<svc>.proto | 新增 NewMethod |
| cmd | common/game_proto/core/cmd.proto | 新增 CMD_<SVR>_NEW_REQ |
| 代码 | src/<svc>svr/service/ | handler 实现 |

## 二、新增/修改的协议 / 配置表 / 存储结构
## 三、自测结果（编译/单测/集成/联调）
## 四、遗留问题与后续优化
```

---

## 设计原则

1. **方案先行**：技术方案确认后才编码
2. **IDL 驱动**：proto 定义是契约，改 proto 必须跑生成 + 校验
3. **生成代码边界**：禁手改 `api/gen`、`*.pb.go`、`*.gen.go`
4. **ssrpc 注册标准化**：走 `RegisterXxxToRegistry`，禁旧 API（CI 门禁）
5. **分层约束**：`lib/`/`module/` 禁引用 globals；`cmd/` 只做入口
6. **配置不可变**：启动后只 gamedata 热更
7. **自测闭环**：编译 + 单测 + 集成 + 联调全过才算完成

---

## 相关规范文档

本技能 `specs/` 目录下有各服务的业务开发规范（按 GoOne 真实服务组织）：

| 规范文档 | 服务 |
|---------|------|
| [specs/connsvr-业务开发规范.md](specs/connsvr-业务开发规范.md) | connsvr（网关） |
| [specs/mainsvr-业务开发规范.md](specs/mainsvr-业务开发规范.md) | mainsvr（核心业务） |
| [specs/infosvr-业务开发规范.md](specs/infosvr-业务开发规范.md) | infosvr（缓存/查询） |
| [specs/mysqlsvr-业务开发规范.md](specs/mysqlsvr-业务开发规范.md) | mysqlsvr（持久化） |
| [specs/roomcentersvr-业务开发规范.md](specs/roomcentersvr-业务开发规范.md) | roomcentersvr（房间） |
| [specs/web_svr-业务开发规范.md](specs/web_svr-业务开发规范.md) | web_svr（对外接口） |
