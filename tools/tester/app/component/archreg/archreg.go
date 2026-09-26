// Package archreg 是架构迭代（F01-F09）专项回归组件：对 connsvr/mainsvr/
// roomcentersvr/infosvr 的今日改动做联调级端到端验证。用例与缺陷编号对应：
//   - F01 登录身份：私有会话登录回环、多端登录互踢
//   - F04/F05 持久化：登出-重登数据一致、登出幂等
//   - F06 心跳：心跳应答与会话保持
//   - F02/F03 房间：快开分配属性匹配（stage/coin 互换回归）、目录属性一致、
//     拒绝模式下的占位回滚（票据路径，env ARCHREG_QS_REJECT=1 + gamesim 拒绝模式）
//   - F06 长周期：心跳过期踢下线（env ARCHREG_EXPIRY=1 单独跑，约 2.5 分钟）
package archreg

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
	component.Register("archreg", func() component.TesterComponent {
		return &ArchRegComponent{}
	})
}

type ArchRegComponent struct {
	actorID   int
	accountID string
	userID    int64
	sender    component.MessageSender
	requester component.Requester
	cfg       *testcfg.Config

	qsRoomID uint64
}

func (c *ArchRegComponent) Name() string { return "archreg" }

func (c *ArchRegComponent) OnInit(ctx *component.ComponentContext) error {
	c.actorID = ctx.ActorID
	c.accountID = ctx.AccountID
	c.userID = ctx.UserID
	c.sender = ctx.Sender
	c.requester = ctx.Requester
	c.cfg = ctx.Cfg
	log.Printf("[Actor %d][ArchReg] initialized", c.actorID)
	return nil
}

func (c *ArchRegComponent) OnConnected() error                { return nil }
func (c *ArchRegComponent) OnAccountLogin(accountID string) error { return nil }
func (c *ArchRegComponent) OnRoleLogin(userID int64) error    { return nil }
func (c *ArchRegComponent) OnMessage(cmd uint32, data []byte) bool { return false }

// RunTests 回归用例集（顺序敏感：互踢用例放在私有会话组内，不影响主会话）。
func (c *ArchRegComponent) RunTests(ctx context.Context) error {
	qsReject := os.Getenv("ARCHREG_QS_REJECT") == "1"
	tests := []struct {
		name string
		fn   func(ctx context.Context) error
	}{
		{"R01_PrivateLoginRoleData", c.testPrivateLogin},
		{"R02_PersistAcrossLogout", c.testPersistAcrossLogout},
		{"R03_LogoutIdempotent", c.testLogoutIdempotent},
		{"R04_MultiPlaceKick", c.testMultiPlaceKick},
		{"R06_Heartbeat", c.testHeartbeat},
	}
	if !qsReject {
		// 正常路径房间用例（拒绝模式下快开必然失败，由 R09 专门覆盖）。
		tests = append(tests,
			struct {
				name string
				fn   func(ctx context.Context) error
			}{"R07_QuickStartAttrs", c.testQuickStartAttrs},
			struct {
				name string
				fn   func(ctx context.Context) error
			}{"R08_RoomListAttrs", c.testRoomListAttrs})
	}
	if qsReject {
		tests = append(tests, struct {
			name string
			fn   func(ctx context.Context) error
		}{"R09_QSRejectRollback", c.testQSRejectRollback})
	}
	if os.Getenv("ARCHREG_EXPIRY") == "1" {
		tests = append(tests, struct {
			name string
			fn   func(ctx context.Context) error
		}{"R10_HeartbeatExpiryKick", c.testExpiryKick})
	}

	for _, t := range tests {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}
		log.Printf("[Actor %d][ArchReg] --- %s ---", c.actorID, t.name)
		if err := t.fn(ctx); err != nil {
			return fmt.Errorf("%s: %w", t.name, err)
		}
		log.Printf("[Actor %d][ArchReg] --- %s PASSED ---", c.actorID, t.name)
	}
	log.Printf("[Actor %d][ArchReg] ===== all archreg tests PASSED =====", c.actorID)
	return nil
}

