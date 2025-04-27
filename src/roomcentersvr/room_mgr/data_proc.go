package room_mgr

import (
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
			impl.TexasMgr[index] = data
		}

		impl.Unlock()
	} else {
		impl.RUnlock()
	}

	return data
}
