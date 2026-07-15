package room

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/Iori372552686/GoOne/tools/tester/app/component"
	"github.com/Iori372552686/GoOne/tools/tester/internal/session"
	"github.com/Iori372552686/GoOne/tools/tester/internal/testcfg"
	g1_protocol "github.com/Iori372552686/game_protocol/protocol"
)

func init() {
	component.Register("room", func() component.TesterComponent {
		return &RoomComponent{}
	})
}

// RoomComponent 房间相关回归/压测组件。
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

func (r *RoomComponent) RunTests(ctx context.Context) error {
	log.Printf("[Actor %d][Room] ===== Starting room tests =====", r.actorID)

	tests := []struct {
		name string
		fn   func(ctx context.Context) error
	}{
		{"T01_RoomList", r.testRoomList},
		{"T02_QuickStart", r.testQuickStart},
		{"T03_LeaveRoom", r.testLeaveRoom},
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

func (r *RoomComponent) RunStress(ctx context.Context) error {
	// 压测循环：快速开始 → 离开房间，反复执行。
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

func (r *RoomComponent) testRoomList(ctx context.Context) error {
	req := &g1_protocol.RoomListReq{
		GameId:    g1_protocol.GameTypeId_TEXAS_NORMAL,
		Stage:     g1_protocol.RoomStage_Free,
		RoomIds:   make([]uint32, 0),
		PageIndex: 0,
		PageSize:  10,
		SortType:  g1_protocol.RoomSortType_SORT_TYPE_ID,
		CoinType:  g1_protocol.CoinType_COIN_NONE,
	}
	resp := &g1_protocol.RoomListRsp{Ret: &g1_protocol.Ret{}}
	if err := r.requester.RequestProto(ctx, uint32(g1_protocol.CMD_MAIN_GAME_ROOM_LIST_REQ), req, resp, 15*time.Second); err != nil {
		return err
	}
	if resp.Ret != nil && session.IsErrCode(int32(resp.Ret.Code)) {
		return fmt.Errorf("room list failed: code=%d msg=%s", resp.Ret.Code, resp.Ret.Msg)
	}
	log.Printf("[Actor %d][Room] T01: Room list OK, count=%d", r.actorID, len(resp.RoomList))
	return nil
}

func (r *RoomComponent) testQuickStart(ctx context.Context) error {
	if err := r.quickStart(ctx); err != nil {
		return err
	}
	log.Printf("[Actor %d][Room] T02: Quick start OK, roomID=%d", r.actorID, r.roomID)
	return nil
}

func (r *RoomComponent) quickStart(ctx context.Context) error {
	req := &g1_protocol.QuickStartReq{
		GameId:    g1_protocol.GameTypeId_TEXAS_NORMAL,
		CoinType:  g1_protocol.CoinType_COIN_NONE,
		Stage:     g1_protocol.RoomStage_Free,
		ConnBusId: 0,
	}
	resp := &g1_protocol.QuickStartRsp{Ret: &g1_protocol.Ret{}}
	if err := r.requester.RequestProto(ctx, uint32(g1_protocol.CMD_MAIN_GAME_QUICK_START_REQ), req, resp, 30*time.Second); err != nil {
		return err
	}
	if resp.Ret != nil && session.IsErrCode(int32(resp.Ret.Code)) {
		return fmt.Errorf("quick start failed: code=%d msg=%s", resp.Ret.Code, resp.Ret.Msg)
	}
	if resp.RoomInfo != nil && resp.RoomInfo.RoomId != 0 {
		r.roomID = resp.RoomInfo.RoomId
	}
	return nil
}

func (r *RoomComponent) testLeaveRoom(ctx context.Context) error {
	if err := r.leaveRoom(ctx); err != nil {
		return err
	}
	log.Printf("[Actor %d][Room] T03: Leave room OK", r.actorID)
	return nil
}

func (r *RoomComponent) leaveRoom(ctx context.Context) error {
	req := &g1_protocol.LeaveGameReq{RoomId: r.roomID}
	resp := &g1_protocol.LeaveGameRsp{Ret: &g1_protocol.Ret{}}
	if err := r.requester.RequestProto(ctx, uint32(g1_protocol.CMD_MAIN_GAME_LEAVE_GAME_REQ), req, resp, 15*time.Second); err != nil {
		return err
	}
	if resp.Ret != nil && session.IsErrCode(int32(resp.Ret.Code)) {
		return fmt.Errorf("leave room failed: code=%d msg=%s", resp.Ret.Code, resp.Ret.Msg)
	}
	r.roomID = 0
	return nil
}
