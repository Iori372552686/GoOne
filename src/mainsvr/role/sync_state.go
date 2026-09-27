package role

import (
	"errors"
	"sort"
	"strings"

	"github.com/Iori372552686/GoOne/lib/api/cmd_handler"
	"github.com/Iori372552686/GoOne/lib/service/router"
	"github.com/Iori372552686/GoOne/module/conf"
	g1_protocol "github.com/Iori372552686/g1_common/protocol"
	"google.golang.org/protobuf/proto"
)

const defaultRolePersistDebounceSec = 10

type int32Set map[int32]struct{}
type stringSet map[string]struct{}

func hasRoleSection(mask, flag g1_protocol.ERoleSectionFlag) bool {
	return mask&flag != 0
}

// roleSectionNames 段名列表（日志用），按注册表顺序。
func roleSectionNames(mask g1_protocol.ERoleSectionFlag) []string {
	if mask == 0 {
		return nil
	}
	if mask == g1_protocol.ERoleSectionFlag_ALL {
		return []string{"ALL"}
	}

	names := make([]string, 0, len(roleSectionRegistry))
	for i := range roleSectionRegistry {
		if roleSectionRegistry[i].flag != 0 && hasRoleSection(mask, roleSectionRegistry[i].flag) {
			names = append(names, roleSectionRegistry[i].name)
		}
	}
	return names
}

func roleSectionSummary(mask g1_protocol.ERoleSectionFlag) string {
	names := roleSectionNames(mask)
	if len(names) == 0 {
		return "none"
	}
	return strings.Join(names, "|")
}

func setInt32Value(m *int32Set, value int32) {
	if *m == nil {
		*m = make(int32Set)
	}
	(*m)[value] = struct{}{}
}

func delInt32Value(m *int32Set, value int32) {
	if *m == nil {
		return
	}
	delete(*m, value)
}

func sortedInt32Values(m int32Set) []int32 {
	if len(m) == 0 {
		return nil
	}

	values := make([]int32, 0, len(m))
	for value := range m {
		values = append(values, value)
	}
	sort.Slice(values, func(i, j int) bool { return values[i] < values[j] })
	return values
}

func setStringValue(m *stringSet, value string) {
	if value == "" {
		return
	}
	if *m == nil {
		*m = make(stringSet)
	}
	(*m)[value] = struct{}{}
}

func sortedStringValues(m stringSet) []string {
	if len(m) == 0 {
		return nil
	}

	values := make([]string, 0, len(m))
	for value := range m {
		values = append(values, value)
	}
	sort.Strings(values)
	return values
}

func shouldTrackMutation(reason *Reason) bool {
	return reason == nil || reason.Reason != g1_protocol.Reason_REASON_INIT
}

// trackItemMutation 背包道具域的脏标记入口（货币域由 CurrencyComponent
// 在 Add/Deduct 内自行标记，不经此路径）。
func (r *Role) trackItemMutation(itemID int32, deleted bool, reason *Reason) {
	if !shouldTrackMutation(reason) {
		return
	}
	r.MarkInventoryDirty(itemID, deleted)
}

func (r *Role) MarkFullSync(flag g1_protocol.ERoleSectionFlag) {
	if flag == 0 {
		return
	}

	if flag == g1_protocol.ERoleSectionFlag_ALL {
		r.pendingFullSyncMask = flag
		r.pendingPatchMask = 0
		return
	}

	r.pendingFullSyncMask |= flag
	r.pendingPatchMask &^= flag
}

// sectionComp 按段位查组件（冷路径遍历；热路径组件走 Role 上的直接访问器）。
func (r *Role) sectionComp(flag g1_protocol.ERoleSectionFlag) SectionComponent {
	for _, sc := range r.sectionComps {
		if sc.Flag() == flag {
			return sc
		}
	}
	return nil
}

// ===== Touch* / Mark*Dirty：组件转发（保持既有调用点零改动） =====

func (r *Role) TouchBasicInfo(reason string) {
	if r.Basic != nil {
		r.Basic.Touch(reason)
	}
}

func (r *Role) TouchGameInfo(reason string) {
	if sc := r.sectionComp(g1_protocol.ERoleSectionFlag_GAME_INFO); sc != nil {
		sc.Touch(reason)
	}
}

func (r *Role) TouchGuideInfo(reason string) {
	if sc := r.sectionComp(g1_protocol.ERoleSectionFlag_GUIDE_INFO); sc != nil {
		sc.Touch(reason)
	}
}

func (r *Role) TouchOpenFuncInfo(reason string) {
	if sc := r.sectionComp(g1_protocol.ERoleSectionFlag_OPEN_FUNC_INFO); sc != nil {
		sc.Touch(reason)
	}
}

func (r *Role) TouchMainTaskInfo(reason string) {
	if sc := r.sectionComp(g1_protocol.ERoleSectionFlag_MAIN_TASK_INFO); sc != nil {
		sc.Touch(reason)
	}
}