// RunStress 饱和压测循环：心跳 + 快开/离房（覆盖今日改动路径的轻量正常流）。
func (c *ArchRegComponent) RunStress(ctx context.Context) error {
	if err := c.heartbeat(ctx); err != nil {
		return err
	}
	if err := c.quickStartAndLeave(ctx); err != nil {
		return err
	}
	return nil
}

// ---- 私有会话辅助 ----

// newPrivateSession 构造独立 uid 的私有会话（不与主会话/其他 actor 冲突：
// uid = start_uid + 50000 + offset）。
func (c *ArchRegComponent) newPrivateSession(offset int64) *session.Session {
	uid := c.cfg.Player.StartUID + 50000 + offset
	return session.New(session.Options{
		ID:         int(9000 + offset),
		Transport:  c.cfg.Server.Transport,
		Host:       c.cfg.Server.Host,
		TcpPort:    c.cfg.Server.TcpPort,
		WsPort:     c.cfg.Server.WsPort,
		WsPath:     c.cfg.Server.WsPath,
		Channel:    c.cfg.Player.Channel,
		AccountID:  fmt.Sprintf("%s_%d", c.cfg.Player.AccountPrefix, uid),
		DeviceID:   fmt.Sprintf("%s_%d", c.cfg.Player.DevicePrefix, uid),
		UserID:     uid,
		Token:      c.cfg.Player.Token,
	})
}

func (c *ArchRegComponent) request(ctx context.Context, cmd g1_protocol.CMD, req, rsp proto.Message) error {
	return c.requester.RequestProto(ctx, uint32(cmd), req, rsp, 30*time.Second)
}

// ---- 用例 ----

// R01 私有会话登录回环（F01 dev 预分配模型 + LoginRsp 数据链路）：
// 预分配 uid + 非数字账号登录成功，响应角色 uid 与预分配一致、角色名非空。
func (c *ArchRegComponent) testPrivateLogin(ctx context.Context) error {
	s := c.newPrivateSession(1)
	defer s.Close()
	if err := s.Connect(ctx); err != nil {
		return fmt.Errorf("connect: %w", err)
	}
	if err := s.Login(ctx); err != nil {
		return fmt.Errorf("login: %w", err)
	}
	if s.UserID() != c.cfg.Player.StartUID+50001 {
		return fmt.Errorf("登录后 uid 应回传预分配值，实际 %d", s.UserID())
	}
	return nil
}

// R02 登出-重登数据一致（F04 保存退出 + F05 原子保存）：
// GM 加道具 → 背包计数 → 登出 → 重登 → 计数一致。
func (c *ArchRegComponent) testPersistAcrossLogout(ctx context.Context) error {
	const itemID = int32(20101001)
	s := c.newPrivateSession(2)
	defer s.Close()
	if err := s.Connect(ctx); err != nil {
		return err
	}
	if err := s.Login(ctx); err != nil {
		return err
	}

	// 背包基线（uid 持久化跨轮次存在，用增量断言保证用例幂等）。
	baseCnt, err := privateItemCount(ctx, s, itemID)
	if err != nil {
		return err
	}

	// GM 加 3 个道具并确认增量入包。
	if err := privateGmAddItem(ctx, s, itemID, 3); err != nil {
		return err
	}
	before, err := privateItemCount(ctx, s, itemID)
	if err != nil {
		return err
	}
	if before != baseCnt+3 {
		return fmt.Errorf("GM 加道具后背包应 %d 个，实际 %d", baseCnt+3, before)
	}

	// 客户端登出（F04 用例路径）→ 应成功。
	if err := privateLogout(ctx, s); err != nil {
		return fmt.Errorf("logout: %w", err)
	}

	// 重登 → 数据从 Redis hash 重载（F05 单命令提交后的完整状态）。
	if err := s.Login(ctx); err != nil {
		return fmt.Errorf("relogin: %w", err)
	}
	after, err := privateItemCount(ctx, s, itemID)
	if err != nil {
		return err
	}
	if after != before {
		return fmt.Errorf("登出-重登后道具数应保持 %d，实际 %d（F04/F05 持久化回归）", before, after)
	}
	return nil
}

