package role

import (
	"context"
	"errors"
	"testing"

	"github.com/Iori372552686/GoOne/lib/api/cmd_handler"
	"github.com/Iori372552686/GoOne/lib/api/datetime"
	"github.com/Iori372552686/GoOne/src/mainsvr/globals/rds"
	g1_protocol "github.com/Iori372552686/g1_common/protocol"
	goredis "github.com/redis/go-redis/v9"
	"github.com/Iori372552686/GoOne/lib/db/redis"
	"github.com/golang/protobuf/proto"
)

// ---- 测试桩 ----

// stubIContext 只提供身份字段的空事务上下文。
type stubIContext struct{ uid uint64 }

func (f *stubIContext) Uid() uint64         { return f.uid }
func (f *stubIContext) Zone() uint32        { return 0 }
func (f *stubIContext) Rid() uint64         { return 0 }
func (f *stubIContext) OriSrcBusId() uint32 { return 0 }
func (f *stubIContext) Ip() uint32          { return 0 }
func (f *stubIContext) Flag() uint32        { return 0 }
func (f *stubIContext) ParseMsg(_ []byte, _ proto.Message) error {
	return nil
}
func (f *stubIContext) CallMsgBySvrType(uint32, g1_protocol.CMD, proto.Message, proto.Message) error {
	panic("not used")
}
func (f *stubIContext) CallMsgByRouter(uint32, uint64, g1_protocol.CMD, proto.Message, proto.Message) error {
	panic("not used")
}
func (f *stubIContext) CallOtherMsgBySvrType(uint32, uint64, uint64, uint32, g1_protocol.CMD, proto.Message, proto.Message) error {
	panic("not used")
}
func (f *stubIContext) SendMsgBack(proto.Message) {}
func (f *stubIContext) SendMsgByServerType(uint32, g1_protocol.CMD, proto.Message) error {
	panic("not used")
}
func (f *stubIContext) SendMsgByRouter(uint32, uint64, g1_protocol.CMD, proto.Message) error {
	panic("not used")
}
func (f *stubIContext) Errorf(string, ...interface{})   {}
func (f *stubIContext) Warningf(string, ...interface{}) {}
func (f *stubIContext) Infof(string, ...interface{})    {}
func (f *stubIContext) Debugf(string, ...interface{})   {}

var _ cmd_handler.IContext = (*stubIContext)(nil)

// stubRedisClient 可控失败的 Redis 桩：err 非 nil 时 HSet 返回错误；
// 始终尊重传入 ctx 的取消（模拟真实客户端行为，验证 ctx 贯通）。
type stubRedisClient struct {
	goredis.UniversalClient
	err   error
	calls int
}

func (c *stubRedisClient) HSet(ctx context.Context, _ string, _ ...interface{}) *goredis.IntCmd {
	cmd := goredis.NewIntCmd(ctx)
	c.calls++
	if err := ctx.Err(); err != nil {
		cmd.SetErr(err)
		return cmd
	}
	if c.err != nil {
		cmd.SetErr(c.err)
	} else {
		cmd.SetVal(1)
	}
	return cmd
}

// withStubRedis 把 rds.RedisMgr 换成带桩实例的 mgr，返回还原函数与桩。
func withStubRedis(client *stubRedisClient) func() {
	oldMgr := rds.RedisMgr
	rds.RedisMgr = redis.NewRedisMgr()
	rds.RedisMgr.AddClientInstance(uint32(g1_protocol.DBType_DB_TYPE_ROLE), client)
	return func() { rds.RedisMgr = oldMgr }
}

func newOnlineRole(t *testing.T, mgr *RoleMgr, uid uint64, heartbeatFresh bool) *Role {
	t.Helper()
	role := NewRole(uid)
	if heartbeatFresh {
		role.PbRole.LoginInfo.LastHartBeatTime = datetime.Now()
	} else {
		role.PbRole.LoginInfo.LastHartBeatTime = datetime.Now() - 300
	}
	mgr.setRole(uid, role)
	return role
}

// ---- F04 验收用例 ----

// 保存失败时角色必须保留（有可恢复/重试状态），且返回明确错误。
func TestLogoutRetainsRoleWhenSaveFails(t *testing.T) {
	stub := &stubRedisClient{err: errors.New("redis unavailable")}
	restore := withStubRedis(stub)
	defer restore()

	mgr := NewRoleMgr()
	newOnlineRole(t, mgr, 1001, false)
	trans := &stubIContext{uid: 1001}

	if err := mgr.Logout(1001, trans, false, "client"); err == nil {
		t.Fatal("保存失败时 Logout 必须返回错误")
	}
	if mgr.GetRole(1001) == nil {
		t.Fatal("保存失败时角色必须保留在内存供重试（F04）")
	}
	if stub.calls == 0 {
		t.Fatal("Logout 应已尝试保存")
	}
}

