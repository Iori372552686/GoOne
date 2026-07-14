package role

import (
	"errors"
	"sort"
	"strings"

	"github.com/Iori372552686/GoOne/common/gconf"
	"github.com/Iori372552686/GoOne/lib/api/cmd_handler"
	"github.com/Iori372552686/GoOne/lib/service/router"
	g1_protocol "github.com/Iori372552686/game_protocol/protocol"
	"google.golang.org/protobuf/proto"
)

const defaultRolePersistDebounceSec = 10

const patchableRoleSectionMask = g1_protocol.ERoleSectionFlag_INVENTORY_INFO |
	g1_protocol.ERoleSectionFlag_ICON_INFO |
	g1_protocol.ERoleSectionFlag_MALL_INFO |
	g1_protocol.ERoleSectionFlag_ACTVITY_TASK_INFO

type int32Set map[int32]struct{}
type stringSet map[string]struct{}

type roleSectionDef struct {
	flag g1_protocol.ERoleSectionFlag
	name string
}

var roleSectionDefs = []roleSectionDef{
	{flag: g1_protocol.ERoleSectionFlag_REGISTER_INFO, name: "register"},
	{flag: g1_protocol.ERoleSectionFlag_LOGIN_INFO, name: "login"},
	{flag: g1_protocol.ERoleSectionFlag_GAME_INFO, name: "game"},
	{flag: g1_protocol.ERoleSectionFlag_BASIC_INFO, name: "basic"},
	{flag: g1_protocol.ERoleSectionFlag_INVENTORY_INFO, name: "inventory"},
	{flag: g1_protocol.ERoleSectionFlag_ICON_INFO, name: "icon"},
	{flag: g1_protocol.ERoleSectionFlag_MALL_INFO, name: "mall"},
	{flag: g1_protocol.ERoleSectionFlag_MAIN_TASK_INFO, name: "main_task"},
	{flag: g1_protocol.ERoleSectionFlag_GUILD_INFO, name: "guild"},
	{flag: g1_protocol.ERoleSectionFlag_GUIDE_INFO, name: "guide"},
	{flag: g1_protocol.ERoleSectionFlag_OPEN_FUNC_INFO, name: "open_func"},
	{flag: g1_protocol.ERoleSectionFlag_ACTVITY_TASK_INFO, name: "activity_task"},
}

func hasRoleSection(mask, flag g1_protocol.ERoleSectionFlag) bool {
	return mask&flag != 0
}

