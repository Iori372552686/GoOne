---
name: "G1-服务器-微服务开发"
description: "基于 GoOne 微服务框架（lib/ runtime+bussvc+ssrpc）进行业务微服务设计。当需要新建微服务、决定服务边界、选择无状态/有状态架构模式、设计数据库与路由方案时调用。"
---

# G1-服务器-微服务开发

## 概述

本技能在**技术分析方案确认**后，基于 GoOne 的微服务框架（`lib/service/` 下的 runtime/bussvc/router/svrinstmgr/bus/ssrpc/transaction），进行**业务微服务的架构设计**。核心目标：为每个业务需求精准判断无状态或有状态架构模式，以运营目标（DAU、并发）为驱动设计数据存储与路由方案，并从框架层面施加设计约束，确保服务标准化。

> **定位边界**：
> - ✅ 基于技术分析方案进行微服务架构设计决策
> - ✅ 精准判断服务应采用无状态还是有状态架构
> - ✅ 从框架层面约束无状态服务和有状态服务的开发规范
> - ✅ 基于运营目标（全球同服/大服模式 DAU）设计数据存储与部署方案
> - ✅ 设计服务实例路由策略（`SvrRouterRule_*`）
> - ✅ 制定微服务设计标准——约束而非发散
> - ✅ **定义项目目录结构规范**——`cmd/<svc>`（入口）+ `src/<svc>svr`（实现）结构及依赖约束
> - ❌ 不替代技术分析方案的设计维度（配置表/协议/模块接口）
> - ❌ 不涉及具体业务代码编写
> - ❌ 不修改 `lib/` 框架层代码
> - ❌ 不涉及运维部署操作

## 适用场景

- 技术分析方案中识别出需要新建一个微服务
- 需要决定某个业务模块应作为独立服务还是合并到已有服务
- 需要对已有服务做架构升级
- 需要为全球同服/大服模式设计数据存储方案
- DAU 目标提升，需要对现有架构做容量规划评估
- 新增业务模块需要选型：挂在 mainsvr 的 RoleMgr 下 vs 独立服务 vs 合并到已有无状态服务

---

## 前置条件

本技能依赖以下文档已存在：

1. 知识库中 `{analysis_output_dir}/{需求名称}/技术分析方案.md` 已存在且状态为"已确认"
2. 技术分析方案中已明确：需求分类、涉及业务模块、预估QPS、核心数据实体
3. 如果技术分析方案不存在，提示用户先使用"G1-服务器-技术分析设计"技能完成技术方案设计

---

## 规范遵循（强制）

> 在进行数据存储方案、路由策略、数据模型设计时，**先去 `.GoOne/spec/` 目录下自行寻找并读取相关规范文档**（尤其 `03-玩家服务数据库协议规范文档.md`、`04-游戏微服务数据库协议规范文档.md`），严格遵循其中的存储与路由约定。技能不固化具体规范文件名，以目录下实际文档为准。

## 框架核心能力速查

在设计微服务之前，必须理解 GoOne 框架（`lib/service/`）提供的核心能力边界：

### 服务类型与路由（`module/misc` + `lib/service/svrinstmgr`）

| 服务类型 | 路由模式 | 路由规则 | 典型服务 |
|---------|---------|---------|---------|
| **无状态服务** | 随机/负载均衡 | `SvrRouterRule_Random` | web_svr、chat |
| **有状态服务（UID 粘性）** | 按 UID 取模选定实例 | `SvrRouterRule_Hash_UID` | mainsvr、infosvr |
| **有状态服务（RouterID 粘性）** | 按 RouterID 取模选定实例 | `SvrRouterRule_Hash_RouterID` | mysqlsvr、roomcentersvr、TexasGameSvr |
| **排行榜类** | 按 ZoneID 取模 | `SvrRouterRule_Hash_ZoneID` | ranksvr |
| **主备** | 永远取第一个 | `SvrRouterRule_Master` | - |

> 一致性哈希变体：`ConsistentHash_UID`/`ConsistentHash_ZoneID`/`ConsistentHash_RouterID`、`IoCache_RouterID`。

### 事务与并发模型（`lib/service/transaction`）

GoOne **不使用 Actor 模型**，而是用 **sharded transaction**：

