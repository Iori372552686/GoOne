package texas_room

import (
	"sync"

	"github.com/Iori372552686/GoOne/lib/api/datetime"
	"github.com/Iori372552686/GoOne/lib/api/logger"
	"github.com/Iori372552686/GoOne/src/roomcentersvr/globals/idgen"
	"github.com/Iori372552686/GoOne/src/roomcentersvr/room_mgr/texas_room/texas"
	g1_protocol "github.com/Iori372552686/g1_common/protocol"
)

// RoomSpec 建房条件（报告 §7.3）：具名字段 + 各自的枚举类型，消除同为 int32
// 的位置参数歧义（F02 即此类缺陷：stage/coinType 互换编译期无法发现）。
type RoomSpec struct {
	GameID   g1_protocol.GameTypeId
	Stage    g1_protocol.RoomStage
	CoinType g1_protocol.CoinType
}

// CreateRoomFunc 建房工厂签名：生成新房间 Base 信息并向游戏服发送创建请求（one-way）。
// 以可注入字段暴露（默认 room_ai.OnAiCreateRoom），便于单元测试替换与未来多玩法扩展。
type CreateRoomFunc func(spec RoomSpec) (*g1_protocol.RoomBaseInfo, error)

// reservation 一次座位占位的预约票据（F03）：回滚按票据至多执行一次。
type reservation struct {
	roomID    uint64
	stage     int32
	createdAt int64 // 秒
	expireAt  int64 // 秒；过期后票据不可再取消（加入结果未知的占位由 gamesvr 上报收敛）
}

// reservationExpireSec 预约票据有效期：覆盖 mainsvr 3 次重试 × 5s RPC 超时 + 余量。
const reservationExpireSec = 60

// TexasRoomCenterMgr 单个路由分片（zone）的房间中心管理器。
//
// 并发约定（重要，违反即数据竞争/进程 fatal）：
//   - impl.RWMutex 同时保护 TexasMap（stage -> TexasRoom）与各 TexasRoom.RoomsMap
//     （roomId -> RoomShowInfo）的结构和内容；任何对二者的读写都必须持锁。
//   - 对外只暴露本文件与 data_proc.go 中的封装方法（UpdateRoomInfo/DelRoomInfo/
//     QuickStart/QuickStartRollback/RoomListPage/CheckAndCreateRooms/...），
//     调用方不得直接触碰裸 map —— 把"数据 + 行为 + 锁"绑定在同一实体内
//     （参考 due 框架 Actor 的封装方式：实体状态只在实体方法内访问）。
//   - 同 zone 的总线请求（QuickStart/RoomList 等）已被 TransMgr 按 routerID 串行键
//     串行化，但 gamesvr 房间上报、scheduler 巡检/持久化走不同串行键甚至独立协程，
//     因此锁是正确性的唯一保证，不能依赖串行键。
type TexasRoomCenterMgr struct {
	Index    uint64                                 // 路由分片索引（zone 级）
	TexasMap map[int32]*texas.TexasRoom             // map[stage] 房间表
	// more ... game（未来新增玩法在此扩展对应的 RoomMgr 与工厂）

	//private
	sync.RWMutex
	isOpen       bool
	lastTick     int64        // 上次 tick 时间戳（ms）
	createRoomFn CreateRoomFunc // 建房工厂（可注入，测试/多玩法用）
	// reservations 是座位占位预约表（F03）：reservationID -> 票据。由 zone 锁
	// 保护（与 TexasMap/RoomsMap 同一把）。回滚按票据至多执行一次。
	reservations map[uint64]*reservation
	// resvSeq 进程内预约号兜底序列（IDGen 未注入时保证 zone 内唯一）。
	resvSeq uint64
	// resvIDFn 可注入的票据生成器（测试用）；nil 时走默认实现。
	resvIDFn func() uint64
}

// NewTexasRoomCenterMgr 创建 zone 级房间中心管理器。
// 默认建房工厂由装配层（room_mgr.GetRoomMgrObj）注入 room_ai.OnAiCreateRoom，
// 避免本包反向依赖 room_ai 形成导入环；未注入时建房分支返回错误（见 QuickStart）。
func NewTexasRoomCenterMgr(index uint64) *TexasRoomCenterMgr {
	ins := &TexasRoomCenterMgr{
		Index:        index,
		TexasMap:     make(map[int32]*texas.TexasRoom),
		reservations: make(map[uint64]*reservation),
	}
	ins.init()
	return ins
}

// SetCreateRoomFn 替换建房工厂（仅测试/启动装配期调用，不保证运行期并发安全）。
func (impl *TexasRoomCenterMgr) SetCreateRoomFn(fn CreateRoomFunc) {
	impl.createRoomFn = fn
}

