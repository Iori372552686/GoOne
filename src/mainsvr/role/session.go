package role

import (
	"github.com/Iori372552686/GoOne/lib/api/cmd_handler"
)

// 登出原因字面量。与发送方保持一致：lib/net/net_mgr 三传输在连接关闭时发送
// "disconnect"；RoleMgr 心跳过期投递 "heartbeat expired"。两侧字符串必须同步。
const (
	LogoutReasonDisconnect       = "disconnect"
	LogoutReasonHeartbeatExpired = "heartbeat expired"
)

// Logout 收敛"保存并退出"用例（F04）：完整拥有停写、保存、移除的语义，
// 调用方（协议 handler）只做协议转换，不再自行拼接 SaveToDB + DeleteRole。
//
// 契约：
//   - 角色不在内存：幂等成功（重复退出不会二次删除）。
//   - 保存失败：返回错误，角色与未清的 persistDirtyMask 保留在内存——由既有
//     10s 防抖持久化与 role_tick 过期扫描重试；成功保存前退出未完成。
//   - 保存成功：从内存移除，仅此一次。
//
// 新鲜度守卫（心跳过期路径）：过期判定在 Tick 协程做出，请求投递到 uid 队列
// 期间玩家可能已重新登录并刷新心跳，此时不得删除新会话的角色。disconnect
// 路径不设此守卫：断开登出即使迟到也先保存再删除，重登方下一次请求会从存储
// 重载，不损失数据。
func (m *RoleMgr) Logout(uid uint64, trans cmd_handler.IContext, byServer bool, reason string) error {
	role := m.GetRole(uid)
	if role == nil {
		return nil
	}

	if byServer && reason == LogoutReasonHeartbeatExpired && role.IsOnline() {
		role.Infof("logout skipped: heartbeat expired request raced with a fresh session {reason:%q}", reason)
		return nil
	}

	role.PbRole.LoginInfo.LastLogoutTime = role.Now()
	if err := role.SaveToDB(trans); err != nil {
		role.Errorf("logout save failed, role retained for retry | %v", err)
		return err
	}

	m.DeleteRole(uid)
	return nil
}