// R03 重复登出幂等（F04）：角色已移除后的再次登出应返回 ERR_OK 且不产生错误。
func (c *ArchRegComponent) testLogoutIdempotent(ctx context.Context) error {
	s := c.newPrivateSession(3)
	defer s.Close()
	if err := s.Connect(ctx); err != nil {
		return err
	}
	if err := s.Login(ctx); err != nil {
		return err
	}
	if err := privateLogout(ctx, s); err != nil {
		return err
	}
	// 第二次登出：角色已不在内存，用例幂等成功。
	req := &g1_protocol.LogoutReq{}
	rsp := &g1_protocol.LogoutRsp{Ret: &g1_protocol.Ret{}}
	if err := s.RequestProto(ctx, uint32(g1_protocol.CMD_MAIN_LOGOUT_REQ), req, rsp, 10*time.Second); err != nil {
		return fmt.Errorf("重复登出超时/传输错误: %w", err)
	}
	if rsp.GetRet().GetCode() != g1_protocol.ErrorCode_ERR_OK {
		return fmt.Errorf("重复登出应幂等返回 ERR_OK，实际 code=%d", rsp.GetRet().GetCode())
	}
	return nil
}

// R04 多端登录互踢（F01 会话绑定/顶替）：同 uid 第二个连接登录成功后，
// 旧连接应在短时间内被断开（MULTI_PLACE_LOGIN），新连接可用。
func (c *ArchRegComponent) testMultiPlaceKick(ctx context.Context) error {
	s1 := c.newPrivateSession(4)
	defer s1.Close()
	if err := s1.Connect(ctx); err != nil {
		return err
	}
	if err := s1.Login(ctx); err != nil {
		return fmt.Errorf("s1 login: %w", err)
	}

	s2 := c.newPrivateSession(4) // 同 uid
	defer s2.Close()
	if err := s2.Connect(ctx); err != nil {
		return err
	}
	if err := s2.Login(ctx); err != nil {
		return fmt.Errorf("s2 login: %w", err)
	}

	// 轮询旧连接断开（kick 写包后 close）。
	deadline := time.Now().Add(5 * time.Second)
	for s1.Connected() && time.Now().Before(deadline) {
		time.Sleep(100 * time.Millisecond)
	}
	if s1.Connected() {
		return fmt.Errorf("多端登录后旧连接应被踢断（5s 内），仍在线")
	}

	// 新连接心跳可用。
	if err := privateHeartbeat(ctx, s2); err != nil {
		return fmt.Errorf("顶替连接心跳失败: %w", err)
	}
	if err := privateLogout(ctx, s2); err != nil {
		return fmt.Errorf("顶替连接登出: %w", err)
	}
	return nil
}

// R06 心跳应答（F06）：连续心跳成功且 ServerNowMs 单调可用。
func (c *ArchRegComponent) testHeartbeat(ctx context.Context) error {
	for i := 0; i < 3; i++ {
		if err := c.heartbeat(ctx); err != nil {
			return err
		}
	}
	return nil
}

func (c *ArchRegComponent) heartbeat(ctx context.Context) error {
	req := &g1_protocol.HeartBeatReq{ClientNowMs: time.Now().UnixMilli()}
	rsp := &g1_protocol.HeartBeatRsp{Ret: &g1_protocol.Ret{}}
	if err := c.request(ctx, g1_protocol.CMD_MAIN_HEARTBEAT_REQ, req, rsp); err != nil {
		return err
	}
	if rsp.GetRet().GetCode() != g1_protocol.ErrorCode_ERR_OK {
		return fmt.Errorf("heartbeat code=%d", rsp.GetRet().GetCode())
	}
	if rsp.GetServerNowMs() <= 0 {
		return fmt.Errorf("heartbeat ServerNowMs 应为正数，实际 %d", rsp.GetServerNowMs())
	}
	return nil
}

