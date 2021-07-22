package role

import (
	"fmt"
	"strconv"
	"sync"

	`GoOne/common/misc`
	`GoOne/common/module/datetime`
	`GoOne/lib/cmd_handler`
	`GoOne/lib/logger`
	`GoOne/lib/router`
	g1_protocol `GoOne/protobuf/protocol`

	"github.com/golang/glog"
)

type Role struct {
	sync.Mutex // Role的锁交由外部来控制
	PbRole     *g1_protocol.RoleInfo

	// 下面可以添加临时内存数据，不会持久化到数据库
	HeartBeatExpiryTime int32
	HeartBeatCount      int32
}

func NewRole(uid uint64) *Role {
	role := new(Role)

	role.PbRole = new(g1_protocol.RoleInfo)
	role.RoleInitField(uid)
	// TODO add test item
	//role.ItemAdd(1201001, 5, &Reason{REASON_INIT, 0})
	//role.ItemAdd(1201002, 5, &Reason{REASON_INIT, 0})
	//role.ItemAdd(1201003, 5, &Reason{REASON_INIT, 0})


	role.OnRoleCreate()
	return role
}

// Role的初始化函数 新增成员以后记得加上去
func (r *Role) RoleInitField(uid uint64) {
	now := datetime.Now()
	if r.PbRole.ConnSvrInfo == nil {
		r.PbRole.ConnSvrInfo = &g1_protocol.ConnSvrInfo{}
	}
	if r.PbRole.RegisterInfo == nil {
		r.PbRole.RegisterInfo = &g1_protocol.RoleRegisterInfo{}
		r.PbRole.RegisterInfo.Uid = uid
		r.PbRole.RegisterInfo.RegisterTime = now
	}
	if r.PbRole.LoginInfo == nil {
		r.PbRole.LoginInfo = &g1_protocol.RoleLoginInfo{}
	}
	if r.PbRole.DescInfo == nil {
		r.PbRole.DescInfo = &g1_protocol.RoleDescInfo{}
		r.PbRole.DescInfo.Name = "游客" + strconv.FormatInt(int64(uid), 10)
		//r.PbRole.DescInfo.FrameId = gamedata.Const.DefaultFrame()
		//r.PbRole.DescInfo.IconId = gamedata.Const.DefaultIcon()
		//r.PbRole.DescInfo.ImageId = gamedata.Const.DefaultImage()
		r.PbRole.DescInfo.FreeCnt = 1
	}
	if r.PbRole.BasicInfo == nil {
		r.PbRole.BasicInfo = &g1_protocol.RoleBasicInfo{}
		r.PbRole.BasicInfo.Level = 1
	}
	if r.PbRole.MallInfo == nil {
		r.PbRole.MallInfo = &g1_protocol.RoleMallInfo{}
	}
	if r.PbRole.MainTaskInfo == nil {
		r.PbRole.MainTaskInfo = &g1_protocol.RoleMainTaskInfo{}
	}
	if r.PbRole.OpenFunInfo == nil {
		r.PbRole.OpenFunInfo = &g1_protocol.RoleOpenFunction{}
		r.PbRole.OpenFunInfo.IsAllOpen = true
	}

}

func (r *Role) Now() int32 {
	return datetime.Now() + r.PbRole.RegisterInfo.TimeOffsetMinute*datetime.SECONDS_PER_MINUTE
}

func (r *Role) NowMs() int64 {
	return datetime.NowMs() + int64(r.PbRole.RegisterInfo.TimeOffsetMinute*datetime.SECONDS_PER_MINUTE)*datetime.MS_PER_SECOND
}

// 日志
func (r *Role) Errorf(format string, args ...interface{}) {
	f := fmt.Sprintf("[%v|%v] %v", r.Uid(), 0, format)
	glog.ErrorDepth(1, fmt.Sprintf(f, args...))
}

func (r *Role) Warningf(format string, args ...interface{}) {
	f := fmt.Sprintf("[%v|%v] %v", r.Uid(), 0, format)
	glog.WarningDepth(1, fmt.Sprintf(f, args...))
}

func (r *Role) Infof(format string, args ...interface{}) {
	f := fmt.Sprintf("[%v|%v] %v", r.Uid(), 0, format)
	glog.InfoDepth(1, fmt.Sprintf(f, args...))
}

func (r *Role) Debugf(format string, args ...interface{}) {
	r.DebugDepthf(1, format, args...)
}
func (r *Role) DebugDepthf(depth int, format string, args ...interface{}) {
	f := fmt.Sprintf("[%v|%v] %v", r.Uid(), 0, format)
	logger.DebugDepthf(1+depth, f, args...)
}

func (r *Role) Uid() uint64 {
	return r.PbRole.RegisterInfo.Uid
}

func (r *Role) Zone() int32 {
	return r.PbRole.RegisterInfo.Zone
}

