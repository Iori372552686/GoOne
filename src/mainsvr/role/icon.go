/// 玩家邮箱，相框，立绘相关

package role

import (
	"fmt"

	"github.com/Iori372552686/GoOne/lib/util/convert"
	g1_protocol "github.com/Iori372552686/g1_common/protocol"
)

func (r *Role) GetIconDesc() *g1_protocol.PbIconDesc {
	desc := &g1_protocol.PbIconDesc{}
	desc.Name = r.PbRole.BasicInfo.Name
	desc.IconUrl = r.PbRole.IconInfo.IconUrl
	desc.Frame = r.PbRole.IconInfo.FrameId
	desc.Level = r.PbRole.BasicInfo.Level
	desc.IsOnline = r.IsOnline()
	desc.Uid = r.PbRole.RegisterInfo.Uid
	//desc.VipLevel = 0
	return desc
}

func (r *Role) IconGet(id int32, addIfExist bool) *g1_protocol.PbIcon {
	if r.PbRole.IconInfo.IconMap == nil {
		r.PbRole.IconInfo.IconMap = make(map[int32]*g1_protocol.PbIcon)
	}

	if r.PbRole.IconInfo.IconMap[id] != nil {
		return r.PbRole.IconInfo.IconMap[id]
	}

	if addIfExist {
		ins := &g1_protocol.PbIcon{Id: id}
		r.PbRole.IconInfo.IconMap[id] = ins
		return ins
	}
	return nil
}

func (r *Role) FrameGet(id int32, addIfExist bool) *g1_protocol.PbFrame {
	if r.PbRole.IconInfo.FrameMap == nil {
		r.PbRole.IconInfo.FrameMap = make(map[int32]*g1_protocol.PbFrame)
	}

	if r.PbRole.IconInfo.FrameMap[id] != nil {
		return r.PbRole.IconInfo.FrameMap[id]
	}

	if addIfExist {
		ins := &g1_protocol.PbFrame{Id: id}
		r.PbRole.IconInfo.FrameMap[id] = ins
		return ins
	}
	return nil
}

func (r *Role) IconAdd(id int32, reason *Reason) int {
	icon := r.IconGet(id, true)
	if reason.Reason != g1_protocol.Reason_REASON_INIT {
		icon.RedPoint = true
	}
	if shouldTrackMutation(reason) {
		r.MarkIconDirty(id, false)
	}
	return 0
}

func (r *Role) IconHas(id int32) bool {
	v := r.IconGet(id, false)
	return v != nil
}

func (r *Role) FrameAdd(id int32, reason *Reason) int {
	frame := r.FrameGet(id, true)
	if reason.Reason != g1_protocol.Reason_REASON_INIT {
		frame.RedPoint = true
	}
	if shouldTrackMutation(reason) {
		r.MarkFrameDirty(id, false)
	}
	return 0
}

func (r *Role) FrameHas(id int32) bool {
	v := r.FrameGet(id, false)
	return v != nil
}

func (r *Role) IconChange(iconId int32) g1_protocol.ErrorCode {
	if iconId <= 0 { //&& !r.IconHas(iconId)
		return g1_protocol.ErrorCode_ERR_ICON_NOT_HAVE
	}
	r.PbRole.IconInfo.IconUrl = fmt.Sprintf("headicon_%d", iconId)
	r.MarkIconEquipDirty()
	return g1_protocol.ErrorCode_ERR_OK
}

func (r *Role) FrameChange(frameId int32) g1_protocol.ErrorCode {
	if frameId > 0 && !r.FrameHas(frameId) {
		return g1_protocol.ErrorCode_ERR_FRAME_NOT_HAVE
	}
	r.PbRole.IconInfo.FrameId = frameId
	r.MarkIconEquipDirty()
	return g1_protocol.ErrorCode_ERR_OK
}

func (r *Role) ImageChange(imageId int32) g1_protocol.ErrorCode {

	return g1_protocol.ErrorCode_ERR_OK
}

func (r *Role) IconTouchRedPoint(id int32) int {
	icon := r.IconGet(id, false)
	if icon != nil {
		icon.RedPoint = false
		r.MarkIconDirty(id, false)
	}
	return 0
}

func (r *Role) FrameTouchRedPoint(id int32) int {
	frame := r.FrameGet(id, false)
	if frame != nil {
		frame.RedPoint = false
		r.MarkFrameDirty(id, false)
	}
	return 0
}

// ============================================================
// 头像/相框段组件（增量档）
// ============================================================

type IconComponent struct {
	sectionBase

	iconUpserts    int32Set
	iconDeletes    int32Set
	frameUpserts   int32Set
	frameDeletes   int32Set
	iconEquipDirty bool
}

func NewIconComponent() *IconComponent { return &IconComponent{} }

func (c *IconComponent) Name() string { return "icon" }

