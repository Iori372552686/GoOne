# tools/tester 测试用例开发规范

> **核心目标**：模拟真实客户端行为，通过业务数据和配置表驱动测试，发现服务器端的逻辑错误、数据不合理和 BUG。
> 每一条协议消息对应一组测试用例（≥10 个），其中**至少一半用于验证业务正确性**。

---

## 第1章 测试的核心理念

### 1.1 目标定位

tester 的本质是一个**模拟游戏客户端**。测试的目标不是验证协议格式，而是：

```
发现服务器 BUG = 发现消息数据不合理 + 发现业务逻辑错误 + 发现状态转换异常
```

因此每条消息的测试分为两类：

| 类别 | 占比 | 目标 | 示例 |
|------|------|------|------|
| **业务正确性测试** | ≥50% | 模拟真实玩家操作，验证业务结果 | 用配表中的真实道具 ID 添加道具，验证获得数量和属性 |
| **协议鲁棒性测试** | ≤50% | 边界值/异常输入，验证服务器不崩溃 | Count=0、负数、超大值 |

### 1.2 测试数据必须来自真实环境

测试数据**禁止凭空编造**，必须按以下优先级获取：

```
优先级1: 从配置表 JSON 读取真实 ID、属性名、枚举值
优先级2: 从 common/protocol 包获取状态枚举与 CMD 常量
优先级3: 从当前角色数据状态出发，构造符合业务流程的测试序列
```

错误做法：
```go
AddItem{ItemId: 999}  // ← 999 可能不存在于配表，服务器直接拒绝
```

正确做法：
```go
// 从配置表中读到有效道具 ID
AddItem{ItemId: loadValidItemID()} // ← 真实存在的道具
```

### 1.3 完全模拟客户端逻辑行为

测试必须模拟客户端在以下场景中的行为：

| 客户端行为 | 对应的测试设计 |
|-----------|--------------|
| 登录后首次进入 | 同步所有系统数据 → 展示 UI → 玩家操作 |
| 获得新道具 | 检查背包是否正确更新 → 若为货币类道具检查余额 |
| 完成任务领奖 | 验证奖励的每种道具/货币到账，任务状态变为已领奖 |
| 购买操作 | 扣除货币 → 确认余额正确 → 确认购买物进入背包 |
| 重复操作 | 连续点击同一按钮，服务器不应重复发放 |

### 1.4 发现 BUG 的测试设计思路

设计测试用例时，以"找到服务器逻辑漏洞"为驱动：

1. **数据合理性**：货币扣除后余额应为正数？道具使用后 Count 是否减到负数？
2. **状态一致性**：任务领奖后，道具增加了但任务状态未变？
3. **顺序依赖**：先 A 后 B 和先 B 后 A 结果是否一致？是否出现了未预期的状态？
4. **跨系统联动**：道具使用触发的属性变更是否正确推送？货币变化是否同步？
5. **配置表偏差**：服务器返回的数值是否与配表中定义的一致？

---

## 第2章 每条消息的测试用例设计框架

每一条协议消息的 ≥10 个测试用例，按以下比例分配：

```
┌─────────────────────────────────────────────────────┐
│  业务正确性测试 (≥50%)                                │
│  ├── 基本业务流程（用真实配表数据走通完整流程）          │
│  ├── 带当前角色状态的操作（基于已有道具/货币/属性）       │
│  ├── 跨系统联动验证（操作 A 查系统 B 的变化）            │
│  ├── 重复操作验证（连续执行，结果是否累积正确）           │
│  ├── 操作序列验证（增→删→增→查，最终数据一致）            │
│  └── 配置表数据验证（响应中的数值与配表定义一致）          │
├─────────────────────────────────────────────────────┤
│  协议鲁棒性测试 (≤50%)                                │
│  ├── 字段零值/负值/极大值                              │
│  ├── 不存在的 ID                                     │
│  └── 服务端推送消息是否到达                             │
└─────────────────────────────────────────────────────┘
```

---

## 第3章 从配表和当前状态设计业务测试数据

### 3.1 配表驱动测试

所有测试数据必须来自真实配置表（JSON 文件，参见 `.GoOne/spec/01-游戏配置规范文档.md`）：

