---
name: "G1-服务器-模拟测试"
description: "基于策划PRD、需求分析报告和技术设计文档，通过tools/tester模拟客户端框架进行联调级业务BUG探测。当业务代码开发完成后需要进行模拟联调测验、发现服务器逻辑漏洞、修复BUG并循环回归时调用。"
---

# G1-服务器-模拟测试

## 目标

模拟真实客户端行为，通过 **`tools/tester` 模拟客户端框架**对服务器进行**联调级业务 BUG 探测**。核心目标：发现服务器业务逻辑漏洞、数据不合理和 BUG → 修复 → 回归，直到问题闭环。

> **定位边界**：
> - ✅ 基于 PRD + 需求分析报告 + 技术分析方案提取测试业务范围
> - ✅ 设计模拟测试用例方案（每协议 ≥20 个用例）
> - ✅ 使用 `tools/tester` 框架开发测试组件代码
> - ✅ 构建并运行 tester / stress → 启动服务 → 运行联调级模拟测试
> - ✅ 循环测试：发现 BUG → 溯源服务器代码 → 修复 → 回归
> - ✅ 输出 BUG 问题清单及解决方案报告
> - ❌ 不涉及需求审查（"需求分析"技能职责）
> - ❌ 不涉及技术方案设计（"技术分析设计"技能职责）
> - ❌ 不涉及业务编码开发（"业务开发"技能职责）
> - ❌ 不修改 lib/ 框架层代码

---

## 环境假设（无需排查）

以下基础设施默认已就绪，**遇到连接失败或超时不要排查 etcd/zk/Redis/MySQL/RabbitMQ 等中间件**，直接归因为业务逻辑 BUG 或配置错误：

| 组件 | 状态 | 说明 |
|------|------|------|
| **etcd / zk** | ✅ 已启动（注册中心） | 服务发现/注册中心 |
| **Redis** | ✅ 已启动 `127.0.0.1:6379` | 缓存/状态存储（`lib/db/redis`） |
| **MySQL** | ✅ 已启动（`lib/db/xorm`） | 持久化存储 |
| **RabbitMQ** | ✅ 已启动（`lib/service/bus`） | 消息总线 |

> GoOne 不使用 MongoDB / NATS。如果遇到相关引用，属于历史残留，按上面四类中间件对号入座。

