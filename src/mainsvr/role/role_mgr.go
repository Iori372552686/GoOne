/// 角色管理器

package role

import (
	"GoOne/lib/api/cmd_handler"
	"GoOne/lib/api/datetime"
	"GoOne/lib/api/logger"
	"GoOne/lib/service/router"
	"sync"

	g1_protocol "GoOne/protobuf/protocol"

	"github.com/golang/glog"
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
	/*	if uid != trans.Uid() {
			glog.Errorf("inconsistent uid {uid:%v, transUid:%v}", uid, trans.Uid())
			return errors.New("inconsistent uid"), nil
		}

		req := g1_protocol.DBUidGetReq{}
		rsp := g1_protocol.DBUidGetRsp{}
		req.DbType = uint32(g1_protocol.DBType_DB_TYPE_ROLE)
		req.Uid = uid
		err := trans.CallMsgBySvrType(misc.ServerType_DBSvr, uint32(g1_protocol.CMD_DB_INNER_UID_GET_REQ), &req, &rsp)
		if err != nil {
			return err, nil
		}

		ret := rsp.Ret.Ret
		switch ret {
		case int32(g1_protocol.ErrorCode_ERR_NOT_EXIST):
			return nil, nil
		default:
			return fmt.Errorf("rsp error {ret:%v}", ret), nil
		case 0:
		}

		role := Role{}
		role.PbRole = new(g1_protocol.RoleInfo)
		err = proto.Unmarshal(rsp.Data, role.PbRole)
		if err != nil {
			glog.Error(err)
			return err, nil
		}
		// 这里主要是老的数据添加新增的数据段，不然新数据段就是nil
		role.RoleInitField(role.PbRole.RegisterInfo.Uid)
		return nil, &role*/
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
		glog.Errorf("failed to load role {uid:%v} | %v", uid, err)
		return nil
	}
	if role == nil && createIfNotExist { // err==nil && role==nil : 数据库中不存在
		createHere = true
		role = NewRole(uid)
		role.Lock()
		defer role.Unlock()
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
		_ = role.SaveToDB(trans)
		_ = role.SaveToMysql(trans)
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
		_ = router.SendPbMsgByBusIdSimple(busIdList[i], uid, uint32(g1_protocol.CMD_CONN_KICK_OUT_REQ), &req)
	}

	// 从map里面删除超时的role
	for _, uid := range expiredUidList {
		m.mapUidToRole.Delete(uid)
	}
}
