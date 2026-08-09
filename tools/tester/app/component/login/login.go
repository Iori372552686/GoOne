package login

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/Iori372552686/GoOne/tools/tester/app/component"
	"github.com/Iori372552686/GoOne/tools/tester/internal/session"
	"github.com/Iori372552686/GoOne/tools/tester/internal/testcfg"
	g1_protocol "github.com/Iori372552686/g1_common/protocol"
)

func init() {
	component.Register("login", func() component.TesterComponent {
		return &LoginComponent{}
	})
}

// LoginComponent 登录流程回归测试组件。
type LoginComponent struct {
	actorID   int
	accountID string
	userID    int64
	sender    component.MessageSender
	requester component.Requester
	cfg       *testcfg.Config
}

func (c *LoginComponent) Name() string { return "login" }

func (c *LoginComponent) OnInit(ctx *component.ComponentContext) error {
	c.actorID = ctx.ActorID
	c.accountID = ctx.AccountID
	c.userID = ctx.UserID
	c.sender = ctx.Sender
	c.requester = ctx.Requester
	c.cfg = ctx.Cfg
	log.Printf("[Actor %d][Login] Component initialized", c.actorID)
	return nil
}

func (c *LoginComponent) OnConnected() error {
	log.Printf("[Actor %d][Login] Connected to gateway", c.actorID)
	return nil
}

func (c *LoginComponent) OnAccountLogin(accountID string) error {
	c.accountID = accountID
	log.Printf("[Actor %d][Login] Account logged in: %s", c.actorID, accountID)
	return nil
}

func (c *LoginComponent) OnRoleLogin(userID int64) error {
	c.userID = userID
	log.Printf("[Actor %d][Login] Role logged in: uid=%d", c.actorID, userID)
	return nil
}

func (c *LoginComponent) RunTests(ctx context.Context) error {
	log.Printf("[Actor %d][Login] ===== Starting login tests =====", c.actorID)

	tests := []struct {
		name string
		fn   func(ctx context.Context) error
	}{
		{"T01_Login_ExistingAccount", c.testLoginExisting},
		{"T02_Login_WithToken", c.testLoginWithToken},
		{"T03_Heartbeat", c.testHeartbeat},
	}

	for _, test := range tests {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		log.Printf("[Actor %d][Login] --- %s ---", c.actorID, test.name)
		if err := test.fn(ctx); err != nil {
			return fmt.Errorf("%s: %w", test.name, err)
		}
		log.Printf("[Actor %d][Login] --- %s PASSED ---", c.actorID, test.name)
	}

	log.Printf("[Actor %d][Login] ===== All login tests PASSED =====", c.actorID)
	return nil
}

func (c *LoginComponent) OnMessage(cmd uint32, data []byte) bool {
	return false
}

// RunStress 压测正常路径：发送一次心跳（在线玩家最高频的操作）。
// 登录在会话建立时已由 session.Login 自动完成，循环里只做保持在线的轻量请求，
// 与 room 组件的快速开始/离开互补，共同构成压测负载。
func (c *LoginComponent) RunStress(ctx context.Context) error {
	return c.testHeartbeat(ctx)
}

// testLoginExisting 使用已有账号再次登录应稳定返回。
func (c *LoginComponent) testLoginExisting(ctx context.Context) error {
	req := &g1_protocol.LoginReq{
		Account:   c.accountID,
		LoginType: "guest",
		ChannelId: 1,
		DeviceOs:  "tester",
	}
	if c.cfg.Player.Token != "" {
		req.Token = c.cfg.Player.Token
	}

	resp := &g1_protocol.LoginRsp{Ret: &g1_protocol.Ret{}}
	if err := c.requester.RequestProto(ctx, uint32(g1_protocol.CMD_MAIN_LOGIN_REQ), req, resp, 15*time.Second); err != nil {
		return err
	}
	if resp.Ret != nil && session.IsErrCode(int32(resp.Ret.Code)) {
		return fmt.Errorf("login failed: code=%d msg=%s", resp.Ret.Code, resp.Ret.Msg)
	}
	if resp.RoleInfo == nil || resp.RoleInfo.RegisterInfo == nil {
		return fmt.Errorf("login response missing role info")
	}
	log.Printf("[Actor %d][Login] T01: Login OK uid=%d", c.actorID, resp.RoleInfo.RegisterInfo.Uid)
	return nil
}

// testLoginWithToken 验证 token 登录（若配置 token）。
func (c *LoginComponent) testLoginWithToken(ctx context.Context) error {
	if c.cfg.Player.Token == "" {
		log.Printf("[Actor %d][Login] T02: skipped (no token configured)", c.actorID)
		return nil
	}
	req := &g1_protocol.LoginReq{
		Account:   c.accountID,
		Token:     c.cfg.Player.Token,
		LoginType: "guest",
		ChannelId: 1,
		DeviceOs:  "tester",
	}
	resp := &g1_protocol.LoginRsp{Ret: &g1_protocol.Ret{}}
	if err := c.requester.RequestProto(ctx, uint32(g1_protocol.CMD_MAIN_LOGIN_REQ), req, resp, 15*time.Second); err != nil {
		return err
	}
	if resp.Ret != nil && session.IsErrCode(int32(resp.Ret.Code)) {
		return fmt.Errorf("token login failed: code=%d msg=%s", resp.Ret.Code, resp.Ret.Msg)
	}
	log.Printf("[Actor %d][Login] T02: Token login OK", c.actorID)
	return nil
}

// testHeartbeat 发送心跳并等待响应。
func (c *LoginComponent) testHeartbeat(ctx context.Context) error {
	req := &g1_protocol.HeartBeatReq{ClientNowMs: time.Now().UnixMilli()}
	resp := &g1_protocol.HeartBeatRsp{Ret: &g1_protocol.Ret{}}
	if err := c.requester.RequestProto(ctx, uint32(g1_protocol.CMD_MAIN_HEARTBEAT_REQ), req, resp, 10*time.Second); err != nil {
		return err
	}
	if resp.Ret != nil && session.IsErrCode(int32(resp.Ret.Code)) {
		return fmt.Errorf("heartbeat failed: code=%d msg=%s", resp.Ret.Code, resp.Ret.Msg)
	}
	log.Printf("[Actor %d][Login] T03: Heartbeat OK", c.actorID)
	return nil
}
