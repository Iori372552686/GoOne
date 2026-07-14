package room_mgr

import (
	roomcenterv1 "github.com/Iori372552686/GoOne/api/gen/game/roomcenter/v1"
	"github.com/Iori372552686/GoOne/common/gconf"
	"github.com/Iori372552686/GoOne/lib/api/datetime"
	"github.com/Iori372552686/GoOne/lib/service/bus"
	"github.com/Iori372552686/GoOne/src/roomcentersvr/room_mgr/texas_room"
	pb "github.com/Iori372552686/game_protocol/protocol"
	"sync"
)

type RoomMgr struct {

	//public
	TexasMgr map[uint64]*texas_room.TexasRoomCenterMgr //德州游戏房间管理
	// more ... game room mgr

	//private
	isOpen    bool
	lastTick  int64
	eventTick int64
	sync.RWMutex
}

func NewRoomMgr() *RoomMgr {
	impl := &RoomMgr{}
	impl.TexasMgr = make(map[uint64]*texas_room.TexasRoomCenterMgr)
	return impl
}

func (impl *RoomMgr) Init() error {
	impl.isOpen = true
	return nil
}

func (impl *RoomMgr) Tick(nowMs int64) {
	if !impl.checkOpen() {
		return
	}

	if (nowMs - impl.lastTick) > 5*datetime.MS_PER_SECOND {
		impl.lastTick = nowMs

		for _, zone := range impl.TexasMgr {
			if zone.TexasMap == nil {
				continue
			}

			// 内部转发，使用 IDL 生成的一次性 one-way helper。
			roomcenterv1.NewRoomCenterInnerServiceClient().TickByRouterSimple(
				zone.Index,
				0,
				0,
				&pb.InnerTickReq{NowMs: nowMs, SrcBusId: bus.IpStringToInt(gconf.RoomCenterSvrCfg.Identity.SelfBusId)},
			)
		}
	}
}

// persistTickSec 持久化节流间隔（秒）。房间快照变更后最多 lag 这么久落盘。
const persistIntervalMs = 10 * datetime.MS_PER_SECOND

// TickPersist 周期持久化变更的房间快照，与 Tick 解耦独立节拍。
// 由 app.OnTick 驱动；无 Redis 配置时 SaveDirtyToDB 内部跳过。
func (impl *RoomMgr) TickPersist(nowMs int64) {
	if !impl.checkOpen() {
		return
	}
	if nowMs-impl.eventTick < persistIntervalMs {
		return
	}
	impl.eventTick = nowMs
	impl.SaveDirtyToDB()
}

// onExit, save data
func (impl *RoomMgr) Exit() {
	if !impl.checkOpen() {
		return
	}

	for _, zone := range impl.TexasMgr {
		zone.Exit()
	}
}

// LoadAllFromDB 启动时从 Redis 恢复所有 zone 的房间快照。
func (impl *RoomMgr) LoadAllFromDB() {
	impl.RLock()
	zones := make([]*texas_room.TexasRoomCenterMgr, 0, len(impl.TexasMgr))
	for _, zone := range impl.TexasMgr {
		zones = append(zones, zone)
	}
	impl.RUnlock()

	for _, zone := range zones {
		if err := zone.LoadRoomDataFromDB(); err != nil {
			// room_mgr 日志在 zone 层打印，这里仅避免中断其它 zone
			continue
		}
	}
}

// FlushAllToDB 强制全量写所有 zone 的房间快照（停机用）。
func (impl *RoomMgr) FlushAllToDB() (totalSaved, totalFailed int) {
	impl.RLock()
	zones := make([]*texas_room.TexasRoomCenterMgr, 0, len(impl.TexasMgr))
	for _, zone := range impl.TexasMgr {
		zones = append(zones, zone)
	}
	impl.RUnlock()

	for _, zone := range zones {
		s, f := zone.FlushAllRoomsToDB()
		totalSaved += s
		totalFailed += f
	}
	return totalSaved, totalFailed
}

// SaveDirtyToDB 周期持久化所有 zone 中变更的房间（OnTick 调用）。
func (impl *RoomMgr) SaveDirtyToDB() {
	impl.RLock()
	zones := make([]*texas_room.TexasRoomCenterMgr, 0, len(impl.TexasMgr))
	for _, zone := range impl.TexasMgr {
		zones = append(zones, zone)
	}
	impl.RUnlock()

	for _, zone := range zones {
		_ = zone.SaveRoomDataToDB()
	}
}

func (impl *RoomMgr) checkOpen() bool {
	return impl.isOpen
}

// ----------------------------------------------public----------------------------------------------
