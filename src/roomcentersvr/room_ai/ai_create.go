package room_ai

import (
	"fmt"
	"time"

	"github.com/Iori372552686/GoOne/lib/api/datetime"
	"github.com/Iori372552686/GoOne/lib/service/router"
	"github.com/Iori372552686/GoOne/module/gamedata/repository/texas"
	"github.com/Iori372552686/GoOne/module/gfunc"
	"github.com/Iori372552686/GoOne/module/misc"
	id "github.com/Iori372552686/GoOne/src/roomcentersvr/globals/idgen"
	"github.com/Iori372552686/GoOne/src/roomcentersvr/room_mgr/texas_room"
	pb "github.com/Iori372552686/g1_common/protocol"
)

// OnAiInitRoom checks if the AI can create rooms for all game types
func OnAiInitRoom() {
	gameconfs := texas.GetTexasAll()

	time.Sleep(2 * time.Second)
	if gameconfs != nil {
		for _, conf := range gameconfs {
			OnAiCreateRoom(texas_room.RoomSpec{
				GameID:   pb.GameTypeId(1),
				Stage:    pb.RoomStage(conf.RoomStage),
				CoinType: pb.CoinType(conf.CoinType),
			})
			time.Sleep(10 * time.Millisecond)
		}
	}
}

// OnAiCreateRoom 按具名 RoomSpec 建房（向游戏服发送创建请求，one-way）。
func OnAiCreateRoom(spec texas_room.RoomSpec) (*pb.RoomBaseInfo, error) {
	sysUid := uint64(100000)
	conf := texas.GetTexasByRoomStageCoinType(int32(spec.Stage), int32(spec.CoinType))
	if conf == nil {
		return nil, fmt.Errorf("room config not found for stage: %d, coinType: %d", spec.Stage, spec.CoinType)
	}

	genId, err := id.IDGen.GenID()
	if err != nil {
		return nil, err
	}

	rpcReq := &pb.InnerCreateRoomReq{
		Base: &pb.RoomBaseInfo{
			Id:         genId,
			Zone:       1,
			RoomId:     gfunc.GenerateRoomId(genId),
			OwerId:     sysUid,
			Name:       spec.GameID.String(),
			GameId:     spec.GameID,
			Blind:      fmt.Sprintf("%d/%d", conf.SmallBlind, conf.BigBlind),
			MinBuyIn:   conf.MinBuyIn,
			MaxBuyIn:   conf.MaxBuyIn,
			MaxPlayer:  conf.MaxPlayerCount,
			MaxMember:  conf.MaxRoomCount,
			CreateTime: datetime.NowInt64(),
			StartTime:  datetime.NowInt64(),
			EndTime:    datetime.NowInt64() + conf.RoomKeepLive*60,
			CoinType:   spec.CoinType,
			Stage:      spec.Stage,
		}}

	return rpcReq.Base, router.SendPbMsgByRouter(misc.ServerType_TexasGameSvr, rpcReq.Base.RoomId, sysUid, 1, pb.CMD_TEXAS_INNER_CREATEROOM_REQ, rpcReq)
}
