package texas_room

import (
	"sync"
	"sync/atomic"
	"testing"

	g1_protocol "github.com/Iori372552686/g1_common/protocol"
)

// ---- 测试辅助 ----

// newTestMgr 构造测试用 mgr：注入 fake 建房工厂，避免触碰全局 router/总线。
// created 记录工厂被调用次数，用于断言补房/建房行为。
func newTestMgr(index uint64) (mgr *TexasRoomCenterMgr, created *int32) {
	mgr = NewTexasRoomCenterMgr(index)
	cnt := int32(0)
	mgr.SetCreateRoomFn(func(gameId g1_protocol.GameTypeId, stage, coinType int32) (*g1_protocol.RoomBaseInfo, error) {
		atomic.AddInt32(&cnt, 1)
		id := atomic.LoadInt32(&cnt)
		return &g1_protocol.RoomBaseInfo{
			RoomId:       uint64(9000 + id), // 与预置房间错开的房号
			GameId:       gameId,
			Stage:        g1_protocol.RoomStage(stage),
			CoinType:     g1_protocol.CoinType(coinType),
			MaxPlayer:    9,
			MaxMember:    100,
			CurPlayerNum: 0,
			EndTime:      9999999999, // 不过期
		}, nil
	})
	return mgr, &cnt
}

// seedRoom 预置一间房（直接走公开接口 UpdateRoomInfo，保持语义一致）。
func seedRoom(t *testing.T, mgr *TexasRoomCenterMgr, roomId, cur, max uint32, stage g1_protocol.RoomStage, endTime int64) {
	t.Helper()
	code := mgr.UpdateRoomInfo(&g1_protocol.RoomShowInfo{
		Base: &g1_protocol.RoomBaseInfo{
			RoomId:       uint64(roomId),
			GameId:       1,
			Stage:        stage,
			CurPlayerNum: cur,
			MaxPlayer:    max,
			EndTime:      endTime,
		},
	})
	if code != g1_protocol.ErrorCode_ERR_OK {
		t.Fatalf("seedRoom %d failed: %v", roomId, code)
	}
}

// roomOf 从 mgr 中取指定房间的当前计数（测试断言用）。
func roomOf(t *testing.T, mgr *TexasRoomCenterMgr, stage g1_protocol.RoomStage, roomId uint64) *g1_protocol.RoomShowInfo {
	t.Helper()
	room := mgr.GetTexasObj(int32(stage))
	mgr.RLock()
	defer mgr.RUnlock()
	info, ok := room.RoomsMap[roomId]
	if !ok {
		return nil
	}
	return info
}

// ---- 快速开始：选房方向修复（原 bug 回归测试） ----

// TestQuickStartPicksNonFullRoom 验证修复"满员判断方向写反"：
// 原 bug（MaxPlayer < CurPlayerNum 才选中）导致有空位的房永远不命中。
// 修复后应选中"未满员且 RoomId 最小"的房。
func TestQuickStartPicksNonFullRoom(t *testing.T) {
	mgr, created := newTestMgr(1)

	seedRoom(t, mgr, 100, 5, 5, g1_protocol.RoomStage_LOW, 9999999999)  // 已满员：不应被选中
	seedRoom(t, mgr, 300, 0, 9, g1_protocol.RoomStage_LOW, 9999999999)  // 有空位：RoomId 更大
	seedRoom(t, mgr, 200, 3, 9, g1_protocol.RoomStage_LOW, 9999999999)  // 有空位：RoomId 最小，应被选中
	seedRoom(t, mgr, 150, 10, 9, g1_protocol.RoomStage_LOW, 9999999999) // 已超员（脏数据）：更不应被选中

	rsp := mgr.QuickStart(&g1_protocol.QuickStartReq{Stage: g1_protocol.RoomStage_LOW, GameId: 1})
	if rsp.Ret.Code != g1_protocol.ErrorCode_ERR_OK {
		t.Fatalf("quick start failed: %v", rsp.Ret.Code)
	}
	if rsp.RoomInfo == nil || rsp.RoomInfo.RoomId != 200 {
		t.Fatalf("应选中 RoomId=200 的未满房，实际: %v", rsp.RoomInfo)
	}
	if got := roomOf(t, mgr, g1_protocol.RoomStage_LOW, 200).Base.CurPlayerNum; got != 4 {
		t.Fatalf("选中房应占位 3->4，实际 %d", got)
	}
	if n := atomic.LoadInt32(created); n != 0 {
		t.Fatalf("存在空位时不应建房，实际建房 %d 次", n)
	}
}