func (r *Role) SaveToDB(trans cmd_handler.IContext) error {
/*	if r.Uid() != trans.Uid() {
		r.Errorf("inconsistent uid {roleUid:%v, transUid:%v}", r.Uid(), trans.Uid())
		return errors.New("inconsistent uid")
	}

	data, err := proto.Marshal(r.PbRole)
	if err != nil {
		r.Errorf("marshaling error {role:%v} | %v", r.PbRole, err)
		return err
	}

	req := g1_protocol.DBUidSetReq{}
	rsp := g1_protocol.DBUidSetRsp{}
	req.Uid = r.Uid()
	req.DbType = uint32(g1_protocol.DBType_DB_TYPE_ROLE)
	req.Data = data
	err = trans.CallMsgBySvrType(misc.ServerType_DBSvr, uint32(g1_protocol.CMD_DB_INNER_UID_SET_REQ), &req, &rsp)
	if err != nil {
		return err
	}

	if rsp.Ret.Ret != 0 {
		return fmt.Errorf("save role response error {ret:%v}", rsp.Ret.Ret)
	}
*/
	return nil
}

func (r *Role) SaveToMysql(trans cmd_handler.IContext) error {
	req := g1_protocol.MysqlInnerUpdateRoleInfoReq{}
	rsp := g1_protocol.MysqlInnerUpdateRoleInfoRsp{}
	req.Name = r.PbRole.DescInfo.Name
	r.Infof("update mysql")
	err := trans.CallMsgBySvrType(misc.ServerType_MysqlSvr, uint32(g1_protocol.CMD_MYSQL_INNER_UPDATE_ROLE_INFO_REQ), &req, &rsp)
	if err != nil {
		return err
	}
	r.Infof("update mysql")

	if rsp.Ret.Ret != 0 {
		r.Errorf("save role to mysql error {ret:%v}", rsp.Ret.Ret)
	}
	r.Infof("update mysql")
	return nil
}

// 保存玩家数据，不等待返回结果。只在特殊情况下使用。比如：RoleMgr::removeExpiredRoles中
func (r *Role) SaveToDBIgnoreRsp() {
/*	data, err := proto.Marshal(r.PbRole)
	if err != nil {
		r.Errorf("Failed to marshal role when removing expired role {role:%v} | %v", r.PbRole, err)
		return
	}

	req := g1_protocol.DBUidSetReq{}
	req.Uid = r.Uid()
	req.DbType = uint32(g1_protocol.DBType_DB_TYPE_ROLE)
	req.IgnoreRsp = true
	req.Data = data
	err = router.SendPbMsgBySvrTypeSimple(misc.ServerType_DBSvr, r.Uid(), uint32(g1_protocol.CMD_DB_INNER_UID_SET_REQ), &req)
	if err != nil {
		r.Errorf("Failed to saveToDBIgnoreRsp {role:%v} | %v", r.PbRole, err)
		return
	}*/
}

func (r *Role) OnRoleCreate() {

}

// 这里服务端主动同步数据到客户端
// 理论上每次玩家数据有变动，都要将相应的数据段同步过去
// 通过Flag控制同步有改变的数据段，减少数据的同步量
func (r *Role) SyncDataToClient(dataFlag g1_protocol.ERoleSectionFlag) error {
	connsvrBusId := r.PbRole.ConnSvrInfo.BusId
	if connsvrBusId == 0 {
		logger.Errorf("connect svr bus id 0")
		return fmt.Errorf("the player are not online")
	}

	data := g1_protocol.ScSyncUserData{}
	data.RoleInfo = new(g1_protocol.RoleInfo)

	if dataFlag&g1_protocol.ERoleSectionFlag_REGISTER_INFO != 0 {
		data.RoleInfo.RegisterInfo = r.PbRole.RegisterInfo
	}
	if dataFlag&g1_protocol.ERoleSectionFlag_LOGIN_INFO != 0 {
		data.RoleInfo.LoginInfo = r.PbRole.LoginInfo
	}
	if dataFlag&g1_protocol.ERoleSectionFlag_DESC_INFO != 0 {
		data.RoleInfo.DescInfo = r.PbRole.DescInfo
	}
	if dataFlag&g1_protocol.ERoleSectionFlag_BASIC_INFO != 0 {
		data.RoleInfo.BasicInfo = r.PbRole.BasicInfo
	}
	if dataFlag&g1_protocol.ERoleSectionFlag_INVENTORY_INFO != 0 {
		data.RoleInfo.InventoryInfo = r.PbRole.InventoryInfo
	}
	if dataFlag&g1_protocol.ERoleSectionFlag_ICON_INFO != 0 {
		data.RoleInfo.IconInfo = r.PbRole.IconInfo
	}
	if dataFlag&g1_protocol.ERoleSectionFlag_MALL_INFO != 0 {
		data.RoleInfo.MallInfo = r.PbRole.MallInfo
	}
	if dataFlag&g1_protocol.ERoleSectionFlag_MAIN_TASK_INFO != 0 {
		data.RoleInfo.MainTaskInfo = r.PbRole.MainTaskInfo
	}
	if dataFlag&g1_protocol.ERoleSectionFlag_GUILD_INFO != 0 {
		data.RoleInfo.GuildInfo = r.PbRole.GuildInfo
	}
	if dataFlag&g1_protocol.ERoleSectionFlag_GUIDE_INFO != 0 {
		data.RoleInfo.GuideInfo = r.PbRole.GuideInfo
	}
	if dataFlag&g1_protocol.ERoleSectionFlag_OPEN_FUNC_INFO != 0 {
		data.RoleInfo.OpenFunInfo = r.PbRole.OpenFunInfo
	}
	if dataFlag&g1_protocol.ERoleSectionFlag_ACTVITY_TASK_INFO != 0 {
		data.RoleInfo.Actvity_Info = r.PbRole.Actvity_Info
	}

	r.Infof("sync: %v", data.String())
	return router.SendPbMsgByBusIdSimple(connsvrBusId, r.Uid(), uint32(g1_protocol.CMD_SC_SYNC_USER_DATA), &data)
}