如果启动服务器时出现连接 refused / timeout 等错误，**直接检查服务器配置（etc/*.toml）的地址端口是否正确**，而非排查中间件是否在运行。测试过程中关注业务逻辑本身。

---

## 规范遵循（强制）

> 在进行模拟测试时，应先去 `.GoOne/spec/` 目录下自行读取相关规范文档，确保测试用例的协议字段、配置表数据、DB 结构遵循项目规范。测试数据须从真实配置表和协议映射中提取。技能不固化具体规范文件名，以目录下实际文档为准。

## 约束规则

你必须严格遵守以下规则：

### 规则1：测试数据必须来自真实环境

**禁止凭空编造测试数据。** 数据来源优先级：

1. 从配置表读取真实 ID、属性名、枚举值（参见 `.GoOne/spec/01-游戏配置规范文档.md`）
2. 从 `common/protocol` 获取状态/类型枚举与 CMD 常量
3. 从当前角色数据状态出发构造测试序列

当配置表数据不足时，使用预留 ID 段填充 `[TEST]` 标记的模拟数据，不破坏已有数据。

### 规则2：主动准备测试数据—GM 注入 + 配置表填充

测试执行前，必须确保角色和相关系统拥有足够的依赖数据，**不允许因数据不足导致测试逻辑空走或跳过**。

**方式一：通过 GM 命令注入运行时数据**

> ⚠️ **GoOne GM 协议限制（重要）**：GoOne 的 GM 不是统一的 `CMD_PLAYER_GM`，而是**一组独立的 CMD**（如 `CMD_MAIN_GM_ADD_ITEM_REQ` 等），各字段语义不同，无法像旧框架那样用统一的 `Cmd/Params` 封装。`tools/tester/app/component/gm.go` 中的 `SendGM` 是**未实现的占位**，调用会返回 `not implemented` 错误。
>
> 因此在 GoOne 中注入 GM 数据的正确做法是：**在业务组件内直接构造对应 GM 协议的 proto 请求并发送**（例如构造 `CMD_MAIN_GM_ADD_ITEM_REQ` 的请求体），而非调用通用 `SendGM`。

- 连续 GM 命令之间应加适当 `time.Sleep` 等待服务端处理完毕（GM 是异步的）
- 典型流程：GM 注入 → 查询同步 → 缓存到本地 → 开始业务测试

**方式二：填充运行时配置表**（补充业务数据）

当某个模块的配置表数据为空或不足以支撑业务测试时，直接在运行时目录下填充测试数据：

- 配置表位置：项目运行时配置目录下的 JSON 文件
- 填充原则：不破坏已有数据（使用新 ID）、引用完整性（外键指向的记录必须存在）、字段完备
- 测试数据名称添加 `[TEST]` 标记，便于区分
- 填充后重新构建（`./build.sh` 会自动处理依赖）

**两种方式的选择策略**：

| 场景 | 推荐方式 | 原因 |
|------|---------|------|
| 角色维度的数据（背包、货币、属性） | GM 注入 | 灵活、可重复、不影响其他角色 |
| 系统维度的数据（道具定义、任务定义、属性定义） | 配置表填充 | 服务器启动时加载，GM 无法动态修改 |
| 需要大量数据的压力测试 | 配置表填充 + GM 注入 | 配置表提供定义，GM 提供实例 |

### 规则3：先读规范再动手

在编写任何代码前，必须先完整阅读开发规范：

**[tester-开发规范.md](./tester-开发规范.md)**

该文档涵盖：核心理念、测试用例设计框架、组件结构、文件组织、GM 命令使用、配置表填充等全部细节。AI 应根据规范自主生成代码，技能不再提供代码模板。

已有测试组件位于 `tools/tester/app/component/<name>/`，参考其实现风格（当前内置：`login`、`room` 注册组件；`rummy` 是纯单测 `rummy_test.go`）。

### 规则4：每条协议 ≥20 个用例，以发现 BUG 为驱动

| 类别 | 占比 | 覆盖维度 |
|------|------|---------|
| **业务正确性测试** | ≥60%（≥12 个） | 基本流程、状态依赖、跨系统联动、重复操作、操作序列、配置校验、幂等性、并发 |
| **协议鲁棒性测试** | ≤40%（≤8 个） | 零值/负值/极大值、不存在 ID、推送验证 |

每个用例的设计出发点：**"这个数据/序列可能触发服务器的什么逻辑漏洞？"**

### 规则5：模拟真实客户端行为

测试不是孤立发消息，而是模拟玩家的完整生命周期：登录 → 同步数据 → 操作 → 验证 → 跨系统检查。

- 使用 `Requester.RequestProto`（或 `Request`）同步等待模式（发送请求 → 等待响应/超时）
- 组件维护本地缓存（道具、货币、属性），与服务器数据保持一致
- 每步操作后验证：数值正确、状态转换、跨系统联动
- 超时的请求即 BUG，必须溯源

### 规则6：先建立预期再操作，预期不符即 BUG

每个测试用例在发送请求前，必须基于当前本地缓存数据和业务逻辑，**预先推导出操作后所有受影响系统的预期状态**，再发送请求验证。

```
步骤1【建立预期】: 基于当前缓存 + 业务逻辑，推导所有受影响系统的预期状态
    ├─ 直接响应预期：返回的字段值、错误码
    ├─ 联动模块预期：操作道具后货币是否变化？操作任务后背包是否新增？
    ├─ 状态变化预期：Count 增减量、状态枚举转换、属性差分更新
    └─ 推送预期：此次操作应触发哪些 S2C Push？推送内容应包含什么？

步骤2【执行操作】: 发送请求并等待响应

步骤3【逐项比对】: 将实际响应和联动模块数据与预期逐项对比
    ├─ 直接响应字段 == 预期值？
    ├─ 联动模块(背包/货币/属性/任务)数据变化 == 预期变化？
    └─ 本地缓存同步后与服务器数据一致？

步骤4【预期不符即 BUG】: 任何预期与实际不一致，必须追溯根因
    ├─ 测试代码预期写错了？→ 修正预期，继续验证
    ├─ 服务器行为与业务逻辑不一致？→ 服务器有 BUG，按规则 7 修复+回归
    └─ 文档/配表定义模糊？→ 记录并确认预期行为
```

**预期必须量化**，不能是模糊描述。例如：

```go
// 错误：模糊预期
resp := &pb.SomeRsp{}
c.requester.RequestProto(ctx, uint32(cmd), req, resp, timeout)
// 没有验证任何字段

// 正确：量化预期
beforeGold := c.currencies[GoldCurrencyID]
resp := &pb.SomeRsp{}
c.requester.RequestProto(ctx, uint32(cmd), req, resp, timeout)
// 预期1：响应中的 CurrencyNum == beforeGold + 100
// 预期2：背包道具列表新增了兑换消耗的道具
// 预期3：如果 currencyID 是体力类型，结果不超过上限
```

这条规则是 BUG 探测的核心手段——**服务器行为与客户端预期之间的每一次偏差，都是 BUG 或理解错误的信号**。

### 规则7：发现 BUG 后必须修复 + 回归 + 新增用例

```
发现 BUG → 溯源服务器代码（按协议注册 → handler → 根因）
         → 修复服务器代码
         → 在测试用例中增加该边界场景（防止回归）
         → 重新运行全部测试
         → 循环直到：所有用例通过 + 无新增 BUG + 致命/严重 BUG 已修复
```

**覆盖深度要求**：整轮模拟测试至少完成 5 次单人次迭代循环 + 2 轮 100 人饱和测试。每次迭代不是简单重复，而是：
- 基于上一轮发现的 BUG 拓展测试边界
- 针对每个根因新增 1~3 个覆盖该维度的用例
- 全面回归，确保修复不引入新问题
- 单人闭环后转入饱和并发测试，排查数据串扰、竞态、资源泄漏等多人问题

### 规则8：必须输出报告

通过 obsidian-cli 输出到知识库 `{analysis_output_dir}/{需求名称}/模拟测试报告-{YYYY-MM-DD}.md`：
```bash
obsidian vault_path="{vault_path}" create name="{analysis_output_dir}/{需求名称}/模拟测试报告-{YYYY-MM-DD}" content="{报告内容}"
```
包含：
- BUG 清单（严重程度、根因、解决方案、状态）
- 服务器代码修复清单
- 测试覆盖汇总
- 迭代轮次记录

---

## 工作流程

### 第一步：准备

调用 `G1-项目配置` 技能获取项目配置信息：

> - `vault_path` = Obsidian Vault 路径（用于 obsidian-cli 的 `vault_path=` 参数）
> - `prd_dir` = 策划需求文档目录（相对 vault 根目录）
> - `analysis_output_dir` = 技术输出目录（相对 vault 根目录）
>
> **输出目录**固定为 `{analysis_output_dir}/{需求名称}/`。

如果 `.GoOne/conf.json` 不存在，`G1-项目配置` 技能会自动引导创建。

- 通过 obsidian-cli 读取 PRD、需求分析报告、技术分析方案、开发规范：
  ```bash
  obsidian vault_path="{vault_path}" read path="{prd_dir}/{文档名称}"
  obsidian vault_path="{vault_path}" read path="{analysis_output_dir}/{需求名称}/需求分析报告.md"
  obsidian vault_path="{vault_path}" read path="{analysis_output_dir}/{需求名称}/技术分析方案.md"
  obsidian vault_path="{vault_path}" read path="{analysis_output_dir}/{需求名称}/需求开发报告.md"
  ```
- 提取模拟测试业务范围清单（涉及服务、通讯协议、BUG 探测重点）

### 第二步：设计测试用例方案

- 通过 obsidian-cli 输出到知识库：
  ```bash
  obsidian vault_path="{vault_path}" create name="{analysis_output_dir}/{需求名称}/模拟测试用例" content="{用例方案内容}"
  ```
- 每协议 ≥20 个用例，按 Rule 4 分配
- 每个用例必须明确标注**预期状态**（直接响应 + 联动模块 + 推送），按 Rule 6 格式
- 明确测试数据来源（哪个配置表、哪个 ID 段）

### 第三步：开发测试组件代码

- 参考 **[tester-开发规范.md](./tester-开发规范.md)** 实现，AI 自主生成代码
- 参考已有组件实现风格：`tools/tester/app/component/<name>/`
- 新建组件的注册步骤详见 tester-开发规范 第8章
- **每个测试函数必须遵循 Rule 6 的 4 步模式**：建立预期 → 执行操作 → 逐项比对 → 预期不符即 BUG

### 第四步：构建 & 运行

`tools/tester` 提供两个入口（均通过 `go run` 启动，二进制由 `./build.sh tester|stress` 产出至 `build/`）：

- **tester**：all-in-one **回归/单元测试**（默认模式 `regression`），单进程拉起模拟客户端连接外部服务器，跑完整用例集，输出 PASS/FAIL。
- **stress**：**压测客户端**，连接已启动的游戏服务器，多玩家循环执行业务操作，生成 Markdown 压测报告。

**回归测试（默认）**：

```bash
# 构建（可选，go run 会自动编译）
./build.sh tester

# 推荐：直接 go run
go run ./tools/tester/cmd/tester -config ./tools/tester/tester.toml
```

- 配置文件：`tools/tester/tester.toml`（`[run].mode` 须为 `regression`）
- 模块开关：在 `tester.toml` 的 `[modules.<name>]` 中设置 `enabled = true`
- 外部依赖：被测服务器（connsvr/mainsvr 等）须已启动，且 etcd/Redis/MySQL/RabbitMQ 可用
- 启动后自动发起测试，完成后以退出码 0/1 表示结果

**压测（多玩家饱和）**：

```bash
# 先启动被测服务器（示例：分别拉起各 svr，或用部署脚本）

# 再启动压测客户端
./build.sh stress
go run ./tools/tester/cmd/stress -config ./tools/tester/stress.toml
```

- 配置文件：`tools/tester/stress.toml`（正式压测）
- 玩家数量：`[player].players`

### 第五步：循环测试（核心）

**至少循环迭代 5 次**，每次迭代必须完整执行一轮"测试→归因→修复→扩用例→回归"闭环：

```
第 N 轮 (N=1..5+)
  ├─ 1. 运行全部回归测试用例（go run ./tools/tester/cmd/tester -config ...）
  ├─ 2. 归因分析：对所有 FAIL 和不符预期的结果，逐一排查
  │     ├─ 协议字段错误？→ 检查 handler 响应构造逻辑
  │     ├─ 状态转换异常？→ 检查状态机跳转条件
  │     ├─ 联动模块未更新？→ 检查跨系统调用链路
  │     ├─ 配置表读取偏差？→ 对比服务器加载值与配表定义
  │     ├─ 并发竞态？→ 检查 TransactionMgr 的 key 分片与锁
  │     └─ 消息遗漏？→ 检查 S2C Push 发送路径
  ├─ 3. 针对每个根因，迭代 1~3 个新测试用例覆盖该边界
  ├─ 4. 修复服务器代码
  └─ 5. 回归验证：全部用例通过 → 进入下一轮

**终止条件**（满足其一即可退出单人循环）：
  - 连续 2 轮无新增 BUG
  - 所有致命/严重 BUG 已修复并回归通过
  - 已完成 ≥5 轮迭代
```

#### 单人循环结束后 → 饱和玩家并发测试（2 轮）

单人循环闭环后，必须继续执行 **2 轮 100 人饱和玩家测试**，排查多人并发带来的问题。

```
饱和测试第 1 轮 (100 players)
  ├─ 配置：修改 stress.toml
  │        [player].players = 100
  │        [stress].flow = "random"      # random=按权重随机 | sequential=按 order 轮转
  │        [stress].loop = true
  │        [stress].duration = "10m"
  │        [stress].ramp_up_per_sec = 20
  ├─ 启动：go run ./tools/tester/cmd/stress -config ./tools/tester/stress.toml
  ├─ 监控重点：
  │     ├─ 数据串扰：玩家 A 的操作是否影响了玩家 B 的数据？
  │     ├─ 并发竞态：多玩家同时操作同一资源（如全服排行榜）是否出现脏写？
  │     ├─ 资源泄漏：100 连接同时运行，goroutine / 内存是否持续增长？
  │     ├─ 超时激增：相比单人模式，请求超时率是否显著上升？
  │     └─ 服务器 panic：是否有未捕获的并发安全 panic？
  ├─ 发现问题 → 溯源修复 → 回归单人全部用例 + 再跑 1 轮饱和
  └─ 通过标准：所有玩家全部 PASS，无非预期超时、无 panic、无数据串扰

饱和测试第 2 轮 (100 players)
  ├─ 数据清理后重新开始，确认第 1 轮的修复有效
  └─ 通过标准同上
```

**饱和测试发现的并发 BUG 必须同样记录到最终报告**，按 BUG 分类标准标注严重程度。

### 第六步：输出报告

输出 BUG 问题清单报告，按 Rule 8 格式。

---

## 参考指引

| 内容 | 位置 |
|------|------|
| **开发规范（必读）** | [tester-开发规范.md](./tester-开发规范.md) |
| **已有测试组件** | `tools/tester/app/component/<name>/`（login、room） |
| **rummy 纯单测** | `tools/tester/app/component/rummy/rummy_test.go`（非注册组件） |
| **gm 占位（未实现）** | `tools/tester/app/component/gm.go`（GoOne GM 走独立 CMD，请直接构造 GM proto 请求） |
| **协议常量与消息** | `common/protocol/`（CMD_MAIN_*、Req/Rsp 定义） |
| **组件接口定义** | `tools/tester/app/component/component.go`（TesterComponent / StressRunner） |
| **组件注册器** | `tools/tester/app/component/registry.go`（`component.Register`） |
| **回归测试配置** | `tools/tester/tester.toml` |
| **压测配置** | `tools/tester/stress.toml` |
| **服务端源码** | `src/<svc>svr/`（connsvr/mainsvr/roomcentersvr/infosvr/mysqlsvr） |
| **测试框架基础路径** | `tools/tester/` |
