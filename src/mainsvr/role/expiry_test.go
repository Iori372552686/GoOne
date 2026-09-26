package role

import (
	"context"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/Iori372552686/GoOne/lib/api/datetime"
	g1_protocol "github.com/Iori372552686/g1_common/protocol"
)

// F06：Tick 过期筛选只读原子快照，不读 PbRole 字段——并发心跳与 Tick 同时
// 运行不 panic、候选判定正确（数据竞争由 CI -race 覆盖，此处验证行为不变量）。
func TestExpirySweepUsesAtomicSnapshot(t *testing.T) {
	stub := &stubRedisClient{}
	restore := withStubRedis(stub)
	defer restore()

	mgr := NewRoleMgr()
	fresh := NewRole(3001)
	mgr.PutRole(3001, fresh)
	fresh.OnClientHeartbeat(datetime.Now()) // 新鲜心跳（含原子快照）

	stale := NewRole(3002)
	mgr.PutRole(3002, stale)
	stale.OnClientHeartbeat(datetime.Now() - 300) // 5 分钟前

	var queued int32
	oldSender := SelfLogoutSender
	SelfLogoutSender = func(uid uint64, zone uint32, req *g1_protocol.LogoutReq) {
		atomic.AddInt32(&queued, 1)
	}
	defer func() { SelfLogoutSender = oldSender }()

	mgr.Tick(context.Background())

	if got := atomic.LoadInt32(&queued); got != 1 {
		t.Fatalf("只应投递 1 个过期候选（stale），实际 %d", got)
	}
	if mgr.GetRole(3001) == nil {
		t.Fatal("新鲜角色不应被投递过期登出")
	}
	if mgr.GetRole(3002) == nil {
		t.Fatal("Tick 只投递不删除，删除在 UID 串行域内完成")
	}
}

// F06 验收：排队期间合法重连（心跳刷新）后，旧过期任务不得删除新会话角色。
func TestStaleExpiryTaskDoesNotDeleteReloginedSession(t *testing.T) {
	stub := &stubRedisClient{}
	restore := withStubRedis(stub)
	defer restore()

	mgr := NewRoleMgr()
	role := NewRole(3003)
	mgr.PutRole(3003, role)
	role.OnClientHeartbeat(datetime.Now() - 300) // 过期 → Tick 投递

	// 投递到 uid 队列后、Logout 执行前，玩家重新登录刷新心跳。
	role.OnClientHeartbeat(datetime.Now())

	// 模拟队列中迟到的过期登出在 UID 串行域内执行。
	if err := mgr.Logout(3003, &stubIContext{uid: 3003}, true, LogoutReasonHeartbeatExpired); err != nil {
		t.Fatalf("迟到过期任务应跳过且无错误: %v", err)
	}
	if mgr.GetRole(3003) == nil {
		t.Fatal("旧过期任务不得删除重新登录后的会话角色")
	}
}

// 确认过期的任务在 UID 串行域内正常完成踢人+保存+删除（kick 因 busId=0 跳过，
// 保存走桩成功）。
func TestConfirmedExpiryTaskCompletesLogout(t *testing.T) {
	stub := &stubRedisClient{}
	restore := withStubRedis(stub)
	defer restore()

	mgr := NewRoleMgr()
	role := NewRole(3004)
	mgr.PutRole(3004, role)
	role.OnClientHeartbeat(datetime.Now() - 300)

	if err := mgr.Logout(3004, &stubIContext{uid: 3004}, true, LogoutReasonHeartbeatExpired); err != nil {
		t.Fatalf("确认过期的登出应完成: %v", err)
	}
	if mgr.GetRole(3004) != nil {
		t.Fatal("过期角色应被移除")
	}
}

// 并发不变量：多 goroutine 心跳 + Tick 混跑（-race 下验证无数据竞争）。
func TestConcurrentHeartbeatAndSweep(t *testing.T) {
	stub := &stubRedisClient{}
	restore := withStubRedis(stub)
	defer restore()

	mgr := NewRoleMgr()
	roles := make([]*Role, 8)
	for i := range roles {
		roles[i] = NewRole(uint64(3100 + i))
		mgr.PutRole(roles[i].Uid(), roles[i])
		roles[i].OnClientHeartbeat(datetime.Now())
	}

	oldSender := SelfLogoutSender
	var mu sync.Mutex
	queuedUids := make([]uint64, 0, 8)
	SelfLogoutSender = func(uid uint64, zone uint32, req *g1_protocol.LogoutReq) {
		mu.Lock()
		queuedUids = append(queuedUids, uid)
		mu.Unlock()
	}
	defer func() { SelfLogoutSender = oldSender }()

	var wg sync.WaitGroup
	// 2 个心跳协程：持续刷新全部角色（永不过期）。
	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < 2000; j++ {
				for _, r := range roles {
					r.OnClientHeartbeat(datetime.Now())
				}
			}
		}()
	}
	// 2 个 Tick 协程：并发过期扫描。
	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < 200; j++ {
				mgr.Tick(context.Background())
			}
		}()
	}
	wg.Wait()

	mu.Lock()
	defer mu.Unlock()
	for _, uid := range queuedUids {
		t.Fatalf("持续心跳的角色不应成为过期候选（uid=%d 被误投递）", uid)
	}
}
