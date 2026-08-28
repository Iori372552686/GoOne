package room_mgr

import (
	"sync"

	roomcenterv1 "github.com/Iori372552686/GoOne/api/gen/game/roomcenter/v1"
	"github.com/Iori372552686/GoOne/lib/api/datetime"
	"github.com/Iori372552686/GoOne/lib/service/bus"
	"github.com/Iori372552686/GoOne/module/conf"
	"github.com/Iori372552686/GoOne/src/roomcentersvr/room_mgr/texas_room"
	pb "github.com/Iori372552686/g1_common/protocol"
)

// RoomMgr 房间中心顶层管理器：按路由分片（zone）索引各玩法的房间管理器。
//
// 并发约定：
//   - TexasMgr（zone -> TexasRoomCenterMgr）的读写由 impl.RWMutex 保护；
//     zone 由首个请求经 GetRoomMgrObj 懒创建（写锁下插入）。
//   - 遍历 TexasMgr（Tick/持久化/Flush）必须先在 RLock 下做快照再处理，
//     杜绝"遍历中写"导致的 concurrent map iteration/write fatal。
type RoomMgr struct {

	//public
	TexasMgr map[uint64]*texas_room.TexasRoomCenterMgr //德州游戏房间管理
	// more ... game room mgr（未来新增玩法在此扩展）

	//private
	isOpen    bool
	lastTick  int64 // 上次巡检分发时间戳（ms）
	eventTick int64 // 上次持久化时间戳（ms）
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

// snapshotZonesLocked 在读锁下快照当前全部 zone（持锁时间仅做指针拷贝）。
// 后续处理（RPC 发送/落盘）都在锁外进行，避免慢 IO 阻塞 zone 懒创建。
func (impl *RoomMgr) snapshotZonesLocked() []*texas_room.TexasRoomCenterMgr {
	impl.RLock()
	zones := make([]*texas_room.TexasRoomCenterMgr, 0, len(impl.TexasMgr))
	for _, zone := range impl.TexasMgr {
		zones = append(zones, zone)
	}
	impl.RUnlock()
	return zones
}

// Tick 巡检分发（5s 周期驱动）：向每个 zone 自身发 one-way InnerTickReq，
// 经 TransMgr 按 zone 串行键执行该 zone 的巡检（过期清理 + 满员补房），
// 保证同 zone 巡检与业务请求串行。
func (impl *RoomMgr) Tick(nowMs int64) {
	if !impl.checkOpen() {
		return
	}

	if (nowMs - impl.lastTick) > 5*datetime.MS_PER_SECOND {
		impl.lastTick = nowMs

		// 快照后处理：GetRoomMgrObj 会在写锁下新增 zone（懒创建），
		// 裸遍历 map 与之并发是 data race（历史缺陷，修复）。
		zones := impl.snapshotZonesLocked()
		for _, zone := range zones {
			if zone == nil {
				continue
			}

			// 内部转发，使用 IDL 生成的一次性 one-way helper。
			roomcenterv1.NewRoomCenterInnerServiceClient().TickByRouterSimple(
				zone.Index,
				0,
				0,
				&pb.InnerTickReq{NowMs: nowMs, SrcBusId: bus.IpStringToInt(conf.Get("roomcentersvr.identity.self_bus_id").String())},
			)
		}
	}
}

// persistIntervalMs 持久化节流间隔（毫秒）。房间快照变更后最多 lag 这么久落盘。
const persistIntervalMs = 10 * datetime.MS_PER_SECOND

// TickPersist 周期持久化变更的房间快照，与 Tick 解耦独立节拍。
// 无 Redis 配置时 SaveDirtyToDB 内部跳过。
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

// FlushAllToDB 强制全量写所有 zone 的房间快照（停机 Drain 用）。
// 必须在 TransMgr 排空之后调用（组件注册顺序保证），此时已无 handler 并发修改。
func (impl *RoomMgr) FlushAllToDB() (totalSaved, totalFailed int) {
	for _, zone := range impl.snapshotZonesLocked() {
		s, f := zone.FlushAllRoomsToDB()
		totalSaved += s
		totalFailed += f
	}
	return totalSaved, totalFailed
}

// SaveDirtyToDB 周期持久化所有 zone 中变更的房间（TickPersist 调用）。
func (impl *RoomMgr) SaveDirtyToDB() {
	for _, zone := range impl.snapshotZonesLocked() {
		_ = zone.SaveRoomDataToDB()
	}
}

func (impl *RoomMgr) checkOpen() bool {
	return impl.isOpen
}

// ----------------------------------------------public----------------------------------------------
