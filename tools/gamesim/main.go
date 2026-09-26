// gamesim 是 TexasGameSvr 的最小模拟器（模拟测试基础设施，见
// tools/tester/app/component/room 的联调说明）：
//
// 以 ServerType_TexasGameSvr(0x50) 身份注册进服务发现，处理对局侧内部协议：
//   - CMD_TEXAS_INNER_CREATEROOM_REQ（roomcenter 建房工厂 one-way）：登记房间，
//     并把房间上报回 roomcenter 目录（权威人数来源）
//   - CMD_TEXAS_INNER_QUICK_START_REQ（mainsvr 调用）：入座；拒绝模式下返回错误
//     （用于驱动 mainsvr → roomcenter 的占位回滚路径，验证 F03 预约票据）
//   - CMD_TEXAS_INNER_JOINROOM_REQ / CMD_TEXAS_INNER_LEAVE_GAME_REQ（mainsvr 调用）
//
// 环境变量：
//   - GAMESIM_REJECT_QUICKSTART=1  拒绝所有快速开始（回滚路径测试）
//   - GAMESIM_NO_AUTOSEED=1        建房请求不上报目录（制造空目录 → QS 走建房分支）
//   - GAMESIM_REPORT_ZONE          建房上报使用的 zone（默认 0，与 tester 一致）
//
// 启动：gamesim.exe -svr_conf etc/config/server_conf_ide.yaml
package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"strconv"
	"sync"

	roomcenterv1 "github.com/Iori372552686/GoOne/api/gen/game/roomcenter/v1"
	"github.com/Iori372552686/GoOne/lib/api/cmd_handler"
	"github.com/Iori372552686/GoOne/lib/api/logger"
	"github.com/Iori372552686/GoOne/lib/db/redis"
	"github.com/Iori372552686/GoOne/lib/service/bus/driver/rabbitmq"
	"github.com/Iori372552686/GoOne/lib/service/router"
	"github.com/Iori372552686/GoOne/lib/service/runtime/bussvc"
	"github.com/Iori372552686/GoOne/lib/service/transaction"
	"github.com/Iori372552686/GoOne/module/gfunc"
	"github.com/golang/protobuf/proto"
	g1_protocol "github.com/Iori372552686/g1_common/protocol"
)

var transMgr = transaction.NewTransactionMgr()

