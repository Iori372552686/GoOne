package room_ai

import (
	"fmt"
	"github.com/Iori372552686/GoOne/common/gamedata/repository/texas_config"
	"github.com/Iori372552686/GoOne/module/misc"
	id "github.com/Iori372552686/GoOne/src/roomcentersvr/globals/idgen"
	"time"

	"github.com/Iori372552686/GoOne/common/gfunc"
	"github.com/Iori372552686/GoOne/lib/api/datetime"
	"github.com/Iori372552686/GoOne/lib/service/router"
	pb "github.com/Iori372552686/game_protocol/protocol"
)

// OnAiInitRoom checks if the AI can create rooms for all game types
func OnAiInitRoom() {
	gameconfs := texas_config.GetAll()

	time.Sleep(2 * time.Second)
	if gameconfs != nil {
		for _, conf := range gameconfs {
			OnAiCreatRoom(pb.GameTypeId(1), conf.RoomStage, conf.CoinType)
			time.Sleep(10 * time.Millisecond)
		}
	}
}

func OnAiCreatRoom(gameId pb.GameTypeId, stage, coinType int32) (*pb.RoomBaseInfo, error) {
	sysUid := uint64(100000)
	conf := texas_config.GetByRoomStageCoinType(stage, coinType)
	if conf == nil {
		return nil, fmt.Errorf("room config not found for stage: %d, coinType: %d", stage, coinType)
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
			Name:       gameId.String(),
			GameId:     gameId,
			Blind:      fmt.Sprintf("%d/%d", conf.SmallBlind, conf.BigBlind),
			MinBuyIn:   conf.MinBuyIn,
			MaxBuyIn:   conf.MaxBuyIn,
			MaxPlayer:  conf.MaxPlayerCount,
			MaxMember:  conf.MaxRoomCount,
			CreateTime: datetime.NowInt64(),
			StartTime:  datetime.NowInt64(),
			EndTime:    datetime.NowInt64() + conf.RoomKeepLive*60,
			CoinType:   pb.CoinType(coinType),
			Stage:      pb.RoomStage(stage),
		}}

	return rpcReq.Base, router.SendPbMsgByRouter(misc.ServerType_TexasGameSvr, rpcReq.Base.RoomId, sysUid, 1, pb.CMD_TEXAS_INNER_CREATEROOM_REQ, rpcReq)
}