func (r *Role) OnLogin(now int32) {
	r.ExpAdd(0)
	r.SyncOpenFuncData()
}

func (r *Role) AfterLogin(now int32) {
	r.LoginByTaskCheck()
}

func (r *Role) LoginByTaskCheck() {
}

// 客户端会每隔几秒发送一次心跳包到服务器
// 服务器根据时间来驱动玩家的周期性事件
func (r *Role) OnClientHeartbeat(now int32) {
	lastClientHeartBeatTime := r.PbRole.LoginInfo.LastHartBeatTime

	syncFlag := g1_protocol.ERoleSectionFlag(0)
	if !datetime.IsSameMinute(lastClientHeartBeatTime, now) {
		syncFlag |= r.everyMinuteCheck(now)
	}
	if !datetime.IsSameHour(lastClientHeartBeatTime, now) {
		syncFlag |= r.everyHourCheck(lastClientHeartBeatTime,now)
	}
	// 早上0点的刷新
	if !datetime.IsSameDay(lastClientHeartBeatTime, now) {
		syncFlag |= r.everyDayCheck(now)
	}
	// 晚上9点的刷新
	if datetime.IsSameDayByDayBeginHour(lastClientHeartBeatTime, now, 21) {
		syncFlag |= r.everyDayCheck21(now)
	}
	if !datetime.IsSameWeek(lastClientHeartBeatTime, now) {
		syncFlag |= r.everyWeakCheck(now)
	}

	// 10秒一次心跳
	if r.HeartBeatCount%1 == 0 {
		//r.Debugf("update brief info, %d", r.Now())
		_ = r.UpdateBriefInfo()
	}

	if syncFlag > 0 {
		_ = r.SyncDataToClient(syncFlag)
	}

	r.HeartBeatCount++
	r.PbRole.LoginInfo.LastHartBeatTime = now
}

// 返回同步数据的Flag
func (r *Role) everyMinuteCheck(now int32) g1_protocol.ERoleSectionFlag {
	return 0
}

func (r *Role) everyHourCheck(last,now int32) g1_protocol.ERoleSectionFlag {
	//weekday:= datetime.GetDayOfWeek(now)
	//,_:= datetime.GetHourMinuteForTime(now)

	return g1_protocol.ERoleSectionFlag_ACTVITY_TASK_INFO
}

func (r *Role) everyDayCheck21(now int32) g1_protocol.ERoleSectionFlag {

	return 0
}

func (r *Role) everyDayCheck(now int32) g1_protocol.ERoleSectionFlag {
	logger.Debugf("on daily clear")
	//day := datetime.GetDayOfMonth(now)
	//hour,_:= datetime.GetHourMinuteForTime(now)

	r.MallDailyRefresh()

	return	g1_protocol.ERoleSectionFlag_MALL_INFO |
		g1_protocol.ERoleSectionFlag_ACTVITY_TASK_INFO
}

func (r *Role) everyWeakCheck(now int32) g1_protocol.ERoleSectionFlag {
	return 0
}



// 玩家的简要信息，一般用于展示给其它玩家查看
func (r *Role) GetBriefInfo() *g1_protocol.PbRoleBriefInfo {
	info := &g1_protocol.PbRoleBriefInfo{}

	info.Uid = r.Uid()
	info.Name = r.PbRole.DescInfo.Name
	info.Level = r.PbRole.BasicInfo.Level
	info.Exp = r.PbRole.BasicInfo.Exp
	info.Icon = r.PbRole.DescInfo.IconId
	info.Frame = r.PbRole.DescInfo.FrameId
	info.RegisterTime = r.PbRole.RegisterInfo.RegisterTime

	info.LastOnlineTime = r.Now()
	info.ConnBusId = r.PbRole.ConnSvrInfo.BusId
	return info
}

// 更新玩家简要信息到数据库
func (r *Role) UpdateBriefInfo() error {
	req := g1_protocol.InfoSetBriefInfoReq{}
	req.Uid = r.Uid()
	req.Info = r.GetBriefInfo()
	req.IgnoreRsp = true

	return router.SendPbMsgBySvrTypeSimple(misc.ServerType_InfoSvr, r.Uid(), uint32(g1_protocol.CMD_INFO_INNER_SET_BRIEF_INFO_REQ), &req)
}

func (r *Role) ExpAdd(exp int32) {
}

func (r *Role) IsOnline() bool {
	return r.PbRole.LoginInfo.LastHartBeatTime+2*30 > datetime.Now()
}