| 属性 | 约束 |
|------|------|
| **分片** | 请求按 `RouterID`/`Uid` 串行键分片；响应按 `DstTransID` 路由 |
| **Shard 数** | 固定 `DefaultShardCount()`（基于 `GOMAXPROCS`，上限 32），不可外部调参 |
| **同键 backpressure** | 默认 `DefaultMaxPendingPerKey = 100`；roomcentersvr 覆盖为 200 |
| **handler 约束** | **必须保持 key-local**（同一 RouterID/Uid 串行），禁止依赖全局单线程排序 |
| **goroutine 约束** | handler 内禁止与后台 goroutine 共享可变对象（参考 mainsvr `SelfLogoutSender` 模式：经事务串行化） |

### 消息流（AGENTS.md "Runtime Model"）

```
client → connsvr（TCP/WS/KCP + CSPacketHeader）
       → lib/service/router（按 cmd 的 ServerType + ServerRouteRules 选 svrinst）
       → bus（默认 rabbitmq）→ 目标服务 globals.TransMgr（sharded）
       → ssrpc handler（经 Dispatcher 查 cmd → middleware 链 → 业务 impl）
```

`web_svr` 例外：Gin HTTP 路由 + 可选 gRPC listener，不参与 bus 事务循环。

### 组件生命周期（`lib/service/runtime`）

```
状态机：Starting → Ready → (Allocated) → Draining → Stopping → Stopped / Failed
```

- `Component` 接口：`Name()` / `Start(ctx)` / `Stop(ctx)`
- 可选：`Quiescer`（停止接新请求）、`Drainer`（排空在途）、`RuntimeErrorSource`
- 注册顺序即 Start 顺序；Quiesce/Drain/Stop 按注册**逆序**
- `Start` 返回 nil 即可用，失败须自行清理（App 不调 Stop）；`Stop` 幂等
- admin 端点：`/statez` `/healthz` `/readyz` `/info` `/components`（Draining 时 healthz=200、readyz=503）

---

## 项目目录结构规范

所有微服务设计必须遵循以下项目目录结构约束，这是项目的**强制架构规范**（见 `AGENTS.md` 与 `docs/STYLE.md`）。

### 双层目录结构（入口 + 实现）

```
cmd/<svc>/              # [入口层] 程序入口
└── main.go             # flag.Parse() → <pkg>.NewApp().Run(ctx)

src/<svc>svr/           # [实现层] 服务装配与业务实现
├── app.go              # NewApp() *runtime.App（装配 Component）
├── globals/            # 服务级全局（TransMgr、RoleMgr、OrmMgr...）
├── service/            # ssrpc handler 实现（<Svc>ServiceImpl）
├── controller/         # （仅 web_svr）HTTP controller
└── <业务子目录>/        # role/、manager/、logic/ 等
```

> 目录命名：`<name>svr` 连写（`roomcentersvr`）。存量的 `web_svr` 保留下划线，但**新服务不得用下划线**（`docs/STYLE.md`）。

### 各层职责

| 层 | 目录 | 职责 |
|----|------|------|
| **入口层** | `cmd/<svc>/main.go` | flag 解析、logger flush、调用 `<pkg>.NewApp().Run(ctx)` |
| **实现层** | `src/<svc>svr/app.go` | `NewApp()` 装配 Component（`bussvc.MustNew` + `MustRegister`） |
| **实现层** | `src/<svc>svr/globals/` | 服务级全局变量（仅限 `src/<svc>svr/globals`，`lib/` 与 `module/` 禁引用 globals） |
| **实现层** | `src/<svc>svr/service/` | ssrpc handler 实现 |

### 依赖约束（强制）

| 编号 | 约束内容 | 类型 |
|------|---------|------|
| **IM-01** | `lib/` 与 `module/` 禁止引用任何 `src/<svc>svr/globals` | 必须 |
| **IM-02** | `globals.*` 仅限 `src/<svc>svr/globals` 包内定义 | 必须 |
| **IM-03** | `cmd/<svc>/main.go` 只做入口，业务逻辑在 `src/<svc>svr/` | 必须 |
| **IM-04** | 新服务用 `tools/cmd/scaffold -name <svc>` 生成（结构同 infosvr） | 建议 |