// R07 快开分配属性匹配（F02 回归）：请求 Stage=MIDDLE/Coin=GOLD（两值不同，
// 互换即可暴露），响应房间的 Stage/CoinType 必须与请求一致；连续两次覆盖
// 建房分支与选房分支。
func (c *ArchRegComponent) testQuickStartAttrs(ctx context.Context) error {
	for i := 0; i < 2; i++ {
		if err := c.quickStartAndLeave(ctx); err != nil {
			return err
		}
	}
	return nil
}

func (c *ArchRegComponent) quickStartAndLeave(ctx context.Context) error {
	roomID, err := c.quickStart(ctx)
	if err != nil {
		return err
	}
	c.qsRoomID = roomID
	return c.leaveRoom(ctx, roomID)
}

func (c *ArchRegComponent) quickStart(ctx context.Context) (uint64, error) {
	req := &g1_protocol.QuickStartReq{
		GameId:   g1_protocol.GameTypeId_TEXAS_NORMAL,
		CoinType: g1_protocol.CoinType_COIN_GOLD,
		Stage:    g1_protocol.RoomStage_MIDDLE,
	}
	rsp := &g1_protocol.QuickStartRsp{Ret: &g1_protocol.Ret{}}
	if err := c.request(ctx, g1_protocol.CMD_MAIN_GAME_QUICK_START_REQ, req, rsp); err != nil {
		return 0, err
	}
	if session.IsErrCode(int32(rsp.Ret.GetCode())) {
		return 0, fmt.Errorf("quick start failed: code=%d msg=%s", rsp.Ret.GetCode(), rsp.Ret.GetMsg())
	}
	info := rsp.GetRoomInfo()
	if info == nil || info.RoomId == 0 {
		return 0, fmt.Errorf("quick start OK but no room info")
	}
	// F02 断言：分配房间的场次/币种必须与请求一致（历史缺陷曾互换两参数）。
	if info.GetStage() != g1_protocol.RoomStage_MIDDLE || info.GetCoinType() != g1_protocol.CoinType_COIN_GOLD {
		return 0, fmt.Errorf("F02 回归：房间属性与请求不符 stage=%v coin=%v（期望 MIDDLE/GOLD）",
			info.GetStage(), info.GetCoinType())
	}
	return info.RoomId, nil
}

func (c *ArchRegComponent) leaveRoom(ctx context.Context, roomID uint64) error {
	req := &g1_protocol.LeaveGameReq{RoomId: roomID}
	rsp := &g1_protocol.LeaveGameRsp{Ret: &g1_protocol.Ret{}}
	if err := c.request(ctx, g1_protocol.CMD_MAIN_GAME_LEAVE_GAME_REQ, req, rsp); err != nil {
		return err
	}
	if session.IsErrCode(int32(rsp.Ret.GetCode())) {
		return fmt.Errorf("leave room failed: code=%d msg=%s", rsp.Ret.GetCode(), rsp.Ret.GetMsg())
	}
	return nil
}