// 重试成功后仅删除一次；重复退出幂等。
func TestLogoutRetrySuccessDeletesOnceThenIdempotent(t *testing.T) {
	stub := &stubRedisClient{}
	restore := withStubRedis(stub)
	defer restore()

	mgr := NewRoleMgr()
	newOnlineRole(t, mgr, 1002, false)
	trans := &stubIContext{uid: 1002}

	// 第一次失败
	stub.err = errors.New("redis unavailable")
	if err := mgr.Logout(1002, trans, false, "client"); err == nil {
		t.Fatal("第一次保存应失败")
	}
	// 重试成功 → 删除
	stub.err = nil
	if err := mgr.Logout(1002, trans, false, "client"); err != nil {
		t.Fatalf("重试成功应完成退出: %v", err)
	}
	if mgr.GetRole(1002) != nil {
		t.Fatal("保存成功后角色应被移除")
	}
	// 重复退出：幂等成功
	if err := mgr.Logout(1002, trans, false, "client"); err != nil {
		t.Fatalf("重复退出应幂等成功: %v", err)
	}
}

// 保存成功路径正常删除（正常回归）。
func TestLogoutDeletesOnSaveSuccess(t *testing.T) {
	stub := &stubRedisClient{}
	restore := withStubRedis(stub)
	defer restore()

	mgr := NewRoleMgr()
	role := newOnlineRole(t, mgr, 1003, false)
	trans := &stubIContext{uid: 1003}

	if err := mgr.Logout(1003, trans, false, "client"); err != nil {
		t.Fatalf("保存成功应完成退出: %v", err)
	}
	if mgr.GetRole(1003) != nil {
		t.Fatal("角色应被移除")
	}
	if role.PbRole.LoginInfo.LastLogoutTime == 0 {
		t.Fatal("应记录 LastLogoutTime")
	}
}

// 心跳过期路径的新鲜度守卫：过期请求排队期间玩家重新登录（心跳新鲜），
// 不得删除角色，也不应触发保存。
func TestLogoutHeartbeatExpiredSkipsFreshSession(t *testing.T) {
	stub := &stubRedisClient{}
	restore := withStubRedis(stub)
	defer restore()

	mgr := NewRoleMgr()
	newOnlineRole(t, mgr, 1004, true) // 心跳新鲜
	trans := &stubIContext{uid: 1004}

	if err := mgr.Logout(1004, trans, true, LogoutReasonHeartbeatExpired); err != nil {
		t.Fatalf("新鲜会话下的过期登出应跳过且无错误: %v", err)
	}
	if mgr.GetRole(1004) == nil {
		t.Fatal("旧过期任务不得删除重新登录后的会话角色")
	}
	if stub.calls != 0 {
		t.Fatalf("跳过时不应触发保存，实际 HSet %d 次", stub.calls)
	}
}

// disconnect 路径不设新鲜度守卫：断开登出照常保存并删除（迟到的断开登出
// 先保存再删除，重登方从存储重载，不损失数据）。
func TestLogoutDisconnectProceedsWithFreshHeartbeat(t *testing.T) {
	stub := &stubRedisClient{}
	restore := withStubRedis(stub)
	defer restore()

	mgr := NewRoleMgr()
	newOnlineRole(t, mgr, 1005, true)
	trans := &stubIContext{uid: 1005}

	if err := mgr.Logout(1005, trans, true, LogoutReasonDisconnect); err != nil {
		t.Fatalf("disconnect 登出应正常完成: %v", err)
	}
	if mgr.GetRole(1005) != nil {
		t.Fatal("disconnect 登出应移除角色")
	}
}

// F07 验收：SaveHashSync 的 ctx（Drain 排空预算）能传导到 Redis 调用，
// 取消后快速失败且保留 dirty 供后续重试。
func TestSaveHashSyncRespectsCancelledContext(t *testing.T) {
	stub := &stubRedisClient{}
	restore := withStubRedis(stub)
	defer restore()

	role := NewRole(1006)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	err := role.SaveHashSync(ctx)
	if err == nil {
		t.Fatal("取消的 ctx 应使保存快速失败")
	}
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("错误应包含 context.Canceled，实际: %v", err)
	}
	if stub.calls == 0 {
		t.Fatal("ctx 应已传导到 Redis 调用")
	}
}