```go
// 示例：读取有效道具 ID 列表来测试背包
func (c *XxxComponent) loadValidItemIDs() []int32 {
    // 读取配置表，返回真实存在的道具 ID
}

func (c *XxxComponent) test_AddItem(ctx context.Context) error {
    validIDs := c.loadValidItemIDs()
    for _, id := range validIDs {
        req := &pbGame.ReqAddItem{ItemId: id, Count: 1}
        resp := &pbGame.RspAddItem{}
        if err := c.requester.RequestProto(ctx, uint32(cmdAddItem), req, resp, 10*time.Second); err != nil {
            return fmt.Errorf("add real item %d failed: %w", id, err)
        }
        // 验证响应中的 Item.ItemId == id
        // 验证配表中该道具的类型与响应一致
    }
    return nil
}
```

### 3.2 基于当前角色状态设计测试序列

测试不是孤立发送消息，而是模拟玩家在游戏中逐渐积累数据的过程：

```go
func (c *XxxComponent) test_BusinessFlow_EarnAndSpend(ctx context.Context) error {
    // 1. 查询初始状态（新角色可能无道具、无货币）
    initialItems := len(c.items)
    initialGold := c.currencies[GoldCurrencyID]

    // 2. 模拟"获得道具" → 检查道具系统和货币系统
    //    道具中的金币类是否会转入货币系统？
    for _, id := range c.loadValidItemIDs() {
        c.sender.SendMessage(uint32(cmdAddItem), &pbGame.ReqAddItem{ItemId: id, Count: 1})
    }
    // 查询背包，确认所有道具都在
    // ... RequestProto 查询并校验

    // 3. 模拟"使用道具" → 是否触发效果？数量是否正确减少？
    // 4. 模拟"丢弃道具" → 是否真的移除？

    // 5. 最终验证：数据一致性
    //    道具数量变化 + 货币数量变化 = 预期变化
    return nil
}
```

### 3.3 业务正确性验证要点

每步操作后必须验证：

| 验证项 | 方法 | 发现什么BUG |
|--------|------|-----------|
| 数值符合配表 | 对比响应数据与配置表中的定义 | 配表加载错误、服务器计算错误 |
| 状态转换正确 | 对比操作前后的状态 | 状态机跳转逻辑错误 |
| 跨系统联动 | 操作道具系统后查货币系统 | 联动丢失、数据不同步 |
| 操作可逆性 | 先增后删，数据归零 | 计数残留、内存泄漏 |
| 操作幂等性 | 同一请求发2次，结果是否正确 | 重复扣费、重复发奖 |

---

## 第4章 业务测试的展开方法

### 4.1 基本业务流程测试（≥3 个用例）

用配表中真实数据走通完整业务链路。如果链路走不通，说明服务端有 BUG。

```go
// BUG 发现示例：道具消耗链路测试
func (c *XxxComponent) test_ItemConsumeChain(ctx context.Context) error {
    // 步骤1: 添加道具（ItemId 从配表取）
    // 步骤2: 查询背包，确认道具存在且 Count 正确
    // 步骤3: 使用道具，验证响应中 Item.Count 减少
    // 步骤4: 再次查询，验证服务端返回的 Count 与本地 cache 一致
    //
    // 如果步骤3的响应 Count 未变 → BUG: 使用道具未生效
    // 如果步骤4返回的 Count 与 cache 不一致 → BUG: 服务端状态与客户端不同步
    return nil
}
```

### 4.2 跨系统联动验证（≥2 个用例）

```go
// 任务领奖联动验证：操作任务系统，检查道具+货币系统
func (c *XxxComponent) test_TaskClaim_CrossSystem(ctx context.Context) error {
    beforeGold := c.currencies[GoldCurrencyID]
    beforeItems := len(c.items)

    // 领取任务
    resp := &pb.TaskClaimRsp{}
    c.requester.RequestProto(ctx, uint32(cmdTaskClaim), &pb.C2STaskClaim{TaskID: completedID}, resp, 10*time.Second)

    // 同步道具和货币
    // ... RequestProto 查询并更新本地缓存

    afterGold := c.currencies[GoldCurrencyID]
    afterItems := len(c.items)

    if afterGold == beforeGold && afterItems == beforeItems {
        return fmt.Errorf("BUG: task claimed but no reward received")
    }
    // 更进一步：验证奖励内容与配置表中定义的奖励一致
    return nil
}
```

### 4.3 操作序列与累积验证（≥2 个用例）

模拟真实玩家连续操作，服务端数据是否正确累积：