func (r *Role) TouchActvityTaskInfo(reason string) {
	if sc := r.sectionComp(g1_protocol.ERoleSectionFlag_ACTVITY_TASK_INFO); sc != nil {
		sc.Touch(reason)
	}
}

func (r *Role) markPatchSection(flag g1_protocol.ERoleSectionFlag) {
	if flag == 0 {
		return
	}
	if r.pendingFullSyncMask == g1_protocol.ERoleSectionFlag_ALL || hasRoleSection(r.pendingFullSyncMask, flag) {
		return
	}
	r.pendingPatchMask |= flag
}

func (r *Role) MarkInventoryDirty(itemID int32, deleted bool) {
	if r.Item != nil {
		r.Item.MarkDirty(itemID, deleted)
	}
}

func (r *Role) MarkMallDirty(confID int32, deleted bool) {
	if r.Mall != nil {
		r.Mall.MarkDirty(confID, deleted)
	}
}

func (r *Role) MarkIconDirty(iconID int32, deleted bool) {
	if r.Icon != nil {
		r.Icon.MarkIconDirty(iconID, deleted)
	}
}

func (r *Role) MarkFrameDirty(frameID int32, deleted bool) {
	if r.Icon != nil {
		r.Icon.MarkFrameDirty(frameID, deleted)
	}
}

func (r *Role) MarkIconEquipDirty() {
	if r.Icon != nil {
		r.Icon.MarkIconEquipDirty()
	}
}

func (r *Role) MarkActvityTaskDirty(taskID int32, deleted bool) {
	if sc := r.sectionComp(g1_protocol.ERoleSectionFlag_ACTVITY_TASK_INFO); sc != nil {
		if pc, ok := sc.(PatchComponent); ok {
			pc.MarkDirty(taskID, deleted)
		}
	}
}

func (r *Role) MarkPersistDirty(reason string) {
	if !r.needPersist {
		r.persistDirtySince = r.Now()
	}
	r.needPersist = true
	setStringValue(&r.persistReasons, reason)
}

// markPersistSectionDirty 在 MarkPersistDirty 基础上把 flag 并入 persistDirtyMask，
// 供 hash 模式按模块增量落盘。各 Touch* 方法在已知变更模块时调用此版本。
func (r *Role) markPersistSectionDirty(flag g1_protocol.ERoleSectionFlag, reason string) {
	if flag != 0 {
		r.persistDirtyMask |= flag
	}
	r.MarkPersistDirty(reason)
}

func (r *Role) persistDebounceSec() int32 {
	if v := conf.Get("mainsvr.capacity.role_persist_debounce_sec").Int(); v > 0 {
		return int32(v)
	}
	return defaultRolePersistDebounceSec
}

func (r *Role) shouldFlushPersistNow(now int32, force bool) bool {
	if !r.needPersist {
		return false
	}
	if force {
		return true
	}
	if r.persistDirtySince == 0 {
		return false
	}
	return now-r.persistDirtySince >= r.persistDebounceSec()
}

func (r *Role) MaybeFlushPersist(trans cmd_handler.IContext, force bool) error {
	if trans == nil {
		return nil
	}

	now := r.Now()
	if !r.shouldFlushPersistNow(now, force) {
		return nil
	}

	if err := r.SaveHash(trans); err != nil {
		return err
	}

	r.needPersist = false
	r.persistDirtySince = 0
	r.lastPersistAt = now
	r.persistReasons = nil

	// L2 已持久，顺手消费 L3 待写标记（独立防抖；force 语义透传）。
	// L3 投递失败不影响本次持久化结果——标记保留，下次 FlushPending 重试。
	if err := r.MaybeFlushL3(force); err != nil {
		r.Errorf("role l3 flush deferred for retry | %v", err)
	}
	return nil
}

func (r *Role) FlushPending(trans cmd_handler.IContext, forcePersist bool) error {
	var firstErr error
	if err := r.FlushClientSync(); err != nil {
		firstErr = err
	}
	if err := r.MaybeFlushPersist(trans, forcePersist); err != nil && firstErr == nil {
		firstErr = err
	}
	return firstErr
}

func (r *Role) ShouldUseSyncPatch() bool {
	if !conf.Get("mainsvr.capacity.role_sync_patch_enabled").Bool() {
		return false
	}
	var allowUids []uint64
	_ = conf.Unmarshal("mainsvr.capacity.role_sync_patch_allow_uids", &allowUids)
	if len(allowUids) == 0 {
		return true
	}
	for _, uid := range allowUids {
		if uid == r.Uid() {
			return true
		}
	}
	return false
}

