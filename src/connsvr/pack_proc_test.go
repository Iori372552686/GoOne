package connsvr

import (
	"net"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Iori372552686/GoOne/lib/api/sharedstruct"
	"github.com/Iori372552686/GoOne/lib/net/net_mgr"
	"github.com/Iori372552686/GoOne/lib/web/http_sign"
	"github.com/Iori372552686/GoOne/lib/web/rest_api"
	"github.com/Iori372552686/GoOne/module/conf"
	"github.com/Iori372552686/GoOne/src/connsvr/globals"
	g1_protocol "github.com/Iori372552686/g1_common/protocol"
	"google.golang.org/protobuf/proto"
)

// fakeAddrConn 包装 net.Pipe 连接，提供可解析的 RemoteAddr（BindClient 内
// net.SplitHostPort 必须成功）。
type fakeAddr struct{}

func (fakeAddr) Network() string { return "tcp" }
func (fakeAddr) String() string  { return "192.0.2.1:51234" }

type addrConn struct{ net.Conn }

func (c addrConn) RemoteAddr() net.Addr { return fakeAddr{} }

func newTestConn(t *testing.T) (net.Conn, func()) {
	t.Helper()
	server, client := net.Pipe()
	return addrConn{client}, func() {
		_ = server.Close()
		_ = client.Close()
	}
}

// fakeGateway 实现网关会话接口，用独立 SessionHub 记录绑定结果。
type fakeGateway struct {
	hub   *net_mgr.SessionHub
	binds []uint64
}

func newFakeGateway() *fakeGateway {
	return &fakeGateway{hub: net_mgr.NewSessionHub(nil)}
}

func (f *fakeGateway) SendByUid(uint64, []byte, []byte) error { return nil }
func (f *fakeGateway) BroadcastByZone(int32, []byte, []byte)  {}
func (f *fakeGateway) Kick(uint64, g1_protocol.EKickOutReason) {}
func (f *fakeGateway) KickByRemoteAddr(uint64, g1_protocol.EKickOutReason, string) {
}
func (f *fakeGateway) GetClientByUid(uid uint64) *net_mgr.Client {
	return f.hub.GetClientByUid(uid)
}
func (f *fakeGateway) GetClientByConn(conn net.Conn) *net_mgr.Client {
	return f.hub.GetClientByConn(conn)
}
func (f *fakeGateway) UpdateClientByUid(conn net.Conn, uid uint64, zone uint32) *net_mgr.Client {
	c, _, err := f.hub.BindClient(conn, uid, zone)
	if err != nil {
		return nil
	}
	f.binds = append(f.binds, uid)
	return c
}

func buildPacket(t *testing.T, uid uint64, cmd uint32, msg proto.Message) []byte {
	t.Helper()
	var body []byte
	if msg != nil {
		var err error
		if body, err = proto.Marshal(msg); err != nil {
			t.Fatalf("marshal body: %v", err)
		}
	}
	header := sharedstruct.CSPacketHeader{
		Version: 1, PassCode: 1, Seq: 1,
		Uid: uid, AppVersion: 1, Cmd: cmd, BodyLen: uint32(len(body)),
	}
	return append(header.ToBytes(), body...)
}

// setEnvMode 加载最小 conf 控制 env_mode；调用方 defer conf.ResetForTest()。
func setEnvMode(t *testing.T, mode string) {
	t.Helper()
	conf.ResetForTest()
	if err := conf.LoadBytes([]byte("base_cfg:\n  runtime:\n    env_mode: "+mode+"\n"), ".yaml"); err != nil {
		t.Fatalf("conf.LoadBytes: %v", err)
	}
}

// TestHandleClientPacketRejectsShortPacket 短包（< 28 字节头）被拒绝。
func TestHandleClientPacketRejectsShortPacket(t *testing.T) {
	gw := newFakeGateway()
	conn, cleanup := newTestConn(t)
	defer cleanup()

	handleClientPacket(gw, "tcp", conn, make([]byte, 10))
	if len(gw.binds) != 0 {
		t.Fatalf("short packet must not bind, got binds=%v", gw.binds)
	}
}