// TestQuickStartAllFullCreatesAndRegisters 全部满员时：建房 + 乐观登记（占位1）。
// 修复"返回 gamesvr 尚未建成的房导致 join 必然失败且无登记残留可收敛"。
func TestQuickStartAllFullCreatesAndRegisters(t *testing.T) {
	mgr, created := newTestMgr(1)
	seedRoom(t, mgr, 100, 9, 9, g1_protocol.RoomStage_MIDDLE, 9999999999) // 唯一的房已满

	rsp := mgr.QuickStart(&g1_protocol.QuickStartReq{Stage: g1_protocol.RoomStage_MIDDLE, GameId: 1})
	if rsp.Ret.Code != g1_protocol.ErrorCode_ERR_OK || rsp.RoomInfo == nil {
		t.Fatalf("quick start create branch failed: code=%v room=%v", rsp.Ret.Code, rsp.RoomInfo)
	}
	if n := atomic.LoadInt32(created); n != 1 {
		t.Fatalf("应建房 1 次，实际 %d 次", n)
	}
	info := roomOf(t, mgr, g1_protocol.RoomStage_MIDDLE, rsp.RoomInfo.RoomId)
	if info == nil {
		t.Fatal("新建房应被乐观登记进房间表")
	}
	if info.Base.CurPlayerNum != 1 {
		t.Fatalf("乐观登记应占位 1，实际 %d", info.Base.CurPlayerNum)
	}
}

// TestQuickStartStageAllRejected Stage_ALL 是列表聚合值，快速开始应拒绝。
func TestQuickStartStageAllRejected(t *testing.T) {
	mgr, _ := newTestMgr(1)
	rsp := mgr.QuickStart(&g1_protocol.QuickStartReq{Stage: g1_protocol.RoomStage_Stage_ALL})
	if rsp.Ret.Code != g1_protocol.ErrorCode_ERR_ARGV {
		t.Fatalf("Stage_ALL 应返回 ERR_ARGV，实际 %v", rsp.Ret.Code)
	}
}

// ---- 快速开始回滚：幂等与边界 ----

// TestQuickStartRollback 归还占位；重复回滚不减为负；房间不存在幂等成功。
func TestQuickStartRollback(t *testing.T) {
	mgr, _ := newTestMgr(1)
	seedRoom(t, mgr, 100, 2, 9, g1_protocol.RoomStage_LOW, 9999999999)

	req := &g1_protocol.QuickStartRollbackReq{RoomId: 100, Stage: g1_protocol.RoomStage_LOW}
	// 第一次回滚：2 -> 1
	if code := mgr.QuickStartRollback(req); code != g1_protocol.ErrorCode_ERR_OK {
		t.Fatalf("rollback failed: %v", code)
	}
	if got := roomOf(t, mgr, g1_protocol.RoomStage_LOW, 100).Base.CurPlayerNum; got != 1 {
		t.Fatalf("回滚后应为 1，实际 %d", got)
	}
	// 第二次：1 -> 0
	_ = mgr.QuickStartRollback(req)
	if got := roomOf(t, mgr, g1_protocol.RoomStage_LOW, 100).Base.CurPlayerNum; got != 0 {
		t.Fatalf("二次回滚后应为 0，实际 %d", got)
	}
	// 第三次：0 不再下减（下限保护）
	_ = mgr.QuickStartRollback(req)
	if got := roomOf(t, mgr, g1_protocol.RoomStage_LOW, 100).Base.CurPlayerNum; got != 0 {
		t.Fatalf("计数不应减为负数，实际 %d", got)
	}
	// 不存在的房间：幂等成功
	if code := mgr.QuickStartRollback(&g1_protocol.QuickStartRollbackReq{RoomId: 404, Stage: g1_protocol.RoomStage_LOW}); code != g1_protocol.ErrorCode_ERR_OK {
		t.Fatalf("回滚不存在房间应幂等成功，实际 %v", code)
	}
}