func (r *Role) prepareSyncPayload(usePatch bool) (
	fullMask g1_protocol.ERoleSectionFlag,
	requestedPatchMask g1_protocol.ERoleSectionFlag,
	actualPatchMask g1_protocol.ERoleSectionFlag,
	legacy *g1_protocol.ScSyncUserData,
	v2 *g1_protocol.ScSyncUserDataV2,
) {
	fullMask = r.pendingFullSyncMask
	requestedPatchMask = r.pendingPatchMask & r.patchableMask

	if !usePatch {
		fullMask |= requestedPatchMask
		requestedPatchMask = 0
		if fullMask != 0 {
			legacy = r.buildLegacySyncData(fullMask)
		}
		return
	}

	if fullMask == g1_protocol.ERoleSectionFlag_ALL {
		requestedPatchMask = 0
	} else {
		requestedPatchMask &^= fullMask
	}
	if fullMask == 0 && requestedPatchMask == 0 {
		return
	}

	v2, actualPatchMask = r.buildSyncDataV2(fullMask, requestedPatchMask)
	return
}

func (r *Role) FlushClientSync() error {
	fullMask, requestedPatchMask, actualPatchMask, legacy, v2 := r.prepareSyncPayload(r.ShouldUseSyncPatch())
	if fullMask == 0 && requestedPatchMask == 0 {
		return nil
	}

	connsvrBusID := r.PbRole.ConnSvrInfo.BusId
	if connsvrBusID == 0 {
		return errors.New("the player are not online")
	}

	if legacy != nil {
		r.Infof("role sync {mode:legacy, full:%s, size:%d}", roleSectionSummary(fullMask), proto.Size(legacy))
		if err := router.SendPbMsgByBusIdSimple(connsvrBusID, r.Uid(), g1_protocol.CMD_SC_SYNC_USER_DATA, legacy); err != nil {
			return err
		}
		r.clearSyncedState(fullMask, 0)
		return nil
	}

	if v2 == nil && fullMask == 0 && actualPatchMask == 0 {
		r.clearSyncedState(0, requestedPatchMask)
		return nil
	}

	r.Infof("role sync {mode:v2, full:%s, patch:%s, size:%d}",
		roleSectionSummary(fullMask), roleSectionSummary(actualPatchMask), proto.Size(v2))
	if err := router.SendPbMsgByBusIdSimple(connsvrBusID, r.Uid(), g1_protocol.CMD_SC_SYNC_USER_DATA_V2, v2); err != nil {
		return err
	}
	r.clearSyncedState(fullMask, requestedPatchMask)
	return nil
}

func (r *Role) clearSyncedState(fullMask, patchMask g1_protocol.ERoleSectionFlag) {
	if fullMask == g1_protocol.ERoleSectionFlag_ALL {
		r.pendingFullSyncMask = 0
		r.pendingPatchMask = 0
		for _, pc := range r.patchComps {
			pc.ClearDirty()
		}
		return
	}

	r.pendingFullSyncMask &^= fullMask
	r.pendingPatchMask &^= patchMask
	r.pendingPatchMask &^= fullMask

	for _, pc := range r.patchComps {
		flag := pc.Flag()
		if hasRoleSection(fullMask, flag) || hasRoleSection(patchMask, flag) {
			pc.ClearDirty()
		}
	}
}

// fillRoleInfoByMask 按段位掩码把内存数据填充到同步用 RoleInfo（注册表驱动）。
func (r *Role) fillRoleInfoByMask(dst *g1_protocol.RoleInfo, mask g1_protocol.ERoleSectionFlag) {
	if dst == nil || mask == 0 {
		return
	}
	for i := range roleSectionRegistry {
		sec := &roleSectionRegistry[i]
		if sec.flag == 0 || !hasRoleSection(mask, sec.flag) {
			continue
		}
		if msg := sec.getMsg(r.PbRole); msg != nil {
			sec.setMsg(dst, msg)
		}
	}
}

func (r *Role) buildLegacySyncData(mask g1_protocol.ERoleSectionFlag) *g1_protocol.ScSyncUserData {
	data := &g1_protocol.ScSyncUserData{
		RoleInfo: new(g1_protocol.RoleInfo),
	}
	r.fillRoleInfoByMask(data.RoleInfo, mask)
	return data
}

func (r *Role) buildSyncDataV2(
	fullMask, requestedPatchMask g1_protocol.ERoleSectionFlag,
) (*g1_protocol.ScSyncUserDataV2, g1_protocol.ERoleSectionFlag) {
	data := &g1_protocol.ScSyncUserDataV2{
		FullSectionMask:  int32(fullMask),
		PatchSectionMask: int32(requestedPatchMask),
	}
	if fullMask != 0 {
		data.RoleInfo = new(g1_protocol.RoleInfo)
		r.fillRoleInfoByMask(data.RoleInfo, fullMask)
	}

	actualPatchMask := g1_protocol.ERoleSectionFlag(0)
	for _, pc := range r.patchComps {
		if !hasRoleSection(requestedPatchMask, pc.Flag()) {
			continue
		}
		if pc.BuildPatch(data) {
			actualPatchMask |= pc.Flag()
		}
	}

	data.PatchSectionMask = int32(actualPatchMask)
	if fullMask == 0 && actualPatchMask == 0 {
		return nil, 0
	}
	return data, actualPatchMask
}