```go
func (c *XxxComponent) test_RepeatedOperations(ctx context.Context) error {
    // 连续添加同一道具 5 次
    for i := 0; i < 5; i++ {
        c.requester.RequestProto(ctx, uint32(cmdAddItem), &pbGame.ReqAddItem{ItemId: validID, Count: 1}, &pbGame.RspAddItem{}, 10*time.Second)
    }
    // BUG 检测：如果每次返回 Item.Count 都是 1 而非递增 → 堆叠逻辑错误

    // 连续扣除货币 3 次
    for i := 0; i < 3; i++ {
        c.requester.RequestProto(ctx, uint32(cmdDeduct), &pbGame.ReqDeduct{CurrencyID: 1, Amount: 10}, &pbGame.RspDeduct{}, 10*time.Second)
    }
    // BUG 检测：如果余额未正确递减 → 扣费逻辑错误
    return nil
}
```

### 4.4 状态依赖验证（≥2 个用例）

```go
// 基于当前角色实际的任务状态来测试 CLAIM
func (c *XxxComponent) test_TaskClaim_ByRealState(ctx context.Context) error {
    for taskID, task := range c.tasks {
        switch task.TaskState {
        case StateFinished:
            // 正常领取
        case StateReceived:
            // 重复领取 → 应返回 ErrorCode=ERR_RECEIVED
            // 如果返回 OK → BUG: 可重复领取任务奖励
        case StateActive:
            // 未完成领取 → 应拒绝
            // 如果返回 OK → BUG: 可提前领取奖励
        }
    }
    return nil
}
```

---

## 第5章 覆盖矩阵：每条消息的测试用例要求

以下列出 tester 当前已实现的注册组件及其覆盖的 C2S/S2C 消息。GoOne 的协议以 `CMD_MAIN_*` 形式集中定义在 `common/protocol/` 中，CMD 是 uint32（主子命令组合）。每条消息都要求 ≥10 个测试用例。

> **重要说明**：
> - **gm 组件是未实现的占位**。GoOne GM 协议是独立 CMD（如 `CMD_MAIN_GM_ADD_ITEM_REQ`），各字段语义不同，无法统一封装。`tools/tester/app/component/gm.go` 的 `SendGM` 调用会返回 `not implemented`。需要 GM 数据时请在业务组件内直接构造对应 GM proto 请求发送。
> - **rummy 不是注册组件**，而是纯单元测试 `tools/tester/app/component/rummy/rummy_test.go`（用 `go test` 运行），用于验证牌局算法本身，不参与 tester/stress 的客户端流程。

### 5.1 login 组件（在线/心跳）

参考实现：`tools/tester/app/component/login/login.go`

| CMD 常量 | 消息名 | Direction | 测试用例要求 |
|----------|--------|-----------|-------------|
| `CMD_MAIN_LOGIN_REQ`(0x20000) | LoginReq / LoginRsp | C2S | 已有账号登录、token 登录、异常账号、连续登录 ≥10 |
| `CMD_MAIN_HEARTBEAT_REQ`(0x20004) | HeartBeatReq / HeartBeatRsp | C2S | 心跳间隔、连续心跳、跨天心跳 ≥10 |

**LoginReq 测试用例清单（≥10，≥6 业务）**：

| # | 类别 | 测试数据 | 验证点 |
|---|------|---------|--------|
| 1 | 业务 | 已有账号再次登录 | Ret.Code=OK，RoleInfo 完整 |
| 2 | 业务 | 带 token 登录 | token 校验通过 |
| 3 | 业务 | 连续登录 3 次 | 每次都返回稳定结果 |
| 4 | 业务 | 登录后查询角色数据 | 各系统初始数据正确 |
| 5 | 业务 | 不同 ChannelId 登录 | 渠道标识正确记录 |
| 6 | 业务 | 登录响应 RoleInfo.RegisterInfo.Uid | 与请求 uid 一致 |
| 7 | 协议 | Account 为空 | 返回错误码 |
| 8 | 协议 | Account 超长 | 不崩溃 |
| 9 | 协议 | LoginType 非法值 | 返回错误码 |
| 10 | 协议 | ChannelId=0 | 处理或拒绝 |

### 5.2 room 组件（房间列表/快速开始/离开）

参考实现：`tools/tester/app/component/room/room.go`

| CMD 常量 | 消息名 | Direction | 测试用例要求 |
|----------|--------|-----------|-------------|
| `CMD_MAIN_GAME_ROOM_LIST_REQ`(0x20200) | RoomListReq / RoomListRsp | C2S | 空列表/有房间/分页 ≥10 |
| `CMD_MAIN_GAME_QUICK_START_REQ`(0x20244) | QuickStartReq / QuickStartRsp | C2S | 正常匹配/无可用房间/重复匹配 ≥10 |
| `CMD_MAIN_GAME_LEAVE_GAME_REQ`(0x2020C) | LeaveGameReq / LeaveGameRsp | C2S | 房间内离开/未进房离开 ≥10 |

