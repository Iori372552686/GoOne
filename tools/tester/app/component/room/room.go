package room

import (
	"context"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/Iori372552686/GoOne/tools/tester/app/component"
	"github.com/Iori372552686/GoOne/tools/tester/internal/session"
	"github.com/Iori372552686/GoOne/tools/tester/internal/testcfg"
	g1_protocol "github.com/Iori372552686/g1_common/protocol"
	"github.com/golang/protobuf/proto"
)

func init() {
	component.Register("room", func() component.TesterComponent {
		return &RoomComponent{}
	})
}

// RoomComponent 房间相关回归/压测组件。
//
// 覆盖 roomcentersvr 重构后的关键路径（与 tools/gamesim 模拟游戏服联调）：
//   - 房间列表：正常分页 / 参数校验（PageIndex=0 → ERR_ARGV）/ Stage_ALL 聚合
//   - 快速开始：选房 + 加入对局 + gamesvr 权威人数上报收敛
//   - 回滚路径：TESTER_EXPECT_QS_FAIL=1 时（配合 gamesim GAMESIM_REJECT_QUICKSTART=1）
//     快开应失败，且失败后 roomcenter 占位被回滚（房间人数回到基线）
//   - 加入/离开指定房间
type RoomComponent struct {
	actorID   int
	accountID string
	userID    int64
	sender    component.MessageSender
	requester component.Requester
	cfg       *testcfg.Config

	roomID uint64
}

func (r *RoomComponent) Name() string { return "room" }

func (r *RoomComponent) OnInit(ctx *component.ComponentContext) error {
	r.actorID = ctx.ActorID
	r.accountID = ctx.AccountID
	r.userID = ctx.UserID
	r.sender = ctx.Sender
	r.requester = ctx.Requester
	r.cfg = ctx.Cfg
	log.Printf("[Actor %d][Room] Component initialized", r.actorID)
	return nil
}

func (r *RoomComponent) OnConnected() error {
	log.Printf("[Actor %d][Room] Connected to gateway", r.actorID)
	return nil
}

func (r *RoomComponent) OnAccountLogin(accountID string) error {
	r.accountID = accountID
	log.Printf("[Actor %d][Room] Account logged in: %s", r.actorID, accountID)
	return nil
}

func (r *RoomComponent) OnRoleLogin(userID int64) error {
	r.userID = userID
	log.Printf("[Actor %d][Room] Role logged in: uid=%d", r.actorID, userID)
	return nil
}

// expectQSFail 是否处于"快开失败"场景（验证回滚路径）。
func (r *RoomComponent) expectQSFail() bool { return os.Getenv("TESTER_EXPECT_QS_FAIL") == "1" }

// RunTests 回归用例集。用例间存在顺序依赖（T05 加入的房间来自 T04 的快开结果）。
func (r *RoomComponent) RunTests(ctx context.Context) error {
	log.Printf("[Actor %d][Room] ===== Starting room tests (qsFailMode=%v) =====",
		r.actorID, r.expectQSFail())

	tests := []struct {
		name string
		fn   func(ctx context.Context) error
	}{
		{"T01_RoomList_OK", r.testRoomList},
		{"T02_RoomList_InvalidPageArg", r.testRoomListInvalidPage},
		{"T03_RoomList_StageALL", r.testRoomListStageAll},
		{"T04_QuickStart", r.testQuickStart},
		{"T05_JoinRoom", r.testJoinRoom},
		{"T06_LeaveRoom", r.testLeaveRoom},
	}

	for _, test := range tests {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		log.Printf("[Actor %d][Room] --- %s ---", r.actorID, test.name)
		if err := test.fn(ctx); err != nil {
			return fmt.Errorf("%s: %w", test.name, err)
		}
		log.Printf("[Actor %d][Room] --- %s PASSED ---", r.actorID, test.name)
	}

	log.Printf("[Actor %d][Room] ===== All room tests PASSED =====", r.actorID)
	return nil
}

// RunStress 压测循环：快速开始 → 离开房间，反复执行。
func (r *RoomComponent) RunStress(ctx context.Context) error {
	if err := r.quickStart(ctx); err != nil {
		return err
	}
	if err := r.leaveRoom(ctx); err != nil {
		return err
	}
	return nil
}

func (r *RoomComponent) OnMessage(cmd uint32, data []byte) bool {
	return false
}

// ---- 请求辅助 ----

// request 通用请求-响应封装。
func (r *RoomComponent) request(ctx context.Context, cmd g1_protocol.CMD, req, rsp proto.Message) error {
	return r.requester.RequestProto(ctx, uint32(cmd), req, rsp, 30*time.Second)
}