func roleSectionNames(mask g1_protocol.ERoleSectionFlag) []string {
	if mask == 0 {
		return nil
	}
	if mask == g1_protocol.ERoleSectionFlag_ALL {
		return []string{"ALL"}
	}

	names := make([]string, 0, len(roleSectionDefs))
	for _, def := range roleSectionDefs {
		if hasRoleSection(mask, def.flag) {
			names = append(names, def.name)
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

func isBasicInfoItem(itemID int32) bool {
	switch g1_protocol.EItemID(itemID) {
	case g1_protocol.EItemID_GOLD,
		g1_protocol.EItemID_DIAMOND,
		g1_protocol.EItemID_CREDIT,
		g1_protocol.EItemID_LIVENESS,
		g1_protocol.EItemID_GUILDGOLD,
		g1_protocol.EItemID_ACECOIN,
		g1_protocol.EItemID_WINACECOIN:
		return true
	default:
		return false
	}
}

func (r *Role) trackItemMutation(itemID int32, deleted bool, reason *Reason) {
	if !shouldTrackMutation(reason) {
		return
	}
	if isBasicInfoItem(itemID) {
		r.TouchBasicInfo("basic_info")
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

func (r *Role) TouchBasicInfo(reason string) {
	r.MarkFullSync(g1_protocol.ERoleSectionFlag_BASIC_INFO)
	r.markPersistSectionDirty(g1_protocol.ERoleSectionFlag_BASIC_INFO, reason)
}

func (r *Role) TouchGameInfo(reason string) {
	r.MarkFullSync(g1_protocol.ERoleSectionFlag_GAME_INFO)
	r.markPersistSectionDirty(g1_protocol.ERoleSectionFlag_GAME_INFO, reason)
}

func (r *Role) TouchGuideInfo(reason string) {
	r.MarkFullSync(g1_protocol.ERoleSectionFlag_GUIDE_INFO)
	r.markPersistSectionDirty(g1_protocol.ERoleSectionFlag_GUIDE_INFO, reason)
}

func (r *Role) TouchOpenFuncInfo(reason string) {
	r.MarkFullSync(g1_protocol.ERoleSectionFlag_OPEN_FUNC_INFO)
	r.markPersistSectionDirty(g1_protocol.ERoleSectionFlag_OPEN_FUNC_INFO, reason)
}

func (r *Role) TouchMainTaskInfo(reason string) {
	r.MarkFullSync(g1_protocol.ERoleSectionFlag_MAIN_TASK_INFO)
	r.markPersistSectionDirty(g1_protocol.ERoleSectionFlag_MAIN_TASK_INFO, reason)
}

func (r *Role) TouchActvityTaskInfo(reason string) {
	r.markPatchSection(g1_protocol.ERoleSectionFlag_ACTVITY_TASK_INFO)
	r.markPersistSectionDirty(g1_protocol.ERoleSectionFlag_ACTVITY_TASK_INFO, reason)
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
	r.markPatchSection(g1_protocol.ERoleSectionFlag_INVENTORY_INFO)
	if deleted {
		delInt32Value(&r.inventoryUpserts, itemID)
		setInt32Value(&r.inventoryDeletes, itemID)
	} else {
		delInt32Value(&r.inventoryDeletes, itemID)
		setInt32Value(&r.inventoryUpserts, itemID)
	}
	r.markPersistSectionDirty(g1_protocol.ERoleSectionFlag_INVENTORY_INFO, "inventory")
}

func (r *Role) MarkMallDirty(confID int32, deleted bool) {
	r.markPatchSection(g1_protocol.ERoleSectionFlag_MALL_INFO)
	if deleted {
		delInt32Value(&r.mallUpserts, confID)
		setInt32Value(&r.mallDeletes, confID)
	} else {
		delInt32Value(&r.mallDeletes, confID)
		setInt32Value(&r.mallUpserts, confID)
	}
	r.markPersistSectionDirty(g1_protocol.ERoleSectionFlag_MALL_INFO, "mall")
}

func (r *Role) MarkIconDirty(iconID int32, deleted bool) {
	r.markPatchSection(g1_protocol.ERoleSectionFlag_ICON_INFO)
	if deleted {
		delInt32Value(&r.iconUpserts, iconID)
		setInt32Value(&r.iconDeletes, iconID)
	} else {
		delInt32Value(&r.iconDeletes, iconID)
		setInt32Value(&r.iconUpserts, iconID)
	}
	r.markPersistSectionDirty(g1_protocol.ERoleSectionFlag_ICON_INFO, "icon")
}

func (r *Role) MarkFrameDirty(frameID int32, deleted bool) {
	r.markPatchSection(g1_protocol.ERoleSectionFlag_ICON_INFO)
	if deleted {
		delInt32Value(&r.frameUpserts, frameID)
		setInt32Value(&r.frameDeletes, frameID)
	} else {
		delInt32Value(&r.frameDeletes, frameID)
		setInt32Value(&r.frameUpserts, frameID)
	}
	r.markPersistSectionDirty(g1_protocol.ERoleSectionFlag_ICON_INFO, "icon")
}

func (r *Role) MarkIconEquipDirty() {
	r.markPatchSection(g1_protocol.ERoleSectionFlag_ICON_INFO)
	r.iconEquipDirty = true
	r.markPersistSectionDirty(g1_protocol.ERoleSectionFlag_ICON_INFO, "icon")
}

func (r *Role) MarkActvityTaskDirty(taskID int32, deleted bool) {
	r.markPatchSection(g1_protocol.ERoleSectionFlag_ACTVITY_TASK_INFO)
	if deleted {
		delInt32Value(&r.actTaskUpserts, taskID)
		setInt32Value(&r.actTaskDeletes, taskID)
	} else {
		delInt32Value(&r.actTaskDeletes, taskID)
		setInt32Value(&r.actTaskUpserts, taskID)
	}
	r.markPersistSectionDirty(g1_protocol.ERoleSectionFlag_ACTVITY_TASK_INFO, "activity_task")
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
	if gconf.MainSvrCfg.Capacity.RolePersistDebounceSec > 0 {
		return int32(gconf.MainSvrCfg.Capacity.RolePersistDebounceSec)
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

	if err := r.SaveToDB(trans); err != nil {
		return err
	}

	r.needPersist = false
	r.persistDirtySince = 0
	r.lastPersistAt = now
	r.persistReasons = nil
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
	if !gconf.MainSvrCfg.Capacity.RoleSyncPatchEnabled {
		return false
	}
	allowUids := gconf.MainSvrCfg.Capacity.RoleSyncPatchAllowUids
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
	requestedPatchMask = r.pendingPatchMask & patchableRoleSectionMask

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
		r.clearInventoryDirty()
		r.clearMallDirty()
		r.clearIconDirty()
		r.clearActTaskDirty()
		return
	}

	r.pendingFullSyncMask &^= fullMask
	r.pendingPatchMask &^= patchMask
	r.pendingPatchMask &^= fullMask

	if hasRoleSection(fullMask, g1_protocol.ERoleSectionFlag_INVENTORY_INFO) ||
		hasRoleSection(patchMask, g1_protocol.ERoleSectionFlag_INVENTORY_INFO) {
		r.clearInventoryDirty()
	}
	if hasRoleSection(fullMask, g1_protocol.ERoleSectionFlag_MALL_INFO) ||
		hasRoleSection(patchMask, g1_protocol.ERoleSectionFlag_MALL_INFO) {
		r.clearMallDirty()
	}
	if hasRoleSection(fullMask, g1_protocol.ERoleSectionFlag_ICON_INFO) ||
		hasRoleSection(patchMask, g1_protocol.ERoleSectionFlag_ICON_INFO) {
		r.clearIconDirty()
	}
	if hasRoleSection(fullMask, g1_protocol.ERoleSectionFlag_ACTVITY_TASK_INFO) ||
		hasRoleSection(patchMask, g1_protocol.ERoleSectionFlag_ACTVITY_TASK_INFO) {
		r.clearActTaskDirty()
	}
}

func (r *Role) clearInventoryDirty() {
	r.inventoryUpserts = nil
	r.inventoryDeletes = nil
}

func (r *Role) clearMallDirty() {
	r.mallUpserts = nil
	r.mallDeletes = nil
}

func (r *Role) clearIconDirty() {
	r.iconUpserts = nil
	r.iconDeletes = nil
	r.frameUpserts = nil
	r.frameDeletes = nil
	r.iconEquipDirty = false
}

func (r *Role) clearActTaskDirty() {
	r.actTaskUpserts = nil
	r.actTaskDeletes = nil
}

func (r *Role) fillRoleInfoByMask(roleInfo *g1_protocol.RoleInfo, mask g1_protocol.ERoleSectionFlag) {
	if roleInfo == nil || mask == 0 {
		return
	}
	if hasRoleSection(mask, g1_protocol.ERoleSectionFlag_REGISTER_INFO) {
		roleInfo.RegisterInfo = r.PbRole.RegisterInfo
	}
	if hasRoleSection(mask, g1_protocol.ERoleSectionFlag_LOGIN_INFO) {
		roleInfo.LoginInfo = r.PbRole.LoginInfo
	}
	if hasRoleSection(mask, g1_protocol.ERoleSectionFlag_GAME_INFO) {
		roleInfo.GameInfo = r.PbRole.GameInfo
	}
	if hasRoleSection(mask, g1_protocol.ERoleSectionFlag_BASIC_INFO) {
		roleInfo.BasicInfo = r.PbRole.BasicInfo
	}
	if hasRoleSection(mask, g1_protocol.ERoleSectionFlag_INVENTORY_INFO) {
		roleInfo.InventoryInfo = r.PbRole.InventoryInfo
	}
	if hasRoleSection(mask, g1_protocol.ERoleSectionFlag_ICON_INFO) {
		roleInfo.IconInfo = r.PbRole.IconInfo
	}
	if hasRoleSection(mask, g1_protocol.ERoleSectionFlag_MALL_INFO) {
		roleInfo.MallInfo = r.PbRole.MallInfo
	}
	if hasRoleSection(mask, g1_protocol.ERoleSectionFlag_MAIN_TASK_INFO) {
		roleInfo.MainTaskInfo = r.PbRole.MainTaskInfo
	}
	if hasRoleSection(mask, g1_protocol.ERoleSectionFlag_GUILD_INFO) {
		roleInfo.GuildInfo = r.PbRole.GuildInfo
	}
	if hasRoleSection(mask, g1_protocol.ERoleSectionFlag_GUIDE_INFO) {
		roleInfo.GuideInfo = r.PbRole.GuideInfo
	}
	if hasRoleSection(mask, g1_protocol.ERoleSectionFlag_OPEN_FUNC_INFO) {
		roleInfo.OpenFunInfo = r.PbRole.OpenFunInfo
	}
	if hasRoleSection(mask, g1_protocol.ERoleSectionFlag_ACTVITY_TASK_INFO) {
		roleInfo.Actvity_Info = r.PbRole.Actvity_Info
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
	if hasRoleSection(requestedPatchMask, g1_protocol.ERoleSectionFlag_INVENTORY_INFO) {
		if patch := r.buildInventoryPatch(); patch != nil {
			data.InventoryPatch = patch
			actualPatchMask |= g1_protocol.ERoleSectionFlag_INVENTORY_INFO
		}
	}
	if hasRoleSection(requestedPatchMask, g1_protocol.ERoleSectionFlag_MALL_INFO) {
		if patch := r.buildMallPatch(); patch != nil {
			data.MallPatch = patch
			actualPatchMask |= g1_protocol.ERoleSectionFlag_MALL_INFO
		}
	}
	if hasRoleSection(requestedPatchMask, g1_protocol.ERoleSectionFlag_ICON_INFO) {
		if patch := r.buildIconPatch(); patch != nil {
			data.IconPatch = patch
			actualPatchMask |= g1_protocol.ERoleSectionFlag_ICON_INFO
		}
	}
	if hasRoleSection(requestedPatchMask, g1_protocol.ERoleSectionFlag_ACTVITY_TASK_INFO) {
		if patch := r.buildActvityTaskPatch(); patch != nil {
			data.ActvityTaskPatch = patch
			actualPatchMask |= g1_protocol.ERoleSectionFlag_ACTVITY_TASK_INFO
		}
	}

	data.PatchSectionMask = int32(actualPatchMask)
	if fullMask == 0 && actualPatchMask == 0 {
		return nil, 0
	}
	return data, actualPatchMask
}

func (r *Role) buildInventoryPatch() *g1_protocol.RoleInventoryPatch {
	patch := &g1_protocol.RoleInventoryPatch{}
	for _, itemID := range sortedInt32Values(r.inventoryUpserts) {
		if r.PbRole.InventoryInfo == nil || r.PbRole.InventoryInfo.ItemMap == nil {
			continue
		}
		if item := r.PbRole.InventoryInfo.ItemMap[itemID]; item != nil {
			patch.UpsertItems = append(patch.UpsertItems, item)
		}
	}
	patch.DeleteItemIds = sortedInt32Values(r.inventoryDeletes)
	if len(patch.UpsertItems) == 0 && len(patch.DeleteItemIds) == 0 {
		return nil
	}
	return patch
}

func (r *Role) buildMallPatch() *g1_protocol.RoleMallPatch {
	patch := &g1_protocol.RoleMallPatch{}
	for _, confID := range sortedInt32Values(r.mallUpserts) {
		if r.PbRole.MallInfo == nil || r.PbRole.MallInfo.ItemMap == nil {
			continue
		}
		if item := r.PbRole.MallInfo.ItemMap[confID]; item != nil {
			patch.UpsertItems = append(patch.UpsertItems, item)
		}
	}
	patch.DeleteConfIds = sortedInt32Values(r.mallDeletes)
	if len(patch.UpsertItems) == 0 && len(patch.DeleteConfIds) == 0 {
		return nil
	}
	return patch
}

func (r *Role) buildIconPatch() *g1_protocol.RoleIconPatch {
	patch := &g1_protocol.RoleIconPatch{}
	if r.iconEquipDirty {
		patch.HasCurrentIconUrl = true
		patch.CurrentIconUrl = r.PbRole.IconInfo.IconUrl
		patch.HasCurrentFrameId = true
		patch.CurrentFrameId = r.PbRole.IconInfo.FrameId
	}
	for _, iconID := range sortedInt32Values(r.iconUpserts) {
		if r.PbRole.IconInfo == nil || r.PbRole.IconInfo.IconMap == nil {
			continue
		}
		if icon := r.PbRole.IconInfo.IconMap[iconID]; icon != nil {
			patch.UpsertIcons = append(patch.UpsertIcons, icon)
		}
	}
	patch.DeleteIconIds = sortedInt32Values(r.iconDeletes)
	for _, frameID := range sortedInt32Values(r.frameUpserts) {
		if r.PbRole.IconInfo == nil || r.PbRole.IconInfo.FrameMap == nil {
			continue
		}
		if frame := r.PbRole.IconInfo.FrameMap[frameID]; frame != nil {
			patch.UpsertFrames = append(patch.UpsertFrames, frame)
		}
	}
	patch.DeleteFrameIds = sortedInt32Values(r.frameDeletes)
	if !patch.HasCurrentIconUrl && !patch.HasCurrentFrameId &&
		len(patch.UpsertIcons) == 0 && len(patch.DeleteIconIds) == 0 &&
		len(patch.UpsertFrames) == 0 && len(patch.DeleteFrameIds) == 0 {
		return nil
	}
	return patch
}

func (r *Role) buildActvityTaskPatch() *g1_protocol.RoleActvityTaskPatch {
	patch := &g1_protocol.RoleActvityTaskPatch{}
	for _, taskID := range sortedInt32Values(r.actTaskUpserts) {
		if r.PbRole.Actvity_Info == nil || r.PbRole.Actvity_Info.TaskMap == nil {
			continue
		}
		if task := r.PbRole.Actvity_Info.TaskMap[taskID]; task != nil {
			patch.UpsertTasks = append(patch.UpsertTasks, task)
		}
	}
	patch.DeleteTaskIds = sortedInt32Values(r.actTaskDeletes)
	if len(patch.UpsertTasks) == 0 && len(patch.DeleteTaskIds) == 0 {
		return nil
	}
	return patch
}