// TestHandleClientPacketRejectsInnerCmd 内部（非 GM）命令即使在 dev 也被网关拒绝。
func TestHandleClientPacketRejectsInnerCmd(t *testing.T) {
	setEnvMode(t, "dev")
	defer conf.ResetForTest()

	gw := newFakeGateway()
	conn, cleanup := newTestConn(t)
	defer cleanup()

	// cmd=0x21000：MsgType=1 → IsInnerCmd=true，且非 GM 类型。
	handleClientPacket(gw, "tcp", conn, buildPacket(t, 12345, 0x21000, nil))
	if len(gw.binds) != 0 {
		t.Fatalf("inner cmd must be rejected even in dev, got binds=%v", gw.binds)
	}
}

// F01 验收：非 dev 下未认证连接直接发自报 uid 的业务包，必须被拒绝（不得绑定）。
func TestUnauthenticatedBusinessPacketRejectedNonDev(t *testing.T) {
	setEnvMode(t, "test")
	defer conf.ResetForTest()

	gw := newFakeGateway()
	conn, cleanup := newTestConn(t)
	defer cleanup()

	handleClientPacket(gw, "tcp", conn, buildPacket(t, 12345, uint32(g1_protocol.CMD_MAIN_HEARTBEAT_REQ), nil))
	if len(gw.binds) != 0 {
		t.Fatalf("unauthenticated business packet must not bind, got binds=%v", gw.binds)
	}
	if gw.hub.GetClientByUid(12345) != nil {
		t.Fatal("forged uid must not create a session")
	}
}

// dev 预分配模型：业务首包携带 uid 直接绑定（tester/stress 依赖）；同连接
// 后续包不重复绑定。
func TestDevPreallocatedUidBinds(t *testing.T) {
	setEnvMode(t, "dev")
	defer conf.ResetForTest()

	gw := newFakeGateway()
	conn, cleanup := newTestConn(t)
	defer cleanup()

	handleClientPacket(gw, "tcp", conn, buildPacket(t, 777, uint32(g1_protocol.CMD_MAIN_HEARTBEAT_REQ), nil))
	if len(gw.binds) != 1 || gw.binds[0] != 777 {
		t.Fatalf("dev pre-allocated uid should bind once, got binds=%v", gw.binds)
	}

	handleClientPacket(gw, "tcp", conn, buildPacket(t, 777, uint32(g1_protocol.CMD_MAIN_HEARTBEAT_REQ), nil))
	if len(gw.binds) != 1 {
		t.Fatalf("bound conn must not re-bind, got binds=%v", gw.binds)
	}
}

// F01 验收：已绑定连接伪造不同 header uid 的包必须被拒绝（防跨 UID 重绑）。
func TestBoundConnRejectsMismatchedUid(t *testing.T) {
	setEnvMode(t, "dev")
	defer conf.ResetForTest()

	gw := newFakeGateway()
	conn, cleanup := newTestConn(t)
	defer cleanup()

	handleClientPacket(gw, "tcp", conn, buildPacket(t, 777, uint32(g1_protocol.CMD_MAIN_HEARTBEAT_REQ), nil))
	// 同一连接自报 uid=888：拒绝，不得抢占 888 的会话。
	handleClientPacket(gw, "tcp", conn, buildPacket(t, 888, uint32(g1_protocol.CMD_MAIN_HEARTBEAT_REQ), nil))

	if got := gw.hub.GetClientByUid(888); got != nil {
		t.Fatalf("cross-uid rebind must be rejected, uid 888 got client %v", got)
	}
	if c := gw.hub.GetClientByConn(conn); c == nil || c.Uid != 777 {
		t.Fatalf("session must stay bound to 777, got %+v", c)
	}
	if len(gw.binds) != 1 {
		t.Fatalf("no new bind expected, got binds=%v", gw.binds)
	}
}