// fetchRoomList 拉取指定 stage 的房间列表（合法分页参数），返回房间信息切片。
func (r *RoomComponent) fetchRoomList(ctx context.Context, stage g1_protocol.RoomStage) ([]*g1_protocol.RoomShowInfo, error) {
	req := &g1_protocol.RoomListReq{
		GameId:    g1_protocol.GameTypeId_TEXAS_NORMAL,
		Stage:     stage,
		PageIndex: 1,
		PageSize:  100,
		SortType:  g1_protocol.RoomSortType_SORT_TYPE_ID,
		CoinType:  g1_protocol.CoinType_COIN_GOLD,
	}
	resp := &g1_protocol.RoomListRsp{Ret: &g1_protocol.Ret{}}
	if err := r.request(ctx, g1_protocol.CMD_MAIN_GAME_ROOM_LIST_REQ, req, resp); err != nil {
		return nil, err
	}
	if session.IsErrCode(int32(resp.Ret.GetCode())) {
		return nil, fmt.Errorf("room list failed: code=%d msg=%s", resp.Ret.GetCode(), resp.Ret.GetMsg())
	}
	return resp.GetRoomList(), nil
}

// stageTotalPlayers 统计某 stage 全部房间的在座人数（回滚校验基线用）。
func (r *RoomComponent) stageTotalPlayers(ctx context.Context, stage g1_protocol.RoomStage) (int, error) {
	rooms, err := r.fetchRoomList(ctx, stage)
	if err != nil {
		return 0, err
	}
	total := 0
	for _, room := range rooms {
		if room.GetBase() != nil {
			total += int(room.GetBase().GetCurPlayerNum())
		}
	}
	return total, nil
}

// ---- 用例实现 ----

// T01 房间列表正常路径（修复历史用例缺陷：PageIndex 必须从 1 开始）。
func (r *RoomComponent) testRoomList(ctx context.Context) error {
	rooms, err := r.fetchRoomList(ctx, g1_protocol.RoomStage_LOW)
	if err != nil {
		return err
	}
	log.Printf("[Actor %d][Room] T01: room list OK, stage=Free count=%d", r.actorID, len(rooms))
	return nil
}

// T02 参数校验：PageIndex=0 应被 roomcenter 拒绝并返回 ERR_ARGV
// （验证重构后的参数防御链路真实到达客户端，而非超时/静默）。
func (r *RoomComponent) testRoomListInvalidPage(ctx context.Context) error {
	req := &g1_protocol.RoomListReq{
		GameId:    g1_protocol.GameTypeId_TEXAS_NORMAL,
		Stage:     g1_protocol.RoomStage_LOW,
		PageIndex: 0,
		PageSize:  10,
		CoinType:  g1_protocol.CoinType_COIN_GOLD,
	}
	resp := &g1_protocol.RoomListRsp{Ret: &g1_protocol.Ret{}}
	if err := r.request(ctx, g1_protocol.CMD_MAIN_GAME_ROOM_LIST_REQ, req, resp); err != nil {
		return err
	}
	if resp.Ret.GetCode() != g1_protocol.ErrorCode_ERR_ARGV {
		return fmt.Errorf("PageIndex=0 应返回 ERR_ARGV(-7)，实际 code=%v", resp.Ret.GetCode())
	}
	log.Printf("[Actor %d][Room] T02: invalid page rejected with ERR_ARGV as expected", r.actorID)
	return nil
}

// T03 Stage_ALL 聚合查询：跨 stage 汇总房间。
func (r *RoomComponent) testRoomListStageAll(ctx context.Context) error {
	rooms, err := r.fetchRoomList(ctx, g1_protocol.RoomStage_Stage_ALL)
	if err != nil {
		return err
	}
	log.Printf("[Actor %d][Room] T03: stage=ALL aggregated OK, count=%d", r.actorID, len(rooms))
	return nil
}

// T04 快速开始：
//   - 正常模式：应成功拿到房间并加入（gamesim 入座 + 权威上报收敛）；
//   - 失败模式（TESTER_EXPECT_QS_FAIL=1）：应收到明确错误码（非超时），
//     且失败后 roomcenter 占位被回滚 —— 以 stage 总人数回到基线验证。
func (r *RoomComponent) testQuickStart(ctx context.Context) error {
	if !r.expectQSFail() {
		if err := r.quickStart(ctx); err != nil {
			return err
		}
		if r.roomID == 0 {
			return fmt.Errorf("quick start OK but no room allocated")
		}
		log.Printf("[Actor %d][Room] T04: quick start OK, roomID=%d", r.actorID, r.roomID)
		return nil
	}

	// 失败模式：先取基线 → 快开（期望失败）→ 校验回滚。
	baseline, err := r.stageTotalPlayers(ctx, g1_protocol.RoomStage_LOW)
	if err != nil {
		return fmt.Errorf("rollback baseline: %w", err)
	}

	qsErr := r.quickStart(ctx)
	if qsErr == nil {
		return fmt.Errorf("期望快开失败（gamesim 拒绝模式），实际成功 roomID=%d", r.roomID)
	}
	log.Printf("[Actor %d][Room] T04: quick start failed as expected: %v", r.actorID, qsErr)

	// 回滚是 one-way，留出在途时间窗口。
	time.Sleep(2 * time.Second)

	after, err := r.stageTotalPlayers(ctx, g1_protocol.RoomStage_LOW)
	if err != nil {
		return fmt.Errorf("rollback verify: %w", err)
	}
	if after != baseline {
		return fmt.Errorf("回滚未生效：快开失败后 stage 总人数应回到基线 %d，实际 %d", baseline, after)
	}
	log.Printf("[Actor %d][Room] T04: rollback verified, stage players %d -> %d", r.actorID, baseline, after)
	return nil
}