### 装配模板（所有 bus 服务一致）

```go
// src/<svc>svr/app.go
func NewApp() *runtime.App {
    app := bussvc.MustNew("<svc>", router.ReadyCheck, bussvc.WithConfLoader(<hooks>...))
    transMgr := &bussvc.TransMgrComponent{Mgr: globals.TransMgr}
    registerHandlers := ssrpc.NewRegistryComponent(
        "ssrpc_registry",
        func(r *ssrpc.Registry) error {
            srv := <svc>v1.New<Svc>SServer(&service.Impl{}, ssrpc.DefaultMWOptions{})
            return <svc>v1.Register<Svc>ToRegistry(r, srv)
        },
        ssrpc.WithTransactionManager(globals.TransMgr),
    )
    routerComp := bussvc.NewRouterComponent(app, globals.TransMgr, rabbitmq.NewRegistry())
    app.MustRegister(<业务Deps>, registerHandlers, transMgr, routerComp)
    return app
}
```

- 标准组件（datetime/logger/admin/tracing）由 `bussvc.MustNew` 集中注册。
- `bussvc.WithConfLoader(hooks...)`：`conf.Load` → `conf.RunValidators(svc)` → hooks（如 gamedata 加载）。
- web_svr 是例外：非 bus 服务，`MustNew("<svc>", nil, ...)`，装配 `webRuntimeComponent`（Gin + 可选 gRPC）。

---

## 工作流程

### 第一步：获取项目配置

调用 `G1-项目配置` 技能获取项目配置信息：

> - `vault_path` = Obsidian Vault 路径（定位知识库根目录）
> - `prd_dir` = 策划需求文档目录
> - `analysis_output_dir` = 技术分析输出目录
>
> **微服务设计输出目录**为 `{analysis_output_dir}/{需求名称}/`。

如果 `.GoOne/conf.json` 不存在，`G1-项目配置` 技能会自动引导创建。

---

### 第二步：收集输入信息

#### 2.1 从技术分析方案提取

1. **业务模块清单**：哪些业务模块需要承载
2. **数据实体清单**：每个模块操作哪些数据（Redis key / MySQL 表）
3. **协议清单**：哪些 cmd 需要新增
4. **预估 QPS**：每个协议的预估调用频率
5. **关联模块**：本模块与哪些已有模块有数据依赖或事件依赖

#### 2.2 从需求分析报告提取

1. **DAU 预期**：需求的用户规模（日活、峰值在线）
2. **数据增长预估**：单用户数据量、总体数据增长曲线
3. **可用性要求**：是否要求 7×24 高可用

#### 2.3 从现有服务架构收集

1. **已有服务清单**：当前部署了哪些服务，各自的 `ServerType` 与 cmd 号段
2. **已有数据存储**：当前 MySQL/Redis 的部署拓扑
3. **已有 cmd 号段分配**：避免冲突（见 `common/game_proto/core/cmd.proto`）

#### 2.4 存量分析：查找已有设计方案（优化/升级/完善场景）

##### 2.4.1 查找已有设计文档

1. **搜索知识库**：通过 obsidian-cli 搜索该服务的已有方案文档：
   ```bash
   obsidian vault="{vault_path}" search query="{服务名}"
   obsidian vault="{vault_path}" search query="{服务名} 设计方案"
   ```

2. **搜索已有 Skills 目录**：在 `.GoOne/skills/G1-服务器-业务开发/specs/` 下查找该服务的开发规范文档。

