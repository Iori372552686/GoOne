package service

import (
	"context"
	"errors"
	"testing"

	"github.com/Iori372552686/GoOne/lib/api/cmd_handler"
	"github.com/Iori372552686/GoOne/lib/db/redis"
	"github.com/Iori372552686/GoOne/lib/service/ssrpc"
	"github.com/Iori372552686/GoOne/src/mainsvr/globals"
	"github.com/Iori372552686/GoOne/src/mainsvr/globals/rds"
	"github.com/Iori372552686/GoOne/src/mainsvr/role"
	g1_protocol "github.com/Iori372552686/g1_common/protocol"
	"github.com/golang/protobuf/proto"
	goredis "github.com/redis/go-redis/v9"
)

var _ cmd_handler.IContext = (*stubIContext)(nil)

// ---- 测试桩（与 role/session_test 同构，service 层独立持有） ----

type stubIContext struct{ uid uint64 }

func (f *stubIContext) Uid() uint64         { return f.uid }
func (f *stubIContext) Zone() uint32        { return 0 }
func (f *stubIContext) Rid() uint64         { return 0 }
func (f *stubIContext) OriSrcBusId() uint32 { return 0 }
func (f *stubIContext) Ip() uint32          { return 0 }
func (f *stubIContext) Flag() uint32        { return 0 }

func (f *stubIContext) ParseMsg(data []byte, msg proto.Message) error { return nil }
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

type stubRedisClient struct {
	goredis.UniversalClient
	err error
}

func (c *stubRedisClient) HSet(ctx context.Context, _ string, _ ...interface{}) *goredis.IntCmd {
	cmd := goredis.NewIntCmd(ctx)
	if c.err != nil {
		cmd.SetErr(c.err)
	} else {
		cmd.SetVal(1)
	}
	return cmd
}

func withStubRedis(client *stubRedisClient) func() {
	oldMgr := rds.RedisMgr
	rds.RedisMgr = redis.NewRedisMgr()
	rds.RedisMgr.AddClientInstance(uint32(g1_protocol.DBType_DB_TYPE_ROLE), client)
	return func() { rds.RedisMgr = oldMgr }
}

// ---- DI 验收（报告 §7.1：同进程构造两个互不污染的用例实例） ----

// TestConstructorInjectsRoles 构造注入生效；nil 回退全局。
func TestConstructorInjectsRoles(t *testing.T) {
	mgr := role.NewRoleMgr()
	svc := NewMainC2SServiceImpl(mgr)
	if svc.roles != mgr {
		t.Fatal("构造应注入显式依赖")
	}
	if fallback := NewMainC2SServiceImpl(nil); fallback.roles != globals.RoleMgr {
		t.Fatal("nil 依赖应回退全局 RoleMgr（兼容旧装配）")
	}
}

// TestTwoServiceInstancesIsolated 两个实例各自持有独立 RoleMgr：A 实例的登出
// 不影响 B 实例的角色状态，全程无需重置全局变量。
func TestTwoServiceInstancesIsolated(t *testing.T) {
	stub := &stubRedisClient{}
	restore := withStubRedis(stub)
	defer restore()

	mgrA, mgrB := role.NewRoleMgr(), role.NewRoleMgr()
	svcA, _ := NewMainC2SServiceImpl(mgrA), NewMainC2SServiceImpl(mgrB)

	roleA := role.NewRole(2001)
	roleA.PbRole.LoginInfo.LastHartBeatTime = 1 // 过期，允许登出删除
	mgrA.PutRole(2001, roleA)
	// B 持有同一 uid 的另一角色实例
	roleB := role.NewRole(2001)
	mgrB.PutRole(2001, roleB)

	ctx := &ssrpc.Context{IContext: &stubIContext{uid: 2001}}
	rsp, err := svcA.Logout(ctx, &g1_protocol.LogoutReq{})
	if err != nil || rsp.GetRet().GetCode() != g1_protocol.ErrorCode_ERR_OK {
		t.Fatalf("A 实例登出应成功: rsp=%v err=%v", rsp.GetRet().GetCode(), err)
	}
	if mgrA.GetRole(2001) != nil {
		t.Fatal("A 实例登出后角色应从 A 移除")
	}
	if mgrB.GetRole(2001) == nil {
		t.Fatal("B 实例的角色不受 A 实例登出影响（实例隔离）")
	}
}

// 保存失败时 handler 层映射 ERR_DB（logout incomplete），协议边界可感知失败。
func TestLogoutHandlerMapsSaveFailure(t *testing.T) {
	stub := &stubRedisClient{err: errors.New("redis unavailable")}
	restore := withStubRedis(stub)
	defer restore()

	mgr := role.NewRoleMgr()
	svc := NewMainC2SServiceImpl(mgr)
	roleC := role.NewRole(2002)
	roleC.PbRole.LoginInfo.LastHartBeatTime = 1
	mgr.PutRole(2002, roleC)

	ctx := &ssrpc.Context{IContext: &stubIContext{uid: 2002}}
	rsp, err := svc.Logout(ctx, &g1_protocol.LogoutReq{})
	if err != nil {
		t.Fatalf("协议层应返回错误码而非 transport error: %v", err)
	}
	if rsp.GetRet().GetCode() != g1_protocol.ErrorCode_ERR_DB {
		t.Fatalf("保存失败应映射 ERR_DB，实际 %v", rsp.GetRet().GetCode())
	}
	if mgr.GetRole(2002) == nil {
		t.Fatal("保存失败时角色应保留（F04 契约在 handler 层不破坏）")
	}
}
