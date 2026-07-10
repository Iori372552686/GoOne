package session

import (
	"context"
	"fmt"
	"time"

	g1_protocol "github.com/Iori372552686/game_protocol/protocol"
)

// Login 执行 GoOne 单步游客/账号登录；成功后同步 uid。
func (s *Session) Login(ctx context.Context) error {
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
	if resp.Ret != nil && resp.Ret.Code != 0 {
		return fmt.Errorf("session %d: login ret code: %d, msg: %s", s.opts.ID, resp.Ret.Code, resp.Ret.Msg)
	}
	if resp.RoleInfo != nil && resp.RoleInfo.RegisterInfo != nil && resp.RoleInfo.RegisterInfo.Uid != 0 {
		uid := resp.RoleInfo.RegisterInfo.Uid
		s.uid.Store(uid)
		s.userID = int64(uid)
	}
	return nil
}