// ---- 房间上报/删除：nil 防御与幂等 ----

// TestDelRoomInfoNilBase 原 bug 回归：req.Base 为 nil 时原实现直接解引用 panic。
func TestDelRoomInfoNilBase(t *testing.T) {
	mgr, _ := newTestMgr(1)
	if code := mgr.DelRoomInfo(nil); code != g1_protocol.ErrorCode_ERR_ARGV {
		t.Fatalf("nil req 应返回 ERR_ARGV，实际 %v", code)
	}
	if code := mgr.DelRoomInfo(&g1_protocol.RoomShowInfo{}); code != g1_protocol.ErrorCode_ERR_ARGV {
		t.Fatalf("nil Base 应返回 ERR_ARGV，实际 %v", code)
	}
}

// TestDelRoomInfoIdempotent 删除不存在的房间幂等成功；删除后快速开始不再选中。
func TestDelRoomInfoIdempotent(t *testing.T) {
	mgr, _ := newTestMgr(1)
	seedRoom(t, mgr, 100, 1, 9, g1_protocol.RoomStage_LOW, 9999999999)

	if code := mgr.DelRoomInfo(&g1_protocol.RoomShowInfo{Base: &g1_protocol.RoomBaseInfo{RoomId: 100, Stage: g1_protocol.RoomStage_LOW}}); code != g1_protocol.ErrorCode_ERR_OK {
		t.Fatalf("del failed: %v", code)
	}
	if code := mgr.DelRoomInfo(&g1_protocol.RoomShowInfo{Base: &g1_protocol.RoomBaseInfo{RoomId: 100, Stage: g1_protocol.RoomStage_LOW}}); code != g1_protocol.ErrorCode_ERR_OK {
		t.Fatalf("重复删除应幂等成功，实际 %v", code)
	}
	if roomOf(t, mgr, g1_protocol.RoomStage_LOW, 100) != nil {
		t.Fatal("房间应已删除")
	}
}

// TestUpdateRoomInfoReplace gamesvr 上报为权威数据源：整房替换（含真实人数收敛）。
func TestUpdateRoomInfoReplace(t *testing.T) {
	mgr, _ := newTestMgr(1)
	seedRoom(t, mgr, 100, 3, 9, g1_protocol.RoomStage_LOW, 9999999999)

	// gamesvr 上报真实人数 5（覆盖 roomcenter 侧的占位计数）
	code := mgr.UpdateRoomInfo(&g1_protocol.RoomShowInfo{Base: &g1_protocol.RoomBaseInfo{
		RoomId: 100, Stage: g1_protocol.RoomStage_LOW, CurPlayerNum: 5, MaxPlayer: 9, EndTime: 9999999999,
	}})
	if code != g1_protocol.ErrorCode_ERR_OK {
		t.Fatalf("update failed: %v", code)
	}
	if got := roomOf(t, mgr, g1_protocol.RoomStage_LOW, 100).Base.CurPlayerNum; got != 5 {
		t.Fatalf("上报应整房替换为 5，实际 %d", got)
	}
}

// ---- 房间列表：分页与排序 ----