// SetReservationIDFn 替换预约票据生成器（仅测试注入用）。
func (impl *TexasRoomCenterMgr) SetReservationIDFn(fn func() uint64) {
	impl.resvIDFn = fn
}

// nextReservationID 生成预约票据：优先全局 IDGen（雪花，跨进程唯一）；
// 未初始化（测试/未启动完成）时退化为 时间戳<<20 | 进程内序列，保证 zone 内唯一。
// 调用方必须已持锁。
func (impl *TexasRoomCenterMgr) nextReservationID() uint64 {
	if impl.resvIDFn != nil {
		return impl.resvIDFn()
	}
	if g := id.IDGen; g != nil {
		if rid, err := g.GenID(); err == nil {
			return rid
		}
		logger.Warningf("IDGen.GenID failed, fallback to local sequence {index:%d}", impl.Index)
	}
	impl.resvSeq++
	return uint64(datetime.NowMs())<<20 | (impl.resvSeq & 0xFFFFF)
}

// issueReservation 记录一张占位票据并返回票据 ID。调用方必须已持写锁。
func (impl *TexasRoomCenterMgr) issueReservation(roomID uint64, stage int32, nowSec int64) uint64 {
	rid := impl.nextReservationID()
	impl.reservations[rid] = &reservation{
		roomID:    roomID,
		stage:     stage,
		createdAt: nowSec,
		expireAt:  nowSec + reservationExpireSec,
	}
	return rid
}

// sweepExpiredReservations 清理过期票据（Tick 驱动）。过期 ≠ 释放座位：加入结果
// 未知的占位由 gamesvr 周期上报的真实人数收敛（报告 §7.5 的回收策略）。
// 调用方必须已持写锁。
func (impl *TexasRoomCenterMgr) sweepExpiredReservations(nowSec int64) {
	for rid, resv := range impl.reservations {
		if nowSec > resv.expireAt {
			delete(impl.reservations, rid)
		}
	}
}

func (impl *TexasRoomCenterMgr) checkOpen() bool {
	return impl.isOpen
}

// ----------------------------------------------public----------------------------------------------

func (impl *TexasRoomCenterMgr) init() error {
	impl.isOpen = true
	return nil
}

// Tick 巡检入口（由 InnerTickReq 经 TransMgr 按 zone 串行键驱动）。
// 职责收敛到 CheckAndCreateRooms：过期房间清理 + 满员补房。
func (impl *TexasRoomCenterMgr) Tick(nowMs int64) {
	if !impl.checkOpen() {
		return
	}
	impl.CheckAndCreateRooms(nowMs)
}

// UpdateRoomInfo 更新/插入一间房间的展示信息（gamesvr 周期上报入口，one-way）。
// 整房替换语义：以 gamesvr 上报为权威数据源，覆盖本地记录（含真实 CurPlayerNum）。
func (impl *TexasRoomCenterMgr) UpdateRoomInfo(req *g1_protocol.RoomShowInfo) g1_protocol.ErrorCode {
	if req == nil || req.Base == nil {
		return g1_protocol.ErrorCode_ERR_ARGV
	}

	// GetTexasObj 自带锁与懒创建（新建 stage 时会先恢复 Redis 快照，见 data_proc.go）。
	room := impl.GetTexasObj(int32(req.Base.Stage))

	impl.Lock()
	defer impl.Unlock()

	if room.RoomsMap == nil {
		room.RoomsMap = make(map[uint64]*g1_protocol.RoomShowInfo)
	}
	room.RoomsMap[req.Base.RoomId] = req
	room.MarkDirty()
	return g1_protocol.ErrorCode_ERR_OK
}

// DelRoomInfo 删除一间房间的登记（gamesvr 上报房间解散，one-way）。
// 带 req.Base nil 防御（历史缺陷：缺 Base 时直接解引用 panic）。
func (impl *TexasRoomCenterMgr) DelRoomInfo(req *g1_protocol.RoomShowInfo) g1_protocol.ErrorCode {
	if req == nil || req.Base == nil {
		return g1_protocol.ErrorCode_ERR_ARGV
	}

	room := impl.GetTexasObj(int32(req.Base.Stage))

	impl.Lock()
	defer impl.Unlock()

	if room.RoomsMap == nil {
		return g1_protocol.ErrorCode_ERR_OK
	}
	if _, ok := room.RoomsMap[req.Base.RoomId]; !ok {
		return g1_protocol.ErrorCode_ERR_OK // 幂等：重复删除直接成功
	}
	delete(room.RoomsMap, req.Base.RoomId)
	room.MarkDirty()
	return g1_protocol.ErrorCode_ERR_OK
}

