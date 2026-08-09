package role

import (
	"errors"
	"fmt"

	infosvrv1 "github.com/Iori372552686/GoOne/api/gen/game/infosvr/v1"
	mysqlsvrv1 "github.com/Iori372552686/GoOne/api/gen/game/mysqlsvr/v1"

	"sync"

	"github.com/Iori372552686/GoOne/lib/api/cmd_handler"
	"github.com/Iori372552686/GoOne/lib/api/datetime"
	"github.com/Iori372552686/GoOne/lib/api/logger"
	"github.com/Iori372552686/GoOne/lib/util/convert"
	g1_protocol "github.com/Iori372552686/g1_common/protocol"
)

type Role struct {
	sync.Mutex // Role的锁交由外部来控制
	PbRole     *g1_protocol.RoleInfo

	// 下面可以添加临时内存数据，不会持久化到数据库
	HeartBeatExpiryTime int32
	HeartBeatCount      int32

	pendingFullSyncMask g1_protocol.ERoleSectionFlag
	pendingPatchMask    g1_protocol.ERoleSectionFlag

	inventoryUpserts int32Set
	inventoryDeletes int32Set
	mallUpserts      int32Set
	mallDeletes      int32Set
	iconUpserts      int32Set
	iconDeletes      int32Set
	frameUpserts     int32Set
	frameDeletes     int32Set
	actTaskUpserts   int32Set
	actTaskDeletes   int32Set
	iconEquipDirty   bool

	needPersist       bool
	persistDirtySince int32
	lastPersistAt     int32
	persistReasons    stringSet
	// persistDirtyMask 记录自上次成功落盘后哪些模块变更过，供 hash 模式增量写。
	// full 模式不读取此字段。落盘成功后清零。
	persistDirtyMask g1_protocol.ERoleSectionFlag
}

