/// 角色管理器

package role

import (
	"context"
	"errors"
	"fmt"
	"sync"

	"github.com/Iori372552686/GoOne/lib/api/cmd_handler"
	"github.com/Iori372552686/GoOne/lib/api/datetime"
	"github.com/Iori372552686/GoOne/lib/api/logger"
	g1_protocol "github.com/Iori372552686/g1_common/protocol"
)

type RoleMgr struct {
	mapUidToRole sync.Map // map[uint64]*Role
}

// -------------------------------- public --------------------------------

func NewRoleMgr() *RoleMgr {
	return &RoleMgr{}
}

func (m *RoleMgr) LoadOrCreate(uid uint64, trans cmd_handler.IContext) *Role {
	return m.obtainRole(uid, trans, true)
}

func (m *RoleMgr) Load(uid uint64, trans cmd_handler.IContext) *Role {
	return m.obtainRole(uid, trans, false)
}

func (m *RoleMgr) GetRole(uid uint64) *Role {
	v, exist := m.mapUidToRole.Load(uid)
	roleInMap, ok := v.(*Role)
	if exist && ok && roleInMap != nil {
		return roleInMap
	}

	return nil
}

func (m *RoleMgr) DeleteRole(uid uint64) {
	m.mapUidToRole.Delete(uid)
}

// PutRole 直接插入/替换内存中的角色对象（测试与装配期注入用）。
// 运行期业务代码应走 LoadOrCreate 的正规加载路径。
func (m *RoleMgr) PutRole(uid uint64, r *Role) {
	m.setRole(uid, r)
}

func (m *RoleMgr) Tick(ctx context.Context) {
	m.removeExpiredRoles(ctx)
}

// FlushAllToDB 同步落盘内存中的全部角色数据（L2 全量 + L3 尽力投递）。
// 用于优雅停机：必须在 TransactionMgr 排空之后调用，保证没有 handler 并发修改角色。
// ctx 透传至 Redis 调用，遵守 Drain 的取消与时间预算（F07）。
// L3 为 one-way 尽力投递：部署顺序上先停 mainsvr 再停 mysqlsvr，由 mysqlsvr
// 的 Drain 排空消费；残余投递失败计入 failed（L2 已持久，重启后自愈补齐）。
func (m *RoleMgr) FlushAllToDB(ctx context.Context) (saved int, failed int) {
	m.mapUidToRole.Range(func(key, value interface{}) bool {
		role, ok := value.(*Role)
		if !ok || role == nil {
			return true
		}
		if err := role.SaveHashSync(ctx); err != nil {
			failed++
		} else if err := role.MaybeFlushL3(true); err != nil {
			failed++
		} else {
			saved++
		}
		return true
	})

	logger.Infof("RoleMgr FlushAllToDB done {saved:%d, failed:%d}", saved, failed)
	return saved, failed
}

// -------------------------------- private --------------------------------

func (m *RoleMgr) setRole(uid uint64, role *Role) {
	m.mapUidToRole.Store(uid, role)
}

// loadRole 读穿加载角色（报告 7.7：结果在前、error 在后的 Go 惯例；
// 旧签名为 (error, *Role) 顺序相反）。
// L2（Redis hash）miss 时经 L3（mysqlsvr role_data）回源并回填 L2——这是
// TTL 过期与 Redis 丢 key 后的数据安全网；L3 也 miss 才视为新角色。
// L3 回源失败（mysqlsvr 不可用）直接报错拒绝加载：宁可不登录，不能拿
// 空数据当新角色把存量覆盖掉。
func loadRole(uid uint64, trans cmd_handler.IContext) (*Role, error) {
	if uid != trans.Uid() {
		logger.Errorf("inconsistent uid {uid:%v, transUid:%v}", uid, trans.Uid())
		return nil, errors.New("inconsistent uid")
	}

	key := fmt.Sprintf("%s:%d", g1_protocol.DBType_DB_TYPE_ROLE.String(), uid)
	info, found, err := roleStore.Load(context.Background(), trans, uid)
	if err != nil {
		logger.Errorf("load role from storage failed {err:%v, uid:%v}", err, uid)
		return nil, err
	}
	if !found {
		logger.Debugf("role not found in storage, creating new {key=%v}", key)
		return nil, nil
	}

	role := Role{}
	role.PbRole = info
	// 这里主要是老的数据添加新增的数据段，不然新数据段就是nil
	role.RoleInitField(info.RegisterInfo.Uid)
	// L2 命中意味着 L3 可能滞后（写回是异步防抖）：置待写标记，
	// 由在线期间的防抖循环自愈补齐；L3 回源命中的数据是新鲜的。
	role.needL3Flush = true
	role.lastL3FlushAt = role.Now()
	return &role, nil
}

