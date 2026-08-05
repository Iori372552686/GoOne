/// 角色管理器

package role

import (
	"errors"
	"fmt"
	"sync"

	connsvrv1 "github.com/Iori372552686/GoOne/api/gen/game/connsvr/v1"
	"github.com/Iori372552686/GoOne/lib/api/cmd_handler"
	"github.com/Iori372552686/GoOne/lib/api/datetime"
	"github.com/Iori372552686/GoOne/lib/api/logger"
	g1_protocol "github.com/Iori372552686/game_protocol/protocol"
)

type RoleMgr struct {
	mapUidToRole sync.Map // map[uint64]*Role
}

// -------------------------------- public --------------------------------

func NewRoleMgr() *RoleMgr {
	return &RoleMgr{}
}

func (m *RoleMgr) GetOrLoadOrCreateRole(uid uint64, trans cmd_handler.IContext) *Role {
	return m.obtainRole(uid, trans, true)
}

func (m *RoleMgr) GetOrLoadRole(uid uint64, trans cmd_handler.IContext) *Role {
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

func (m *RoleMgr) Tick() {
	m.removeExpiredRoles()
}

// FlushAllToDB 同步落盘内存中的全部角色数据。
// 用于优雅停机：必须在 TransactionMgr 排空之后调用，保证没有 handler 并发修改角色。
func (m *RoleMgr) FlushAllToDB() (saved int, failed int) {
	m.mapUidToRole.Range(func(key, value interface{}) bool {
		role, ok := value.(*Role)
		if !ok || role == nil {
			return true
		}
		if err := role.SaveToDBSync(); err != nil {
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

func loadRole(uid uint64, trans cmd_handler.IContext) (error, *Role) {
	if uid != trans.Uid() {
		logger.Errorf("inconsistent uid {uid:%v, transUid:%v}", uid, trans.Uid())
		return errors.New("inconsistent uid"), nil
	}

	key := fmt.Sprintf("%s:%d", g1_protocol.DBType_DB_TYPE_ROLE.String(), uid)
	info, err := loadRoleHash(uid)
	if err != nil {
		logger.Errorf("get redis error {err:%v, uid:%v}", err, uid)
		return err, nil
	}
	if info == nil {
		logger.Debugf("get role redis nil {key=%v}", key)
		return nil, nil
	}

	role := Role{}
	role.PbRole = info
	// 这里主要是老的数据添加新增的数据段，不然新数据段就是nil
	role.RoleInitField(info.RegisterInfo.Uid)
	return nil, &role
}

func (m *RoleMgr) obtainRole(uid uint64, trans cmd_handler.IContext, createIfNotExist bool) *Role {
	role := m.GetRole(uid)
	if role != nil {
		return role
	}

	createHere := false
	err, role := loadRole(uid, trans)
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

	// SaveToDB必须放在上面对mapUidToRole的二次检测之后，
	// 因为在loadRole的过程中，可能已经有其他协程save了一个role，这里不能覆盖它。
	if createHere {
		role.SaveToDB(trans)
		role.SaveToMysql(trans)
	}

	return role
}

// SelfLogoutSender 由 app 装配层注入：把过期角色的登出请求投递回本进程的
// TransactionMgr（CMD_MAIN_LOGOUT_REQ），使保存与删除按 uid 串行键与业务
// handler 串行执行，避免 Tick 协程与 handler 并发读写同一 *Role。
var SelfLogoutSender func(uid uint64, zone uint32, req *g1_protocol.LogoutReq)

// 删除内存中没有心跳的角色数据。
// 本函数运行在 Tick 协程：只做过期检测与踢人 RPC，角色的落盘与删除
// 通过 SelfLogoutSender 交由事务串行执行（Logout handler 内完成）。
func (m *RoleMgr) removeExpiredRoles() {
	now := datetime.Now()
	expiredUidList := make([]uint64, 0)
	busIdList := make([]uint32, 0)
	zoneList := make([]uint32, 0)

	expiryThreshold := 60 * 2
	m.mapUidToRole.Range(func(key, value interface{}) bool {
		role, ok := value.(*Role)
		if ok && role != nil && now-role.PbRole.LoginInfo.LastHartBeatTime > int32(expiryThreshold) &&
			now > role.HeartBeatExpiryTime+1 {
			expiredUidList = append(expiredUidList, role.Uid())
			busIdList = append(busIdList, role.PbRole.ConnSvrInfo.BusId)
			zoneList = append(zoneList, role.Zone())
			// HeartBeatExpiryTime 仅由本 Tick 协程读写，用于防止重复投递。
			role.HeartBeatExpiryTime = now
		}
		return true
	})

	connClient := connsvrv1.NewConnServiceClient()
	for i, uid := range expiredUidList {
		logger.Infof("Logout for heartbeat expired {uid:%v}", uid)

		req := g1_protocol.ConnKickOutReq{}
		req.Reason = g1_protocol.EKickOutReason_HEARTBEAT_TIMEOUT
		_ = connClient.KickOutByBusIdSimple(busIdList[i], uid, &req)

		if SelfLogoutSender != nil {
			SelfLogoutSender(uid, zoneList[i], &g1_protocol.LogoutReq{
				ByServer: true,
				Reason:   "heartbeat expired",
			})
			continue
		}

		// 兜底路径（未注入时）：同步保存后删除，保持旧行为但不再 fire-and-forget。
		if role := m.GetRole(uid); role != nil {
			if err := role.SaveToDBSync(); err != nil {
				logger.Errorf("failed to save expired role {uid:%v} | %v", uid, err)
			}
		}
		m.mapUidToRole.Delete(uid)
	}
}
