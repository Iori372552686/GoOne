# 旧德州项目代码移除记录

日期：2026-09-27。目标：GoOne 仓库与本地位点 `g1_common`（`common/` 子模块）中，
清除种子项目（PokerGo / big_server 德州扑克）遗留的玩法业务代码与协议定义，
保留与具体玩法无关的框架层，为后续新游戏（Rummy 方向）落位留出干净基座。

依据：本仓库从未包含德州游戏服实现（`c2s_room.go` 注释原话"当前 checkout 不含游戏服实现"），
所谓德州代码实为：以德州命名的房间目录管线、对局转发 RPC、三张 `mysql_texas_*` 持久化表、
texas 配置表与一批玩法协议枚举/消息。

## 删除范围

### 服务业务（GoOne 主仓）

| 位置 | 内容 |
| --- | --- |
| `src/roomcentersvr/room_mgr/`（含 `texas_room/`） | 分区房间目录、场次快照、并行排序、AI 补房 |
| `src/roomcentersvr/room_ai/`、`service/` | AI 建房工厂、RoomCenterInnerService 处理器 |
| `src/roomcentersvr/globals/{idgen,rds}` | 房间 ID 生成、房间快照 Redis（随业务孤儿化） |
| `src/mainsvr/room/`、`service/c2s_room.go` | 建房/加入/快开/对局操作 C2S 转发 |
| `src/mysqlsvr/`（manager 包、仓储方法、handler） | `MysqlTexas*` 三表注册、`Update` 异步写池、`Query*` 三个查询 RPC |
| `module/misc/constant.go` | `ServerType_TexasGameSvr`(0x50) 及其路由规则 |
| `module/gfunc/{game_func.go,notify.go}` | `GetTexasRoomListIndex`、`GenerateRoomId`、玩法事件推送信封（零调用者） |
| `module/define/` | `RoomIdLen`（唯一使用者已删） |
| `module/gamedata/repository/texas/` | TexasConfig 系列生成代码 |
| `tools/gamesim/` | TexasGameSvr 最小模拟器 |
| `tools/tester/app/component/{room,archreg,rummy}/` | 房间链路/架构回归/复用德州牌结构的实验组件，及其 main 导入、toml 模块开关 |

保留：`roomcentersvr` 服务骨架（可启动的最小总线服务，新游戏房间系统在此落位）、
`ServerType_RummyGameSvr`(0x51)、role/session/DAL 三层持久化（未提交的在途工作未受影响）。

### 协议定义（common/game_proto → common/protocol）

- 删除文件：`config/texas.proto`、`core/room.proto`、`core/struct.proto`、
  `storage/db.proto`、`service/roomcentersvr.proto`。
- `core/cmd.proto`：删除 `TEXAS_INNER_*`(0x501000-0x501031)、`ROOM_CENTER_INNER_*`(0x0B1000-0x0B100B)、
  `MAIN_GAME_*`/`MAIN_QUERY_*`(0x020200-0x02024D)、`MYSQL_INNER_UPDATE/QUERY_*`(0x041006-0x04100D)、
  `SC_GAME_EVENT_NOTIFY`；`api/proto/goone/cmd/v1/cmd.proto` 镜像经 gencmdproto 同步。
- `core/game_enum.proto`：删除扑克语义枚举（CardType、GameState、BettingRound、OperateType、
  GameNotifyType、PlayerState、DealType、DataType、GameCompetitionType）与 `GameTypeId` 的
  TEXAS 值（0-7，新增 `GAME_TYPE_NONE=0` 占位满足 proto3 零值要求，RUMMY 20/21 原值保留）；
  保留通用原语 Color/Rank/CoinType/RoomStage/RoomState/RoomSortType。
- `core/client.proto`：删除房间/对局消息块（RoomList→QuickStartRollback）、
  `MysqlInnerUpdate*`、`Query*`、`GameUserEventNotify`。
- `core/database.proto`：删除 `MysqlTexasRoomInfo/PlayerInfo/GameInfo` 与五条
  `TexasGame*Record` 战绩消息。
- `core/error_code.proto`：删除 `ERR_TEXAS_*`(-20000~-20015)。
- `core/common.proto`：`DBType` 删除 `DB_TYPE_ROOM_CENTER_INFO`/`DB_TYPE_TEXAS_ROOM`。
- `service/mainsvrc2s.proto`：删除全部德州玩法 RPC（约 40 个，多为一键 stub）。
- `service/mysqlservice.proto`：删除 `Update`/`QueryRoomInfo`/`QueryPlayerInfo`/`QueryGameInfo`；
  DAL 角色四 RPC（UpdateRoleInfo/SearchRole/SaveRoleData/LoadRoleData）保留。
- 配置数据：`common/game_data/Texas*.conf`、`tools/cfgtool/xls/Texas.xlsx` 及再生的
  json/lua/proto 产物。

### 验证

- `go build ./...`、`go vet ./...`、`go test ./...` 全绿（主仓）；六个服务与 tester/stress 工具均可出二进制。
- 生成链重跑且幂等：`common/gen_proto.sh`、`go run ./tools/cmd/genproto`、`scripts/gencmdproto.sh`。
  `./main.sh check-genproto --full` 在未提交前必报"out of date"（其实现为 `git diff` 对比已提交状态），
  提交后即通过。
- `lib/db/gorm` 的 schema/integration 测试夹具由 `MysqlTexasRoomInfo` 换为 `MysqlRoleInfo`/`MysqlRoleData`。

### 已知遗留（非本次范围）

- `common/game_conf/gen_error.gen.go` 包名为 `package const`（保留字），系 xlsx 工具生成的
  预先存在损坏，主仓不引用该包；需在 cfgtool 侧修复生成器后重生成。
- `common/game_data/gamedata.tar` 打包产物未重建（内含旧 Texas 配置，重打包前勿上线分发）。
- `common` 为 g1_common 子模块，本批协议删除需随既有的"发布 g1_common"流程一并处理
  （go.mod replace 本地生效中）。
- 历史 bug 修复成果（F02/F03/F07、2026-09-27 前的快开回滚等）的测试载体已随业务删除，
  结论沉淀在 `docs/architecture_optimization_report_2026-09-08.md`（已加状态注记）。