func main() {
	flag.Parse()
	defer logger.Flush()

	app := bussvc.MustNew("gamesim", router.ReadyCheck, bussvc.WithConfLoader())

	registerHandlers := &bussvc.FuncComponent{
		ComponentName: "gamesim_handlers",
		OnStart: func(_ context.Context) error {
			return registerCmds()
		},
	}

	// businessDeps：Redis 房间持久化（真实游戏服重启不丢房间的最小等价物）。
	// 启动时恢复房间表并上报目录，使 roomcenter 侧的登记与权威侧收敛。
	businessDeps := &bussvc.FuncComponent{
		ComponentName: "gamesim_deps",
		OnStart: func(ctx context.Context) (err error) {
			redisStarted := false
			defer func() {
				if err != nil && redisStarted {
					_ = redisMgr.Close()
				}
			}()
			if err = redisMgr.OnStart(ctx); err != nil {
				return err
			}
			redisStarted = true
			restoreRooms(ctx)
			return nil
		},
		OnStop: func(_ context.Context) error {
			return redisMgr.Close()
		},
	}

	// DriverRegistry 只注册 rabbitmq。
	routerComp := bussvc.NewRouterComponent(app, transMgr, rabbitmq.NewRegistry())

	// Start 顺序：handlers 注册 → deps（Redis+恢复）→ TransMgr → router/bus。
	app.MustRegister(registerHandlers, businessDeps, &bussvc.TransMgrComponent{Mgr: transMgr}, routerComp)

	if err := app.Run(context.Background()); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func registerCmds() error {
	cmds := []struct {
		cmd g1_protocol.CMD
		fn  cmd_handler.CmdHandlerFunc
	}{
		{g1_protocol.CMD_TEXAS_INNER_CREATEROOM_REQ, onCreateRoom},
		{g1_protocol.CMD_TEXAS_INNER_QUICK_START_REQ, onQuickStart},
		{g1_protocol.CMD_TEXAS_INNER_JOINROOM_REQ, onJoinRoom},
		{g1_protocol.CMD_TEXAS_INNER_LEAVE_GAME_REQ, onLeaveGame},
	}
	for _, c := range cmds {
		if err := transMgr.RegisterCmdE(c.cmd, c.fn); err != nil {
			return fmt.Errorf("register %v: %w", c.cmd, err)
		}
	}
	logger.Infof("[gamesim] handlers registered (rejectQS=%v noAutoSeed=%v)",
		rejectQuickStart(), noAutoSeed())
	return nil
}

// ---- 模式开关 ----

func rejectQuickStart() bool { return os.Getenv("GAMESIM_REJECT_QUICKSTART") == "1" }
func noAutoSeed() bool       { return os.Getenv("GAMESIM_NO_AUTOSEED") == "1" }
func reportZone() uint32 {
	if v, err := strconv.Atoi(os.Getenv("GAMESIM_REPORT_ZONE")); err == nil && v >= 0 {
		return uint32(v)
	}
	return 0
}

// ---- 房间存储 ----

type gameRoom struct {
	base    *g1_protocol.RoomBaseInfo
	players map[uint64]bool // uid set
}

var store struct {
	mu    sync.Mutex
	rooms map[uint64]*gameRoom
}

func init() { store.rooms = make(map[uint64]*gameRoom) }

// redisMgr 房间持久化（复用 db_instances 实例，key 前缀独立，不与业务冲突）。
var redisMgr = redis.NewRedisMgr()

const (
	roomsStoreKey    = "GAMESIM_ROOMS"
	roomsStoreInstID = 1
)

// persistRooms 全量持久化房间表（低频操作，整表单 key 简化恢复）。
func persistRooms() {
	store.mu.Lock()
	blob := &g1_protocol.RoomListRsp{RoomList: make([]*g1_protocol.RoomShowInfo, 0, len(store.rooms))}
	for _, r := range store.rooms {
		base := proto.Clone(r.base).(*g1_protocol.RoomBaseInfo)
		base.CurPlayerNum = uint32(len(r.players))
		blob.RoomList = append(blob.RoomList, &g1_protocol.RoomShowInfo{Base: base})
	}
	store.mu.Unlock()

	data, err := proto.Marshal(blob)
	if err != nil {
		logger.Errorf("[gamesim] persist rooms marshal | %v", err)
		return
	}
	if err := redisMgr.SetBytes(context.Background(), roomsStoreInstID, roomsStoreKey, data, 0); err != nil {
		logger.Errorf("[gamesim] persist rooms | %v", err)
	}
}

// restoreRooms 启动时恢复房间表（人数归零重新计），并把权威状态上报目录收敛。
func restoreRooms(ctx context.Context) {
	data, err := redisMgr.GetBytes(ctx, roomsStoreInstID, roomsStoreKey)
	if err != nil {
		logger.Warningf("[gamesim] restore rooms read failed (start with empty) | %v", err)
		return
	}
	if len(data) == 0 {
		logger.Infof("[gamesim] no persisted rooms, start empty")
		return
	}
	blob := &g1_protocol.RoomListRsp{}
	if err := proto.Unmarshal(data, blob); err != nil {
		logger.Errorf("[gamesim] restore rooms unmarshal | %v", err)
		return
	}

	store.mu.Lock()
	for _, info := range blob.GetRoomList() {
		if info.GetBase() == nil {
			continue
		}
		base := proto.Clone(info.GetBase()).(*g1_protocol.RoomBaseInfo)
		base.CurPlayerNum = 0
		store.rooms[base.RoomId] = &gameRoom{base: base, players: make(map[uint64]bool)}
	}
	n := len(store.rooms)
	store.mu.Unlock()

	// 上报恢复后的房间（人数 0）使 roomcenter 目录收敛为权威值。
	if !noAutoSeed() {
		store.mu.Lock()
		bases := make([]*g1_protocol.RoomBaseInfo, 0, n)
		for _, r := range store.rooms {
			bases = append(bases, r.base)
		}
		store.mu.Unlock()
		for _, b := range bases {
			reportRoom(reportZone(), b, 0)
		}
	}
	logger.Infof("[gamesim] restored %d rooms from persistence", n)
}

// reportRoom 把权威房间状态上报给 roomcenter 目录（UpdateRoomInfo 整房替换）。
func reportRoom(zone uint32, base *g1_protocol.RoomBaseInfo, count int) {
	clone := proto.Clone(base).(*g1_protocol.RoomBaseInfo)
	clone.CurPlayerNum = uint32(count)
	idx := gfunc.GetTexasRoomListIndex(zone, clone.GameId, clone.CoinType)
	if err := roomcenterv1.NewRoomCenterInnerServiceClient().UpdateRoomInfoByRouterSimple(idx, 100000, zone, &g1_protocol.RoomShowInfo{Base: clone}); err != nil {
		logger.Errorf("[gamesim] report room %d failed | %v", clone.RoomId, err)
	}
}

// ---- 协议处理 ----

// onCreateRoom 建房（one-way，不回包）。房间进入本地存储；默认同时上报目录，
// 使后续玩家的 RoomList/QuickStart 能"选房"而非每次建房。
func onCreateRoom(c cmd_handler.IContext, data []byte) g1_protocol.ErrorCode {
	req := &g1_protocol.InnerCreateRoomReq{}
	if err := proto.Unmarshal(data, req); err != nil || req.GetBase() == nil {
		logger.Errorf("[gamesim] create room bad request")
		return g1_protocol.ErrorCode_ERR_ARGV
	}

	store.mu.Lock()
	base := proto.Clone(req.GetBase()).(*g1_protocol.RoomBaseInfo)
	base.CurPlayerNum = 0
	store.rooms[base.RoomId] = &gameRoom{base: base, players: make(map[uint64]bool)}
	store.mu.Unlock()
	persistRooms()

	logger.Infof("[gamesim] room created {id:%d, stage:%v, coin:%v, zone:%d}", base.RoomId, base.Stage, base.CoinType, base.Zone)

	if !noAutoSeed() {
		reportRoom(reportZone(), base, 0)
	}
	return g1_protocol.ErrorCode_ERR_OK
}

// onQuickStart 快速开始入座（Call-Response）。房间 ID 来自路由键 c.Rid()
//（mainsvr 按分配到的 roomId 路由）。拒绝模式用于驱动回滚路径测试（F03）。
func onQuickStart(c cmd_handler.IContext, data []byte) g1_protocol.ErrorCode {
	req := &g1_protocol.QuickStartReq{}
	if err := proto.Unmarshal(data, req); err != nil {
		return badRequest()
	}
	rsp := &g1_protocol.QuickStartRsp{Ret: &g1_protocol.Ret{}}

	if rejectQuickStart() {
		rsp.Ret.Code = g1_protocol.ErrorCode_ERR_FAIL
		rsp.Ret.Msg = "gamesim reject mode"
		c.SendMsgBack(rsp)
		return g1_protocol.ErrorCode_ERR_OK
	}

	roomID := c.Rid()
	store.mu.Lock()
	room, ok := store.rooms[roomID]
	if !ok {
		// 权威侧重建：目录残留（如模拟器早于持久化前的重启）时按请求属性重建房间，
		// 使目录与权威状态自愈收敛（真实游戏服具备同等的重建能力）。
		base := &g1_protocol.RoomBaseInfo{
			RoomId:   roomID,
			GameId:   req.GetGameId(),
			Stage:    req.GetStage(),
			CoinType: req.GetCoinType(),
			MaxPlayer: 9,
			EndTime:  9999999999,
		}
		room = &gameRoom{base: base, players: make(map[uint64]bool)}
		store.rooms[roomID] = room
		ok = true
		logger.Infof("[gamesim] room %d reconstructed from quickstart {stage:%v, coin:%v}", roomID, base.Stage, base.CoinType)
	}
	room.players[c.Uid()] = true
	rsp.RoomInfo = proto.Clone(room.base).(*g1_protocol.RoomBaseInfo)
	rsp.RoomInfo.CurPlayerNum = uint32(len(room.players))
	store.mu.Unlock()

	if !ok {
		rsp.Ret.Code = g1_protocol.ErrorCode_ERR_NOT_EXIST_GAME_ROOM
		rsp.Ret.Msg = "gamesim: room not found"
		c.SendMsgBack(rsp)
		return g1_protocol.ErrorCode_ERR_OK
	}

	reportRoom(c.Zone(), rsp.RoomInfo, lenInt(rsp.RoomInfo.CurPlayerNum))
	persistRooms()
	rsp.Ret.Code = g1_protocol.ErrorCode_ERR_OK
	c.SendMsgBack(rsp)
	return g1_protocol.ErrorCode_ERR_OK
}

// onJoinRoom 显式加入（Call-Response）。房间不存在返回 ERR_NOT_EXIST_GAME_ROOM
//（mainsvr 依赖该码清理玩家的对局状态）。
func onJoinRoom(c cmd_handler.IContext, data []byte) g1_protocol.ErrorCode {
	req := &g1_protocol.JoinRoomReq{}
	if err := proto.Unmarshal(data, req); err != nil {
		return badRequest()
	}
	rsp := &g1_protocol.JoinRoomRsp{Ret: &g1_protocol.Ret{}}

	store.mu.Lock()
	room, ok := store.rooms[req.GetRoomId()]
	if ok {
		room.players[c.Uid()] = true
		rsp.RoomInfo = proto.Clone(room.base).(*g1_protocol.RoomBaseInfo)
		rsp.RoomInfo.CurPlayerNum = uint32(len(room.players))
	}
	store.mu.Unlock()

	if !ok {
		rsp.Ret.Code = g1_protocol.ErrorCode_ERR_NOT_EXIST_GAME_ROOM
		rsp.Ret.Msg = "gamesim: room not found"
		c.SendMsgBack(rsp)
		return g1_protocol.ErrorCode_ERR_OK
	}

	reportRoom(c.Zone(), rsp.RoomInfo, lenInt(rsp.RoomInfo.CurPlayerNum))
	persistRooms()
	rsp.Ret.Code = g1_protocol.ErrorCode_ERR_OK
	c.SendMsgBack(rsp)
	return g1_protocol.ErrorCode_ERR_OK
}

// onLeaveGame 离开对局（Call-Response）。人走后房间保留（人数上报收敛为真实值）。
func onLeaveGame(c cmd_handler.IContext, data []byte) g1_protocol.ErrorCode {
	req := &g1_protocol.LeaveGameReq{}
	if err := proto.Unmarshal(data, req); err != nil {
		return badRequest()
	}
	rsp := &g1_protocol.LeaveGameRsp{Ret: &g1_protocol.Ret{}}

	store.mu.Lock()
	if room, ok := store.rooms[req.GetRoomId()]; ok {
		delete(room.players, c.Uid())
		reportBase := proto.Clone(room.base).(*g1_protocol.RoomBaseInfo)
		reportBase.CurPlayerNum = uint32(len(room.players))
		reportRoom(c.Zone(), reportBase, len(room.players))
	}
	store.mu.Unlock()
	persistRooms()

	rsp.Ret.Code = g1_protocol.ErrorCode_ERR_OK
	c.SendMsgBack(rsp)
	return g1_protocol.ErrorCode_ERR_OK
}

func badRequest() g1_protocol.ErrorCode {
	return g1_protocol.ErrorCode_ERR_ARGV
}

func lenInt(v uint32) int { return int(v) }