func TestRoomListPage(t *testing.T) {
	mgr, _ := newTestMgr(1)
	seedRoom(t, mgr, 300, 1, 9, g1_protocol.RoomStage_LOW, 9999999999)
	seedRoom(t, mgr, 100, 2, 9, g1_protocol.RoomStage_LOW, 9999999999)
	seedRoom(t, mgr, 200, 3, 9, g1_protocol.RoomStage_LOW, 9999999999)

	// 指定 stage + ID 排序 + 第一页取 2 条
	rsp := mgr.RoomListPage(&g1_protocol.RoomListReq{
		Stage: g1_protocol.RoomStage_LOW, SortType: g1_protocol.RoomSortType_SORT_TYPE_ID,
		PageIndex: 1, PageSize: 2,
	})
	if rsp.Ret.Code != g1_protocol.ErrorCode_ERR_OK {
		t.Fatalf("room list failed: %v", rsp.Ret.Code)
	}
	if rsp.TotalCount != 3 || len(rsp.RoomList) != 2 {
		t.Fatalf("分页错误: total=%d len=%d", rsp.TotalCount, len(rsp.RoomList))
	}
	if rsp.RoomList[0].Base.RoomId != 100 || rsp.RoomList[1].Base.RoomId != 200 {
		t.Fatalf("ID 排序错误: %d, %d", rsp.RoomList[0].Base.RoomId, rsp.RoomList[1].Base.RoomId)
	}

	// 起始越界返回空页
	rsp = mgr.RoomListPage(&g1_protocol.RoomListReq{Stage: g1_protocol.RoomStage_LOW, PageIndex: 9, PageSize: 2})
	if len(rsp.RoomList) != 0 || rsp.TotalCount != 3 {
		t.Fatalf("越界分页应返回空页: len=%d", len(rsp.RoomList))
	}

	// 非法 PageSize 拒绝
	rsp = mgr.RoomListPage(&g1_protocol.RoomListReq{Stage: g1_protocol.RoomStage_LOW, PageIndex: 1, PageSize: 501})
	if rsp.Ret.Code != g1_protocol.ErrorCode_ERR_ARGV {
		t.Fatalf("PageSize>500 应拒绝，实际 %v", rsp.Ret.Code)
	}

	// 未知排序类型拒绝
	rsp = mgr.RoomListPage(&g1_protocol.RoomListReq{Stage: g1_protocol.RoomStage_LOW, PageIndex: 1, PageSize: 10, SortType: g1_protocol.RoomSortType(99)})
	if rsp.Ret.Code != g1_protocol.ErrorCode_ERR_ARGV {
		t.Fatalf("未知排序应拒绝，实际 %v", rsp.Ret.Code)
	}

	// Stage_ALL 跨 stage 聚合
	seedRoom(t, mgr, 400, 1, 9, g1_protocol.RoomStage_HIGH, 9999999999)
	rsp = mgr.RoomListPage(&g1_protocol.RoomListReq{Stage: g1_protocol.RoomStage_Stage_ALL, PageIndex: 1, PageSize: 10})
	if rsp.TotalCount != 4 {
		t.Fatalf("Stage_ALL 应聚合 4 间，实际 %d", rsp.TotalCount)
	}
}

// ---- 周期巡检：过期清理与满员补房 ----

// TestCheckAndCreateRoomsExpiryCleanup 过期房（now > EndTime）应被清理；
// gamesvr 若仍存活可经 UpdateRoomInfo 重新上报（此处验证清理本身）。
func TestCheckAndCreateRoomsExpiryCleanup(t *testing.T) {
	mgr, _ := newTestMgr(1)
	seedRoom(t, mgr, 100, 0, 9, g1_protocol.RoomStage_LOW, 1000)     // 已过期（EndTime=1000）
	seedRoom(t, mgr, 200, 1, 9, g1_protocol.RoomStage_LOW, 9999999999) // 存活

	mgr.CheckAndCreateRooms(5000 * 1000) // now=5000s > 1000s

	if roomOf(t, mgr, g1_protocol.RoomStage_LOW, 100) != nil {
		t.Fatal("过期房应被清理")
	}
	if roomOf(t, mgr, g1_protocol.RoomStage_LOW, 200) == nil {
		t.Fatal("存活房不应被误删")
	}
}