// R08 目录属性一致（F02/F03）：快开分配的房间在 RoomList(MIDDLE) 中可见且属性一致。
func (c *ArchRegComponent) testRoomListAttrs(ctx context.Context) error {
	roomID, err := c.quickStart(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = c.leaveRoom(ctx, roomID) }()

	rooms, err := c.fetchRoomList(ctx, g1_protocol.RoomStage_MIDDLE)
	if err != nil {
		return err
	}
	for _, r := range rooms {
		if r.GetBase().GetRoomId() == roomID {
			if r.GetBase().GetStage() != g1_protocol.RoomStage_MIDDLE ||
				r.GetBase().GetCoinType() != g1_protocol.CoinType_COIN_GOLD {
				return fmt.Errorf("目录房间属性与请求不符 stage=%v coin=%v", r.GetBase().GetStage(), r.GetBase().GetCoinType())
			}
			return nil
		}
	}
	return fmt.Errorf("RoomList(MIDDLE) 未包含快开分配的房间 %d", roomID)
}

// R09 拒绝模式回滚（F03 票据路径端到端，需 gamesim GAMESIM_REJECT_QUICKSTART=1）：
// 快开失败（明确错误码非超时）→ mainsvr 按票据回滚 ×3 次重试 → stage 总人数回到基线
//（旧实现重复回滚会多减席位；票据实现每张至多释放一次）。
func (c *ArchRegComponent) testQSRejectRollback(ctx context.Context) error {
	baseline, err := c.stageTotalPlayers(ctx, g1_protocol.RoomStage_MIDDLE)
	if err != nil {
		return fmt.Errorf("baseline: %w", err)
	}

	qsErr := func() error {
		_, err := c.quickStart(ctx)
		return err
	}()
	if qsErr == nil {
		return fmt.Errorf("拒绝模式下快开应失败，实际成功")
	}

	// 回滚 one-way 在途窗口。
	time.Sleep(3 * time.Second)

	after, err := c.stageTotalPlayers(ctx, g1_protocol.RoomStage_MIDDLE)
	if err != nil {
		return fmt.Errorf("verify: %w", err)
	}
	if after != baseline {
		return fmt.Errorf("F03 回滚未收敛：stage 总人数应回基线 %d，实际 %d", baseline, after)
	}
	return nil
}

// R10 心跳过期踢下线（F06 长周期，单独跑）：登录后停止心跳，但用"不刷新角色
// 心跳"的背包查询保活连接（避开 connsvr TCP 空闲 60s 关闭的干扰），应在过期
// 阈值(120s) + Tick(60s) + UID 域复检踢人的窗口内被服务端主动断开。
func (c *ArchRegComponent) testExpiryKick(ctx context.Context) error {
	s := c.newPrivateSession(5)
	defer s.Close()
	if err := s.Connect(ctx); err != nil {
		return err
	}
	if err := s.Login(ctx); err != nil {
		return err
	}

	// 用背包查询保活 TCP（QueryBackpack 不触发 OnClientHeartbeat）。
	kicked := false
	deadline := time.Now().Add(300 * time.Second)
	for time.Now().Before(deadline) {
		if !s.Connected() {
			kicked = true
			break
		}
		req := &g1_protocol.QueryBackpackReq{BagType: 0, PageIdx: 1, PageSize: 10}
		rsp := &g1_protocol.QueryBackpackRsp{Ret: &g1_protocol.Ret{}}
		if err := s.RequestProto(ctx, uint32(g1_protocol.CMD_MAIN_BACKPACK_QUERY_REQ), req, rsp, 10*time.Second); err != nil {
			// 请求失败且连接已断 → 被服务端踢下线。
			if !s.Connected() {
				kicked = true
				break
			}
			return fmt.Errorf("保活背包查询失败: %w", err)
		}
		time.Sleep(20 * time.Second)
	}
	if !kicked {
		return fmt.Errorf("300s 内未被心跳过期踢下线（F06：阈值 120s + Tick 60s）")
	}
	return nil
}

// ---- 通用子操作 ----

