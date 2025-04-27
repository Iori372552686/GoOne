package texas_room

import (
	"github.com/Iori372552686/GoOne/src/roomcentersvr/room_mgr/texas_room/texas"
	g1_protocol "github.com/Iori372552686/game_protocol/protocol"
)

func (impl *TexasRoomCenterMgr) saveRoomDataToDB() error {
	for _, roomInfo := range impl.TexasMap {
		if roomInfo.CheckChange() {
			/*			impl.InnerInsGuildTexasGameRoomDataSave(ctx, &pb.InsGuildTexasGameRoomDataSaveReq{
							DataType: pb.TexasGameRoom_DATA_TYPE_game_data,
							roomId:  GameData.roomId, SrvKey: frpc.GetLocalAddr(),
						}, "", GameData.roomId)
			*/
			//logger.Debugf("saveGameData gid:%v", GameData.Base.RoomId)
		}
	}

	return nil
}

// ----------------------------------------------public----------------------------------------------
func (impl *TexasRoomCenterMgr) GetTexasObj(stage int32) *texas.TexasRoom {
	var data *texas.TexasRoom
	var has bool

	impl.RLock()
	if data, has = impl.TexasMap[stage]; !has {
		impl.RUnlock()
		impl.Lock()

		//map  double-check
		if data, has = impl.TexasMap[stage]; !has {
			data = texas.NewTexasRoomObj(impl.Index, stage)
			impl.TexasMap[stage] = data
		}

		impl.Unlock()
	} else {
		impl.RUnlock()
	}

	return data
}

func (impl *TexasRoomCenterMgr) SetTexasRoom(data *g1_protocol.DBTexasRoomCenterInfo) error {
	if data == nil {
		return nil
	}
	return impl.GetTexasObj(int32(data.Stage)).Set(data)
}

func (impl *TexasRoomCenterMgr) OnCleanData(Stage int32) error {
	if !impl.checkOpen() || Stage == 0 {
		return nil
	}

	impl.Lock()
	delete(impl.TexasMap, Stage)
	impl.Unlock()
	return nil
}