// quickStart 发起快速开始。成功时记录 roomID；失败时返回错误。
func (r *RoomComponent) quickStart(ctx context.Context) error {
	req := &g1_protocol.QuickStartReq{
		GameId:   g1_protocol.GameTypeId_TEXAS_NORMAL,
		CoinType: g1_protocol.CoinType_COIN_GOLD,
		Stage:    g1_protocol.RoomStage_LOW,
	}
	resp := &g1_protocol.QuickStartRsp{Ret: &g1_protocol.Ret{}}
	if err := r.request(ctx, g1_protocol.CMD_MAIN_GAME_QUICK_START_REQ, req, resp); err != nil {
		return err
	}
	if session.IsErrCode(int32(resp.Ret.GetCode())) {
		return fmt.Errorf("quick start failed: code=%d msg=%s", resp.Ret.GetCode(), resp.Ret.GetMsg())
	}
	if resp.RoomInfo != nil && resp.RoomInfo.RoomId != 0 {
		r.roomID = resp.RoomInfo.RoomId
	}
	return nil
}

// T05 显式加入房间（roomID 来自 T04 快开结果；失败模式下跳过加入动作本身）。
func (r *RoomComponent) testJoinRoom(ctx context.Context) error {
	if r.expectQSFail() {
		// 失败模式下没有可用房间：用 0 号房间验证"非法房间号"参数防御。
		req := &g1_protocol.JoinRoomReq{RoomId: 0}
		resp := &g1_protocol.JoinRoomRsp{Ret: &g1_protocol.Ret{}}
		if err := r.request(ctx, g1_protocol.CMD_MAIN_GAME_JOIN_ROOM_REQ, req, resp); err != nil {
			return err
		}
		if resp.Ret.GetCode() != g1_protocol.ErrorCode_ERR_ARGV {
			return fmt.Errorf("RoomId=0 应返回 ERR_ARGV(-7)，实际 code=%v", resp.Ret.GetCode())
		}
		log.Printf("[Actor %d][Room] T05: join invalid room rejected with ERR_ARGV as expected", r.actorID)
		return nil
	}

	if r.roomID == 0 {
		return fmt.Errorf("no room to join (T04 did not allocate)")
	}
	req := &g1_protocol.JoinRoomReq{RoomId: r.roomID, ConnBusId: 0}
	resp := &g1_protocol.JoinRoomRsp{Ret: &g1_protocol.Ret{}}
	if err := r.request(ctx, g1_protocol.CMD_MAIN_GAME_JOIN_ROOM_REQ, req, resp); err != nil {
		return err
	}
	if session.IsErrCode(int32(resp.Ret.GetCode())) {
		return fmt.Errorf("join room failed: code=%d msg=%s", resp.Ret.GetCode(), resp.Ret.GetMsg())
	}
	log.Printf("[Actor %d][Room] T05: join room OK, roomID=%d", r.actorID, r.roomID)
	return nil
}

// T06 离开房间（失败模式下 roomID=0，验证参数拒绝路径 ERR_ARGV）。
func (r *RoomComponent) testLeaveRoom(ctx context.Context) error {
	if r.expectQSFail() {
		req := &g1_protocol.LeaveGameReq{RoomId: 0}
		resp := &g1_protocol.LeaveGameRsp{Ret: &g1_protocol.Ret{}}
		if err := r.request(ctx, g1_protocol.CMD_MAIN_GAME_LEAVE_GAME_REQ, req, resp); err != nil {
			return err
		}
		if resp.Ret.GetCode() != g1_protocol.ErrorCode_ERR_ARGV {
			return fmt.Errorf("RoomId=0 应返回 ERR_ARGV(-7)，实际 code=%v", resp.Ret.GetCode())
		}
		log.Printf("[Actor %d][Room] T06: leave invalid room rejected with ERR_ARGV as expected", r.actorID)
		return nil
	}

	if err := r.leaveRoom(ctx); err != nil {
		return err
	}
	log.Printf("[Actor %d][Room] T06: leave room OK", r.actorID)
	return nil
}

func (r *RoomComponent) leaveRoom(ctx context.Context) error {
	req := &g1_protocol.LeaveGameReq{RoomId: r.roomID}
	resp := &g1_protocol.LeaveGameRsp{Ret: &g1_protocol.Ret{}}
	if err := r.request(ctx, g1_protocol.CMD_MAIN_GAME_LEAVE_GAME_REQ, req, resp); err != nil {
		return err
	}
	if session.IsErrCode(int32(resp.Ret.GetCode())) {
		return fmt.Errorf("leave room failed: code=%d msg=%s", resp.Ret.GetCode(), resp.Ret.GetMsg())
	}
	r.roomID = 0
	return nil
}