func (m *RoleMgr) obtainRole(uid uint64, trans cmd_handler.IContext, createIfNotExist bool) *Role {
	role := m.GetRole(uid)
	if role != nil {
		return role
	}

	createHere := false
	role, err := loadRole(uid, trans)
	if err != nil {
		logger.Errorf("failed to load role {uid:%v} | %v", uid, err)
		return nil
	}

	if role == nil && createIfNotExist { // err==nil && role==nil : 数据库中不存在
		createHere = true
		role = NewRole(uid)
	}

	if role == nil {
		return nil
	}

	roleInMap := m.GetRole(uid)
	if roleInMap != nil {
		return roleInMap
	}
	m.setRole(uid, role)

	// SaveHash 必须放在上面对 mapUidToRole 的二次检测之后，
	// 因为在 loadRole 的过程中，可能已经有其他协程 save 了一个 role，这里不能覆盖它。
	if createHere {
		role.SaveHash(trans)
		role.SaveToMysql(trans)
		// 新角色立即落 L3 快照（force）：崩溃时 L2 若同时丢失，L3 兜底可恢复。
		// 投递失败仅记日志——L2 已写，needL3Flush 标记保留重试。
		if err := role.MaybeFlushL3(true); err != nil {
			role.Errorf("role l3 initial flush failed, retained for retry | %v", err)
		}
	}

	return role
}

// SelfLogoutSender 由 app 装配层注入：把过期角色的登出请求投递回本进程的
// TransactionMgr（CMD_MAIN_LOGOUT_REQ），使保存与删除按 uid 串行键与业务
// handler 串行执行，避免 Tick 协程与 handler 并发读写同一 *Role。
var SelfLogoutSender func(uid uint64, zone uint32, req *g1_protocol.LogoutReq)

// 删除内存中没有心跳的角色数据（F06：过期判定/踢人/保存/删除收敛到 UID 串行域）。
//
// 本函数运行在 Tick 调度协程，只做两件事：
//  1. 只读原子心跳快照（heartbeatSnapshot）筛选过期候选——不读 PbRole 字段，
//     与业务 handler 的心跳写入无数据竞争；
//  2. 经 SelfLogoutSender 把过期登出投递到该 uid 的事务串行队列。
//
// 权威过期复检、踢连接、保存与删除在 Logout 用例（session.go）内于 UID 串行域
// 完成：排队期间玩家合法重连刷新心跳后，旧过期任务在复检处放行跳过。
// HeartBeatExpiryTime 仅由本协程读写，用于防止重复投递。
func (m *RoleMgr) removeExpiredRoles(ctx context.Context) {
	now := datetime.Now()
	expiryThreshold := int32(60 * 2)

	type candidate struct {
		uid  uint64
		zone uint32
	}
	var candidates []candidate

	m.mapUidToRole.Range(func(key, value interface{}) bool {
		role, ok := value.(*Role)
		if !ok || role == nil {
			return true
		}
		if now-int32(role.heartbeatSnapshot()) > expiryThreshold &&
			now > role.HeartBeatExpiryTime+1 {
			role.HeartBeatExpiryTime = now
			candidates = append(candidates, candidate{uid: role.Uid(), zone: role.Zone()})
		}
		return true
	})

	for _, c := range candidates {
		logger.Infof("logout queued for heartbeat expired {uid:%v}", c.uid)

		if SelfLogoutSender != nil {
			SelfLogoutSender(c.uid, c.zone, &g1_protocol.LogoutReq{
				ByServer: true,
				Reason:   LogoutReasonHeartbeatExpired,
			})
			continue
		}

		// 兜底路径（未注入时）：保存成功才删除；失败保留角色由下一轮 Tick 重试
		//（F04：保存失败不再无条件丢弃内存状态）。
		if role := m.GetRole(c.uid); role != nil {
			if err := role.SaveHashSync(ctx); err != nil {
				logger.Errorf("failed to save expired role, retained for retry {uid:%v} | %v", c.uid, err)
				continue
			}
		}
		m.mapUidToRole.Delete(c.uid)
	}
}
