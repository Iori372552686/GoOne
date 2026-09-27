package role

import (
	"context"
	"errors"
	"fmt"
	"sync/atomic"

	infosvrv1 "github.com/Iori372552686/GoOne/api/gen/game/infosvr/v1"
	mysqlsvrv1 "github.com/Iori372552686/GoOne/api/gen/game/mysqlsvr/v1"

	"sync"

	"github.com/Iori372552686/GoOne/lib/api/cmd_handler"
	"github.com/Iori372552686/GoOne/lib/api/datetime"
	"github.com/Iori372552686/GoOne/lib/api/logger"
	"github.com/Iori372552686/GoOne/module/conf"
	g1_protocol "github.com/Iori372552686/g1_common/protocol"
)

// defaultRoleL3FlushDebounceSec L3 快照写回默认防抖：显著大于 L2 的 10s，
// 控制 MySQL 整包写放大（在线角色每 60s 至多一次快照落库）。
const defaultRoleL3FlushDebounceSec = 60

type Role struct {
	sync.Mutex // Role的锁交由外部来控制
	PbRole     *g1_protocol.RoleInfo

	// 下面可以添加临时内存数据，不会持久化到数据库
	HeartBeatExpiryTime int32
	HeartBeatCount      int32

	// lastHeartbeatUnix 心跳时间原子快照（F06）：OnClientHeartbeat 在 UID 串行域
	// 内写；role_tick 调度协程只读此快照做过期候选筛选，不读 PbRole 字段
	//（PbRole 字段无锁，跨域读写是数据竞争）。权威过期判定在 Logout 用例内
	// 于 UID 串行域完成。
	lastHeartbeatUnix atomic.Int64

	// 组件体系：注册顺序即初始化顺序（依赖底层先行）。
	comps        []RoleComponent
	sectionComps []SectionComponent
	patchComps   []PatchComponent
	// patchableMask 全部增量档组件的段位并集（原 patchableRoleSectionMask）。
	patchableMask g1_protocol.ERoleSectionFlag

	// 核心组件 O(1) 访问器（initComponents 装配时填充）。
	Basic    *BasicComponent
	Currency *CurrencyComponent
	Item     *ItemComponent
	Icon     *IconComponent
	Mall     *MallComponent
	Drop     *DropComponent
	Obtain   *ObtainComponent

	pendingFullSyncMask g1_protocol.ERoleSectionFlag
	pendingPatchMask    g1_protocol.ERoleSectionFlag

	needPersist       bool
	persistDirtySince int32
	lastPersistAt     int32
	persistReasons    stringSet
	// persistDirtyMask 记录自上次成功落盘后哪些模块变更过，供 hash 模式增量写。
	// full 模式不读取此字段。落盘成功后清零。
	persistDirtyMask g1_protocol.ERoleSectionFlag

	// L3（role_data 全量快照）写回状态。needL3Flush 在 L2 成功后置位，
	// MaybeFlushL3 按独立防抖消费；投递失败保留标记由 role_tick / 下次
	// FlushPending 重试。lastL3FlushAt 用 Role 本地时钟（防抖区间自洽）。
	needL3Flush   bool
	lastL3FlushAt int32
}

func NewRole(uid uint64) *Role {
	role := new(Role)

	role.PbRole = new(g1_protocol.RoleInfo)
	role.initComponents()
	role.RoleInitField(uid)
	role.OnRoleCreate()
	return role
}

// initComponents 装配全部功能组件。顺序即 OnInit/InitField 顺序：
// 数据段组件依赖底层先行（基础 → 背包 → 外观/商城 → 任务 → 逻辑型）。
func (r *Role) initComponents() {
	r.comps = []RoleComponent{
		NewRegisterComponent(),
		NewLoginComponent(),
		NewGameComponent(),
		NewBasicComponent(),
		NewCurrencyComponent(),
		NewItemComponent(),
		NewIconComponent(),
		NewMallComponent(),
		NewMainTaskComponent(),
		NewGuildComponent(),
		NewGuideComponent(),
		NewOpenFuncComponent(),
		NewActivityTaskComponent(),
		NewDropComponent(),
		NewObtainComponent(),
		NewItemUseComponent(),
	}
	for _, c := range r.comps {
		if err := c.OnInit(r); err != nil {
			r.Errorf("component %s init failed: %v", c.Name(), err)
			continue
		}
		if sc, ok := c.(SectionComponent); ok {
			r.sectionComps = append(r.sectionComps, sc)
		}
		if pc, ok := c.(PatchComponent); ok {
			r.patchComps = append(r.patchComps, pc)
			r.patchableMask |= pc.Flag()
		}
		switch comp := c.(type) {
		case *BasicComponent:
			r.Basic = comp
		case *CurrencyComponent:
			r.Currency = comp
		case *ItemComponent:
			r.Item = comp
		case *IconComponent:
			r.Icon = comp
		case *MallComponent:
			r.Mall = comp
		case *DropComponent:
			r.Drop = comp
		case *ObtainComponent:
			r.Obtain = comp
		}
	}
}