func (c *IconComponent) Flag() g1_protocol.ERoleSectionFlag {
	return g1_protocol.ERoleSectionFlag_ICON_INFO
}

func (c *IconComponent) OnInit(r *Role) error { c.bind(r); return nil }
func (c *IconComponent) OnDestroy()           {}

func (c *IconComponent) InitField(uid uint64) {
	if c.role.PbRole.IconInfo == nil {
		c.role.PbRole.IconInfo = &g1_protocol.RoleIconInfo{}
		c.role.PbRole.IconInfo.IconUrl = "headicon_" + convert.Int64ToString(int64(uid)%31)
	}
}

func (c *IconComponent) Touch(reason string) {
	c.touch(g1_protocol.ERoleSectionFlag_ICON_INFO, reason)
}

// MarkDirty 接口方法按头像 id 语义处理；相框走 MarkFrameDirty。
func (c *IconComponent) MarkDirty(iconID int32, deleted bool) {
	c.MarkIconDirty(iconID, deleted)
}

func (c *IconComponent) MarkIconDirty(iconID int32, deleted bool) {
	c.role.markPatchSection(g1_protocol.ERoleSectionFlag_ICON_INFO)
	if deleted {
		delInt32Value(&c.iconUpserts, iconID)
		setInt32Value(&c.iconDeletes, iconID)
	} else {
		delInt32Value(&c.iconDeletes, iconID)
		setInt32Value(&c.iconUpserts, iconID)
	}
	c.role.markPersistSectionDirty(g1_protocol.ERoleSectionFlag_ICON_INFO, "icon")
}

func (c *IconComponent) MarkFrameDirty(frameID int32, deleted bool) {
	c.role.markPatchSection(g1_protocol.ERoleSectionFlag_ICON_INFO)
	if deleted {
		delInt32Value(&c.frameUpserts, frameID)
		setInt32Value(&c.frameDeletes, frameID)
	} else {
		delInt32Value(&c.frameDeletes, frameID)
		setInt32Value(&c.frameUpserts, frameID)
	}
	c.role.markPersistSectionDirty(g1_protocol.ERoleSectionFlag_ICON_INFO, "icon")
}

func (c *IconComponent) MarkIconEquipDirty() {
	c.role.markPatchSection(g1_protocol.ERoleSectionFlag_ICON_INFO)
	c.iconEquipDirty = true
	c.role.markPersistSectionDirty(g1_protocol.ERoleSectionFlag_ICON_INFO, "icon")
}

func (c *IconComponent) BuildPatch(dst *g1_protocol.ScSyncUserDataV2) bool {
	patch := &g1_protocol.RoleIconPatch{}
	if c.iconEquipDirty {
		patch.HasCurrentIconUrl = true
		patch.CurrentIconUrl = c.role.PbRole.IconInfo.IconUrl
		patch.HasCurrentFrameId = true
		patch.CurrentFrameId = c.role.PbRole.IconInfo.FrameId
	}
	for _, iconID := range sortedInt32Values(c.iconUpserts) {
		if c.role.PbRole.IconInfo == nil || c.role.PbRole.IconInfo.IconMap == nil {
			continue
		}
		if icon := c.role.PbRole.IconInfo.IconMap[iconID]; icon != nil {
			patch.UpsertIcons = append(patch.UpsertIcons, icon)
		}
	}
	patch.DeleteIconIds = sortedInt32Values(c.iconDeletes)
	for _, frameID := range sortedInt32Values(c.frameUpserts) {
		if c.role.PbRole.IconInfo == nil || c.role.PbRole.IconInfo.FrameMap == nil {
			continue
		}
		if frame := c.role.PbRole.IconInfo.FrameMap[frameID]; frame != nil {
			patch.UpsertFrames = append(patch.UpsertFrames, frame)
		}
	}
	patch.DeleteFrameIds = sortedInt32Values(c.frameDeletes)
	if !patch.HasCurrentIconUrl && !patch.HasCurrentFrameId &&
		len(patch.UpsertIcons) == 0 && len(patch.DeleteIconIds) == 0 &&
		len(patch.UpsertFrames) == 0 && len(patch.DeleteFrameIds) == 0 {
		return false
	}
	dst.IconPatch = patch
	return true
}

func (c *IconComponent) ClearDirty() {
	c.iconUpserts = nil
	c.iconDeletes = nil
	c.frameUpserts = nil
	c.frameDeletes = nil
	c.iconEquipDirty = false
}

func iconSection() roleSection {
	return messageSection(g1_protocol.ERoleSectionFlag_ICON_INFO, "icon",
		func(i *g1_protocol.RoleInfo) *g1_protocol.RoleIconInfo { return i.IconInfo },
		func(i *g1_protocol.RoleInfo, m *g1_protocol.RoleIconInfo) { i.IconInfo = m },
		func() *g1_protocol.RoleIconInfo { return new(g1_protocol.RoleIconInfo) })
}