// TestCheckAndCreateRoomsAutoCreate 某 stage 全部满员时应补一间房；
// 存在未满房时不补（防止建房风暴——旧死代码 checkAndSync 的缺陷即每满员房建一间）。
func TestCheckAndCreateRoomsAutoCreate(t *testing.T) {
	mgr, created := newTestMgr(1)
	seedRoom(t, mgr, 100, 9, 9, g1_protocol.RoomStage_HIGH, 9999999999) // 满员
	seedRoom(t, mgr, 200, 9, 9, g1_protocol.RoomStage_HIGH, 9999999999) // 满员
	seedRoom(t, mgr, 300, 1, 9, g1_protocol.RoomStage_LOW, 9999999999)  // 另一 stage 未满

	mgr.CheckAndCreateRooms(1000) // now=1s，均未过期

	if n := atomic.LoadInt32(created); n != 1 {
		t.Fatalf("HIGH 全满应补 1 间，LOW 有空位不应补，实际建房 %d 次", n)
	}
}

// ---- 并发不变量（须以 go test -race 运行） ----

// TestConcurrentQuickStartNoOverfill 验证"锁内选房+占位"的原子性：
// 并发快速开始 N 次后，任何房间的人数都不会超过 MaxPlayer。
// 这是不变量测试：总数守恒（分配次数 == 各房计数增量之和）且无人超员。
func TestConcurrentQuickStartNoOverfill(t *testing.T) {
	mgr, _ := newTestMgr(1)
	seedRoom(t, mgr, 100, 0, 5, g1_protocol.RoomStage_LOW, 9999999999) // 容量5
	seedRoom(t, mgr, 200, 0, 5, g1_protocol.RoomStage_LOW, 9999999999) // 容量5

	const n = 30 // 两房总容量 10，其余 20 次应走建房分支（fake 工厂房容量 9，RoomId=9001+）
	var wg sync.WaitGroup
	var okCount int32
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			rsp := mgr.QuickStart(&g1_protocol.QuickStartReq{Stage: g1_protocol.RoomStage_LOW, GameId: 1})
			if rsp.Ret.Code == g1_protocol.ErrorCode_ERR_OK && rsp.RoomInfo != nil {
				atomic.AddInt32(&okCount, 1)
			}
		}()
	}
	wg.Wait()

	// 校验不变量：所有登记房的人数均不超上限。
	mgr.RLock()
	rooms := mgr.GetTexasObj(int32(g1_protocol.RoomStage_LOW))
	total := uint32(0)
	for id, r := range rooms.RoomsMap {
		if r == nil || r.Base == nil {
			continue
		}
		if r.Base.CurPlayerNum > r.Base.MaxPlayer {
			mgr.RUnlock()
			t.Fatalf("房间 %d 超员: %d/%d", id, r.Base.CurPlayerNum, r.Base.MaxPlayer)
		}
		total += r.Base.CurPlayerNum
	}
	mgr.RUnlock()

	if total != n {
		t.Fatalf("总量不守恒: 30 次分配应有 30 个占位，实际 %d", total)
	}
	if atomic.LoadInt32(&okCount) != n {
		t.Fatalf("应全部成功，实际成功 %d/30", okCount)
	}
}

