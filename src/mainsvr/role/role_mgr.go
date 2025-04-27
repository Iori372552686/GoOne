/// 角色管理器

package role

import (
	"errors"
	"fmt"
	"sync"

	"github.com/Iori372552686/GoOne/lib/api/cmd_handler"
	"github.com/Iori372552686/GoOne/lib/api/datetime"
	"github.com/Iori372552686/GoOne/lib/api/logger"
	"github.com/Iori372552686/GoOne/lib/service/router"
	"github.com/Iori372552686/GoOne/src/mainsvr/globals/rds"
	g1_protocol "github.com/Iori372552686/game_protocol"
	"github.com/golang/protobuf/proto"
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

// -------------------------------- private --------------------------------

func (m *RoleMgr) setRole(uid uint64, role *Role) {
	m.mapUidToRole.Store(uid, role)
}

func loadRole(uid uint64, trans cmd_handler.IContext) (error, *Role) {
	if uid != trans.Uid() {
		logger.Errorf("inconsistent uid {uid:%v, transUid:%v}", uid, trans.Uid())
		return errors.New("inconsistent uid"), nil
	}

	dbType := uint32(g1_protocol.DBType_DB_TYPE_ROLE)
	key := fmt.Sprintf("%s:%d", g1_protocol.DBType_DB_TYPE_ROLE.String(), uid)
	logger.Debugf("get redis for uid {key=%s}", key)
	result, err := rds.RedisMgr.GetBytes(dbType, key)
	if err != nil {
		logger.Errorf("get redis error {err:%v, dbType:%v, uid:%v}", err, dbType, uid)
		return err, nil
	}

	if result == nil {
		logger.Debugf("get role redis nil {key=%v}", key)
		return nil, nil
	}

	role := Role{}
	role.PbRole = new(g1_protocol.RoleInfo)
	err = proto.Unmarshal(result, role.PbRole)
	if err != nil {
		logger.Error(err)
		return err, nil
	}

	// 这里主要是老的数据添加新增的数据段，不然新数据段就是nil
	role.RoleInitField(role.PbRole.RegisterInfo.Uid)
	return nil, &role
	return nil, nil
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
		//role.SaveToMysql(trans) 双写
	}

	return role
}

// 删除内存中没有心跳的角色数据
func (m *RoleMgr) removeExpiredRoles() {
	now := datetime.Now()
	expiredUidList := make([]uint64, 0)
	busIdList := make([]uint32, 0)

	expiryThreshold := 60 * 2
	m.mapUidToRole.Range(func(key, value interface{}) bool {
		role, ok := value.(*Role)
		if ok && role != nil && now-role.PbRole.LoginInfo.LastHartBeatTime > int32(expiryThreshold) &&
			now > role.HeartBeatExpiryTime+1 {
			expiredUidList = append(expiredUidList, role.Uid())
			busIdList = append(busIdList, role.PbRole.ConnSvrInfo.BusId)
			role.HeartBeatExpiryTime = now
			role.SaveToDBIgnoreRsp() // 保存一遍数据
		}
		return true
	})

	for i, uid := range expiredUidList {
		logger.Infof("Logout for heartbeat expired {uid:%v}", uid)

		req := g1_protocol.ConnKickOutReq{}
		req.Reason = g1_protocol.EKickOutReason_HEARTBEAT_TIMEOUT
		_ = router.SendPbMsgByBusIdSimple(busIdList[i], uid, g1_protocol.CMD_CONN_KICK_OUT_REQ, &req)
	}

	// 从map里面删除超时的role
	for _, uid := range expiredUidList {
		m.mapUidToRole.Delete(uid)
	}
}
