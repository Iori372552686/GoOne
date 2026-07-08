package texas_room

import (
	"sync"

	"github.com/Iori372552686/GoOne/src/roomcentersvr/room_ai"
	"github.com/Iori372552686/GoOne/src/roomcentersvr/room_mgr/texas_room/texas"
	g1_protocol "github.com/Iori372552686/game_protocol/protocol"

	"github.com/Iori372552686/GoOne/lib/api/logger"
)

type TexasRoomCenterMgr struct {
	Index    uint64
	TexasMap map[int32]*texas.TexasRoom // map[stage] * roominfo
	//more ... game

	//private
	sync.RWMutex
	isOpen   bool
	lastTick int64 // tick
	syncTick int64 // tick
	errCnt   int32 // error count

}

func NewTexasRoomCenterMgr(index uint64) *TexasRoomCenterMgr {
	ins := &TexasRoomCenterMgr{
		Index:    index,
		TexasMap: make(map[int32]*texas.TexasRoom),
	}

	ins.init()
	return ins
}

func (impl *TexasRoomCenterMgr) checkOpen() bool {
	return impl.isOpen
}

// ----------------------------------------------public----------------------------------------------

func (impl *TexasRoomCenterMgr) init() error {
	impl.isOpen = true
	return nil
}

func (impl *TexasRoomCenterMgr) IncrErrCnt() {
	impl.errCnt++
}

func (impl *TexasRoomCenterMgr) GetErrCnt() int32 {
	return impl.errCnt
}

func (impl *TexasRoomCenterMgr) Tick(nowMs int64) {
	if !impl.checkOpen() {
		return
	}

	impl.checkAndCreate(nowMs)
}

func (impl *TexasRoomCenterMgr) Exit() {
	impl.saveRoomDataToDB()
	logger.Debugf("room center exit Done!")
}

func (impl *TexasRoomCenterMgr) checkAndSync(nowMs int64) {
	if !impl.checkOpen() {
		return
	}

	for _, rstage := range impl.TexasMap {
		if rstage == nil || rstage.RoomsMap == nil {
			continue
		}

		//check creat room
		for _, room := range rstage.RoomsMap {
			if room.Base.CurPlayerNum >= room.Base.MaxPlayer {
				room_ai.OnAiCreatRoom(room.Base.GameId, int32(room.Base.Stage), int32(room.Base.CoinType))
			}
		}

	}

}

func (impl *TexasRoomCenterMgr) checkAndCreate(nowMs int64) {
	if !impl.checkOpen() {
		return
	}

	for _, rstage := range impl.TexasMap {
		if rstage == nil || len(rstage.RoomsMap) == 0 {
			continue
		}

		fullCnt := 0
		var sample *g1_protocol.RoomShowInfo
		for _, room := range rstage.RoomsMap {
			sample = room
			if room.Base.CurPlayerNum >= room.Base.MaxPlayer {
				fullCnt += 1
			}
		}

		// 该 stage 下所有房间都满员时补一间新房，保证玩家总有可加入的房间。
		if fullCnt == len(rstage.RoomsMap) && sample != nil {
			room_ai.OnAiCreatRoom(sample.Base.GameId, int32(sample.Base.Stage), int32(sample.Base.CoinType))
		}
	}
}