**QuickStartReq 测试用例清单（≥10，≥6 业务）**：

| # | 类别 | 测试数据 | 验证点 |
|---|------|---------|--------|
| 1 | 业务 | 正常快速开始 | 成功进入房间 |
| 2 | 业务 | 连续快速开始 2 次 | 第二次合理处理（已在对局/排队） |
| 3 | 业务 | 快速开始后查询房间列表 | 自己所在房间可见 |
| 4 | 业务 | 不同玩法类型快速开始 | 进入对应玩法房间 |
| 5 | 业务 | 快速开始→离开→再快速开始 | 链路完整 |
| 6 | 业务 | 满房后快速开始 | 排队或新开房 |
| 7 | 协议 | 玩法 ID 不存在 | 返回错误码 |
| 8 | 协议 | 玩法 ID=0 | 处理或拒绝 |
| 9 | 协议 | 非法玩法 ID（负数） | 不崩溃 |
| 10 | 协议 | 重复请求触发 | 无副作用累积 |

> 注：以上 CMD 数值与覆盖维度是 GoOne 当前真实协议。新增玩法协议时，请到 `common/protocol/` 核对最新 CMD 常量后再补充用例。

---

## 第6章 跨系统数据一致性验证

### 6.1 联动链路

系统中操作一个模块会影响其他模块。测试时必须同时验证所有受影响系统的数据：

```
道具添加 ──→ 道具系统：背包 ItemInfo 变化
         └─→ 货币系统：若为货币道具，余额变化

任务领奖 ──→ 任务系统：TaskState → RECEIVED
         ├─→ 道具系统：奖励道具入库
         └─→ 货币系统：奖励货币到账
```

### 6.2 一致性验证代码模式

```go
func (c *XxxComponent) verifyCrossSystem(
    desc string,
    beforeItems int, afterItems int,
    beforeCurrencies map[int32]int64, afterCurrencies map[int32]int64,
) error {
    log.Printf("[Actor %d] Cross-check [%s]: items %d→%d, currencies=%v→%v",
        c.actorID, desc, beforeItems, afterItems, beforeCurrencies, afterCurrencies)
    // 根据操作类型预期变化方向
    return nil
}
```

---

## 第7章 测试代码文件组织规范

### 7.1 文件拆分标准

| 文件行数 | 措施 |
|---------|------|
| ≤200 行 | 单文件 OK |
| 200~400 行 | 可将测试方法拆分到独立 test_xxx.go |
| ≥400 行 | 必须拆分 |

### 7.2 拆分示例

```
tools/tester/app/component/xxx/
├── xxx.go          # <200行：接口实现、OnMessage、RequestProto 封装
├── test_room.go    # >200行：房间相关全部测试
├── test_item.go    # ~150行：道具相关全部测试
└── test_task.go    # ~200行：任务相关全部测试
```

### 7.3 test_xxx.go 文件模板

```go
package xxx

import (
    "context"
    "fmt"
    "log"
    "time"

    pb "github.com/Iori372552686/g1_common/protocol"
)

// testAll 触发该组件的全部消息测试
func (c *XxxComponent) testAll(ctx context.Context) error {
    if err := c.test_RoomList(ctx); err != nil { return err }
    if err := c.test_QuickStart(ctx); err != nil { return err }
    if err := c.test_LeaveGame(ctx); err != nil { return err }
    return nil
}

// test_RoomList 测试 CMD_MAIN_GAME_ROOM_LIST_REQ
// 覆盖：空列表查询、有房间查询、分页、连续查询
func (c *XxxComponent) test_RoomList(ctx context.Context) error {
    testCases := []struct {
        name   string
        preOp  func()
        verify func(*pb.RoomListRsp) error
    }{
        {"空列表查询", nil, func(resp *pb.RoomListRsp) error {
            if resp == nil { return fmt.Errorf("nil resp") }
            return nil
        }},
        // ... 继续 ≥10 个用例
    }
    for _, tc := range testCases {
        log.Printf("[Actor %d][Xxx] Test: %s", c.actorID, tc.name)
        if tc.preOp != nil { tc.preOp() }
        resp := &pb.RoomListRsp{}
        if err := c.requester.RequestProto(ctx, uint32(pb.CMD_MAIN_GAME_ROOM_LIST_REQ), &pb.RoomListReq{}, resp, 10*time.Second); err != nil {
            return fmt.Errorf("%s: %w", tc.name, err)
        }
        if err := tc.verify(resp); err != nil { return fmt.Errorf("%s: %w", tc.name, err) }
    }
    return nil
}
```