// TestConcurrentMixedAccess 混合并发（上报/删除/快开/列表/巡检/持久化），
// 验证封装在 -race 下无数据竞争。Redis 未配置时 SaveRoomDataToDB 为 no-op，可安全调用。
func TestConcurrentMixedAccess(t *testing.T) {
	mgr, _ := newTestMgr(1)

	var wg sync.WaitGroup
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func(i int) { // gamesvr 上报者
			defer wg.Done()
			for j := 0; j < 50; j++ {
				roomId := uint64(100 + (i*50+j)%3)
				_ = mgr.UpdateRoomInfo(&g1_protocol.RoomShowInfo{Base: &g1_protocol.RoomBaseInfo{
					RoomId: roomId, Stage: g1_protocol.RoomStage_LOW,
					CurPlayerNum: 1, MaxPlayer: 9, EndTime: 9999999999,
				}})
			}
		}(i)

		wg.Add(1)
		go func() { // 快速开始者
			defer wg.Done()
			for j := 0; j < 50; j++ {
				_ = mgr.QuickStart(&g1_protocol.QuickStartReq{Stage: g1_protocol.RoomStage_LOW, GameId: 1})
			}
		}()

		wg.Add(1)
		go func() { // 列表查询者
			defer wg.Done()
			for j := 0; j < 50; j++ {
				_ = mgr.RoomListPage(&g1_protocol.RoomListReq{
					Stage: g1_protocol.RoomStage_LOW, PageIndex: 1, PageSize: 10,
					SortType: g1_protocol.RoomSortType_SORT_TYPE_ID,
				})
			}
		}()
	}

	wg.Add(1)
	go func() { // 巡检者（含过期清理与补房）
		defer wg.Done()
		for j := 0; j < 50; j++ {
			mgr.CheckAndCreateRooms(int64(1000 + j))
		}
	}()
	wg.Add(1)
	go func() { // 持久化者（无 Redis 配置时为 no-op）
		defer wg.Done()
		for j := 0; j < 50; j++ {
			_ = mgr.SaveRoomDataToDB()
		}
	}()
	wg.Add(1)
	go func() { // 删除者
		defer wg.Done()
		for j := 0; j < 50; j++ {
			_ = mgr.DelRoomInfo(&g1_protocol.RoomShowInfo{Base: &g1_protocol.RoomBaseInfo{
				RoomId: uint64(100 + j%3), Stage: g1_protocol.RoomStage_LOW,
			}})
		}
	}()

	wg.Wait()
	// 能走到这里且 -race 未报即通过；做一次终态读取确保数据结构可用。
	_ = mgr.RoomListPage(&g1_protocol.RoomListReq{Stage: g1_protocol.RoomStage_Stage_ALL, PageIndex: 1, PageSize: 10})
}

// TestGetTexasObjLazyRestoreNoRedis 无 Redis 配置时懒创建不 panic、返回空表
// （快照恢复降级路径：由 gamesvr 上报重建）。
func TestGetTexasObjLazyRestoreNoRedis(t *testing.T) {
	mgr, _ := newTestMgr(7)
	room := mgr.GetTexasObj(int32(g1_protocol.RoomStage_LOW))
	if room == nil || room.RoomsMap == nil {
		t.Fatal("懒创建应返回带空 map 的房间表")
	}
	if len(room.RoomsMap) != 0 {
		t.Fatalf("无快照时应为空表，实际 %d 间", len(room.RoomsMap))
	}
}

// TestQuickStartDeterministic 同输入重复调用选房结果确定（RoomId 最小优先），
// 避免 map 随机遍历导致行为不可复现。
func TestQuickStartDeterministic(t *testing.T) {
	for round := 0; round < 20; round++ {
		mgr, _ := newTestMgr(uint64(round))
		seedRoom(t, mgr, 500, 0, 9, g1_protocol.RoomStage_LOW, 9999999999)
		seedRoom(t, mgr, 400, 0, 9, g1_protocol.RoomStage_LOW, 9999999999)
		seedRoom(t, mgr, 600, 0, 9, g1_protocol.RoomStage_LOW, 9999999999)

		rsp := mgr.QuickStart(&g1_protocol.QuickStartReq{Stage: g1_protocol.RoomStage_LOW, GameId: 1})
		if rsp.RoomInfo == nil || rsp.RoomInfo.RoomId != 400 {
			t.Fatalf("round %d: 应稳定选中 RoomId=400，实际 %v", round, rsp.RoomInfo)
		}
	}
}