// RoleInitField 数据段 nil 兜底（ConnSvrInfo 为运行时态单独处理）。
// 各段的具体默认值由所属组件的 InitField 提供。
func (r *Role) RoleInitField(uid uint64) {
	if r.PbRole.ConnSvrInfo == nil {
		r.PbRole.ConnSvrInfo = &g1_protocol.ConnSvrInfo{}
	}
	for _, sc := range r.sectionComps {
		sc.InitField(uid)
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

func (r *Role) SaveHash(trans cmd_handler.IContext) error {
	if r.Uid() != trans.Uid() {
		r.Errorf("inconsistent uid {roleUid:%v, transUid:%v}", r.Uid(), trans.Uid())
		return errors.New("inconsistent uid")
	}

	// 运行期事务保存：IContext 不暴露标准 ctx，时限上界由 Redis 客户端
	// read/write timeout 配置承担；停机排空的取消预算走 SaveHashSync(ctx)。
	// 按模块增量写 Redis hash（落盘格式详见 persist_hash.go）。
	if err := saveRoleHash(context.Background(), r, false); err != nil {
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

// SaveHashSync 同步持久化角色数据到 redis，不依赖事务上下文。
// 用于优雅停机等没有 transaction 可用的场景。force=true 全量写所有模块。
// ctx 透传至 Redis 调用：Drain 路径传入排空预算 ctx，取消可传导（F07）。
func (r *Role) SaveHashSync(ctx context.Context) error {
	if err := saveRoleHash(ctx, r, true); err != nil {
		return err
	}
	r.needPersist = false
	r.persistDirtySince = 0
	r.persistReasons = nil
	r.clearPersistDirtyMask()
	return nil
}

// l3FlushDebounceSec L3 快照写回防抖（与 L2 的 10s 防抖解耦，控制 MySQL 写放大）。
func (r *Role) l3FlushDebounceSec() int32 {
	if v := conf.Get("mainsvr.capacity.role_l3_flush_debounce_sec").Int(); v > 0 {
		return int32(v)
	}
	return defaultRoleL3FlushDebounceSec
}

// MaybeFlushL3 按防抖消费 needL3Flush，整包快照 one-way 写回 L3（mysqlsvr）。
// force=true（logout/停机/首次创建）或任一 L3Critical 组件有待写变更时无视
// 防抖立即投递（货币等价值敏感段不落在防抖窗口后面）。
// 失败语义：仅投递失败返回 error 且保留标记；落库结果由持久层 update_time
// 守卫兜底，调用方无需等待 ack。
func (r *Role) MaybeFlushL3(force bool) error {
	if !r.needL3Flush {
		return nil
	}
	critical := false
	for _, c := range r.comps {
		if lc, ok := c.(L3Critical); ok && lc.RequireImmediateL3() {
			critical = true
			break
		}
	}
	now := r.Now()
	if !force && !critical && (r.lastL3FlushAt == 0 || now-r.lastL3FlushAt < r.l3FlushDebounceSec()) {
		return nil
	}
	if err := saveRoleDataL3(r.Uid(), r.PbRole); err != nil {
		return err
	}
	r.needL3Flush = false
	r.lastL3FlushAt = now
	for _, c := range r.comps {
		if ob, ok := c.(L3FlushObserver); ok {
			ob.OnL3Flushed()
		}
	}
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
	r.lastHeartbeatUnix.Store(int64(now))
}

// heartbeatSnapshot 返回心跳时间的原子快照（role_tick 候选筛选专用）。
func (r *Role) heartbeatSnapshot() int64 {
	return r.lastHeartbeatUnix.Load()
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

	//r.MallDailyRefresh()

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
	info.Exp = int32(r.Currency.Get(int32(g1_protocol.EItemID_EXP)))
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

func (r *Role) IsOnline() bool {
	return r.PbRole.LoginInfo.LastHartBeatTime+30 > datetime.Now()
}