---

## 第8章 组件开发通用模板

### 8.1 新建组件的步骤清单

- [ ] 1. 在 `common/protocol/` 中确定该组件需要覆盖的 CMD 常量列表
- [ ] 2. 阅读服务端业务代码（`src/<svc>svr/`），理解每条消息的 Request 字段含义、响应字段、状态依赖性
- [ ] 3. 创建 `tools/tester/app/component/<name>/<name>.go`，实现 `TesterComponent` 接口（必要时再实现 `StressRunner`）
- [ ] 4. 在 `init()` 中调用 `component.Register("name", factory)`
- [ ] 5. 在 tester 入口（`tools/tester/cmd/tester/main.go`）通过 blank import 注册组件包
- [ ] 6. 为每条 CMD 编写 ≥10 个测试用例（如文件过大则拆分 test_xxx.go）
- [ ] 7. 必要时在配置文件中扩展 `[modules.<name>]` 私有参数（用 `Cfg.DecodeModule(name, &v)` 解码）
- [ ] 8. 在 `tools/tester/tester.toml` / `stress.toml` 的 `[modules.<name>]` 中启用组件

### 8.2 组件接口与结构模板

> 接口定义见 `tools/tester/app/component/component.go`。`TesterComponent` 必须实现：`Name / OnInit / OnConnected / OnAccountLogin / OnRoleLogin / RunTests / OnMessage`。压测场景额外实现 `StressRunner.RunStress`。

```go
package xxx

import (
    "context"
    "fmt"
    "log"
    "sync"
    "time"

    "github.com/Iori372552686/GoOne/tools/tester/app/component"
    "github.com/Iori372552686/GoOne/tools/tester/internal/testcfg"
    "github.com/Iori372552686/GoOne/tools/tester/internal/session"
    g1_protocol "github.com/Iori372552686/g1_common/protocol"
)

func init() {
    component.Register("xxx", func() component.TesterComponent {
        return &XxxComponent{}
    })
}

type XxxComponent struct {
    actorID   int
    accountID string
    userID    int64
    sender    component.MessageSender
    requester component.Requester
    cfg       *testcfg.Config

    mu    sync.Mutex
    items map[int32]int64 // 本地数据缓存（按需）
}

func (c *XxxComponent) Name() string { return "xxx" }

func (c *XxxComponent) OnInit(ctx *component.ComponentContext) error {
    c.actorID = ctx.ActorID
    c.accountID = ctx.AccountID
    c.userID = ctx.UserID
    c.sender = ctx.Sender
    c.requester = ctx.Requester
    c.cfg = ctx.Cfg
    c.items = make(map[int32]int64)
    return nil
}

func (c *XxxComponent) OnConnected() error                     { return nil }
func (c *XxxComponent) OnAccountLogin(accountID string) error  { c.accountID = accountID; return nil }
func (c *XxxComponent) OnRoleLogin(userID int64) error         { c.userID = userID; return nil }

func (c *XxxComponent) OnMessage(cmd uint32, data []byte) bool {
    // 主动推送（S2C）在这里按 cmd 分发；返回 true 表示已处理
    return false
}

func (c *XxxComponent) RunTests(ctx context.Context) error {
    // 调用各 test_xxx.go 中的 testAll 方法
    return c.testAll(ctx)
}

// RunStress 可选：压测正常路径（实现 StressRunner 接口）
func (c *XxxComponent) RunStress(ctx context.Context) error {
    // 一轮轻量、正常路径的业务操作
    return nil
}

// requestAndWait 同步请求-响应的内部封装示例
func (c *XxxComponent) requestAndWait(ctx context.Context, cmd uint32, req, rsp g1_protocol.Message, timeout time.Duration) error {
    if err := c.requester.RequestProto(ctx, cmd, req, rsp, timeout); err != nil {
        return err
    }
    // 业务错误码统一校验
    return nil
}

var _ component.TesterComponent = (*XxxComponent)(nil)
```

---

## 第9章 测试执行与判定标准

### 9.1 判定标准

```
PASS = 所有 test_CMD_* 方法无 error 返回
     + 所有跨系统一致性验证通过
     + 所有 RequestProto 在超时前返回
     + Summary 数据与预期一致

FAIL = 任一 assert 失败 / RequestProto 超时 / 数据不一致
```