func NewRole(uid uint64) *Role {
	role := new(Role)

	role.PbRole = new(g1_protocol.RoleInfo)
	role.RoleInitField(uid)
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
	if r.PbRole.GameInfo == nil {
		r.PbRole.GameInfo = &g1_protocol.RoleGameInfo{}
		r.PbRole.GameInfo.PlayRoomIds = make([]uint64, 0)
	}
	if r.PbRole.BasicInfo == nil {
		r.PbRole.BasicInfo = &g1_protocol.RoleBasicInfo{}
		r.PbRole.BasicInfo.Level = 1
		r.PbRole.BasicInfo.Name = "Player" + convert.Int64ToString(int64(uid))
		//r.PbRole.DescInfo.FrameId = gamedata.Const.DefaultFrame()
		//r.PbRole.DescInfo.ImageId = gamedata.Const.DefaultImage()
		r.PbRole.BasicInfo.Gold = 1000000
		r.PbRole.BasicInfo.Diamond = 10000
		r.PbRole.BasicInfo.AceCoin = 100000
		r.PbRole.BasicInfo.WinAceCoin = 20000
		r.PbRole.BasicInfo.Credit = 10000
		r.PbRole.BasicInfo.FreeCnt = 1
	}
	if r.PbRole.IconInfo == nil {
		r.PbRole.IconInfo = &g1_protocol.RoleIconInfo{}
		//r.PbRole.IconInfo.FrameId = ConstConfig.MGetByName("DefaultFrame").Value //gamedata.Const.DefaultFrame()
		r.PbRole.IconInfo.IconUrl = "headicon_" + convert.Int64ToString(int64(uid)%31)
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
	if r.PbRole.InventoryInfo == nil {
		r.PbRole.InventoryInfo = &g1_protocol.RoleInventoryInfo{}
		r.PbRole.InventoryInfo.ItemMap = make(map[int32]*g1_protocol.PbItem)
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
	logger.ErrorDepthf(1, fmt.Sprintf(f, args...))
}

func (r *Role) Warningf(format string, args ...interface{}) {
	f := fmt.Sprintf("[%v|%v] %v", r.Uid(), 0, format)
	logger.WarningDepthf(1, fmt.Sprintf(f, args...))
}

func (r *Role) Infof(format string, args ...interface{}) {
	f := fmt.Sprintf("[%v|%v] %v", r.Uid(), 0, format)
	logger.InfoDepthf(1, fmt.Sprintf(f, args...))
}

func (r *Role) Debugf(format string, args ...interface{}) {
	r.DebugDepthf(1, format, args...)
}
func (r *Role) DebugDepthf(depth int, format string, args ...interface{}) {
	if !logger.DebugEnabled() {
		return
	}
	f := fmt.Sprintf("[%v|%v] %v", r.Uid(), r.Zone(), format)
	logger.DebugDepthf(1+depth, f, args...)
}

func (r *Role) Uid() uint64 {
	return r.PbRole.RegisterInfo.Uid
}

func (r *Role) Zone() uint32 {
	return uint32(r.PbRole.RegisterInfo.Zone)
}

func (r *Role) SaveToDB(trans cmd_handler.IContext) error {
	if r.Uid() != trans.Uid() {
		r.Errorf("inconsistent uid {roleUid:%v, transUid:%v}", r.Uid(), trans.Uid())
		return errors.New("inconsistent uid")
	}

	// 按模块增量写 Redis hash（落盘格式详见 persist_hash.go）。
	if err := saveRoleHash(r, false); err != nil {
		return err
	}
	r.clearPersistDirtyMask()
	r.Debugf("role SaveToDB(hash) success | uid:%v", r.Uid())
	return nil
}

func (r *Role) SaveToMysql(trans cmd_handler.IContext) error {
	req := g1_protocol.MysqlInnerUpdateRoleInfoReq{}
	req.Name = r.PbRole.BasicInfo.Name
	r.Infof("update mysql")
	rsp, err := mysqlsvrv1.NewMysqlServiceClient().UpdateRoleInfo(trans, &req)
	if err != nil {
		return err
	}
	r.Infof("update mysql")

	if rsp == nil || rsp.Ret == nil {
		r.Errorf("save role to mysql error {ret:nil}")
		return nil
	}
	if rsp.Ret.Code != 0 {
		r.Errorf("save role to mysql error {ret:%v}", rsp.Ret.Code)
	}
	r.Infof("update mysql")
	return nil
}

// SaveToDBSync 同步持久化角色数据到 redis，不依赖事务上下文。
// 用于优雅停机等没有 transaction 可用的场景。force=true 全量写所有模块。
func (r *Role) SaveToDBSync() error {
	if err := saveRoleHash(r, true); err != nil {
		return err
	}
	r.needPersist = false
	r.persistDirtySince = 0
	r.persistReasons = nil
	r.clearPersistDirtyMask()
	return nil
}

func (r *Role) OnRoleCreate() {

	// add test item
	r.ItemAdd(int32(g1_protocol.EItemID_GOLD), 500000, &Reason{g1_protocol.Reason_REASON_INIT, 0})
	r.ItemAdd(int32(g1_protocol.EItemID_DIAMOND), 500000, &Reason{g1_protocol.Reason_REASON_INIT, 0})
	r.ItemAdd(int32(g1_protocol.EItemID_CREDIT), 500000, &Reason{g1_protocol.Reason_REASON_INIT, 0})
}

// 这里服务端主动同步数据到客户端
// 理论上每次玩家数据有变动，都要将相应的数据段同步过去
// 通过Flag控制同步有改变的数据段，减少数据的同步量
func (r *Role) SyncDataToClient(dataFlag g1_protocol.ERoleSectionFlag) error {
	r.MarkFullSync(dataFlag)
	return r.FlushClientSync()
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
	//if !datetime.IsSameMinute(int64(lastClientHeartBeatTime), int64(now)) {
	//	syncFlag |= r.everyMinuteCheck(now)
	//}
	if !datetime.IsSameHour(int64(lastClientHeartBeatTime), int64(now)) {
		syncFlag |= r.everyHourCheck(lastClientHeartBeatTime, now)
	}
	// 早上0点的刷新
	if !datetime.IsSameDay(int64(lastClientHeartBeatTime), int64(now)) {
		syncFlag |= r.everyDayCheck(now)
	}
	// 晚上9点的刷新
	//if datetime.IsSameDayByDayBeginHour(int64(lastClientHeartBeatTime), int64(now), 21) {
	//	syncFlag |= r.everyDayCheck21(now)
	//}
	if !datetime.IsSameWeek(int64(lastClientHeartBeatTime), int64(now)) {
		syncFlag |= r.everyWeakCheck(now)
	}

	// 10秒一次心跳
	if r.HeartBeatCount%2 == 0 {
		r.Debugf("update %v brief info, now: %d", r.Uid(), r.Now())
		_ = r.UpdateBriefInfo()
	}

	if syncFlag > 0 {
		r.MarkFullSync(syncFlag)
	}

	r.HeartBeatCount++
	r.PbRole.LoginInfo.LastHartBeatTime = now
}

// 返回同步数据的Flag
func (r *Role) everyMinuteCheck(now int32) g1_protocol.ERoleSectionFlag {
	return 0
}

func (r *Role) everyHourCheck(last, now int32) g1_protocol.ERoleSectionFlag {
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

	return g1_protocol.ERoleSectionFlag_MALL_INFO |
		g1_protocol.ERoleSectionFlag_ACTVITY_TASK_INFO
}

func (r *Role) everyWeakCheck(now int32) g1_protocol.ERoleSectionFlag {
	return 0
}

// 玩家的简要信息，一般用于展示给其它玩家查看
func (r *Role) GetBriefInfo() *g1_protocol.PbRoleBriefInfo {
	info := &g1_protocol.PbRoleBriefInfo{}

	info.Uid = r.Uid()
	info.Name = r.PbRole.BasicInfo.Name
	info.Level = r.PbRole.BasicInfo.Level
	info.Exp = int32(r.PbRole.BasicInfo.Exp)
	info.IconUrl = r.PbRole.IconInfo.IconUrl
	info.Frame = r.PbRole.IconInfo.FrameId
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

	return infosvrv1.NewInfoServiceClient().SetBriefInfoSimple(r.Uid(), r.Zone(), &req)
}

func (r *Role) ExpAdd(exp int64) {
}

func (r *Role) IsOnline() bool {
	return r.PbRole.LoginInfo.LastHartBeatTime+30 > datetime.Now()
}