// QuickStart 快速开始：为玩家选定一间未满员房间并占位；全部满员则建房并乐观登记。
//
// 语义说明：
//   - 选房条件是"未满员"（CurPlayerNum < MaxPlayer）。
//     修复历史缺陷：原实现方向写反（MaxPlayer < CurPlayerNum），导致只有"已超员"
//     的房间会被选中、有空位的房间永远不命中，快速开始退化为"每次都建新房"。
//   - 选房规则：RoomId 最小的未满员房。map 遍历顺序随机，确定性选房让行为可复现、
//     日志可对账、测试可断言。
//   - 占位语义：分配即 CurPlayerNum++，并签发唯一 reservationID 一并返回；
//     mainsvr 加入对局失败时凭票据回调 QuickStartRollback，同一票据至多释放一次
//     （F03 幂等）。gamesvr 的周期上报最终收敛为真实人数。
//   - 建房分支采用两阶段：锁内只做选房判定，建房（含 ID 生成与总线发布）在锁外执行，
//     避免总线抖动时长时间持有 zone 锁拖垮房间列表与巡检；随后重新加锁做乐观登记。
func (impl *TexasRoomCenterMgr) QuickStart(req *g1_protocol.QuickStartReq) *g1_protocol.QuickStartRsp {
	rsp := &g1_protocol.QuickStartRsp{Ret: &g1_protocol.Ret{Code: g1_protocol.ErrorCode_ERR_OK}}

	// Stage_ALL 是列表查询用的聚合值，不指代具体场次，直接拒绝。
	if req == nil || req.Stage == g1_protocol.RoomStage_Stage_ALL {
		rsp.Ret.Code = g1_protocol.ErrorCode_ERR_ARGV
		return rsp
	}

	room := impl.GetTexasObj(int32(req.Stage))
	nowSec := datetime.NowMs() / datetime.MS_PER_SECOND

	// 第一阶段：锁内确定性选房 + 占位。
	var picked *g1_protocol.RoomShowInfo
	impl.Lock()
	for _, r := range room.RoomsMap {
		if r == nil || r.Base == nil {
			continue
		}
		if r.Base.CurPlayerNum < r.Base.MaxPlayer {
			if picked == nil || r.Base.RoomId < picked.Base.RoomId {
				picked = r
			}
		}
	}
	if picked != nil {
		picked.Base.CurPlayerNum++
		room.MarkDirty()
		rsp.ReservationId = impl.issueReservation(picked.Base.RoomId, int32(req.Stage), nowSec)
		impl.Unlock()
		rsp.RoomInfo = picked.Base
		return rsp
	}
	impl.Unlock()

	// 第二阶段：全部满员，建房（锁外执行，工厂内含总线发布）。
	// 工厂签名按 RoomSpec 具名字段传递（F02：历史缺陷曾在此互换 stage/coinType）。
	if impl.createRoomFn == nil {
		logger.Errorf("quick start create branch without factory wired {index:%d}", impl.Index)
		rsp.Ret.Code = g1_protocol.ErrorCode_ERR_TEXAS_SEAT_NOT_FOUND
		return rsp
	}
	base, err := impl.createRoomFn(RoomSpec{
		GameID:   req.GameId,
		Stage:    req.Stage,
		CoinType: req.CoinType,
	})
	if err != nil || base == nil {
		rsp.Ret.Code = g1_protocol.ErrorCode_ERR_TEXAS_SEAT_NOT_FOUND
		if err != nil {
			rsp.Ret.Msg = err.Error()
		}
		return rsp
	}

	// 乐观登记：新房先入表（占位 1 人）再返回，修复历史缺陷"返回 gamesvr 尚未建成的
	// 房间导致客户端立刻 join 必然失败"。若 gamesvr 最终没建成，由 CheckAndCreateRooms
	// 的过期清理兜底回收（EndTime = 建房时刻 + RoomKeepLive 分钟）。
	impl.Lock()
	if existing, ok := room.RoomsMap[base.RoomId]; ok && existing != nil && existing.Base != nil {
		// gamesvr 已抢先上报该房：以权威上报为准，未满员则在其上占位。
		if existing.Base.CurPlayerNum < existing.Base.MaxPlayer {
			existing.Base.CurPlayerNum++
			room.MarkDirty()
		}
		rsp.ReservationId = impl.issueReservation(existing.Base.RoomId, int32(req.Stage), nowSec)
		rsp.RoomInfo = existing.Base
		impl.Unlock()
		return rsp
	}
	if room.RoomsMap == nil {
		room.RoomsMap = make(map[uint64]*g1_protocol.RoomShowInfo)
	}
	base.CurPlayerNum = 1
	room.RoomsMap[base.RoomId] = &g1_protocol.RoomShowInfo{Base: base}
	room.MarkDirty()
	rsp.ReservationId = impl.issueReservation(base.RoomId, int32(req.Stage), nowSec)
	impl.Unlock()

	rsp.RoomInfo = base
	return rsp
}

