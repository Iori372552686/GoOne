package session

import (
	"context"
	"fmt"
	"time"

	g1_protocol "github.com/Iori372552686/g1_common/protocol"
)

// Login 执行 GoOne 单步游客/账号登录；成功后同步 uid。
//
// GoOne 的登录模型：uid 由外部账号服/配置预分配，登录请求的 CS 头 Uid 字段
// 必须携带该 uid（connsvr 据此做会话绑定与路由；uid==0 会被直接丢弃）。
// 因此登录前先把 session.uid 设为预分配的 UserID。
func (s *Session) Login(ctx context.Context) error {
	if s.uid.Load() == 0 && s.userID > 0 {
		s.uid.Store(uint64(s.userID))
	}

	req := &g1_protocol.LoginReq{
		Account:   s.opts.AccountID,
		LoginType: "guest",
		ChannelId: 1,
		DeviceOs:  "tester",
	}
	if s.opts.Token != "" {
		req.Token = s.opts.Token
	}

	resp := &g1_protocol.LoginRsp{Ret: &g1_protocol.Ret{}}
	if err := s.RequestProto(ctx, uint32(g1_protocol.CMD_MAIN_LOGIN_REQ), req, resp, 15*time.Second); err != nil {
		return fmt.Errorf("session %d: login: %w", s.opts.ID, err)
	}
	if resp.Ret != nil && IsErrCode(int32(resp.Ret.Code)) {
		return fmt.Errorf("session %d: login ret code: %d, msg: %s", s.opts.ID, resp.Ret.Code, resp.Ret.Msg)
	}
	if resp.RoleInfo != nil && resp.RoleInfo.RegisterInfo != nil && resp.RoleInfo.RegisterInfo.Uid != 0 {
		uid := resp.RoleInfo.RegisterInfo.Uid
		s.uid.Store(uid)
		s.userID = int64(uid)
	}
	return nil
}