func (c *ArchRegComponent) fetchRoomList(ctx context.Context, stage g1_protocol.RoomStage) ([]*g1_protocol.RoomShowInfo, error) {
	req := &g1_protocol.RoomListReq{
		GameId:    g1_protocol.GameTypeId_TEXAS_NORMAL,
		Stage:     stage,
		PageIndex: 1,
		PageSize:  100,
		SortType:  g1_protocol.RoomSortType_SORT_TYPE_ID,
		CoinType:  g1_protocol.CoinType_COIN_GOLD,
	}
	rsp := &g1_protocol.RoomListRsp{Ret: &g1_protocol.Ret{}}
	if err := c.request(ctx, g1_protocol.CMD_MAIN_GAME_ROOM_LIST_REQ, req, rsp); err != nil {
		return nil, err
	}
	if session.IsErrCode(int32(rsp.Ret.GetCode())) {
		return nil, fmt.Errorf("room list failed: code=%d", rsp.Ret.GetCode())
	}
	return rsp.GetRoomList(), nil
}

func (c *ArchRegComponent) stageTotalPlayers(ctx context.Context, stage g1_protocol.RoomStage) (int, error) {
	rooms, err := c.fetchRoomList(ctx, stage)
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

// ---- 私有会话子操作 ----

func privateGmAddItem(ctx context.Context, s *session.Session, itemID int32, count int64) error {
	req := &g1_protocol.GMAddItemReq{Id: itemID, Count: count}
	rsp := &g1_protocol.GMAddItemRsp{Ret: &g1_protocol.Ret{}}
	if err := s.RequestProto(ctx, uint32(g1_protocol.CMD_MAIN_GM_ADD_ITEM_REQ), req, rsp, 10*time.Second); err != nil {
		return err
	}
	if session.IsErrCode(int32(rsp.GetRet().GetCode())) {
		return fmt.Errorf("gm add item %d failed: code=%d", itemID, rsp.GetRet().GetCode())
	}
	return nil
}

// privateItemCount 从背包分页中统计指定道具数量。
func privateItemCount(ctx context.Context, s *session.Session, itemID int32) (int64, error) {
	req := &g1_protocol.QueryBackpackReq{BagType: 0, PageIdx: 1, PageSize: 100}
	rsp := &g1_protocol.QueryBackpackRsp{Ret: &g1_protocol.Ret{}}
	if err := s.RequestProto(ctx, uint32(g1_protocol.CMD_MAIN_BACKPACK_QUERY_REQ), req, rsp, 10*time.Second); err != nil {
		return 0, err
	}
	if session.IsErrCode(int32(rsp.GetRet().GetCode())) {
		return 0, fmt.Errorf("query backpack failed: code=%d", rsp.GetRet().GetCode())
	}
	for _, it := range rsp.GetItems() {
		if it.GetId() == itemID {
			return it.GetCount(), nil
		}
	}
	return 0, nil
}

func privateLogout(ctx context.Context, s *session.Session) error {
	req := &g1_protocol.LogoutReq{}
	rsp := &g1_protocol.LogoutRsp{Ret: &g1_protocol.Ret{}}
	if err := s.RequestProto(ctx, uint32(g1_protocol.CMD_MAIN_LOGOUT_REQ), req, rsp, 10*time.Second); err != nil {
		return err
	}
	if session.IsErrCode(int32(rsp.GetRet().GetCode())) {
		return fmt.Errorf("logout failed: code=%d msg=%s", rsp.GetRet().GetCode(), rsp.GetRet().GetMsg())
	}
	return nil
}

func privateHeartbeat(ctx context.Context, s *session.Session) error {
	req := &g1_protocol.HeartBeatReq{ClientNowMs: time.Now().UnixMilli()}
	rsp := &g1_protocol.HeartBeatRsp{Ret: &g1_protocol.Ret{}}
	if err := s.RequestProto(ctx, uint32(g1_protocol.CMD_MAIN_HEARTBEAT_REQ), req, rsp, 10*time.Second); err != nil {
		return err
	}
	if session.IsErrCode(int32(rsp.GetRet().GetCode())) {
		return fmt.Errorf("heartbeat failed: code=%d", rsp.GetRet().GetCode())
	}
	return nil
}