// QuickStartRollback 凭预约票据归还快速开始占用的座位（mainsvr 对局加入失败时
// 回调，one-way）。幂等与边界（F03）：
//   - 携带票据（推荐路径）：票据存在且房间匹配 → 删除票据并减一，同一票据第二次
//     调用直接成功不再减（至多执行一次）；票据未知（未签发/已取消/已过期/重启
//     丢失）→ 幂等成功；票据与房间不匹配 → ERR_ARGV（不得用 A 的票据释放 B 的座位）。
//   - 未携带票据（旧版兼容）：退回按"房间计数"回滚——每次调用减一、只保证不减为
//     负。与其它玩家加入交错时可能多减一席，由 gamesvr 周期上报的真实人数收敛修正。
//   - 房间已删除/不存在：视为已归还，直接成功。
func (impl *TexasRoomCenterMgr) QuickStartRollback(req *g1_protocol.QuickStartRollbackReq) g1_protocol.ErrorCode {
	if req == nil || req.RoomId == 0 {
		return g1_protocol.ErrorCode_ERR_ARGV
	}

	room := impl.GetTexasObj(int32(req.Stage))

	impl.Lock()
	defer impl.Unlock()

	if rid := req.GetReservationId(); rid != 0 {
		resv, ok := impl.reservations[rid]
		if !ok {
			return g1_protocol.ErrorCode_ERR_OK // 未知票据：幂等成功，不重复扣减
		}
		if resv.roomID != req.RoomId {
			logger.Errorf("rollback reservation mismatch {resv:%d room:%d, req room:%d}", rid, resv.roomID, req.RoomId)
			return g1_protocol.ErrorCode_ERR_ARGV
		}
		delete(impl.reservations, rid)
		info := room.RoomsMap[resv.roomID]
		if info != nil && info.Base != nil && info.Base.CurPlayerNum > 0 {
			info.Base.CurPlayerNum--
			room.MarkDirty()
		}
		return g1_protocol.ErrorCode_ERR_OK
	}

	// 旧版路径：按房间计数回滚。
	info, ok := room.RoomsMap[req.RoomId]
	if !ok || info == nil || info.Base == nil {
		return g1_protocol.ErrorCode_ERR_OK
	}
	if info.Base.CurPlayerNum > 0 {
		info.Base.CurPlayerNum--
		room.MarkDirty()
	}
	return g1_protocol.ErrorCode_ERR_OK
}

// RoomListPage 分页获取房间列表：锁内完成快照与排序，锁外完成分页组装。
//
// 排序必须在锁内：less 比较器读取 Base.CurPlayerNum/EndTime 等会被并发修改的字段，
// 锁外排序既是数据竞争也会得到撕裂的比较结果；快照后分页是纯本地切片运算，锁外即可。
func (impl *TexasRoomCenterMgr) RoomListPage(req *g1_protocol.RoomListReq) *g1_protocol.RoomListRsp {
	rsp := &g1_protocol.RoomListRsp{Ret: &g1_protocol.Ret{Code: g1_protocol.ErrorCode_ERR_OK}}
	rsp.Stage = req.GetStage()
	rsp.GameId = req.GetGameId()
	rsp.PageIndex = req.GetPageIndex()
	rsp.PageSize = req.GetPageSize()
	rsp.CoinType = req.GetCoinType()

	if req.PageIndex < 1 || req.PageSize < 1 || req.PageSize > 500 {
		rsp.Ret.Code = g1_protocol.ErrorCode_ERR_ARGV
		return rsp
	}

	var rooms []*g1_protocol.RoomShowInfo
	if req.Stage == g1_protocol.RoomStage_Stage_ALL {
		impl.RLock()
		cnt := 0
		for _, texas := range impl.TexasMap {
			if texas.RoomsMap != nil {
				cnt += len(texas.RoomsMap)
			}
		}
		if cnt > 0 {
			rooms = make([]*g1_protocol.RoomShowInfo, 0, cnt)
			for _, texas := range impl.TexasMap {
				if texas.RoomsMap != nil {
					for _, room := range texas.RoomsMap {
						if room != nil && room.Base != nil {
							rooms = append(rooms, room)
						}
					}
				}
			}
		}
		if code := sortRoomsLocked(rooms, req.SortType); code != g1_protocol.ErrorCode_ERR_OK {
			impl.RUnlock()
			rsp.Ret.Code = code
			return rsp
		}
		impl.RUnlock()
	} else {
		room := impl.GetTexasObj(int32(req.Stage))
		impl.RLock()
		if len(room.RoomsMap) > 0 {
			rooms = make([]*g1_protocol.RoomShowInfo, 0, len(room.RoomsMap))
			for _, r := range room.RoomsMap {
				if r != nil && r.Base != nil {
					rooms = append(rooms, r)
				}
			}
		}
		if code := sortRoomsLocked(rooms, req.SortType); code != g1_protocol.ErrorCode_ERR_OK {
			impl.RUnlock()
			rsp.Ret.Code = code
			return rsp
		}
		impl.RUnlock()
	}

	// 锁外分页（只做本地切片裁剪，不再触碰共享数据）。
	total := uint32(len(rooms))
	start := (req.PageIndex - 1) * req.PageSize
	end := req.PageIndex * req.PageSize

	// 边界保护：起始越界返回空页；结束越界裁到末尾。
	if start >= total {
		start, end = 0, 0
	} else if end > total {
		end = total
	}

	if start < end {
		rsp.RoomList = rooms[start:end:end] // 三索引切片：带容量限制防止后续误修改
	}
	rsp.TotalCount = total
	return rsp
}