3. **搜索 docs/**：`docs/` 下可能有架构/优化文档（如 `docs/architecture_review_*.md`、`docs/optimization_roadmap.md`）。

##### 2.4.2 如果找到已有设计文档

1. 全文读取，提取：架构模式、`ServerType`/cmd 号段、数据存储方案、组件清单、交互协议
2. 与本次需求对比，识别变更点
3. 在本次设计文档中标注"基于存量文档 [[原设计方案路径]] 的增量设计"

##### 2.4.3 如果未找到已有设计文档

需要从现有代码反向分析，输出基线文档后再进行增量设计：

1. **分析代码结构**：
   - 读取 `src/<svc>svr/app.go` 理解装配方式与组件清单
   - 读取 `src/<svc>svr/globals/` 理解服务级全局
   - 读取 `src/<svc>svr/service/` 理解 ssrpc handler
   - 读取对应 `common/game_proto/service/<svc>.proto` 理解协议
   - 读取 `api/gen/game/<svc>/v1/` 理解生成代码

2. **反向输出基线文档**（遵循 `specs/` 下对应规范）：
   - `{服务名}-整体设计方案.md`：遵循 [整体设计规范](specs/微服务-整体设计规范.md)
   - `{服务名}-数据库设计方案.md`：遵循 [数据库设计规范](specs/微服务-数据库设计规范.md)
   - `{服务名}-交互设计方案.md`：遵循 [交互设计规范](specs/微服务-交互设计规范.md)
   - `{服务名}-功能组件化方案.md`：遵循 [有状态](specs/微服务-有状态功能组件化规范.md)/[无状态](specs/微服务-无状态功能组件化规范.md)组件化规范

3. **基线文档输出位置**：`{analysis_output_dir}/{服务名}/`（按服务名归档，与需求名称无关）

---

### 第三步：无状态 vs 有状态决策

#### 3.1 强制决策树

```
需要处理该用户的实时在线状态？
  ├── 是 → 需要频繁 Push 消息给客户端？
  │         ├── 是 → 需要维护该玩家的内存缓存以减少 DB 读取？
  │         │         ├── 是 → 【有状态服务】（如 mainsvr）
  │         │         └── 否 → 【有状态服务】（仍需 Push）
  │         └── 否 → 需要维护该玩家的内存缓存以减少 DB 读取？
  │                   ├── 是 → 【有状态服务】
  │                   └── 否 → 【无状态服务】
  └── 否 → 【无状态服务】
```

#### 3.2 决策速查矩阵

| 特征 | 无状态服务 | 有状态服务 |
|------|-----------|-----------|
| **数据访问模式** | 每次请求访问 DB | 数据加载到内存，定时回写 |
| **请求间状态** | 无依赖，任意节点可处理（`Random` 路由） | 强依赖，按 UID/RouterID 路由到同一实例（`Hash_*`） |
| **消息推送** | 经 connsvr 间接推送 | handler 内直接经事务推送 |
| **水平扩展** | 直接加实例 | 需要重新分配哈希分片 |
| **故障影响** | 单实例故障自动转移 | 故障节点上该 key 的在途事务受影响 |
| **内存占用** | 极低（无玩家缓存） | 高（每个在线玩家持有一份数据，如 mainsvr RoleMgr） |
| **DB 压力** | 高（每次请求读写 DB） | 低（内存命中，批量回写） |
| **并发模型** | 请求级隔离 | TransactionMgr sharded serial key（按 UID/RouterID 串行） |
| **典型业务** | 登录接入、web 接口、聊天 | 玩家业务、房间、缓存查询 |

#### 3.3 禁止模式（强制约束）

| 编号 | 禁止模式 | 原因 | 正确做法 |
|------|---------|------|---------|
| **F-01** | 无状态服务缓存玩家数据到进程内存 | 请求路由到不同实例，缓存不一致 | 使用 Redis 缓存或改为有状态服务 |
| **F-02** | 有状态服务不设置定时保存 | 节点宕机丢失内存数据 | 必须设置定时 Save（如 mainsvr role_tick 每分钟）+ 停机 Drainer 全量落盘 |
| **F-03** | handler 内做阻塞 IO 阻塞 serial key 队列 | 阻塞同 key 的所有后续消息 | 异步化或经事务投递 |
| **F-04** | handler 与后台 goroutine 共享可变对象 | 数据竞争 | 经事务串行化（参考 `SelfLogoutSender` 模式） |
| **F-05** | 同一 UID 的核心数据分散在多个服务 | 跨服务一致性问题 | 聚合在同一服务（如 mainsvr RoleMgr） |
| **F-06** | `lib/` 或 `module/` 引用 `src/<svc>svr/globals` | 循环依赖，破坏分层 | globals 只在 `src/<svc>svr` 内使用 |
| **F-07** | 生产代码用 `RegisterXxxToDispatcher`/`TransMgr.RegisterCmd` | CI 静态门禁禁止 | 走 `RegisterXxxToRegistry` + `ssrpc.NewRegistryComponent` |
| **F-08** | blank-import `bus/driver/all` | CI 门禁禁止 | 显式注册所需 driver（通常仅 rabbitmq） |

---

### 第四步：无状态微服务设计标准

#### 4.1 架构标准

| 标准编号 | 标准内容 | 类型 |
|---------|---------|------|
| **SL-01** | 每个请求独立处理，不依赖前一个请求的状态 | 必须 |
| **SL-02** | 业务数据经 `lib/db/redis` 或 ssrpc 跨服务调用读写，禁止进程内缓存玩家数据 | 必须 |
| **SL-03** | 路由规则用 `SvrRouterRule_Random`（或 `Master`） | 必须 |
| **SL-04** | handler 签名遵循 ssrpc 生成代码（`func(ctx *ssrpc.Context, req *Req) (*Rsp, error)`） | 必须 |
| **SL-05** | 跨服务调用经 ssrpc client（`<svc>v1.New<Svc>Client().Method(trans, &req)`） | 必须 |
| **SL-06** | 推送消息经 connsvr 的 `ConnService.Broadcast` 或下行 cmd | 必须 |

#### 4.2 容量规划公式

```
最大 QPS = min(DB连接池大小 / 平均DB耗时, CPU核心数 × 单核处理能力)
实例数 = 峰值QPS / 单实例最大QPS × 1.5（冗余系数）
```

---

### 第五步：有状态微服务设计标准

#### 5.1 架构标准

| 标准编号 | 标准内容 | 类型 |
|---------|---------|------|
| **SF-01** | 路由规则用 `Hash_UID`（按玩家）或 `Hash_RouterID`（按房间等） | 必须 |
| **SF-02** | 必须装配定时落盘 Component（如 mainsvr `roleTick` 每分钟） | 必须 |
| **SF-03** | 必须装配停机 Drainer 全量落盘（如 mainsvr `roleFlushComponent`） | 必须 |
| **SF-04** | handler 必须保持 key-local（同一 UID/RouterID 串行） | 必须 |
| **SF-05** | 禁止 handler 与后台 goroutine 共享可变对象（经事务串行化） | 必须 |
| **SF-06** | 同 key 队列 backpressure 需评估，高频场景可覆盖 `MaxPendingPerKey`（如 roomcentersvr=200） | 建议 |

#### 5.2 数据一致性标准

| 标准编号 | 标准内容 | 类型 |
|---------|---------|------|
| **SF-07** | 写操作先写内存，标记脏数据，定时批量回写（如 mainsvr Redis hash 增量） | 建议 |
| **SF-08** | 关键操作（货币、道具）先落库再更新内存，防止宕机丢失 | 必须 |

#### 5.3 容量规划公式

```
单节点最大在线 = 节点内存 / 单玩家平均内存占用 × 0.7（安全系数）
所需节点数 = 峰值在线 / 单节点最大在线 × 1.3（冗余系数）
```

#### 5.4 优雅下线标准

| 标准编号 | 标准内容 | 类型 |
|---------|---------|------|
| **SF-09** | 节点关闭前必须经 Drainer 全量落盘（state: Ready → Draining → Stopping） | 必须 |
| **SF-10** | Draining 时 readyz 返回 503（停止接新请求），healthz 仍 200 | 必须 |
| **SF-11** | 网关类（connsvr）须经 `SessionTracker.WaitSessions` 等待在途会话排空 | 必须 |

---

### 第六步：数据存储与路由设计

> ⚠️ GoOne **没有** dbrouter / 分库分表 / StrategyHash/Mod/Range 概念。分片只在**服务实例层**（`SvrRouterRule_*`），数据库是 xorm 主从 + Redis 多实例。详见 [03](../../spec/03-玩家服务数据库协议规范文档.md)、[04](../../spec/04-游戏微服务数据库协议规范文档.md) 主 spec。

#### 6.1 设计前置输入

| 输入参数 | 来源 | 说明 |
|---------|------|------|
| **DAU/PCU** | 运营目标 | 日活/峰值在线 |
| **单用户数据量** | 技术分析方案 | 单个用户的数据大小预估 |
| **读写比** | 技术分析方案 | 读/写比例 |
| **数据保留期** | 运营策略 | 历史数据保留时长 |

#### 6.2 存储选型决策

```
玩家主数据（频繁读写、需内存缓存）？
  ├── 是 → Redis hash（mainsvr 直连） + MySQL 少量关键字段（经 mysqlsvr）
  └── 否 → 关系型结构化数据（报表、记录）？
            ├── 是 → MySQL（mysqlsvr，xorm ORM）
            └── 否 → 缓存/排行榜/计数？
                      ├── 排行榜 → Redis Sorted Set
                      └── 普通 KV → Redis（lib/db/redis 多实例）
```

#### 6.3 路由策略标准（服务实例级）

| 路由规则 | 适用 | 实现 |
|---------|------|------|
| `Hash_UID` | 按玩家粘性（mainsvr/infosvr） | `svrs[uid % len]` |
| `Hash_RouterID` | 按 RouterID 粘性（mysqlsvr/roomcentersvr） | `svrs[routerID % len]` |
| `Hash_ZoneID` | 按 Zone（ranksvr） | `svrs[zoneID % len]` |
| `Random` | 无状态（web_svr/chat） | `rand.Int31n` |
| `Master` | 主备 | `svrs[0]` |
| `ConsistentHash_*` | 需一致性哈希（扩缩容少迁移） | 一致性哈希环 |

#### 6.4 Redis 多实例规划

Redis 实例经 `base_cfg.dependencies.db_instances`（`[]redis.Config`）配置，按 `instID` 区分。可按业务域独立实例：

| instID 域 | 用途 |
|-----------|------|
| 玩家数据 | mainsvr 玩家 hash 落盘 |
| 房间快照 | roomcentersvr |
| 缓存 | infosvr BriefInfo |
| 排行榜 | Redis Sorted Set |

---

### 第七步：cmd 号段分配

#### 7.1 号段规划标准

| 编号 | 标准内容 | 类型 |
|------|---------|------|
| **RT-01** | 每个独立微服务分配独立的 `ServerType`（cmd 高 16 位） | 必须 |
| **RT-02** | 同服务的不同功能用不同 cmd（低 16 位序号） | 必须 |
| **RT-03** | inner cmd（MsgType=1）与 client cmd（MsgType=0/3）区分 | 必须 |
| **RT-04** | `ServerType` 一旦分配不可回收 | 必须 |

#### 7.2 ServerType 号段分配表（`module/misc/constant.go`）

| ServerType | 值 | 服务 | cmd 号段前缀 |
|---|---|---|---|
| ConnSvr | 1 | connsvr | `0x11000+` |
| MainSvr | 2 | mainsvr | `0x20000+` |
| InfoSvr | 3 | infosvr | `0x30000+` / `0x31000+`(inner) |
| MysqlSvr | 4 | mysqlsvr | `0x41000+` |
| GmSvr | 5 | 预留 | `0x50000+` |
| MailSvr | 6 | 预留 | `0x60000+` / `0x61000+` |
| ChatSvr | 7 | 预留 | `0x70000+` |
| FriendSvr | 8 | 预留 | `0x80000+` / `0x81000+` |
| RankSvr | 9 | 预留 | `0x90000+` |
| GuildSvr | 10 | 预留 | `0xA0000+` |
| RoomCenterSvr | 11 | roomcentersvr | `0xB1000+` |
| WebSvr | 12 | web_svr | 多走 HTTP/gRPC |
| TexasGameSvr | 0x50 | 德州玩法 | `0x501000+` |
| RummyGameSvr | 0x51 | 预留 | `0x511000+` |

S2C 下行（ServerType=0）：`0x3000+`。

> 新增 cmd 在 `common/game_proto/core/cmd.proto` 的 `CMD` 枚举里加，命名 `CMD_<SVR>_<ACTION>_<REQ|RSP>`。详见 [02-通讯协议规范文档.md](../../spec/02-通讯协议规范文档.md)。

---

### 第八步：输出微服务设计四件套

每个微服务输出 **4 份标准化设计方案**，存放在 `{analysis_output_dir}/{需求名称}/`，遵循 `specs/` 下模板：

| 文档 | 文件命名 | 设计规范 |
|------|---------|---------|
| **整体设计方案** | `{服务名}-整体设计方案.md` | [整体设计规范](specs/微服务-整体设计规范.md) |
| **数据库设计方案** | `{服务名}-数据库设计方案.md` | [数据库设计规范](specs/微服务-数据库设计规范.md) |
| **交互设计方案** | `{服务名}-交互设计方案.md` | [交互设计规范](specs/微服务-交互设计规范.md) |
| **功能组件化方案** | `{服务名}-功能组件化方案.md` | [有状态](specs/微服务-有状态功能组件化规范.md) / [无状态](specs/微服务-无状态功能组件化规范.md) |

通过 obsidian-cli 创建：

```bash
obsidian vault="{vault_path}" create name="{analysis_output_dir}/{需求名称}/{服务名}-整体设计方案" content="{内容}"
# ...其余三份同理
```

#### 摘要记录

在 `{analysis_output_dir}/{需求名称}/微服务设计方案.md` 输出摘要：

```markdown
# {需求名称} — 微服务设计方案
> 方案版本：v1.0  创建日期：{date}
> 基于技术分析方案：[[{analysis_output_dir}/{需求名称}/技术分析方案]]
> 方案状态：草稿 / 评审中 / 已确认

## 一、架构决策摘要
| 服务名 | 类型 | ServerType | 路由规则 | 变更类型 | 设计文档 |
|--------|------|-----------|---------|---------|---------|
| {服务名} | 无状态/有状态 | {st} | {rule} | 新建/优化 | [[{服务名}-整体设计方案]] |

## 二、设计文档清单 / 三、关键设计决策 / 四、约束检查结果
```

---

### 第九步：方案评审与迭代

1. 输出方案后逐项审核设计约束检查清单
2. 团队评审
3. 禁止模式检查（F-01 ~ F-08）
4. 基于 DAU 目标核算容量
5. 每次重大修改方案版本号 +0.1

---

## 设计原则

1. **约束优先于自由**：规范中的"必须"项不可违反
2. **DAU 驱动设计**：容量规划以运营 DAU 目标为基准
3. **数据聚合原则**：同一用户的核心数据聚合在同一服务（如 mainsvr RoleMgr）
4. **故障可见原则**：每个设计决策明确写出故障时的行为与恢复策略（Drainer/Quiescer）
5. **不可逆决策审慎**：`ServerType` 分配、路由规则选择等不可逆决策须经评审
6. **简单优先**：能用无状态解决的不用有状态，能单库解决的不分实例
7. **先设计后开发**：方案确认前不编写服务代码
8. **分层约束**：`lib/`/`module/` 禁引用 globals；`cmd/` 只做入口

---

## 相关规范文档

本技能的 `specs/` 目录下包含完整的微服务设计规范体系：

### 架构模式规范

| 规范文档 | 说明 |
|---------|------|
| [specs/无状态微服务设计规范.md](specs/无状态微服务设计规范.md) | 无状态服务的架构约束、容量规划、数据一致性 |
| [specs/有状态微服务设计规范.md](specs/有状态微服务设计规范.md) | 有状态服务的事务模型、状态管理、优雅下线 |

### 设计方案规范（四件套）

| 规范文档 | 对应设计方案 |
|---------|------------|
| [specs/微服务-整体设计规范.md](specs/微服务-整体设计规范.md) | `{服务名}-整体设计方案.md` |
| [specs/微服务-数据库设计规范.md](specs/微服务-数据库设计规范.md) | `{服务名}-数据库设计方案.md` |
| [specs/微服务-交互设计规范.md](specs/微服务-交互设计规范.md) | `{服务名}-交互设计方案.md` |
| [specs/微服务-有状态功能组件化规范.md](specs/微服务-有状态功能组件化规范.md) | `{服务名}-功能组件化方案.md`（有状态） |
| [specs/微服务-无状态功能组件化规范.md](specs/微服务-无状态功能组件化规范.md) | `{服务名}-功能组件化方案.md`（无状态） |

这些规范是方案的**验收标准**：方案通过评审的前提是满足规范中的所有强制约束。