// dev 登录回退：账号非纯数字（tester_acc_*）→ (true,0)，绑定客户端自报 uid。
func TestDevLoginFallbackBindsHeaderUid(t *testing.T) {
	setEnvMode(t, "dev")
	defer conf.ResetForTest()

	gw := newFakeGateway()
	conn, cleanup := newTestConn(t)
	defer cleanup()

	body := &g1_protocol.LoginReq{Account: "tester_acc_555", Token: "t", ChannelId: 1, LoginType: "guest"}
	handleClientPacket(gw, "tcp", conn, buildPacket(t, 555, uint32(g1_protocol.CMD_MAIN_LOGIN_REQ), body))
	if len(gw.binds) != 1 || gw.binds[0] != 555 {
		t.Fatalf("dev fallback should bind header uid, got binds=%v", gw.binds)
	}
}

// dev 登录：账号为纯数字时直接映射为 uid，优先于客户端自报 uid。
func TestDevLoginNumericAccountOverridesHeaderUid(t *testing.T) {
	setEnvMode(t, "dev")
	defer conf.ResetForTest()

	gw := newFakeGateway()
	conn, cleanup := newTestConn(t)
	defer cleanup()

	body := &g1_protocol.LoginReq{Account: "888", Token: "", ChannelId: 1, LoginType: "guest"}
	handleClientPacket(gw, "tcp", conn, buildPacket(t, 1, uint32(g1_protocol.CMD_MAIN_LOGIN_REQ), body))
	if len(gw.binds) != 1 || gw.binds[0] != 888 {
		t.Fatalf("numeric account uid must win over self-reported uid, got binds=%v", gw.binds)
	}
}

// F01 验收：非 dev 下认证失败（未配置账号服）的登录首包被丢弃，不建立绑定。
func TestNonDevLoginAuthFailureDropsPacket(t *testing.T) {
	setEnvMode(t, "test")
	defer conf.ResetForTest()

	oldRest := globals.RestMgr
	defer func() { globals.RestMgr = oldRest }()
	globals.RestMgr = rest_api.NewRestApiMgr() // 空：GetRestIns 返回 nil

	gw := newFakeGateway()
	conn, cleanup := newTestConn(t)
	defer cleanup()

	body := &g1_protocol.LoginReq{Account: "10001", Token: "bad-token", ChannelId: 1, LoginType: "guest"}
	handleClientPacket(gw, "tcp", conn, buildPacket(t, 0, uint32(g1_protocol.CMD_MAIN_LOGIN_REQ), body))
	if len(gw.binds) != 0 {
		t.Fatalf("auth failure must drop the first packet, got binds=%v", gw.binds)
	}
}

// F01 验收：非 dev 下认证成功绑定账号服返回的 uid，自报 uid 被覆盖。
func TestNonDevLoginAuthSuccessBindsAccountUid(t *testing.T) {
	setEnvMode(t, "test")
	defer conf.ResetForTest()

	oldRest := globals.RestMgr
	oldSign := globals.SignMgr
	defer func() {
		globals.RestMgr = oldRest
		globals.SignMgr = oldSign
	}()

	globals.SignMgr = http_sign.NewSignMgr()
	globals.SignMgr.InitAndRun([]http_sign.Config{
		{IndexName: "default", PrivateKey: "test", SignName: "sign", ExpiredTime: 1800, TimestampName: "timestamp", SignType: "md5"},
	})
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"code":0,"msg":"success","data":{"account_id":10001,"game_profile":{"uid":200123}}}`))
	}))
	defer ts.Close()
	globals.RestMgr = rest_api.NewRestApiMgr()
	globals.RestMgr.Init([]rest_api.Config{
		{ServiceName: "default", Urls: []string{ts.URL + "/api/v1/account/profile?"}, SignName: "default"},
	}, globals.SignMgr)

	gw := newFakeGateway()
	conn, cleanup := newTestConn(t)
	defer cleanup()

	body := &g1_protocol.LoginReq{Account: "10001", Token: "Bearer tok", ChannelId: 1, LoginType: "guest"}
	handleClientPacket(gw, "tcp", conn, buildPacket(t, 999, uint32(g1_protocol.CMD_MAIN_LOGIN_REQ), body))
	if len(gw.binds) != 1 || gw.binds[0] != 200123 {
		t.Fatalf("server-authenticated uid must win, got binds=%v", gw.binds)
	}
	if c := gw.hub.GetClientByUid(999); c != nil {
		t.Fatal("self-reported uid must not create a session")
	}
}
