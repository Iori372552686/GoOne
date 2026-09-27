package role

import (
	connsvrv1 "github.com/Iori372552686/GoOne/api/gen/game/connsvr/v1"
	"github.com/Iori372552686/GoOne/lib/api/cmd_handler"
	g1_protocol "github.com/Iori372552686/g1_common/protocol"
)

// 登出原因字面量。与发送方保持一致：lib/net/net_mgr 三传输在连接关闭时发送
// "disconnect"；RoleMgr 心跳过期投递 "heartbeat expired"。两侧字符串必须同步。
const (
	LogoutReasonDisconnect       = "disconnect"
	LogoutReasonHeartbeatExpired = "heartbeat expired"
)

// kickExpiredSession 在 UID 串行域内确认过期后踢掉对应连接（F06：原实现在
// Tick 协程未复检即踢，排队期间重连的新会话可能被旧过期任务误踢）。
// busId 未记录（离线后重载等）时跳过——连接本就不在。
// RemoteAddr 取 ConnSvrInfo.ClientPos，使 connsvr 精确定位会话并按其传输踢出。
func kickExpiredSession(r *Role) {
	connInfo := r.PbRole.ConnSvrInfo
	if connInfo == nil || connInfo.BusId == 0 {
		return
	}
	req := &g1_protocol.ConnKickOutReq{
		Reason:     g1_protocol.EKickOutReason_HEARTBEAT_TIMEOUT,
		RemoteAddr: connInfo.ClientPos,
	}
	if err := connsvrv1.NewConnServiceClient().KickOutByBusIdSimple(connInfo.BusId, r.Uid(), req); err != nil {
		r.Errorf("kick expired session failed | %v", err)
	}
}

// Logout 收敛"保存并退出"用例（F04）：完整拥有停写、保存、移除的语义，
// 调用方（协议 handler）只做协议转换，不再自行拼接 SaveToDB + DeleteRole。
//
// 契约：
//   - 角色不在内存：幂等成功（重复退出不会二次删除）。
//   - 保存失败：返回错误，角色与未清的 persistDirtyMask 保留在内存——由既有
//     10s 防抖持久化与 role_tick 过期扫描重试；成功保存前退出未完成。
//   - 保存成功：从内存移除，仅此一次。
//
// 心跳过期路径（F06）：本用例运行于 UID 串行域，复检 PbRole.LastHartBeatTime
// 权威判定过期——新鲜（排队期间已重新登录）则跳过，不踢不删；确认过期则先踢
// 连接再走保存/删除。disconnect 路径不设此守卫：断开登出即使迟到也先保存再
// 删除，重登方下一次请求会从存储重载，不损失数据。
func (m *RoleMgr) Logout(uid uint64, trans cmd_handler.IContext, byServer bool, reason string) error {
	role := m.GetRole(uid)
	if role == nil {
		return nil
	}

	if byServer && reason == LogoutReasonHeartbeatExpired {
		if role.IsOnline() {
			role.Infof("logout skipped: heartbeat expired request raced with a fresh session {reason:%q}", reason)
			return nil
		}
		kickExpiredSession(role)
	}

	role.PbRole.LoginInfo.LastLogoutTime = role.Now()
	if err := role.SaveHash(trans); err != nil {
		role.Errorf("logout save failed, role retained for retry | %v", err)
		return err
	}
	// 登出是角色的最后一次变更机会：强制投递 L3 快照（尽力而为）。
	// 失败不阻断登出——L2 已持久、needL3Flush 保留，重登/role_tick 自愈补齐。
	if err := role.MaybeFlushL3(true); err != nil {
		role.Errorf("logout l3 flush deferred for retry | %v", err)
	}

	m.DeleteRole(uid)
	return nil
}