### 9.2 运行命令

`tools/tester` 提供两个入口（构建产物由 `./build.sh tester|stress` 生成至 `build/`），均通过 `go run` 启动：

**回归测试（默认 all-in-one，推荐）**

```bash
# 构建（可选）
./build.sh tester

# 推荐：直接 go run
go run ./tools/tester/cmd/tester -config ./tools/tester/tester.toml
```

- 配置文件：`tools/tester/tester.toml`（`[run].mode` 须为 `regression`）
- 模块开关：`tester.toml` 中 `[modules.<name>].enabled = true`
- 外部依赖：被测服务器 + etcd/Redis/MySQL/RabbitMQ
- 启动后自动跑用例，退出码 0/1 表示结果

**压测（多玩家饱和测试）**

```bash
# 先启动被测服务器（connsvr/mainsvr/roomcentersvr 等）

# 构建并启动压测客户端
./build.sh stress
go run ./tools/tester/cmd/stress -config ./tools/tester/stress.toml
```

- 配置文件：`tools/tester/stress.toml`

### 9.3 单消息/单模块调试模式

开发阶段可在 `tools/tester/tester.toml` 中只启用目标模块：

```toml
[run]
mode = "regression"

[server]
host = "127.0.0.1"
tcp_port = 11001
transport = "tcp"

[player]
players = 1
start_uid = 100001

[modules.login]
enabled = true
order = 1

# 其余模块 enabled = false
```

---

## 第10章 速查表

### 10.1 已实现组件-测试用例对应速查

| 组件 | 主要 CMD | 用例数要求 | 说明 |
|------|---------|----------|------|
| login | `CMD_MAIN_LOGIN_REQ`、`CMD_MAIN_HEARTBEAT_REQ` | ≥20 | 登录 + 心跳 |
| room | `CMD_MAIN_GAME_ROOM_LIST_REQ`、`CMD_MAIN_GAME_QUICK_START_REQ`、`CMD_MAIN_GAME_LEAVE_GAME_REQ` | ≥30 | 房间全链路 |
| rummy | （算法单测，非注册组件） | — | `go test ./tools/tester/app/component/rummy/` |
| gm | （未实现占位） | — | GoOne GM 走独立 CMD，请在业务组件直接构造 proto 请求 |

### 10.2 测试用例维度速查

编写任意消息的测试用例时，遍历以下清单：

```
□ 字段-正常值（1-2个）
□ 字段-零值
□ 字段-负值
□ 字段-极大值
□ 字段-不存在ID
□ 字段-ID=0
□ 不同枚举值组合
□ 重复调用（2次）
□ 连续调用（3次以上）
□ 操作后查询验证
□ 不同前置状态
□ 跨系统一致性
□ 推送是否正确收到
□ 超时场景
□ 并发操作（多玩家）
```

### 10.3 关键常量与代码来源文件

| 内容 | 文件 |
|------|------|
| CMD 常量与消息定义 | `common/protocol/`（CMD_MAIN_* / Req/Rsp） |
| 组件接口 | `tools/tester/app/component/component.go`（TesterComponent / StressRunner） |
| 组件注册器 | `tools/tester/app/component/registry.go`（`component.Register`） |
| 回归测试配置 | `tools/tester/tester.toml` |
| 压测配置 | `tools/tester/stress.toml` |
| 服务端业务逻辑 | `src/<svc>svr/`（mainsvr/connsvr/roomcentersvr/infosvr/mysqlsvr） |
| CSPacketHeader（线协议） | `lib/api/sharedstruct/cs_packet.go` |

### 10.4 CSPacketHeader 线协议（TCP 二进制）

tester 通过 TCP 与网关通信，线协议为 **28 字节大端 CSPacketHeader + protobuf body**：

| 字段 | 类型 | 字节 |
|------|------|------|
| Version | uint16 | 2 |
| PassCode | uint16 | 2 |
| Seq | uint32 | 4 |
| Uid | uint64 | 8 |
| AppVersion | uint32 | 4 |
| Cmd | uint32 | 4 |
| BodyLen | uint32 | 4 |
| **合计** | | **28** |

约束：`MaxByteLenOfCSPacketBody = 4 * 1024 * 1024`（4MB）。组件开发者通常无需直接拼包——通过 `Requester.RequestProto` / `MessageSender.SendMessage` 传入 CMD（uint32）+ proto 消息即可，框架自动处理线协议。仅在排查底层收发问题时参考此结构。