// CheckAndCreateRooms 周期巡检（5s tick 驱动），两阶段执行：
//
//	第一阶段（锁内）：
//	  1. 过期清理：删除 EndTime 已过的登记房。房间真实生命周期由 gamesvr 决定，
//	     此处只回收列表登记（含乐观登记但 gamesvr 未建成的残留房）；gamesvr 若
//	     仍存活会通过 UpdateRoomInfo 重新上报恢复。
//	  1b. 预约票据清扫：删除过期 reservation（F03；过期不释放座位，人数由上报收敛）。
//	  2. 补房判定：某 stage 下全部房间满员时收集补房任务（每 stage 一间）。
//	第二阶段（锁外）：执行建房（含总线发布），避免持锁做网络 IO。
func (impl *TexasRoomCenterMgr) CheckAndCreateRooms(nowMs int64) {
	if !impl.checkOpen() {
		return
	}

	var jobs []RoomSpec
	nowSec := nowSecOf(nowMs)

	impl.Lock()
	impl.sweepExpiredReservations(nowSec)
	for _, rstage := range impl.TexasMap {
		if rstage == nil {
			continue
		}

		// 1) 过期清理（原地删除 map 元素是安全的，Go 允许遍历中删除）。
		for roomId, room := range rstage.RoomsMap {
			if room == nil || room.Base == nil {
				continue
			}
			if room.Base.EndTime > 0 && nowSec > room.Base.EndTime {
				delete(rstage.RoomsMap, roomId)
				rstage.MarkDirty()
			}
		}

		if len(rstage.RoomsMap) == 0 {
			continue
		}

		// 2) 补房判定：全部满员才补一间（保证玩家总有可加入的房间，又不会过量建房）。
		fullCnt := 0
		var sample *g1_protocol.RoomShowInfo
		for _, room := range rstage.RoomsMap {
			if room == nil || room.Base == nil {
				continue
			}
			sample = room
			if room.Base.CurPlayerNum >= room.Base.MaxPlayer {
				fullCnt++
			}
		}
		if sample != nil && fullCnt == len(rstage.RoomsMap) {
			jobs = append(jobs, RoomSpec{
				GameID:   sample.Base.GameId,
				Stage:    sample.Base.Stage,
				CoinType: sample.Base.CoinType,
			})
		}
	}
	impl.Unlock()

	// 第二阶段：锁外补房。新房间经 gamesvr 上报进入列表后，下一轮巡检自然收敛。
	for _, j := range jobs {
		if impl.createRoomFn == nil {
			logger.Errorf("checkAndCreate without factory wired {index:%d}", impl.Index)
			break
		}
		if _, err := impl.createRoomFn(j); err != nil {
			logger.Warningf("checkAndCreate auto create room failed {index:%d, stage:%d} | %v",
				impl.Index, j.Stage, err)
		}
	}
}

// nowSecOf 毫秒时间戳转秒（EndTime 以秒为单位存储）。
func nowSecOf(nowMs int64) int64 {
	return nowMs / datetime.MS_PER_SECOND
}
