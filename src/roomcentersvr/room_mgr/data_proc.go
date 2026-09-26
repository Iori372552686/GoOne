package room_mgr

import (
	"github.com/Iori372552686/GoOne/src/roomcentersvr/room_ai"
	"github.com/Iori372552686/GoOne/src/roomcentersvr/room_mgr/texas_room"
)

// ----------------------------------------------public----------------------------------------------
func (impl *RoomMgr) GetRoomMgrObj(index uint64) *texas_room.TexasRoomCenterMgr {
	var data *texas_room.TexasRoomCenterMgr
	var has bool

	impl.RLock()
	if data, has = impl.TexasMgr[index]; !has {
		impl.RUnlock()
		impl.Lock()

		//map  double-check
		if data, has = impl.TexasMgr[index]; !has {
			data = texas_room.NewTexasRoomCenterMgr(index)
			// 默认建房工厂在此注入（texas_room 不反向依赖 room_ai，避免导入环）。
			data.SetCreateRoomFn(room_ai.OnAiCreateRoom)
			impl.TexasMgr[index] = data
		}

		impl.Unlock()
	} else {
		impl.RUnlock()
	}

	return data
}
